import {
  timeNumToTimeText,
  utcTimeToLocalTime,
  getScheduleTimezoneOffset,
  getTimezoneReferenceDateForEvent,
} from "@/utils"
import { timeTypes, timeslotDurations } from "@/constants"
import { gridTimeOffset } from "./gridTimeOffset"

/**
 * The time axis of the ScheduleOverlap grid: the rows themselves (splitTimes /
 * times, including the overnight split into two columns of a single day), the
 * per-row geometry that follows from the gathering's time increment, and the
 * timezone offset every date calculation in the component is measured against.
 *
 * Extracted verbatim from ScheduleOverlap.vue's computed block as a Vue 2 mixin
 * (TODO G2). Mixin computeds are evaluated against the same component instance,
 * so every `this.*` reference (props, data, other computeds) resolves exactly as
 * before — a behavior-preserving move, not a rewrite. The component keeps the
 * state these read: `states`/`state`, `timeType`, `curTimezone`, `HOUR_HEIGHT`.
 */
export default {
  computed: {
    /**
     * Returns a two dimensional array of times
     * IF endTime < startTime:
     * the first element is an array of times between 12am and end time and the second element is an array of times between start time and 12am
     * ELSE:
     * the first element is an array of times between start time and end time. the second element is an empty array
     * */
    splitTimes() {
      const splitTimes = [[], []]

      const utcStartTime = this.event.startTime
      const utcEndTime = this.event.startTime + this.event.duration
      const localStartTime = utcTimeToLocalTime(
        utcStartTime,
        this.timezoneOffset
      )
      const localEndTime = utcTimeToLocalTime(utcEndTime, this.timezoneOffset)

      // Weird timezones are timezones that are not a multiple of 60 minutes
      // (e.g. GMT-2:30). See gridTimeOffset for why a specific-times event
      // takes no shift.
      const timeOffset = gridTimeOffset({
        timezoneOffset: this.timezoneOffset,
        eventStartTime: utcStartTime,
        matchesStoredTimes:
          this.isSpecificTimes && this.state !== this.states.SET_SPECIFIC_TIMES,
      })

      const getExtraTimes = (hoursOffset) => {
        if (this.timeslotDuration === timeslotDurations.FIFTEEN_MINUTES) {
          return [
            {
              hoursOffset: hoursOffset + 0.25,
            },
            {
              hoursOffset: hoursOffset + 0.5,
            },
            {
              hoursOffset: hoursOffset + 0.75,
            },
          ]
        } else if (this.timeslotDuration === timeslotDurations.THIRTY_MINUTES) {
          return [
            {
              hoursOffset: hoursOffset + 0.5,
            },
          ]
        }
        return []
      }

      if (this.state === this.states.SET_SPECIFIC_TIMES) {
        // Hours offset for specific times starts from minHours
        for (let i = 0; i <= 23; ++i) {
          const hoursOffset = i
          if (i === 9) {
            // add an id so we can scroll to it
            splitTimes[0].push({
              id: "time-9",
              hoursOffset,
              text: timeNumToTimeText(i, this.timeType === timeTypes.HOUR12),
            })
          } else {
            splitTimes[0].push({
              hoursOffset,
              text: timeNumToTimeText(i, this.timeType === timeTypes.HOUR12),
            })
          }
          splitTimes[0].push(...getExtraTimes(hoursOffset))
        }
        return splitTimes
      }

      if (localEndTime <= localStartTime && localEndTime !== 0) {
        for (let i = 0; i < localEndTime; ++i) {
          splitTimes[0].push({
            hoursOffset: this.event.duration - (localEndTime - i),
            text: timeNumToTimeText(i, this.timeType === timeTypes.HOUR12),
          })
          splitTimes[0].push(
            ...getExtraTimes(this.event.duration - (localEndTime - i))
          )
        }
        for (let i = 0; i < 24 - localStartTime; ++i) {
          const adjustedI = i + timeOffset
          splitTimes[1].push({
            hoursOffset: adjustedI,
            text: timeNumToTimeText(
              localStartTime + adjustedI,
              this.timeType === timeTypes.HOUR12
            ),
          })
          splitTimes[1].push(...getExtraTimes(adjustedI))
        }
      } else {
        for (let i = 0; i < this.event.duration; ++i) {
          const adjustedI = i + timeOffset
          const utcTimeNum = this.event.startTime + adjustedI
          const localTimeNum = utcTimeToLocalTime(
            utcTimeNum,
            this.timezoneOffset
          )

          splitTimes[0].push({
            hoursOffset: adjustedI,
            text: timeNumToTimeText(
              localTimeNum,
              this.timeType === timeTypes.HOUR12
            ),
          })
          splitTimes[0].push(...getExtraTimes(adjustedI))
        }
        if (timeOffset !== 0) {
          const localTimeNum = utcTimeToLocalTime(
            this.event.startTime + this.event.duration - 0.5,
            this.timezoneOffset
          )
          splitTimes[0].push({
            hoursOffset: this.event.duration - 0.5,
            text: timeNumToTimeText(
              localTimeNum,
              this.timeType === timeTypes.HOUR12
            ),
          })
          splitTimes[0].push(...getExtraTimes(this.event.duration - 0.5))
        }
        splitTimes[1] = []
      }

      return splitTimes
    },
    /** Returns the times that are encompassed by startTime and endTime */
    times() {
      return [...this.splitTimes[1], ...this.splitTimes[0]]
    },
    timeslotDuration() {
      return this.event.timeIncrement ?? timeslotDurations.FIFTEEN_MINUTES
    },
    timeslotHeight() {
      if (this.timeslotDuration === timeslotDurations.FIFTEEN_MINUTES) {
        return Math.floor(this.HOUR_HEIGHT / 4)
      } else if (this.timeslotDuration === timeslotDurations.THIRTY_MINUTES) {
        return Math.floor(this.HOUR_HEIGHT / 2)
      } else if (this.timeslotDuration === timeslotDurations.ONE_HOUR) {
        return this.HOUR_HEIGHT
      }
      return Math.floor(this.HOUR_HEIGHT / 4)
    },
    timezoneOffset() {
      return getScheduleTimezoneOffset(
        this.event,
        this.curTimezone,
        this.weekOffset
      )
    },
    timezoneReferenceDate() {
      return getTimezoneReferenceDateForEvent(this.event, this.weekOffset)
    },
  },
}
