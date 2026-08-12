/*
 * Undo for a cascading assignment, on the mounted view (TODO3 N2).
 *
 * The rule this file exists to pin: the Undo button appears exactly when the
 * SERVER said the action was undoable — it returns a token only for a write that
 * cascaded. A client that decided for itself (say, "the row had children") would
 * be a second copy of that rule, free to drift from the one that actually
 * governs whether the restore will be honoured, and the failure would be an Undo
 * button that 404s.
 *
 * The toast is `Event.vue`'s own `v-snackbar` rather than the app-wide
 * `AutoSnackbar`, so it can only be exercised by mounting the view. Everything
 * else in the band is stubbed — those panels own their own fetching and their
 * own bugs.
 */
import { afterEach, describe, expect, it } from "vitest"
import Event from "@/views/Event.vue"
import { apiCalls, mockApi } from "@/test/api"
import { cleanupDom, mountApp } from "@/test/mount"

const EVENT_ID = "abc123"

const AUTH_USER = {
  _id: "u1",
  firstName: "Bilbo",
  lastName: "Baggins",
  email: "bilbo@example.test",
  role: "member",
}

const BART = { _id: "u2", firstName: "Bart", lastName: "Renfrew" }

const EVENT = {
  _id: "64b7f0c2c9e77c0012345678",
  shortId: EVENT_ID,
  name: "Second Breakfast",
  duration: 2,
  dates: ["2026-09-01T16:00:00Z"],
  type: "specific_dates",
  hasSpecificTimes: true,
  daysOnly: false,
  ownerId: AUTH_USER._id,
  responses: {},
  comments: [],
  lists: [],
  remindees: [],
  location: "",
}

const ScheduleOverlapStub = {
  name: "ScheduleOverlap",
  template: "<div class='schedule-overlap-stub' />",
  data: () => ({
    states: { SET_SPECIFIC_TIMES: "set_specific_times" },
    state: "best_times",
    respondents: [],
    allowScheduleEvent: false,
    editing: false,
    scheduling: false,
    unsavedChanges: false,
    selectedGuestRespondent: null,
  }),
}

/** Answers everything the view reaches for, with the assign route configurable. */
function mockApiFor(assignResponse) {
  mockApi("/user/profile", AUTH_USER)
  mockApi(/^\/events\/[^/]+$/, EVENT)
  mockApi(/mentionables/, [])
  mockApi(/\/expenses/, [])
  mockApi(/\/lists\/assignees/, [BART])
  mockApi(/\/lists$/, [])
  mockApi(/\/undo-assign/, {})
  mockApi(/\/assignee$/, assignResponse)
}

const mountEvent = () =>
  mountApp(Event, {
    props: { eventId: EVENT_ID },
    state: { authUser: AUTH_USER },
    route: `/e/${EVENT_ID}`,
    stubs: {
      ScheduleOverlap: ScheduleOverlapStub,
      EventHeader: true,
      EventBottomBar: true,
      EventDescription: true,
      EventLocation: true,
      GatheringRsvp: true,
      GatheringSummary: true,
      EventComments: true,
      EventPolls: true,
      EventLists: true,
      PersonalLists: true,
      PersonalNotes: true,
      EventExpenses: true,
      SettleUpSummary: true,
      NewDialog: true,
      GuestDialog: true,
      MarkAvailabilityDialog: true,
      SignInNotSupportedDialog: true,
    },
  })

async function settle(wrapper) {
  for (let i = 0; i < 8; i++) {
    await Promise.resolve()
    await wrapper.vm.$nextTick()
  }
}

/** The undo toast's text, or null when it isn't showing. */
function toast() {
  const snackbars = [...document.querySelectorAll(".v-snackbar")]
  const open = snackbars.find(
    (s) => s.querySelector(".v-snackbar__wrapper") && s.textContent.trim()
  )
  return open ? open.textContent.replace(/\s+/g, " ").trim() : null
}

function undoButton() {
  return [...document.querySelectorAll(".v-snackbar button")].find((b) =>
    /^Undo$/i.test(b.textContent.trim())
  )
}

/**
 * Opens the Lists tab, which is what fetches the assignee picker.
 *
 * Not incidental setup: `fetchListAssignees` hangs off the tab watcher, so a
 * test that assigns without opening the tab has an empty picker and a toast that
 * cannot name anybody — which is a state the real app cannot reach, since the
 * control being clicked is on that tab.
 */
async function openListsTab(wrapper) {
  wrapper.vm.bandTab = "lists"
  await settle(wrapper)
}

/** Fires the view's assign handler exactly as EventLists' emit would. */
async function assign(wrapper, assigneeId = BART._id) {
  await wrapper.vm.onAssignListItem({
    listId: "l1",
    itemId: "i1",
    assigneeId,
  })
  await settle(wrapper)
}

afterEach(cleanupDom)

describe("Event — undo for a cascading assignment", () => {
  it("offers Undo when the server says the write cascaded", async () => {
    mockApiFor({ affected: 9, undoToken: "tok-1" })
    const wrapper = await mountEvent()
    await settle(wrapper)
    await openListsTab(wrapper)
    await assign(wrapper)

    expect(toast()).toContain("Assigned 9 entries to Bart Renfrew.")
    expect(undoButton()).toBeTruthy()
  })

  // The rule lives on the server; a token's absence is the client's only cue.
  it("stays silent when the server returned no token", async () => {
    mockApiFor({ affected: 1 })
    const wrapper = await mountEvent()
    await settle(wrapper)
    await openListsTab(wrapper)
    await assign(wrapper)

    expect(wrapper.vm.showAssignUndo).toBe(false)
    expect(undoButton()).toBeFalsy()
  })

  it("names a clear differently from a hand-over", async () => {
    mockApiFor({ affected: 4, undoToken: "tok-1" })
    const wrapper = await mountEvent()
    await settle(wrapper)
    await openListsTab(wrapper)
    await assign(wrapper, null)

    expect(toast()).toContain("Cleared 4 entries.")
  })

  it("posts the token back and refetches when Undo is clicked", async () => {
    mockApiFor({ affected: 9, undoToken: "tok-1" })
    const wrapper = await mountEvent()
    await settle(wrapper)
    await openListsTab(wrapper)
    await assign(wrapper)

    undoButton().click()
    await settle(wrapper)

    const undoCall = apiCalls().find((c) => /undo-assign/.test(c.route))
    expect(undoCall).toBeTruthy()
    expect(undoCall.method).toBe("POST")
    expect(undoCall.body).toEqual({ undoToken: "tok-1" })
    // The refetch is what puts the restored names back on screen.
    expect(
      apiCalls().filter((c) => /\/lists$/.test(c.route)).length
    ).toBeGreaterThan(1)
  })

  // The button has one use. Leaving it up invites a second click that can only
  // fail, because the server deletes the record when it honours it.
  it("closes the toast as soon as Undo is pressed", async () => {
    mockApiFor({ affected: 9, undoToken: "tok-1" })
    const wrapper = await mountEvent()
    await settle(wrapper)
    await openListsTab(wrapper)
    await assign(wrapper)

    undoButton().click()
    await settle(wrapper)

    expect(wrapper.vm.showAssignUndo).toBe(false)
    expect(wrapper.vm.assignUndo).toBeNull()
  })

  it("says so plainly when the window has closed", async () => {
    mockApiFor({ affected: 9, undoToken: "tok-1" })
    mockApi(/\/undo-assign/, () => {
      throw { status: 404, body: { error: "undo-unavailable" } }
    })
    const wrapper = await mountEvent()
    await settle(wrapper)
    await openListsTab(wrapper)
    await assign(wrapper)

    undoButton().click()
    await settle(wrapper)

    expect(wrapper.vm.$store.state.error).toBe("That can no longer be undone.")
  })

  // A failed assign has nothing to undo, and must not leave a button claiming
  // otherwise.
  it("offers nothing when the assignment itself failed", async () => {
    mockApiFor({ affected: 9, undoToken: "tok-1" })
    mockApi(/\/assignee$/, () => {
      throw { status: 500, body: { error: "internal-error" } }
    })
    const wrapper = await mountEvent()
    await settle(wrapper)
    await openListsTab(wrapper)
    await assign(wrapper)

    expect(wrapper.vm.showAssignUndo).toBe(false)
    expect(wrapper.vm.$store.state.error).toBe(
      "Could not assign that entry. Please try again."
    )
  })
})
