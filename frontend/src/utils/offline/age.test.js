import { describe, expect, it } from "vitest"
import { formatAge } from "./age"

const NOW = 1_700_000_000_000
const MINUTE = 60 * 1000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

const ago = (ms) => formatAge(NOW - ms, NOW)

describe("formatAge", () => {
  it("says nothing when there is nothing to say", () => {
    expect(formatAge(null, NOW)).toBeNull()
    expect(formatAge(undefined, NOW)).toBeNull()
    expect(formatAge(NaN, NOW)).toBeNull()
    expect(formatAge("earlier", NOW)).toBeNull()
  })

  it("reads as just now inside the first minute", () => {
    expect(ago(0)).toBe("just now")
    expect(ago(59 * 1000)).toBe("just now")
  })

  it("counts minutes, then hours, then days", () => {
    expect(ago(MINUTE)).toBe("1 minute ago")
    expect(ago(5 * MINUTE)).toBe("5 minutes ago")
    expect(ago(59 * MINUTE)).toBe("59 minutes ago")
    expect(ago(HOUR)).toBe("1 hour ago")
    expect(ago(23 * HOUR)).toBe("23 hours ago")
    expect(ago(DAY)).toBe("1 day ago")
    expect(ago(9 * DAY)).toBe("9 days ago")
  })

  // A stamp from the future means the clock moved, not that the data is
  // negatively old. "just now" is the honest answer; "-3 minutes ago" is not.
  it("does not render a negative duration", () => {
    expect(formatAge(NOW + HOUR, NOW)).toBe("just now")
  })
})
