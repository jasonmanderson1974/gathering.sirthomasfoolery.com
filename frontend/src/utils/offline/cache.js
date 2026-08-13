/*
  Offline read cache — persists allowlisted API reads so a member with no
  connection can still open the app and read their gatherings.

  Two layers, on purpose. An in-memory Map is the working copy; IndexedDB is
  what survives the tab being evicted, which on a phone is most of the time.
  The Map also means every path through this module still behaves where
  IndexedDB does not exist (the happy-dom test tier, Safari private browsing),
  rather than silently doing nothing there.

  NOTHING HERE MAY THROW. A cache is an optimization; if it breaks, the app
  must carry on exactly as it did before this file existed. Every entry point
  swallows its own failures.
*/

import { ANON_USER, cacheKey, isCacheable, keyUser } from "./policy"

const DB_NAME = "timeful-offline"
// v2 added the write queue's store (O5). Bump this whenever a store is added:
// onupgradeneeded is the only place a store can be created, and it only runs
// when the version changes.
const DB_VERSION = 2
const STORE = "apiCache"
export const QUEUE_STORE = "writeQueue"

// The working copy. Key -> { body, fetchedAt }.
const memory = new Map()

let currentUserId = ANON_USER

/*
  The same gathering is reachable — and therefore cached — under TWO ids.

  Event ids may be either the Mongo _id or the short id (`db.GetEventByEitherId`
  on the server), and the app uses both: the router param is whichever was in
  the link, while Event.vue's write handlers address the gathering by
  `shortId ?? _id`. So `GET /events/<shortId>` and `GET /events/<mongoId>` are
  two cache entries holding one gathering.

  That is harmless for reads and NOT harmless for an offline write: a reducer
  that edits only the spelling the caller happened to use leaves the page
  showing no change at all, because the page was read under the other one. The
  aliases are learned from the payload itself, which carries both.
*/
const aliases = new Map()

const learnAliases = (body) => {
  const ids = [body?._id, body?.shortId].filter(
    (id) => typeof id === "string" && id
  )
  if (ids.length < 2) return
  for (const id of ids) {
    const set = aliases.get(id) ?? new Set()
    for (const other of ids) set.add(other)
    aliases.set(id, set)
  }
}

/** Every id this gathering is also known by, itself included. */
export const eventAliases = (eventId) => {
  const set = aliases.get(eventId)
  return set ? [...set] : [eventId]
}

/*
  Entries are copied in and copied out, and that is not paranoia.

  Views own their payloads and mutate them: Event.vue assigns the response to
  `this.event` and `processEvent` then derives fields on it in place. Handing
  out the stored object would make Vue's reactive proxy and the cache the same
  object, so every later edit on screen would silently rewrite the saved copy —
  and two views reading the same route would share one mutable object between
  them. Neither failure announces itself.
*/
const clone = (value) => {
  try {
    return structuredClone(value)
  } catch {
    // Non-cloneable, or an environment without it. A shared reference is worse
    // than a slow copy, so try JSON before giving up on isolation.
    try {
      return JSON.parse(JSON.stringify(value))
    } catch {
      return value
    }
  }
}

/* ------------------------------------------------------------------ *
 * IndexedDB plumbing
 * ------------------------------------------------------------------ */

let dbPromise = null

// Resolves to an IDBDatabase, or to null if IndexedDB is unavailable or
// refuses to open. Memoized: the failure is cached too, so a browser that
// can't do this doesn't get asked again on every read.
const openDb = () => {
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve) => {
    try {
      if (typeof indexedDB === "undefined") return resolve(null)
      const req = indexedDB.open(DB_NAME, DB_VERSION)
      req.onupgradeneeded = () => {
        const db = req.result
        if (!db.objectStoreNames.contains(STORE)) db.createObjectStore(STORE)
        if (!db.objectStoreNames.contains(QUEUE_STORE)) {
          // Keyed by the op's own id, which is ordered: the queue must flush in
          // the order it was filled, because a later op can target something an
          // earlier one created.
          db.createObjectStore(QUEUE_STORE, { keyPath: "id" })
        }
      }
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => resolve(null)
      // Another tab holding an old version open. Don't hang the caller.
      req.onblocked = () => resolve(null)
    } catch {
      resolve(null)
    }
  })
  return dbPromise
}

// Wraps one store operation. `run` receives the object store and returns an
// IDBRequest; the promise resolves with its result, or with `fallback` on any
// failure — including a quota error, which is the expected way for a write to
// fail on a phone and must never surface to the caller.
const withStore = async (mode, run, fallback = null) => {
  const db = await openDb()
  if (!db) return fallback
  return new Promise((resolve) => {
    try {
      const tx = db.transaction(STORE, mode)
      const req = run(tx.objectStore(STORE))
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => resolve(fallback)
      tx.onabort = () => resolve(fallback)
      tx.onerror = () => resolve(fallback)
    } catch {
      resolve(fallback)
    }
  })
}

/* ------------------------------------------------------------------ *
 * Public API
 * ------------------------------------------------------------------ */

/**
 * The cached response for a route, or null.
 *
 * Returns `{ body, fetchedAt }` — fetchedAt is what the UI renders as
 * "last updated", so a member can always tell how stale what they're reading is.
 */
export const readCache = async (route) => {
  if (!isCacheable(route)) return null
  const key = cacheKey(currentUserId, route)
  const hit = memory.get(key)
  if (hit) return { body: clone(hit.body), fetchedAt: hit.fetchedAt }
  const stored = await withStore(
    "readonly",
    (store) => store.get(key),
    undefined
  )
  if (!stored) return null
  memory.set(key, stored)
  return { body: clone(stored.body), fetchedAt: stored.fetchedAt }
}

/**
 * Persists a response. Non-allowlisted routes are dropped silently, which is
 * what makes it safe to call this unconditionally from the fetch choke point.
 */
export const writeCache = async (route, body) => {
  if (!isCacheable(route)) return
  // `undefined` is not structured-cloneable and an empty-body response comes
  // back as "". Neither is worth a cache entry, and storing one would shadow
  // a good entry already there.
  if (body === undefined || body === "") return
  const key = cacheKey(currentUserId, route)
  // An event payload names both of its ids; remember the pairing so a later
  // write can find every spelling of this gathering.
  if (/^\/events\/[^/]+$/.test(route)) learnAliases(body)
  // Copied in as well as out: the caller is about to hand this same object to a
  // view, which will mutate it.
  const entry = { body: clone(body), fetchedAt: Date.now() }
  memory.set(key, entry)
  await withStore("readwrite", (store) => store.put(entry, key))
}

/**
 * Declares whose cache this is, once /user/profile has answered.
 *
 * Does two things, both of which are about correctness rather than tidiness:
 *
 *  - ADOPTS anonymous entries. The boot ordering means the profile read itself
 *    happens before anyone knows the id, so it lands under the `anon` sentinel.
 *    Re-keying rather than discarding is what lets a cold offline boot find the
 *    profile it cached on its last online boot.
 *  - PURGES every other concrete user's entries. Two accounts on one phone must
 *    never read each other's gatherings, and the payloads genuinely differ per
 *    caller (see policy.js). This also bounds growth.
 */
export const setCacheUser = async (userId) => {
  const next = userId || ANON_USER
  if (next === currentUserId) return
  currentUserId = next
  if (next === ANON_USER) return

  const adopt = (key) => `${next}|${String(key).split("|").slice(1).join("|")}`

  for (const [key, entry] of [...memory]) {
    const owner = keyUser(key)
    if (owner === next) continue
    memory.delete(key)
    if (owner === ANON_USER) memory.set(adopt(key), entry)
  }

  const keys = await withStore("readonly", (store) => store.getAllKeys(), [])
  await Promise.all(
    (keys || []).map(async (key) => {
      const owner = keyUser(key)
      if (owner === next) return
      if (owner === ANON_USER) {
        const entry = await withStore("readonly", (store) => store.get(key))
        if (entry) await withStore("readwrite", (s) => s.put(entry, adopt(key)))
      }
      await withStore("readwrite", (store) => store.delete(key))
    })
  )
}

/**
 * Drops everything. Called on sign-out and on a session-ended 401 — a member
 * who has signed out must not leave their gatherings, discussion and ledger
 * readable on the device.
 */
export const clearCache = async () => {
  memory.clear()
  currentUserId = ANON_USER
  await withStore("readwrite", (store) => store.clear())
}

/*
  Staleness marker.

  A cached response is returned to the caller AS THE RESPONSE — that is the
  whole point, since it means no call site has to know it is reading from
  cache. But a view that wants to say "last updated an hour ago" needs to be
  able to tell, so the age rides along on the value itself.

  Non-enumerable and keyed by a Symbol, deliberately: it must survive being
  handed around while staying invisible to JSON.stringify, Object.keys, v-for
  and anything else that walks the payload. A plain `__stale` property would
  eventually be serialized back to the server by something.
*/
const STALE_AT = Symbol("staleAt")

const markStale = (body, fetchedAt) => {
  if (body === null || typeof body !== "object") return body
  try {
    Object.defineProperty(body, STALE_AT, {
      value: fetchedAt,
      enumerable: false,
      configurable: true,
    })
  } catch {
    // Frozen or sealed. The data is still good; it just can't carry its age.
  }
  return body
}

/**
 * When a value was fetched, if it came from cache rather than the network.
 * Null means it is live.
 */
export const staleAt = (value) =>
  value !== null && typeof value === "object" ? (value[STALE_AT] ?? null) : null

/**
 * The cached response for a route, marked with its age, or null on a miss.
 * This is the form fetch_utils hands back when the network is unreachable.
 */
export const readCacheAsResponse = async (route) => {
  const hit = await readCache(route)
  return hit ? markStale(hit.body, hit.fetchedAt) : null
}

/**
 * Runs one operation against an arbitrary store, with the same
 * never-throw contract as everything else here. Used by the write queue, which
 * needs its own store but not its own database connection.
 */
export const withNamedStore = async (storeName, mode, run, fallback = null) => {
  const db = await openDb()
  if (!db) return fallback
  return new Promise((resolve) => {
    try {
      const tx = db.transaction(storeName, mode)
      const req = run(tx.objectStore(storeName))
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => resolve(fallback)
      tx.onabort = () => resolve(fallback)
      tx.onerror = () => resolve(fallback)
    } catch {
      resolve(fallback)
    }
  })
}

/**
 * Reads a cached body, applies `mutate` to it, and writes it back.
 *
 * This is how an offline write becomes visible: the queued op edits the cached
 * payload the same way the server would have, so the view's existing
 * persist-then-refetch reads the change straight back out. Returns whether
 * anything was there to change — a miss is normal (nobody has opened that tab).
 */
export const mutateCached = async (route, mutate) => {
  const hit = await readCache(route)
  if (!hit) return false
  const next = mutate(hit.body)
  if (next === undefined) return false
  // Preserves fetchedAt: a local edit does not make the copy any fresher, and
  // claiming it did would hide real staleness behind the member's own typing.
  const key = cacheKey(currentUserId, route)
  memory.set(key, { body: clone(next), fetchedAt: hit.fetchedAt })
  await withStore("readwrite", (store) =>
    store.put({ body: clone(next), fetchedAt: hit.fetchedAt }, key)
  )
  return true
}

/** Test seam. Resets module state without touching the database. */
export const __resetCacheForTests = () => {
  memory.clear()
  aliases.clear()
  currentUserId = ANON_USER
  dbPromise = null
}
