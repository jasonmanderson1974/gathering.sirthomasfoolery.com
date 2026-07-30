import { mapActions } from "vuex"
import { patch } from "@/utils"

/**
 * The shared half of the two calendar-option switches — WorkingHoursToggle and
 * BufferTimeSwitch, whose update methods were byte-identical modulo the
 * identifier (G1).
 *
 * Both own one object under the user's calendar options, patch the whole object
 * on every change, and emit it back up for `.sync`. The only thing that differed
 * was its name, so this is a factory rather than a plain mixin: `optionName`
 * names the prop, the key in the PATCH body, and the `update:` event.
 *
 * CalendarAccount.vue deliberately does NOT use this. It POSTs to two different
 * toggle routes with if/else semantics, so folding it in here would mean a
 * parameter for every one of those differences.
 *
 * @param {string} optionName - e.g. "workingHours" or "bufferTime"
 * @param {string} errorLabel - what the user sees when the save fails
 */
export const calendarOptionSync = (optionName, errorLabel) => ({
  props: {
    [optionName]: { type: Object, required: true },
    /**
     * Off in the event flow, where the option is a local override that is never
     * persisted to the account; on in Settings.
     */
    syncWithBackend: { type: Boolean, default: false },
  },

  methods: {
    ...mapActions(["showError"]),

    /**
     * Merge one key into the option, persist it if this instance syncs, and
     * emit the merged object for `.sync`.
     *
     * The emit is not gated on the request: the switch has already moved, and
     * holding the UI on a round-trip would make every toggle feel broken. A
     * failed save is surfaced instead — before G1 the patch had no .catch() at
     * all, so the request rejected silently and the UI kept showing a value the
     * server never took.
     */
    updateCalendarOption(key, val) {
      const option = { ...this[optionName], [key]: val }
      if (this.syncWithBackend) {
        patch("/user/calendar-options", { [optionName]: option }).catch(() => {
          this.showError(`Could not save your ${errorLabel}. Please try again.`)
        })
      }
      this.$emit(`update:${optionName}`, option)
    },
  },
})
