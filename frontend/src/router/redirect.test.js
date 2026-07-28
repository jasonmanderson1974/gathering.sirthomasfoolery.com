import { describe, it, expect } from "vitest"

// isSafeRedirect guards the ?redirect the router parks on the sign-in URL when
// it bounces an unauthenticated visitor. Without the same-origin check, a
// crafted link (/sign-in?redirect=https://evil.example) would turn our own
// login into an open redirect — the user signs in for real, then gets handed
// to an attacker's page carrying our referrer.
//
// Duplicated from router/index.js rather than imported: that module builds a
// VueRouter and pulls in the whole view graph at import time, which a unit test
// has no business doing. The predicate is three lines; keeping the copy honest
// is cheaper than booting the app.
const isSafeRedirect = (path) =>
  typeof path === "string" && path.startsWith("/") && !path.startsWith("//")

describe("isSafeRedirect", () => {
  it("accepts same-origin paths", () => {
    expect(isSafeRedirect("/")).toBe(true)
    expect(isSafeRedirect("/e/abc123")).toBe(true)
    expect(isSafeRedirect("/settings?tab=calendars")).toBe(true)
    expect(isSafeRedirect("/e/abc123#responses")).toBe(true)
  })

  it("rejects absolute URLs to other origins", () => {
    expect(isSafeRedirect("https://evil.example")).toBe(false)
    expect(isSafeRedirect("http://evil.example/e/abc")).toBe(false)
  })

  it("rejects protocol-relative URLs, which are off-site despite the leading slash", () => {
    expect(isSafeRedirect("//evil.example")).toBe(false)
    expect(isSafeRedirect("//evil.example/e/abc")).toBe(false)
  })

  it("rejects non-string and empty input", () => {
    expect(isSafeRedirect(undefined)).toBe(false)
    expect(isSafeRedirect(null)).toBe(false)
    expect(isSafeRedirect("")).toBe(false)
    expect(isSafeRedirect(["/e/abc"])).toBe(false)
    // Vue Router hands back an array when a query key repeats.
    expect(isSafeRedirect(["/a", "/b"])).toBe(false)
  })

  it("rejects schemes that aren't navigation at all", () => {
    expect(isSafeRedirect("javascript:alert(1)")).toBe(false)
    expect(isSafeRedirect("data:text/html,<script>alert(1)</script>")).toBe(
      false
    )
  })
})
