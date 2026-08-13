import { serverURL, errors } from "@/constants"
import { readCacheAsResponse, writeCache } from "@/utils/offline/cache"
import { noteNetworkFailure, noteNetworkSuccess } from "@/utils/offline/status"

/* 
  Fetch utils
*/

// Session-expiry handler, registered by the router (which owns navigation and
// the store). Kept as a registration hook rather than an import so this module
// has no dependency on either — importing them here would create a cycle
// (fetch_utils -> router -> store -> fetch_utils).
let unauthorizedHandler = null

/**
 * Registers the callback invoked when the API reports the session is no longer
 * usable. See router/index.js for the installed handler.
 */
export const setUnauthorizedHandler = (fn) => {
  unauthorizedHandler = fn
}

// A 401 carrying one of these means the session is gone — not signed in, the
// account was deleted, or the member was struck from the invite roll
// mid-session. Anything else (e.g. a 403) is the call site's to handle.
const sessionEndedCodes = [
  errors.NotSignedIn,
  errors.UserDoesNotExist,
  errors.NotInvited,
]

// The auth probe is how the app *asks* whether there is a session, so a 401
// from it is the expected answer for a signed-out visitor — not a session
// ending mid-flight. The router guard calls it and handles the failure itself.
//
// Routing it through the handler as well is what broke the signed-out site: the
// guard's own probe fired the handler, which pushed to /sign-in while
// beforeEach was still resolving, cancelling the navigation and re-running the
// guard — an endless / -> /sign-in loop that never rendered either page.
const authProbeRoutes = ["/user/profile"]
const isAuthProbe = (route) =>
  authProbeRoutes.some((r) => route === r || route.startsWith(r + "?"))

// `fetch` rejects — rather than resolving with a bad status — when the request
// never reached the server at all: offline, DNS failure, a dropped connection,
// CORS. That arrives as a bare `TypeError: Failed to fetch` carrying no
// `.status` and no `.error`, so every call site doing `switch (err.error)` fell
// straight through to `default` with no way to say what happened, and the
// router guard could not tell it from a 401 — which is what bounced a signed-in
// member to /sign-in the moment their phone lost signal.
//
// Normalizing it here, at the one function every API call in the app passes
// through, is what lets everything downstream simply ask `err.offline`.
const networkError = (cause) => {
  const err = new Error(
    `Network request failed - ${cause?.message ?? String(cause)}`
  )
  err.offline = true
  err.error = errors.Offline
  err.cause = cause
  return err
}
export const get = (route) => {
  return fetchMethod("GET", route)
}

export const post = (route, body = {}) => {
  return fetchMethod("POST", route, body)
}

export const patch = (route, body = {}) => {
  return fetchMethod("PATCH", route, body)
}

export const put = (route, body = {}) => {
  return fetchMethod("PUT", route, body)
}

export const _delete = (route, body = {}) => {
  return fetchMethod("DELETE", route, body)
}

export const fetchMethod = (method, route, body = {}) => {
  /* Calls the given route with the give method and body */
  const url = serverURL + route
  const params = {
    method,
    credentials: "include",
  }

  if (method !== "GET") {
    // Add params specific to POST/PATCH/DELETE
    params.headers = {
      "Content-Type": "application/json",
    }
    params.body = JSON.stringify(body)
  }

  const request = fetch(url, params)
    // Chained onto fetch itself, deliberately: this must catch the transport
    // failure only. An error thrown by the response handler below (a non-2xx,
    // a JSON parse failure) is a real answer from the server and must not be
    // relabelled as offline.
    .catch((cause) => {
      noteNetworkFailure()
      throw networkError(cause)
    })
    .then(async (res) => {
      // We got an answer. Any answer, including a 500, proves the server is
      // reachable — which is a stronger signal than navigator.onLine, and the
      // one that clears a captive portal's false "online".
      noteNetworkSuccess()
      const text = await res.text()

      // Parse JSON if text is not empty
      let returnValue
      if (text.length === 0) {
        returnValue = text
      } else {
        try {
          returnValue = JSON.parse(text)
        } catch (err) {
          throw { error: errors.JsonError }
        }
      }

      // If the request failed, throw an error that exposes the server response
      // in both shapes call sites use, so error handling is consistent:
      //   err.error  -> the server error code, e.g. `switch (err.error)` or
      //                 `err.error?.code` (the body's `error` field, or the raw
      //                 body if it isn't an object)
      //   err.parsed -> the full parsed body, e.g. `err.parsed?.error`
      // plus err.status and a readable err.message for debugging.
      if (!res.ok) {
        // Without this a mid-session strike-off left the SPA sitting in a
        // stale signed-in state, showing empty screens instead of a login.
        if (
          res.status === 401 &&
          unauthorizedHandler &&
          !isAuthProbe(route) &&
          sessionEndedCodes.includes(
            returnValue && typeof returnValue === "object"
              ? returnValue.error
              : returnValue
          )
        ) {
          unauthorizedHandler()
        }

        const snippet =
          typeof returnValue === "string"
            ? returnValue.slice(0, 500)
            : JSON.stringify(returnValue).slice(0, 500)
        const err = new Error(
          `HTTP ${res.status} ${res.statusText} - ${snippet}`
        )
        err.status = res.status
        err.parsed = returnValue
        err.error =
          returnValue && typeof returnValue === "object"
            ? returnValue.error
            : returnValue
        throw err
      }

      return returnValue
    })

  // Writes are passed straight through in this phase; only reads are cached.
  if (method !== "GET") return request

  return request
    .then((data) => {
      // Fire and forget. A member should never wait on the cache to read a
      // page, and writeCache swallows its own failures (a full disk on a phone
      // is an expected outcome, not an error worth surfacing).
      writeCache(route, data).catch(() => {})
      return data
    })
    .catch(async (err) => {
      // Only a transport failure may be answered from cache. A 403, a 404 or a
      // 500 is the server's real answer and must reach the call site unchanged
      // — serving stale data over a genuine "you may not see this" would be a
      // permissions bug wearing an offline costume.
      if (err?.offline) {
        const cached = await readCacheAsResponse(route)
        // The value carries its own age (see `staleAt`), so call sites that
        // want to say "last updated ..." can, and the rest need no changes.
        if (cached !== null) return cached
      }
      throw err
    })
}
