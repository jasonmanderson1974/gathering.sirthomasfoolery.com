/*
 * The event page with no connection.
 *
 * The three surfaces this feature is for — Discussion, Lists and Settle Up —
 * all hang off a page whose entire body is `v-if="event"`, and whose created()
 * hook used to swallow every error that wasn't EventNotFound. So an offline
 * load rendered an empty document: no error, no explanation, nothing logged.
 *
 * These mount the real view against a real fetch_utils and a real cache, and
 * only cut the network. Note that Discussion arrives INSIDE the event payload
 * (there is no GET .../comments endpoint at all), which is why one cached read
 * is enough to bring the discussion back with the page.
 */
import { afterEach, beforeEach, describe, expect, it } from "vitest"
import Event from "@/views/Event.vue"
import { mockApi, mockOffline } from "@/test/api"
import { cleanupDom, mountApp } from "@/test/mount"
import {
  __resetCacheForTests,
  setCacheUser,
} from "@/utils/offline/cache"
import { __resetStatusForTests } from "@/utils/offline/status"

const EVENT_ID = "abc123"

const AUTH_USER = {
  _id: "u1",
  firstName: "Bilbo",
  lastName: "Baggins",
  email: "bilbo@example.test",
  role: "member",
}

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
  comments: [{ _id: "c1", text: "Bring the good port", author: AUTH_USER }],
  lists: [{ _id: "l1", name: "To bring", items: [] }],
  remindees: [],
  location: "",
}

const EXPENSES = [
  { _id: "x1", title: "Port", amountCents: 4200, paidBy: AUTH_USER._id },
]

function mockEventApi() {
  mockApi("/user/profile", AUTH_USER)
  mockApi(/^\/events\/[^/]+$/, EVENT)
  mockApi(/mentionables/, [])
  mockApi(/\/expenses/, EXPENSES)
  mockApi(/\/lists/, EVENT.lists)
}

/** See Event.band.spec.js — Event.vue reads `states` off this ref on render. */
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

beforeEach(async () => {
  __resetCacheForTests()
  __resetStatusForTests()
  // The store used by mountApp is a test double whose setAuthUser does not
  // touch the offline layer, so the cache namespace is declared here — the
  // same thing main.js does at boot from the remembered session.
  await setCacheUser(AUTH_USER._id)
})

afterEach(() => {
  cleanupDom()
})

describe("a gathering read online, then opened with no connection", () => {
  it("renders the page instead of an empty document", async () => {
    mockEventApi()
    const online = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))
    online.unmount()

    mockOffline()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.event).toBeTruthy()
    expect(wrapper.vm.event.name).toBe("Second Breakfast")
    expect(wrapper.find(".schedule-overlap-stub").exists()).toBe(true)
  })

  // Discussion rides along inside the event payload rather than having an
  // endpoint of its own, so it comes back with the page or not at all.
  it("brings the discussion back with the page", async () => {
    mockEventApi()
    const online = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))
    online.unmount()

    mockOffline()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.event.comments).toHaveLength(1)
    expect(wrapper.vm.event.comments[0].text).toBe("Bring the good port")
  })

  it("brings the lists back with the page", async () => {
    mockEventApi()
    const online = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))
    online.unmount()

    mockOffline()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.sharedLists).toHaveLength(1)
  })

  // Settle Up is a separate read, and the one the user called out as mattering
  // most. It is fetched on arrival rather than on tab open, so it caches on
  // arrival too.
  it("brings the ledger back", async () => {
    mockEventApi()
    const online = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))
    online.unmount()

    mockOffline()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.expenses).toHaveLength(1)
    expect(wrapper.vm.expenses[0].amountCents).toBe(4200)
  })

  // Reading a Settle Up figure without knowing it may predate the last three
  // expenses is worse than knowing the page is stale.
  it("says how old the copy on screen is", async () => {
    mockEventApi()
    const online = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))
    online.unmount()

    mockOffline()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.cachedAgeLabel).toBe("just now")
    expect(wrapper.text()).toContain("Saved copy — last updated just now")
  })

  it("says nothing of the sort when the page is live", async () => {
    mockEventApi()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.cachedAgeLabel).toBeNull()
    expect(wrapper.text()).not.toContain("Saved copy")
  })

  // Clearing authUser offline would fire the watcher that drops the ledger and
  // resets the open tab — over a lost signal, on a valid session.
  it("keeps the member signed in", async () => {
    mockEventApi()
    const online = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))
    online.unmount()

    mockOffline()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.authUser).toMatchObject({ _id: "u1" })
  })
})

describe("a gathering never read on this device", () => {
  it("explains itself rather than rendering nothing", async () => {
    mockOffline()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.event).toBeNull()
    expect(wrapper.vm.unavailableOffline).toBe(true)
    expect(wrapper.text()).toContain("not available offline")
  })

  // "Please try again" is the wrong advice when there is nothing to try again
  // with, and the offline banner already says why.
  it("does not raise a retry snackbar", async () => {
    mockOffline()
    const wrapper = await mountEvent()
    await new Promise((r) => setTimeout(r, 0))

    expect(wrapper.vm.$store.state.error).toBe("")
  })
})
