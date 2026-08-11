/*
 * The event page's band tabs switch, and switching one refetches (TODO3 M6).
 *
 * The band is five `v-show` panels behind a wrapping button row, and it has
 * already cost this repo twice: F22's fifth tab put a 390px phone into a
 * horizontal scroll, and the `v-show` construct itself is the one Tailwind's
 * `important: true` silently defeats. Neither of those is visible here — this
 * tier has no real CSS and no layout, which is exactly why `check-routes.js`
 * keeps asserting "exactly one band panel visible" at 1280px and at 390px.
 *
 * What IS visible here, in ~1s instead of 3m19s: that the row renders the right
 * tabs for the viewer, that clicking one shows its panel and hides the rest,
 * that the tabs which need a session are absent without one, and that the two
 * refetch-on-select watchers fire. All four are logic on a mounted component,
 * and none of them can fail a test that never mounts one.
 *
 * The panels' CHILDREN are stubbed. They own their own fetching and their own
 * bugs; mounting the real EventComments and EventExpenses here would be testing
 * them by accident, in a file named after the band.
 */
import { afterEach, describe, expect, it } from "vitest"
import Event from "@/views/Event.vue"
import { calledApi, mockApi } from "@/test/api"
import { cleanupDom, mountApp } from "@/test/mount"

const EVENT_ID = "abc123"

const AUTH_USER = {
  _id: "u1",
  firstName: "Bilbo",
  lastName: "Baggins",
  email: "bilbo@example.test",
  role: "member",
}

/**
 * A gathering the view can actually render.
 *
 * `dates` and `duration` are load-bearing beyond looking realistic:
 * `processEvent` derives `startTime`/`endTime` off them on arrival, and an
 * empty `dates` array makes that a `new Date(undefined)`.
 */
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

/** Everything Event.vue reaches for on arrival, answered plausibly. */
function mockEventApi(overrides = {}) {
  mockApi("/user/profile", AUTH_USER)
  mockApi(/^\/events\/[^/]+$/, { ...EVENT, ...overrides })
  mockApi(/mentionables/, [])
  mockApi(/\/expenses/, [])
  mockApi(/\/lists/, [])
}

/**
 * ScheduleOverlap, reduced to the surface Event.vue reads off its `$ref`.
 *
 * A bare `true` stub is not enough, and the reason is worth writing down:
 * `isSettingSpecificTimes` is `scheduleOverlapComponent?.states.SET_SPECIFIC_TIMES`,
 * where the optional chain guards the ref being absent but NOT `states` being
 * absent on it. Against a stub with no data that throws on every render — which
 * is a fair description of the coupling, so the stub carries the contract
 * instead of the assertion being weakened. The same shape guards
 * `?.respondents.length` two lines above it.
 */
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

const mountEvent = (state = { authUser: AUTH_USER }) =>
  mountApp(Event, {
    props: { eventId: EVENT_ID },
    state,
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

/**
 * The band's tab buttons and its panels, found the way `check-routes.js` finds
 * them: off the Discussion button, up to the row, up to the band. Duplicated
 * shape on purpose — when one tier fails and the other does not, they should be
 * disagreeing about the same elements.
 */
function band() {
  const buttons = [...document.querySelectorAll("button")]
  const discussion = buttons.find((b) =>
    /^Discussion/.test(b.textContent.trim())
  )
  if (!discussion) return null
  const row = discussion.parentElement
  const panels = [...row.parentElement.children].filter((el) => el !== row)
  return {
    tabs: [...row.querySelectorAll("button")],
    titles: [...row.querySelectorAll("button")].map((b) =>
      b.textContent.trim()
    ),
    panels,
    // `v-show` toggles the INLINE display, which is all this tier can see. That
    // a Tailwind display utility with `!important` beats it is a CSS fact, and
    // only the browser check has CSS.
    visible: panels.filter((el) => el.style.display !== "none"),
  }
}

/** Lets `created()`'s awaits, the render and the watchers all settle. */
async function settle(wrapper) {
  for (let i = 0; i < 6; i++) {
    await Promise.resolve()
    await wrapper.vm.$nextTick()
  }
}

async function clickTab(wrapper, title) {
  const tab = band().tabs.find((b) =>
    new RegExp(`^${title}( |$)`).test(b.textContent.trim())
  )
  expect(tab, `no band tab titled ${title}`).toBeTruthy()
  tab.click()
  await settle(wrapper)
}

afterEach(cleanupDom)

describe("Event — band tabs", () => {
  it("renders all five tabs for a signed-in member", async () => {
    mockEventApi()
    const wrapper = await mountEvent()
    await settle(wrapper)

    expect(band().titles).toEqual([
      "Discussion",
      "Lists",
      "My Lists",
      "My Notes",
      "Settle Up",
    ])
  })

  it("counts what is behind the tabs you are not looking at", async () => {
    // The counts are the only sign a closed tab has anything in it, so they are
    // part of the contract rather than decoration. Omitted at zero.
    mockEventApi({
      comments: [{ _id: "c1" }, { _id: "c2" }],
      lists: [{ _id: "l1" }],
    })
    const wrapper = await mountEvent()
    await settle(wrapper)

    expect(band().titles.slice(0, 2)).toEqual(["Discussion (2)", "Lists (1)"])
  })

  it("shows exactly one panel, and it starts on the discussion", async () => {
    mockEventApi()
    const wrapper = await mountEvent()
    await settle(wrapper)

    expect(wrapper.vm.bandTab).toBe("discussion")
    expect(band().visible).toHaveLength(1)
    expect(band().visible[0]).toBe(band().panels[0])
  })

  it("switches to each tab, one panel at a time", async () => {
    mockEventApi()
    const wrapper = await mountEvent()
    await settle(wrapper)

    const expected = {
      Lists: "lists",
      "My Lists": "my-lists",
      "My Notes": "my-notes",
      "Settle Up": "settle-up",
      Discussion: "discussion",
    }
    for (const [title, value] of Object.entries(expected)) {
      await clickTab(wrapper, title)
      expect(wrapper.vm.bandTab, `clicking ${title}`).toBe(value)
      expect(band().visible, `showing ${title}`).toHaveLength(1)
    }
  })

  it("refetches the shared lists when their tab is selected", async () => {
    // The panels are kept alive with v-show, so there is no created() hook to
    // hang this off — the watcher is the only thing that makes an opened tab
    // current, and nothing else in the repo can observe it firing.
    mockEventApi()
    const wrapper = await mountEvent()
    await settle(wrapper)
    expect(calledApi(/\/lists/)).toBe(false)

    await clickTab(wrapper, "Lists")

    expect(calledApi(/\/lists/)).toBe(true)
  })

  it("hides the signed-in tabs, and the private panels, without a session", async () => {
    mockEventApi()
    // Starting the store at `authUser: null` is not enough on its own:
    // `created()` probes `/user/profile` and writes whatever comes back into
    // the store, so a signed-out run is one where the PROBE fails. Registered
    // after mockEventApi because the last handler registered wins.
    mockApi("/user/profile", () => {
      throw { status: 401, body: { error: "not-signed-in" } }
    })

    const wrapper = await mountEvent({ authUser: null })
    await settle(wrapper)

    expect(wrapper.vm.authUser).toBe(null)

    expect(band().titles).toEqual(["Discussion", "Lists"])
    // The two private panels and the ledger are `v-if="authUser"` INSIDE their
    // wrappers, so the wrappers survive: assert on the components, not the
    // divs. Without an account there is nothing to key a private document to.
    expect(wrapper.findComponent({ name: "PersonalLists" }).exists()).toBe(
      false
    )
    expect(wrapper.findComponent({ name: "PersonalNotes" }).exists()).toBe(
      false
    )
    expect(wrapper.findComponent({ name: "EventExpenses" }).exists()).toBe(
      false
    )
  })
})
