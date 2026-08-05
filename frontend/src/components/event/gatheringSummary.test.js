import { describe, it, expect } from "vitest"
import {
  recurrenceLabel,
  reminderSummaryText,
  icsUrl,
} from "./gatheringSummary"
import { serverURL } from "@/constants"

describe("recurrenceLabel", () => {
  it("names each supported frequency", () => {
    expect(recurrenceLabel({ gatheringRecurrence: { frequency: "weekly" } })).toBe(
      "Repeats weekly"
    )
    expect(
      recurrenceLabel({ gatheringRecurrence: { frequency: "biweekly" } })
    ).toBe("Repeats every 2 weeks")
    expect(
      recurrenceLabel({ gatheringRecurrence: { frequency: "monthly" } })
    ).toBe("Repeats monthly")
  })

  it("is empty for a one-off, an unknown frequency, or no event", () => {
    expect(recurrenceLabel({})).toBe("")
    expect(recurrenceLabel({ gatheringRecurrence: { frequency: "" } })).toBe("")
    expect(
      recurrenceLabel({ gatheringRecurrence: { frequency: "yearly" } })
    ).toBe("")
    expect(recurrenceLabel(undefined)).toBe("")
  })
})

describe("reminderSummaryText", () => {
  it("says so explicitly when no reminder is set", () => {
    expect(reminderSummaryText({})).toBe("No reminder email")
    expect(reminderSummaryText({ gatheringReminder: { enabled: false } })).toBe(
      "No reminder email"
    )
  })

  it("reports the lead time, defaulting to 24h", () => {
    expect(
      reminderSummaryText({
        gatheringReminder: { enabled: true, leadTimeHours: 48 },
      })
    ).toBe("Reminder 48h before")
    expect(reminderSummaryText({ gatheringReminder: { enabled: true } })).toBe(
      "Reminder 24h before"
    )
  })

  it("puts the recurrence first when the gathering repeats", () => {
    expect(
      reminderSummaryText({
        gatheringRecurrence: { frequency: "weekly" },
        gatheringReminder: { enabled: true, leadTimeHours: 12 },
      })
    ).toBe("Repeats weekly · Reminder 12h before")
  })
})

describe("icsUrl", () => {
  it("prefers the short id, since that is what members see", () => {
    expect(icsUrl({ shortId: "abc123", _id: "65f0000000000000000000aa" })).toBe(
      `${serverURL}/events/abc123/ics`
    )
  })

  it("falls back to the mongo id", () => {
    expect(icsUrl({ _id: "65f0000000000000000000aa" })).toBe(
      `${serverURL}/events/65f0000000000000000000aa/ics`
    )
  })
})
