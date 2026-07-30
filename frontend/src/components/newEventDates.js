import dayjs from "dayjs"
import utcPlugin from "dayjs/plugin/utc"
import timezonePlugin from "dayjs/plugin/timezone"
import { eventTypes, dayIndexToDayString } from "@/constants"
import { timeNumToTimeString } from "@/utils"

dayjs.extend(utcPlugin)
dayjs.extend(timezonePlugin)

/**
 * The two ways the creation form lets you pick days. The values are the labels
 * shown in the `v-select`, so they double as the option list.
 */
export const dateOptions = Object.freeze({
  SPECIFIC: "Specific dates",
  DOW: "Days of the week",
})

/**
 * Turns the creation form's day/time selections into the three fields the API
 * takes — `dates`, `duration` and `type`.
 *
 * Extracted from the submit handlers of NewEvent and NewSignUp (G2), which held
 * near-identical copies of this. "Near" is why it is worth having in one place:
 * the sign-up copy had drifted in two ways, both fixed by folding them together
 * — see the notes on the days-only and DOW branches below.
 *
 * Pure, so the timezone arithmetic is testable without mounting a form.
 * `selectedDaysOfWeek` is returned rather than mutated in place; the caller
 * assigns it back, because the toggle group is bound to it.
 *
 * @param {boolean} daysOnly - dates carry no time of day
 * @param {string[]} selectedDays - ISO `YYYY-MM-DD` strings, pre-sorted
 * @param {number[]} selectedDaysOfWeek - day indices, 0 = Sunday .. 7 = Sunday
 * @param {string} selectedDateOption - one of `dateOptions`
 * @param {number} startTime - hour of day, may be fractional
 * @param {number} endTime - hour of day; wraps past midnight when <= startTime
 * @param {boolean} startOnMonday - which Sunday index the week's toggle uses
 * @param {{ value: string }} timezone - IANA name the times are entered in
 * @returns {{ dates: Date[], duration: number, type: string,
 *             selectedDaysOfWeek: number[] }}
 */
export function buildEventDates({
  daysOnly,
  selectedDays,
  selectedDaysOfWeek,
  selectedDateOption,
  startTime,
  endTime,
  startOnMonday,
  timezone,
}) {
  let duration = endTime - startTime
  if (duration <= 0) duration += 24

  const dates = []

  if (daysOnly) {
    // A days-only event still stores specific dates — it just stores them at
    // midnight UTC with no duration. The sign-up copy set the type from
    // `eventTypes.SIGNUP`, which does not exist, so the field went out as
    // `undefined` and the API (where `type` is `binding:"required"`) rejected
    // every dates-only sign-up sheet.
    for (const day of selectedDays) {
      dates.push(new Date(`${day} 00:00:00Z`))
    }
    return {
      dates,
      duration: 0,
      type: eventTypes.SPECIFIC_DATES,
      selectedDaysOfWeek,
    }
  }

  const startTimeString = timeNumToTimeString(startTime)

  if (selectedDateOption === dateOptions.SPECIFIC) {
    for (const day of selectedDays) {
      dates.push(dayjs.tz(`${day} ${startTimeString}`, timezone.value).toDate())
    }
    return {
      dates,
      duration,
      type: eventTypes.SPECIFIC_DATES,
      selectedDaysOfWeek,
    }
  }

  if (selectedDateOption === dateOptions.DOW) {
    // Drop whichever wrap-around Sunday the toggle group isn't showing
    const daysOfWeek = [...selectedDaysOfWeek]
      .sort((a, b) => a - b)
      .filter((dayIndex) => (startOnMonday ? dayIndex !== 0 : dayIndex !== 7))

    for (const dayIndex of daysOfWeek) {
      const date = dayjs.tz(
        `${dayIndexToDayString[dayIndex]} ${startTimeString}`,
        timezone.value
      )

      // The reference dates (dayIndexToDayString) are from June 2018, which may
      // have a different DST offset than the current date. Adjust so the stored
      // UTC time corresponds to the user's current timezone offset. The sign-up
      // copy of this loop omitted the adjustment, so a recurring sign-up sheet
      // made in the opposite DST half of the year was stored an hour out.
      const refOffset = date.utcOffset()
      const currentOffset = dayjs().tz(timezone.value).utcOffset()
      dates.push(date.subtract(currentOffset - refOffset, "minutes").toDate())
    }

    return {
      dates,
      duration,
      type: eventTypes.DOW,
      selectedDaysOfWeek: daysOfWeek,
    }
  }

  // No recognised date option — the form's validation rules make this
  // unreachable, but returning an empty type keeps the shape consistent
  return { dates, duration, type: "", selectedDaysOfWeek }
}
