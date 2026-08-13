/*
 * The hover card, mounted (N3).
 *
 * Two things here cannot be caught anywhere else in the repo.
 *
 * The first is the privacy split. Which endpoint the card reaches for is
 * decided by `canInvite`, and getting it wrong is not a degraded card — a guest
 * sent to /admin/allowlist gets a 403, because that route is behind
 * CanInviteRequired. The `node` tier cannot see it (it dispatches nothing) and
 * `check:routes` cannot see it (it signs in as one superAdmin). So the "never
 * calls the roll" assertion below is the only thing standing between a guest
 * and a wall of empty cards.
 *
 * The second is that the card must NOT open where a person cannot be resolved.
 * Bare-name rows are everywhere — a guest respondent, a deleted comment author,
 * a legacy name-keyed RSVP — and the failure mode is a 96px monogram with no
 * information under it, which looks like a bug on a page that is working.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import MemberHoverCard from "@/components/general/MemberHoverCard.vue"
import { cleanupDom, mountApp } from "@/test/mount"
import { apiCalls, calledApi, mockApi } from "@/test/api"

const MEMBER = {
  _id: "invite-row-1",
  userId: "acct-1",
  email: "bart@example.test",
  phone: "5551234567",
  firstName: "Bartholomew",
  lastName: "Fitzwilliam",
  nickname: "Bart",
  role: "member",
  hasAccount: true,
}

/**
 * What a call site already holds — the slim user the server attaches to a
 * comment or an RSVP. Guests never bulk-load the roll, so this is what their
 * card is anchored on until the public profile arrives.
 */
const BYLINE = {
  _id: "acct-1",
  firstName: "Bartholomew",
  lastName: "Fitzwilliam",
}

/** A member+ viewer, which is what makes the roll readable. */
const AS_MEMBER = { authUser: { _id: "me", role: "member" } }
const AS_GUEST = { authUser: { _id: "me", role: "guest" } }

const mountCard = (props = {}, state = AS_MEMBER) =>
  mountApp(MemberHoverCard, {
    props,
    state,
    slots: { default: '<button class="trigger">Bart</button>' },
  })

const trigger = () => document.querySelector(".trigger")

/**
 * The element that actually carries the hover listeners.
 *
 * Both Vuetify's activator bindings and our own prefetch sit on the wrapper
 * span, not on the slot content — and `mouseenter` does not bubble, so
 * dispatching it at the button inside would reach neither.
 */
const activator = () => trigger()?.closest("span")

/** The teleported overlay panel, or null while the card is shut. */
const card = () => document.querySelector(".v-overlay__content")

/**
 * Hover the trigger and let the open delay elapse.
 *
 * The delay is the feature — the card is not supposed to appear the instant a
 * pointer crosses it — so the wait is driven with fake timers rather than by
 * sleeping half a second per assertion.
 */
async function hover(wrapper, ms = 600) {
  // Let the roll fetched in `created()` land first: until something resolves,
  // there is deliberately no hover target to dispatch at.
  await vi.advanceTimersByTimeAsync(0)
  await wrapper.vm.$nextTick()

  activator()?.dispatchEvent(new Event("mouseenter"))
  await wrapper.vm.$nextTick()
  await vi.advanceTimersByTimeAsync(ms)
  await wrapper.vm.$nextTick()
  await wrapper.vm.$nextTick()
}

beforeEach(() => {
  vi.useFakeTimers()
  mockApi("/admin/allowlist", [MEMBER])
})

afterEach(() => {
  vi.useRealTimers()
  cleanupDom()
})

describe("MemberHoverCard", () => {
  it("shows the roll's details for a member+ viewer", async () => {
    const wrapper = await mountCard({ userId: "acct-1" })
    await hover(wrapper)

    const panel = card()
    expect(panel).toBeTruthy()
    // Nickname leads, real name underneath — the club calls him Bart.
    expect(panel.textContent).toContain("Bart")
    expect(panel.textContent).toContain("Bartholomew Fitzwilliam")
    expect(panel.textContent).toContain("bart@example.test")
    // Rendered through formatPhone, not raw.
    expect(panel.textContent).toContain("(555) 123-4567")
  })

  it("renders the avatar at the size the Settings page uses", async () => {
    const wrapper = await mountCard({ userId: "acct-1" })
    await hover(wrapper)

    // 96px is the whole point of "same as what's in Settings" — a card that
    // quietly rendered the 22px byline avatar would still pass every other
    // assertion in this file.
    const avatar = card().querySelector(".v-avatar")
    expect(avatar.style.width).toBe("96px")
  })

  it("does not open before the delay has elapsed", async () => {
    const wrapper = await mountCard({ userId: "acct-1" })
    await hover(wrapper, 200)
    expect(card()).toBeNull()
  })

  it("fetches the roll once no matter how many cards are hovered", async () => {
    const wrapper = await mountCard({ userId: "acct-1" })
    await hover(wrapper)
    activator().dispatchEvent(new Event("mouseenter"))
    await vi.advanceTimersByTimeAsync(600)

    const rollCalls = apiCalls().filter((c) => c.route === "/admin/allowlist")
    expect(rollCalls).toHaveLength(1)
  })

  it("sends a guest to the public profile and NEVER to the roll", async () => {
    mockApi("/users/acct-1", {
      _id: "acct-1",
      firstName: "Bartholomew",
      lastName: "Fitzwilliam",
      nickname: "Bart",
    })

    const wrapper = await mountCard(
      { userId: "acct-1", fallback: BYLINE },
      AS_GUEST
    )
    await hover(wrapper)

    expect(calledApi("/users/acct-1")).toBe(true)
    // The assertion this file exists for.
    expect(calledApi("/admin/allowlist")).toBe(false)
  })

  it("gives a guest a card with names but no contact details", async () => {
    mockApi("/users/acct-1", {
      _id: "acct-1",
      firstName: "Bartholomew",
      lastName: "Fitzwilliam",
      nickname: "Bart",
    })

    const wrapper = await mountCard(
      { userId: "acct-1", fallback: BYLINE },
      AS_GUEST
    )
    await hover(wrapper)

    const panel = card()
    expect(panel.textContent).toContain("Bartholomew Fitzwilliam")
    expect(panel.textContent).not.toContain("@example.test")
    expect(panel.textContent).not.toContain("555")
  })

  it("leaves a bare name inert — no wrapper, no card, no request", async () => {
    // What `userFromDisplayName` produces: a first and last name and nothing
    // else. There is no one to look up.
    const wrapper = await mountCard({
      fallback: { firstName: "Percival", lastName: "Thorne" },
    })
    await hover(wrapper)

    expect(card()).toBeNull()
    expect(calledApi("/admin/allowlist")).toBe(false)
    // The trigger is still rendered, exactly as it was handed over.
    expect(trigger()).toBeTruthy()
  })

  it("takes a pre-resolved row without looking anything up", async () => {
    // Fellowship and the Roll already hold the allowlist; re-fetching it to
    // render a card over the row it came from would be absurd.
    const wrapper = await mountCard({ person: MEMBER })
    await hover(wrapper)

    expect(card().textContent).toContain("bart@example.test")
    expect(calledApi("/admin/allowlist")).toBe(false)
  })

  it("opens on what the page already had while the roll is in flight", async () => {
    // The fetch rides the open delay; a slow one must not produce an empty card.
    mockApi("/admin/allowlist", () => new Promise(() => {}))

    const wrapper = await mountCard({
      userId: "acct-1",
      fallback: { _id: "acct-1", firstName: "Ada", lastName: "Lovelace" },
    })
    await hover(wrapper)

    expect(card().textContent).toContain("Ada Lovelace")
  })
})
