/*
  Registering the service worker, and handling the fact that a new one waits.

  Production only. In development the app is served by webpack-dev-server with
  hot reload, and a worker holding a precached shell in front of that is a
  debugging nightmare for no benefit — the same reason @vue/cli-plugin-pwa only
  ever registered in production.
*/

/** The URL is fixed; see the note in src/service-worker.js. */
const SW_URL = "/service-worker.js"

let waitingWorker = null

const supported = () =>
  typeof navigator !== "undefined" && "serviceWorker" in navigator

/**
 * @param {object} handlers
 * @param {Function} [handlers.onUpdateReady] called when a new version has
 *   installed and is waiting for the member to accept it
 */
export const registerServiceWorker = ({ onUpdateReady } = {}) => {
  if (process.env.NODE_ENV !== "production" || !supported()) return

  window.addEventListener("load", async () => {
    try {
      const registration = await navigator.serviceWorker.register(SW_URL)

      registration.addEventListener("updatefound", () => {
        const installing = registration.installing
        if (!installing) return
        installing.addEventListener("statechange", () => {
          // `controller` being set is what distinguishes an UPDATE from a
          // first install. On a first install there is nothing to interrupt
          // and nothing to tell anyone about.
          if (
            installing.state === "installed" &&
            navigator.serviceWorker.controller
          ) {
            waitingWorker = installing
            onUpdateReady?.()
          }
        })
      })
    } catch {
      // A worker that won't register costs offline support and nothing else.
      // The app is fully functional without one.
    }
  })
}

/**
 * Accepts a waiting update: tells it to take over, then reloads onto it.
 *
 * The reload is driven by `controllerchange` rather than called straight after
 * `postMessage`, because skipWaiting is asynchronous — reloading too early
 * lands back on the OLD worker and the update appears not to have happened.
 * The guard stops the reload loop that a second controllerchange would cause.
 */
export const applyServiceWorkerUpdate = () => {
  if (!waitingWorker) return
  let reloading = false
  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (reloading) return
    reloading = true
    window.location.reload()
  })
  waitingWorker.postMessage({ type: "SKIP_WAITING" })
  waitingWorker = null
}
