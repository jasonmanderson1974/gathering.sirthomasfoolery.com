/**
 * Grouping logic for the event discussion (C7) and its threads (C13).
 *
 * The API returns one flat, chronologically-sorted comment list per event; this
 * turns it into the shape the UI renders. Kept as a plain module (no Vue, no
 * Vuetify) so it can be unit-tested directly, mirroring
 * `components/schedule_overlap/scheduleLocation.js`.
 */

/**
 * Truncate a thread root's text for use as its collapsed header. Threads have no
 * separate title — the comment that started them *is* the topic.
 * @param {string} text
 * @param {number} maxLength
 */
export const threadTitle = (text, maxLength = 80) => {
  const collapsed = (text ?? "").replace(/\s+/g, " ").trim()
  if (collapsed.length <= maxLength) return collapsed
  // Prefer breaking at a word boundary so the ellipsis doesn't split a word.
  const clipped = collapsed.slice(0, maxLength)
  const lastSpace = clipped.lastIndexOf(" ")
  return `${lastSpace > maxLength / 2 ? clipped.slice(0, lastSpace) : clipped}…`
}

/**
 * Split a flat comment list into the top-level stream and each thread's replies.
 *
 * Replies whose root isn't present are dropped rather than surfaced at top
 * level: the server filters out members-only roots for guests, and a stray reply
 * rendered without its parent would both look like a non-sequitur and leak the
 * fact that a hidden thread exists.
 *
 * @param {Array<Object>} comments flat list, oldest first, as returned by the API
 * @returns {{topLevel: Array<Object>, repliesByThreadId: Record<string, Array<Object>>}}
 */
export const groupComments = (comments) => {
  const list = Array.isArray(comments) ? comments : []

  const topLevel = list.filter((c) => !c.threadId)
  const rootIds = new Set(topLevel.filter((c) => c.isThread).map((c) => c._id))

  const repliesByThreadId = {}
  for (const comment of list) {
    if (!comment.threadId || !rootIds.has(comment.threadId)) continue
    if (!repliesByThreadId[comment.threadId]) repliesByThreadId[comment.threadId] = []
    repliesByThreadId[comment.threadId].push(comment)
  }

  return { topLevel, repliesByThreadId }
}

/** Number of replies in a thread, for the collapsed header's count. */
export const replyCount = (repliesByThreadId, threadId) =>
  repliesByThreadId[threadId]?.length ?? 0

/** "3 replies" / "1 reply" / "No replies yet" for the collapsed header. */
export const replyCountLabel = (count) => {
  if (count === 0) return "No replies yet"
  return count === 1 ? "1 reply" : `${count} replies`
}
