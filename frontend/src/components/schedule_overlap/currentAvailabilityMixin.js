import {
  dateCompare,
  getDateHoursOffset,
  post,
  put,
  _delete,
  get,
  getDateDayOffset,
  isDateBetween,
  getISODateString,
  getDateWithTimezone,
  timeNumToTimeString,
} from "@/utils"
import { calendarOptionsDefaults } from "@/constants"
import dayjs from "dayjs"

/**
 * "Current user availability" methods for ScheduleOverlap — reset/populate the
 * signed-in user's own availability, derive it from calendar events, and the
 * fill animation (getAvailabilityFromCalendarEvents, setAvailabilityAutomatically,
 * animateAvailability, stopAvailabilityAnim).
 *
 * Extracted verbatim from ScheduleOverlap.vue as a Vue 2 mixin (TODO A5, step 3).
 * Mixin methods run against the same component instance, so every `this.*`
 * reference (data, computed, mapped Vuex actions, other methods) resolves exactly
 * as before — a behavior-preserving move, not a rewrite.
 */
export default {
  methods: {
    async refreshAuthUser() {
      this.hasRefreshedAuthUser = true
      await get("/user/profile").then((authUser) => {
        this.setAuthUser(authUser)
      })
    },
    /** resets cur user availability to the response stored on the server */
    resetCurUserAvailability() {
      this.availability = new Set()
      this.ifNeeded = new Set()
      if (this.userHasResponded) {
        this.populateUserAvailability(this.authUser._id)
      }
    },
    /** Populates the availability set for the auth user from the responses object stored on the server */
    populateUserAvailability(id) {
      this.availability =
        new Set(this.parsedResponses[id]?.availability) ?? new Set()
      this.ifNeeded = new Set(this.parsedResponses[id]?.ifNeeded) ?? new Set()
      this.$nextTick(() => (this.unsavedChanges = false))
    },
    /** Returns true if the calendar event is in the first split */
    getIsTimeBlockInFirstSplit(timeBlock) {
      return (
        timeBlock.hoursOffset >= this.splitTimes[0][0].hoursOffset &&
        timeBlock.hoursOffset <=
          this.splitTimes[0][this.splitTimes[0].length - 1].hoursOffset
      )
    },
    /** Returns the style for the calendar event block */
    getTimeBlockStyle(timeBlock) {
      const style = {}
      const hasSecondSplit = this.splitTimes[1].length > 0
      if (!hasSecondSplit || this.getIsTimeBlockInFirstSplit(timeBlock)) {
        style.top = `calc(${
          timeBlock.hoursOffset - this.splitTimes[0][0].hoursOffset
        } * ${this.HOUR_HEIGHT}px)`
        style.height = `calc(${timeBlock.hoursLength} * ${this.HOUR_HEIGHT}px)`
      } else {
        style.top = `calc(${this.splitTimes[0].length} * ${
          this.timeslotHeight
        }px + ${this.SPLIT_GAP_HEIGHT}px + ${
          timeBlock.hoursOffset - this.splitTimes[1][0].hoursOffset
        } * ${this.HOUR_HEIGHT}px)`
        style.height = `calc(${timeBlock.hoursLength} * ${this.HOUR_HEIGHT}px)`
      }
      return style
    },
    /** Returns a set containing the available times based on the given calendar events object */
    getAvailabilityFromCalendarEvents({
      calendarEventsByDay = [],
      calendarOptions = calendarOptionsDefaults, // User id of the user we are getting availability for
    }) {
      const availability = new Set()

      for (let i = 0; i < this.allDays.length; ++i) {
        const day = this.allDays[i]
        const date = day.dateObject

        // Calculate buffer time
        const bufferTimeInMS = calendarOptions.bufferTime.enabled
          ? calendarOptions.bufferTime.time * 1000 * 60
          : 0

        // Calculate working hours
        const startTimeString = timeNumToTimeString(
          calendarOptions.workingHours.startTime
        )
        const isoDateString = getISODateString(getDateWithTimezone(date), true)
        const workingHoursStartDate = dayjs
          .tz(`${isoDateString} ${startTimeString}`, this.curTimezone.value)
          .toDate()
        let duration =
          calendarOptions.workingHours.endTime -
          calendarOptions.workingHours.startTime
        if (duration <= 0) duration += 24
        const workingHoursEndDate = getDateHoursOffset(
          workingHoursStartDate,
          duration
        )

        for (let j = 0; j < this.times.length; ++j) {
          const startDate = this.getDateFromDayTimeIndex(i, j)
          if (!startDate) continue
          const endDate = getDateHoursOffset(
            startDate,
            this.timeslotDuration / 60
          )

          // Working hours
          if (calendarOptions.workingHours.enabled) {
            if (
              endDate.getTime() <= workingHoursStartDate.getTime() ||
              startDate.getTime() >= workingHoursEndDate.getTime()
            ) {
              continue
            }
          }

          // Check if there exists a calendar event that overlaps [startDate, endDate]
          const index = calendarEventsByDay[i]?.findIndex((e) => {
            const startDateBuffered = new Date(
              e.startDate.getTime() - bufferTimeInMS
            )
            const endDateBuffered = new Date(
              e.endDate.getTime() + bufferTimeInMS
            )

            const notIntersect =
              dateCompare(endDate, startDateBuffered) <= 0 ||
              dateCompare(startDate, endDateBuffered) >= 0
            return !notIntersect && !e.free
          })
          if (index === -1) {
            availability.add(startDate.getTime())
          }
        }
      }
      return availability
    },
    /** Constructs the availability array using calendarEvents array */
    setAvailabilityAutomatically() {
      // This is not a computed property because we should be able to change it manually from what it automatically fills in
      this.availability = new Set()
      const tmpAvailability = this.getAvailabilityFromCalendarEvents({
        calendarEventsByDay: this.calendarEventsByDay,
        calendarOptions: {
          bufferTime: this.bufferTime,
          workingHours: this.workingHours,
        },
      })

      const pageStartDate = getDateDayOffset(
        new Date(this.event.dates[0]),
        this.page * this.maxDaysPerPage
      )
      const pageEndDate = getDateDayOffset(pageStartDate, this.maxDaysPerPage)
      this.animateAvailability(tmpAvailability, pageStartDate, pageEndDate)
    },
    /** Animate the filling out of availability using setTimeout, between startDate and endDate */
    animateAvailability(availability, startDate, endDate) {
      this.availabilityAnimEnabled = true
      this.availabilityAnimTimeouts = []

      let msPerGroup = 25
      let blocksPerGroup = 2
      if (
        (availability.size / blocksPerGroup) * msPerGroup >
        this.maxAnimTime
      ) {
        blocksPerGroup = (availability.size * msPerGroup) / this.maxAnimTime
      }
      let availabilityArray = [...availability]
      availabilityArray = availabilityArray.filter((a) =>
        isDateBetween(a, startDate, endDate)
      )

      for (let i = 0; i < availabilityArray.length / blocksPerGroup + 1; ++i) {
        const timeout = setTimeout(() => {
          for (const a of availabilityArray.slice(
            i * blocksPerGroup,
            i * blocksPerGroup + blocksPerGroup
          )) {
            this.availability.add(a)
          }
          this.availability = new Set(this.availability)
          if (i >= availabilityArray.length / blocksPerGroup) {
            // Make sure the entire availability has been added (will not be guaranteed when only animating a portion of availability)
            this.availability = new Set(availability)
            this.availabilityAnimTimeouts.push(
              setTimeout(() => {
                this.availabilityAnimEnabled = false

                if (this.showSnackbar) {
                  this.showInfo("Your availability has been autofilled!")
                }
                this.unsavedChanges = false
              }, 500)
            )
          }
        }, i * msPerGroup)

        this.availabilityAnimTimeouts.push(timeout)
      }
    },
    stopAvailabilityAnim() {
      for (const timeout of this.availabilityAnimTimeouts) {
        clearTimeout(timeout)
      }
      this.availabilityAnimEnabled = false
    },
    async submitAvailability(guestPayload = { name: "", email: "" }) {
      const payload = {}

      const type = "availability"
      payload.availability = this.availabilityArray
      payload.ifNeeded = this.ifNeededArray
      if (this.authUser && !this.addingAvailabilityAsGuest) {
        payload.guest = false
      } else {
        payload.guest = true
        payload.name = guestPayload.name
        payload.email = guestPayload.email

        localStorage[this.guestNameKey] = guestPayload.name
      }

      await post(`/events/${this.event._id}/response`, payload)

      // Update analytics
      const addedIfNeededTimes = this.ifNeededArray.length > 0
      if (this.authUser) {
        if (this.authUser._id in this.parsedResponses) {
          this.$posthog?.capture(`Edited ${type}`, {
            eventId: this.event._id,
            addedIfNeededTimes,
          })
        } else {
          this.$posthog?.capture(`Added ${type}`, {
            eventId: this.event._id,
            addedIfNeededTimes,
            // bufferTime: this.bufferTime,
            bufferTime: this.bufferTime.time,
            bufferTimeActive: this.bufferTime.enabled,
            workingHoursEnabled: this.workingHours.enabled,
            workingHoursStartTime: this.workingHours.startTime,
            workingHoursEndTime: this.workingHours.endTime,
          })
        }
      } else {
        if (guestPayload.name in this.parsedResponses) {
          this.$posthog?.capture(`Edited ${type} as guest`, {
            eventId: this.event._id,
            addedIfNeededTimes,
          })
        } else {
          this.$posthog?.capture(`Added ${type} as guest`, {
            eventId: this.event._id,
            addedIfNeededTimes,
          })
        }
      }

      this.refreshEvent()
      this.unsavedChanges = false
    },
    async submitNewSignUpBlocks() {
      if (
        this.signUpBlocksToAddByDay.flat().length +
          this.signUpBlocksByDay.flat().length ===
        0
      ) {
        this.showError("Please add at least one sign-up block!")
        return false
      }

      for (let i = 0; i < this.signUpBlocksToAddByDay.length; ++i) {
        this.signUpBlocksByDay[i] = this.signUpBlocksByDay[i].concat(
          this.signUpBlocksToAddByDay[i]
        )
        this.signUpBlocksToAddByDay[i] = []
      }

      const payload = {
        name: this.event.name,
        duration: this.event.duration,
        dates: this.event.dates,
        type: this.event.type,
        signUpBlocks: this.signUpBlocksByDay.flat().map((block) => {
          return {
            _id: block._id,
            name: block.name,
            capacity: block.capacity,
            startDate: block.startDate,
            endDate: block.endDate,
          }
        }),
      }

      put(`/events/${this.event._id}`, payload)
        .then(() => {
          // window.location.reload()
        })
        .catch((err) => {
          console.error(err)
          this.showError(
            "There was a problem editing this event! Please try again later."
          )
        })

      return true
    },

    async deleteAvailability(name = "") {
      const payload = {}
      if (this.authUser && !this.addingAvailabilityAsGuest) {
        payload.guest = false
        payload.userId = this.authUser._id

        this.$posthog?.capture("Deleted availability", {
          eventId: this.event._id,
        })
      } else {
        payload.guest = true
        payload.name = name

        this.$posthog?.capture("Deleted availability as guest", {
          eventId: this.event._id,
          name,
        })
      }
      await _delete(`/events/${this.event._id}/response`, payload)
      this.availability = new Set()
      this.refreshEvent()
    },
  },
}
