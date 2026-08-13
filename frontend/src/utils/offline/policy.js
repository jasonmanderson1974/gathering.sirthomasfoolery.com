/*
  Offline cache policy — which API reads may be persisted, and under what key.

  Kept separate from cache.js, which owns the IndexedDB plumbing, so all of the
  decision-making here is pure and runs in the fast `node` test tier.
*/

// An event id is either a Mongo ObjectID hex or a short id; both are drawn from
// this alphabet. Kept deliberately narrow so a route carrying a query string or
// an extra segment cannot match one of the patterns below by accident.
const ID = "[A-Za-z0-9_-]+"

/*
  What may be cached is an ALLOWLIST, not "every GET".

  Two reasons, and the second is the one that matters. The obvious one is size.
  The real one is that this is an invite-only club: every entry here is member
  data written to disk on a phone, so the set wants to be short enough that a
  reader can audit it at a glance and see exactly what leaves memory.

  Deliberately absent:
    /user/calendars, /events/:id/calendar-availabilities
        Keyed by a time range, so the key space is unbounded and the payload is
        the member's real calendar — far more than reading a gathering needs.
    /auth/status
        A liveness probe. A cached answer is worse than no answer.
    /users/:id
        The public profile behind a guest's hover card. Members come from
        /admin/allowlist, which is cached; a guest's card is the one hover that
        degrades offline, which is a fair trade for not persisting a row per
        person ever hovered.
*/
const CACHEABLE = [
  /^\/user\/profile$/,
  /^\/user\/events$/,
  /^\/user\/folders$/,
  new RegExp(`^/events/${ID}$`),
  new RegExp(`^/events/${ID}/lists$`),
  // The pool for the assignee picker. Cached because assigning DOES work
  // offline now: without it the picker offers nothing but "Unassigned", which
  // reads as though every member had vanished rather than as a feature being
  // unavailable. Reported from a real phone.
  new RegExp(`^/events/${ID}/lists/assignees$`),
  new RegExp(`^/events/${ID}/expenses$`),
  new RegExp(`^/events/${ID}/expenses/participants$`),
  new RegExp(`^/events/${ID}/my-lists$`),
  new RegExp(`^/events/${ID}/my-notes$`),
  new RegExp(`^/events/${ID}/mentionables$`),
  /^\/admin\/allowlist$/,
]

/**
 * Whether a GET route's response may be persisted.
 *
 * The patterns are fully anchored, so a query string means no match. That is
 * the intended default rather than an oversight: none of the routes above is
 * called with one today (checked at the call sites), and a route that grows a
 * parameter should have to come back through here rather than silently
 * caching one arbitrary variant of itself under a shared key.
 */
export const isCacheable = (route) =>
  typeof route === "string" && CACHEABLE.some((re) => re.test(route))

// The user a cached entry belongs to is part of its key, never an assumption.
// GET /events/:id is caller-dependent — the server strips emails for
// non-owners, nils Remindees, filters members-only threads from guests,
// privatizes blind availability by session, and computes CanEdit/
// CanManageThread per requester. One shared cache across two accounts on the
// same phone would hand one member the other's view.
export const cacheKey = (userId, route) => `${userId || ANON_USER}|${route}`

// Reads made before /user/profile has answered — the boot ordering — land here.
// setCacheUser adopts them once the real id is known; see cache.js.
export const ANON_USER = "anon"

/** The user portion of a key, for deciding what to purge. */
export const keyUser = (key) => String(key).split("|", 1)[0]
