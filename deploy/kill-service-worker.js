/* eslint-env serviceworker */
/*
  THE KILL SWITCH. Read this before you need it.

  This file is not served. It is the replacement to drop in place of
  `frontend/dist/service-worker.js` when the real worker has to be withdrawn —
  a bad precache, a build that pins clients to dead chunks, or a decision to
  stop shipping a worker at all. Deploying it unregisters the worker from every
  client that checks in, empties every cache it made, and reloads the page onto
  the plain, workerless app.

  WHY IT LIVES AT THAT EXACT FILENAME. A stale worker only ever refetches the
  URL it was REGISTERED under, which is `/service-worker.js`. A kill switch at
  any other path — `/kill-sw.js`, say — is a URL nothing ever asks for; it
  would look like a fix and change nothing. That mistake is already recorded in
  this repo's history: the inherited `kill-sw.js` was deleted rather than moved,
  precisely because moving it would have produced a served file that did
  nothing while looking handled.

  AND WHY DELETING THE WORKER IS NOT ENOUGH. `GET /service-worker.js` against a
  server with no such file does not 404 — the SPA fallback answers it with
  index.html as `text/html`. The worker's update check fails on the MIME type
  rather than on a missing file, so the browser keeps the worker it has, and
  the client stays pinned indefinitely. Removing the file makes the problem
  permanent. Replacing it with this fixes it.

  TO USE IT:

      cp deploy/kill-service-worker.js frontend/dist/service-worker.js
      ./deploy.sh

  Then leave it in place for at least a few weeks. Clients only find it when
  they next check for an update, and someone who has not opened the app since
  the bad deploy is still carrying the bad worker until they do.

  REHEARSE IT BEFORE YOU SHIP A WORKER, not after something has gone wrong.
*/

self.addEventListener("install", () => {
  // Take over immediately. This is the one worker that SHOULD skip waiting —
  // the ordinary worker deliberately does not, but here the whole purpose is
  // to displace a worker that may be actively breaking the app.
  self.skipWaiting()
})

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      // Every cache, not just the ones this app named: whatever the withdrawn
      // worker created has to go, and by definition we may not know what it
      // called them.
      const names = await caches.keys()
      await Promise.all(names.map((name) => caches.delete(name)))

      await self.registration.unregister()

      // Reload each controlled page so it comes back with no worker at all,
      // rather than sitting on whatever the dead one last served it.
      const clients = await self.clients.matchAll({ type: "window" })
      for (const client of clients) {
        try {
          client.navigate(client.url)
        } catch {
          // Cross-origin or otherwise not navigable. Unregistering has already
          // happened, so the next ordinary load is clean regardless.
        }
      }
    })()
  )
})

// No fetch handler, deliberately. Every request goes straight to the network
// while this is briefly in charge.
