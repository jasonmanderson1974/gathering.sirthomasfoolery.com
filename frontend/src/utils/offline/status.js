/*
  Is the app reachable?

  `navigator.onLine` alone is not the answer and never was: it reports whether
  the device has *a* network interface up, not whether our server is reachable.
  A phone parked on a hotel wifi with no route out reports `true`, which is
  precisely the case an offline mode has to survive.

  So the authoritative signal is the outcome of real requests — fetch_utils
  reports each one here — and `navigator.onLine` is used only as a fast hint,
  because a device that says it is offline certainly is.
*/

const listeners = new Set()

// Set when a request fails at the transport layer, cleared the moment one
// succeeds. This is what makes a dead captive portal read as offline.
let sawNetworkFailure = false

const navigatorOffline = () =>
  typeof navigator !== "undefined" && navigator.onLine === false

export const isOffline = () => navigatorOffline() || sawNetworkFailure

const notify = () => {
  const offline = isOffline()
  for (const fn of listeners) {
    try {
      fn(offline)
    } catch {
      // A subscriber's own failure is not this module's to propagate; the rest
      // of the subscribers still need telling.
    }
  }
}

const set = (failed) => {
  if (sawNetworkFailure === failed) return
  sawNetworkFailure = failed
  notify()
}

/** Reported by fetch_utils when a request never reached the server. */
export const noteNetworkFailure = () => set(true)

/** Reported by fetch_utils when one does reach it — the all-clear. */
export const noteNetworkSuccess = () => set(false)

/**
 * Subscribes to changes. Returns an unsubscribe function.
 *
 * The browser's own events are a hint in both directions: `offline` is
 * trustworthy, but `online` only means an interface came up, so it clears the
 * failure flag optimistically and the next real request settles it either way.
 */
export const onOfflineChange = (fn) => {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

let started = false

export const startOfflineWatch = () => {
  if (started || typeof window === "undefined") return
  started = true
  window.addEventListener("offline", notify)
  window.addEventListener("online", () => set(false))
}

/** Test seam. */
export const __resetStatusForTests = () => {
  listeners.clear()
  sawNetworkFailure = false
  started = false
}
