/* eslint-env serviceworker */
/*
  The Fellowship's service worker.

  IT PRECACHES THE APP SHELL AND NOTHING ELSE. It does not touch /api, it holds
  no data, and it makes no decisions about what a member may see. Every byte of
  gathering data lives in IndexedDB under `src/utils/offline/`, keyed by user
  and wiped on sign-out — a worker cache is far harder to scope and to clear,
  and this is an invite-only club.

  That narrowness is also the safety argument. Reintroducing a worker here
  reverses a deliberate removal, and the recorded reason to be careful is
  specific: because the Go server serves index.html for any unmatched path,
  `GET /service-worker.js` returning HTML would make a broken worker fail its
  update check ON MIME TYPE and pin that client to a dead build forever. Three
  things guard against that, and none of them is optional:

    1. server/main.go registers this file explicitly, with a JavaScript
       content type and `Cache-Control: no-cache`, so it is always refetchable.
    2. `deploy/kill-service-worker.js` exists and has been rehearsed. It is
       served AT THIS URL, because that is the only URL a stale worker ever
       asks for again.
    3. No skipWaiting on install — see below.
*/
import { precacheAndRoute } from "workbox-precaching"
import { NavigationRoute, registerRoute } from "workbox-routing"

/*
  The build's hashed JS, CSS and font assets. index.html is deliberately NOT in
  here — see SHELL below — and source maps are excluded as dead weight on a
  phone. Both exclusions are configured in vue.config.js.
*/
precacheAndRoute(self.__WB_MANIFEST || [])

/*
  The shell is fetched, not precached from disk.

  `frontend/dist/index.html` is a Go html/template: it still contains
  `{{ or .title $defaultTitle }}` and friends, which the server fills in per
  request. Precaching the built FILE would cache the template source and render
  its braces at people. So the shell is taken from the server's own rendered
  output at `/`, on install and again on activate, which also means a deploy's
  new shell arrives with the new worker rather than a version behind it.

  `cache: "reload"` because the server sends the shell `no-store`; this must
  come from the network, not from whatever the HTTP cache thinks.
*/
const SHELL_CACHE = "fellowship-shell-v1"
const SHELL_URL = "/"

const cacheShell = async () => {
  try {
    const cache = await caches.open(SHELL_CACHE)
    await cache.add(new Request(SHELL_URL, { cache: "reload" }))
  } catch {
    // Offline at install time, or the server refused. The worker is still
    // useful for the precached assets; the shell is retried on activate.
  }
}

self.addEventListener("install", (event) => {
  // NO skipWaiting. A deploy deletes the previous build's hashed chunks, so a
  // worker that took over mid-session could leave the running tab asking for a
  // chunk the new precache does not name. The new worker waits; the app offers
  // a reload (see utils/offline/sw.js), and the member chooses when.
  event.waitUntil(cacheShell())
})

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      await cacheShell()
      // Drop shells from earlier cache versions so an old one can't be served.
      const names = await caches.keys()
      await Promise.all(
        names
          .filter((name) => name.startsWith("fellowship-shell-") && name !== SHELL_CACHE)
          .map((name) => caches.delete(name))
      )
      await self.clients.claim()
    })()
  )
})

// The app's "a new version is ready — reload" prompt calls this. Skipping the
// wait is fine HERE because the page is about to reload anyway.
self.addEventListener("message", (event) => {
  if (event.data?.type === "SKIP_WAITING") self.skipWaiting()
})

/*
  Navigations get the shell; the SPA router takes it from there.

  The denylist is load-bearing rather than tidy. `noRouteHandler` has no path
  guard, so `GET /api/anything-unmatched` answers 200 with the HTML shell — a
  worker that served navigations for /api paths would be caching and returning
  HTML under API URLs. API requests are not navigations in the normal case, but
  the cost of being wrong here is high and the cost of the denylist is nothing.
*/
const shellHandler = async () => {
  const cached = await caches.match(SHELL_URL, { cacheName: SHELL_CACHE })
  if (cached) return cached
  return fetch(SHELL_URL)
}

registerRoute(new NavigationRoute(shellHandler, { denylist: [/^\/api\//] }))
