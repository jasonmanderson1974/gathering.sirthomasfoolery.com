/*
 * The New Gathering dialog opens, and opens quietly (TODO3 M6).
 *
 * This is K3's bug, and K3 shipped to production: opening this dialog threw a
 * TypeError on every open for the whole Vue 3 migration. Nothing caught it —
 * Vue reports a throw from a lifecycle hook through `console.error` and carries
 * on, so the page looks fine, the build is fine, and the unit suite never
 * mounted the component. It took a human clicking the button.
 *
 * The `console.error` half of that is the whole reason the console guard in
 * `src/test/setup.dom.js` fails every test in this project on an unasserted
 * error. This file's assertions are the visible half; the guard is the half
 * that would actually have caught K3.
 *
 * `NewDialog` is mounted rather than `NewEvent` directly, because the failure
 * was in the *opening*: the v-dialog, its teleport and the `:key` that forces a
 * fresh form are all part of what broke.
 */
import { afterEach, describe, expect, it } from "vitest"
import NewDialog from "@/components/NewDialog.vue"
import { cleanupDom, mountApp, openDialogs } from "@/test/mount"

const AUTH_USER = {
  _id: "u1",
  firstName: "Bilbo",
  lastName: "Baggins",
  email: "bilbo@example.test",
  role: "member",
}

const mountDialog = (props = {}) =>
  mountApp(NewDialog, {
    props: { modelValue: true, ...props },
    state: { authUser: AUTH_USER },
  })

afterEach(cleanupDom)

describe("NewDialog", () => {
  it("opens with the gathering form in it", async () => {
    await mountDialog()

    expect(openDialogs()).toHaveLength(1)
    const text = document.body.textContent
    expect(text).toContain("Call a Gathering")
    // The three parts of the form the browser check also asserts on, so a
    // failure here and a failure there name the same thing.
    expect(text).toContain("Dates and times")
    expect(text).toContain("What times might work?")
    expect(
      document.querySelector('input[placeholder="Name your event..."]')
    ).toBeTruthy()
  })

  it("renders no dialog when closed", async () => {
    await mountDialog({ modelValue: false })

    expect(openDialogs()).toHaveLength(0)
  })

  it("titles itself for an amendment when editing", async () => {
    // The edit path takes a different branch through NewEvent's mixin — it
    // seeds every field off an existing event rather than off defaults, which
    // is where a shape change in the event model would land.
    await mountDialog({
      edit: true,
      event: {
        _id: "e1",
        name: "Second Breakfast",
        duration: 1,
        dates: [],
        hasSpecificTimes: true,
        daysOnly: false,
        location: "",
        remindees: [],
      },
    })

    expect(document.body.textContent).toContain("Amend the Gathering")
    expect(
      document.querySelector('input[placeholder="Name your event..."]').value
    ).toBe("Second Breakfast")
  })

  it("closes on request, without an unsaved-changes prompt when creating", async () => {
    // `handleDialogInput` reaches into `$refs.form` for `hasEventBeenEdited()`
    // and `reset()`. A ref that does not resolve is silent in Vue 3 until
    // something calls a method on it, and this is the only path that does.
    const wrapper = await mountDialog()

    wrapper.vm.handleDialogInput()
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted("update:modelValue")).toEqual([[false]])
  })
})
