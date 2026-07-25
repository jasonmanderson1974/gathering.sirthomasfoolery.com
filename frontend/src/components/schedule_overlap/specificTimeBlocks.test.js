import { describe, it, expect } from "vitest"
import {
  getSpecificTimeBlocks,
  buildSlotToBlockMap,
} from "./specificTimeBlocks"

const MIN = 60 * 1000
// Base timestamp (arbitrary)
const t0 = new Date("2026-08-01T18:00:00.000Z").getTime()
const at = (offsetMinutes) => t0 + offsetMinutes * MIN

describe("getSpecificTimeBlocks", () => {
  it("returns no blocks for empty input", () => {
    expect(getSpecificTimeBlocks([], 15)).toEqual([])
  })

  it("groups consecutive 15-min slots into one block", () => {
    const times = [at(0), at(15), at(30)]
    const blocks = getSpecificTimeBlocks(times, 15)
    expect(blocks).toHaveLength(1)
    expect(blocks[0].start).toBe(at(0))
    expect(blocks[0].end).toBe(at(45)) // last slot + one duration
    expect(blocks[0].slots).toEqual([at(0), at(15), at(30)])
  })

  it("splits a gap into separate blocks", () => {
    // 11:00-11:30 (two 15-min slots) then a gap, then 12:00-12:15
    const times = [at(0), at(15), at(60), at(75)]
    const blocks = getSpecificTimeBlocks(times, 15)
    expect(blocks).toHaveLength(2)
    expect(blocks[0].slots).toEqual([at(0), at(15)])
    expect(blocks[1].slots).toEqual([at(60), at(75)])
  })

  it("sorts and de-dupes unordered input", () => {
    const times = [at(30), at(0), at(15), at(15)]
    const blocks = getSpecificTimeBlocks(times, 15)
    expect(blocks).toHaveLength(1)
    expect(blocks[0].slots).toEqual([at(0), at(15), at(30)])
  })

  it("respects a 30-minute granularity", () => {
    const times = [at(0), at(30), at(90)]
    const blocks = getSpecificTimeBlocks(times, 30)
    expect(blocks).toHaveLength(2)
    expect(blocks[0].slots).toEqual([at(0), at(30)])
    expect(blocks[1].slots).toEqual([at(90)])
  })
})

describe("buildSlotToBlockMap", () => {
  it("maps every slot to its block", () => {
    const blocks = getSpecificTimeBlocks([at(0), at(15), at(60)], 15)
    const map = buildSlotToBlockMap(blocks)
    expect(map.get(at(0))).toBe(blocks[0])
    expect(map.get(at(15))).toBe(blocks[0])
    expect(map.get(at(60))).toBe(blocks[1])
    expect(map.get(at(999))).toBeUndefined()
  })
})
