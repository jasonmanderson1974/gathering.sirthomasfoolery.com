import { describe, it, expect } from "vitest"
import {
  MAX_ITEM_DEPTH,
  flattenListItems,
  canAddChild,
  checkStateLabel,
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
