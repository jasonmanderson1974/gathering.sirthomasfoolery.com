import { describe, it, expect } from "vitest"
import { getResponseCountColorClass } from "./responseCounts"

describe("getResponseCountColorClass", () => {
  it("returns empty string when there is nothing to show", () => {
    expect(getResponseCountColorClass(0, 5)).toBe("")
    expect(getResponseCountColorClass(3, 0)).toBe("")
    expect(getResponseCountColorClass(0, 0)).toBe("")
  })

  it("returns green when at least two thirds are free", () => {
    expect(getResponseCountColorClass(2, 3)).toBe("tw-text-[#16A34A]")
    expect(getResponseCountColorClass(3, 3)).toBe("tw-text-[#16A34A]")
    expect(getResponseCountColorClass(4, 5)).toBe("tw-text-[#16A34A]")
  })

  it("returns yellow between one third and two thirds", () => {
    expect(getResponseCountColorClass(1, 3)).toBe("tw-text-[#CA8A04]")
    expect(getResponseCountColorClass(2, 5)).toBe("tw-text-[#CA8A04]")
    expect(getResponseCountColorClass(3, 6)).toBe("tw-text-[#CA8A04]")
  })

  it("returns red below one third", () => {
    expect(getResponseCountColorClass(1, 4)).toBe("tw-text-[#DC2626]")
    expect(getResponseCountColorClass(1, 10)).toBe("tw-text-[#DC2626]")
  })

  it("colors a lone respondent green", () => {
    expect(getResponseCountColorClass(1, 1)).toBe("tw-text-[#16A34A]")
  })
})
