/*
 * The Fellowship directory, reduced to the shape a hover card renders (N3).
 *
 * WHY THIS IS A SEPARATE MODULE: the join it performs is the whole feature, and
 * getting it wrong is silent. A checklist byline knows an `assigneeId` and a
 * name string; a comment knows a `userId`; an expense split knows a `userId`.
 * None of them knows a phone number, because the server strips `Phone` from
 * every event payload on purpose (`stripSensitiveUserFields`,
 * server/routes/event_responses.go). So the card cannot read what it shows off
 * the object beside it — it has to look the person up. Keeping the lookup here,
 * as plain functions over plain data, is what lets the `node` tier test it
 * without mounting anything.
 *
 * Not exported through `src/utils/index.js`: nothing else needs it, and the
 * barrel is already imported by ~40 components.
 */

/**
 * Index `/admin/allowlist` rows by the account they belong to.
 *
 * Rows with no `userId` are DROPPED, and that is the point rather than an
 * oversight: an allowlist row is an *invitation*, and its `_id` is the
 * invitation, not the person. An unclaimed invite has no account, no avatar and
 * no name — there is nothing for a card to show and nothing that could ever
 * hover it, since a person with no account has never posted, responded or been
 * assigned anything.
 *
 * This is the same trap `avatarUrl` documents (general_utils.js): take `_id`
 * here and every lookup silently keys on the wrong id space, matching nothing.
 */
export const indexDirectory = (rows) => {
  const byId = {}
  for (const row of Array.isArray(rows) ? rows : []) {
    if (row && row.userId) byId[row.userId] = row
  }
  return byId
}

/**
 * An account id fit to look up, or "" when there isn't one.
 *
 * The zero check is not defensive padding. A non-pointer `primitive.ObjectID`
 * cannot be omitted by `omitempty` — it is a [12]byte, which encoding/json never
 * considers empty — so an entry with no author serializes `userId` as
 * **24 zeros**, and that reads as a real id everywhere that only checks for
 * presence. `EventListItem.AssigneeId` is a POINTER specifically to dodge this;
 * `UserId` and `Comment.UserId` are not, so anything taking an id off a list
 * entry has to filter the zero itself.
 *
 * Left as a hex-string test rather than a length/format test: a legacy row can
 * hold a guest's NAME in an id field, and a name is not an account either.
 */
export const accountId = (id) => {
  const hex = typeof id === "string" ? id.trim() : ""
  if (!hex || /^0+$/.test(hex)) return ""
  return /^[0-9a-f]{24}$/i.test(hex) ? hex : ""
}

/** Trimmed "First Last", or "" when neither half is set. */
const realNameOf = (source) =>
  `${(source.firstName ?? "").trim()} ${(source.lastName ?? "").trim()}`.trim()

/**
 * What the card should render for one person, or null when it should not open
 * at all.
 *
 * `record` is the directory entry (everything) or a public profile (names only)
 * or nothing; `fallback` is whatever user object the call site already had in
 * hand. The record wins field by field rather than wholesale, so a card can
 * show the avatar the page is already displaying while the phone number is
 * still in flight.
 *
 * Returning null is the "render the slot bare" signal, and it is deliberately
 * generous about what counts as nothing: a bare name string with no account
 * behind it (a guest respondent, a deleted author, a legacy name-keyed RSVP)
 * produces a fallback with a first/last name and no id, and a card offering a
 * 96px monogram and nothing else is worse than no card. So a name ALONE is not
 * enough — something only an account has must be present.
 */
export const personDetail = (record, fallback) => {
  const rec = record ?? {}
  const fb = fallback ?? {}

  const nickname = (rec.nickname ?? fb.nickname ?? "").trim()
  const realName = realNameOf(rec.firstName || rec.lastName ? rec : fb)
  const email = (rec.email ?? fb.email ?? "").trim()
  const phone = (rec.phone ?? fb.phone ?? "").trim()

  // An account is what makes a card worth opening. `avatarUpdatedAt` and
  // `picture` both mean "there is a photo"; an email or phone is contact detail
  // the page is not already showing. A nickname counts too — it is a fact about
  // the account that a byline showing the real name would not have told you.
  const hasAccount = Boolean(
    rec.userId ||
      rec._id ||
      fb._id ||
      email ||
      phone ||
      nickname ||
      rec.avatarUpdatedAt ||
      fb.avatarUpdatedAt ||
      rec.picture ||
      fb.picture
  )
  if (!hasAccount) return null
  if (!nickname && !realName && !email) return null

  return {
    // Handed straight to UserAvatarContent, which resolves `userId ?? _id` and
    // needs `avatarUpdatedAt`/`picture` alongside the names for its monogram.
    avatarUser: { ...fb, ...rec },
    nickname,
    realName,
    email,
    phone,
    role: rec.role ?? fb.role ?? "",
  }
}
