<template>
  <div ref="root">
    <v-date-picker
      ref="datePicker"
      v-model:month="pickerMonth"
      v-model:year="pickerYear"
      :model-value="selectedDates"
      @update:model-value="onPickerModel"
      multiple
      hide-header
      color="primary"
      class="tw-min-w-full tw-rounded-md tw-border-0 tw-drop-shadow sm:tw-min-w-0"
      :min="minDate"
      :first-day-of-week="startCalendarOnMonday ? 1 : 0"
    ></v-date-picker>
  </div>
</template>

<script>
/**
 * Multi-date picker with drag-to-select.
 *
 * The public contract is unchanged from the Vuetify 2 version: `modelValue` is
 * an array of `YYYY-MM-DD` strings, and that is what the rest of the new-event
 * form and the API speak. Vuetify 3's picker works in `Date` objects, so the
 * conversion happens here and nowhere else.
 *
 * **Vuetify 3 removed the `:date` event suffix** (`@mousedown:date`,
 * `@mouseover:date`, `@touchstart:date`), which is how the drag used to be
 * driven. Rather than give the feature up, the mouse now goes through the same
 * delegated DOM listeners the touch path always used — the day cell is located
 * in the event's composed path and the date is reconstructed from the month and
 * year the picker is showing.
 *
 * The division of labour changed with it, and the reason is visual: the v2
 * version ran the picker `readonly` and did every toggle itself, but v3 renders
 * a readonly picker greyed out, which reads as "you can't pick a date here".
 * So **Vuetify owns the click and this component owns the drag** — mousedown
 * only records which way the drag goes, and the cell under the pointer is
 * toggled by Vuetify's own handler. Both paths agree on the rule (present ⇒
 * remove, absent ⇒ add), so a drag that crosses a date twice ends where it
 * started either way.
 *
 * Adjacent-month days are not rendered by default in v3, so a day number read
 * off a visible cell always belongs to the displayed month.
 */
const pad = (n) => String(n).padStart(2, "0")

export default {
  name: "DatePicker",

  props: {
    modelValue: { type: Array, required: true },
    minCalendarDate: { type: String, default: "" },
    startCalendarOnMonday: { type: Boolean, default: false },
  },

  emits: ["update:modelValue"],

  data() {
    const today = new Date()
    return {
      rootEl: null,
      dragging: false,
      // Whether the pointer reached a date other than the one it started on.
      dragMoved: false,
      dragStart: null,
      dragState: "add",
      dragStates: { ADD: "add", REMOVE: "remove" },
      pickerMonth: today.getMonth(),
      pickerYear: today.getFullYear(),
    }
  },

  computed: {
    /** The ISO strings the parent owns, as the Date objects Vuetify 3 wants. */
    selectedDates() {
      return this.modelValue.map((iso) => {
        const [y, m, d] = iso.split("-").map(Number)
        return new Date(y, m - 1, d)
      })
    },
    minDate() {
      if (!this.minCalendarDate) return undefined
      const [y, m, d] = this.minCalendarDate.split("-").map(Number)
      return new Date(y, m - 1, d)
    },
  },

  methods: {
    /**
     * The `YYYY-MM-DD` a pointer event landed on, or null if it missed a day.
     *
     * Uses `composedPath` rather than `event.target` so it still resolves when
     * the pointer is over the button's inner content rather than the cell.
     */
    dateFromEvent(e) {
      const path = e.composedPath ? e.composedPath() : [e.target]
      const cell = path.find(
        (el) =>
          el instanceof HTMLElement &&
          el.classList?.contains("v-date-picker-month__day")
      )
      if (!cell) return null
      const btn = cell.querySelector("button")
      if (!btn) return null // an adjacent-month placeholder
      const day = parseInt(btn.textContent.trim(), 10)
      if (isNaN(day)) return null
      return `${this.pickerYear}-${pad(this.pickerMonth + 1)}-${pad(day)}`
    },

    /** Whether a date is before `minCalendarDate`, which must stay unselectable. */
    isBelowMin(date) {
      return !!this.minCalendarDate && date < this.minCalendarDate
    },

    /**
     * Arm the drag. Deliberately does NOT toggle the date under the pointer:
     * Vuetify's own click handler does that a moment later, and doing it here
     * too would toggle it twice and leave it where it started.
     */
    onPointerDown(e) {
      const date = this.dateFromEvent(e)
      if (!date || this.isBelowMin(date)) return
      this.dragging = true
      this.dragMoved = false
      this.dragStart = date
      this.setDragState(date)
    },

    /** Vuetify toggled a date itself (a plain click). Convert and pass it up. */
    onPickerModel(dates) {
      this.$emit(
        "update:modelValue",
        (dates ?? []).map(
          (d) =>
            `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
        )
      )
    },

    onPointerOver(e) {
      if (!this.dragging) return
      const date = this.dateFromEvent(e)
      if (!date || this.isBelowMin(date) || date === this.dragStart) return
      // The moment this becomes a drag, take over the starting day too. A
      // mouseup on a different element means the browser never synthesises a
      // `click`, so Vuetify's handler — which would otherwise have toggled it —
      // never runs, and the day the drag began on would be the one day it
      // missed.
      if (!this.dragMoved) {
        this.dragMoved = true
        // Both in ONE emit. Two calls would each rebuild from `modelValue`,
        // which the parent has not written back yet within a single tick — so
        // the second would silently discard the first.
        this.addRemoveDates([this.dragStart, date])
        return
      }
      this.addRemoveDates([date])
    },

    onTouchMove(e) {
      if (!this.dragging) return
      e.preventDefault()
      const touch = e.changedTouches[0]
      const target = document.elementFromPoint(touch.clientX, touch.clientY)
      if (!target || !this.rootEl.contains(target)) return
      const date = this.dateFromEvent({ composedPath: () => [target] })
      if (!date || this.isBelowMin(date)) return
      this.addRemoveDates([date])
    },

    /**
     * End the drag.
     *
     * Only swallows the event when the pointer actually moved onto another
     * date. That condition is load-bearing: `stopPropagation` on mouseup
     * prevents the browser from ever synthesising the `click`, and the click is
     * what Vuetify toggles on — so swallowing unconditionally made every plain
     * click do nothing at all. A real drag still gets swallowed, which is what
     * stops a sideways sweep from also flipping the month.
     */
    endDrag(e) {
      if (!this.dragging) return
      if (this.dragMoved) {
        e.preventDefault()
        e.stopPropagation()
      }
      this.dragging = false
      this.dragMoved = false
      this.dragStart = null
    },

    setDragState(date) {
      this.dragState = new Set(this.modelValue).has(date)
        ? this.dragStates.REMOVE
        : this.dragStates.ADD
    },
    /** Apply the current drag state to every date, in a single emit. */
    addRemoveDates(dates) {
      const set = new Set(this.modelValue)
      const before = set.size
      for (const date of dates) {
        if (this.dragState === this.dragStates.ADD) set.add(date)
        else set.delete(date)
      }
      if (set.size === before) return
      this.$emit("update:modelValue", [...set])
    },
  },

  mounted() {
    this.rootEl = this.$refs.root
    this.rootEl.addEventListener("mousedown", this.onPointerDown)
    this.rootEl.addEventListener("mouseover", this.onPointerOver)
    this.rootEl.addEventListener("touchstart", this.onPointerDown)
    this.rootEl.addEventListener("touchmove", this.onTouchMove)
    this.rootEl.addEventListener("mouseup", this.endDrag)
    this.rootEl.addEventListener("touchend", this.endDrag, { capture: true })
    // A drag released outside the calendar must still end it, or the next
    // hover keeps painting dates.
    window.addEventListener("mouseup", this.endDrag)
  },

  beforeUnmount() {
    this.rootEl.removeEventListener("mousedown", this.onPointerDown)
    this.rootEl.removeEventListener("mouseover", this.onPointerOver)
    this.rootEl.removeEventListener("touchstart", this.onPointerDown)
    this.rootEl.removeEventListener("touchmove", this.onTouchMove)
    this.rootEl.removeEventListener("mouseup", this.endDrag)
    this.rootEl.removeEventListener("touchend", this.endDrag, { capture: true })
    window.removeEventListener("mouseup", this.endDrag)
  },
}
</script>
