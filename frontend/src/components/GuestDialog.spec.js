/*
 * The submit guard actually guards (TODO3 M6).
 *
 * This is L1's bug class, and L1 shipped: `if (!this.$refs.form.validate())
 * return` reads as a guard and is dead code, because Vuetify 3's `validate()`
 * returns `Promise<{ valid, errors }>` and a Promise is always truthy. Nothing
 * in the repo could fail on it — the guard is inside a method on a component no
 * test mounted, and the browser check does not fill in forms.
 *
 * GuestDialog is the awkward one of the app's forms and therefore the one worth
 * pinning: its rules are installed at submit time rather than declared on the
 * fields, which only works because a rules change does not itself trigger
 * validation. Get that ordering wrong and the first submit always passes.
 */
import { afterEach, describe, expect, it } from "vitest"
import GuestDialog from "@/components/GuestDialog.vue"
import { cleanupDom, mountApp, openDialogs } from "@/test/mount"

const EVENT = { _id: "e1", collectEmails: false, responses: {} }

const mountDialog = (props = {}) =>
  mountApp(GuestDialog, {
    props: { modelValue: true, event: EVENT, respondents: ["Bilbo"], ...props },
  })

/** The dialog's fields are teleported with it, so they are found off document. */
const fieldFor = (placeholder) =>
  document.querySelector(`input[placeholder="${placeholder}"]`)

async function typeName(wrapper, value) {
  const input = fieldFor("Guest's name...")
  input.value = value
  input.dispatchEvent(new Event("input", { bubbles: true }))
  await wrapper.vm.$nextTick()
}

/** Clicks Continue and lets the async submit path settle. */
async function submit(wrapper) {
  await wrapper.vm.submit()
  await wrapper.vm.$nextTick()
}

afterEach(cleanupDom)

describe("GuestDialog", () => {
  it("opens", async () => {
    await mountDialog()
    expect(openDialogs()).toHaveLength(1)
    expect(document.body.textContent).toContain("Add availability for a guest")
    expect(fieldFor("Guest's name...")).toBeTruthy()
  })

  it("does not submit an empty name", async () => {
    const wrapper = await mountDialog()

    await submit(wrapper)

    expect(wrapper.emitted("submit")).toBeUndefined()
    // Not just "it didn't emit": the guard has to have RUN, and the reason has
    // to have reached the person looking at the form.
    expect(document.body.textContent).toContain("Name is required")
  })

  it("does not submit a name already taken", async () => {
    const wrapper = await mountDialog()

    await typeName(wrapper, "Bilbo")
    await submit(wrapper)

    expect(wrapper.emitted("submit")).toBeUndefined()
    expect(document.body.textContent).toContain("Name already taken")
  })

  it("does not submit an ObjectID-shaped name", async () => {
    // The server rejects these because responses key members by id and guests
    // by name in one map, so this name could overwrite a member's response.
    const wrapper = await mountDialog()

    await typeName(wrapper, "0123456789abcdef01234567")
    await submit(wrapper)

    expect(wrapper.emitted("submit")).toBeUndefined()
    expect(document.body.textContent).toContain("That name isn't allowed")
  })

  it("submits a valid name", async () => {
    const wrapper = await mountDialog()

    await typeName(wrapper, "Frodo")
    await submit(wrapper)

    expect(wrapper.emitted("submit")).toEqual([[{ name: "Frodo", email: "" }]])
  })

  it("requires an email when the event collects them", async () => {
    const wrapper = await mountDialog({
      event: { ...EVENT, collectEmails: true },
    })

    await typeName(wrapper, "Frodo")
    await submit(wrapper)

    expect(wrapper.emitted("submit")).toBeUndefined()
    expect(document.body.textContent).toContain("Email is required")
  })
})
