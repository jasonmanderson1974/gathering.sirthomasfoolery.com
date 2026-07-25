/**
 * Traffic-light coloring for the per-cell response counts shown on the
 * availability grid. The color reflects what share of respondents are leaning
 * available (available or if-needed) at a given timeslot.
 */

/**
 * Returns the Tailwind pill classes (background + contrasting text) for the
 * given count out of total, or "" when there is nothing to show (count of 0 or
 * no respondents). The solid pill background keeps the digit legible over any
 * heatmap cell color.
 *
 * >= 2/3 free -> green, >= 1/3 -> yellow/amber, > 0 -> red. White text on
 * green/red; dark text on amber (white on amber fails contrast).
 */
export function getResponseCountColorClass(count, total) {
  if (!count || !total) return ""
  const frac = count / total
  if (frac >= 2 / 3) return "tw-bg-[#16A34A] tw-text-white" // green
  if (frac >= 1 / 3) return "tw-bg-[#CA8A04] tw-text-black" // yellow/amber
  return "tw-bg-[#DC2626] tw-text-white" // red
}
