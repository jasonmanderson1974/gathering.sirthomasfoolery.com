import { setCacheUser } from "./cache"
import { recallSession } from "./session"
import { onOfflineChange, startOfflineWatch } from "./status"
import { flush } from "./queue"

/**
 * Brings the offline layer up. Called from main.js before the app mounts.
 *
 * The ordering here is load-bearing. On a cold boot the router guard's first
 * act is to read GET /user/profile, and offline that read can only be answered
 * from cache — but cache entries are keyed by user, and the user is precisely
 * what that read was going to establish. Restoring the remembered id first is
 * what closes the circle.
 *
 * `setCacheUser` is async but assigns the current user synchronously before it
 * awaits anything, so the very first read already looks under the right key
 * without this having to be awaited. That is deliberate: mounting the app must
 * not wait on IndexedDB.
 */
export const initOffline = ({ onFlushed } = {}) => {
  startOfflineWatch()
  const remembered = recallSession()
  if (remembered) setCacheUser(remembered.userId)

  // Send whatever was written while the connection was gone, the moment it
  // comes back — and once on startup, since the app may well have been closed
  // between filling the queue and the signal returning.
  const drain = async () => {
    const result = await flush()
    if (result.sent > 0 || result.dropped.length > 0) onFlushed?.(result)
  }
  onOfflineChange((offline) => {
    if (!offline) drain()
  })
  drain()
}
