import { describe, expect, it } from "vitest"
import { indexDirectory, personDetail } from "./directory"

/*
 * The join behind the hover card (N3).
 *
 * Both of these fail silently when they are wrong — an index keyed on the wrong
 * id matches nothing and every card just never opens, which reads as "the
 * feature didn't ship" rather than as a bug. So the id-space cases below are
 * the point of the file, not padding.
 */

describe("indexDirectory", () => {
  it("keys on userId, not on the allowlist row's own _id", () => {
    // `_id` is the INVITATION; `userId` is the account. Every call site holds an
    // account id, so keying on `_id` would match none of them.
    const byId = indexDirectory([
      { _id: "invite-row-1", userId: "acct-1", email: "ambrose@example.test" },
    ])
    expect(Object.keys(byId)).toEqual(["acct-1"])
    expect(byId["acct-1"].email).toBe("ambrose@example.test")
  })

  it("drops invitations nobody has claimed", () => {
    // No account ⇒ no avatar, no name, and nothing that could ever hover it.
    const byId = indexDirectory([
      { _id: "invite-row-2", email: "nobody@example.test" },
      { _id: "invite-row-3", userId: "acct-2", email: "ada@example.test" },
    ])
    expect(Object.keys(byId)).toEqual(["acct-2"])
  })

  it("survives a response that is not an array", () => {
    // A failed fetch surfaces as {} through fetch_utils, and a card hovering
    // during an outage must not throw inside a computed — Vue 3 blanks the
    // whole subtree for that.
    expect(indexDirectory(undefined)).toEqual({})
    expect(indexDirectory({})).toEqual({})
    expect(indexDirectory([null])).toEqual({})
  })
})

describe("personDetail", () => {
  const directoryRow = {
    _id: "invite-row-1",
    userId: "acct-1",
    email: "bart@example.test",
    phone: "+15550100",
    firstName: "Bartholomew",
    lastName: "Fitzwilliam",
    nickname: "Bart",
    role: "member",
    avatarUpdatedAt: "2026-08-12T10:00:00Z",
  }

  it("returns every field the card renders", () => {
    const detail = personDetail(directoryRow, null)
    expect(detail).toMatchObject({
      nickname: "Bart",
      realName: "Bartholomew Fitzwilliam",
      email: "bart@example.test",
      phone: "+15550100",
      role: "member",
    })
  })

  it("hands the avatar an object that resolves to the ACCOUNT id", () => {
    // avatarUrl prefers `userId` over `_id` precisely because of this row
    // shape; asking for the invitation's avatar is a 404 and a broken image.
    const { avatarUser } = personDetail(directoryRow, null)
    expect(avatarUser.userId).toBe("acct-1")
    expect(avatarUser.avatarUpdatedAt).toBe("2026-08-12T10:00:00Z")
  })

  it("lets the record win over what the page already had", () => {
    // The page shows a stale snapshot name; the directory is the live record.
    const detail = personDetail(directoryRow, {
      _id: "acct-1",
      firstName: "Bartholomew",
      lastName: "Smith",
    })
    expect(detail.realName).toBe("Bartholomew Fitzwilliam")
    expect(detail.phone).toBe("+15550100")
  })

  it("renders what it has while the rest is still in flight", () => {
    // No record yet: the fetch rides the 500ms open delay and may not have
    // landed. The card still opens on the object the call site held.
    const detail = personDetail(null, {
      _id: "acct-1",
      firstName: "Ada",
      lastName: "Lovelace",
    })
    expect(detail).toMatchObject({
      realName: "Ada Lovelace",
      email: "",
      phone: "",
    })
  })

  it("gives a guest's public profile a card with no contact details", () => {
    // GET /users/:id returns names + photo and nothing else, by design.
    const detail = personDetail(
      {
        _id: "acct-1",
        firstName: "Ada",
        lastName: "Lovelace",
        nickname: "Ada",
        picture: "https://example.test/a.jpg",
      },
      null
    )
    expect(detail.nickname).toBe("Ada")
    expect(detail.email).toBe("")
    expect(detail.phone).toBe("")
  })

  it("returns null for a bare name with no account behind it", () => {
    // userFromDisplayName's shape: a guest respondent, a deleted author, a
    // legacy name-keyed RSVP. A 96px monogram and nothing else is worse than
    // leaving the name inert.
    expect(
      personDetail(null, { firstName: "Percival", lastName: "Thorne" })
    ).toBeNull()
    expect(personDetail(null, null)).toBeNull()
    expect(personDetail(undefined, undefined)).toBeNull()
  })

  it("opens for an account that has nothing but an id and a name", () => {
    expect(
      personDetail(null, { _id: "acct-9", firstName: "Reginald" })
    ).toMatchObject({ realName: "Reginald" })
  })

  it("does not open for an id with no name and no contact detail at all", () => {
    expect(personDetail(null, { _id: "acct-9" })).toBeNull()
  })
})
