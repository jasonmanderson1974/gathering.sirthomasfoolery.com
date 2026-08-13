/*
  The operations the write queue knows how to replay, and how each one edits the
  cached copy while it waits.

  THE REDUCERS ARE THE DESIGN DECISION HERE. This codebase is deliberately
  non-optimistic — every handler does `await service(); await refreshEvent()`,
  and `expensesMixin.js` says so outright: "Nothing here is optimistic". Making
  offline writes visible by rewriting ~30 call sites to update local state would
  reverse that convention and be wrong in thirty places instead of one.

  So a queued op instead applies a pure reducer to the CACHED PAYLOAD — the same
  edit the server would have made. The refetch that every handler already does
  then reads the change straight back out of cache, and not one call site
  changes. The cost is one reducer per op, which is pure JS and belongs in the
  fast node tier.

  Each entry:
    send(op, id)  performs the real request. `id` maps a temporary id minted
                  offline onto the real one the server gave it, for ops queued
                  against something still queued itself.
    apply(op)     edits the cache. Never throws; a miss just means that tab was
                  never opened.
    creates       true when the op mints something, so the flush records its
                  real id for the ops behind it.
*/
// Raw HTTP, NOT the service wrappers. The services are where the "am I
// offline?" decision lives, so calling one from here would put a replay
// straight back into the queue it came from. This module owns the wire format
// for a replay; the services own the decision to defer.
import { post, put, _delete } from "@/utils/fetch_utils"
import { eventAliases, mutateCached } from "./cache"

/* ---------------------------------------------------------------- *
 * cache helpers
 * ---------------------------------------------------------------- */

/*
  Every mutation is applied to EVERY spelling of the gathering's id.

  An event is cached under both its Mongo _id and its short id (see
  cache.js/eventAliases). The page is read under whichever was in the link, and
  the write handlers address it as `shortId ?? _id` — so editing only the
  caller's spelling routinely edits the copy nobody is looking at, and the
  change appears not to have happened.
*/
const acrossAliases = async (eventId, suffix, mutate) => {
  let touched = false
  for (const id of eventAliases(eventId)) {
    if (await mutateCached(`/events/${id}${suffix}`, mutate)) touched = true
  }
  return touched
}

const mutateEvent = (eventId, mutate) => acrossAliases(eventId, "", mutate)
const mutateExpenses = (eventId, mutate) =>
  acrossAliases(eventId, "/expenses", mutate)
const mutateMyLists = (eventId, mutate) =>
  acrossAliases(eventId, "/my-lists", mutate)
const mutateMyNotes = (eventId, mutate) =>
  acrossAliases(eventId, "/my-notes", mutate)

/**
 * Applies `mutate` to a list inside whichever cached shapes hold it.
 *
 * The shared lists appear in TWO cached payloads — inside the event
 * (`GET /events/:id` carries `lists`) and on their own (`GET
 * /events/:id/lists`, which the Lists tab refetches). Editing only one leaves
 * the tab and the page disagreeing depending on which was read last, so both
 * are kept in step.
 */
const mutateSharedList = async (eventId, listId, mutate) => {
  const inList = (lists) => {
    if (!Array.isArray(lists)) return false
    const list = lists.find((l) => l?._id === listId)
    if (!list) return false
    list.items = mutate(Array.isArray(list.items) ? list.items : [])
    return true
  }
  await mutateEvent(eventId, (event) => {
    inList(event?.lists)
    return event
  })
  await acrossAliases(eventId, "/lists", (lists) => {
    inList(lists)
    return lists
  })
}

const mutatePersonalList = async (eventId, listId, mutate) =>
  mutateMyLists(eventId, (lists) => {
    if (!Array.isArray(lists)) return lists
    const list = lists.find((l) => l?._id === listId)
    if (list) list.items = mutate(Array.isArray(list.items) ? list.items : [])
    return lists
  })

/** Every descendant of `itemId`, plus itself — mirrors the server's cascade. */
const withDescendants = (items, itemId) => {
  const doomed = new Set([itemId])
  let grew = true
  while (grew) {
    grew = false
    for (const item of items) {
      const parent = item?.parentId
      if (parent && doomed.has(parent) && !doomed.has(item._id)) {
        doomed.add(item._id)
        grew = true
      }
    }
  }
  return doomed
}

const nextOrder = (items) =>
  items.reduce((max, item) => Math.max(max, item?.order ?? 0), 0) + 1000

/* ---------------------------------------------------------------- *
 * the table
 * ---------------------------------------------------------------- */

export const OPS = {
  /* ---- discussion ---- */

  "comment.add": {
    creates: true,
    send: (op, id) =>
      post(`/events/${op.eventId}/comments`, {
        text: op.text,
        threadId: id(op.threadId) || undefined,
        clientId: op.clientId,
      }),
    apply: (op) =>
      mutateEvent(op.eventId, (event) => {
        if (!event) return event
        if (!Array.isArray(event.comments)) event.comments = []
        event.comments.push({
          _id: op.tempId,
          text: op.text,
          threadId: op.threadId ?? null,
          authorName: op.authorName,
          author: op.author ?? null,
          userId: op.userId,
          createdAt: op.createdAt,
          // Marks it as not yet acknowledged by the server. The UI can show it
          // differently; nothing depends on it.
          pending: true,
        })
        return event
      }),
  },

  "comment.edit": {
    send: (op, id) =>
      put(`/events/${op.eventId}/comments/${id(op.commentId)}`, {
        text: op.text,
      }),
    apply: (op) =>
      mutateEvent(op.eventId, (event) => {
        const comment = event?.comments?.find((c) => c?._id === op.commentId)
        if (comment) comment.text = op.text
        return event
      }),
  },

  "comment.delete": {
    send: (op, id) =>
      _delete(`/events/${op.eventId}/comments/${id(op.commentId)}`),
    apply: (op) =>
      mutateEvent(op.eventId, (event) => {
        if (!Array.isArray(event?.comments)) return event
        // Replies go with the root, as the server's cascade does.
        event.comments = event.comments.filter(
          (c) => c?._id !== op.commentId && c?.threadId !== op.commentId
        )
        return event
      }),
  },

  /* ---- shared lists ---- */

  "listItem.add": {
    creates: true,
    send: (op, id) =>
      post(`/events/${op.eventId}/lists/${id(op.listId)}/items`, {
        text: op.text,
        parentId: id(op.parentId) || undefined,
        clientId: op.clientId,
      }),
    apply: (op) =>
      mutateSharedList(op.eventId, op.listId, (items) => [
        ...items,
        {
          _id: op.tempId,
          text: op.text,
          parentId: op.parentId ?? null,
          order: nextOrder(items),
          userId: op.userId,
          authorName: op.authorName,
          createdAt: op.createdAt,
          pending: true,
        },
      ]),
  },

  "listItem.edit": {
    send: (op, id) =>
      put(
        `/events/${op.eventId}/lists/${id(op.listId)}/items/${id(op.itemId)}`,
        { text: op.text }
      ),
    apply: (op) =>
      mutateSharedList(op.eventId, op.listId, (items) =>
        items.map((item) =>
          item?._id === op.itemId ? { ...item, text: op.text } : item
        )
      ),
  },

  "listItem.delete": {
    send: (op, id) =>
      _delete(
        `/events/${op.eventId}/lists/${id(op.listId)}/items/${id(op.itemId)}`
      ),
    apply: (op) =>
      mutateSharedList(op.eventId, op.listId, (items) => {
        // The server takes the whole subtree with it (F17); so does this, or
        // the cached copy keeps orphans the refetch would not have.
        const doomed = withDescendants(items, op.itemId)
        return items.filter((item) => !doomed.has(item?._id))
      }),
  },

  "listItem.check": {
    send: (op, id) =>
      put(
        `/events/${op.eventId}/lists/${id(op.listId)}/items/${id(op.itemId)}/checked`,
        { checked: op.checked }
      ),
    apply: (op) =>
      mutateSharedList(op.eventId, op.listId, (items) =>
        items.map((item) =>
          item?._id === op.itemId
            ? {
                ...item,
                checked: op.checked,
                checkedBy: op.checked ? op.userId : undefined,
                checkedByName: op.checked ? op.authorName : undefined,
                checkedAt: op.checked ? op.createdAt : undefined,
              }
            : item
        )
      ),
  },

  /* ---- Settle Up ---- */

  "expense.add": {
    creates: true,
    send: (op) =>
      post(`/events/${op.eventId}/expenses`, {
        ...op.payload,
        clientId: op.clientId,
      }),
    apply: (op) =>
      mutateExpenses(op.eventId, (expenses) => {
        const rows = Array.isArray(expenses) ? expenses : []
        // Same order the server lists in: date desc, newest first.
        return [{ ...op.preview, _id: op.tempId, pending: true }, ...rows]
      }),
  },

  "expense.edit": {
    send: (op, id) =>
      put(`/events/${op.eventId}/expenses/${id(op.expenseId)}`, op.payload),
    apply: (op) =>
      mutateExpenses(op.eventId, (expenses) =>
        (Array.isArray(expenses) ? expenses : []).map((row) =>
          row?._id === op.expenseId
            ? { ...row, ...op.preview, _id: row._id, pending: true }
            : row
        )
      ),
  },

  "expense.delete": {
    send: (op, id) =>
      _delete(`/events/${op.eventId}/expenses/${id(op.expenseId)}`),
    apply: (op) =>
      mutateExpenses(op.eventId, (expenses) =>
        (Array.isArray(expenses) ? expenses : []).filter(
          (row) => row?._id !== op.expenseId
        )
      ),
  },

  /* ---- My Lists ---- */

  "personalItem.add": {
    creates: true,
    send: (op, id) =>
      post(`/events/${op.eventId}/my-lists/${id(op.listId)}/items`, {
        text: op.text,
        parentId: id(op.parentId) || undefined,
        clientId: op.clientId,
      }),
    apply: (op) =>
      mutatePersonalList(op.eventId, op.listId, (items) => [
        ...items,
        {
          _id: op.tempId,
          text: op.text,
          parentId: op.parentId ?? null,
          order: nextOrder(items),
          createdAt: op.createdAt,
          pending: true,
        },
      ]),
  },

  "personalItem.edit": {
    send: (op, id) =>
      put(
        `/events/${op.eventId}/my-lists/${id(op.listId)}/items/${id(op.itemId)}`,
        { text: op.text }
      ),
    apply: (op) =>
      mutatePersonalList(op.eventId, op.listId, (items) =>
        items.map((item) =>
          item?._id === op.itemId ? { ...item, text: op.text } : item
        )
      ),
  },

  "personalItem.delete": {
    send: (op, id) =>
      _delete(
        `/events/${op.eventId}/my-lists/${id(op.listId)}/items/${id(op.itemId)}`
      ),
    apply: (op) =>
      mutatePersonalList(op.eventId, op.listId, (items) => {
        const doomed = withDescendants(items, op.itemId)
        return items.filter((item) => !doomed.has(item?._id))
      }),
  },

  "personalItem.check": {
    send: (op, id) =>
      put(
        `/events/${op.eventId}/my-lists/${id(op.listId)}/items/${id(op.itemId)}/checked`,
        { checked: op.checked }
      ),
    apply: (op) =>
      mutatePersonalList(op.eventId, op.listId, (items) =>
        items.map((item) =>
          item?._id === op.itemId
            ? { ...item, checked: op.checked, checkedAt: op.createdAt }
            : item
        )
      ),
  },

  /* ---- My Notes ---- */

  // Absolute state, so a replay is harmless and two queued saves collapse to
  // the last one (see the queue's collapse rules).
  "note.save": {
    send: (op) => put(`/events/${op.eventId}/my-notes`, { text: op.text }),
    apply: (op) =>
      mutateMyNotes(op.eventId, (note) => ({
        ...(note ?? {}),
        text: op.text,
        updatedAt: op.createdAt,
      })),
  },
}

/** Which ops, if queued twice for the same target, need only the last one. */
export const ABSOLUTE_STATE_OPS = new Set([
  "note.save",
  "listItem.check",
  "personalItem.check",
])
