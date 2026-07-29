/**
 * Respondent hover/selection methods for ScheduleOverlap.
 *
 * Extracted verbatim from ScheduleOverlap.vue as a Vue 2 mixin (TODO A5).
 * Mixin methods run against the same component instance, so every `this.*`
 * reference resolves exactly as before — a behavior-preserving move, not a
 * rewrite. `deselectRespondents` is registered as a window click listener in
 * ScheduleOverlap's mounted/destroyed hooks.
 */
export default {
  methods: {
    mouseOverRespondent(e, id) {
      if (this.curRespondents.length === 0) {
        if (this.state === this.defaultState) {
          this.state = this.states.SINGLE_AVAILABILITY
        }

        this.curRespondent = id
      }
    },
    mouseLeaveRespondent() {
      if (this.curRespondents.length === 0) {
        if (this.state === this.states.SINGLE_AVAILABILITY) {
          this.state = this.defaultState
        }

        this.curRespondent = ""
      }
    },
    clickRespondent(e, id) {
      this.state = this.states.SUBSET_AVAILABILITY
      this.curRespondent = ""

      if (this.curRespondentsSet.has(id)) {
        // Remove id
        this.curRespondents = this.curRespondents.filter((r) => r != id)

        // Go back to default state if all users deselected
        if (this.curRespondents.length === 0) {
          this.state = this.defaultState
        }
      } else {
        // Add id
        this.curRespondents.push(id)
      }

      e.stopPropagation()
    },
    deselectRespondents(e) {
      // Don't deselect respondents if toggled best times
      // or if this was fired by clicking on a timeslot
      if (
        e?.target?.previousElementSibling?.id === "show-best-times-toggle" ||
        e?.target?.firstChild?.firstChild?.id === "show-best-times-toggle" ||
        e?.target?.classList?.contains("timeslot") //&& this.isPhone)
      )
        return

      if (this.state === this.states.SUBSET_AVAILABILITY) {
        this.state = this.defaultState
      }

      this.curRespondents = []

      // Stop persisting timeslot
      this.timeslotSelected = false
      this.resetCurTimeslot()
    },

    isGuest(user) {
      return user._id == user.firstName
    },
  },
}
