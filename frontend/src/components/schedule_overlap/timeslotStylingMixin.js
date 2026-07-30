import { lightOrDark, removeTransparencyFromHex } from "@/utils"
import { availabilityTypes, timeTypes } from "@/constants"
import { getResponseCountColorClass } from "./responseCounts"
import dayjs from "dayjs"
import utcPlugin from "dayjs/plugin/utc"
dayjs.extend(utcPlugin)

/**
 * Timeslot sizing/styling/event-handler methods for ScheduleOverlap.
 *
 * Extracted verbatim from ScheduleOverlap.vue as a Vue 2 mixin (TODO A5).
 * Mixin methods run against the same component instance, so every `this.*`
 * reference resolves exactly as before — a behavior-preserving move, not a
 * rewrite. The component template binds these via the timeslotClassStyle /
 * timeslotVon maps below — computed fan-outs that call the per-cell methods
 * once for every cell of the current page, so the template can index a flat
 * array instead of invoking a method per cell on every render (TODO G2).
 */
export default {
  computed: {
    timeslotClassStyle() {
      const classStyles = []
      for (let d = 0; d < this.days.length; ++d) {
        const day = this.days[d]
        for (let t = 0; t < this.splitTimes[0].length; ++t) {
          const time = this.splitTimes[0][t]
          classStyles.push(this.getTimeTimeslotClassStyle(day, time, d, t))
        }
        for (let t = 0; t < this.splitTimes[1].length; ++t) {
          const time = this.splitTimes[1][t]
          classStyles.push(
            this.getTimeTimeslotClassStyle(
              day,
              time,
              d,
              t + this.splitTimes[0].length
            )
          )
        }
      }
      return classStyles
    },
    dayTimeslotClassStyle() {
      const classStyles = []
      for (let i = 0; i < this.monthDays.length; ++i) {
        classStyles.push(
          this.getDayTimeslotClassStyle(this.monthDays[i].dateObject, i)
        )
      }
      return classStyles
    },
    /** Per-cell traffic-light response counts for the time grid (indexed like timeslotClassStyle) */
    timeslotCounts() {
      const counts = []
      for (let d = 0; d < this.days.length; ++d) {
        for (let t = 0; t < this.splitTimes[0].length; ++t) {
          counts.push(this.getTimeslotCount(this.getDateFromRowCol(t, d)))
        }
        for (let t = 0; t < this.splitTimes[1].length; ++t) {
          counts.push(
            this.getTimeslotCount(
              this.getDateFromRowCol(t + this.splitTimes[0].length, d)
            )
          )
        }
      }
      return counts
    },
    /** Per-cell traffic-light response counts for the daysOnly grid */
    dayTimeslotCounts() {
      const counts = []
      for (let i = 0; i < this.monthDays.length; ++i) {
        const date = this.monthDays[i].dateObject
        counts.push(
          this.monthDayIncluded.get(date.getTime())
            ? this.getTimeslotCount(date)
            : null
        )
      }
      return counts
    },
    timeslotVon() {
      const vons = []
      for (let d = 0; d < this.days.length; ++d) {
        for (let t = 0; t < this.times.length; ++t) {
          vons.push(this.getTimeslotVon(t, d))
        }
      }
      return vons
    },
    dayTimeslotVon() {
      const vons = []
      for (let i = 0; i < this.monthDays.length; ++i) {
        const row = Math.floor(i / 7)
        const col = i % 7
        vons.push(this.getTimeslotVon(row, col))
      }
      return vons
    },
  },
  methods: {
    /**
     * Returns the traffic-light response count to render inside a timeslot, or
     * null when nothing should be shown. The count is the number of respondents
     * leaning available (available or if-needed) at the given date; the color
     * reflects that count as a share of the total respondents.
     */
    getTimeslotCount(date) {
      if (!this.showResponseCounts || !date) return null
      if (this.respondents.length === 0) return null

      const countableStates = [
        this.states.HEATMAP,
        this.states.BEST_TIMES,
        this.states.SCHEDULE_EVENT,
        this.states.SUBSET_AVAILABILITY,
        this.states.EDIT_AVAILABILITY,
      ]
      if (!countableStates.includes(this.state)) return null

      // In whole-block mode, show a single pill at the start of each block
      // rather than repeating the count in every slot of the block.
      if (this.isWholeBlockSelection) {
        const block = this.slotToBlock.get(date.getTime())
        if (!block || block.start !== date.getTime()) return null
      }

      const respondentsSet =
        this.responsesFormatted.get(date.getTime()) ?? new Set()

      let count
      let total
      if (this.state === this.states.SUBSET_AVAILABILITY) {
        count = [...respondentsSet].filter((r) =>
          this.curRespondentsSet.has(r)
        ).length
        total = this.curRespondents.length
      } else {
        count = respondentsSet.size
        total = this.respondents.length
      }

      if (count === 0 || total === 0) return null

      return {
        text: String(count),
        class: getResponseCountColorClass(count, total),
      }
    },
    setTimeslotSize() {
      /* Gets the dimensions of each timeslot and assigns it to the timeslot variable */
      const timeslotEl = document.querySelector(".timeslot")
      if (timeslotEl) {
        const { width, height } = timeslotEl.getBoundingClientRect()
        this.timeslot.width = width
        this.timeslot.height = height
      }
    },
    /** Returns a class string and style object for the given time timeslot div */
    getTimeTimeslotClassStyle(day, time, d, t) {
      const row = t
      const col = d
      const date = this.getDateFromRowCol(row, col)
      const classStyle = this.getTimeslotClassStyle(date, row, col)

      // Add time timeslot specific stuff
      const isFirstSplit = t < this.splitTimes[0].length
      const isDisabled = !date

      // Animation
      if (this.animateTimeslotAlways || this.availabilityAnimEnabled) {
        classStyle.class += "animate-bg-color "
      }

      // Height
      classStyle.style.height = `${this.timeslotHeight}px`

      // Border style
      if (
        (this.respondents.length > 0 ||
          this.editing ||
          this.state === this.states.SET_SPECIFIC_TIMES) &&
        this.curTimeslot.row === row &&
        this.curTimeslot.col === col &&
        !isDisabled
      ) {
        // Dashed border for currently selected timeslot
        classStyle.class +=
          "tw-border tw-border-dashed tw-border-black tw-z-10 "
      } else {
        // Normal border
        if (date) {
          const localDate = new Date(
            date.getTime() - this.timezoneOffset * 60 * 1000
          )
          const fractionalTime = localDate.getMinutes()
          if (fractionalTime === 0) {
            classStyle.class += "tw-border-t "
          } else if (fractionalTime === 30) {
            classStyle.class += "tw-border-t "
            classStyle.style.borderTopStyle = "dashed"
          }
        }

        classStyle.class += "tw-border-r "
        if (col === 0 || !this.isColConsecutive(col))
          classStyle.class += "tw-border-l tw-border-l-gray "
        if (col === this.days.length - 1 || !this.isColConsecutive(col + 1))
          classStyle.class += "tw-border-r-gray "
        if (isFirstSplit && row === 0)
          classStyle.class += "tw-border-t tw-border-t-gray "
        if (!isFirstSplit && row === this.splitTimes[0].length)
          classStyle.class += "tw-border-t tw-border-t-gray "
        if (isFirstSplit && row === this.splitTimes[0].length - 1)
          classStyle.class += "tw-border-b tw-border-b-gray "
        if (
          !isFirstSplit &&
          row === this.splitTimes[0].length + this.splitTimes[1].length - 1
        )
          classStyle.class += "tw-border-b tw-border-b-gray "

        const totalRespondents =
          this.state === this.states.SUBSET_AVAILABILITY
            ? this.curRespondents.length
            : this.respondents.length
        if (
          this.state === this.states.EDIT_AVAILABILITY ||
          this.state === this.states.SINGLE_AVAILABILITY ||
          totalRespondents === 1
        ) {
          classStyle.class += "tw-border-[#999999] "
        } else {
          classStyle.class += "tw-border-[#DDDDDD99] "
        }
      }

      // Edit fill color and border color if day is not interactable
      if (isDisabled) {
        classStyle.class += "tw-bg-light-gray-stroke tw-border-brass-dim "
      }

      // Change default red:
      if (classStyle.style.backgroundColor === "#E523230D") {
        classStyle.style.backgroundColor = "#E5232333"
      }

      return classStyle
    },
    /** Returns the shared class string and style object for the given timeslot (either time timeslot or day timeslot) */
    getTimeslotClassStyle(date, row, col) {
      let c = ""
      const s = {}
      if (!date) return { class: c, style: s }

      const timeslotRespondents =
        this.responsesFormatted.get(date.getTime()) ?? new Set()

      // Fill style

      if (
        (!this.overlayAvailability &&
          this.state === this.states.EDIT_AVAILABILITY) ||
        this.state === this.states.SET_SPECIFIC_TIMES
      ) {
        // Set default background color to red (unavailable)
        s.backgroundColor = "#E523230D"

        // Show only current user availability
        const inDragRange = this.inDragRange(row, col)
        if (inDragRange) {
          // Set style if drag range goes over the current timeslot
          if (this.dragType === this.DRAG_TYPES.ADD) {
            if (this.state === this.states.SET_SPECIFIC_TIMES) {
              c += "tw-bg-white "
            } else {
              if (this.availabilityType === availabilityTypes.AVAILABLE) {
                s.backgroundColor = "#00994C77"
              } else if (
                this.availabilityType === availabilityTypes.IF_NEEDED
              ) {
                c += "tw-bg-yellow "
              }
            }
          } else if (this.dragType === this.DRAG_TYPES.REMOVE) {
            if (this.state === this.states.SET_SPECIFIC_TIMES) {
              c += "tw-bg-gray "
            }
          }
        } else {
          // Otherwise just show the current availability
          // Show current availability from availability set
          if (this.state === this.states.SET_SPECIFIC_TIMES) {
            if (this.tempTimes.has(date.getTime())) {
              c += "tw-bg-white "
            } else {
              c += "tw-bg-gray "
            }
          } else {
            if (this.availability.has(date.getTime())) {
              s.backgroundColor = "#00994C77"
            } else if (this.ifNeeded.has(date.getTime())) {
              c += "tw-bg-yellow "
            }
          }
        }
      }

      if (this.state === this.states.SINGLE_AVAILABILITY) {
        // Show only the currently selected respondent's availability
        const respondent = this.curRespondent
        if (timeslotRespondents.has(respondent)) {
          if (this.parsedResponses[respondent]?.ifNeeded?.has(date.getTime())) {
            c += "tw-bg-yellow "
          } else {
            s.backgroundColor = "#00994C77"
          }
        } else {
          s.backgroundColor = "#E523230D"
        }
        return { class: c, style: s }
      }

      if (
        this.overlayAvailability ||
        this.state === this.states.BEST_TIMES ||
        this.state === this.states.HEATMAP ||
        this.state === this.states.SCHEDULE_EVENT ||
        this.state === this.states.SUBSET_AVAILABILITY
      ) {
        let numRespondents
        let max

        if (
          this.state === this.states.BEST_TIMES ||
          this.state === this.states.HEATMAP ||
          this.state === this.states.SCHEDULE_EVENT
        ) {
          numRespondents = timeslotRespondents.size
          max = this.max
        } else if (this.state === this.states.SUBSET_AVAILABILITY) {
          numRespondents = [...timeslotRespondents].filter((r) =>
            this.curRespondentsSet.has(r)
          ).length

          max = this.curRespondentsMax
        } else if (this.overlayAvailability) {
          if (
            (this.userHasResponded || this.curGuestId?.length > 0) &&
            timeslotRespondents.has(this.authUser?._id ?? this.curGuestId)
          ) {
            // Subtract 1 because we do not want to include current user's availability
            numRespondents = timeslotRespondents.size - 1
            max = this.max
          } else {
            numRespondents = timeslotRespondents.size
            max = this.max
          }
        }

        const totalRespondents =
          this.state === this.states.SUBSET_AVAILABILITY
            ? this.curRespondents.length
            : this.respondents.length

        if (this.defaultState === this.states.BEST_TIMES) {
          if (max > 0 && numRespondents === max) {
            // Only set timeslot to green for the times that most people are available
            if (totalRespondents === 1 || this.overlayAvailability) {
              // Make single responses less saturated
              const green = "#00994C88"
              s.backgroundColor = green
            } else {
              const green = "#00994C"
              s.backgroundColor = green
            }
          }
        } else if (this.defaultState === this.states.HEATMAP) {
          if (numRespondents > 0) {
            if (totalRespondents === 1) {
              const respondentId =
                this.state === this.states.SUBSET_AVAILABILITY
                  ? this.curRespondents[0]
                  : this.respondents[0]._id
              if (
                this.parsedResponses[respondentId]?.ifNeeded?.has(
                  date.getTime()
                )
              ) {
                c += "tw-bg-yellow "
              } else {
                const green = "#00994C88"
                s.backgroundColor = green
              }
            } else {
              // Determine color of timeslot based on number of people available
              const frac = numRespondents / max
              const green = "#00994C"
              let alpha
              if (!this.overlayAvailability) {
                alpha = Math.floor(frac * (255 - 30))
                  .toString(16)
                  .toUpperCase()
                  .substring(0, 2)
                  .padStart(2, "0")
                if (
                  frac == 1 &&
                  ((this.curRespondents.length > 0 &&
                    max === this.curRespondents.length) ||
                    (this.curRespondents.length === 0 &&
                      max === this.respondents.length))
                ) {
                  alpha = "FF"
                }
              } else {
                alpha = Math.floor(frac * (255 - 85))
                  .toString(16)
                  .toUpperCase()
                  .substring(0, 2)
                  .padStart(2, "0")
              }

              s.backgroundColor = green + alpha
            }
          } else if (totalRespondents === 1) {
            const red = "#E523230D"
            s.backgroundColor = red
          }
        }
      }

      return { class: c, style: s }
    },
    getDayTimeslotClassStyle(date, i) {
      const row = Math.floor(i / 7)
      const col = i % 7

      let classStyle
      // Only compute class style for days that are included
      if (this.monthDayIncluded.get(date.getTime())) {
        classStyle = this.getTimeslotClassStyle(date, row, col)
        if (this.state === this.states.EDIT_AVAILABILITY) {
          classStyle.class += "tw-cursor-pointer "
        }

        const backgroundColor = classStyle.style.backgroundColor
        if (
          backgroundColor &&
          lightOrDark(removeTransparencyFromHex(backgroundColor)) === "dark"
        ) {
          classStyle.class += "tw-text-white "
        }
      } else {
        classStyle = {
          class: "tw-bg-leather tw-text-parchment-dim ",
          style: {},
        }
      }

      // Change default red:
      if (classStyle.style.backgroundColor === "#E523230D") {
        classStyle.style.backgroundColor = "#E523233B"
      }

      // Change edit green
      // if (classStyle.style.backgroundColor === "#00994C88") {
      //   classStyle.style.backgroundColor = "#29BC6880"
      // }

      // Border style
      if (
        (this.respondents.length > 0 ||
          this.state === this.states.EDIT_AVAILABILITY) &&
        this.curTimeslot.row === row &&
        this.curTimeslot.col === col &&
        this.monthDayIncluded.get(date.getTime())
      ) {
        // Dashed border for currently selected timeslot
        classStyle.class +=
          "tw-outline-2 tw-outline-dashed tw-outline-black tw-z-10 "
      } else {
        // Normal border
        if (col === 0) classStyle.class += "tw-border-l tw-border-l-gray "
        classStyle.class += "tw-border-r tw-border-r-gray "
        if (col !== 7 - 1) {
          classStyle.style.borderRightStyle = "dashed"
        }

        if (row === 0) classStyle.class += "tw-border-t tw-border-t-gray "
        classStyle.class += "tw-border-b tw-border-b-gray "
        if (row !== Math.floor(this.monthDays.length / 7)) {
          classStyle.style.borderBottomStyle = "dashed"
        }
      }

      return classStyle
    },
    getTimeslotVon(row, col) {
      if (this.interactable) {
        return {
          click: () => {
            if (this.timeslotSelected) {
              // Get rid of persistent timeslot selection if clicked on the same timeslot that is currently being persisted
              if (
                row === this.curTimeslot.row &&
                col === this.curTimeslot.col
              ) {
                this.timeslotSelected = false
              }
            } else if (
              this.state !== this.states.EDIT_AVAILABILITY &&
              this.userHasResponded
            ) {
              // Persist timeslot selection if user has already responded
              this.timeslotSelected = true
            }

            this.showAvailability(row, col)
          },
          mousedown: () => {
            // Highlight availability button
            if (
              this.state === this.defaultState &&
              ((!this.isPhone && !this.userHasResponded) ||
                this.respondents.length == 0)
            )
              this.highlightAvailabilityBtn()
          },
          mouseover: () => {
            // Only show availability on hover if timeslot is not being persisted
            if (!this.timeslotSelected) {
              this.showAvailability(row, col)
              if (!this.event.daysOnly) {
                const date = this.getDateFromRowCol(row, col)
                if (date) {
                  // Debug logging for hover slot

                  date.setTime(date.getTime() - this.timezoneOffset * 60 * 1000)
                  const startDate = dayjs(date).utc()
                  const endDate = dayjs(date)
                    .utc()
                    .add(this.timeslotDuration, "minutes")

                  const timeFormat =
                    this.timeType === timeTypes.HOUR12 ? "h:mm A" : "HH:mm"
                  let dateFormat
                  if (this.isSpecificDates) {
                    dateFormat = "ddd, MMM D, YYYY"
                  } else {
                    dateFormat = "ddd"
                  }

                  const formattedTimeRange = `${startDate.format(
                    dateFormat
                  )} ${startDate.format(timeFormat)} to ${endDate.format(
                    timeFormat
                  )}`

                  this.tooltipContent = formattedTimeRange
                }
              }
            }
          },
          mouseleave: () => {
            this.tooltipContent = ""
          },
        }
      }
      return {}
    },
    resetCurTimeslot() {
      // Only reset cur timeslot if it isn't being persisted
      if (this.timeslotSelected) return

      this.curTimeslotAvailability = {}
      for (const respondent of this.respondents) {
        this.curTimeslotAvailability[respondent._id] = true
      }
      this.curTimeslot = { row: -1, col: -1 }

      // End drag if mouse left time grid
      this.endDrag()
    },
    /** Returns all valid displayed time ranges using existing logic (for validation for set slots)
     * Returns an object that maps time slot Date objects to their row/col coordinates:
     * - Map keys: time slot startTime Date objects (using getTime() as the key)
     * - Map values: { row, col, startTime, endTime } objects
     */
    getAllValidTimeRanges() {
      const timeSlotToRowCol = new Map()

      // Skip if event is daysOnly (no time slots)
      if (this.event.daysOnly) {
        return timeSlotToRowCol
      }

      // Iterate through all displayed days (columns)
      for (let col = 0; col < this.days.length; col++) {
        // Iterate through all displayed times (rows)
        for (let row = 0; row < this.times.length; row++) {
          // Use existing getDateFromRowCol method - returns UTC Date representing the local time
          // For example, if event is 9 AM PST, this returns 2026-12-21T17:00:00.000Z (9 AM PST = 17:00 UTC)
          const date = this.getDateFromRowCol(row, col)
          if (!date) continue

          // getDateFromRowCol already returns the correct UTC Date representing the local time
          // No need to adjust - use it directly and add timeslot duration
          const startDate = dayjs(date).utc()
          const endDate = dayjs(date)
            .utc()
            .add(this.timeslotDuration, "minutes")

          // Convert dayjs UTC objects to Date objects using UTC milliseconds directly
          const startTime = new Date(startDate.valueOf())
          const endTime = new Date(endDate.valueOf())

          // Map the startTime (using getTime() as key) to its row/col coordinates
          timeSlotToRowCol.set(startTime.getTime(), {
            row,
            col,
            startTime, // Date object matching exactly what hover tooltip displays
            endTime, // Date object (startTime + timeslotDuration)
          })
        }
      }

      return timeSlotToRowCol
    },
  },
}
