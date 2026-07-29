import { get, post, patch, put, _delete } from "../fetch_utils"

export const archiveEvent = (eventId, archive) => {
  return post(`/events/${eventId}/archive`, {
    archive: archive,
  })
}

/**
 * Confirm (or cancel) a gathering's locked-in time and arm the pre-gathering
 * reminder email. Pass { scheduled: false } to cancel a previously-set gathering.
 * @param {string} eventId
 * A recurrenceFrequency of "weekly" | "biweekly" | "monthly" makes it a
 * repeating gathering (C5); omit or "none" for a one-off.
 * @param {{scheduled: boolean, startDate?: string, endDate?: string, summary?: string, timezone?: string, reminderEnabled?: boolean, reminderLeadTimeHours?: number, recurrenceFrequency?: "none"|"weekly"|"biweekly"|"monthly", recurrenceUntil?: string}} payload
 */
export const setScheduledEvent = (eventId, payload) => {
  return post(`/events/${eventId}/schedule`, payload)
}

/**
 * RSVP to a confirmed gathering. Keyed to the session server-side — identity is
 * never taken from the payload.
 * @param {string} eventId
 * @param {{status: "going"|"maybe"|"no", guestCount?: number}} payload
 */
export const setRsvp = (eventId, payload) => {
  return post(`/events/${eventId}/rsvp`, payload)
}

/** Remove the caller's own RSVP. */
export const clearRsvp = (eventId) => {
  return _delete(`/events/${eventId}/rsvp`)
}

// --- Discussion (C7) + threads (C13) ---
// All of these require a signed-in user; the discussion is not open to guests.

/** Post a comment, or a reply when threadId is set. payload: {text, threadId?} */
export const addComment = (eventId, payload) => {
  return post(`/events/${eventId}/comments`, payload)
}

/** Edit your own comment. payload: {text} */
export const editComment = (eventId, commentId, payload) => {
  return put(`/events/${eventId}/comments/${commentId}`, payload)
}

/** Delete a comment (own, or any if you're the owner). Deleting a thread root
 *  also deletes its replies. */
export const deleteComment = (eventId, commentId) => {
  return _delete(`/events/${eventId}/comments/${commentId}`)
}

/** Tag a comment as a thread (member+). payload: {membersOnly: boolean} */
export const tagThread = (eventId, commentId, payload) => {
  return post(`/events/${eventId}/comments/${commentId}/thread`, payload)
}

/** Flip a thread's members-only setting. payload: {membersOnly: boolean} */
export const setThreadMembersOnly = (eventId, commentId, payload) => {
  return patch(`/events/${eventId}/comments/${commentId}/thread`, payload)
}

/** Un-tag a thread. Rejects with 409 "thread-has-replies" once it has replies. */
export const untagThread = (eventId, commentId) => {
  return _delete(`/events/${eventId}/comments/${commentId}/thread`)
}

// --- Venue / activity polls (C6) ---

/** Create a poll (owner only). payload: {title, allowMultiple?, options: string[]} */
export const createPoll = (eventId, payload) => {
  return post(`/events/${eventId}/polls`, payload)
}

/** Delete a poll (owner only). */
export const deletePoll = (eventId, pollId) => {
  return _delete(`/events/${eventId}/polls/${pollId}`)
}

/**
 * Cast/update the caller's vote in a poll (members + guests). An empty
 * optionIds clears the caller's vote. payload: {optionIds, guest, name?}
 */
export const votePoll = (eventId, pollId, payload) => {
  return post(`/events/${eventId}/polls/${pollId}/vote`, payload)
}

// --- Shared lists (F13/F14, extended by F15/F16) ---
// The planner owns the lists; everyone signed in owns what they put on them.

/**
 * Fetch just the lists, author and checker names resolved.
 *
 * This is the refresh path (F15). Refetching the whole event to see one new
 * entry means paying for a user lookup per availability response and again per
 * sign-up response, which is the wrong trade when several people are working a
 * list at once. Always an array, never null.
 */
export const getLists = (eventId) => {
  return get(`/events/${eventId}/lists`)
}

/** Create a list (planner/admin). payload: {name, kind: "text"|"location"|"checklist"} */
export const createList = (eventId, payload) => {
  return post(`/events/${eventId}/lists`, payload)
}

/** Rename a list (planner/admin). payload: {name} */
export const renameList = (eventId, listId, payload) => {
  return patch(`/events/${eventId}/lists/${listId}`, payload)
}

/** Delete a list and everything on it (planner/admin). */
export const deleteList = (eventId, listId) => {
  return _delete(`/events/${eventId}/lists/${listId}`)
}

/**
 * Add an item to a list (any signed-in user).
 * payload: {text, parentId?} — parentId nests it under an existing item, up to
 * three levels deep.
 */
export const addListItem = (eventId, listId, payload) => {
  return post(`/events/${eventId}/lists/${listId}/items`, payload)
}

/** Edit one of your own items. payload: {text} */
export const editListItem = (eventId, listId, itemId, payload) => {
  return put(`/events/${eventId}/lists/${listId}/items/${itemId}`, payload)
}

/**
 * Delete an item and everything nested under it (own, or any if you're a member
 * or above). The subtree goes with it, including children someone else added.
 */
export const deleteListItem = (eventId, listId, itemId) => {
  return _delete(`/events/${eventId}/lists/${listId}/items/${itemId}`)
}

/** Tick or untick a checklist item (any signed-in user). */
export const setListItemChecked = (eventId, listId, itemId, checked) => {
  return put(`/events/${eventId}/lists/${listId}/items/${itemId}/checked`, {
    checked,
  })
}
