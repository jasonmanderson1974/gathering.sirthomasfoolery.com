/*
 * The cold offline boot, end to end through the real store and the real
 * fetch_utils.
 *
 * This is the failure the whole read layer exists to fix: the router guard's
 * first act is to read GET /user/profile, and a transport failure there was
 * indistinguishable from a 401 — so a signed-in member whose phone had no
 * signal was told to sign in, by an invite-only app, with their session still
 * perfectly valid.
 *
 * Nothing is mocked here except `fetch` itself, so the store action, the cache
 * and the key-scoping all run for real.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import store from "@/store"
import {
  __resetCacheForTests,
  setCacheUser,
  writeCache,
} from "@/utils/offline/cache"
import { __resetStatusForTests } from "@/utils/offline/status"

const USER = { _id: "userA", firstName: "Jason", email: "jason@example.com" }

const online = (payloads) => {
  global.fetch = vi.fn(async (url) => {
    const route = String(url).replace(/^.*\/api/, "")
    const body = payloads[route]
    return {
      ok: body !== undefined,
      status: body !== undefined ? 200 : 404,
      statusText: "",
      text: async () => JSON.stringify(body ?? { error: "not-found" }),
    }
  })
}

const offline = () => {
  global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"))
}

beforeEach(() => {
  __resetCacheForTests()
  __resetStatusForTests()
  vi.stubGlobal("navigator", { onLine: true })
  vi.stubGlobal("localStorage", {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
  })
  store.commit("setAuthUser", null)
  store.commit("setEvents", [])
  store.commit("setFolders", [])
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe("refreshAuthUser with no connection", () => {
  it("resolves the member from cache instead of failing", async () => {
    online({ "/user/profile": USER })
    await store.dispatch("refreshAuthUser")

    // A new session: the cache survives, the store does not.
    store.commit("setAuthUser", null)
    offline()

    const authUser = await store.dispatch("refreshAuthUser")
    expect(authUser).toMatchObject({ _id: "userA" })
    expect(store.state.authUser).toMatchObject({ _id: "userA" })
  })

  // The guard treats a rejection as "not signed in" and redirects. With
  // nothing cached that is still the right answer — there would be nothing to
  // render — but it must be a rejection the guard can recognise as offline.
  it("still rejects, as offline, when nothing was ever cached", async () => {
    offline()
    await expect(store.dispatch("refreshAuthUser")).rejects.toMatchObject({
      offline: true,
    })
  })

  it("does not serve one member's profile to another", async () => {
    await setCacheUser("userA")
    await writeCache("/user/profile", USER)
    await setCacheUser("userB")
    offline()

    await expect(store.dispatch("refreshAuthUser")).rejects.toMatchObject({
      offline: true,
    })
  })
})

describe("getEvents with no connection", () => {
  const FOLDERS = [{ _id: "f1", name: "Upcoming" }]
  const EVENTS = [{ _id: "e1", name: "Michaelmas Dinner" }]

  it("renders the dashboard from cache", async () => {
    online({
      "/user/profile": USER,
      "/user/folders": FOLDERS,
      "/user/events": EVENTS,
    })
    await store.dispatch("refreshAuthUser")
    await store.dispatch("getEvents")

    store.commit("setEvents", [])
    store.commit("setFolders", [])
    offline()

    await store.dispatch("getEvents")
    expect(store.state.events).toEqual(EVENTS)
    expect(store.state.folders).toEqual(FOLDERS)
  })

  // These two reads used to be all-or-nothing, which offline is the ordinary
  // case rather than a rare one: a cached copy of one may well exist without
  // the other.
  it("commits whichever read succeeded when only one is cached", async () => {
    online({ "/user/profile": USER })
    await store.dispatch("refreshAuthUser")
    await writeCache("/user/events", EVENTS)

    offline()
    await store.dispatch("getEvents")

    expect(store.state.events).toEqual(EVENTS)
    expect(store.state.folders).toEqual([])
  })

  it("does not raise an error snackbar merely for being offline", async () => {
    online({ "/user/profile": USER })
    await store.dispatch("refreshAuthUser")
    offline()

    await store.dispatch("getEvents")
    expect(store.state.error).toBe("")
  })

  // A real server failure must still be reported — the offline path must not
  // become a way to swallow genuine errors.
  it("still reports a genuine failure", async () => {
    online({ "/user/profile": USER })
    await store.dispatch("refreshAuthUser")

    global.fetch = vi.fn(async () => ({
      ok: false,
      status: 500,
      statusText: "",
      text: async () => JSON.stringify({ error: "internal-error" }),
    }))
    vi.spyOn(console, "error").mockImplementation(() => {})

    await store.dispatch("getEvents")
    await vi.waitFor(() => expect(store.state.error).not.toBe(""))
  })
})
