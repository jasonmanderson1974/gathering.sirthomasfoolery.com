/*
  How long ago a cached payload was fetched, in words.

  Hand-rolled rather than reaching for dayjs's relativeTime plugin: this needs
  four coarse buckets, and date_utils.js is deliberately the only place dayjs is
  configured (it is 946 lines with a documented reason not to grow). A pure
  function is also testable in the fast node tier, which is where it belongs.
*/

const MINUTE = 60 * 1000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

const plural = (n, unit) => `${n} ${unit}${n === 1 ? "" : "s"} ago`

/**
 * @param {number|null} fetchedAt epoch ms, or null if the value is live
 * @param {number} [now] injectable for tests
 * @returns {string|null} null when there is nothing to say
 */
export const formatAge = (fetchedAt, now = Date.now()) => {
  if (typeof fetchedAt !== "number" || !Number.isFinite(fetchedAt)) return null

  const ms = now - fetchedAt
  // A clock that moved, or a stamp from this same millisecond. Either way
  // "just now" is the honest answer; a negative duration is not.
  if (ms < MINUTE) return "just now"
  if (ms < HOUR) return plural(Math.floor(ms / MINUTE), "minute")
  if (ms < DAY) return plural(Math.floor(ms / HOUR), "hour")
  return plural(Math.floor(ms / DAY), "day")
}
