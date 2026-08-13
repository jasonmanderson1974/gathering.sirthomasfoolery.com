/*
 * The write queue's rules, against the real ops table and the real cache.
 *
 * Only `fetch` is faked. The reducers, the collapse rules and the id remapping
 * all run for real, because those are the parts that decide whether a member's
 * offline afternoon arrives intact.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import {
  __resetQueueForTests,
  clearQueue,
  enqueue,
  flush,
  pendingCount,
} from "./queue"
import {
  __resetCacheForTests,
  readCache,
  setCacheUser,
  writeCache,
} from "./cache"
import {
  __resetStatusForTests,
  noteNetworkFailure,
  noteNetworkSuccess,
} from "./status"

const EVENT = "abc123"
const LIST = "list1"

/** Requests the fake server saw, in order. */
let seen
/** route -> handler, for the ones a test cares about. */
let handlers

const server = () => {
  seen = []
  handlers = {}
  global.fetch = vi.fn(async (url, params = {}) => {
    const route = String(url).replace(/^.*\/api/, "")
    const method = params.method || "GET"
    const body = params.body ? JSON.parse(params.body) : undefined
    seen.push({ method, route, body })

    const handler = handlers[route]
    if (handler) return handler({ method, route, body })
    return {
      ok: true,
      status: 200,
      statusText: "",
      text: async () => JSON.stringify({ _id: `server-${seen.length}` }),
    }
  })
}

const fails = (status, error = "nope") => () => ({
  ok: false,
  status,
  statusText: "",
  text: async () => JSON.stringify({ error }),
})

const offline = () => {
  noteNetworkFailure()
  global.fetch = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"))
}

const online = () => {
  noteNetworkSuccess()
  server()
}

const seedEventWithList = () =>
  writeCache(`/events/${EVENT}`, {
    _id: EVENT,
    name: "Second Breakfast",
    comments: [],
    lists: [{ _id: LIST, name: "To bring", items: [] }],
  })

let storage

beforeEach(async () => {
  storage = new Map()
  vi.stubGlobal("localStorage", {
    getItem: (k) => (storage.has(k) ? storage.get(k) : null),
    setItem: (k, v) => storage.set(k, String(v)),
    removeItem: (k) => storage.delete(k),
  })
  __resetQueueForTests()
  __resetCacheForTests()
  __resetStatusForTests()
  vi.stubGlobal("navigator", { onLine: true })
  await setCacheUser("userA")
  await clearQueue()
  server()
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

// No spacing in tests: it exists to protect the server's mention-email budget,
// not to change what is sent.
const drain = () => flush({ spacingMs: 0 })

describe("queueing while offline", () => {
  it("holds the write instead of losing it", async () => {
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "Bring the port" })
    expect(await pendingCount()).toBe(1)
  })

  // The whole point of the reducers: the page must show the change without a
  // single call site being rewritten to update local state.
  it("shows the change on the cached page immediately", async () => {
    await seedEventWithList()
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "Bring the port" })

    const cached = await readCache(`/events/${EVENT}`)
    expect(cached.body.comments).toHaveLength(1)
    expect(cached.body.comments[0].text).toBe("Bring the port")
  })

  // A local edit is not a refresh. Claiming otherwise would hide real staleness
  // behind the member's own typing.
  it("does not make the cached copy look fresher than it is", async () => {
    await seedEventWithList()
    const before = (await readCache(`/events/${EVENT}`)).fetchedAt
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "x" })
    expect((await readCache(`/events/${EVENT}`)).fetchedAt).toBe(before)
  })

  it("keeps a list item's cascade when deleting offline", async () => {
    await writeCache(`/events/${EVENT}`, {
      _id: EVENT,
      lists: [
        {
          _id: LIST,
          items: [
            { _id: "parent", text: "Drinks" },
            { _id: "child", text: "Port", parentId: "parent" },
            { _id: "other", text: "Cheese" },
          ],
        },
      ],
    })
    offline()
    await enqueue("listItem.delete", {
      eventId: EVENT,
      listId: LIST,
      itemId: "parent",
    })

    const items = (await readCache(`/events/${EVENT}`)).body.lists[0].items
    expect(items.map((i) => i._id)).toEqual(["other"])
  })
})

// The bug this catches was invisible to every unit test until a real browser
// found it: the event page is read under whichever id was in the link, while
// Event.vue's write handlers address the gathering as `shortId ?? _id`. So the
// reducer edited a cache entry nobody was looking at, and an offline comment
// appeared to vanish.
describe("a gathering known by two ids", () => {
  const MONGO = "64b7f0c2c9e77c0012345678"

  it("shows an offline write on the copy the page is actually reading", async () => {
    // Cached under the Mongo id, as a link to /e/<mongoId> would leave it.
    await writeCache(`/events/${MONGO}`, {
      _id: MONGO,
      shortId: EVENT,
      comments: [],
      lists: [],
    })

    offline()
    // ...but written under the short id, as the view's handlers do.
    await enqueue("comment.add", { eventId: EVENT, text: "Bring the port" })

    const cached = await readCache(`/events/${MONGO}`)
    expect(cached.body.comments).toHaveLength(1)
  })
})

describe("assigning offline", () => {
  const withBranch = () =>
    writeCache(`/events/${EVENT}`, {
      _id: EVENT,
      lists: [
        {
          _id: LIST,
          items: [
            { _id: "parent", text: "Drinks" },
            { _id: "child", text: "Port", parentId: "parent" },
            { _id: "grandchild", text: "Decanter", parentId: "child" },
            { _id: "other", text: "Cheese" },
          ],
        },
      ],
    })

  // The server takes the whole subtree (N1), so the cached copy must too — or
  // the page shows one entry assigned and its children still free, which is not
  // what will come back on the next refetch.
  it("cascades to the whole subtree, as the server does", async () => {
    await withBranch()
    offline()
    await enqueue("listItem.assign", {
      eventId: EVENT,
      listId: LIST,
      itemId: "parent",
      assigneeId: "member-1",
      assigneeName: "Ambrose",
    })

    const items = (await readCache(`/events/${EVENT}`)).body.lists[0].items
    const held = Object.fromEntries(items.map((i) => [i._id, i.assigneeId]))
    expect(held).toEqual({
      parent: "member-1",
      child: "member-1",
      grandchild: "member-1",
      other: undefined,
    })
  })

  it("renders a name straight away, without waiting for the server", async () => {
    await withBranch()
    offline()
    await enqueue("listItem.assign", {
      eventId: EVENT,
      listId: LIST,
      itemId: "parent",
      assigneeId: "member-1",
      assigneeName: "Ambrose",
    })

    const items = (await readCache(`/events/${EVENT}`)).body.lists[0].items
    expect(items.find((i) => i._id === "parent").assigneeName).toBe("Ambrose")
  })

  // Clearing cascades too — un-assigning a parent is a reset, not an undo.
  it("clears the whole subtree when assigned to nobody", async () => {
    await withBranch()
    offline()
    await enqueue("listItem.assign", {
      eventId: EVENT,
      listId: LIST,
      itemId: "parent",
      assigneeId: "member-1",
      assigneeName: "Ambrose",
    })
    await enqueue("listItem.assign", {
      eventId: EVENT,
      listId: LIST,
      itemId: "parent",
      assigneeId: "",
      assigneeName: "",
    })

    const items = (await readCache(`/events/${EVENT}`)).body.lists[0].items
    expect(items.every((i) => !i.assigneeId)).toBe(true)
  })

  it("sends the assignment on reconnect", async () => {
    await withBranch()
    offline()
    await enqueue("listItem.assign", {
      eventId: EVENT,
      listId: LIST,
      itemId: "parent",
      assigneeId: "member-1",
      assigneeName: "Ambrose",
    })

    online()
    const result = await drain()
    expect(result.sent).toBe(1)
    const call = seen.find((c) => c.route.endsWith("/assignee"))
    expect(call.body).toEqual({ assigneeId: "member-1" })
  })
})

describe("flushing on reconnect", () => {
  it("sends what was queued, in the order it was made", async () => {
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "first" })
    await enqueue("comment.add", { eventId: EVENT, text: "second" })

    online()
    const result = await drain()

    expect(result.sent).toBe(2)
    expect(seen.map((c) => c.body.text)).toEqual(["first", "second"])
    expect(await pendingCount()).toBe(0)
  })

  // What makes the replay safe (O4): the server stores this and returns the
  // original row rather than making a second one.
  it("sends a clientId with every create", async () => {
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "x" })
    online()
    await drain()
    expect(seen[0].body.clientId).toBeTruthy()
  })

  it("stops and keeps everything when the connection goes again", async () => {
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "first" })
    await enqueue("comment.add", { eventId: EVENT, text: "second" })

    noteNetworkSuccess()
    let calls = 0
    global.fetch = vi.fn(async (url, params = {}) => {
      calls++
      if (calls > 1) throw new TypeError("Failed to fetch")
      seen.push({ body: JSON.parse(params.body) })
      return { ok: true, status: 200, statusText: "", text: async () => "{}" }
    })

    const result = await drain()
    expect(result.sent).toBe(1)
    expect(await pendingCount()).toBe(1)
  })

  // A create made offline has no real id yet. The mapping has to OUTLIVE the
  // flush that learned it: the cached page still shows the temporary id until
  // something refetches, so an edit made offline the next day would otherwise
  // address a dead id and 404.
  it("still resolves a temporary id in a later flush", async () => {
    await seedEventWithList()
    offline()
    const created = await enqueue("listItem.add", {
      eventId: EVENT,
      listId: LIST,
      text: "Port",
    })

    online()
    handlers[`/events/${EVENT}/lists/${LIST}/items`] = () => ({
      ok: true,
      status: 200,
      statusText: "",
      text: async () => JSON.stringify({ _id: "real-item-id" }),
    })
    await drain()

    // Now, online, tick the item the client still knows by its temporary id.
    // (Queued behind a completed create, this is the case collapse can't cover.)
    offline()
    await enqueue("listItem.check", {
      eventId: EVENT,
      listId: LIST,
      itemId: created._id,
      checked: true,
    })
    online()
    await drain()

    const tick = seen.find((c) => c.route.endsWith("/checked"))
    expect(tick).toBeDefined()
    expect(tick.route).toContain("real-item-id")
    expect(tick.route).not.toContain(created._id)
  })
})

describe("collapse", () => {
  // A queue built over a train journey that only grows is a bug.
  it("folds an edit into the create still waiting to be sent", async () => {
    await seedEventWithList()
    offline()
    const created = await enqueue("listItem.add", {
      eventId: EVENT,
      listId: LIST,
      text: "Port",
    })
    await enqueue("listItem.edit", {
      eventId: EVENT,
      listId: LIST,
      itemId: created._id,
      text: "Good port",
    })

    expect(await pendingCount()).toBe(1)

    online()
    await drain()
    expect(seen).toHaveLength(1)
    expect(seen[0].body.text).toBe("Good port")
  })

  it("drops both when something created offline is deleted again", async () => {
    await seedEventWithList()
    offline()
    const created = await enqueue("listItem.add", {
      eventId: EVENT,
      listId: LIST,
      text: "Port",
    })
    await enqueue("listItem.delete", {
      eventId: EVENT,
      listId: LIST,
      itemId: created._id,
    })

    expect(await pendingCount()).toBe(0)

    online()
    const result = await drain()
    expect(result.sent).toBe(0)
    expect(seen).toHaveLength(0)
  })

  it("keeps only the last save of a note", async () => {
    offline()
    await enqueue("note.save", { eventId: EVENT, text: "one" })
    await enqueue("note.save", { eventId: EVENT, text: "two" })
    await enqueue("note.save", { eventId: EVENT, text: "three" })

    expect(await pendingCount()).toBe(1)

    online()
    await drain()
    expect(seen).toHaveLength(1)
    expect(seen[0].body.text).toBe("three")
  })

  it("keeps only the last tick of one checkbox, but not across items", async () => {
    offline()
    await enqueue("listItem.check", { eventId: EVENT, listId: LIST, itemId: "a", checked: true })
    await enqueue("listItem.check", { eventId: EVENT, listId: LIST, itemId: "a", checked: false })
    await enqueue("listItem.check", { eventId: EVENT, listId: LIST, itemId: "b", checked: true })

    expect(await pendingCount()).toBe(2)
  })
})

describe("a write the server refuses", () => {
  // A poison op must never wedge the queue: everything behind it is somebody's
  // afternoon of work.
  it("is dropped, and the rest still go", async () => {
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "doomed" })
    await enqueue("comment.add", { eventId: EVENT, text: "fine" })

    noteNetworkSuccess()
    let call = 0
    global.fetch = vi.fn(async (url, params = {}) => {
      call++
      seen.push({ body: JSON.parse(params.body) })
      if (call === 1) return fails(404)()
      return { ok: true, status: 200, statusText: "", text: async () => "{}" }
    })

    const result = await drain()
    expect(result.sent).toBe(1)
    expect(result.dropped).toHaveLength(1)
    expect(result.dropped[0].reason).toMatch(/already deleted/)
    expect(await pendingCount()).toBe(0)
  })

  it("reports why, in words a member could act on", async () => {
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "x" })
    noteNetworkSuccess()
    handlers = {}
    global.fetch = vi.fn(async () => fails(403)())

    const result = await drain()
    expect(result.dropped[0].reason).toMatch(/permission/)
  })

  // A 5xx is the server's problem, not this request's — retrying is right.
  it("keeps a write the server failed on, rather than dropping it", async () => {
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "x" })
    noteNetworkSuccess()
    global.fetch = vi.fn(async () => fails(500)())

    const result = await drain()
    expect(result.dropped).toHaveLength(0)
    expect(await pendingCount()).toBe(1)
  })
})

describe("sign-out", () => {
  it("leaves no unsent work behind", async () => {
    offline()
    await enqueue("comment.add", { eventId: EVENT, text: "private" })
    await clearQueue()
    expect(await pendingCount()).toBe(0)
  })
})
