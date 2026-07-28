import { describe, it, expect } from "vitest"
import { nextScheduleLocation } from "./scheduleLocation"

describe("nextScheduleLocation", () => {
  it("follows a venue set inline while the menu field was untouched", () => {
    // The bug this exists to prevent: field seeded "", venue set inline,
    // confirming a time would otherwise send "" and wipe it.
    expect(nextScheduleLocation("The Fox & Hound", "", "")).toBe(
      "The Fox & Hound"
    )
  })

  it("follows a venue that was changed inline", () => {
    expect(
      nextScheduleLocation("The Fox & Hound", "Greg's garden", "Greg's garden")
    ).toBe("The Fox & Hound")
  })

  it("follows a venue that was cleared inline", () => {
    expect(nextScheduleLocation("", "Greg's garden", "Greg's garden")).toBe("")
  })

  it("keeps an unsaved venue typed into the menu", () => {
    // The user's in-progress edit is newer than the event's value
    expect(nextScheduleLocation("The Fox & Hound", "", "Greg's garden")).toBe(
      "Greg's garden"
    )
  })

  it("keeps the field when the event's location did not change", () => {
    expect(
      nextScheduleLocation("Greg's garden", "Greg's garden", "The Fox & Hound")
    ).toBe("The Fox & Hound")
  })

  it("treats null and undefined as an empty venue", () => {
    expect(nextScheduleLocation(undefined, undefined, undefined)).toBe("")
    expect(nextScheduleLocation(null, "", "")).toBe("")
    expect(nextScheduleLocation("The Fox & Hound", null, null)).toBe(
      "The Fox & Hound"
    )
  })

  it("is a no-op on the first, immediate call for an event with a venue", () => {
    // watcher runs with oldEvent undefined; the field was already seeded
    expect(
      nextScheduleLocation("Greg's garden", undefined, "Greg's garden")
    ).toBe("Greg's garden")
  })
})
