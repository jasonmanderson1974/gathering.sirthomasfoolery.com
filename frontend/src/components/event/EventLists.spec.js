/*
 * Assigning a checklist entry to a member, and the derived "Assigned" list
 * (TODO3 N1).
 *
 * The rule this feature turns on is a split one — a guest SEES who each entry is
 * for and cannot change it — and a split rule is exactly the kind that reads
 * correct in a template and renders wrong. `v-if="canAssign"` on the wrong
 * element hides the assignment from guests entirely, and nothing else in the
 * repo would notice: the node tier renders nothing, and `check:routes` signs in
 * as one account.
 *
 * The other half is the derived list. Its rows are the club's entries showing
 * inside a panel that passes `viewerOwnsAll`, which collapses every per-entry
 * right to always-allowed — so a missing guard offers a member the Edit and
 * Remove buttons on a shared entry from inside their own private notebook.
 *
 * `src/test/setup.dom.js` fails any test in this tier on an unasserted
 * `[Vue warn]`, which is the half that catches a prop declared wrong.
 */
import { afterEach, describe, expect, it } from "vitest"
import EventLists from "@/components/event/EventLists.vue"
import { cleanupDom, mountApp } from "@/test/mount"

const MEMBER = { _id: "u1", firstName: "Bilbo", lastName: "Baggins" }
const BART = { _id: "u2", firstName: "Bart", lastName: "Renfrew" }
const ADA = { _id: "u3", firstName: "Ada", lastName: "King", nickname: "Ada" }

const SHARED_LIST_ID = "l1"

/** A shared checklist holding one entry, assigned or not. */
const checklist = (itemOverrides = {}) => ({
  _id: SHARED_LIST_ID,
  name: "Menu",
  kind: "checklist",
  items: [
    {
      _id: "i1",
      text: "Bring the port",
      order: 1024,
      userId: MEMBER._id,
      authorName: "Bilbo Baggins",
      ...itemOverrides,
    },
  ],
})

/**
 * The derived list as `GET /my-lists` builds it: the nil id, the virtual flag,
 * and each row carrying the shared list it really lives on.
 */
const assignedList = (itemOverrides = {}) => ({
  _id: "000000000000000000000000",
  name: "Assigned",
  kind: "checklist",
  virtual: true,
  items: [
    {
      _id: "i1",
      text: "Bring the port",
      order: 0,
      sourceListId: SHARED_LIST_ID,
      sourceListName: "Menu",
      assigneeId: MEMBER._id,
      assigneeName: "Bilbo Baggins",
      ...itemOverrides,
    },
  ],
})

const mountLists = async (props = {}, authUser = MEMBER, role = "member") => {
  const wrapper = await mountApp(EventLists, {
    props: { lists: [checklist()], ...props },
    state: { authUser: authUser && { ...authUser, role } },
  })
  await settle(wrapper)
  return wrapper
}

async function settle(wrapper) {
  for (let i = 0; i < 4; i++) {
    await Promise.resolve()
    await wrapper.vm.$nextTick()
  }
}

/** Opens the one list in the panel — lists render collapsed. */
async function expandList(wrapper, listId = SHARED_LIST_ID) {
  wrapper.vm.expandedLists = [listId]
  await settle(wrapper)
}

const buttonTitled = (pattern) =>
  [...document.querySelectorAll("button")].find((b) =>
    pattern.test(b.getAttribute("title") ?? "")
  )

/** The whole row's text, which is where the byline lives. */
const rowText = () =>
  document.querySelector("[data-item-id]")?.textContent.replace(/\s+/g, " ") ??
  ""

afterEach(cleanupDom)

describe("EventLists — the assignee control", () => {
  it("offers the picker to a member on a checklist", async () => {
    const wrapper = await mountLists({ canAssign: true, assignees: [BART] })
    await expandList(wrapper)

    expect(buttonTitled(/assign to a member/i)).toBeTruthy()
  })

  // The split rule, both halves in one test: no control, but the name is there.
  it("shows a guest who an entry is for, and no way to change it", async () => {
    const wrapper = await mountLists(
      { canAssign: false, lists: [checklist({ assigneeName: "Bart Renfrew" })] },
      MEMBER,
      "guest"
    )
    await expandList(wrapper)

    expect(buttonTitled(/assign|assigned to/i)).toBeFalsy()
    expect(rowText()).toContain("For Bart Renfrew")
  })

  it("names the assignee on the byline for a member too", async () => {
    const wrapper = await mountLists({
      canAssign: true,
      assignees: [BART],
      lists: [checklist({ assigneeId: BART._id, assigneeName: "Bart Renfrew" })],
    })
    await expandList(wrapper)

    expect(rowText()).toContain("For Bart Renfrew")
    // Alongside the author, not instead of it.
    expect(rowText()).toContain("Bilbo Baggins")
  })

  // Assignability follows the list's KIND, exactly as the checkbox does.
  it("offers nothing on a list that is not a checklist", async () => {
    const wrapper = await mountLists({
      canAssign: true,
      assignees: [BART],
      lists: [{ ...checklist(), kind: "text" }],
    })
    await expandList(wrapper)

    expect(buttonTitled(/assign to a member/i)).toBeFalsy()
  })

  // Assigning a parent overwrites everyone below it. That is the intended
  // behaviour and a nasty surprise if it is invisible, and a confirm dialog on
  // every assign would be worse than the problem — so the count has to be on the
  // control before the click.
  it("warns on the control that assigning a parent is a bulk action", async () => {
    const nested = {
      _id: SHARED_LIST_ID,
      name: "Gear",
      kind: "checklist",
      items: [
        { _id: "p1", text: "Sleeping", order: 1024, userId: MEMBER._id },
        { _id: "c1", text: "1 Tent", parentId: "p1", order: 1024, userId: MEMBER._id },
        { _id: "c2", text: "2 Cots", parentId: "p1", order: 2048, userId: MEMBER._id },
        { _id: "g1", text: "Pump", parentId: "c1", order: 1024, userId: MEMBER._id },
        { _id: "solo", text: "Cooking", order: 2048, userId: MEMBER._id },
      ],
    }
    const wrapper = await mountLists({
      canAssign: true,
      assignees: [BART],
      lists: [nested],
    })
    await expandList(wrapper)

    const titles = [...document.querySelectorAll("button")]
      .map((b) => b.getAttribute("title") || "")
      .filter((t) => /assign to a member/i.test(t))

    // The parent counts its whole subtree, not just its direct children.
    expect(titles).toContain(
      "Assign to a member — also applies to 3 sub-entries"
    )
    // Singular where it should be.
    expect(titles).toContain("Assign to a member — also applies to 1 sub-entry")
    // A leaf says nothing extra at all.
    expect(titles).toContain("Assign to a member")
  })

  it("emits assign-item with the chosen member", async () => {
    const wrapper = await mountLists({
      canAssign: true,
      assignees: [BART, ADA],
    })
    await expandList(wrapper)

    buttonTitled(/assign to a member/i).click()
    await settle(wrapper)

    // The menu teleports to document.body, which is why mountApp attaches.
    const rows = [...document.querySelectorAll(".v-list-item")]
    expect(rows.map((r) => r.textContent.trim())).toEqual([
      "Unassigned",
      "Bart Renfrew",
      "Ada",
    ])

    rows[1].click()
    await settle(wrapper)

    expect(wrapper.emitted("assign-item")).toEqual([
      [{ listId: SHARED_LIST_ID, itemId: "i1", assigneeId: BART._id }],
    ])
  })

  it("emits a null assigneeId to unassign", async () => {
    const wrapper = await mountLists({
      canAssign: true,
      assignees: [BART],
      lists: [checklist({ assigneeId: BART._id, assigneeName: "Bart Renfrew" })],
    })
    await expandList(wrapper)

    buttonTitled(/assigned to bart/i).click()
    await settle(wrapper)

    const rows = [...document.querySelectorAll(".v-list-item")]
    rows[0].click()
    await settle(wrapper)

    expect(wrapper.emitted("assign-item")[0][0].assigneeId).toBeNull()
  })

  // Re-picking the name already on the entry is a no-op, not a round trip.
  it("does not emit when the selection has not changed", async () => {
    const wrapper = await mountLists({
      canAssign: true,
      assignees: [BART],
      lists: [checklist({ assigneeId: BART._id, assigneeName: "Bart Renfrew" })],
    })
    await expandList(wrapper)

    buttonTitled(/assigned to bart/i).click()
    await settle(wrapper)

    const rows = [...document.querySelectorAll(".v-list-item")]
    rows[1].click()
    await settle(wrapper)

    expect(wrapper.emitted("assign-item")).toBeUndefined()
  })
})

describe("EventLists — the derived Assigned list", () => {
  /** The private panel's props, which is the only place a virtual list appears. */
  const privatePanel = (lists) => ({
    lists,
    canManage: true,
    viewerOwnsAll: true,
    collaborative: true,
    title: "My Lists",
  })

  it("offers no way to edit, remove, nest or reorder its entries", async () => {
    const wrapper = await mountLists(
      privatePanel([assignedList()]),
      MEMBER,
      "member"
    )
    await expandList(wrapper, "000000000000000000000000")

    // `viewerOwnsAll` would otherwise make all three of these appear.
    expect(buttonTitled(/edit entry/i)).toBeFalsy()
    expect(buttonTitled(/remove entry/i)).toBeFalsy()
    expect(buttonTitled(/add sub-entry/i)).toBeFalsy()
    // Nor the list-level controls, nor a composer to add to it.
    expect(buttonTitled(/rename list/i)).toBeFalsy()
    expect(buttonTitled(/delete list/i)).toBeFalsy()
    expect(
      [...document.querySelectorAll("input")].some(
        (i) => i.getAttribute("placeholder") === "Add an entry…"
      )
    ).toBe(false)
  })

  it("keeps the checkbox — ticking it is the whole point", async () => {
    const wrapper = await mountLists(
      privatePanel([assignedList()]),
      MEMBER,
      "member"
    )
    await expandList(wrapper, "000000000000000000000000")

    const checkbox = buttonTitled(/check off/i)
    expect(checkbox).toBeFalsy() // it is an icon, not a button
    const icon = [...document.querySelectorAll(".v-icon")].find(
      (el) => el.getAttribute("title") === "Check off"
    )
    expect(icon).toBeTruthy()

    icon.click()
    await settle(wrapper)

    // The source list id is what routes the write to the shared entry rather
    // than to a private document, and it is the reason the two views cannot
    // disagree about whether the box is ticked.
    expect(wrapper.emitted("toggle-item-checked")).toEqual([
      [
        {
          listId: "000000000000000000000000",
          itemId: "i1",
          checked: true,
          sourceListId: SHARED_LIST_ID,
        },
      ],
    ])
  })

  it("says which of the club's lists each entry came from", async () => {
    const wrapper = await mountLists(
      privatePanel([assignedList()]),
      MEMBER,
      "member"
    )
    await expandList(wrapper, "000000000000000000000000")

    expect(rowText()).toContain("from Menu")
  })

  it("offers no assignee picker on its own rows", async () => {
    const wrapper = await mountLists(
      { ...privatePanel([assignedList()]), canAssign: true, assignees: [BART] },
      MEMBER,
      "member"
    )
    await expandList(wrapper, "000000000000000000000000")

    expect(buttonTitled(/assign to a member|assigned to/i)).toBeFalsy()
  })

  // A stored list keeps every right it had; only the derived one loses them.
  it("leaves a real list beside it fully editable", async () => {
    const own = {
      _id: "own1",
      name: "Packing",
      kind: "checklist",
      items: [{ _id: "o1", text: "Boots", order: 1024, userId: MEMBER._id }],
    }
    const wrapper = await mountLists(
      privatePanel([assignedList(), own]),
      MEMBER,
      "member"
    )
    await expandList(wrapper, "own1")

    expect(buttonTitled(/edit entry/i)).toBeTruthy()
    expect(buttonTitled(/remove entry/i)).toBeTruthy()
  })

  it("emits no sourceListId from an ordinary list", async () => {
    const own = {
      _id: "own1",
      name: "Packing",
      kind: "checklist",
      items: [{ _id: "o1", text: "Boots", order: 1024, userId: MEMBER._id }],
    }
    const wrapper = await mountLists(privatePanel([own]), MEMBER, "member")
    await expandList(wrapper, "own1")

    const icon = [...document.querySelectorAll(".v-icon")].find(
      (el) => el.getAttribute("title") === "Check off"
    )
    icon.click()
    await settle(wrapper)

    expect(wrapper.emitted("toggle-item-checked")[0][0].sourceListId).toBeNull()
  })
})
