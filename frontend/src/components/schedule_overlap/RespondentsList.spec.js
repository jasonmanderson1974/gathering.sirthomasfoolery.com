/*
 * The respondents list is the one place where the hover card shares a gesture
 * with something else, and this file exists for that overlap alone.
 *
 * Hovering a respondent highlights their availability in the schedule grid
 * (`mouseOverRespondent`). N3 then wrapped the avatar and the name in a hover
 * card, which installs its own hover listener inside that row. The two coexist
 * only because they listen to DIFFERENT events: the row uses `@mouseover`,
 * which BUBBLES and so still fires from inside the wrapper, while the card
 * opens on `mouseenter`, which does not bubble and so is never triggered by
 * anything the row does.
 *
 * If someone "tidies" either listener to match the other, the grid highlight
 * silently stops following the pointer — a page that still renders perfectly
 * and quietly lost a feature. Nothing else in the repo would notice.
 */
import { afterEach, describe, expect, it } from "vitest"
import RespondentsList from "@/components/schedule_overlap/RespondentsList.vue"
import { cleanupDom, mountApp } from "@/test/mount"

// 24-hex: `accountId()` requires it, which is how it tells an account from a
// guest — a guest respondent's `_id` IS their first name (see isGuest).
const BART = {
  _id: "6a7df273e5a28be5086551b2",
  firstName: "Bart",
  lastName: "Renfrew",
}
const GUEST = { _id: "Percival", firstName: "Percival", lastName: "" }

const mountList = async (respondents) => {
  const wrapper = await mountApp(RespondentsList, {
    props: {
      eventId: "e1",
      event: { _id: "e1", responses: {}, collectEmails: false },
      curRespondents: [],
      curTimeslot: {},
      curTimeslotAvailability: {},
      respondents,
      parsedResponses: {},
      isOwner: false,
      timezone: { value: "America/New_York" },
      showBestTimes: false,
      hideIfNeeded: false,
      showEventOptions: false,
    },
  })
  await wrapper.vm.$nextTick()
  return wrapper
}

const row = () => document.querySelector(".tw-group")
const triggers = () => document.querySelectorAll("[data-member-hover]")

afterEach(cleanupDom)

describe("RespondentsList — hover card alongside the grid highlight", () => {
  it("still emits mouseOverRespondent when the pointer is inside the card's wrapper", async () => {
    const wrapper = await mountList([BART])
    expect(triggers().length).toBeGreaterThan(0)

    // Dispatched AT the card's wrapper, not at the row — this is precisely the
    // case that breaks if the row's listener stops relying on bubbling.
    triggers()[0].dispatchEvent(new MouseEvent("mouseover", { bubbles: true }))
    await wrapper.vm.$nextTick()

    const emitted = wrapper.emitted("mouseOverRespondent")
    expect(emitted).toBeTruthy()
    expect(emitted[0][1]).toBe(BART._id)
  })

  it("wraps both the avatar and the name for an account", async () => {
    await mountList([BART])
    expect(triggers().length).toBe(2)
  })

  it("leaves a guest alone — their id is their own name", async () => {
    // No account to describe, and `accountId()` would reject the id anyway.
    // Asserted here as well so the guard cannot be removed from just this file.
    await mountList([GUEST])
    expect(triggers().length).toBe(0)
    expect(row()).toBeTruthy()
  })
})
