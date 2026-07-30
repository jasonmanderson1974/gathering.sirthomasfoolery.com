/**
 * Options-panel toggle/handler methods for ScheduleOverlap.
 *
 * Extracted verbatim from ScheduleOverlap.vue as a Vue 2 mixin (TODO A5).
 * Mixin methods run against the same component instance, so every `this.*`
 * reference (state, showEditOptions, ...) resolves exactly as
 * before — a behavior-preserving move, not a rewrite.
 */
export default {
  methods: {
    getLocalTimezone() {
      const split = new Date(this.event.dates[0])
        .toLocaleTimeString("en-us", { timeZoneName: "short" })
        .split(" ")
      const localTimezone = split[split.length - 1]

      return localTimezone
    },
    onShowBestTimesChange() {
      localStorage["showBestTimes"] = this.showBestTimes
      if (
        this.state == this.states.BEST_TIMES ||
        this.state == this.states.HEATMAP
      )
        this.state = this.defaultState
    },
    toggleShowEditOptions() {
      this.showEditOptions = !this.showEditOptions
      localStorage["showEditOptions"] = this.showEditOptions
    },
    toggleShowEventOptions() {
      this.showEventOptions = !this.showEventOptions
      localStorage["showEventOptions"] = this.showEventOptions
    },
    updateOverlayAvailability(val) {
      this.overlayAvailability = !!val
    },
  },
}
