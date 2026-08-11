import { afterEach, describe, expect, it, vi } from "vitest"
import { buildEventDates, dateOptions } from "./newEventDates"
import { eventTypes } from "@/constants"

/** The fields every call needs, so each test only states what it varies */
const base = {
  daysOnly: false,
  selectedDays: [],
  selectedDaysOfWeek: [],
  selectedDateOption: dateOptions.SPECIFIC,
  startTime: 9,
  endTime: 17,
  startOnMonday: false,
  timezone: { value: "America/New_York" },
}

const build = (overrides) => buildEventDates({ ...base, ...overrides })

describe("buildEventDates", () => {
  describe("duration", () => {
    it("is the span between start and end", () => {
      expect(build({ startTime: 9, endTime: 17 }).duration).toBe(8)
    })

    it("wraps past midnight rather than going negative", () => {
      expect(build({ startTime: 22, endTime: 2 }).duration).toBe(4)
    })

    it("wraps a full day when start and end are equal", () => {
      expect(build({ startTime: 9, endTime: 9 }).duration).toBe(24)
    })

    it("is zero for a days-only event", () => {
      expect(
        build({ daysOnly: true, selectedDays: ["2026-08-01"] }).duration
      ).toBe(0)
    })
  })

  describe("days-only events", () => {
    it("stores each date at midnight UTC", () => {
      const { dates } = build({
        daysOnly: true,
        selectedDays: ["2026-08-01", "2026-08-02"],
      })

      expect(dates.map((d) => d.toISOString())).toEqual([
        "2026-08-01T00:00:00.000Z",
        "2026-08-02T00:00:00.000Z",
      ])
    })

    it("uses a known event type — a sign-up sheet used to send undefined here", () => {
      const { type } = build({ daysOnly: true, selectedDays: ["2026-08-01"] })

      expect(type).toBe(eventTypes.SPECIFIC_DATES)
      expect(eventTypes.SPECIFIC_DATES).toBeDefined()
    })

    it("ignores the date option — days-only always means specific dates", () => {
      const { type, dates } = build({
        daysOnly: true,
        selectedDateOption: dateOptions.DOW,
        selectedDays: ["2026-08-01"],
        selectedDaysOfWeek: [1, 2],
      })

      expect(type).toBe(eventTypes.SPECIFIC_DATES)
      expect(dates).toHaveLength(1)
    })
  })

  describe("specific dates", () => {
    it("places each day at the start time in the given timezone", () => {
      const { dates, type } = build({
        selectedDays: ["2026-08-01"],
        startTime: 9,
      })

      // 09:00 in New York in August (EDT, UTC-4) is 13:00 UTC
      expect(type).toBe(eventTypes.SPECIFIC_DATES)
      expect(dates[0].toISOString()).toBe("2026-08-01T13:00:00.000Z")
    })

    it("honours the timezone it is given, not the host's", () => {
      const { dates } = build({
        selectedDays: ["2026-08-01"],
        startTime: 9,
        timezone: { value: "Europe/London" },
      })

      // 09:00 in London in August (BST, UTC+1) is 08:00 UTC
      expect(dates[0].toISOString()).toBe("2026-08-01T08:00:00.000Z")
    })

    it("handles a half-hour offset timezone", () => {
      const { dates } = build({
        selectedDays: ["2026-08-01"],
        startTime: 9,
        timezone: { value: "Asia/Kolkata" },
      })

      // 09:00 IST (UTC+5:30) is 03:30 UTC
      expect(dates[0].toISOString()).toBe("2026-08-01T03:30:00.000Z")
    })
  })

  describe("days of the week", () => {
    const dow = { selectedDateOption: dateOptions.DOW }

    it("sorts the selected indices", () => {
      const { selectedDaysOfWeek, type } = build({
        ...dow,
        selectedDaysOfWeek: [5, 1, 3],
      })

      expect(type).toBe(eventTypes.DOW)
      expect(selectedDaysOfWeek).toEqual([1, 3, 5])
    })

    it("drops the trailing Sunday when the week starts on Sunday", () => {
      const { selectedDaysOfWeek, dates } = build({
        ...dow,
        selectedDaysOfWeek: [0, 3, 7],
        startOnMonday: false,
      })

      expect(selectedDaysOfWeek).toEqual([0, 3])
      expect(dates).toHaveLength(2)
    })

    it("drops the leading Sunday when the week starts on Monday", () => {
      const { selectedDaysOfWeek } = build({
        ...dow,
        selectedDaysOfWeek: [0, 3, 7],
        startOnMonday: true,
      })

      expect(selectedDaysOfWeek).toEqual([3, 7])
    })

    it("does not mutate the array it is given", () => {
      const selectedDaysOfWeek = [5, 1, 7]
      build({ ...dow, selectedDaysOfWeek })

      expect(selectedDaysOfWeek).toEqual([5, 1, 7])
    })

    /**
     * The reference dates are in June 2018 — inside DST for the northern
     * hemisphere. Creating from the other half of the year means the reference
     * date and today disagree about the offset, and without the correction the
     * time is stored an hour out. That is exactly what the sign-up copy of this
     * loop did, so both halves of the year are pinned here.
     */
    describe("the June 2018 reference dates vs. today's DST offset", () => {
      const mondayAt9 = () =>
        build({
          ...dow,
          selectedDaysOfWeek: [1], // Monday
          startTime: 9,
          timezone: { value: "America/New_York" },
        }).dates[0]

      afterEach(() => {
        vi.useRealTimers()
      })

      it("stores 09:00 EST when created in winter", () => {
        vi.useFakeTimers()
        vi.setSystemTime(new Date("2026-01-15T12:00:00Z"))

        // EST is UTC-5, so 09:00 local is 14:00 UTC. Uncorrected, the June
        // reference date's EDT (UTC-4) would store 13:00 — an hour early.
        expect(mondayAt9().getUTCHours()).toBe(14)
      })

      it("stores 09:00 EDT when created in summer", () => {
        vi.useFakeTimers()
        vi.setSystemTime(new Date("2026-07-15T12:00:00Z"))

        // EDT is UTC-4, matching the reference date, so nothing shifts
        expect(mondayAt9().getUTCHours()).toBe(13)
      })
    })

    it("leaves a timezone without DST untouched by the correction", () => {
      const { dates } = build({
        ...dow,
        selectedDaysOfWeek: [1], // Monday, 2018-06-18
        startTime: 9,
        timezone: { value: "America/Phoenix" }, // UTC-7 year round
      })

      expect(dates[0].toISOString()).toBe("2018-06-18T16:00:00.000Z")
    })
  })

  it("returns an empty type when no date option matches", () => {
    const { dates, type } = build({ selectedDateOption: "nonsense" })

    expect(type).toBe("")
    expect(dates).toEqual([])
  })
})
