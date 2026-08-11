import { afterEach, describe, expect, it, vi } from "vitest"
import { get, post, setUnauthorizedHandler } from "./fetch_utils"
import { errors } from "@/constants"

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

afterEach(() => {
  vi.restoreAllMocks()
})

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
