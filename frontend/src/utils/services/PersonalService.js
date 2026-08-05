import { get, post, patch, put, _delete } from "../fetch_utils"

/**
 * The private tabs on a gathering: My Lists (F19) and My Notes (F20).
 *
 * Everything here is scoped to the signed-in caller by the SERVER — the userId
 * comes from the session and goes into every Mongo filter, so none of these
 * calls can name whose data it wants and none of them needs to. There is no
 * user id in any of these paths, and that is the point.
 *
 * Kept out of EventService.js: these read and write a different collection
 * entirely, and one file per privacy boundary is worth the extra import.
 */

// --- My Lists (F19) ---
// Deliberately mirrors the shared-lists section of EventService.js one for one:
// the two panels are the same component, so the two APIs staying the same shape
// is what keeps its handlers indifferent to which one it is rendering.

/** Fetch the caller's own lists. Always an array, never null. */
export const getPersonalLists = (eventId) => {
  return get(`/events/${eventId}/my-lists`)
}

/** Create a list. payload: {name, kind: "text"|"location"|"checklist"} */
export const createPersonalList = (eventId, payload) => {
  return post(`/events/${eventId}/my-lists`, payload)
}

/** Rename a list. payload: {name} */
export const renamePersonalList = (eventId, listId, payload) => {
  return patch(`/events/${eventId}/my-lists/${listId}`, payload)
}

/** Delete a list and everything on it. */
export const deletePersonalList = (eventId, listId) => {
  return _delete(`/events/${eventId}/my-lists/${listId}`)
}

/** Add an entry. payload: {text, parentId?} — parentId nests it. */
export const addPersonalListItem = (eventId, listId, payload) => {
  return post(`/events/${eventId}/my-lists/${listId}/items`, payload)
}

/** Rewrite an entry's text. payload: {text} */
export const editPersonalListItem = (eventId, listId, itemId, payload) => {
  return put(`/events/${eventId}/my-lists/${listId}/items/${itemId}`, payload)
}

/** Delete an entry and everything nested under it. */
export const deletePersonalListItem = (eventId, listId, itemId) => {
  return _delete(`/events/${eventId}/my-lists/${listId}/items/${itemId}`)
}

/** Tick or untick a checklist entry. */
export const setPersonalListItemChecked = (eventId, listId, itemId, checked) => {
  return put(`/events/${eventId}/my-lists/${listId}/items/${itemId}/checked`, {
    checked,
  })
}

/**
 * Reposition an entry, within its list or onto another of the caller's own.
 * payload: {targetListId, order, parentId?} — parentId may ONLY name the
 * entry's current parent, for a reorder among its siblings; anything else
 * flattens it to the top level. See resolveDrop in eventLists.js.
 */
export const movePersonalListItem = (eventId, listId, itemId, payload) => {
  return put(`/events/${eventId}/my-lists/${listId}/items/${itemId}/move`, payload)
}

// --- My Notes (F20) ---

/**
 * Fetch the caller's own note. A note nobody has written yet comes back as
 * `{text: "", updatedAt: null}` rather than a 404 — "no note" and "empty note"
 * are the same thing to the person looking at the editor.
 * @returns {Promise<{text: string, updatedAt: number|null}>}
 */
export const getPersonalNote = (eventId) => {
  return get(`/events/${eventId}/my-notes`)
}

/**
 * Replace the whole note. Sending "" is how a note is cleared; there is no
 * DELETE. Returns what was actually stored, so the panel's "Saved 12:04" comes
 * from the server's clock rather than the client's.
 * @returns {Promise<{text: string, updatedAt: number|null}>}
 */
export const savePersonalNote = (eventId, text) => {
  return put(`/events/${eventId}/my-notes`, { text })
}
