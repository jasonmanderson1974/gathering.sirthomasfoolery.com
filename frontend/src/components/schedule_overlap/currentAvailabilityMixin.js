import {
  dateCompare,
  getDateHoursOffset,
  post,
  _delete,
  get,
  getDateDayOffset,
  isDateBetween,
  getISODateString,
  getDateWithTimezone,
  timeNumToTimeString,
  splitTimeBlocksByDay,
} from "@/utils"
import { availabilityTypes, calendarOptionsDefaults } from "@/constants"
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
 *
 * The computed block (added by TODO G2) is the same concern read-only: the
 * user's own availability in the shapes the template wants — plain arrays, the
 * blocks drawn on top of everyone else's heatmap, and the calendar events those
 * are derived from.
 */
export default {
  computed: {
    /** Returns the availability as an array */
    availabilityArray() {
      return [...this.availability].map((item) => new Date(item))
    },
    /** Returns the if needed availability as an array */
    ifNeededArray() {
      return [...this.ifNeeded].map((item) => new Date(item))
    },
    /** Returns an array of calendar events for all of the authUser's enabled calendars, separated by the day they occur on */
    calendarEventsByDay() {
      // If this is an example calendar
      if (this.sampleCalendarEventsByDay) return this.sampleCalendarEventsByDay

      // If the user isn't logged in or is adding availability as a guest
      if (!this.authUser || this.addingAvailabilityAsGuest) return []

      let events = []
      let event

      const calendarAccounts = this.authUser.calendarAccounts

      // Adds events from calendar accounts that are enabled
      for (const id in calendarAccounts) {
        if (!calendarAccounts[id].enabled) continue

        if (Object.prototype.hasOwnProperty.call(this.calendarEventsMap, id)) {
          for (const index in this.calendarEventsMap[id].calendarEvents) {
            event = this.calendarEventsMap[id].calendarEvents[index]

            // Check if we need to update authUser (to get latest subcalendars)
            const subCalendars = calendarAccounts[id].subCalendars
            if (!subCalendars || !(event.calendarId in subCalendars)) {
              // authUser doesn't contain the subCalendar, so push event to events without checking if subcalendar is enabled
              // and queue the authUser to be refreshed
              events.push(event)
              if (!this.hasRefreshedAuthUser) {
                this.refreshAuthUser()
              }
              continue
            }

            // Push event to events if subcalendar is enabled
            if (subCalendars[event.calendarId].enabled) {
              events.push(event)
            }
          }
        }
      }

      const eventsCopy = JSON.parse(JSON.stringify(events))

      const calendarEventsByDay = splitTimeBlocksByDay(
        this.event,
        eventsCopy,
        this.weekOffset,
        this.timezoneOffset
      )

      return calendarEventsByDay
    },
    /** Returns an array of time blocks representing the current user's availability
     * (used for displaying current user's availability on top of everybody else's availability)
     */
    overlaidAvailability() {
      const overlaidAvailability = []
      this.days.forEach((day, d) => {
        overlaidAvailability.push([])
        let curBlockIndex = 0
        const addOverlaidAvailabilityBlocks = (time, t) => {
          const date = this.getDateFromRowCol(t, d)
          if (!date) return

          const dragAdd =
            this.dragging &&
            this.inDragRange(t, d) &&
            this.dragType === this.DRAG_TYPES.ADD
          const dragRemove =
            this.dragging &&
            this.inDragRange(t, d) &&
            this.dragType === this.DRAG_TYPES.REMOVE

          // Check if timeslot is available or if needed or in the drag region
          if (
            dragAdd ||
            (!dragRemove &&
              (this.availability.has(date.getTime()) ||
                this.ifNeeded.has(date.getTime())))
          ) {
            // Determine whether to render as available or if needed block
            let type = availabilityTypes.AVAILABLE
            if (dragAdd) {
              type = this.availabilityType
            } else {
              type = this.availability.has(date.getTime())
                ? availabilityTypes.AVAILABLE
                : availabilityTypes.IF_NEEDED
            }

            if (curBlockIndex in overlaidAvailability[d]) {
              if (overlaidAvailability[d][curBlockIndex].type === type) {
                // Increase block length if matching type and curBlockIndex exists
                overlaidAvailability[d][curBlockIndex].hoursLength += 0.25
              } else {
                // Add a new block because type is different
                overlaidAvailability[d].push({
                  hoursOffset: time.hoursOffset,
                  hoursLength: 0.25,
                  type,
                })
                curBlockIndex++
              }
            } else {
              // Add a new block because block doesn't exist for current index
              overlaidAvailability[d].push({
                hoursOffset: time.hoursOffset,
                hoursLength: 0.25,
                type,
              })
            }
          } else if (curBlockIndex in overlaidAvailability[d]) {
            // Only increment cur block index if block already exists at the current index
            curBlockIndex++
          }
        }
        for (let t = 0; t < this.splitTimes[0].length; ++t) {
          addOverlaidAvailabilityBlocks(this.splitTimes[0][t], t)
        }
        if (curBlockIndex in overlaidAvailability[d]) {
          curBlockIndex++
        }
        for (let t = 0; t < this.splitTimes[1].length; ++t) {
          addOverlaidAvailabilityBlocks(
            this.splitTimes[1][t],
            t + this.splitTimes[0].length
          )
        }
      })
      return overlaidAvailability
    },
  },
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

      payload.availability = this.availabilityArray
      payload.ifNeeded = this.ifNeededArray
      if (this.addingAvailabilityAsGuest) {
        // On-behalf entry: a signed-in member filling in availability for
        // someone without an account. No localStorage identity — that existed
        // to remember an ANONYMOUS visitor between reloads, and there are none.
        payload.guest = true
        payload.name = guestPayload.name
        payload.email = guestPayload.email
      } else {
        payload.guest = false
      }

      await post(`/events/${this.event._id}/response`, payload)

      this.refreshEvent()
      this.unsavedChanges = false
    },
    async deleteAvailability(name = "") {
      const payload = {}
      if (this.authUser && !this.addingAvailabilityAsGuest) {
        payload.guest = false
        payload.userId = this.authUser._id
      } else {
        payload.guest = true
        payload.name = name
      }
      await _delete(`/events/${this.event._id}/response`, payload)
      this.availability = new Set()
      this.refreshEvent()
    },
  },
}
