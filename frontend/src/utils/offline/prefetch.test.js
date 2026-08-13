import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { __resetPrefetchForTests, prefetchGatherings } from "./prefetch"
import { __resetCacheForTests, readCache, setCacheUser } from "./cache"
import {
  __resetStatusForTests,
  noteNetworkFailure,
} from "./status"

const EVENTS = [
  { shortId: "aaa", name: "Michaelmas Dinner", isArchived: false },
  { shortId: "bbb", name: "Boxing Day Shoot", isArchived: false },
  { shortId: "zzz", name: "Last Year's Wake", isArchived: true },
]

let routes

const respond = () => {
  routes = []
  global.fetch = vi.fn(async (url) => {
    const route = String(url).replace(/^.*\/api/, "")
    routes.push(route)
    return {
      ok: true,
      status: 200,
      statusText: "",
      text: async () => JSON.stringify({ route }),
    }
  })
}

// No pause and no start delay: both exist only to keep this out of the user's
// way, and neither changes what it fetches.
const run = (events) =>
  prefetchGatherings(events, { pauseMs: 0, startDelayMs: 0 })

beforeEach(async () => {
  __resetPrefetchForTests()
  __resetCacheForTests()
  __resetStatusForTests()
  vi.stubGlobal("navigator", { onLine: true })
  await setCacheUser("userA")
  respond()
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe("prefetchGatherings", () => {
  it("caches the payload and the ledger for each active gathering", async () => {
    await run(EVENTS)

    expect(await readCache("/events/aaa")).not.toBeNull()
    expect(await readCache("/events/aaa/expenses")).not.toBeNull()
    expect(await readCache("/events/bbb")).not.toBeNull()
    expect(await readCache("/events/bbb/expenses")).not.toBeNull()
  })

  // The two private tabs live in their own collections rather than on the event
  // (deliberately — it is what makes them impossible to leak through
  // GET /events/:id), so they are absent offline unless fetched in their own
  // right. Reported from a real phone: My Lists was empty after installing,
  // opening a gathering and switching to airplane mode.
  it("caches My Lists and My Notes, which do not ride along with the event", async () => {
    await run(EVENTS)

    expect(await readCache("/events/aaa/my-lists")).not.toBeNull()
    expect(await readCache("/events/aaa/my-notes")).not.toBeNull()
  })

  it("leaves archived gatherings alone", async () => {
    await run(EVENTS)
    expect(routes).not.toContain("/events/zzz")
  })

  // One event payload carries the discussion and the lists, so a warm-up cut
  // short still leaves the page readable.
  it("fetches the event payload before its ledger", async () => {
    await run(EVENTS)
    expect(routes.indexOf("/events/aaa")).toBeLessThan(
      routes.indexOf("/events/aaa/expenses")
    )
  })

  it("does nothing at all when already offline", async () => {
    noteNetworkFailure()
    await run(EVENTS)
    expect(routes).toEqual([])
  })

  // Losing signal mid-warm is the ordinary case on a phone. Grinding through a
  // dozen failing requests to discover it is not useful.
  it("stops as soon as the connection drops", async () => {
    global.fetch = vi.fn(async (url) => {
      const route = String(url).replace(/^.*\/api/, "")
      routes.push(route)
      if (routes.length >= 2) throw new TypeError("Failed to fetch")
      return { ok: true, status: 200, statusText: "", text: async () => "{}" }
    })

    await run(EVENTS)
    // Intent, not a count: it must give up rather than work through the rest of
    // the sweep. The second gathering is never reached at all.
    expect(routes.some((r) => r.includes("bbb"))).toBe(false)
    expect(routes.length).toBeLessThan(EVENTS.length * 4)
  })

  it("survives a gathering that fails without abandoning the rest", async () => {
    global.fetch = vi.fn(async (url) => {
      const route = String(url).replace(/^.*\/api/, "")
      routes.push(route)
      if (route === "/events/aaa") {
        return {
          ok: false,
          status: 500,
          statusText: "",
          text: async () => JSON.stringify({ error: "internal-error" }),
        }
      }
      return { ok: true, status: 200, statusText: "", text: async () => "{}" }
    })

    await run(EVENTS)
    expect(routes).toContain("/events/bbb")
  })

  it("does not re-warm on every visit to the dashboard", async () => {
    await run(EVENTS)
    const first = routes.length
    await run(EVENTS)
    expect(routes.length).toBe(first)
  })

  it("ignores a missing or malformed list", async () => {
    await expect(run(undefined)).resolves.toBeUndefined()
    await expect(run(null)).resolves.toBeUndefined()
    await expect(run([null, {}])).resolves.toBeUndefined()
    expect(routes).toEqual([])
  })
})
