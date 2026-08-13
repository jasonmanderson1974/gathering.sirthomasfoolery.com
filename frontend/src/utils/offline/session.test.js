import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { forgetSession, recallSession, rememberSession } from "./session"

const KEY = "offline-session"
const DAY = 24 * 60 * 60 * 1000

// This tier is `node`, so there is no localStorage. Standing one up here keeps
// the test honest about what the module actually does with it — including the
// removals, which are the part that matters for not leaving a user id on disk.
let store

beforeEach(() => {
  store = new Map()
  vi.stubGlobal("localStorage", {
    getItem: (k) => (store.has(k) ? store.get(k) : null),
    setItem: (k, v) => store.set(k, String(v)),
    removeItem: (k) => store.delete(k),
  })
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe("remember / recall", () => {
  it("round-trips a signed-in user", () => {
    rememberSession("userA")
    expect(recallSession().userId).toBe("userA")
  })

  it("recalls nothing when none was ever written", () => {
    expect(recallSession()).toBeNull()
  })

  it("ignores a falsy user id rather than writing a useless record", () => {
    rememberSession(null)
    rememberSession("")
    expect(recallSession()).toBeNull()
  })

  it("forgets on sign-out", () => {
    rememberSession("userA")
    forgetSession()
    expect(recallSession()).toBeNull()
  })
})

describe("expiry", () => {
  it("still recalls a record inside the cookie's 30-day lifetime", () => {
    vi.useFakeTimers()
    rememberSession("userA")
    vi.advanceTimersByTime(29 * DAY)
    expect(recallSession()?.userId).toBe("userA")
  })

  it("refuses one past it", () => {
    vi.useFakeTimers()
    rememberSession("userA")
    vi.advanceTimersByTime(31 * DAY)
    expect(recallSession()).toBeNull()
  })

  // Not merely ignored — deleted. An expired record cannot correspond to a
  // live session, so keeping it leaves a user id on the device for nothing.
  it("deletes an expired record rather than leaving it on disk", () => {
    vi.useFakeTimers()
    rememberSession("userA")
    vi.advanceTimersByTime(31 * DAY)
    recallSession()
    expect(store.has(KEY)).toBe(false)
  })

  it("refuses a record stamped in the future (clock moved backwards)", () => {
    store.set(KEY, JSON.stringify({ userId: "userA", lastVerifiedAt: Date.now() + DAY }))
    expect(recallSession()).toBeNull()
  })
})

describe("malformed records", () => {
  it.each([
    ["unparseable", "{not json"],
    ["missing userId", JSON.stringify({ lastVerifiedAt: Date.now() })],
    ["missing timestamp", JSON.stringify({ userId: "userA" })],
    ["timestamp not a number", JSON.stringify({ userId: "a", lastVerifiedAt: "x" })],
  ])("refuses and clears a %s record", (_label, raw) => {
    store.set(KEY, raw)
    expect(recallSession()).toBeNull()
    expect(store.has(KEY)).toBe(false)
  })
})

describe("localStorage unavailable", () => {
  // Safari private browsing throws on access. The app must still run; it just
  // can't boot offline.
  it("does not throw when storage refuses", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => {
        throw new Error("denied")
      },
      setItem: () => {
        throw new Error("denied")
      },
      removeItem: () => {
        throw new Error("denied")
      },
    })
    expect(() => rememberSession("userA")).not.toThrow()
    expect(() => forgetSession()).not.toThrow()
    expect(recallSession()).toBeNull()
  })
})
