import { describe, expect, it } from "vitest"
import { gridTimeOffset, isWeirdTimezone } from "./gridTimeOffset"

// Offsets in minutes, as the component holds them.
const UTC = 0
const NEW_YORK = -300
const KOLKATA = 330 // +5:30
const KATHMANDU = 345 // +5:45
const NEWFOUNDLAND = -210 // -3:30

describe("isWeirdTimezone", () => {
  it("accepts whole-hour zones", () => {
    expect(isWeirdTimezone(UTC)).toBe(false)
    expect(isWeirdTimezone(NEW_YORK)).toBe(false)
  })

  it("flags the half- and quarter-hour zones", () => {
    expect(isWeirdTimezone(KOLKATA)).toBe(true)
    expect(isWeirdTimezone(KATHMANDU)).toBe(true)
    expect(isWeirdTimezone(NEWFOUNDLAND)).toBe(true)
  })
})

describe("gridTimeOffset", () => {
  it("leaves a whole-hour viewer on a whole-hour event unshifted", () => {
    expect(
      gridTimeOffset({ timezoneOffset: NEW_YORK, eventStartTime: 9 })
    ).toBe(0)
  })

  it("shifts a half-hour viewer looking at a whole-hour event", () => {
    // Otherwise every row would read :30 past.
    expect(gridTimeOffset({ timezoneOffset: KOLKATA, eventStartTime: 9 })).toBe(
      -0.5
    )
    expect(
      gridTimeOffset({ timezoneOffset: NEWFOUNDLAND, eventStartTime: 9 })
    ).toBe(-0.5)
  })

  it("shifts a whole-hour viewer looking at a half-hour event", () => {
    expect(
      gridTimeOffset({ timezoneOffset: NEW_YORK, eventStartTime: 9.5 })
    ).toBe(-0.5)
  })

  it("leaves a half-hour viewer on a half-hour event unshifted", () => {
    // The two half hours cancel: the rows already land on the local hour.
    expect(
      gridTimeOffset({ timezoneOffset: KOLKATA, eventStartTime: 9.5 })
    ).toBe(0)
  })

  // G1: the shift is a labelling convenience, and it is wrong wherever cells
  // are matched against stored instants — it moved every cell 30 minutes off
  // the times the event actually stores, so a half-hour viewer saw an empty
  // grid rather than a mislabelled one.
  describe("viewing an event with specific times", () => {
    it("never shifts, whatever the timezone", () => {
      for (const timezoneOffset of [
        UTC,
        NEW_YORK,
        KOLKATA,
        KATHMANDU,
        NEWFOUNDLAND,
      ]) {
        expect(
          gridTimeOffset({
            timezoneOffset,
            eventStartTime: 9,
            matchesStoredTimes: true,
          })
        ).toBe(0)
      }
    })

    it("never shifts for a half-hour event start either", () => {
      expect(
        gridTimeOffset({
          timezoneOffset: NEW_YORK,
          eventStartTime: 9.5,
          matchesStoredTimes: true,
        })
      ).toBe(0)
    })

    it("still shifts when the flag is off, so the ordinary grid is untouched", () => {
      // Guards the fix against being widened into the common path.
      expect(
        gridTimeOffset({
          timezoneOffset: KOLKATA,
          eventStartTime: 9,
          matchesStoredTimes: false,
        })
      ).toBe(-0.5)
    })
  })
})
