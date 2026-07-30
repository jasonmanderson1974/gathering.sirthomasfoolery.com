import { describe, it, expect } from "vitest"
import {
  MAX_ITEM_DEPTH,
  flattenListItems,
  canAddChild,
  checkStateLabel,
  ORDER_STEP,
  orderBetween,
  resolveDrop,
  countDescendants,
  describeListDeletion,
  describeItemDeletion,
} from "./eventLists"

const item = (id, parentId = null, overrides = {}) => ({
  _id: id,
  text: id,
  parentId,
  ...overrides,
})

/** The ids of the returned rows, which is what the assertions mostly care about. */
const idsOf = (rows) => rows.map((r) => r.item._id)

describe("flattenListItems", () => {
  it("renders items written before nesting existed at the top level", () => {
    const rows = flattenListItems([
      { _id: "a", text: "Hotdogs" },
      { _id: "b", text: "Hamburgers" },
    ])

    expect(idsOf(rows)).toEqual(["a", "b"])
    expect(rows.every((r) => r.depth === 0)).toBe(true)
    expect(rows.every((r) => r.hasChildren === false)).toBe(true)
  })

  it("walks depth-first, parents before children, siblings in insertion order", () => {
    const rows = flattenListItems([
      item("mains"),
      item("salad", "mains"),
      item("hotdogs", "mains"),
      item("mustard", "hotdogs"),
      item("drinks"),
    ])

    expect(idsOf(rows)).toEqual([
      "mains",
      "salad",
      "hotdogs",
      "mustard",
      "drinks",
    ])
    expect(rows.map((r) => r.depth)).toEqual([0, 1, 1, 2, 0])
  })

  it("flags which rows have children", () => {
    const rows = flattenListItems([item("mains"), item("hotdogs", "mains")])

    expect(rows[0].hasChildren).toBe(true)
    expect(rows[1].hasChildren).toBe(false)
  })

  it("hides the descendants of a collapsed item but keeps the item itself", () => {
    const rows = flattenListItems(
      [
        item("mains"),
        item("hotdogs", "mains"),
        item("mustard", "hotdogs"),
        item("drinks"),
      ],
      ["mains"]
    )

    expect(idsOf(rows)).toEqual(["mains", "drinks"])
    expect(rows[0].collapsed).toBe(true)
    expect(rows[0].hasChildren).toBe(true)
  })

  it("collapses only the subtree asked for", () => {
    const rows = flattenListItems(
      [item("mains"), item("hotdogs", "mains"), item("mustard", "hotdogs")],
      ["hotdogs"]
    )

    expect(idsOf(rows)).toEqual(["mains", "hotdogs"])
    expect(rows[1].collapsed).toBe(true)
  })

  it("never marks a childless item as collapsed", () => {
    const rows = flattenListItems([item("mains")], ["mains"])

    expect(rows[0].collapsed).toBe(false)
  })

  // Its parent was deleted between someone else's read and write. Dropping it
  // would hide an entry a person actually typed.
  it("renders an item whose parent is gone at the top level", () => {
    const rows = flattenListItems([
      item("mains"),
      item("orphan", "deleted-parent"),
    ])

    expect(idsOf(rows)).toEqual(["mains", "orphan"])
    expect(rows[1].depth).toBe(0)
  })

  // The app cannot produce this; a hand-edited document could, and rendering
  // must not hang.
  it("terminates on a cycle, emitting each item at most once", () => {
    const rows = flattenListItems([
      item("a", "b"),
      item("b", "a"),
      item("top"),
    ])

    expect(idsOf(rows)).toEqual(["top"])
  })

  it("handles an empty or missing list", () => {
    expect(flattenListItems([])).toEqual([])
    expect(flattenListItems(undefined)).toEqual([])
    expect(flattenListItems(null, null)).toEqual([])
  })
})

describe("canAddChild", () => {
  it("allows children down to the second level and no further", () => {
    expect(canAddChild(0)).toBe(true)
    expect(canAddChild(1)).toBe(true)
    expect(canAddChild(2)).toBe(false)
  })

  it("stops one short of the depth cap", () => {
    expect(canAddChild(MAX_ITEM_DEPTH - 1)).toBe(false)
  })
})

describe("checkStateLabel", () => {
  it("names whoever ticked the box", () => {
    expect(checkStateLabel({ checked: true, checkedByName: "Ada" })).toBe(
      "Checked by Ada"
    )
  })

  // Clearing a tick is a change of state, not a return to untouched.
  it("names whoever cleared it", () => {
    expect(checkStateLabel({ checked: false, checkedByName: "Bart" })).toBe(
      "Unchecked by Bart"
    )
  })

  it("says nothing about an item nobody has touched", () => {
    expect(checkStateLabel({ text: "Bring ice" })).toBeNull()
    expect(checkStateLabel(undefined)).toBeNull()
  })
})

describe("flattenListItems ordering", () => {
  it("sorts each sibling group by order", () => {
    const rows = flattenListItems([
      item("b", null, { order: 2048 }),
      item("c", null, { order: 3072 }),
      item("a", null, { order: 1024 }),
    ])

    expect(idsOf(rows)).toEqual(["a", "b", "c"])
  })

  it("keeps insertion order for entries written before ordering existed", () => {
    // No order field anywhere: every entry ties at 0 and the stable sort has to
    // leave them exactly as the API returned them.
    const rows = flattenListItems([item("first"), item("second"), item("third")])

    expect(idsOf(rows)).toEqual(["first", "second", "third"])
  })

  it("sorts ordered entries below legacy ones, ties keeping array order", () => {
    const rows = flattenListItems([
      item("legacyA"),
      item("legacyB"),
      item("ordered", null, { order: 1024 }),
    ])

    expect(idsOf(rows)).toEqual(["legacyA", "legacyB", "ordered"])
  })

  it("orders each sibling group independently of the others", () => {
    const rows = flattenListItems([
      item("mains", null, { order: 1024 }),
      item("hotdogs", "mains", { order: 2048 }),
      item("salad", "mains", { order: 1024 }),
      item("drinks", null, { order: 2048 }),
    ])

    // A child's order competes only with its siblings, never with a top-level
    // entry that happens to share the number.
    expect(idsOf(rows)).toEqual(["mains", "salad", "hotdogs", "drinks"])
  })
})

describe("orderBetween", () => {
  it("gives the first entry on an empty list the step", () => {
    expect(orderBetween(null, null)).toBe(ORDER_STEP)
  })

  it("steps below the first entry when dropped above everything", () => {
    expect(orderBetween(null, 1024)).toBe(0)
    expect(orderBetween(null, 0)).toBe(-ORDER_STEP)
  })

  it("steps past the last entry when dropped below everything", () => {
    expect(orderBetween(3072, null)).toBe(3072 + ORDER_STEP)
  })

  it("takes the midpoint between two neighbours", () => {
    expect(orderBetween(1024, 2048)).toBe(1536)
    expect(orderBetween(0, 1024)).toBe(512)
  })

  it("falls back to prev rather than NaN when the gap is exhausted", () => {
    // Adjacent doubles: there is no value between them.
    const prev = 1024
    const next = 1024 + Number.EPSILON * 1024
    expect(orderBetween(prev, next)).toBe(prev)
    expect(orderBetween(5, 5)).toBe(5)
  })

  it("never returns NaN for any pair", () => {
    for (const [prev, next] of [
      [null, null],
      [0, 0],
      [-1024, 1024],
      [undefined, 512],
      [512, undefined],
    ]) {
      expect(Number.isNaN(orderBetween(prev, next))).toBe(false)
    }
  })
})

/** Rows as the component renders them, for the drop tests. */
const rowsFrom = (items, collapsedIds = []) =>
  flattenListItems(items, collapsedIds)

describe("resolveDrop", () => {
  // a(1024) b(2048) c(3072), all top-level.
  const flatItems = [
    item("a", null, { order: 1024 }),
    item("b", null, { order: 2048 }),
    item("c", null, { order: 3072 }),
  ]

  it("resolves a drop at the top of a list", () => {
    const rows = rowsFrom(flatItems)
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 2,
      newIndex: 0,
      sameList: true,
      draggedItem: flatItems[2],
    })

    expect(got).toEqual({ parentId: null, prevOrder: null, nextOrder: 1024 })
    expect(orderBetween(got.prevOrder, got.nextOrder)).toBe(0)
  })

  it("resolves a drop at the bottom of a list", () => {
    const rows = rowsFrom(flatItems)
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 0,
      newIndex: 2,
      sameList: true,
      draggedItem: flatItems[0],
    })

    expect(got).toEqual({ parentId: null, prevOrder: 3072, nextOrder: null })
  })

  it("accounts for the splice shift when dragging downward in one list", () => {
    // a moves between b and c. With a lifted out the remaining rows are [b, c],
    // so index 1 means "after b, before c" — not "after a".
    const rows = rowsFrom(flatItems)
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 0,
      newIndex: 1,
      sameList: true,
      draggedItem: flatItems[0],
    })

    expect(got).toEqual({ parentId: null, prevOrder: 2048, nextOrder: 3072 })
    expect(orderBetween(got.prevOrder, got.nextOrder)).toBe(2560)
  })

  it("resolves an upward drag between two entries", () => {
    const rows = rowsFrom(flatItems)
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 2,
      newIndex: 1,
      sameList: true,
      draggedItem: flatItems[2],
    })

    expect(got).toEqual({ parentId: null, prevOrder: 1024, nextOrder: 2048 })
  })

  // mains > [salad(1024), hotdogs(2048)], then drinks.
  const nestedItems = [
    item("mains", null, { order: 1024 }),
    item("salad", "mains", { order: 1024 }),
    item("hotdogs", "mains", { order: 2048 }),
    item("drinks", null, { order: 2048 }),
  ]

  it("keeps the parent when reordering among siblings", () => {
    const rows = rowsFrom(nestedItems)
    // hotdogs (row 2) moves above salad (row 1).
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 2,
      newIndex: 1,
      sameList: true,
      draggedItem: nestedItems[2],
    })

    expect(got.parentId).toBe("mains")
    expect(got.prevOrder).toBe(null)
    expect(got.nextOrder).toBe(1024)
  })

  it("keeps the parent when reordering the other way", () => {
    const rows = rowsFrom(nestedItems)
    // salad (row 1) moves below hotdogs. Lifting it out leaves
    // [mains, hotdogs, drinks], so index 2 is after hotdogs.
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 1,
      newIndex: 2,
      sameList: true,
      draggedItem: nestedItems[1],
    })

    expect(got.parentId).toBe("mains")
    expect(got.prevOrder).toBe(2048)
    expect(got.nextOrder).toBe(null)
  })

  it("flattens a sub-entry dragged clear of its parent", () => {
    const rows = rowsFrom(nestedItems)
    // salad dragged to the very top, above mains.
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 1,
      newIndex: 0,
      sameList: true,
      draggedItem: nestedItems[1],
    })

    expect(got.parentId).toBe(null)
    expect(got.prevOrder).toBe(null)
    expect(got.nextOrder).toBe(1024)
  })

  it("resolves a release inside a foreign subtree to after that subtree", () => {
    // "drinks" released between salad and hotdogs — inside mains' children,
    // which is not its own parent's block. It cannot nest, so it lands at the
    // top level after the whole of mains.
    const rows = rowsFrom(nestedItems)
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 3,
      newIndex: 2,
      sameList: true,
      draggedItem: nestedItems[3],
    })

    expect(got.parentId).toBe(null)
    expect(got.prevOrder).toBe(1024) // mains
    expect(got.nextOrder).toBe(null) // nothing top-level after it
  })

  it("drops into an empty list at the step", () => {
    const got = resolveDrop({
      sourceRows: rowsFrom(flatItems),
      targetRows: [],
      oldIndex: 0,
      newIndex: 0,
      sameList: false,
      draggedItem: flatItems[0],
    })

    expect(got).toEqual({ parentId: null, prevOrder: null, nextOrder: null })
    expect(orderBetween(got.prevOrder, got.nextOrder)).toBe(ORDER_STEP)
  })

  it("drops between entries on another list, ignoring the source index", () => {
    const got = resolveDrop({
      sourceRows: rowsFrom(nestedItems),
      targetRows: rowsFrom(flatItems),
      oldIndex: 1,
      newIndex: 1,
      sameList: false,
      draggedItem: nestedItems[1],
    })

    // A sub-entry moved across lists pops out to the top level.
    expect(got).toEqual({ parentId: null, prevOrder: 1024, nextOrder: 2048 })
  })

  it("treats a dragged parent as one row, its subtree hidden", () => {
    // While dragging, the component collapses the dragged entry, so its
    // children are not rows at all and cannot skew the indices.
    const rows = rowsFrom(nestedItems, ["mains"])
    expect(idsOf(rows)).toEqual(["mains", "drinks"])

    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 0,
      newIndex: 1,
      sameList: true,
      draggedItem: nestedItems[0],
    })

    expect(got).toEqual({ parentId: null, prevOrder: 2048, nextOrder: null })
  })

  it("survives an out-of-range index rather than producing NaN", () => {
    const rows = rowsFrom(flatItems)
    const got = resolveDrop({
      sourceRows: rows,
      targetRows: rows,
      oldIndex: 0,
      newIndex: 99,
      sameList: true,
      draggedItem: flatItems[0],
    })

    expect(Number.isNaN(orderBetween(got.prevOrder, got.nextOrder))).toBe(false)
  })

  it("falls back to the top level when the parent is no longer rendered", () => {
    // The parent was deleted by someone else mid-drag; the orphan resolves at
    // the top level rather than throwing.
    const orphanRows = rowsFrom([item("orphan", "gone", { order: 1024 })])
    const got = resolveDrop({
      sourceRows: orphanRows,
      targetRows: orphanRows,
      oldIndex: 0,
      newIndex: 0,
      sameList: true,
      draggedItem: { _id: "orphan", parentId: "gone", order: 1024 },
    })

    expect(got.parentId).toBe(null)
  })
})

describe("countDescendants", () => {
  const tree = [
    item("mains"),
    item("hotdogs", "mains"),
    item("mustard", "hotdogs"),
    item("salad", "mains"),
    item("drinks"),
  ]

  it("counts the whole subtree, not just direct children", () => {
    expect(countDescendants(tree, "mains")).toBe(3)
  })

  it("counts one level", () => {
    expect(countDescendants(tree, "hotdogs")).toBe(1)
  })

  it("counts nothing for a leaf", () => {
    expect(countDescendants(tree, "drinks")).toBe(0)
    expect(countDescendants(tree, "mustard")).toBe(0)
  })

  it("terminates on a cycle", () => {
    const cyclic = [item("a", "b"), item("b", "a")]
    expect(countDescendants(cyclic, "a")).toBe(1)
  })

  it("survives an empty or missing list", () => {
    expect(countDescendants([], "a")).toBe(0)
    expect(countDescendants(undefined, "a")).toBe(0)
  })
})

describe("describeListDeletion", () => {
  it("names the list and counts what goes with it", () => {
    const got = describeListDeletion({ name: "Menu", items: [item("a"), item("b")] })
    expect(got.title).toBe('Delete "Menu"?')
    expect(got.body).toBe("This removes the list and its 2 entries.")
  })

  it("says entry, singular, for one", () => {
    const got = describeListDeletion({ name: "Menu", items: [item("a")] })
    expect(got.body).toBe("This removes the list and its 1 entry.")
  })

  it("says so when the list is empty", () => {
    expect(describeListDeletion({ name: "Menu", items: [] }).body).toBe(
      "This list is empty."
    )
  })

  it("truncates a very long name", () => {
    const got = describeListDeletion({ name: "x".repeat(200), items: [] })
    expect(got.title.length).toBeLessThan(100)
    expect(got.title).toContain("…")
  })
})

describe("describeItemDeletion", () => {
  const tree = [
    item("mains", null, { text: "Mains" }),
    item("hotdogs", "mains", { text: "Hotdogs" }),
    item("mustard", "hotdogs", { text: "Mustard" }),
  ]

  it("names the entry and warns about the subtree", () => {
    const got = describeItemDeletion(tree, "mains")
    expect(got.title).toBe('Delete "Mains"?')
    expect(got.body).toContain("2 sub-entries")
    expect(got.body).toContain("other people")
  })

  it("says sub-entry, singular, for one", () => {
    expect(describeItemDeletion(tree, "hotdogs").body).toContain("1 sub-entry")
  })

  it("gives a leaf no body at all", () => {
    const got = describeItemDeletion(tree, "mustard")
    expect(got.title).toBe('Delete "Mustard"?')
    expect(got.body).toBe("")
  })

  it("truncates long entry text", () => {
    const long = [item("x", null, { text: "y".repeat(200) })]
    const got = describeItemDeletion(long, "x")
    expect(got.title.length).toBeLessThan(100)
    expect(got.title).toContain("…")
  })
})
