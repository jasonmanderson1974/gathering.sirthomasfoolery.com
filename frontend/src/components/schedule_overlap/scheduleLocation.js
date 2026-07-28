/**
 * Keeps the Schedule menu's venue field in step with the event.
 *
 * A gathering's location can be set from three places, two of which are on the
 * event page at the same time: the inline editor (which patches the event in
 * place) and the Schedule menu (which sends the venue along with the confirmed
 * time). The menu's field is seeded from the event when ScheduleOverlap is
 * created and the component is never re-created, so without this it would keep
 * sending its original value and overwrite anything set inline afterwards.
 */

/**
 * Returns what the Schedule menu's venue field should hold after the event
 * changed from `previous` to `incoming`, given the field currently holds
 * `current`.
 *
 * Follows the event when the field still matches what the event used to say;
 * keeps `current` otherwise, because a venue typed here and not yet saved is
 * the newer edit and shouldn't be thrown away.
 */
export function nextScheduleLocation(incoming, previous, current) {
  const to = incoming ?? ""
  const from = previous ?? ""
  const held = current ?? ""

  if (to === from) return held
  return held === from ? to : held
}
