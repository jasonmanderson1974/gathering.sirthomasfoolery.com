# Plugin API Documentation

This document describes the plugin API for interacting with The Fellowship's events. Plugins can retrieve and update user availability through `get-slots` and `set-slots` methods.

> **Breaking change (E3, 2026-07-28): the `guestName` / `guestEmail` parameters are gone.**
> The plugin now acts as the **signed-in user**, always. Every event route requires a session,
> so the page the plugin talks to always has one, and the server keys the response off that
> session rather than a name in the payload. Consequences for plugin authors:
> - `guestName` no longer forces "guest mode", and passing it has no effect (it is ignored).
> - `guestEmail` is likewise ignored; the address comes from the account.
> - The `localStorage[eventId + ".guestName"]` identity is gone and is neither read nor written.
> - `get-slots` no longer accepts a guest identity, so it returns what the signed-in user is
>   permitted to see — which, when *Hide responses from respondents* is on, is their own response.
>
> Entering availability on behalf of someone without an account is still supported, but only
> through the UI ("+ Add guest availability"), not through this API.

## Overview

Plugins communicate with the Timeful frontend via `window.postMessage`. The frontend validates incoming messages and routes them to the appropriate handler method. Responses are sent back using the same mechanism.

## Message Handler

The `handleMessage` method validates incoming messages and routes them to the appropriate handler:

- Messages with `payload.type === "get-slots"` are routed to `getSlots()`
- Messages with `payload.type === "set-slots"` are routed to `setSlots()`

## Request Format

All plugin API requests must follow this format:

```javascript
window.postMessage({
  type: "FILL_CALENDAR_EVENT",
  requestId: "unique-request-id",  // Used to match requests with responses
  payload: {
    type: "get-slots" | "set-slots",
    // ... additional payload fields (see below)
  }
}, "*")
```

## get-slots

### Description

Retrieves availability slots for all respondents to an event. Returns slots in the user's local timezone by default, with the ability to optionally specify timezone.

### Request Format

```javascript
{
  type: "FILL_CALENDAR_EVENT",
  requestId: "get-slots-123",
  payload: {
    type: "get-slots",
    timezone: "GMT" // Optional: IANA timezone name
  }
}
```

**Optional payload fields:**
- `timezone`: IANA timezone name (e.g., `"America/Los_Angeles"`, `"Asia/Kolkata"`). If not provided, uses `localStorage["timezone"].value` or browser's local timezone.

### Response Format

**Success response:**
```javascript
{
  type: "FILL_CALENDAR_EVENT_RESPONSE",
  command: "get-slots",
  requestId: "get-slots-123",
  ok: true,
  payload: {
    slots: {
      "userId1": {
        name: "John Doe",
        email: "john@example.com",
        availability: ["2026-01-07T09:00:00", "2026-01-07T09:15:00", ...],
        ifNeeded: ["2026-01-07T14:00:00", ...]
      },
      "Mary Smith": {
        name: "Mary Smith",  // A legacy name-keyed response (pre-E3, or entered
        email: "",           // on-behalf through the UI). Read-only via this API.
        availability: [...],
        ifNeeded: [...]
      }
    },
    timeIncrement: 15  // Time increment in minutes (15, 30, or 60)
  }
}
```

**Error response:**
```javascript
{
  type: "FILL_CALENDAR_EVENT_RESPONSE",
  command: "get-slots",
  requestId: "get-slots-123",
  ok: false,
  error: {
    message: "Error message here"
  }
}
```

### Example Request

```javascript
window.postMessage({
  type: "FILL_CALENDAR_EVENT",
  requestId: "test-get-slots-" + Date.now(),
  payload: {
    type: "get-slots"
  }
}, "*")
```

### Timezone Conversion

- Slots are stored in UTC in the backend
- `get-slots` converts UTC timestamps to the user's local timezone before returning
- Timezone is determined by (in priority order):
  1. `localStorage["timezone"].value` (if set)
  2. Browser's local timezone (`Intl.DateTimeFormat().resolvedOptions().timeZone`)
- Returned timestamps are in ISO format without timezone (e.g., `"2026-01-07T09:00:00"`)

## set-slots

### Description

Sets availability slots for the current user (logged-in user or guest). Converts timestamps from the user's timezone to UTC before storing in the backend. **Completely overwrites** existing availability (does not merge with previous slots).

### Request Format

```javascript
{
  type: "FILL_CALENDAR_EVENT",
  requestId: "set-slots-123",
  payload: {
    type: "set-slots",
    timezone: "America/Los_Angeles",  // Optional: IANA timezone name
    slots: [
      {
        start: "2026-01-07T09:00:00",  // ISO format without timezone
        end: "2026-01-07T12:00:00",
        status: "available" | "if-needed"
      },
      {
        start: "2026-01-07T14:00:00",
        end: "2026-01-07T16:00:00",
        status: "if-needed"
      }
    ]
  }
}
```

**Required payload fields:**
- `slots`: Array of slot objects, each with:
  - `start`: Start time (ISO string without timezone)
  - `end`: End time (ISO string without timezone)
  - `status`: Either `"available"` or `"if-needed"`

**Optional payload fields:**
- `timezone`: IANA timezone name (e.g., `"America/Los_Angeles"`, `"Asia/Kolkata"`). If not provided, uses `localStorage["timezone"].value` or browser's local timezone.

### Response Format

**Success response:**
```javascript
{
  type: "FILL_CALENDAR_EVENT_RESPONSE",
  command: "set-slots",
  requestId: "set-slots-123",
  ok: true
}
```

**Error response:**
```javascript
{
  type: "FILL_CALENDAR_EVENT_RESPONSE",
  command: "set-slots",
  requestId: "set-slots-123",
  ok: false,
  error: {
    message: "Error message here"
  }
}
```

### Example Request

**Basic request (logged-in user or guest with name in localStorage):**
```javascript
window.postMessage({
  type: "FILL_CALENDAR_EVENT",
  requestId: "test-set-slots-" + Date.now(),
  payload: {
    type: "set-slots",
    timezone: "Asia/Kolkata",  // IST
    slots: [
      {
        start: "2026-01-07T09:00:00",
        end: "2026-01-07T12:00:00",
        status: "available"
      },
      {
        start: "2026-01-07T12:00:00",
        end: "2026-01-07T16:00:00",
        status: "if-needed"
      }
    ]
  }
}, "*")
```

- Validates that all slots fall within the event's date/time range
- Validates that `start` < `end` for each slot
- Validates that `status` is either `"available"` or `"if-needed"`
- **Overlapping intervals with different statuses**: If overlapping intervals have conflicting statuses (one "available", one "if-needed"), an error will be thrown: `"Conflicting status for timestamp [timestamp]: already marked as '[status1]' but also marked as '[status2]'. Overlapping intervals must have the same status."`
- For **DOW (days of week) events**: Validates that the date part of `start` and `end` timestamps match one of the hardcoded day dates listed above
- Returns appropriate error messages if validation fails

### Limitations

- **Group events are not supported** - returns an error if the event type is GROUP
- **Acts as the signed-in user only** - there is no way to write availability under another
  name through this API. Use the UI's "+ Add guest availability" for that.
- **A session is required** - every event endpoint is behind authentication, so the plugin only
  works on a page where someone is signed in.
- **Complete overwrite** - The slots you send in **completely clear out** any old slots and write these new ones in. This is not a merge operation.


## Testing

Use the browser console to test (don't forget to add listeners to intercept error/success messages):

**Get slots:**
```javascript
// Add listener first
window.addEventListener("message", (e) => {
  if (e.data?.type === "FILL_CALENDAR_EVENT_RESPONSE" && e.data?.command === "get-slots") {
    if (e.data.ok) {
      console.log("Success:", e.data.payload)
    } else {
      console.error("Error:", e.data.error)
    }
  }
})

// Then send request
window.postMessage({
  type: "FILL_CALENDAR_EVENT",
  requestId: "test-" + Date.now(),
  payload: { type: "get-slots" }
}, "*")
```

**Set slots:**
```javascript
// Add listener first
window.addEventListener("message", (e) => {
  if (e.data?.type === "FILL_CALENDAR_EVENT_RESPONSE" && e.data?.command === "set-slots") {
    if (e.data.ok) {
      console.log("Success: Slots updated")
    } else {
      console.error("Error:", e.data.error)
    }
  }
})

// Then send request
window.postMessage({
  type: "FILL_CALENDAR_EVENT",
  requestId: "test-" + Date.now(),
  payload: {
    type: "set-slots",
    timezone: "Asia/Kolkata",
    slots: [
      { start: "2026-01-07T09:00:00", end: "2026-01-07T12:00:00", status: "available" }
    ]
  }
}, "*")
```

