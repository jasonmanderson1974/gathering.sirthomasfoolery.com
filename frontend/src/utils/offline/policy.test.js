import { describe, expect, it } from "vitest"
import { ANON_USER, cacheKey, isCacheable, keyUser } from "./policy"

describe("isCacheable", () => {
  it("allows the reads a member needs to open a gathering offline", () => {
    const allowed = [
      "/user/profile",
      "/user/events",
      "/user/folders",
      "/events/652f1a9b8c4d3e2f10a9b8c7",
      "/events/abc123",
      "/events/abc123/lists",
      "/events/abc123/expenses",
      "/events/abc123/expenses/participants",
      "/events/abc123/my-lists",
      "/events/abc123/my-notes",
      "/events/abc123/mentionables",
      "/admin/allowlist",
    ]
    for (const route of allowed) {
      expect(isCacheable(route), route).toBe(true)
    }
  })

  // Each of these is deliberately excluded; see the comment block in policy.js.
  // Persisting member data is the cost, so the exclusions are asserted rather
  // than merely documented.
  it("refuses the routes deliberately kept off disk", () => {
    const refused = [
      "/user/calendars?timeMin=1&timeMax=2",
      "/events/abc123/calendar-availabilities?timeMin=1",
      "/auth/status",
      "/events/abc123/lists/assignees",
      "/users/652f1a9b8c4d3e2f10a9b8c7",
      "/user/events/abc/set-folder",
    ]
    for (const route of refused) {
      expect(isCacheable(route), route).toBe(false)
    }
  })

  // The patterns are anchored, so a parameter means no match. A route that
  // grows one must come back through policy.js rather than quietly caching a
  // single arbitrary variant of itself under a shared key.
  it("refuses an otherwise-allowed route carrying a query string", () => {
    expect(isCacheable("/events/abc123")).toBe(true)
    expect(isCacheable("/events/abc123?guestName=x")).toBe(false)
    expect(isCacheable("/user/profile?x=1")).toBe(false)
  })

  it("does not let an extra path segment ride in on a prefix", () => {
    expect(isCacheable("/events/abc123/responses")).toBe(false)
    expect(isCacheable("/admin/allowlist/export")).toBe(false)
    expect(isCacheable("/user/profile/secrets")).toBe(false)
  })

  it("survives a non-string route", () => {
    expect(isCacheable(undefined)).toBe(false)
    expect(isCacheable(null)).toBe(false)
    expect(isCacheable(42)).toBe(false)
  })
})

describe("cacheKey", () => {
  it("scopes an entry to its user", () => {
    expect(cacheKey("userA", "/user/events")).not.toBe(
      cacheKey("userB", "/user/events")
    )
  })

  it("falls back to the anon sentinel before the user is known", () => {
    expect(keyUser(cacheKey(null, "/user/profile"))).toBe(ANON_USER)
    expect(keyUser(cacheKey(undefined, "/user/profile"))).toBe(ANON_USER)
  })

  it("keyUser reads back the owner, and a route containing | can't spoof it", () => {
    expect(keyUser(cacheKey("userA", "/events/a|b"))).toBe("userA")
  })
})
