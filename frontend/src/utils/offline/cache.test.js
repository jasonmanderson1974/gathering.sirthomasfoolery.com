import { beforeEach, describe, expect, it } from "vitest"
import {
  __resetCacheForTests,
  clearCache,
  readCache,
  setCacheUser,
  writeCache,
} from "./cache"

// This tier has no IndexedDB, which makes it exactly the right place to prove
// the module still works without one — the same situation as Safari private
// browsing, or a browser that refuses to open the database. Every assertion
// below therefore also doubles as a check that nothing here throws when the
// persistent half is missing.
beforeEach(() => {
  __resetCacheForTests()
})

describe("read/write", () => {
  it("round-trips an allowlisted route with a fetchedAt stamp", async () => {
    await writeCache("/user/events", [{ _id: "e1" }])
    const hit = await readCache("/user/events")
    expect(hit.body).toEqual([{ _id: "e1" }])
    expect(hit.fetchedAt).toBeTypeOf("number")
  })

  it("misses for a route never written", async () => {
    expect(await readCache("/user/events")).toBeNull()
  })

  it("drops a write to a non-allowlisted route rather than storing it", async () => {
    await writeCache("/user/calendars?timeMin=1", { events: [] })
    expect(await readCache("/user/calendars?timeMin=1")).toBeNull()
  })

  // An empty-body response arrives as "". Storing it would shadow a good
  // entry already in the cache with something a view can't render.
  it("does not let an empty or undefined body shadow a real entry", async () => {
    await writeCache("/user/events", [{ _id: "e1" }])
    await writeCache("/user/events", "")
    await writeCache("/user/events", undefined)
    expect((await readCache("/user/events")).body).toEqual([{ _id: "e1" }])
  })

  it("overwrites on a fresh read of the same route", async () => {
    await writeCache("/user/events", [{ _id: "old" }])
    await writeCache("/user/events", [{ _id: "new" }])
    expect((await readCache("/user/events")).body).toEqual([{ _id: "new" }])
  })
})

// Views own their payloads and mutate them in place — Event.vue assigns the
// response to `this.event` and processEvent derives fields on it. Sharing the
// stored object would let every edit on screen silently rewrite the saved copy.
describe("isolation from the caller", () => {
  it("does not let a caller's later mutation reach the stored copy", async () => {
    const body = { name: "Gathering", lists: [{ _id: "l1" }] }
    await writeCache("/events/abc123", body)

    body.name = "Renamed locally"
    body.lists.push({ _id: "l2" })

    const hit = await readCache("/events/abc123")
    expect(hit.body.name).toBe("Gathering")
    expect(hit.body.lists).toHaveLength(1)
  })

  it("does not let two readers share one object", async () => {
    await writeCache("/events/abc123", { name: "Gathering" })
    const first = await readCache("/events/abc123")
    first.body.name = "Mutated by the first reader"

    const second = await readCache("/events/abc123")
    expect(second.body.name).toBe("Gathering")
  })
})

describe("user scoping", () => {
  // GET /events/:id is caller-dependent — emails stripped for non-owners,
  // members-only threads filtered for guests, blind availability privatized by
  // session. Two accounts on one phone must never read each other's view.
  it("does not serve one user's entry to another", async () => {
    await setCacheUser("userA")
    await writeCache("/events/abc123", { name: "A's view" })
    await setCacheUser("userB")
    expect(await readCache("/events/abc123")).toBeNull()
  })

  it("purges the previous user's entries on a switch", async () => {
    await setCacheUser("userA")
    await writeCache("/events/abc123", { name: "A's view" })
    await setCacheUser("userB")
    await setCacheUser("userA")
    expect(await readCache("/events/abc123")).toBeNull()
  })

  // The boot ordering: /user/profile is read before anyone knows the id, so it
  // lands under the anon sentinel. Adopting rather than discarding is what
  // lets the NEXT cold boot — the offline one — find it.
  it("adopts entries cached before the user was known", async () => {
    await writeCache("/user/profile", { _id: "userA", firstName: "Jason" })
    await setCacheUser("userA")
    expect((await readCache("/user/profile")).body).toEqual({
      _id: "userA",
      firstName: "Jason",
    })
  })

  it("adopting preserves the original fetchedAt, not the adoption time", async () => {
    await writeCache("/user/profile", { _id: "userA" })
    const before = (await readCache("/user/profile")).fetchedAt
    await setCacheUser("userA")
    expect((await readCache("/user/profile")).fetchedAt).toBe(before)
  })

  it("re-declaring the same user is a no-op, not a purge", async () => {
    await setCacheUser("userA")
    await writeCache("/events/abc123", { name: "A's view" })
    await setCacheUser("userA")
    expect(await readCache("/events/abc123")).not.toBeNull()
  })
})

describe("clearCache", () => {
  it("leaves nothing readable after a sign-out", async () => {
    await setCacheUser("userA")
    await writeCache("/events/abc123", { name: "A's view" })
    await writeCache("/user/events", [{ _id: "e1" }])
    await clearCache()
    await setCacheUser("userA")
    expect(await readCache("/events/abc123")).toBeNull()
    expect(await readCache("/user/events")).toBeNull()
  })
})
