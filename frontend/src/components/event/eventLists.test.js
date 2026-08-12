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
  canManageEventLists,
  isVirtualList,
  canAssignListItems,
  assigneeMenuOptions,
  assigneeLabel,
  describeAssignCascade,
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
    const rows = flattenListItems([item("a", "b"), item("b", "a"), item("top")])

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
    const rows = flattenListItems([
      item("first"),
      item("second"),
      item("third"),
    ])

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
    const got = describeListDeletion({
      name: "Menu",
      items: [item("a"), item("b")],
    })
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

// The rule that decides whether the "Add list" button and the per-list
// rename/delete buttons appear. It lived in EventLists.vue until F19, where
// making the component serve a second panel meant lifting it out — which is
// also the first time it could be tested.
describe("canManageEventLists", () => {
  const user = { _id: "u1" }
  const owned = { ownerId: "u1" }
  const someoneElses = { ownerId: "u2" }
  const legacy = {} // no ownerId — created before anonymous creation was removed

  it("refuses a signed-out visitor outright", () => {
    expect(
      canManageEventLists({
        authUser: null,
        event: owned,
        canManageUsers: true,
        canInvite: true,
      })
    ).toBe(false)
  })

  it("lets an admin manage someone else's event", () => {
    expect(
      canManageEventLists({
        authUser: user,
        event: someoneElses,
        canManageUsers: true,
        canInvite: true,
      })
    ).toBe(true)
  })

  it("lets the planner manage their own", () => {
    expect(
      canManageEventLists({
        authUser: user,
        event: owned,
        canManageUsers: false,
        canInvite: true,
      })
    ).toBe(true)
  })

  it("refuses a member who isn't the planner", () => {
    expect(
      canManageEventLists({
        authUser: user,
        event: someoneElses,
        canManageUsers: false,
        canInvite: true,
      })
    ).toBe(false)
  })

  it("falls back to member+ on an ownerless legacy event", () => {
    expect(
      canManageEventLists({
        authUser: user,
        event: legacy,
        canManageUsers: false,
        canInvite: true,
      })
    ).toBe(true)
    expect(
      canManageEventLists({
        authUser: user,
        event: legacy,
        canManageUsers: false,
        canInvite: false,
      })
    ).toBe(false)
  })

  it("treats ownerId 0 as ownerless, not as a user id", () => {
    expect(
      canManageEventLists({
        authUser: user,
        event: { ownerId: 0 },
        canManageUsers: false,
        canInvite: true,
      })
    ).toBe(true)
  })
})

// --- Assignment (N1) ---

describe("isVirtualList", () => {
  it("is true only for the flag the server sets", () => {
    expect(isVirtualList({ virtual: true })).toBe(true)
    expect(isVirtualList({ virtual: false })).toBe(false)
    expect(isVirtualList({ name: "Assigned" })).toBe(false)
    expect(isVirtualList(null)).toBe(false)
    expect(isVirtualList(undefined)).toBe(false)
  })

  // The name is not the discriminator: a member may have a real list of their
  // own called "Assigned", and it must stay fully editable.
  it("does not treat a viewer's own list named Assigned as derived", () => {
    expect(isVirtualList({ _id: "abc", name: "Assigned" })).toBe(false)
  })
})

describe("canAssignListItems", () => {
  const authUser = { _id: "u1" }

  it("allows member and up, refuses guests and the signed out", () => {
    expect(canAssignListItems({ authUser, canInvite: true })).toBe(true)
    expect(canAssignListItems({ authUser, canInvite: false })).toBe(false)
    expect(canAssignListItems({ authUser: null, canInvite: true })).toBe(false)
  })
})

describe("assigneeMenuOptions", () => {
  const bart = { _id: "u2", firstName: "Bart", lastName: "Renfrew" }
  const ada = { _id: "u3", firstName: "Ada", lastName: "King", nickname: "Ada" }

  it("puts Unassigned first, carrying a null id", () => {
    const options = assigneeMenuOptions([bart, ada], null)
    expect(options[0]).toMatchObject({ id: null, name: "Unassigned" })
    expect(options.map((o) => o.id)).toEqual([null, "u2", "u3"])
  })

  it("marks exactly one option selected", () => {
    const options = assigneeMenuOptions([bart, ada], "u2")
    expect(options.filter((o) => o.selected).map((o) => o.id)).toEqual(["u2"])
  })

  it("selects Unassigned when nothing is assigned", () => {
    for (const current of [null, undefined]) {
      const options = assigneeMenuOptions([bart, ada], current)
      expect(options[0].selected).toBe(true)
    }
  })

  it("prefers a nickname, matching the server's DisplayName", () => {
    const [, , adaRow] = assigneeMenuOptions([bart, ada], null)
    expect(adaRow.name).toBe("Ada")
    const [, bartRow] = assigneeMenuOptions([bart, ada], null)
    expect(bartRow.name).toBe("Bart Renfrew")
  })

  // A blank row would render as an unclickable gap in the menu.
  it("never produces a blank name", () => {
    const [, row] = assigneeMenuOptions([{ _id: "u9" }], null)
    expect(row.name).toBe("Member")
  })

  it("survives no assignees at all", () => {
    expect(assigneeMenuOptions(undefined, null)).toHaveLength(1)
    expect(assigneeMenuOptions([], null)).toHaveLength(1)
  })
})

describe("assigneeLabel", () => {
  it("names the assignee, or nobody", () => {
    expect(assigneeLabel({ assigneeName: "Bart" })).toBe("For Bart")
    expect(assigneeLabel({})).toBe(null)
    expect(assigneeLabel(null)).toBe(null)
  })

  // Read off the item, not looked up: someone who assigned an entry and then
  // RSVP'd "no" drops out of the picker while still holding the entry, and the
  // work must not silently look unclaimed.
  it("still names someone who is no longer in the picker", () => {
    expect(assigneeLabel({ assigneeId: "gone", assigneeName: "Bart" })).toBe(
      "For Bart"
    )
  })
})

describe("describeAssignCascade", () => {
  it("states the count, which is the only place the number is said", () => {
    expect(
      describeAssignCascade({ count: 9, assigned: true, assigneeName: "Bart" })
    ).toBe("Assigned 9 entries to Bart.")
  })

  it("reads a clear differently from a hand-over", () => {
    expect(describeAssignCascade({ count: 9 })).toBe("Cleared 9 entries.")
    expect(describeAssignCascade({ count: 9, assigned: false })).toBe(
      "Cleared 9 entries."
    )
  })

  // The case that matters: the picker is fetched with the Lists tab, so the
  // assignee can be unnameable. Inferring "cleared" from the missing name would
  // tell the reader the exact opposite of what happened.
  it("loses the name but never the verb", () => {
    expect(describeAssignCascade({ count: 9, assigned: true })).toBe(
      "Assigned 9 entries."
    )
    expect(
      describeAssignCascade({ count: 9, assigned: true, assigneeName: null })
    ).toBe("Assigned 9 entries.")
  })

  it("pluralises", () => {
    expect(
      describeAssignCascade({ count: 1, assigned: true, assigneeName: "Ada" })
    ).toBe("Assigned 1 entry to Ada.")
    expect(
      describeAssignCascade({ count: 2, assigned: true, assigneeName: "Ada" })
    ).toBe("Assigned 2 entries to Ada.")
    expect(describeAssignCascade({ count: 1 })).toBe("Cleared 1 entry.")
  })

  it("never renders NaN or undefined for a missing count", () => {
    for (const args of [undefined, {}, { count: null }, { count: "x" }]) {
      expect(describeAssignCascade(args)).toBe("Cleared 0 entries.")
    }
  })
})
