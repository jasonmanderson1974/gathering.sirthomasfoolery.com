/*
  "Was this member signed in last time we could ask?"

  The session cookie is HttpOnly (server/main.go sets it that way), so JS cannot
  read it or its expiry. Offline there is therefore no way to *verify* a
  session — the only honest question is whether one plausibly still exists.

  This record answers that. It is written whenever the server confirms a
  session, and read by the router guard when an offline boot means the server
  cannot be asked. It is a hint for rendering cached data, never an
  authorization decision: every actual read and write still goes to the server,
  which enforces the real thing.
*/

import { clearCache } from "./cache"
import { clearQueue } from "./queue"

const KEY = "offline-session"

// Matches the cookie's MaxAge in server/main.go (30 days). If the two drift,
// the failure is benign in the safe direction — the guard offers cached data
// for a session the server would reject, the first reconnect 401s, and the
// session-ended handler clears everything and redirects to sign-in.
const MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000

/**
 * Records that the server confirmed a session for this user, just now.
 *
 * localStorage access is wrapped because it throws outright in some privacy
 * modes — the same defence Dashboard.vue's folder state uses.
 */
export const rememberSession = (userId) => {
  if (!userId) return
  try {
    localStorage.setItem(
      KEY,
      JSON.stringify({ userId, lastVerifiedAt: Date.now() })
    )
  } catch {
    // No persistence available. The app works, it just can't boot offline.
  }
}

/**
 * The remembered session, or null if there isn't a usable one.
 *
 * A record older than the cookie's own lifetime is not merely ignored, it is
 * deleted: it cannot correspond to a live session, so keeping it around only
 * leaves a user id on disk for no purpose.
 */
export const recallSession = () => {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed?.userId || typeof parsed.lastVerifiedAt !== "number") {
      localStorage.removeItem(KEY)
      return null
    }
    const age = Date.now() - parsed.lastVerifiedAt
    // A negative age means the clock moved backwards, or the record was
    // tampered with. Either way it isn't evidence of anything.
    if (age < 0 || age > MAX_AGE_MS) {
      localStorage.removeItem(KEY)
      return null
    }
    return parsed
  } catch {
    // Unparseable — clear it rather than failing this way on every boot.
    try {
      localStorage.removeItem(KEY)
    } catch {
      /* nothing further to try */
    }
    return null
  }
}

/** Sign-out, or a session the server has rejected. */
export const forgetSession = () => {
  try {
    localStorage.removeItem(KEY)
  } catch {
    /* nothing to clear */
  }
}

/**
 * Tears down everything this device remembers about the signed-in member.
 *
 * The two halves are paired here so they cannot drift apart — leaving the
 * cache behind while forgetting the session would strand a readable copy of
 * someone's gatherings, discussion and ledger on the device.
 *
 * Call this ONLY where the session is known to be over: an explicit sign-out,
 * or the session-ended 401 handler. Notably NOT from the several places that
 * commit `setAuthUser(null)` in a failure catch — those fire on a transient
 * 500 or an offline boot, where wiping the cache would destroy the data the
 * member is about to need.
 */
export const endOfflineSession = () => {
  forgetSession()
  // The queue goes too. It holds one member's unsent work, addressed to a
  // session that no longer exists — flushing it later would either fail or,
  // worse, post it as whoever signs in next.
  return Promise.all([clearCache(), clearQueue()])
}
