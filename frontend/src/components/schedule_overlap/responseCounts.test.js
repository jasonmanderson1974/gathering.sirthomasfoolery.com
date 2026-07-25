import { describe, it, expect } from "vitest"
import { getResponseCountColorClass } from "./responseCounts"

describe("getResponseCountColorClass", () => {
  it("returns empty string when there is nothing to show", () => {
    expect(getResponseCountColorClass(0, 5)).toBe("")
    expect(getResponseCountColorClass(3, 0)).toBe("")
    expect(getResponseCountColorClass(0, 0)).toBe("")
  })

  it("returns a green pill when at least two thirds are free", () => {
    expect(getResponseCountColorClass(2, 3)).toBe("tw-bg-[#16A34A] tw-text-white")
    expect(getResponseCountColorClass(3, 3)).toBe("tw-bg-[#16A34A] tw-text-white")
    expect(getResponseCountColorClass(4, 5)).toBe("tw-bg-[#16A34A] tw-text-white")
  })

  it("returns a yellow pill between one third and two thirds", () => {
    expect(getResponseCountColorClass(1, 3)).toBe("tw-bg-[#CA8A04] tw-text-black")
    expect(getResponseCountColorClass(2, 5)).toBe("tw-bg-[#CA8A04] tw-text-black")
    expect(getResponseCountColorClass(3, 6)).toBe("tw-bg-[#CA8A04] tw-text-black")
  })

  it("returns a red pill below one third", () => {
    expect(getResponseCountColorClass(1, 4)).toBe("tw-bg-[#DC2626] tw-text-white")
    expect(getResponseCountColorClass(1, 10)).toBe("tw-bg-[#DC2626] tw-text-white")
  })

  it("colors a lone respondent green", () => {
    expect(getResponseCountColorClass(1, 1)).toBe("tw-bg-[#16A34A] tw-text-white")
  })
})
