import { describe, expect, it } from "vitest"
import { serverURL } from "@/constants"
import {
  avatarUrl,
  isOwnAvatarUrl,
  monogram,
  userFromDisplayName,
} from "./general_utils"

// The photo a member uploads is served immutably, so the URL is the only thing
// that tells a browser to fetch a new one. These cases are about that: which
// source wins, and whether the URL actually changes when the photo does.

describe("avatarUrl", () => {
  const uploaded = {
    _id: "6570a1b2c3d4e5f601020304",
    avatarUpdatedAt: "2026-07-28T19:41:32.792Z",
  }

  it("points at the serving route for a user with an uploaded photo", () => {
    expect(avatarUrl(uploaded)).toBe(
      `${serverURL}/users/6570a1b2c3d4e5f601020304/avatar?v=2026-07-28T19%3A41%3A32.792Z`
    )
  })

  it("prefers the uploaded photo over the Google picture", () => {
    const url = avatarUrl({
      ...uploaded,
      picture: "https://lh3.googleusercontent.com/a/whatever",
    })
    expect(url).toContain("/avatar?v=")
    expect(url).not.toContain("googleusercontent")
  })

  it("changes when the photo does", () => {
    // The server sends Cache-Control: immutable, so a stable URL would leave
    // the old photo on screen indefinitely after a re-upload.
    const before = avatarUrl(uploaded)
    const after = avatarUrl({
      ...uploaded,
      avatarUpdatedAt: "2026-07-28T20:02:11.004Z",
    })
    expect(after).not.toBe(before)
  })

  it("encodes the timestamp", () => {
    // avatarUpdatedAt is RFC 3339, not milliseconds — it carries colons, which
    // have meaning in a query string.
    expect(avatarUrl(uploaded)).not.toContain(":41:")
  })

  it("falls back to the Google picture", () => {
    expect(
      avatarUrl({ _id: "abc", picture: "https://example.test/p.jpg" })
    ).toBe("https://example.test/p.jpg")
  })

  it("returns empty when there is no photo of either kind", () => {
    expect(avatarUrl({ _id: "abc" })).toBe("")
    expect(avatarUrl(null)).toBe("")
    expect(avatarUrl(undefined)).toBe("")
  })

  it("accepts the roll's `userId` as well as `_id`", () => {
    // /admin/allowlist entries name the account id `userId` — an unclaimed
    // invitation has no account, so it can't live in `_id`.
    expect(
      avatarUrl({
        userId: "6570a1b2c3d4e5f601020304",
        avatarUpdatedAt: "2026-07-28T19:41:32.792Z",
      })
    ).toContain("/users/6570a1b2c3d4e5f601020304/avatar?v=")
  })

  it("prefers `userId` over `_id` when a row carries both", () => {
    // An allowlist entry's `_id` is the INVITATION, not the person. Reading it
    // first asked for the avatar of an id with no account — the roll and the
    // directory rendered a broken image for every member with a photo.
    expect(
      avatarUrl({
        _id: "aaaaaaaaaaaaaaaaaaaaaaaa",
        userId: "6570a1b2c3d4e5f601020304",
        avatarUpdatedAt: "2026-07-28T19:41:32.792Z",
      })
    ).toContain("/users/6570a1b2c3d4e5f601020304/avatar?v=")
  })

  it("ignores an avatar flag on a user with no id", () => {
    // Google contacts arrive with a picture and no account, so the id can't be
    // assumed — a URL built without one would 404.
    expect(
      avatarUrl({
        avatarUpdatedAt: "2026-07-28T19:41:32.792Z",
        picture: "https://example.test/p.jpg",
      })
    ).toBe("https://example.test/p.jpg")
  })
})

describe("monogram", () => {
  it("gives a nickname one initial", () => {
    expect(
      monogram({
        firstName: "Bartholomew",
        lastName: "Fitzwilliam",
        nickname: "Bart",
      })
    ).toBe("B")
  })

  it("gives a real name two", () => {
    expect(monogram({ firstName: "Ada", lastName: "Lovelace" })).toBe("AL")
  })

  it("copes with half a name", () => {
    expect(monogram({ firstName: "Ada" })).toBe("A")
    expect(monogram({ lastName: "Lovelace" })).toBe("L")
  })

  it("treats a whitespace-only nickname as absent", () => {
    // Matches displayName, so the monogram and the name beside it never
    // disagree about whether a nickname exists.
    expect(
      monogram({ firstName: "Ada", lastName: "Lovelace", nickname: "   " })
    ).toBe("AL")
  })

  it("falls back to the email for an account with no name", () => {
    expect(monogram({ email: "cecil@example.test" })).toBe("C")
  })

  it("has something to show for a user with nothing to go on", () => {
    expect(monogram({})).toBe("?")
    expect(monogram(null)).toBe("")
  })
})

// The serving route requires a session. Only our own avatar URLs may carry the
// credentialed-CORS attribute that makes an <img> send one — Google's CDN
// refuses credentialed requests, so marking a `picture` URL would break the
// fallback for every member without an uploaded photo.
describe("isOwnAvatarUrl", () => {
  it("recognises the avatar serving route", () => {
    expect(
      isOwnAvatarUrl(
        avatarUrl({
          _id: "6570a1b2c3d4e5f601020304",
          avatarUpdatedAt: "2026-07-28T19:41:32.792Z",
        })
      )
    ).toBe(true)
  })

  it("rejects the Google picture fallback", () => {
    expect(
      isOwnAvatarUrl(
        avatarUrl({ picture: "https://lh3.googleusercontent.com/a/whatever" })
      )
    ).toBe(false)
  })

  it("rejects a lookalike host", () => {
    // Guards the check against being loosened to a bare "/users/" contains.
    expect(isOwnAvatarUrl("https://evil.test/users/123/avatar")).toBe(false)
  })

  it("handles a user with no avatar at all", () => {
    expect(isOwnAvatarUrl(avatarUrl({}))).toBe(false)
    expect(isOwnAvatarUrl("")).toBe(false)
    expect(isOwnAvatarUrl(null)).toBe(false)
    expect(isOwnAvatarUrl(undefined)).toBe(false)
  })
})

// The fallback for rows whose account did not resolve: a guest, or an account
// since deleted. Its whole job is to feed monogram, so the cases are about the
// initials that come out the far end.
describe("userFromDisplayName", () => {
  it("splits a two-word name into the same two initials the account would give", () => {
    expect(monogram(userFromDisplayName("Ada Lovelace"))).toBe("AL")
  })

  it("gives a one-word name one initial", () => {
    expect(monogram(userFromDisplayName("Cecil"))).toBe("C")
  })

  it("uses the last word of a three-part name, matching monogram on the account", () => {
    expect(monogram(userFromDisplayName("Ada King Lovelace"))).toBe("AL")
  })

  it("ignores surrounding and repeated whitespace", () => {
    expect(monogram(userFromDisplayName("  Ada   Lovelace  "))).toBe("AL")
  })

  it("yields '?' for a name it cannot use", () => {
    // A roster row always renders an avatar, so every one of these has to
    // produce something rather than throw.
    expect(monogram(userFromDisplayName(""))).toBe("?")
    expect(monogram(userFromDisplayName("   "))).toBe("?")
    expect(monogram(userFromDisplayName(null))).toBe("?")
    expect(monogram(userFromDisplayName(undefined))).toBe("?")
  })
})
