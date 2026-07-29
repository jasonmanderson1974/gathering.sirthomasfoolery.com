import { describe, expect, it } from "vitest"
import { displayName, rollDisplayName } from "./general_utils"

// These mirror User.DisplayName() in server/models/user.go — the two run over
// the same documents, so a divergence shows up as a name that changes when a
// page re-renders from a different source. Keep the cases in step.

describe("displayName", () => {
  it("prefers the nickname", () => {
    expect(
      displayName({
        firstName: "Bartholomew",
        lastName: "Fitzwilliam",
        nickname: "Bart",
      })
    ).toBe("Bart")
  })

  it("falls back to the full name", () => {
    expect(displayName({ firstName: "Ada", lastName: "Lovelace" })).toBe(
      "Ada Lovelace"
    )
  })

  it("does not leave a stray space when half the name is missing", () => {
    expect(displayName({ firstName: "Ada" })).toBe("Ada")
    expect(displayName({ lastName: "Lovelace" })).toBe("Lovelace")
  })

  it("treats a whitespace-only nickname as absent", () => {
    expect(
      displayName({ firstName: "Ada", lastName: "Lovelace", nickname: "   " })
    ).toBe("Ada Lovelace")
  })

  it("trims", () => {
    expect(displayName({ nickname: "  Bart  " })).toBe("Bart")
    expect(displayName({ firstName: " Ada ", lastName: " Lovelace " })).toBe(
      "Ada Lovelace"
    )
  })

  // Templates call this on values that are briefly null (a dialog's target
  // before it is set, a respondent still loading), so it must not throw.
  it("survives a missing or empty user", () => {
    expect(displayName(null)).toBe("")
    expect(displayName(undefined)).toBe("")
    expect(displayName({})).toBe("")
  })
})

describe("rollDisplayName", () => {
  it("pairs the nickname with the real name", () => {
    expect(
      rollDisplayName({
        firstName: "Bartholomew",
        lastName: "Fitzwilliam",
        nickname: "Bart",
      })
    ).toBe("Bart (Bartholomew Fitzwilliam)")
  })

  it("is the plain name when there is no nickname", () => {
    expect(rollDisplayName({ firstName: "Ada", lastName: "Lovelace" })).toBe(
      "Ada Lovelace"
    )
  })

  // An allowlist entry nobody has claimed has no name at all; a nickname
  // without a name shouldn't render empty parens.
  it("omits the parenthetical when there is no real name", () => {
    expect(rollDisplayName({ nickname: "Bart" })).toBe("Bart")
  })

  it("survives a missing or empty member", () => {
    expect(rollDisplayName(null)).toBe("")
    expect(rollDisplayName({})).toBe("")
  })
})
