import {
  get,
  post,
  sendPluginError,
  sendPluginSuccess,
  isValidPluginMessage,
  convertToUTC,
  convertUTCSlotsToLocalISO,
  validateDOWPayload,
  timezoneObservesDST,
} from "@/utils"
import { eventTypes, allTimezones } from "@/constants"
import dayjs from "dayjs"
import utcPlugin from "dayjs/plugin/utc"
import timezonePlugin from "dayjs/plugin/timezone"
dayjs.extend(utcPlugin)
dayjs.extend(timezonePlugin)

/**
 * Browser-plugin postMessage API handlers for the Event view.
 *
 * Extracted verbatim from Event.vue as a Vue 2 mixin (TODO A11). Mixin methods
 * run against the same component instance, so every `this.*` reference
 * (this.event, this.authUser, this.refreshEvent, ...) resolves exactly as
 * before — a behavior-preserving move, not a rewrite.
 *
 * `handleMessage` is registered/removed on `window` in Event.vue's
 * mounted/destroyed hooks. Message shapes are specced in PLUGIN_API_README.md —
 * don't change them without updating that doc.
 */
export default {
  methods: {
    handleMessage(event) {
      if (!isValidPluginMessage(event)) return

      const payload = event.data.payload

      if (payload?.type === "get-slots") {
        this.getSlots(event)
      }

      if (payload?.type === "set-slots") {
        this.setSlots(event)
      }
    },

    async setSlots(event) {
      const requestId = event.data?.requestId
      const command = "set-slots"
      // Validation: Check event exists
      if (!this.event) {
        sendPluginError(requestId, command, "Event not loaded yet")
        return
      }

      // Validation: Check timeIncrement exists, default to 15 if not
      const timeIncrement = this.event.timeIncrement ?? 15

      // E3: the plugin acts as the signed-in user. The `guestName` parameter
      // (which forced guest mode and let the caller write availability under an
      // arbitrary name) is gone, along with the localStorage guest identity it
      // fell back to. The server keys the response off the session now, and the
      // blind-availability check that used to guard this is enforced there
      // rather than in the client. See PLUGIN_API_README.md.

      // Get slots from payload - new format: [{ start, end, status }]
      let slots = event.data?.payload?.slots

      if (!Array.isArray(slots)) {
        sendPluginError(requestId, command, "Slots must be an array")
        return
      }

      // Validate DOW payload if this is a DOW event (only if slots are provided)
      // Check if timezone is provided - if so, skip same-day check since timezone conversion may cause day boundary crossing
      const hasTimezone = !!event.data?.payload?.timezone
      if (this.event.type === eventTypes.DOW && slots.length > 0) {
        const validationResult = validateDOWPayload(slots, hasTimezone)
        if (validationResult) {
          sendPluginError(requestId, command, validationResult.error)
          return
        }
      }

      if (this.event.type === eventTypes.DOW && slots.length > 0) {
        //need to offset for DOW cuz dow dates are in DST
        slots = slots.map((slot) => {
          const startDate = dayjs(slot.start)
          const endDate = dayjs(slot.end)
          return {
            ...slot,
            start: startDate.add(1, "hour").format("YYYY-MM-DDTHH:mm:ss"),
            end: endDate.add(1, "hour").format("YYYY-MM-DDTHH:mm:ss"),
          }
        })
      }

      // Determine timezone for conversion
      // Priority: 1. User-provided timezone in payload, 2. localStorage, 3. Browser's local timezone
      let timezoneValue = null
      if (event.data?.payload?.timezone) {
        // User provided timezone in the message (should be IANA timezone name)
        const providedTimezone = event.data.payload.timezone

        // Validate that the provided timezone exists in allTimezones
        if (!(providedTimezone in allTimezones)) {
          sendPluginError(
            requestId,
            command,
            `Invalid timezone: "${providedTimezone}". Please provide a valid IANA timezone name from the supported timezones list.`
          )
          return
        }

        timezoneValue = providedTimezone
      } else {
        // Use timezone from localStorage (should have IANA timezone name in .value)
        try {
          const timezoneObj = JSON.parse(localStorage["timezone"])
          timezoneValue = timezoneObj.value
        } catch (err) {
          // If parsing fails, fall back to browser's local timezone
          timezoneValue = Intl.DateTimeFormat().resolvedOptions().timeZone
        }
      }

      // Generate all valid displayed time ranges using ScheduleOverlap's existing logic
      // Returns a Map that maps time slot startTime.getTime() to { row, col, startTime, endTime }
      const timeSlotToRowCol =
        this.scheduleOverlapComponent &&
        typeof this.scheduleOverlapComponent.getAllValidTimeRanges ===
          "function"
          ? this.scheduleOverlapComponent.getAllValidTimeRanges()
          : new Map()

      // Validate each slot has required fields
      for (let i = 0; i < slots.length; i++) {
        const slot = slots[i]
        if (!slot.start || !slot.end) {
          sendPluginError(
            requestId,
            command,
            `Slot at index ${i} is missing required 'start' or 'end' field`
          )
          return
        }
        if (!slot.status) {
          sendPluginError(
            requestId,
            command,
            `Slot at index ${i} is missing required 'status' field`
          )
          return
        }
        if (slot.status !== "available" && slot.status !== "if-needed") {
          sendPluginError(
            requestId,
            command,
            `Invalid status '${slot.status}' at index ${i}. Must be 'available' or 'if-needed'`
          )
          return
        }
      }

      // Validate that all start/end times fall within event's date range
      const eventDates = this.event.dates.map((d) => new Date(d))
      const eventStartTime = this.event.startTime // Hours (e.g., 9 for 9am)
      const eventDuration = this.event.duration // Hours

      // Convert all slot times from user's timezone to UTC and validate
      const convertedSlots = []
      for (let i = 0; i < slots.length; i++) {
        const slot = slots[i]

        // Convert timestamps from user's timezone to UTC
        let startTime, endTime
        try {
          startTime = convertToUTC(slot.start, timezoneValue)
          endTime = convertToUTC(slot.end, timezoneValue)
        } catch (err) {
          sendPluginError(
            requestId,
            command,
            `Failed to parse time at index ${i}: ${err.message}`
          )
          return
        }

        if (isNaN(startTime.getTime())) {
          sendPluginError(
            requestId,
            command,
            `Invalid start time at index ${i}: ${slot.start}`
          )
          return
        }

        if (isNaN(endTime.getTime())) {
          sendPluginError(
            requestId,
            command,
            `Invalid end time at index ${i}: ${slot.end}`
          )
          return
        }

        if (endTime <= startTime) {
          sendPluginError(
            requestId,
            command,
            `End time must be after start time at index ${i}`
          )
          return
        }
      }

      // Split slots into intervals based on timeIncrement
      const allAvailabilityTimestamps = []
      const allIfNeededTimestamps = []
      // Track timestamps and their statuses to detect conflicts
      const timestampStatusMap = new Map()

      let isBrokenBounds = false
      slots.forEach((slot, i) => {
        const userStartDate = dayjs.tz(slot.start, timezoneValue)
        const userEndDate = dayjs.tz(slot.end, timezoneValue)
        const userStartMs = userStartDate.valueOf()
        const userEndMs = userEndDate.valueOf()

        // Calculate the width of the user's interval
        const intWidth = userEndMs - userStartMs

        // Calculate total covered width by summing all overlapping slot intersections
        // Also generate timestamps in the same loop
        let coveredWidth = 0

        timeSlotToRowCol.forEach((value, key) => {
          const slotStartMs = value.startTime.valueOf()
          const slotEndMs = value.endTime.valueOf()

          // Check for overlap: userStart <= slotEnd && userEnd >= slotStart
          if (userStartMs <= slotEndMs && userEndMs >= slotStartMs) {
            // Calculate intersection of user interval and slot
            const intersectionStartMs = Math.max(userStartMs, slotStartMs)
            const intersectionEndMs = Math.min(userEndMs, slotEndMs)

            // Add this intersection's width to the total for bounds checking
            coveredWidth += intersectionEndMs - intersectionStartMs

            // Generate timestamps at timeIncrement intervals
            const incrementMs = timeIncrement * 60 * 1000
            let currentTimeMs = intersectionStartMs

            // Generate timestamps for the intersection
            // Use <= to include boundary timestamps when intersection is exactly at slot boundaries
            while (currentTimeMs < intersectionEndMs) {
              const timestamp = new Date(currentTimeMs)
              const timestampKey = timestamp.getTime()

              // Check for status conflicts
              if (timestampStatusMap.has(timestampKey)) {
                const existingStatus = timestampStatusMap.get(timestampKey)
                if (existingStatus !== slot.status) {
                  sendPluginError(
                    requestId,
                    command,
                    `Time slot at index ${i} overlaps with another time slot with different status`
                  )
                  return
                }
              } else {
                timestampStatusMap.set(timestampKey, slot.status)
              }

              // Add Date object (not milliseconds) to appropriate array
              if (slot.status === "available") {
                allAvailabilityTimestamps.push(timestamp)
              } else {
                allIfNeededTimestamps.push(timestamp)
              }

              currentTimeMs += incrementMs

              // Stop if we've exceeded the intersection end
              if (currentTimeMs > intersectionEndMs) {
                break
              }
            }
          }
        })

        if (coveredWidth < intWidth) {
          sendPluginError(
            requestId,
            command,
            `Time slot at index ${i} (${slot.start} to ${slot.end}) falls outside the event's date/time range.`
          )
          isBrokenBounds = true
        }
      })

      if (isBrokenBounds) return

      // Send new slots (overwrites existing availability)
      try {
        const sanitizedId = this.eventId.replaceAll(".", "")
        const payload = {
          availability: allAvailabilityTimestamps,
          ifNeeded: allIfNeededTimestamps,
        }

        // The backend identifies the responder from the session.
        payload.guest = false

        await post(`/events/${sanitizedId}/response`, payload)

        // Trigger frontend refresh to update UI
        await this.refreshEvent()

        sendPluginSuccess(requestId, command)
      } catch (err) {
        sendPluginError(
          requestId,
          command,
          `Failed to set slots: ${err.message || "Unknown error"}`
        )
      }
    },

    async getSlots(event) {
      const requestId = event.data?.requestId
      const command = "get-slots"

      // Need the event to calculate timeMin and timeMax
      if (!this.event) {
        sendPluginError(requestId, command, "Event not loaded yet")
        return
      }

      // Resolve timezone: same logic as set-slots (payload → localStorage → browser)
      let timezoneValue = null
      if (event.data?.payload?.timezone) {
        const providedTimezone = event.data.payload.timezone
        if (!(providedTimezone in allTimezones)) {
          sendPluginError(
            requestId,
            command,
            `Invalid timezone: "${providedTimezone}". Please provide a valid IANA timezone name from the supported timezones list.`
          )
          return
        }
        timezoneValue = providedTimezone
      } else {
        try {
          const timezoneObj = JSON.parse(localStorage["timezone"])
          timezoneValue = timezoneObj.value
        } catch (err) {
          timezoneValue = Intl.DateTimeFormat().resolvedOptions().timeZone
        }
      }

      let sanitizedId = this.eventId.replaceAll(".", "")

      // Calculate timeMin and timeMax using the same logic as fetchResponses in ScheduleOverlap
      let timeMin, timeMax
      if (this.event.dates.length > 0) {
        // Fetch the entire time range of availabilities
        timeMin = new Date(this.event.dates[0])
        timeMax = new Date(this.event.dates[this.event.dates.length - 1])
        timeMax.setDate(timeMax.getDate() + 1)
      }

      if (!timeMin || !timeMax) {
        sendPluginError(
          requestId,
          command,
          "Could not calculate timeMin and timeMax"
        )
        return
      }

      try {
        // Fetch responses between timeMin and timeMax

        const url = `/events/${sanitizedId}/responses?timeMin=${timeMin.toISOString()}&timeMax=${timeMax.toISOString()}`
        const responses = await get(url)

        // Build response object with all users' slots
        const allSlots = {}

        for (const userId in responses) {
          const response = responses[userId]

          // Get name and email
          let name = ""
          let email = ""

          // For guests, name and email are in the response directly
          if (response.name && response.name.length > 0) {
            name = response.name
            email = response.email || ""
          } else {
            // For logged-in users, get from this.event.responses (populated by getEvent endpoint)
            const eventResponse = this.event.responses?.[userId]
            if (eventResponse?.user) {
              const user = eventResponse.user
              name = `${user.firstName || ""} ${user.lastName || ""}`.trim()
              email = user.email || ""
            } else {
              // Fallback: use userId if user info not available
              name = userId
              email = ""
            }
          }

          // Convert UTC to requested timezone. For DOW events, if timezone observes DST, subtract 1 hour
          // (hardcoded DOW dates are in DST, so conversion in DST timezones is 1 hour ahead)
          let availability = convertUTCSlotsToLocalISO(
            response.availability,
            timezoneValue
          )
          let ifNeeded = convertUTCSlotsToLocalISO(
            response.ifNeeded,
            timezoneValue
          )
          if (
            this.event.type === eventTypes.DOW &&
            timezoneObservesDST(timezoneValue)
          ) {
            const subtractOneHour = (s) =>
              dayjs
                .tz(s, timezoneValue)
                .subtract(1, "hour")
                .format("YYYY-MM-DDTHH:mm:ss")
            availability = availability.map(subtractOneHour)
            ifNeeded = ifNeeded.map(subtractOneHour)
          }

          allSlots[userId] = {
            name,
            email,
            availability,
            ifNeeded,
          }
        }

        // Get time increment (default to 15 if not set)
        const timeIncrement = this.event.timeIncrement ?? 15

        sendPluginSuccess(requestId, command, {
          slots: allSlots,
          timeIncrement,
          timezone: timezoneValue,
        })
      } catch (err) {
        sendPluginError(
          requestId,
          command,
          `Failed to fetch responses: ${err.message || "Unknown error"}`
        )
      }
    },
  },
}
