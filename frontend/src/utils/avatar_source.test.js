import { describe, it, expect } from "vitest"
import { avatarSourceError, AVATAR_EXPORT_PX } from "@/utils/general_utils"

// H9. The finding was recorded as "the refusal is right, the wording is wrong";
// driving the real dialog showed the opposite — there was no refusal at all.
// getCroppedCanvas() returned a canvas for every source tried, 1x1 through
// 4000x3000 including 2000x10 strips, so the save-time branch never fired and
// each one was uploaded as a 256x256 smear. These pin the guard that replaces it.
describe("avatarSourceError", () => {
  it("accepts a real photo", () => {
    expect(avatarSourceError(4000, 3000)).toBeNull()
  })

  it("accepts a source exactly at the export size", () => {
    expect(avatarSourceError(AVATAR_EXPORT_PX, AVATAR_EXPORT_PX)).toBeNull()
  })

  it("rejects one pixel under, on either axis", () => {
    expect(avatarSourceError(AVATAR_EXPORT_PX - 1, 4000)).not.toBeNull()
    expect(avatarSourceError(4000, AVATAR_EXPORT_PX - 1)).not.toBeNull()
  })

  // The shortest side is what bounds a square crop, so a huge strip is still
  // too small — this is the 2000x10 case that uploaded a 1-2px smear.
  it("rejects a strip however long it is", () => {
    expect(avatarSourceError(2000, 10)).not.toBeNull()
    expect(avatarSourceError(10, 2000)).not.toBeNull()
  })

  it("rejects the sizes that were silently accepted before", () => {
    for (const [w, h] of [
      [1, 1],
      [8, 8],
      [32, 24],
      [64, 64],
      [100, 75],
      [150, 150],
    ]) {
      expect(avatarSourceError(w, h), `${w}x${h}`).not.toBeNull()
    }
  })

  // The whole reason it returns a message instead of a boolean: the complaint
  // that sent people in circles was the one that didn't say what was wrong.
  it("names the actual size and the minimum", () => {
    const message = avatarSourceError(100, 75)
    expect(message).toContain("100x75")
    expect(message).toContain(String(AVATAR_EXPORT_PX))
  })

  it("treats a source it cannot measure as unreadable", () => {
    expect(avatarSourceError(0, 0)).not.toBeNull()
    expect(avatarSourceError(NaN, NaN)).not.toBeNull()
    expect(avatarSourceError(undefined, undefined)).not.toBeNull()
  })
})
