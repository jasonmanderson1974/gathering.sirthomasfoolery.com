import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { get, post, setUnauthorizedHandler } from "./fetch_utils"
import { errors } from "@/constants"
import {
  __resetCacheForTests,
  staleAt,
  writeCache,
} from "@/utils/offline/cache"
import { __resetStatusForTests, isOffline } from "@/utils/offline/status"

// Mock global.fetch to return a canned Response-like object. fetchMethod only
// reads ok/status/statusText/text(), so that's all we stub.
function mockFetch({ ok, status = 200, statusText = "", body = "" }) {
  global.fetch = vi.fn().mockResolvedValue({
    ok,
    status,
    statusText,
    text: async () => body,
  })
}

beforeEach(() => {
  __resetCacheForTests()
  __resetStatusForTests()
  vi.stubGlobal("navigator", { onLine: true })
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const offlineFetch = () => {
  global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"))
}

describe("fetchMethod success", () => {
  it("parses and returns a JSON body", async () => {
    mockFetch({ ok: true, body: JSON.stringify({ hello: "world" }) })
    await expect(get("/x")).resolves.toEqual({ hello: "world" })
  })

  it("returns an empty string for an empty body", async () => {
    mockFetch({ ok: true, body: "" })
    await expect(get("/x")).resolves.toBe("")
  })
})

describe("fetchMethod error shape (A10 standardized contract)", () => {
  it("exposes err.error, err.parsed, and err.status for a JSON error body", async () => {
    mockFetch({
      ok: false,
      status: 403,
      statusText: "Forbidden",
      body: JSON.stringify({ error: "not-invited" }),
    })
    // Both shapes call sites use must be present:
    //   err.error        (switch (err.error))
    //   err.parsed.error (err.parsed?.error)
    await expect(post("/x", {})).rejects.toMatchObject({
      error: "not-invited",
      status: 403,
      parsed: { error: "not-invited" },
    })
  })

  it("keeps a nested error object accessible via err.error (e.g. err.error.code)", async () => {
    mockFetch({
      ok: false,
      status: 401,
      body: JSON.stringify({ error: { code: 401 } }),
    })
    await expect(get("/x")).rejects.toMatchObject({
      error: { code: 401 },
      parsed: { error: { code: 401 } },
    })
  })

  it("falls back to the raw parsed value when the error body isn't an object", async () => {
    mockFetch({
      ok: false,
      status: 500,
      body: JSON.stringify("plain string error"),
    })
    await expect(get("/x")).rejects.toMatchObject({
      error: "plain string error",
      status: 500,
    })
  })

  it("throws a JsonError when a non-empty body isn't valid JSON", async () => {
    mockFetch({ ok: true, body: "not json" })
    await expect(get("/x")).rejects.toMatchObject({ error: errors.JsonError })
  })
})

describe("network failure (offline)", () => {
  // fetch REJECTS rather than resolving when the request never reached the
  // server. Untagged, that was indistinguishable from a 401 to the router
  // guard, which bounced a signed-in member to /sign-in on a lost signal.
  it("tags a fetch rejection as offline", async () => {
    global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"))
    await expect(get("/events/abc")).rejects.toMatchObject({
      offline: true,
      error: errors.Offline,
    })
  })

  it("keeps the original failure as err.cause", async () => {
    const cause = new TypeError("Failed to fetch")
    global.fetch = vi.fn().mockRejectedValue(cause)
    await expect(get("/x")).rejects.toHaveProperty("cause", cause)
  })

  it("does NOT tag a non-2xx response as offline", async () => {
    mockFetch({ ok: false, status: 500, body: JSON.stringify({ error: "e" }) })
    await expect(get("/x")).rejects.not.toHaveProperty("offline", true)
  })

  // A body the server really did send back, just unparseable. Relabelling it
  // offline would send the caller looking for a cache that can't help.
  it("does NOT tag a JSON parse failure as offline", async () => {
    mockFetch({ ok: true, body: "not json" })
    await expect(get("/x")).rejects.toMatchObject({ error: errors.JsonError })
  })

  it("does not fire the session-ended handler", async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"))
    await expect(get("/events/abc")).rejects.toBeTruthy()
    expect(onUnauthorized).not.toHaveBeenCalled()
    setUnauthorizedHandler(null)
  })
})

describe("offline reads fall back to cache", () => {
  it("caches an allowlisted GET and serves it when the network is gone", async () => {
    mockFetch({ ok: true, body: JSON.stringify([{ _id: "e1" }]) })
    await get("/user/events")

    offlineFetch()
    await expect(get("/user/events")).resolves.toEqual([{ _id: "e1" }])
  })

  it("marks the cached value with when it was fetched", async () => {
    mockFetch({ ok: true, body: JSON.stringify({ name: "Gathering" }) })
    await get("/events/abc123")

    offlineFetch()
    const served = await get("/events/abc123")
    expect(staleAt(served)).toBeTypeOf("number")
  })

  it("leaves a live response unmarked, so a view can tell the two apart", async () => {
    mockFetch({ ok: true, body: JSON.stringify({ name: "Gathering" }) })
    expect(staleAt(await get("/events/abc123"))).toBeNull()
  })

  // The marker must not be visible to anything that walks the payload —
  // JSON.stringify, Object.keys, v-for — or it would eventually be sent back
  // to the server as a field.
  it("keeps the marker off the enumerable surface of the value", async () => {
    await writeCache("/events/abc123", { name: "Gathering" })
    offlineFetch()
    const served = await get("/events/abc123")
    expect(Object.keys(served)).toEqual(["name"])
    expect(JSON.parse(JSON.stringify(served))).toEqual({ name: "Gathering" })
  })

  it("still rejects when the network is gone and nothing was cached", async () => {
    offlineFetch()
    await expect(get("/user/events")).rejects.toMatchObject({ offline: true })
  })

  it("does not cache a route that is not on the allowlist", async () => {
    mockFetch({ ok: true, body: JSON.stringify({ events: [] }) })
    await get("/user/calendars?timeMin=1&timeMax=2")

    offlineFetch()
    await expect(get("/user/calendars?timeMin=1&timeMax=2")).rejects.toMatchObject(
      { offline: true }
    )
  })

  it("does not cache a non-2xx response", async () => {
    mockFetch({ ok: false, status: 500, body: JSON.stringify({ error: "e" }) })
    await expect(get("/user/events")).rejects.toBeTruthy()

    offlineFetch()
    await expect(get("/user/events")).rejects.toMatchObject({ offline: true })
  })

  // A real answer from the server must reach the call site unchanged. Serving
  // stale data over a genuine "you may not see this" would be a permissions
  // bug wearing an offline costume.
  it("does NOT serve cache for a 403, only for a transport failure", async () => {
    mockFetch({ ok: true, body: JSON.stringify({ name: "Gathering" }) })
    await get("/events/abc123")

    mockFetch({
      ok: false,
      status: 403,
      body: JSON.stringify({ error: errors.NotAuthorized }),
    })
    await expect(get("/events/abc123")).rejects.toMatchObject({ status: 403 })
  })

  it("does not serve cache to a write", async () => {
    await writeCache("/events/abc123", { name: "Gathering" })
    offlineFetch()
    await expect(post("/events/abc123", {})).rejects.toMatchObject({
      offline: true,
    })
  })

  it("refreshes the cached copy on a later successful read", async () => {
    mockFetch({ ok: true, body: JSON.stringify({ name: "Old" }) })
    await get("/events/abc123")
    mockFetch({ ok: true, body: JSON.stringify({ name: "New" }) })
    await get("/events/abc123")

    offlineFetch()
    await expect(get("/events/abc123")).resolves.toMatchObject({ name: "New" })
  })
})

describe("connectivity signal", () => {
  it("reports offline after a transport failure", async () => {
    offlineFetch()
    await expect(get("/user/events")).rejects.toBeTruthy()
    expect(isOffline()).toBe(true)
  })

  // Any answer proves the server is reachable, which is a stronger signal than
  // navigator.onLine and the one that clears a captive portal's false "online".
  it("clears once any response arrives, including an error response", async () => {
    offlineFetch()
    await expect(get("/user/events")).rejects.toBeTruthy()

    mockFetch({ ok: false, status: 500, body: JSON.stringify({ error: "e" }) })
    await expect(get("/user/events")).rejects.toBeTruthy()
    expect(isOffline()).toBe(false)
  })
})

describe("session-ended 401 handler", () => {
  it("fires for a normal route whose session has ended", async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    mockFetch({
      ok: false,
      status: 401,
      body: JSON.stringify({ error: errors.NotSignedIn }),
    })
    await expect(get("/events/abc")).rejects.toBeTruthy()
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
    setUnauthorizedHandler(null)
  })

  // The router guard probes /user/profile to decide whether anyone is signed
  // in; a 401 there is the expected answer, not a session ending. Firing the
  // handler pushed to /sign-in mid-navigation and looped the signed-out site
  // between / and /sign-in, rendering neither.
  it("does NOT fire for the auth probe", async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    mockFetch({
      ok: false,
      status: 401,
      body: JSON.stringify({ error: errors.NotSignedIn }),
    })
    await expect(get("/user/profile")).rejects.toBeTruthy()
    expect(onUnauthorized).not.toHaveBeenCalled()
    setUnauthorizedHandler(null)
  })

  it("does not fire on a 403", async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    mockFetch({
      ok: false,
      status: 403,
      body: JSON.stringify({ error: errors.NotAuthorized }),
    })
    await expect(get("/events/abc")).rejects.toBeTruthy()
    expect(onUnauthorized).not.toHaveBeenCalled()
    setUnauthorizedHandler(null)
  })
})
