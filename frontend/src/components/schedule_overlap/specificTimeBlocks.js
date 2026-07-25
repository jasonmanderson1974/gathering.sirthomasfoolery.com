/**
 * Groups an event's "specific times" (individual slot start times) into
 * contiguous blocks, for the whole-block availability mode where recipients
 * must select an entire block at once.
 *
 * A block is a maximal run of slots spaced exactly `durationMinutes` apart, so
 * gaps between separate ranges (and between days) naturally split into blocks.
 */

/**
 * @param {number[]} timesMs - slot start times as unix ms timestamps
 * @param {number} durationMinutes - slot granularity in minutes (e.g. 15)
 * @returns {{ start: number, end: number, slots: number[] }[]} sorted blocks;
 *   `start` is the first slot, `end` is the last slot + one slot duration,
 *   `slots` lists every slot ms in the block.
 */
export function getSpecificTimeBlocks(timesMs, durationMinutes) {
  const slotMs = durationMinutes * 60 * 1000
  const sorted = [...new Set(timesMs)].sort((a, b) => a - b)

  const blocks = []
  let cur = null
  for (const t of sorted) {
    if (cur && t === cur.slots[cur.slots.length - 1] + slotMs) {
      cur.slots.push(t)
      cur.end = t + slotMs
    } else {
      cur = { start: t, end: t + slotMs, slots: [t] }
      blocks.push(cur)
    }
  }
  return blocks
}

/**
 * Builds a Map from each slot's ms timestamp to the block that contains it, for
 * O(1) lookup of the block a clicked cell belongs to.
 *
 * @param {{ slots: number[] }[]} blocks - output of getSpecificTimeBlocks
 * @returns {Map<number, object>}
 */
export function buildSlotToBlockMap(blocks) {
  const map = new Map()
  for (const block of blocks) {
    for (const slot of block.slots) {
      map.set(slot, block)
    }
  }
  return map
}
