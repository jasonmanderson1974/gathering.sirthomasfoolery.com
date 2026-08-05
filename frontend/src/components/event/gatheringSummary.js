/**
 * The handful of strings that describe a *confirmed* gathering — when it
 * repeats, whether a reminder goes out, and where to download it as an .ics.
 *
 * These used to be duplicated as computeds in ToolRow.vue (the "Gathering set"
 * dropdown) and EventHeader.vue (the chips). GatheringSummary.vue is a third
 * consumer, so they live here instead of being copied a third time.
 *
 * Plain functions taking the event, not a mixin: each consumer wraps them in a
 * one-line computed, which keeps reactivity obvious at the call site.
 */

import { serverURL } from "@/constants"

/** Human label for a repeating gathering (C5); "" for a one-off. */
export function recurrenceLabel(event) {
  switch (event?.gatheringRecurrence?.frequency) {
    case "weekly":
      return "Repeats weekly"
    case "biweekly":
      return "Repeats every 2 weeks"
    case "monthly":
      return "Repeats monthly"
    default:
      return ""
  }
}

/**
 * One dim line under the confirmed time, e.g. "Repeats weekly · Reminder 24h
 * before". Always says something about the reminder, so its absence reads as a
 * deliberate setting rather than a missing feature.
 */
export function reminderSummaryText(event) {
  const parts = []

  const recurrence = recurrenceLabel(event)
  if (recurrence) parts.push(recurrence)

  const reminder = event?.gatheringReminder
  if (reminder && reminder.enabled) {
    parts.push(`Reminder ${reminder.leadTimeHours ?? 24}h before`)
  } else {
    parts.push("No reminder email")
  }

  return parts.join(" · ")
}

/**
 * Universal "add to calendar" (.ics) download for a confirmed gathering — works
 * without any calendar account, and carries the recurrence rule. Served by the
 * backend getEventIcs.
 */
export function icsUrl(event) {
  const id = event?.shortId ?? event?._id
  return `${serverURL}/events/${id}/ics`
}
