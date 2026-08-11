/*
 * The API, faked at `fetch` (TODO3 M6).
 *
 * Faked THERE rather than at `@/utils`, so `fetch_utils.js` is real code under
 * test: its JSON parsing, its `!res.ok` error shape and its session-expiry
 * branch all run. Mocking the module instead would replace exactly the layer
 * every call site's error handling is written against.
 *
 * Installed for every test in the `dom` project whether or not it asks, because
 * an unmocked call is not harmless here — `NewEvent` mounts `EmailInput`, which
 * warms a contacts cache in `mounted()`, and happy-dom will happily open a real
 * socket to localhost:3000 to do it. That is a test suite whose result depends
 * on what is listening on this machine.
 */
import { serverURL } from "@/constants"

/** Handlers, most-recently-registered first. */
let handlers = []
/** Every call made since the current test began. */
let calls = []

/**
 * Answers `route` with `response`.
 *
 * @param {string|RegExp} route matched against the path AFTER `serverURL` —
 *   `"/user/profile"` matches a call to it with or without a query string.
 * @param {*} response the parsed body to return; a function is called with
 *   `{ method, route, body }`. Throw `{ status, body }` from it for a failure.
 */
export function mockApi(route, response) {
  handlers.unshift({ route, response })
}

/** `[{ method, route, body }]` in call order. */
export function apiCalls() {
  return calls
}

/** Whether any call was made to a route matching `route`. */
export function calledApi(route) {
  return calls.some((c) => matches(route, c.route))
}

const matches = (pattern, route) =>
  pattern instanceof RegExp
    ? pattern.test(route)
    : route === pattern || route.startsWith(pattern + "?")

/** Installs the fake and clears both handlers and the call log. */
export function resetApi() {
  handlers = []
  calls = []
  globalThis.fetch = fakeFetch
}

async function fakeFetch(url, params = {}) {
  const full = String(url)
  const route = full.startsWith(serverURL) ? full.slice(serverURL.length) : full
  const method = params.method || "GET"
  const body = params.body ? JSON.parse(params.body) : undefined
  calls.push({ method, route, body })

  const handler = handlers.find((h) => matches(h.route, route))

  // The default is an empty object with a 200 rather than a rejection: an
  // unmocked call is a component doing something this test did not set out to
  // exercise, and failing it would make every spec a list of stubs for calls it
  // does not care about. `apiCalls()` is there for when the call IS the point.
  let payload = {}
  let status = 200
  if (handler) {
    try {
      payload =
        typeof handler.response === "function"
          ? await handler.response({ method, route, body })
          : handler.response
    } catch (thrown) {
      status = thrown?.status ?? 500
      payload = thrown?.body ?? { error: "mock-error" }
    }
  }

  const text = typeof payload === "string" ? payload : JSON.stringify(payload)
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => text,
    json: async () => JSON.parse(text),
  }
}
