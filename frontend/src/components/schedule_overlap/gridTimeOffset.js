/**
 * The half-hour shift ScheduleOverlap's time grid applies to keep row labels on
 * clean local boundaries.
 *
 * Extracted from splitTimes as a pure function of its inputs, following the
 * gridGeometry/responseCounts pattern, so the half-hour-timezone cases can be
 * tested without the component (TODO2 G1).
 */

/**
 * A timezone whose offset is not a whole number of hours — India (+5:30),
 * Nepal (+5:45), Newfoundland (-3:30), and the other half- and quarter-hour
 * zones. A UTC-aligned grid row lands mid-hour for these viewers.
 *
 * @param {number} timezoneOffset - minutes
 */
export const isWeirdTimezone = (timezoneOffset) => timezoneOffset % 60 !== 0

/**
 * How far to shift every row's hoursOffset, in hours.
 *
 * When exactly one of the viewer's timezone and the event's start time sits on
 * a half hour, a UTC-aligned grid would label every row at :30 past. Shifting
 * the whole grid back half an hour puts the labels back on the hour.
 *
 * That shift is a DISPLAY choice, and it is wrong for an event with specific
 * times (G1). Those events store exact instants, and the grid matches each cell
 * against them by timestamp — so a shifted cell matches nothing, and in a
 * half-hour timezone every slot of a specific-times event came back null: not a
 * mislabelled grid, an empty one. The stored instants win, and a viewer in
 * Kolkata correctly sees the event's real half-hour local times.
 *
 * Setting specific times is exempt because that grid is built from a plain
 * 0..23 local hour list, with no shift to undo.
 *
 * @param {object} opts
 * @param {number} opts.timezoneOffset - viewer's offset in minutes
 * @param {number} opts.eventStartTime - event.startTime, in UTC hours
 * @param {boolean} opts.matchesStoredTimes - true when cells are matched
 *   against stored instants, i.e. viewing a specific-times event
 * @returns {number} hours to add to every row's offset — 0 or -0.5
 */
export const gridTimeOffset = ({
  timezoneOffset,
  eventStartTime,
  matchesStoredTimes = false,
}) => {
  if (matchesStoredTimes) return 0

  const startTimeIsWeird = eventStartTime % 1 !== 0
  return isWeirdTimezone(timezoneOffset) !== startTimeIsWeird ? -0.5 : 0
}
