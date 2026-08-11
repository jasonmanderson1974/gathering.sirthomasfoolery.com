import { describe, it, expect } from "vitest"
import {
  applyMention,
  filterMentionables,
  flattenMentions,
  mentionTrigger,
  splitMentions,
} from "./mentionText"

const ada = {
  _id: "aaaaaaaaaaaaaaaaaaaaaaaa",
  firstName: "Ada",
  lastName: "Lovelace",
}
const bart = {
  _id: "bbbbbbbbbbbbbbbbbbbbbbbb",
  firstName: "Bart",
  lastName: "Adams",
}

/** The token the composer writes and the server parses. */
const token = (user) => `@[${user.firstName} ${user.lastName}](${user._id})`

describe("splitMentions", () => {
  it("returns a single text part for a comment with no mentions", () => {
    expect(splitMentions("Bring the tent")).toEqual([
      { type: "text", text: "Bring the tent" },
    ])
  })

  it("returns nothing for empty or missing text", () => {
    expect(splitMentions("")).toEqual([])
    expect(splitMentions(null)).toEqual([])
    expect(splitMentions(undefined)).toEqual([])
  })

  it("splits a mention out of the surrounding text", () => {
    expect(splitMentions(`Ask ${token(ada)} about it`)).toEqual([
      { type: "text", text: "Ask " },
      { type: "mention", text: "Ada Lovelace", userId: ada._id },
      { type: "text", text: " about it" },
    ])
  })

  it("keeps whitespace between parts verbatim, since the discussion renders pre-wrap", () => {
    const parts = splitMentions(`  ${token(ada)}\n\nsecond line`)

    expect(parts[0]).toEqual({ type: "text", text: "  " })
    expect(parts[2]).toEqual({ type: "text", text: "\n\nsecond line" })
  })

  it("handles adjacent mentions with no text between them", () => {
    expect(splitMentions(`${token(ada)}${token(bart)}`)).toEqual([
      { type: "mention", text: "Ada Lovelace", userId: ada._id },
      { type: "mention", text: "Bart Adams", userId: bart._id },
    ])
  })

  it("does not restart mid-comment — a shared regex's lastIndex would skip matches", () => {
    const text = `${token(ada)} and ${token(bart)}`

    // Twice, because a module-level global regex would find the second mention
    // on the first call and the first mention on the second.
    expect(splitMentions(text)).toEqual(splitMentions(text))
    expect(
      splitMentions(text).filter((p) => p.type === "mention")
    ).toHaveLength(2)
  })

  it("leaves text that only looks like a token alone", () => {
    const cases = [
      "@[Ada Lovelace](not-an-id)", // id isn't 24 hex
      "@[Ada Lovelace](AAAAAAAAAAAAAAAAAAAAAAAA)", // uppercase hex is not what Mongo emits
      "@[](aaaaaaaaaaaaaaaaaaaaaaaa)", // empty display half
      "email me at ada@example.com",
    ]

    for (const text of cases) {
      expect(splitMentions(text)).toEqual([{ type: "text", text }])
    }
  })
})

describe("flattenMentions", () => {
  it("rewrites tokens to what the author typed", () => {
    expect(flattenMentions(`Ask ${token(ada)} about it`)).toBe(
      "Ask @Ada Lovelace about it"
    )
  })

  it("flattens every token, not just the first", () => {
    expect(flattenMentions(`${token(ada)} ${token(bart)}`)).toBe(
      "@Ada Lovelace @Bart Adams"
    )
  })

  it("passes through text with no mentions, and tolerates no text", () => {
    expect(flattenMentions("Bring the tent")).toBe("Bring the tent")
    expect(flattenMentions(null)).toBe("")
  })
})

describe("mentionTrigger", () => {
  it("opens on a bare @ at the start of the field", () => {
    expect(mentionTrigger("@")).toEqual({ start: 0, query: "" })
  })

  it("opens on an @ after whitespace, reporting where it starts", () => {
    expect(mentionTrigger("Ask @ad")).toEqual({ start: 4, query: "ad" })
    expect(mentionTrigger("first line\n@ad")).toEqual({
      start: 11,
      query: "ad",
    })
  })

  it("allows spaces inside the partial so a two-word name still matches", () => {
    expect(mentionTrigger("Ask @Ada Lo")).toEqual({ start: 4, query: "Ada Lo" })
  })

  it("does not open on an @ inside a word — an email address is not a mention", () => {
    expect(mentionTrigger("ada@example")).toBeNull()
  })

  it("does not open on a token that has already been inserted", () => {
    expect(mentionTrigger(`Ask ${token(ada)} `)).toBeNull()
    expect(mentionTrigger(`Ask ${token(ada)}`)).toBeNull()
  })

  it("closes once the partial runs past what a name could be", () => {
    expect(mentionTrigger(`@${"a".repeat(40)}`)).toBeNull()
  })

  it("ignores an @ that isn't the last thing typed", () => {
    expect(mentionTrigger("@ada said hello, but")).toBeNull()
  })
})

describe("filterMentionables", () => {
  const roll = [
    ada,
    bart,
    { _id: "cccccccccccccccccccccccc", nickname: "Cadfael" },
  ]

  it("offers everyone, in the order given, for a bare @", () => {
    expect(filterMentionables(roll, "").map((u) => u._id)).toEqual(
      roll.map((u) => u._id)
    )
  })

  it("matches case-insensitively on the display name", () => {
    expect(filterMentionables(roll, "LOVEL").map((u) => u._id)).toEqual([
      ada._id,
    ])
  })

  it("matches a nickname, since that is the name shown", () => {
    expect(filterMentionables(roll, "cadf").map((u) => u._id)).toEqual([
      "cccccccccccccccccccccccc",
    ])
  })

  it("puts prefix matches ahead of ones that only contain the query", () => {
    // "ad" starts Ada Lovelace, and appears inside both Bart Adams and Cadfael
    // — which keep the order they were given in.
    expect(filterMentionables(roll, "ad").map((u) => u._id)).toEqual([
      ada._id,
      bart._id,
      "cccccccccccccccccccccccc",
    ])
  })

  it("drops anyone with no display name — there is nothing to put in the token", () => {
    expect(
      filterMentionables([{ _id: "dddddddddddddddddddddddd" }], "")
    ).toEqual([])
  })

  it("caps the list", () => {
    const many = Array.from({ length: 20 }, (_, i) => ({
      _id: `${i}`.padStart(24, "0"),
      firstName: "Person",
      lastName: `${i}`,
    }))

    expect(filterMentionables(many, "")).toHaveLength(8)
    expect(filterMentionables(many, "", 3)).toHaveLength(3)
  })

  it("tolerates no candidates at all", () => {
    expect(filterMentionables(null, "ad")).toEqual([])
  })
})

describe("applyMention", () => {
  it("replaces the partial with a token the server's pattern matches", () => {
    const result = applyMention("Ask @ad", 7, ada)

    expect(result.text).toBe(`Ask ${token(ada)} `)
    expect(result.caret).toBe(result.text.length)
    expect(splitMentions(result.text)[1]).toEqual({
      type: "mention",
      text: "Ada Lovelace",
      userId: ada._id,
    })
  })

  it("keeps the text after the caret and lands the caret past the token", () => {
    const result = applyMention("Ask @ad about it", 7, ada)

    expect(result.text).toBe(`Ask ${token(ada)}  about it`)
    expect(result.text.slice(result.caret)).toBe(" about it")
  })

  it("leaves the trigger closed, so the picker doesn't reopen on the name inserted", () => {
    const result = applyMention("Ask @ad", 7, ada)

    expect(mentionTrigger(result.text.slice(0, result.caret))).toBeNull()
  })

  it("uses the nickname, matching what the picker showed", () => {
    const cadfael = {
      _id: "cccccccccccccccccccccccc",
      firstName: "Brother",
      nickname: "Cadfael",
    }

    expect(applyMention("@ca", 3, cadfael).text).toBe(
      `@[Cadfael](${cadfael._id}) `
    )
  })

  it("refuses a candidate that can't be written as a token", () => {
    expect(applyMention("@a", 2, { _id: ada._id })).toBeNull() // no name
    expect(applyMention("@a", 2, { _id: "nope", firstName: "Ada" })).toBeNull() // no ObjectID
    expect(applyMention("@a", 2, null)).toBeNull()
  })

  it("returns null when there is no partial mention at the caret", () => {
    expect(applyMention("Bring the tent", 14, ada)).toBeNull()
  })

  it("strips characters the token can't hold rather than emitting one that won't parse", () => {
    const awkward = { _id: ada._id, firstName: "Ada]", lastName: "Love\nlace" }
    const result = applyMention("@a", 2, awkward)

    expect(result.text).toBe(`@[Ada Love lace](${ada._id}) `)
    expect(splitMentions(result.text)[0].type).toBe("mention")
  })

  it("truncates a name past the pattern's 60-character bound", () => {
    const long = { _id: ada._id, firstName: "A".repeat(80) }
    const result = applyMention("@a", 2, long)

    expect(splitMentions(result.text)[0]).toEqual({
      type: "mention",
      text: "A".repeat(60),
      userId: ada._id,
    })
  })
})
