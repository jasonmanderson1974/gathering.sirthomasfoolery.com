/**
 * The three formats a gathering can take, as presented by the primary toggle in
 * the "Call a Gathering" modal.
 *
 * The API instead takes three independent booleans — `daysOnly`,
 * `hasSpecificTimes` and `wholeBlockSelection` — so this module is the mapping
 * between the two representations. Going through a single mode also makes the
 * meaningless combination (`hasSpecificTimes: false` with
 * `wholeBlockSelection: true`) unrepresentable.
 */
export const availabilityModes = Object.freeze({
  // Recipients pick times of day; `customTimes` decides whether every day
  // shares one time range or each day gets its own
  DATES_AND_TIMES: "datesAndTimes",
  // Recipients pick whole blocks only, defined by the owner in the next step
  TIME_BLOCKS: "timeBlocks",
  // Recipients pick dates, no times at all
  DATES_ONLY: "datesOnly",
})

/**
 * Derives the fields the API takes from the selected mode. `customTimes` only
 * applies to DATES_AND_TIMES and is ignored in the other modes.
 */
export const getAvailabilityFields = (mode, customTimes) => ({
  daysOnly: mode === availabilityModes.DATES_ONLY,
  wholeBlockSelection: mode === availabilityModes.TIME_BLOCKS,
  hasSpecificTimes:
    mode === availabilityModes.TIME_BLOCKS ||
    (mode === availabilityModes.DATES_AND_TIMES && !!customTimes),
})

/** Derives the mode and sub-choice to preselect when editing an existing event */
export const getAvailabilityMode = (event) => {
  if (event.daysOnly) {
    return { mode: availabilityModes.DATES_ONLY, customTimes: false }
  }
  if (event.hasSpecificTimes && event.wholeBlockSelection) {
    return { mode: availabilityModes.TIME_BLOCKS, customTimes: false }
  }
  return {
    mode: availabilityModes.DATES_AND_TIMES,
    customTimes: !!event.hasSpecificTimes,
  }
}
