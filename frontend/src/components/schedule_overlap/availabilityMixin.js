import { getDateHoursOffset, get } from "@/utils"

/**
 * Aggregate-availability fetch/format methods for ScheduleOverlap.
 *
 * Extracted verbatim from ScheduleOverlap.vue as a Vue 2 mixin (TODO A5, step 2).
 * Mixin methods run against the same component instance, so every `this.*`
 * reference resolves exactly as before — a behavior-preserving move, not a
 * rewrite. The component keeps all the state these methods read/write.
 *
 * The computed block (added by TODO G2) is the read side of the same concern:
 * the server's `responses` object parsed into the shape everything else here
 * consumes, plus the aggregates derived from it.
 */
export default {
  computed: {
    /** Parses the responses to the gathering, makes necessary changes based on the type of event, and returns it */
    parsedResponses() {
      const parsed = {}

      // Return only current user availability if using blind availabilities and user is not owner
      if (this.event.blindAvailabilityEnabled && !this.isOwner) {
        // Keyed off the session only — the server applies the same rule, and
        // the ?guestName= escape hatch that bypassed it is gone.
        const userId = this.authUser?._id
        if (userId in this.event.responses) {
          const user = {
            ...this.event.responses[userId].user,
            _id: userId,
          }
          parsed[userId] = {
            ...this.event.responses[userId],
            availability: new Set(
              this.fetchedResponses[userId]?.availability?.map((a) =>
                new Date(a).getTime()
              )
            ),
            ifNeeded: new Set(
              this.fetchedResponses[userId]?.ifNeeded?.map((a) =>
                new Date(a).getTime()
              )
            ),
            user: user,
          }
        }
        return parsed
      }

      // Otherwise, parse responses so that if _id is null (i.e. guest user), then it is set to the guest user's name
      for (const k of Object.keys(this.event.responses)) {
        const newUser = {
          ...this.event.responses[k].user,
          _id: k,
        }
        parsed[k] = {
          ...this.event.responses[k],
          availability: new Set(
            this.fetchedResponses[k]?.availability?.map((a) =>
              new Date(a).getTime()
            )
          ),
          ifNeeded: new Set(
            this.fetchedResponses[k]?.ifNeeded?.map((a) =>
              new Date(a).getTime()
            )
          ),
          user: newUser,
        }
      }
      return parsed
    },
    respondents() {
      return Object.values(this.parsedResponses)
        .map((r) => r.user)
        .filter(Boolean)
    },
    /** The largest number of people available at any one time */
    max() {
      let max = 0
      for (const availability of this.responsesFormatted.values()) {
        if (availability.size > max) {
          max = availability.size
        }
      }

      return max
    },
    userHasResponded() {
      return this.authUser && this.authUser._id in this.parsedResponses
    },
  },
  methods: {
    /** Fetches responses from server */
    fetchResponses() {
      if (this.calendarOnly) {
        this.fetchedResponses = this.event.responses
        return
      }

      let timeMin, timeMax
      if (this.allDays.length > 0) {
        // Fetch the entire time range of availabilities
        timeMin = new Date(this.allDays[0].dateObject)
        timeMax = new Date(this.allDays[this.allDays.length - 1].dateObject)
        timeMax.setDate(timeMax.getDate() + 1)
      }

      if (!timeMin || !timeMax) return

      // Fetch responses between timeMin and timeMax
      let url = `/events/${
        this.event._id
      }/responses?timeMin=${timeMin.toISOString()}&timeMax=${timeMax.toISOString()}`

      get(url)
        .then((responses) => {
          this.fetchedResponses = responses
          this.getResponsesFormatted()
        })
        .catch(() => {
          this.showError(
            "There was an error fetching availability! Please refresh the page."
          )
        })
    },
    /** Formats the responses in a map where date/time is mapped to the people that are available then */
    getResponsesFormatted() {
      const lastFetched = new Date().getTime()
      this.loadingResponses.loading = true
      this.loadingResponses.lastFetched = lastFetched

      this.$worker
        .run(
          (days, times, parsedResponses, daysOnly, hideIfNeeded) => {
            // Define functions locally because we can't import functions
            const splitTimeNum = (timeNum) => {
              const hours = Math.floor(timeNum)
              const minutes = Math.floor((timeNum - hours) * 60)
              return { hours, minutes }
            }
            const getDateHoursOffset = (date, hoursOffset) => {
              const { hours, minutes } = splitTimeNum(hoursOffset)
              const newDate = new Date(date)
              newDate.setHours(newDate.getHours() + hours)
              newDate.setMinutes(newDate.getMinutes() + minutes)
              return newDate
            }

            // Create array of all dates in the event
            const dates = []
            if (daysOnly) {
              for (const day of days) {
                dates.push(day.dateObject)
              }
            } else {
              for (const day of days) {
                for (const time of times) {
                  // Iterate through all the times
                  const date = getDateHoursOffset(
                    day.dateObject,
                    time.hoursOffset
                  )
                  dates.push(date)
                }
              }
            }

            // Create a map mapping time to the respondents available during that time
            const formatted = new Map()
            for (const date of dates) {
              formatted.set(date.getTime(), new Set())

              // Check every response and see if they are available for the given time
              for (const response of Object.values(parsedResponses)) {
                // Check availability array
                if (
                  response.availability?.has(date.getTime()) ||
                  (response.ifNeeded?.has(date.getTime()) && !hideIfNeeded)
                ) {
                  formatted.get(date.getTime()).add(response.user._id)
                  continue
                }
              }
            }
            return formatted
          },
          [
            this.allDays,
            this.times,
            this.parsedResponses,
            this.event.daysOnly,
            this.hideIfNeeded,
          ]
        )
        .then((formatted) => {
          // Only set responses formatted for the latest request
          if (lastFetched >= this.loadingResponses.lastFetched) {
            this.responsesFormatted = formatted
          }
        })
        .finally(() => {
          if (this.loadingResponses.lastFetched === lastFetched) {
            this.loadingResponses.loading = false
          }
        })
    },
    /** Returns a set of respondents for the given date/time */
    getRespondentsForHoursOffset(date, hoursOffset) {
      const d = getDateHoursOffset(date, hoursOffset)
      return this.responsesFormatted.get(d.getTime()) ?? new Set()
    },
    showAvailability(row, col) {
      if (this.state === this.states.EDIT_AVAILABILITY && this.isPhone) {
        // Don't show currently selected timeslot when on phone and editing
        return
      }

      // Update current timeslot (the timeslot that has a dotted border around it)
      this.curTimeslot = { row, col }

      if (this.state === this.states.EDIT_AVAILABILITY || this.curRespondent) {
        // Don't show availability when editing or when respondent is selected
        return
      }

      const date = this.getDateFromRowCol(row, col)
      if (!date) return

      // Update current timeslot availability to show who is available for the given timeslot
      const available = this.responsesFormatted.get(date.getTime()) ?? new Set()
      for (const respondent of this.respondents) {
        if (available.has(respondent._id)) {
          this.curTimeslotAvailability[respondent._id] = true
        } else {
          this.curTimeslotAvailability[respondent._id] = false
        }
      }
    },
  },
}
