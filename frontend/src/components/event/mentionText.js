/**
 * Pure helpers for @mentions in the discussion (F9) — both halves of the
 * feature: turning a stored comment into renderable parts, and turning what
 * someone is typing into a stored token.
 *
 * The token format is the server's (`server/routes/mentions.go`): a mention
 * lives inside the comment text as `@[Display Name](userId)`, which is what
 * makes it survive an edit. **The pattern below must stay in step with
 * `mentionPattern` there** — a token this file writes but the server won't
 * match is a mention that renders and never notifies.
 *
 * Kept out of the components so it can be unit-tested: the vitest env is node
 * with no jsdom, so nothing inside a .vue file is reachable. Same split
 * `commentThreads.js` and `eventLists.js` use.
 */

import { displayName } from "@/utils/general_utils"

/**
 * One persisted mention token. Built fresh per call rather than shared: a
 * global regex carries `lastIndex` between uses, and two callers sharing one
 * would skip matches depending on who ran first.
 */
const mentionPattern = () => /@\[([^\]\n]{1,60})\]\(([0-9a-f]{24})\)/g

/** How long a display name may be inside a token — the server's `{1,60}`. */
const maxTokenNameLength = 60

/** Candidates offered at once. Enough to scan, few enough not to cover the composer. */
export const maxMentionCandidates = 8

/**
 * What opens the picker: an `@` at a word boundary, optionally followed by the
 * start of a name. Spaces are allowed inside the partial so "@Ada Lo" still
 * finds Ada Lovelace; the 30-char bound stops a whole paragraph after a stray
 * `@` from being treated as a search.
 */
const triggerPattern = /(^|\s)@([\w][\w ]{0,30})?$/

/**
 * Split a comment into the parts to render.
 *
 * Text between tokens is returned verbatim, including its whitespace — the
 * discussion renders `pre-wrap`, so the parts must not be trimmed or rejoined.
 * A mention part carries the name snapshot the author saw (without the `@`)
 * and the account id, so a caller can style the viewer's own mention.
 *
 * @param {string} text
 * @returns {Array<{type: "text"|"mention", text: string, userId?: string}>}
 */
export const splitMentions = (text) => {
  const source = text ?? ""
  if (!source) return []

  const parts = []
  const pattern = mentionPattern()
  let cursor = 0
  let match

  while ((match = pattern.exec(source)) !== null) {
    if (match.index > cursor) {
      parts.push({ type: "text", text: source.slice(cursor, match.index) })
    }
    parts.push({ type: "mention", text: match[1], userId: match[2] })
    cursor = match.index + match[0].length
  }

  if (cursor < source.length) {
    parts.push({ type: "text", text: source.slice(cursor) })
  }

  return parts
}

/**
 * Rewrite tokens back to what the author typed: `@[Ada Lovelace](66f…)` becomes
 * `@Ada Lovelace`.
 *
 * The inverse of the token format, mirroring `flattenMentions` on the server.
 * Anywhere a comment appears as plain text rather than rendered parts — the
 * thread-title previews — goes through this, or the reader sees the raw markup.
 */
export const flattenMentions = (text) => (text ?? "").replace(mentionPattern(), "@$1")

/**
 * Find the mention being typed immediately before the caret.
 *
 * Takes the text before the caret rather than the whole field so the caller
 * decides what "before" means (the DOM's selectionStart, which is authoritative
 * mid-edit, rather than the prop, which lags a keystroke).
 *
 * @param {string} textBeforeCaret
 * @returns {{start: number, query: string}|null} `start` is the index of the `@`
 */
export const mentionTrigger = (textBeforeCaret) => {
  const match = triggerPattern.exec(textBeforeCaret ?? "")
  if (!match) return null
  return {
    start: match.index + match[1].length,
    query: match[2] ?? "",
  }
}

/**
 * The candidates to offer for a partially-typed name.
 *
 * Prefix matches come first — typing "ad" means you are far more likely to want
 * Ada than Richard — and within each half the server's alphabetical order is
 * kept, so the list doesn't reshuffle as you type. Anyone with no display name
 * at all is dropped: there is nothing to put inside the token.
 *
 * @param {Array<Object>} candidates users from GET /events/:id/mentionables
 * @param {string} query the partial name, without the `@`
 * @param {number} limit
 */
export const filterMentionables = (candidates, query, limit = maxMentionCandidates) => {
  const needle = (query ?? "").trim().toLowerCase()

  const prefix = []
  const contains = []
  for (const candidate of candidates ?? []) {
    const name = displayName(candidate).toLowerCase()
    if (!name) continue
    if (!needle) {
      prefix.push(candidate)
    } else if (name.startsWith(needle)) {
      prefix.push(candidate)
    } else if (name.includes(needle)) {
      contains.push(candidate)
    }
  }

  return [...prefix, ...contains].slice(0, limit)
}

/**
 * The name as it goes inside a token: no `]` or newline (either would let the
 * token swallow the rest of the comment, so the server's pattern refuses them),
 * and no longer than the pattern allows.
 */
const tokenName = (user) =>
  displayName(user).replace(/[\]\r\n]/g, " ").replace(/\s+/g, " ").trim().slice(0, maxTokenNameLength)

/**
 * Replace the partial mention before the caret with a finished token.
 *
 * Returns null when there is nothing to replace or the candidate can't be
 * written as a token — a nameless account, or an id that isn't an ObjectID.
 * Writing one anyway would produce text the server reads as prose, so the
 * mention would render as literal markup and notify nobody.
 *
 * The trailing space is deliberate: it closes the trigger, so the picker
 * doesn't immediately reopen on the name just inserted.
 *
 * @param {string} text the full field contents
 * @param {number} caret selectionStart
 * @param {Object} user the chosen candidate
 * @returns {{text: string, caret: number}|null}
 */
export const applyMention = (text, caret, user) => {
  const source = text ?? ""
  const at = Math.max(0, Math.min(caret ?? source.length, source.length))

  const trigger = mentionTrigger(source.slice(0, at))
  if (!trigger) return null

  const name = tokenName(user)
  const id = user?._id ?? ""
  if (!name || !/^[0-9a-f]{24}$/.test(id)) return null

  const token = `@[${name}](${id}) `
  return {
    text: source.slice(0, trigger.start) + token + source.slice(at),
    caret: trigger.start + token.length,
  }
}
