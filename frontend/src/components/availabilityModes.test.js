import { describe, it, expect } from "vitest"
import {
  availabilityModes,
  getAvailabilityFields,
  getAvailabilityMode,
} from "./availabilityModes"

describe("getAvailabilityFields", () => {
  it("maps 'Same Times Every Day' to a plain timed event", () => {
    expect(
      getAvailabilityFields(availabilityModes.DATES_AND_TIMES, false)
    ).toEqual({
      daysOnly: false,
      hasSpecificTimes: false,
      wholeBlockSelection: false,
    })
  })

  it("maps 'Custom Times Every Day' to specific times without whole blocks", () => {
    expect(
      getAvailabilityFields(availabilityModes.DATES_AND_TIMES, true)
    ).toEqual({
      daysOnly: false,
      hasSpecificTimes: true,
      wholeBlockSelection: false,
    })
  })

  it("maps time blocks to specific times with whole block selection", () => {
    expect(getAvailabilityFields(availabilityModes.TIME_BLOCKS, false)).toEqual(
      {
        daysOnly: false,
        hasSpecificTimes: true,
        wholeBlockSelection: true,
      }
    )
  })

  it("ignores customTimes outside of dates-and-times mode", () => {
    expect(getAvailabilityFields(availabilityModes.TIME_BLOCKS, true)).toEqual(
      getAvailabilityFields(availabilityModes.TIME_BLOCKS, false)
    )
    expect(getAvailabilityFields(availabilityModes.DATES_ONLY, true)).toEqual(
      getAvailabilityFields(availabilityModes.DATES_ONLY, false)
    )
  })

  it("maps dates only to no times at all", () => {
    expect(getAvailabilityFields(availabilityModes.DATES_ONLY, false)).toEqual({
      daysOnly: true,
      hasSpecificTimes: false,
      wholeBlockSelection: false,
    })
  })

  it("never reports whole block selection without specific times", () => {
    for (const mode of Object.values(availabilityModes)) {
      for (const customTimes of [false, true]) {
        const fields = getAvailabilityFields(mode, customTimes)
        if (fields.wholeBlockSelection) {
          expect(fields.hasSpecificTimes).toBe(true)
        }
      }
    }
  })
})

describe("getAvailabilityMode", () => {
  it("round trips every mode through the API fields", () => {
    const cases = [
      [availabilityModes.DATES_AND_TIMES, false],
      [availabilityModes.DATES_AND_TIMES, true],
      [availabilityModes.TIME_BLOCKS, false],
      [availabilityModes.DATES_ONLY, false],
    ]

    for (const [mode, customTimes] of cases) {
      const event = getAvailabilityFields(mode, customTimes)
      expect(getAvailabilityMode(event)).toEqual({ mode, customTimes })
    }
  })

  it("treats missing fields on older events as dates and times", () => {
    expect(getAvailabilityMode({})).toEqual({
      mode: availabilityModes.DATES_AND_TIMES,
      customTimes: false,
    })
  })

  it("prefers dates only even when time fields are set", () => {
    expect(
      getAvailabilityMode({
        daysOnly: true,
        hasSpecificTimes: true,
        wholeBlockSelection: true,
      })
    ).toEqual({ mode: availabilityModes.DATES_ONLY, customTimes: false })
  })

  it("ignores whole block selection when there are no specific times", () => {
    expect(
      getAvailabilityMode({
        hasSpecificTimes: false,
        wholeBlockSelection: true,
      })
    ).toEqual({ mode: availabilityModes.DATES_AND_TIMES, customTimes: false })
  })
})
