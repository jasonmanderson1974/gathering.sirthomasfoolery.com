import { dateToDowDate, getSpecificTimesDayStarts } from "@/utils"

/**
 * The day axis of the ScheduleOverlap grid: which days the gathering covers
 * (allDays), which of them the current page shows (days / monthDays), the
 * month-grid variant used by daysOnly gatherings, and the pagination and
 * column-offset arithmetic that follows from both.
 *
 * Extracted verbatim from ScheduleOverlap.vue's computed block as a Vue 2 mixin
 * (TODO G2). Mixin computeds are evaluated against the same component instance,
 * so every `this.*` reference (props, data, other computeds) resolves exactly as
 * before — a behavior-preserving move, not a rewrite. The component keeps the
 * state these read: `page`, `mobileNumDays`, `months`, `startCalendarOnMonday`.
 */
export default {
  computed: {
    /** Returns the days of the week in the correct order */
    daysOfWeek() {
      if (!this.event.daysOnly) {
        return ["sun", "mon", "tue", "wed", "thu", "fri", "sat"]
      }
      return !this.startCalendarOnMonday
        ? ["sun", "mon", "tue", "wed", "thu", "fri", "sat"]
        : ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
    },
    /** Returns the day offset caused by the timezone offset. If the timezone offset changes the date, dayOffset != 0 */
    dayOffset() {
      return Math.floor((this.event.startTime - this.timezoneOffset / 60) / 24)
    },
    /** Returns all the days that are encompassed by startDate and endDate */
    allDays() {
      const days = []
      const datesSoFar = new Set()

      const getDateString = (date) => {
        let dateString = ""
        let dayString = ""
        const offsetDate = new Date(date)
        if (this.isSpecificTimes) {
          offsetDate.setTime(
            offsetDate.getTime() - this.timezoneOffset * 60 * 1000
          )
        } else {
          offsetDate.setDate(offsetDate.getDate() + this.dayOffset)
        }
        if (this.isSpecificDates) {
          dateString = `${
            this.months[offsetDate.getUTCMonth()]
          } ${offsetDate.getUTCDate()}`
          dayString = this.daysOfWeek[offsetDate.getUTCDay()]
        } else if (this.isWeekly) {
          const tmpDate = dateToDowDate(
            this.event.dates,
            offsetDate,
            this.weekOffset,
            true
          )

          dateString = `${
            this.months[tmpDate.getUTCMonth()]
          } ${tmpDate.getUTCDate()}`
          dayString = this.daysOfWeek[tmpDate.getUTCDay()]
        }
        return { dateString, dayString }
      }

      if (
        this.isSpecificTimes &&
        (this.state === this.states.SET_SPECIFIC_TIMES ||
          this.event.times?.length === 0)
      ) {
        for (const day of getSpecificTimesDayStarts(
          this.event.dates,
          this.curTimezone
        )) {
          const { dayString, dateString } = getDateString(day.dateObject)
          days.push({
            dayText: dayString,
            dateString,
            dateObject: day.dateObject,
            isConsecutive: day.isConsecutive,
          })
        }
        return days
      }

      for (let i = 0; i < this.event.dates.length; ++i) {
        const date = new Date(this.event.dates[i])
        datesSoFar.add(date.getTime())

        const { dayString, dateString } = getDateString(date)
        days.push({
          dayText: dayString,
          dateString,
          dateObject: date,
        })
      }

      let dayIndex = 0
      for (let i = 0; i < this.event.dates.length; ++i) {
        const date = new Date(this.event.dates[i])
        // See if the date goes into the next day
        const localStart = new Date(
          date.getTime() - this.timezoneOffset * 60 * 1000
        )
        const localEnd = new Date(
          date.getTime() +
            this.event.duration * 60 * 60 * 1000 -
            this.timezoneOffset * 60 * 1000
        )
        const localEndIsMidnight =
          localEnd.getUTCHours() === 0 && localEnd.getUTCMinutes() === 0
        if (
          localStart.getUTCDate() !== localEnd.getUTCDate() &&
          !localEndIsMidnight
        ) {
          // The date goes into the next day. Split the date into two dates
          let nextDate = new Date(date)
          nextDate.setUTCDate(nextDate.getUTCDate() + 1)
          if (!datesSoFar.has(nextDate.getTime())) {
            datesSoFar.add(nextDate.getTime())

            const { dayString, dateString } = getDateString(nextDate)
            days.splice(dayIndex + 1, 0, {
              dayText: dayString,
              dateString,
              dateObject: nextDate,
              excludeTimes: true,
            })
            dayIndex++
          }
        }
        dayIndex++
      }

      let prevDate = null // Stores the prevDate to check if the current date is consecutive to the previous date
      for (let i = 0; i < days.length; ++i) {
        let isConsecutive = true
        if (prevDate) {
          isConsecutive =
            prevDate.getTime() ===
            days[i].dateObject.getTime() - 24 * 60 * 60 * 1000
        }

        days[i].isConsecutive = isConsecutive

        prevDate = new Date(days[i].dateObject)
      }

      return days
    },
    /** Returns a subset of all days based on the page number */
    days() {
      const slice = this.allDays.slice(
        this.page * this.maxDaysPerPage,
        (this.page + 1) * this.maxDaysPerPage
      )
      slice[0] = { ...slice[0], isConsecutive: true }
      return slice
    },
    /** Returns all the days of the month */
    monthDays() {
      const monthDays = []
      const allDaysSet = new Set(
        this.allDays.map((d) => d.dateObject.getTime())
      )

      // Calculate monthIndex and year from event start date and page num
      const date = new Date(this.event.dates[0])
      const monthIndex = date.getUTCMonth() + this.page
      const year = date.getUTCFullYear()

      const lastDayOfPrevMonth = new Date(Date.UTC(year, monthIndex, 0))
      const lastDayOfCurMonth = new Date(Date.UTC(year, monthIndex + 1, 0))

      // Calculate num days from prev month, cur month, and next month to show
      const curDate = new Date(lastDayOfPrevMonth)
      let numDaysFromPrevMonth = 0
      const numDaysInCurMonth = lastDayOfCurMonth.getUTCDate()
      const numDaysFromNextMonth = 6 - lastDayOfCurMonth.getUTCDay()
      const hasDaysFromPrevMonth = !this.startCalendarOnMonday
        ? lastDayOfPrevMonth.getUTCDay() < 6
        : lastDayOfPrevMonth.getUTCDay() != 0
      if (hasDaysFromPrevMonth) {
        curDate.setUTCDate(
          curDate.getUTCDate() -
            (lastDayOfPrevMonth.getUTCDay() -
              (this.startCalendarOnMonday ? 1 : 0))
        )
        numDaysFromPrevMonth = lastDayOfPrevMonth.getUTCDay() + 1
      } else {
        curDate.setUTCDate(curDate.getUTCDate() + 1)
      }
      curDate.setUTCHours(this.event.startTime)

      // Add all days from prev month, cur month, and next month
      const totalDays =
        numDaysFromPrevMonth + numDaysInCurMonth + numDaysFromNextMonth
      for (let i = 0; i < totalDays; ++i) {
        // Only include days from the current month
        if (curDate.getUTCMonth() === lastDayOfCurMonth.getUTCMonth()) {
          monthDays.push({
            date: curDate.getUTCDate(),
            time: curDate.getTime(),
            dateObject: new Date(curDate),
            included: allDaysSet.has(curDate.getTime()),
          })
        } else {
          monthDays.push({
            date: "",
            time: curDate.getTime(),
            dateObject: new Date(curDate),
            included: false,
          })
        }

        curDate.setUTCDate(curDate.getUTCDate() + 1)
      }

      return monthDays
    },
    /** Map from datetime to whether that month day is included  */
    monthDayIncluded() {
      const includedMap = new Map()
      for (const monthDay of this.monthDays) {
        includedMap.set(monthDay.dateObject.getTime(), monthDay.included)
      }
      return includedMap
    },
    /** Returns the text to show for the current month */
    curMonthText() {
      const date = new Date(this.event.dates[0])
      const monthIndex = date.getUTCMonth() + this.page
      const year = date.getUTCFullYear()
      const lastDayOfCurMonth = new Date(Date.UTC(year, monthIndex + 1, 0))

      const monthText = this.months[lastDayOfCurMonth.getUTCMonth()]
      const yearText = lastDayOfCurMonth.getUTCFullYear()
      return `${monthText} ${yearText}`
    },
    maxDaysPerPage() {
      return this.isPhone ? this.mobileNumDays : 7
    },
    hasNextPage() {
      if (this.event.daysOnly) {
        const lastDay = new Date(this.event.dates[this.event.dates.length - 1])
        const curDate = new Date(this.event.dates[0])
        const monthIndex = curDate.getUTCMonth() + this.page
        const year = curDate.getUTCFullYear()

        const lastDayOfCurMonth = new Date(Date.UTC(year, monthIndex + 1, 0))

        return lastDayOfCurMonth.getTime() < lastDay.getTime()
      }

      return this.allDays.length - (this.page + 1) * this.maxDaysPerPage > 0
    },
    hasPrevPage() {
      return this.page > 0
    },
    /** Returns whether the event has more than one page */
    hasPages() {
      return this.hasNextPage || this.hasPrevPage
    },
    /** Returns an array of the x-offsets of the columns, taking into account the split gaps from non-consecutive days */
    columnOffsets() {
      const offsets = []
      let accumulatedOffset = 0
      for (let i = 0; i < this.days.length; ++i) {
        offsets.push(accumulatedOffset)
        if (!this.days[i].isConsecutive) {
          accumulatedOffset += this.SPLIT_GAP_WIDTH
        }
        accumulatedOffset += this.timeslot.width
      }
      return offsets
    },
  },
}
