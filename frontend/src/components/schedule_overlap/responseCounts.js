/**
 * Traffic-light coloring for the per-cell response counts shown on the
 * availability grid. The color reflects what share of respondents are leaning
 * available (available or if-needed) at a given timeslot.
 */

/**
 * Returns a Tailwind text-color class for the given count out of total, or ""
 * when there is nothing to show (count of 0 or no respondents).
 *
 * >= 2/3 free -> green, >= 1/3 -> yellow/amber, > 0 -> red.
 */
export function getResponseCountColorClass(count, total) {
  if (!count || !total) return ""
  const frac = count / total
  if (frac >= 2 / 3) return "tw-text-[#16A34A]" // green
  if (frac >= 1 / 3) return "tw-text-[#CA8A04]" // yellow/amber
  return "tw-text-[#DC2626]" // red
}
