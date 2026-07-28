import { describe, expect, it } from "vitest"
import {
  groupComments,
  replyCount,
  replyCountLabel,
  threadTitle,
} from "./commentThreads"

const comment = (id, overrides = {}) => ({
  _id: id,
  text: `text ${id}`,
  authorName: "Someone",
  ...overrides,
})

describe("groupComments", () => {
  it("keeps plain comments in the top-level stream", () => {
    const comments = [comment("a"), comment("b")]
    const { topLevel, repliesByThreadId } = groupComments(comments)

    expect(topLevel.map((c) => c._id)).toEqual(["a", "b"])
    expect(repliesByThreadId).toEqual({})
  })

  it("nests replies under their thread root and keeps roots at top level", () => {
    const comments = [
      comment("plain"),
      comment("root", { isThread: true }),
      comment("r1", { threadId: "root" }),
      comment("r2", { threadId: "root" }),
    ]
    const { topLevel, repliesByThreadId } = groupComments(comments)

    expect(topLevel.map((c) => c._id)).toEqual(["plain", "root"])
    expect(repliesByThreadId.root.map((c) => c._id)).toEqual(["r1", "r2"])
  })

  it("preserves the server's chronological ordering of replies", () => {
    const comments = [
      comment("root", { isThread: true }),
      comment("first", { threadId: "root" }),
      comment("second", { threadId: "root" }),
      comment("third", { threadId: "root" }),
    ]
    const { repliesByThreadId } = groupComments(comments)

    expect(repliesByThreadId.root.map((c) => c._id)).toEqual([
      "first",
      "second",
      "third",
    ])
  })

  it("separates replies belonging to different threads", () => {
    const comments = [
      comment("rootA", { isThread: true }),
      comment("rootB", { isThread: true }),
      comment("a1", { threadId: "rootA" }),
      comment("b1", { threadId: "rootB" }),
      comment("a2", { threadId: "rootA" }),
    ]
    const { repliesByThreadId } = groupComments(comments)

    expect(repliesByThreadId.rootA.map((c) => c._id)).toEqual(["a1", "a2"])
    expect(repliesByThreadId.rootB.map((c) => c._id)).toEqual(["b1"])
  })

  // The server strips members-only roots for guests. Any reply that survives the
  // strip must not resurface at top level — that would both read as a
  // non-sequitur and reveal that a hidden thread exists.
  it("drops orphaned replies whose root was filtered out server-side", () => {
    const comments = [
      comment("plain"),
      comment("orphan", { threadId: "hidden-root" }),
    ]
    const { topLevel, repliesByThreadId } = groupComments(comments)

    expect(topLevel.map((c) => c._id)).toEqual(["plain"])
    expect(repliesByThreadId).toEqual({})
  })

  it("ignores replies pointing at a top-level comment that isn't a thread", () => {
    const comments = [
      comment("notathread"),
      comment("stray", { threadId: "notathread" }),
    ]
    const { topLevel, repliesByThreadId } = groupComments(comments)

    expect(topLevel.map((c) => c._id)).toEqual(["notathread"])
    expect(repliesByThreadId).toEqual({})
  })

  it("tolerates missing or non-array input", () => {
    expect(groupComments(undefined)).toEqual({ topLevel: [], repliesByThreadId: {} })
    expect(groupComments(null)).toEqual({ topLevel: [], repliesByThreadId: {} })
    expect(groupComments([])).toEqual({ topLevel: [], repliesByThreadId: {} })
  })
})

describe("replyCount", () => {
  it("counts a thread's replies and returns 0 for an unknown thread", () => {
    const { repliesByThreadId } = groupComments([
      comment("root", { isThread: true }),
      comment("r1", { threadId: "root" }),
      comment("r2", { threadId: "root" }),
    ])

    expect(replyCount(repliesByThreadId, "root")).toBe(2)
    expect(replyCount(repliesByThreadId, "nope")).toBe(0)
  })
})

describe("replyCountLabel", () => {
  it("pluralizes correctly", () => {
    expect(replyCountLabel(0)).toBe("No replies yet")
    expect(replyCountLabel(1)).toBe("1 reply")
    expect(replyCountLabel(2)).toBe("2 replies")
  })
})

describe("threadTitle", () => {
  it("uses the comment text as-is when it's short", () => {
    expect(threadTitle("Distilleries to visit")).toBe("Distilleries to visit")
  })

  it("collapses whitespace and newlines onto one line", () => {
    expect(threadTitle("Distilleries\n\n  to   visit")).toBe("Distilleries to visit")
  })

  it("truncates long text at a word boundary", () => {
    const long =
      "We should think carefully about which distilleries are worth the drive on Saturday"
    const title = threadTitle(long, 40)

    expect(title.length).toBeLessThanOrEqual(41) // 40 + the ellipsis
    expect(title.endsWith("…")).toBe(true)
    expect(title).not.toMatch(/\s…$/) // no dangling space before the ellipsis
    expect(long.startsWith(title.slice(0, -1))).toBe(true)
  })

  it("falls back to a hard cut when there's no usable word boundary", () => {
    const title = threadTitle("a".repeat(100), 10)
    expect(title).toBe(`${"a".repeat(10)}…`)
  })

  it("handles empty and missing text", () => {
    expect(threadTitle("")).toBe("")
    expect(threadTitle(undefined)).toBe("")
    expect(threadTitle(null)).toBe("")
  })
})
