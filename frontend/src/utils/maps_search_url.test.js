import { describe, expect, it } from "vitest"
import { mapsSearchUrl } from "./general_utils"

// A location is stored as the text somebody typed or picked from the Google
// suggestions — never a place id or coordinates — so the link out to a map is a
// search over that text. Everything here is about surviving what people
// actually type into a venue field.

describe("mapsSearchUrl", () => {
  it("builds a maps search for a plain address", () => {
    expect(mapsSearchUrl("1600 Amphitheatre Parkway")).toBe(
      "https://www.google.com/maps/search/?api=1&query=1600%20Amphitheatre%20Parkway"
    )
  })

  it("escapes the characters that would otherwise break out of the query", () => {
    // "&" is the one that matters: unescaped it would start a new URL param.
    expect(mapsSearchUrl("The Fox & Hound")).toContain(
      "query=The%20Fox%20%26%20Hound"
    )
    expect(mapsSearchUrl("Ada's, #3")).toContain("query=Ada's%2C%20%233")
  })

  it("handles accents and non-Latin venue names", () => {
    expect(mapsSearchUrl("Café Über")).toBe(
      "https://www.google.com/maps/search/?api=1&query=Caf%C3%A9%20%C3%9Cber"
    )
  })

  // A list item is never empty (the server rejects blank text), but the event's
  // own location legitimately is before anyone sets one — the old inline
  // template produced a bare search rather than throwing, and that behaviour is
  // what the shared helper has to keep.
  it("returns a bare search for a missing location rather than throwing", () => {
    const bare = "https://www.google.com/maps/search/?api=1&query="
    expect(mapsSearchUrl("")).toBe(bare)
    expect(mapsSearchUrl(null)).toBe(bare)
    expect(mapsSearchUrl(undefined)).toBe(bare)
  })
})
