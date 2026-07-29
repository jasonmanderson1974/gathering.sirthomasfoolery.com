# Timeful / The Fellowship — Backlog 2 (feature track + carried-forward items)

> Compiled **2026-07-28** after the security/cleanup wave in `TODO.md` completed and deployed.
> `TODO.md` is now the **closed archive** — everything in it is done except the four items
> carried forward into Part G below (re-stated here with corrections found in the 2026-07-28
> re-review, so `TODO.md` no longer needs updating).
>
> Context unchanged: self-hosted, invite-only fork for a ~30–40 person club. Reliability and
> small-club utility over scale. All event access requires sign-in (E3); roles are
> superAdmin > admin > member > guest.
>
> **Status: IN PROGRESS.** F1, H1, F2, F3, F4 and F5 landed 2026-07-28 (`0b5f2272`, `80a20905`,
> `719cb4b8`, `0a9f450d`, `6c281faf`, F5 this commit); F6, H4, H6 and H7 landed 2026-07-29 — see
> their entries for what differed from the plan. F11 and F12 were opened out of F5, H9 out of F6.
> **F13/F14 (Lists of Lists) were added and built 2026-07-29** (`c7ad40b4`, `647a07bc`, plus
> `c6493dc3` making the lists collapsible on a direct request), taking priority over the mention
> track at the user's request. **F7 and F8 landed 2026-07-29** — F7 and F8 are both
> deployed-safe on their own: tokens render as literal text until F9, and mention emails already
> fire for anyone who hand-writes a token.
> **F15/F16 (Lists of Lists v2) were added and built 2026-07-29** (`090a42ea`, `165756b3`,
> `fa0615ee`, `f698432b`), taken ahead of F9 at the user's request — lists are expected to carry
> real weight (5–7 people on one event's lists at once), so the side panel became a tabbed
> full-width band, items nest three deep, a checklist kind arrived and refreshes stopped paying
> for a whole-event refetch. **F9 (the mention composer + rendering) is next**, and is the last
> of the feature track before F10's close-out.
> The feature designs in Part F
> were reviewed against the codebase on 2026-07-28 and the product decisions in the two
> "Confirmed decisions" blocks were made by the user.
> **Line numbers below shift as items land — re-verify before starting one.**

Priority legend: **P0** = do first · **P1** = high value · **P2** = moderate · **P3** = nice-to-have.
Effort: **S** ≈ <½ day · **M** ≈ 1–2 days · **L** ≈ 3+ days.

---

## Confirmed decisions (user, 2026-07-28)

- **Avatar storage: MongoDB.** Cropped image (~200KB cap) in a new `avatars` collection, served
  via `GET /api/users/:userId/avatar`. No Docker volume / compose changes; avatars ride the
  existing `mongodump` backup story.
- **Mention scope:** members+ can @mention anyone on the Fellowship roll; guest-role users can
  only mention people already visible on the event (respondents / RSVPs / comment authors).
- **Nickname display:** nickname replaces the name everywhere it appears; The Roll
  (`MemberAdmin.vue`) and The Fellowship directory (`Fellowship.vue`) show
  **"Nickname (Real Name)"** so identities stay legible to admins and the directory.

## Confirmed decisions (user, 2026-07-29) — Lists of Lists (F13/F14)

- **Priority: ahead of mentions.** F13/F14 are next; F7–F9 follow. They share no code, so the
  order costs nothing either way.
- **List kind is fixed at creation** — the planner picks "text" or "locations" when making the
  list, rather than every item choosing. A locations list uses the Google address lookup for new
  items and renders each entry as a maps link.
- **Whole-list create/rename/delete: planner + admins.** Members and guests cannot remove a list
  someone else set up, even though they can remove individual items in it.
- **Items: anyone signed in adds; you may edit and delete your own; members+ may delete
  anyone's.** Editing is own-only — there is no edit-anyone right at any role.

## Confirmed decisions (user, 2026-07-29) — Lists of Lists **v2** (F15/F16)

Taken after F13/F14 shipped, on the expectation that lists carry real weight: not many events at
once, but **5–7 people working a single event's lists simultaneously** is likely. That number is
what drives the refresh design below — it is too many for "refetch on your own writes only", and
far too few to justify a realtime layer this app has never had.

- **Tabbed, full width — not a side panel.** Discussion and Lists become two tabs over one
  full-width band. The tab is labelled **"Lists"**; the existing list/entry vocabulary stays
  (the user's "Tasks" was shorthand, not a rename).
- **Nesting: 3 levels total** — item → child → grandchild, and a grandchild takes no children.
  **All three kinds nest**, not just checklists. Every level is independently collapsible.
- **Third kind: Checklist**, beside Anything (`text`) and Places (`location`). Entries carry a
  checkbox **anyone signed in may tick or untick**, guests included — same reasoning as adding
  items. Track **only the last person to change the state**, in both directions, with no
  history: "Checked by Ada" / "Unchecked by Bart".
- **Deleting an item deletes its subtree** (cascade), atomically. The right to delete an item is
  the right to delete what hangs off it.
- **Refresh is explicit, not polled**: on selecting the Lists tab, on expanding a list, and from
  a refresh icon in the panel's top-right. No `setInterval` — the app has never had one and
  three deliberate triggers cover a 5–7-person work session.

---

## PART F — Feature track (P1 unless noted)

Each item is independently green-deployable to trunk (new fields `omitempty`, endpoints additive,
UI reads defensively). See "Suggested sequencing" at the bottom for the current order — with F7
and F8 landed, **F9 (the composer) is next**, and it is the last of the feature track before F10's
close-out. Cheap Part-H items slot between deploys.

- [x] **F1 · OTP code visible/copyable from the Android email notification.** `S`
  **DONE 2026-07-28** (`0b5f2272`). Built as planned, with one correction: `buildOtpEmailBody`
  called `RenderEmail` (:168), not `RenderEmailWithFooter` — the chain is
  `RenderEmail → RenderEmailWithFooter → RenderEmailWithPreheader` (new base), and both OTP
  builders call the new helper directly. `autocomplete="one-time-code"` shipped too.
  **Still needs the manual Android check** (Gmail's copy-code action is heuristic).
  The subject already leads with the code (`routes/auth.go` ~:469: `"%s is your Fellowship
  sign-in code"`), but the notification preview pulls body text that doesn't repeat it.
  - `server/utils/email_layout.go`: new `RenderEmailWithPreheader(preheader, heading, bodyHTML,
    footerHTML string, actions ...EmailAction)`; existing `RenderEmailWithFooter` (:175)
    delegates with `preheader=""` — zero caller churn. Preheader = hidden div immediately after
    `<body>` (`display:none`, 1px, zero max-size, `mso-hide:all`, padded with `&nbsp;&zwnj;`
    repeats so Gmail's snippet doesn't fall through into body text). Input HTML-escaped like
    every other layout helper.
  - `routes/auth.go` `buildOtpEmailBody` (~:367): preheader `"<code> is your Fellowship sign-in
    code"`. Mirror on the email-change OTP (`routes/user.go` ~:334 body, ~:202 subject).
  - Optional freebie: `autocomplete="one-time-code"` on the code inputs in `SignIn.vue`
    (~:127-176) and `Settings.vue` (~:88-98).
  - Tests: extend the email-layout/auth-email tests (preheader present, escaped, absent when
    empty; both OTP bodies carry it). **Final verification is manual on a real Android device**
    (Gmail's copy-code smart action is heuristic — best-effort, note it in the commit).

- [x] **F2 · Nickname: model, API, `displayName` sweep.** `M`
  **DONE 2026-07-28** (`719cb4b8`). Built as planned. Notes for F3/F5/F6/F7:
  - `allowlistMember.UserId` is a **hex string, not an ObjectID** — `omitempty` cannot omit an
    ObjectID (a `[12]byte` array), so an unclaimed invitation would ship a zero id reading as
    real. Do the same for any id added to that struct.
  - Three planned sweep sites deliberately skipped: the `$posthog.identify` payloads
    (`App.vue:307`, `SignIn.vue:371`, `Auth.vue:55`) are analytics *traits*, not displayed
    names, and G1 deletes the stub; `EmailInput.vue:159` `contactToQueryString` operates on
    Google contacts, which never carry a nickname, and displayName would drop its
    middle-name trim.
  - Search and sort follow the displayed name now (respondents, CSV export, directory) —
    sorting on `firstName` while rendering a nickname misorders visibly.
  - The roll lives at **`/members`** (route *named* `admin`), not `/admin`.
  - `server/models/user.go`: `Nickname string` (`json:"nickname" bson:"nickname,omitempty"`) +
    method `(u *User) DisplayName()` — nickname if non-empty, else trimmed "First Last".
    `HasCustomName` untouched (it only guards Google-sign-in name overwrites; sign-in never
    writes nickname).
  - `server/routes/user.go`: new `PATCH /user/nickname` mirroring the phone handler — trim +
    cap ~40 runes via `trimAndTruncate` (`routes/text.go`); empty clears (`$unset`).
    `GET /user/profile` serializes the whole struct, so the field flows automatically.
  - `server/routes/users.go` `getPublicUserProfile`: add `Nickname` to the hand-built struct.
  - `server/routes/admin.go` `allowlistMember`: add `Nickname` + `UserId` (userId is needed by
    F5/F6/F7 for avatar URLs, admin edit, and mention identity — add now).
  - Frontend: `displayName(user)` (nickname || first+last) and `rollDisplayName(member)`
    ("Nickname (Real Name)") helpers in `utils/general_utils.js`. Sweep the ~22
    `firstName + " " + lastName` concat sites: `AuthUserMenu.vue:15`,
    `RespondentsList.vue:124-127,280,453`, `ScheduleOverlap.vue:617-628`,
    `ConfirmDetailsDialog.vue:61,95,254`, `ExportCsvMenu.vue:113,120,157`,
    `SignUpBlock.vue:86,109`, `EmailInput.vue:38,158`, `Settings.vue:15,22`,
    `UserAvatarContent.vue:21` (monogram initial → first char of displayName),
    `pluginMessagesMixin.js:407`, `App.vue:307-308`, `SignIn.vue`/`Auth.vue`.
    `MemberAdmin.vue:90` + `Fellowship.vue:111,184,220-224,230` use `rollDisplayName`.
    **Do NOT touch the guest-sentinel comparisons** `user._id == user.firstName`
    (`RespondentsList.vue:505`, `respondentSelectionMixin.js:71`) — the comparison stays on
    `firstName`; only rendered text changes.
  - `Settings.vue`: nickname field in the Profile section, saved via the **phone pattern**
    (patch → `refreshAuthUser` → `showInfo`) — not `saveName`'s `window.location.reload()`.
  - Tests: Go pure `DisplayName` + DB-gated PATCH set/clear; vitest for both helpers.
    Swagger regen.

- [x] **F3 · Nickname read-time resolution for the four name snapshots.** `M` — needs F2.
  **DONE 2026-07-28** (`0a9f450d`). Built as planned; helpers live in new `routes/display_names.go`
  (`resolveEventDisplayNames` is the one entry point). Notes:
  - **No frontend change was needed.** The server overwrites the *serialized* `authorName`, and
    `addComment` — the only handler returning a comment directly — snapshots `DisplayName()`,
    so `comment.authorName` is always current. The planned
    `comment.author ? displayName(comment.author) : comment.authorName` branch would imply the
    two can diverge. `comment.author` is still attached and is what **F5** should use for the
    avatar (it has no `avatarUpdatedAt` yet — F4 adds the field, then add it to
    `slimUserForDisplay`).
  - **Chronicle needed no change**: `captureAttendees` (`services/reminders/reminders.go:205`)
    copies `rsvp.Name`, which is now a `DisplayName()` snapshot — history freezes correctly.
  - Resolution runs *after* `visibleComments`, so a hidden thread's authors are never looked up.
  Names are snapshotted at write time in `Comment.AuthorName` (`routes/comments.go:224`),
  `Rsvp.Name` (`event_responses.go:927`), poll `Votes` values (`polls.go:244-250`), and
  `ChronicleAttendee.Name` — a nickname change would not propagate without this.
  - New `db.GetUsersByIds([]ObjectID) map[string]User` (`db/users.go`, one `$in` query);
    `getEvent` batch-resolves ONE shared id set across comments + rsvps + votes.
  - Comments: new computed field `Author *models.User` (`bson:"-"`; slim — id, names, nickname,
    picture, avatarUpdatedAt via `stripSensitiveUserFields` + Email cleared) attached after
    `visibleComments`; serialized `AuthorName` overwritten with `DisplayName()` when the author
    resolves. Stored `AuthorName` remains the fallback (deleted users, legacy guest rows).
    Write path snapshots `DisplayName()`.
  - RSVPs + poll votes: write path switches to `DisplayName()`; read path overwrites display
    values for ObjectID-keyed entries only (name-keyed legacy guest entries untouched).
  - Chronicle: snapshot-time `DisplayName()` only — history stays frozen; `Chronicle.vue`
    unchanged.
  - Frontend: `CommentRow.vue:12` / `EventComments.vue:34` →
    `comment.author ? displayName(comment.author) : comment.authorName`. RSVP/poll components
    need no change (server serves resolved names).
  - Tests: pure fallback test for the attach helper; DB-gated getEvent test (nicknamed user's
    comment/RSVP/vote render the nickname); PII assertion that `Author` carries no
    email/phone/role (extend the `events_pii_db_test.go` matrix).

- [x] **F4 · Avatar backend: storage, upload, serving.** `M` — parallelizable with F2/F3.
  **DONE 2026-07-28.** Built as planned. Notes for F5/F6:
  - **`avatarUpdatedAt` serializes as an RFC 3339 string, not milliseconds** —
    `primitive.DateTime.MarshalJSON` emits a time string, same as every other timestamp this
    API returns. So F5's cache-buster is
    `"?v=" + encodeURIComponent(user.avatarUpdatedAt)`, not a number. The `ETag` is
    milliseconds, deliberately: it is opaque and only ever echoed back, so the two never need
    to agree.
  - **Downscaling is a hand-written box filter** (`boxDownscale`, `routes/avatars.go`) — the
    stdlib has no resampler and `golang.org/x/image` isn't a dependency. It only ever shrinks;
    an upload smaller than 256px is stored at its own size, not upscaled. If a future item
    needs real resampling, that's the place to swap in `x/image/draw`.
  - **Three guards, not one.** `http.MaxBytesReader` on the body (512KB) → encoded/decoded byte
    caps (300KB/200KB) → a `DecodeConfig` pixel cap (16MP) *before* `image.Decode`. The last one
    is the only thing bounding memory: Go's decoders allocate from the declared bounds, so a
    2KB PNG announcing 30000×30000 would otherwise allocate gigabytes.
  - Transparency is flattened onto **white** (JPEG has no alpha; unflattened comes out black).
    Rectangles are centre-cropped. Re-encoding strips EXIF — including orientation, which is
    why F5's rotate buttons must bake rotation into the pixels before sending.
  - **The 304 path uses `c.AbortWithStatus`, not `c.Status`** — gin's `Status` only records the
    code on its writer and waits for a later write to flush it, which a bodyless 304 never
    makes. (`c.Status` there returns a silent 200.)
  - `deleteUser` now also drops the avatar (best-effort) — the bytes are in their own
    collection and don't go with the account otherwise.
  - `avatarUpdatedAt` is exposed on `getPublicUserProfile`, `allowlistMember` (so F6's roll can
    render photos) and `slimUserForDisplay` (so F5's comment avatars have it), plus every
    event-response `*models.User` for free.
  - F6 reuses `saveAvatarForUser(userId, dataURL)` / `db.DeleteAvatar(userId)` verbatim; both
    take a user id and know nothing about who is asking.
  - New `avatars` collection — deliberately NOT on the User doc: `GetUserById` runs on every
    authed request and `getEvent` embeds a full `*models.User` per respondent, so a 200KB
    Binary there would be fetched constantly. `models/avatar.go`:
    `{Id ObjectID (== user id), Data primitive.Binary, ContentType string, UpdatedAt DateTime}`;
    register `AvatarsCollection` in `db/init.go`; `db/avatars.go` Upsert/Get/Delete.
  - `models/user.go`: `AvatarUpdatedAt *primitive.DateTime` — doubles as has-avatar flag and
    cache-buster. Add to `getPublicUserProfile` and `allowlistMember` too.
  - `PUT /user/avatar` (behind existing auth), JSON `{image: "<data URL>"}` — fits the
    JSON-only `fetch_utils.js`, no multipart plumbing. Shared helper in new
    `routes/avatars.go`: `saveAvatarForUser(userId, imageBase64)` — strip data-URI prefix,
    decode (reject > ~300KB encoded / 200KB decoded), `image.Decode` (jpeg+png), downscale to
    256×256, **re-encode JPEG q85** (canonicalizes + strips EXIF), upsert + `$set
    avatarUpdatedAt`. New errs codes `invalid-image` / `image-too-large`.
    `DELETE /user/avatar` clears both.
  - `GET /users/:userId/avatar` in `routes/users.go` — ~~unauthenticated (consistent with
    `getPublicUserProfile` exposing `Picture`)~~ **superseded 2026-07-28: the route requires a
    session** (user's call on the deploy pass). A member's face is membership data on an
    invite-only club, and the plan's reasoning — that it is no more private than the Google
    `picture` URL — was too weak to leave every photo fetchable by anyone holding a user id.
    Gated per-route, not on the `/users` group, so `getPublicUserProfile` beneath it stays
    reachable while signed out; `routes/users_auth_gate_test.go` drives the real `InitUsers`
    so a later route can't land on the wrong side of the gate unnoticed. Two knock-ons:
    `Cache-Control` is **`private`**, not `public` (an authed response behind Cloudflare must
    not be storable by a shared cache), and `UserAvatarContent` sets
    `crossorigin="use-credentials"` on **our own** avatar URLs only — production is same-origin
    so the cookie rides along already, but `npm run serve` is not, and marking the Google
    `picture` fallback would break it (that CDN refuses credentialed requests).
    Otherwise unchanged: `max-age=31536000, immutable`, `ETag` = updatedAt,
    `If-None-Match` → 304, 404 when none, 401 when signed out.
  - Tests: pure decode/validate/re-encode (generate tiny PNG/JPEG fixtures in-test via the
    `image` package; junk + oversized rejection); DB-gated PUT→GET→304→DELETE round-trip.
    Swagger regen. Deployable alone — endpoints are inert without UI.

- [x] **F5 · Avatar frontend: cropper dialog + `UserAvatarContent` rollout.** `M/L` — needs F4
  (+F3 for comment avatars).
  **DONE 2026-07-28.** Built as planned, with two decisions taken with the user first (rosters
  skipped — see F11; monogram unified on the Fellowship palette) and these notes for F6:
  - **`avatarUrl()` prefers `userId` over `_id`, and the order is load-bearing.** An
    `allowlistMember` embeds `AllowlistEntry.Id` as `_id` — the *invitation* row, not the person —
    so reading `_id` first built `/users/<invitationId>/avatar` and the roll and directory showed a
    broken image for every member with a photo. Caught by the browser pass, not by lint, tests or
    the build. Regression test in `src/utils/avatar.test.js`.
  - **Two pure helpers in `general_utils.js`, not component logic** (`avatarUrl`, `monogram`): the
    vitest env is `node` with no jsdom and no `@vue/test-utils`, so anything inside a `.vue` file
    is untestable without new dev deps. Same shape F2 used for `displayName`. `monogram` absorbed
    `Fellowship.vue`'s local `initials()`, which is now deleted.
  - **`cropperjs@^1.6` is loaded with a dynamic `import()`**, so it lands in its own 40KB chunk
    (verified: `dist/js/643.*`) rather than in `app.js`. v2 is a rewrite with a custom-element API
    that does not fit `new Cropper(img, …)`. It adds no transitive deps and no new audit findings.
  - **Follow-up fixed on the deploy pass (2026-07-28): no Tailwind display utility on an element
    a library hides for you.** `tw-block` on the source `<img>` made the dialog render the photo
    twice — once full-size above the cropper — because `tailwind.config.js` sets
    `important: true`, so `.tw-block` emits `display: block !important`, which ties with
    cropperjs's `.cropper-hidden { display: none !important }` on specificity and wins on order.
    Invisible to lint, tests and the build; only a screenshot showed it. Same trap applies to any
    vendor CSS that hides an element it adopts.
  - **A missing dynamically-imported dep is a webpack *warning*, not an error** — `npm run build`
    printed `Module not found: Can't resolve 'cropperjs'` and still exited 0 with "Build
    complete". A local build is therefore not proof the dep is installed: run `npm install` after
    pulling a commit that adds one, and grep the build output for `Module not found`.
  - `AvatarEditorDialog` **does not know the endpoint** — it emits a data URL, and the caller PUTs
    it. F6 reuses it as-is: `$refs.avatarEditor.pickFile()`, then PUT to the admin route.
    Teardown hangs off a `value` watcher, not the Cancel button, so it also fires when the caller
    closes the dialog after a successful save.
  - **The anonymize gate now hides the photo AND the monogram**, not just the name — both identify
    a person as surely as the name does. Extracted as `isVisible(response)` in `SignUpBlock.vue`
    and verified live in both directions.
  - `UserAvatarContent` shows one initial below 32px and two at or above it (`CalendarAccount`
    renders at 24, `SignUpBlock` at 16, where "AL" is a smudge).
  - Verified live: upload → crop → rotate → save → the photo appears on settings, the roll, the
    directory and the event page; `?v=` changes on re-upload; removal restores the monogram and
    the serving route 404s. Both standing browser checks stay green.
  - Dep: `cropperjs@^1.6` (framework-agnostic, Vue-2-safe under webpack 5; avoids wrapper-
    version risk). Verify the lockfile with **npm 10** (CI parity — see memory/feedback).
  - New `components/settings/AvatarEditorDialog.vue`: hidden `<input type="file"
    accept="image/*">` → `FileReader` → `<img>` in a `v-dialog` → `new Cropper(img,
    {aspectRatio: 1, viewMode: 1})`; rotate buttons call `cropper.rotate(90)`; Save emits
    `getCroppedCanvas({width:256, height:256}).toDataURL("image/jpeg", 0.85)`. Caller decides
    the endpoint (so F6 can reuse it for admin-on-behalf uploads): Settings does
    `put("/user/avatar", {image})` → `refreshAuthUser` → `showInfo`; "Remove photo" →
    `_delete("/user/avatar")`.
  - `UserAvatarContent.vue`: computed `avatarSrc` — `user.avatarUpdatedAt ? serverURL +
    "/users/" + user._id + "/avatar?v=" + avatarUpdatedAt : user.picture`; uploaded avatar wins
    over the Google `picture` URL; monogram fallback unchanged. The 5 existing usage sites
    (AuthUserMenu, RespondentsList, ConfirmDetailsDialog, settings/CalendarAccount, EmailInput)
    get avatars for free.
  - Rollout to the sites that bypass it today: `CommentRow.vue:3-5` (hardcoded mdi-account →
    component, pass `comment.author` from F3), `SignUpBlock.vue:74-83,103` (hand-rolled img),
    `UserChip.vue:7-14`, `Fellowship.vue:97-101` (CSS monogram; needs F2's userId +
    avatarUpdatedAt on `allowlistMember`), `MemberAdmin.vue` rows, `GatheringRsvp.vue` roster,
    `EmailInput.vue` (align its hand-rolled item-slot img). `EventPolls.vue` voter roster only
    if voter ids are client-visible — else skip.
  - `Settings.vue`: avatar block in Profile (UserAvatarContent size 96 + Change/Remove).
  - Tests: vitest for `avatarSrc`. Verify live on the dev stack: upload → crop → rotate → save;
    avatar shows in header/respondents/comments; `?v=` changes on re-upload;
    `check-signed-in.js` green.

- [x] **F6 · Admin+ edits a Member/Guest's name, nickname, photo.** `M` — needs F2, F4/F5.
  **DONE 2026-07-29.** Built as planned. Notes:
  - **Names may be edited but not erased** — a sent-but-blank name is a new 400
    `invalid-name`, because `DisplayName` falls back to the name when there is no
    nickname, so an empty pair would leave the person nameless on every surface. Nickname
    keeps the opposite rule (empty = clear), matching self-serve.
  - **All-pointer `db.UserProfileUpdate`**, so "omitted" is distinguishable from "sent
    empty". `hasCustomName` is set only when a *name* field is present — a nickname-only
    PATCH must not pin a name nobody chose.
  - **`UpdateUserProfileByEmail` handles the empty-update case explicitly**: Mongo rejects
    an empty update document, but "change nothing" is a legitimate submit from an untouched
    form. It confirms the account exists and reports a match without writing, so the
    caller's 400-on-no-account still fires.
  - The dialog binds to `editEmail` and resolves `editingMember` out of `members` each
    render rather than editing a copied row — that is what refreshes the photo preview in
    place after an upload, with no extra plumbing.
  - **Two defects only the browser pass caught** (lint, tests and the build were all green):
    Vuetify `solo` fields drop their `label` once they hold a value, so a pre-filled form
    showed unlabelled boxes — use captions above the field, as Settings does. And
    **`UserAvatarContent`'s monogram border drew four tick marks instead of a ring** since
    F5: `v-avatar` clips to a circle, so a square border survives only where the square
    touches the circle. Fixed with `tw-rounded-full`; it was visible on every roll row.
  - `AvatarEditorDialog` reused unchanged apart from an optional `title` prop. One instance
    outside the `v-for` — a per-row copy would mean a hidden file input per member.
  - Verified live end-to-end: admin edits another member's name/nickname, uploads and
    removes their photo through the cropper, member gets 403 on all three writes while
    keeping read access to the roll. Both standing harnesses green.
  - `PATCH /admin/member/profile` `{email (required), firstName, lastName, nickname *string}`.
    Guards in order (reuse the patterns at `admin.go:213-226, 262-278`): actor
    `CanManageUsers()` (admin+); superAdmin target → 403 (immutability precedent); target must
    have an account (`db.GetUserByEmail`) else 400 `user-does-not-exist` — allowlist entries
    with `hasAccount=false` are not editable and the UI hides the control; self-edit allowed
    (harmless; Settings can do the same anyway). New `db.UpdateUserProfileByEmail(email,
    fields)` using the `SetUserRole` case-insensitive-collation pattern (`db/users.go:61-80`).
    Name edits set `hasCustomName: true`; `nickname` uses pointer semantics (nil = leave,
    `""` = `$unset`, value = `$set`).
  - `PUT /admin/member/avatar` `{email, image}` + `DELETE /admin/member/avatar` `{email}` —
    same guards, then F4's `saveAvatarForUser(targetUser.Id, image)` / delete.
  - Register in `InitAdmin` (`admin.go:32-40`). **The `/admin` group is member+-gated
    (`CanInviteRequired`), so the per-handler admin checks are load-bearing — test a plain
    member gets 403 on all three.**
  - `views/MemberAdmin.vue`: pencil per row (`hasAccount && canManageUsers && !superAdmin`),
    edit dialog with name/nickname fields + `AvatarEditorDialog` reuse; `busyEmail` per-row
    busy pattern; re-fetch the allowlist on save.
  - Tests: DB-gated matrix — admin edits member/guest OK; member 403; superAdmin target 403;
    no-account 400; nickname clear via `""`; avatar-for-other round-trip. Swagger regen.

- [x] **F7 · @Mentions backend: parsing, storage, mentionables endpoint.** `M` — needs F2.
  **DONE 2026-07-29.** Built as planned. Notes for F8/F9:
  - **The mentions field is written on BOTH paths and an edit re-parses rather than merges**, so
    `db.UpdateCommentText` `$unset`s it when an edit removes the last mention. Leaving stale ids
    behind would have poisoned F8's edit-diff — they would read as already-notified.
  - **`mentionableUserIds` takes its inputs rather than fetching them**, and the comments handed
    to it must already be through `visibleComments` for that caller. That pairing is what keeps a
    members-only thread's author out of a guest's picker; there is a test asserting exactly this,
    because the filter and the set are otherwise easy to wire up independently and get wrong.
  - **A guest's own event is not in the visible set unless someone is on it.** The event OWNER is
    deliberately NOT mentionable by a guest: `ownerId` is serialized but the owner's NAME is never
    rendered from it, so adding them would have been a new disclosure rather than a mention of
    someone already on the page. Revisit if F9's picker feels short for guests.
  - **Extra DB work is confined to guest authors** — validating a member's mentions is one
    `GetUsersByIds`; a guest's additionally loads the event's responses and comments. Fine at club
    scale, and the alternative (trusting the client's picker) is the thing being defended against.
  - `sortMentionables` orders by display name so the picker is stable between requests (Mongo's
    natural order is not) — F9 can render the list as it arrives.
  - Route added to the `eventRoutes` auth-gate table in `event_auth_gate_test.go`, which is what
    caught it being registered without an entry. Swagger regenerated.
  - Tests: 12 pure (parse matrix incl. 9 malformed-token shapes, the cap, dedupe across differing
    display names, the visible-set matrix, sort) + 9 DB-gated (roll vs event-only, members-only
    author never offered, payload carries no email/phone/role, strip-on-write for a guest, the
    same token kept for a member, unknown account dropped, edit swap + `$unset`).
  <details><summary>Original F7 design (as planned 2026-07-28)</summary>

  - Token format persisted inside comment text: `@[Display Name](24-hex-userId)` —
    self-contained, survives edits, regexable (`@\[([^\]\n]{1,60})\]\(([0-9a-f]{24})\)`);
    counts against the existing 2000-rune cap (acceptable).
    `models/comment.go`: add `Mentions []primitive.ObjectID` (`bson/json omitempty`) for
    edit-diffing and querying.
  - New `routes/mentions.go`, pure + unit-testable: `parseMentions(text)` (dedupe,
    order-preserving, cap 10 — extras stay as text but drop from Mentions/notifications) and
    `mentionableUserIds(event, responses, comments)` — the guest-visible set: ObjectID-keyed
    respondents (skip name-keyed guest rows), `SignUpResponses` keys, `Rsvp.UserId`s,
    ObjectID-parsing poll-vote keys, and authors of `visibleComments` for that viewer.
  - `GET /events/:eventId/mentionables` (events group, behind `AuthRequired`): member+ →
    every user account (new `db.GetAllUsers` with slim projection `{_id, firstName, lastName,
    nickname, picture, avatarUpdatedAt}` — small club, fine); guest-role → resolved
    `mentionableUserIds`, same shape. One endpoint serves both roles → single source of truth.
  - Write-path validation in `addComment`/`editComment`: after `sanitizeCommentText`, parse;
    drop ids that don't resolve to users, and for guest-role authors drop ids outside the
    visible set (strip from `Mentions`, keep the literal text). `db.UpdateCommentText` grows a
    mentions param.
  - Tests: pure parse/visible-set matrices; DB-gated endpoint test (member sees all, guest sees
    event-only) + guest strip-on-write. Deployable alone — tokens render as literal text until
    F9, no emails until F8.
  </details>

- [x] **F8 · Mention notification emails.** `M` — needs F7.
  **DONE 2026-07-29.** Built as planned, in a new `routes/mention_emails.go`. Notes for F9:
  - **`flattenMentions` lives in `mentions.go`, not here** — it is the inverse of the token
    pattern and has to change with it. F9's JS `splitMentions` is the frontend twin; if the token
    shape is ever revised, all three move together.
  - **`mentionThreadIsMembersOnly` fails CLOSED.** A reply carries no flag of its own, so the root
    is read back; if it cannot be resolved the comment is treated as members-only and nothing is
    sent. Withholding an email is recoverable, mailing a hidden thread's contents to a guest is
    not. DB-gated test covers all five shapes incl. the unresolvable root.
  - **Thread context is resolved PER RECIPIENT**, not once per comment: two people named in the
    same reply may see different amounts of the thread above it. A recipient who cannot see the
    root gets no context block at all rather than replies stripped of the question they answer.
  - **The rate limit is counted per EMAIL, not per comment** (30/hr per author), so one comment
    naming ten people spends ten. Matches the "30 mention emails/hr" the plan specified, and is
    what actually bounds inbox damage.
  - **`emailTextBlock` escapes THEN inserts `<br>`.** A comment is the most attacker-controlled
    string the app mails anywhere; doing it the other way round would neuter the breaks and, worse,
    normalise inserting markup before escaping in this file. Test asserts both halves.
  - Rendered a sample against the dev stack and read the output as text to check the copy actually
    scans — quoted context, flattened tokens, preserved line breaks, CTA and fallback URL.
  - Tests: 23 pure (flatten incl. a `$` in a display name, which Go's replacement templates eat if
    done naively; the recipients matrix — self/dedupe/edit-diff/members-only×roles/no-address; the
    context matrix — cap, other threads, hidden root; body assertions incl. escaping of all five
    user-controlled fields) + 3 DB-gated/env.
  <details><summary>Original F8 design (as planned 2026-07-28)</summary>

  - `notifyMentions(event, comment, author, newMentionIds, allComments)` called from
    `addComment` (all mentions) and `editComment` (**diff vs stored `Mentions` — newly-added
    only**). Per recipient: skip self; skip dupes (parse-dedupe + diff); **skip when the
    comment sits in a members-only thread (root resolved via `ThreadId`) and the recipient's
    `EffectiveRole()` lacks `CanSeeMembersOnly()`** — the hard privacy rule; the token may
    still appear in text, which leaks only a name INTO the hidden thread, never content OUT —
    document it. Rate limit: `utils.NewRateLimiter` keyed by author id (~30 mention emails/hr,
    OTP-limiter precedent `routes/auth.go:31-34`); on limit, skip sending silently — the
    comment still posts.
  - Email via the escaped `email_layout.go` helpers: subject
    `"<AuthorDisplayName> mentioned you in <Event Name>"`; body = event name + comment text
    with tokens flattened to `@Display Name` (pure `flattenMentions(text)` helper), plus thread
    context when the comment is a reply — the root + up to 3 preceding sibling replies (from
    `db.GetComments`, filtered through `visibleComments` for the **recipient**), CTA
    `EmailAction{View the discussion → GetBaseUrl() + "/e/" + id}` via `RenderEmailWithFooter`.
    Send with `utils.SendEmailAsync`; wholly best-effort (never fails the request); inherits
    the existing Gmail-env gating.
  - Tests: extract pure `mentionRecipients(...)` — self / guest-in-members-only / dedupe /
    edit-diff matrix; body assertions incl. HTML-escaping of comment text (the layout helpers
    escape, verify end-to-end); DB-gated persistence of `Mentions`.
  </details>

- [x] **F9 · Mention composer UI + rendering.** `L` — needs F7 (F8 independent of this).
  **DONE 2026-07-29.** Built as planned apart from the picker's mechanics, which the browser
  had opinions about. Four notes, three of them traps worth keeping:
  - **The picker is a plain absolutely-positioned list, not a `v-menu`.** A v-menu owns focus
    and an overlay; both fight a textarea the user is mid-sentence in. A list under the field
    needs neither and keeps every keystroke in the composer.
  - **Enter must be bound with `@keydown`, not `@keydown.native`.** `VTextarea.onKeyDown` calls
    `e.stopPropagation()` on Enter while focused (Vuetify's own guard against Enter closing a
    surrounding dialog), so a native listener on the component root sees every key *except* the
    one that matters. It re-emits the event afterwards, so `preventDefault` on the emitted one
    still suppresses the newline. Cost a debugging session; caught only by driving a real
    browser, since it can't fail in a node test.
  - **Escape needed a memory.** `@keyup` re-reads the caret, so the keyup ending the same
    keypress found the same partial mention and reopened the picker Escape had just closed —
    Escape looked like a no-op. `dismissedStart` keeps that one mention dismissed while a later
    `@` still opens normally.
  - `viewerId` is a prop on `CommentRow` rather than a store read, keeping the row
    presentational; `EventComments.threadTitle` now flattens before truncating; candidates are
    fetched fire-and-forget in `Event.vue`'s `created()` so the picker can't hold up the
    calendar grid, and an empty list just means the composer behaves as it did before mentions.
  - Verified live against the dev stack with the CDP harness (16 assertions): picker opens and
    filters, Enter inserts a token the server's pattern matches without breaking the line,
    Escape dismisses and stays dismissed, the posted comment renders `@Ada Lovelace` styled and
    never as raw markup, a mention of the reader is highlighted and one of someone else is not,
    thread headers flatten, the reply composer picks too. Confirmed end-to-end: the token the
    composer wrote persisted with `mentions: [ObjectId(…)]`, and a guest session's
    `/mentionables` returned only the one person visible on the event, not the roll.
  - Rendering: new `components/event/mentionText.js` — pure `splitMentions(text)` →
    `[{type:'text'|'mention', text, userId}]` (vitest). `CommentRow.vue` renders mention parts
    as brass-colored spans, the viewer's own mention with a highlighted background; flatten
    tokens in `EventComments.vue`'s `threadTitle` previews.
  - Composer: new `MentionTextarea.vue` wrapping `v-textarea`, used by BOTH the main composer
    and the per-thread reply composer in `EventComments.vue`. On input/click/keyup, test the
    text before the caret against `/(^|\s)@([\w][\w ]{0,30})?$/`; when matched, open a `v-menu`
    anchored below the textarea (not caret-tracked — fine for v1) listing candidates filtered
    against `displayName(u)`; intercept ArrowUp/Down/Enter/Tab/Esc while open; selection
    replaces the `@partial` with `@[${displayName(u)}](${u._id}) ` and restores focus/caret.
  - Candidates fetched once per event from `GET /events/:eventId/mentionables`, passed down
    from `Event.vue`; mentions ride the existing text payload (no `EventService` change).
    Edit mode shows raw tokens in the textarea — documented v1 limitation (tokens survive
    edits by design).
  - Tests: vitest for `splitMentions` + the trigger/replace logic (keep them pure functions in
    `mentionText.js`). Verify live: member mentions anyone on the roll; guest session (via
    `server/tools/mintsession`) sees only event-visible candidates; email lands with thread
    context; a members-only-thread mention of a guest sends nothing.

- [ ] **F10 · Close-out sweep.** `S` — after the feature track (F1–F9 + F13/F14): final
  `swag init --parseDependency --parseInternal`, full backend suite
  (`go test $(go list ./... | grep -v '/scripts')` with `MONGODB_URI`),
  `npm run test:unit` + build + lint, both headless harnesses
  (`check-signed-out.js` / `check-signed-in.js`), tick items off here.

- [ ] **F11 · Avatars on the RSVP and poll-voter rosters.** `M` · **P3** — deliberately left out
  of F5 (user's call, 2026-07-28), not an oversight. Both rosters are comma-joined text
  (`"Going: Bart, Ada (+1)"` — `GatheringRsvp.vue:65-75`, `EventPolls.vue:61-67`), so this is a
  presentation change, not a component swap:
  - RSVPs are the cheap half: attach `slimUserForDisplay(user)` to each `Rsvp` in
    `resolveEventDisplayNames` (`routes/display_names.go` already batches exactly this lookup),
    then rewrite the roster into avatar+name rows per status group.
  - Poll votes are the expensive half: `Votes map[string]string` stores a bare display name per
    voter key, so voters can only be rendered with an avatar after a stored-shape change
    (`{name, user}`), which drags in the write path and legacy rows. Probably not worth it.

- [ ] **F12 · `getEvent` nil-derefs on an event response with no `response` field.** `S` · **P3** —
  `routes/events.go:534` does `response.User = user` without checking `response != nil`, and
  `getResponsesMap` happily yields a nil for a row whose `response` is absent. **Not reachable
  through the app** — every write path sets `Response: &response`
  (`routes/event_responses.go:271,283`) — so this is a guard against legacy or hand-edited rows,
  found while seeding fixtures for F5. One `if response == nil { continue }` alongside the
  existing deleted-user branch, plus a DB-gated test. Currently a 500 on the app's hottest
  endpoint for the whole event, not just the bad row.

- [x] **F13 · Lists of Lists backend: model, routes, permissions.** `M` · **P1**
  **DONE 2026-07-29** (`c7ad40b4`). Built as planned. Notes:
  - **`requireEventManager` was not reusable here and is not called.** The plan said
    "`requireEventManager(...) || viewer.IsAdmin`", but that function *writes its own 403* and
    takes a `*gin.Context`, so or-ing it with a second condition would emit an error response on
    a request that then succeeds. `canManageLists` restates the rule as a pure method on
    `listViewer` instead — which is also what made the whole permission matrix unit-testable
    without a request. Behaviour is identical, including the legacy-ownerless member+ fallback.
  - **The item cap is advisory and says so in a comment.** It is measured against the event as
    read, so two simultaneous adds at the boundary can both pass and land at 101. Enforcing it
    exactly would mean reading and rewriting the whole array — the precise lost-update bug the
    feature was designed around — so the guardrail loses and concurrency wins.
  - **`addEventListItem` re-checks the write landed** (`modified == false` → 404): the cap check
    reads the event, and the list can be deleted between that read and the `$push`. Without it
    the caller gets a 200 for an item that went nowhere.
  - The auth-gate table in `event_auth_gate_test.go` had to grow the six new routes and two new
    id placeholders — it drives the real `InitEvents`, so it failed loudly until it did. Working
    exactly as intended; nothing landed outside the authed group.
  - `eventDisplayNameIds` now takes a fourth argument (`lists`); the existing pure test was
    extended rather than duplicated, and asserts an item with no author id contributes nothing.
  - Caps as planned (100 / 300 runes, 20 lists, 100 items). Swagger regenerated — all four list
    paths present.

- [x] **F14 · Lists of Lists frontend: the panel beside the discussion.** `M` · **P1**
  **DONE 2026-07-29** (`647a07bc`). Built as planned. Notes:
  - **Draft entry text is a per-list map written through `$set`, not `v-model`.** One shared
    draft string would have leaked what you typed in one list into every other list's field, and
    `v-model="newItemText[list._id]"` into a key that does not exist yet is the Vue 2 reactivity
    trap — the field would not update. Both inputs (plain and `LocationInput`) use
    `:value` + `@input` for the same reason.
  - **`canManageUsers` comes from `mapGetters`, not `this.$store.getters`** — first draft mixed
    the two styles in one component.
  - Verified live on the dev stack across two viewports and both roles, plus a click-through of
    the add flow (type → Add → round-trip → field clears → count updates), because F5 and F6 each
    shipped a defect that lint, tests and the build all passed. Nothing visual was wrong this
    time. Both standing harnesses green; seeded fixtures removed afterwards.
  - `mapsSearchUrl` extracted to `general_utils.js` and adopted by `EventLocation.vue`, with
    vitest covering `&`-escaping (the one that would otherwise split the query string) and the
    empty/nil case the event's own location still hits before a venue is set.
  - **Follow-up, direct request (`c6493dc3`): the lists collapse like discussion threads.** Each
    list is a clickable header (chevron, name, entry count) with its entries and add-entry field
    inside, mirroring `EventComments`' `expandedThreads`. Lists start collapsed, so the panel
    reads as an index rather than a wall — which matters most on a phone, where it sits below the
    whole discussion. Two details the pattern needed: the rename/delete buttons live in the header
    so their clicks are `.stop`ped, and a just-created list opens itself (the create path only
    knows the name it sent, so the watcher matches the refetched list by name, from the end).
  - **The 2/3 + 1/3 side-panel layout is superseded by F16** (2026-07-29) — the panel becomes the
    Lists tab of a full-width tabbed band. Everything else here still stands; the collapse idiom
    above is what F16's per-item nesting copies.

- [x] **F15 · Lists v2 backend: nesting, checklists, cascade delete, a cheap lists GET.** `M` ·
  **P1** — no F-deps (extends F13). **DONE 2026-07-29** (`090a42ea`). Built as planned. Notes:
  - **`SetEventListItemChecked` reports `MatchedCount`, not `ModifiedCount`** — the plan said so
    and it turned out to be load-bearing twice over: re-ticking an already-ticked box modifies
    nothing, and so does an uncheck that repeats the previous state. Either would have 404'd on
    a row that is plainly there.
  - **`collectDescendantIds` needed a `seen` set to terminate, not just to dedupe.** The
    first draft walked the frontier without one; a two-item cycle grows the id list forever
    rather than merely repeating. Same class of bug as the depth walk's bound, and the cycle
    tests are what forced both.
  - The depth-cycle test initially asserted the wrong invariant (that a cycle reports a depth
    *over* the cap). A cycle in an n-item list reports exactly n, and n ≥ 2 for any cycle, so
    what actually holds — and what the caller relies on — is `depth+1 >= maxListItemDepth`,
    i.e. the add is refused. The code was right; the assertion wasn't.
  - `deleteEventListItem` keeps its own permission check on the *root* item only. Deleting an
    item is deleting its subtree, so a guest removing their own entry takes a member's reply
    with it — the thread-root precedent, stated in the Swagger description because it is the
    one rule here someone would otherwise be surprised by.
  - Verified live against the dev stack before the UI existed: nested add stores `parentId`, a
    4th level 400s, a guest ticks and is credited, the owner's uncheck re-credits to the owner,
    and a cascade takes the grandchild while sparing the sibling.
  - **Nesting is stored flat**: `ParentId *primitive.ObjectID` on `EventListItem`
    (`models/event.go:262-268`), nil/absent = top-level, so **existing items need no migration**.
    A pointer, not a bare ObjectID — `omitempty` cannot omit a `[12]byte` array, and a zero id
    would serialize as 24 zeros and read as a real parent (the F2 lesson, restated because it
    bites the same way here).
    Nested *arrays* were rejected: every mutation in `db/event_lists.go` is a single atomic
    `UpdateByID` with 2-level `arrayFilters`, and depth-N arrays would drag back the whole-array
    rewrite that the package comment there exists to warn against (polls' lost-update bug).
  - **Depth is validated at insert** against the event as read, and unlike the item cap that is
    exact rather than advisory: items are never re-parented, so an item's depth is immutable once
    written. `maxListItemDepth = 3`; new pure `listItemDepth(list, item)` walks `ParentId`
    (missing parent counts as root, walk bounded by item count so malformed data can't loop) and
    `addEventListItem` rejects `parent depth + 1 >= 3` with 400 `list-depth-exceeded`.
    The one race left is the parent being deleted between read and `$push`, which yields an
    **orphan** — rendered at root by the frontend, not swept server-side.
  - **Cascade delete**: `collectDescendantIds(list, rootId)` (pure, includes the root) then ONE
    `$pull {"lists.$[l].items": {"_id": {"$in": ids}}}` — a single-document update, therefore
    atomic. `db.DeleteEventListItem` is **replaced** by `DeleteEventListItems(eventId, listId,
    itemIds)` rather than kept alongside it: the single-item case is a slice of one, and two code
    paths for one operation is how they drift.
  - **Checklist kind**: `ListKindChecklist = "checklist"` (`models/event.go:253-256`),
    `validListKind` widens. Four new fields on the item — `Checked bool`,
    `CheckedBy *primitive.ObjectID`, `CheckedByName string`, `CheckedAt primitive.DateTime`, all
    `omitempty`. Absent = never toggled; after the first toggle all four are always `$set`
    together, so `checked:false` **with** a name renders "Unchecked by Bart" while an untouched
    item renders nothing. `omitempty` on the bool is safe because the write is a literal `bson.M`
    `$set` (struct tags don't apply to it) and an absent field reads falsy on the client.
  - `PUT /events/:eventId/lists/:listId/items/:itemId/checked` `{checked}` — **no gate beyond
    being signed in**, same reasoning as `addEventListItem`. Bind as `Checked *bool` with
    `binding:"required"`: a bare `bool` would reject `false`, i.e. every uncheck. Wrong kind of
    list → 400 `not-a-checklist`. `db.SetEventListItemChecked` returns **MatchedCount > 0**, not
    ModifiedCount — re-checking an already-checked item legitimately modifies nothing and must
    not 404.
  - `GET /events/:eventId/lists` — `loadListContext` (so, any signed-in user, the same access
    `getEvent` already grants post-E3), resolve names, return the bare lists array with nil
    normalized to `[]`. **This is the point of the whole refresh story**: `getEvent`
    (`routes/events.go:496-699`) does an N+1 `GetUserById` per availability responder *and*
    another per sign-up response before it ever reaches the lists, so refetching the event to see
    one new checkbox is absurd at 5–7 concurrent users. New `resolveListDisplayNames(lists)` in
    `display_names.go` = `eventDisplayNameIds(nil,nil,nil,lists)` → one `db.GetUsersByIds` →
    `resolveListItemNames`. Zero N+1.
  - Read-time name resolution grows to cover the checker: `eventDisplayNameIds`' lists loop
    (`display_names.go:90-97`) also collects `CheckedBy` when non-nil, and
    `resolveListItemNames` (:162-175) overwrites `CheckedByName` in the same pass — so a nickname
    change propagates to "Checked by …" exactly as F3 did for authors.
  - **Both new routes must be added to the `eventRoutes` table** in `event_auth_gate_test.go`
    (:23-63) — `TestEventRoutes_TableCoversEveryRegisteredRoute` fails until they are, which is
    the intended behaviour and how F13's six routes were caught. `/l1`/`/i1` placeholders already
    map back; `/checked` is a literal segment and needs no replace-chain entry. Also extend
    `listsTestRouter` (`event_lists_db_test.go:19-29`), which duplicates the route table by hand.
  - Tests: pure — `validListKind` + checklist; `listItemDepth` (root/child/grandchild/orphan/
    cycle-terminates); `collectDescendantIds` (leaf, full subtree, siblings excluded); the
    display-name additions. DB-gated — nested add stores `parentId`, a 4th level 400s, unknown
    parent 404s; cascade delete takes the grandchild and spares the sibling, works across authors,
    stays idempotent; check→uncheck round-trip with attribution both directions, 400 on a text
    list, nickname resolving at read time without writing back; the GET (401 anonymous, resolved
    names signed in, `[]` not `null` when empty). Swagger regen.

- [x] **F16 · Lists v2 frontend: the tabbed band, the tree, the checkboxes.** `L` · **P1** —
  needs F15. **DONE 2026-07-29** (`165756b3` plumbing, `fa0615ee` tabs, `f698432b` tree +
  checklists). Built as planned, in the three commits the plan called for. Notes:
  - **Nothing was wrong in the browser this time** — the first list feature where that is true.
    The checks below all passed first try, which is worth recording precisely because F5, F6
    and F14 each needed a second pass.
  - **The leading-icon block was restructured mid-build.** The first draft branched
    chevron → checkbox → bullet with a *second* checkbox rendered afterwards for the
    checklist-entry-with-children case, which is two ways to draw one control. Replaced with a
    fixed-width chevron slot (empty when there are no children, so text still lines up down the
    column) followed by exactly one marker.
  - **A stray `npx vitest` from the repo root** silently created a root `node_modules/` and ran
    the tests under vitest 4 instead of the project's 3.2.4. They passed either way, but the
    run proved nothing about the project's config. Run vitest from `frontend/`.
  - Live-verified with a throwaway CDP script (the `browser-check-lib.js` driver, not committed):
    tab select and list expansion each fire exactly one `GET /lists` and no whole-event fetch;
    three levels indent 8/24/48px with no "+" on the third; a guest's tick renders
    "Checked by …" and an uncheck re-credits; a place still takes free text with the Google key
    unset; the sub-entry composer round-trips and lands nested; expanded state survives a tab
    switch (the `v-show` decision, confirmed rather than assumed); and a guest on an event with
    no lists gets "No lists yet." rather than the blank band the old root `v-if` would have
    left. Both standing harnesses green, fixtures removed.
  - **Plumbing first, UI unchanged.** `EventService.js` gains `getLists(eventId)` (needs the
    `get` import — the lists section has only ever used post/patch/put/delete) and
    `setListItemChecked(...)`. `Event.vue` gains `refreshLists()` — `getLists` then
    `this.$set(this.event, "lists", lists)` — and all six list handlers (:605-658) swap
    `refreshEvent()` for it. Safe to splice: `processEvent` never touches `lists`, and
    `EventLists`' `pendingExpandName` watcher keys off `lists` changing, so create-then-auto-expand
    still fires. A `refreshingLists` flag drops overlapping calls (tab-select and expand can
    coincide).
  - **Tabs: the `NewDialog.vue:15-39` idiom** (a row of small text `v-btn`s, active class
    `tw-bg-brass/10 tw-text-brass`), not `SlideToggle` — that is a segmented *value* control with
    equal-width sliding border, the wrong register for switching panels. The app has no `v-tabs`
    anywhere and this stays consistent with what it does have.
    `bandTab` lives in `Event.vue` (it owns `refreshLists`, so the watcher sits next to what it
    calls). Panels are **`v-show`, not `v-if`** — drafts, `expandedThreads` and `expandedLists`
    must survive a tab switch, and the watcher supplies the fetch-on-select that `v-if` +
    `created()` would otherwise have given. Labels carry counts (`Discussion (12)` / `Lists (3)`,
    omitted at 0): with the other panel now hidden, the count is the only signal there is
    anything behind it.
  - **`EventLists`' root `v-if="lists.length || canManage"` must go** — correct for a side panel
    that should stay out of the way, wrong for a tab, where a guest on an event with no lists
    would select "Lists" and get a blank band. Always render; show "No lists yet."
  - Refresh icon (`mdi-refresh`) in the panel's title row emitting `refresh`; `toggleList`'s
    **expand** branch emits it too. Both land on `Event.vue`'s `refreshLists`.
  - **The tree renders from a flat list of precomputed rows, not a recursive component.** New pure
    `components/event/eventLists.js` (the `commentThreads.js` pattern): `flattenListItems(items,
    collapsedIds)` → `[{item, depth, hasChildren, collapsed}]` DFS, orphans at root, cycle-guarded;
    `canAddChild(depth)`; `checkStateLabel(item)`. The vitest env is node with no jsdom, so
    anything left inside the `.vue` is untestable — same constraint that shaped F5 and F14.
    Indent from a **static** class map `["", "tw-pl-6", "tw-pl-12"]`; Tailwind purges on literal
    source text, so a computed `` `tw-pl-${n}` `` would emit nothing.
  - Per-item: chevron when it has children (toggling `collapsedItemIds` — children default
    **expanded**, since the list header is already the collapse unit and subtrees are small); a
    "+" button only when `canAddChild(row.depth)`, opening ONE inline sub-composer at a time
    (`addingChildOf` + `childDraftText`, mirroring `editingItemId` — a second keyed draft map
    would repeat F14's reactivity trap for no gain); `add-item` now carries `{text, parentId}`.
  - Checklist rows: a clickable brass icon-checkbox (`mdi-checkbox-marked` /
    `mdi-checkbox-blank-outline` in a `v-icon`, the `EventPolls.vue:213-221` idiom) — there is no
    brass-themed `v-checkbox` in the app and a default Vuetify one looks wrong on leather.
    Emits `toggle-item-checked`. Footer line shows `authorName` plus `checkStateLabel(item)`.
    Third radio in the new-list form (`EventLists.vue:232-235`).
  - Tests: vitest for all three pure helpers (legacy flat items → all depth 0; DFS order;
    collapse hides descendants but keeps the row; orphan at root; cycle terminates; empty).
    **Then verify live** — F5, F6 and F14 each shipped or nearly shipped a defect that lint, tests
    and the build all passed: tab state surviving a switch, the network tab showing `/lists` and
    not `/events/:id`, a second session's check appearing after a refresh, 3 levels nesting and
    the 4th blocked, cascade delete, guest vs member rights, free text still accepted by
    `LocationInput` with no Google key, two viewports, both standing harnesses.

<details>
<summary>Original F13/F14 design (as planned 2026-07-29)</summary>

- **F13 · Lists of Lists backend: model, routes, permissions.** `M` · **P1** — no F-deps;
  independent of the mention track, and **scheduled ahead of it** (user's call, 2026-07-29).
  The planner creates a named list on an event ("Menu", "Bars to Visit"); anyone signed in adds
  items to it. Closest existing analogue is polls — a planner-created structure everyone else
  interacts with — so the shapes below are lifted from `routes/polls.go` and `routes/comments.go`
  rather than invented.
  - **Stored embedded on the Event doc, but written with targeted array operators — NOT the polls
    whole-array `$set`.** `routes/polls.go:143,186,255` each rewrite the entire `polls` array from
    a value read earlier in the request; two concurrent votes lose one. Polls get away with it
    because a handful of people vote occasionally. Lists invite *everyone* to append at once
    ("add your dish"), so every mutation here must be a single atomic update:
    `$push {"lists.$[l].items": item}`, `$pull {"lists.$[l].items": {"_id": itemId}}`,
    `$set {"lists.$[l].items.$[i].text": …}` with `arrayFilters`. A separate collection
    (comments' shape) is the alternative, rejected because lists are few and capped, and embedding
    means they ride `getEvent` with no attach step and no extra query.
  - `models/event.go` (next to `Polls []Poll` :325, structs by `Poll` :217-231):
    ```go
    type EventList struct {
        Id    primitive.ObjectID `json:"_id" bson:"_id"`
        Name  string             `json:"name" bson:"name"`
        Kind  string             `json:"kind" bson:"kind"` // "text" | "location"
        Items []EventListItem    `json:"items" bson:"items"`
    }
    type EventListItem struct {
        Id         primitive.ObjectID `json:"_id" bson:"_id"`
        Text       string             `json:"text" bson:"text"`
        UserId     primitive.ObjectID `json:"userId" bson:"userId"`
        AuthorName string             `json:"authorName" bson:"authorName"` // DisplayName() snapshot
        CreatedAt  primitive.DateTime `json:"createdAt" bson:"createdAt"`
    }
    ```
    plus `Lists []EventList` (`json:"lists" bson:"lists,omitempty"`). **Location items store the
    text string only** — no `placeId`, no lat/lng. That matches the event-location precedent
    exactly: `LocationInput.vue:103` already discards the `placeId` that `maps_utils.js` returns,
    and `EventLocation.vue:98-102` rebuilds the maps link from the text. Storing more would mean
    a new Place Details call and a second billing surface for no user-visible gain.
  - New `db/event_lists.go` (Mongo access stays in `db/`, per CLAUDE.md): one function per
    mutation, each a single `UpdateByID`.
  - New `routes/event_lists.go`, registered in the `authed` group of `InitEvents`
    (`routes/events.go:39-67`, alongside the polls routes at :65-67), Swag comment per handler:
    | Route | Who |
    |---|---|
    | `POST /:eventId/lists` `{name, kind}` | planner (`requireEventManager`, `routes/events.go:176`) or admin |
    | `PATCH /:eventId/lists/:listId` `{name}` | planner or admin (rename) |
    | `DELETE /:eventId/lists/:listId` | planner or admin; idempotent 200 if already gone |
    | `POST /:eventId/lists/:listId/items` `{text}` | **any signed-in user, guests included** |
    | `PUT /:eventId/lists/:listId/items/:itemId` `{text}` | **own item only** |
    | `DELETE /:eventId/lists/:listId/items/:itemId` | **own item, or member+**; idempotent 200 |
  - **`requireEventManager` alone is not the whole rule for lists.** It grants the event owner and
    (on legacy ownerless events) member+, but deliberately does *not* grant admins access to
    someone else's event. The confirmed decision is planner **+ admins**, so the list-level gate is
    `requireEventManager(...) || viewer.IsAdmin` — the same override `deleteComment` bolts on at
    `routes/comments.go:311-315`. Don't widen `requireEventManager` itself; polls and scheduling
    depend on its current meaning.
  - Permission logic goes in a pure, DB-free `listViewer` struct + `newListViewer(user, event)`
    (copy `commentViewer` / `newCommentViewer`, `routes/comments.go:50-80`) with
    `canManageList(viewer)` and `canDeleteItem(viewer, item)` as plain functions, so the whole
    matrix is unit-testable without a request. `loadListContext(c)` copies `loadCommentContext`
    (`comments.go:129-147`) — resolve authUser + event + viewer, write its own error response.
  - Identity comes from the session only: `UserId = user.Id`, `AuthorName = user.DisplayName()`
    snapshotted at write time, never read from the payload (`polls.go:245-248` is the precedent;
    E3 removed payload-supplied names).
  - Sanitize + caps as feature-local consts (polls precedent `polls.go:21-25`), all via
    `trimAndTruncate` (`routes/text.go:37`, rune-safe): list name ≤ 100 runes, item text ≤ 300
    (addresses run long), ≤ 20 lists per event, ≤ 100 items per list. Reject an unknown `kind`
    with 400 rather than defaulting — a typo'd kind would silently render as plain text.
  - Read path: lists ride the Event doc, so nothing to attach. Extend `eventDisplayNameIds`
    (`routes/display_names.go:55`) to collect item `UserId`s and add a `resolveListItemNames`
    alongside `resolveRsvpNames` (:114) / `resolvePollVoteNames` (:132), called from
    `resolveEventDisplayNames` (:152) — so a nickname change propagates to items the way F3 did
    for comments/RSVPs/votes. Stored `AuthorName` stays the fallback for deleted users.
  - Tests: pure matrices for the permission helpers (guest deletes another's item → deny; member →
    allow; owner renames → allow; member renames → deny; admin deletes another's list → allow) and
    for sanitize/caps/kind validation. DB-gated round-trip: create list → guest adds item → guest
    edits own → guest edits another's → 403 → member deletes guest's item → admin deletes the list
    → re-delete is still 200. Extend the `events_pii_db_test.go` matrix: an item exposes only
    `userId`/`authorName`, never email/phone/role. Swagger regen.
  - **Deployable alone** — endpoints are inert until F14 ships the UI.

- **F14 · Lists of Lists frontend: the panel beside the discussion.** `M` · **P1** — needs F13.
  - **Layout.** The user's ask is discussion ≈ 2/3 with lists ≈ 1/3 to its right. `Event.vue:85`
    already opens a `lg:tw-flex` row for the whole page, but it has a single child and the split
    wanted here is *within* the discussion band, so wrap the `EventComments` block (~:161-171) in a
    new `lg:tw-flex lg:tw-items-start lg:tw-gap-6` row: comments `lg:tw-w-2/3`, new `EventLists`
    `lg:tw-w-1/3`. Below `lg` (1024px, `tailwind.config.js:60-71`) they stack, lists after the
    discussion. Nothing above the discussion moves.
  - **New `components/event/EventLists.vue`**, modeled on `EventPolls.vue`: props `{event}` only,
    `...mapState(["authUser"])`, `...mapGetters(["canInvite"])` (member+ = the delete-anyone
    right), `isEventOwner` computed (`EventPolls.vue:174-181`), and **emit-only** — `create-list`,
    `rename-list`, `delete-list`, `add-item`, `edit-item`, `delete-item`; the parent owns
    persistence. Panel idiom copied verbatim from `EventPolls.vue:2-4`:
    `tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment
    sm:tw-p-4` with a `tw-mb-2 tw-text-base tw-font-medium` heading. Whole panel hidden when there
    are no lists and the viewer can't create one (`EventPolls.vue:3` pattern), so guests don't see
    an empty box.
  - **Add-list composer** (planner/admin only): name field + a text/location toggle, since the kind
    is fixed at creation.
  - **Add-item input**: dense `v-text-field` for `kind: "text"`; **`LocationInput.vue`** for
    `kind: "location"` — it's a `v-combobox`, so free text stays valid when there's no Google key
    (`isPlacesEnabled()` is just `!!VUE_APP_GOOGLE_MAPS_API_KEY`, `maps_utils.js:18`) and the
    feature must stay usable in that state. Props to pass: `dense`, `hide-details`,
    `placeholder`, `v-model`. It holds one Places session token per pick, so N instances on the
    page are fine, but only render the input for the list being added to — not one per list.
  - **Rendering**: location items render as `<a target="_blank" rel="noopener">` to a maps search
    URL. Extract `mapsSearchUrl(text)` into `utils/general_utils.js` and have
    `EventLocation.vue:98-102` use it too, rather than duplicating the template literal — pure
    helper, vitest-able in the node env (F5's `avatarUrl`/`monogram` precedent).
  - **Per-item controls**: pencil on own items (`item.userId === authUser._id`) opening the same
    input kind inline; trash on own items always, and on every item when `canInvite`. Show
    `authorName` per item so it's clear who added what.
  - **Wiring**: new `// --- Lists (F13/F14) ---` section in `EventService.js` (thin wrappers, polls
    section :70-88 is the shape); handlers in `Event.vue` follow the universal
    `await serviceFn(...); await this.refreshEvent()` + `showError` pattern (:550-576). The
    whole-event refetch picks up embedded lists for free — there is no realtime layer anywhere in
    this app, so two people adding items concurrently only see each other's on their next refetch.
    That's the same deal comments already have; don't add polling for it.
  - Tests: vitest for `mapsSearchUrl` plus any list logic worth extracting into a plain `.js`
    module (the vitest env is node — no jsdom, so nothing inside a `.vue` is testable). Verify
    live: planner creates a text list and a location list; a guest session
    (`server/tools/mintsession`) adds an item, edits and deletes its own, is refused on another's;
    a member deletes anyone's; maps links open; the location input still accepts free text with
    the Google key unset; both standing browser harnesses green.

</details>

---

## PART G — Carried forward from TODO.md (re-verified 2026-07-28)

- [ ] **G1 (was A22) · Small cleanup batch.** `S` · **P3** — corrections from the re-review:
  - Toggle→PATCH duplication is real but the paths were wrong: it's
    `schedule_overlap/WorkingHoursToggle.vue:76-88` + `schedule_overlap/BufferTimeSwitch.vue:65-77`
    (byte-identical modulo identifier) — extract one mixin. **`CalendarAccount.vue` does NOT
    belong in that mixin** (different endpoints — POST toggle-calendar/toggle-sub-calendar —
    and if/else semantics); it has its own internal 20-line duplication
    (`toggleSubCalendarAccount:199` vs `toggleCalendarAccount:220`) — fold separately.
    **Bonus found:** both toggles' `patch()` calls have no `.catch()` — a failed save is
    silent while the UI shows the new value; add error handling in the same extraction.
  - PostHog stub (`plugins/posthog.js`, no npm dep) + ~42 `$posthog` call sites in 20 files —
    remove both. Mixed `$posthog.` vs `$posthog?.` means removal must be all-or-nothing.
  - `errs/errors.go:10` TODO — error codes are bare strings; make them a type. When typing,
    **delete** `FriendRequestNotFound` + `UserNotFriends` (dead — see H2) rather than porting.
  - ~~EventNotFound double redirect (Event.vue/SignUp.vue → home)~~ — **CLOSED, resolved by
    E3**: the `publicRoutes` guard means `home` no longer re-bounces anyone; the only way to
    reach those catch blocks is already signed in. (Residue: stale `TODO(tony)` comment +
    "routeif" typo at `SignUp.vue:52`, cosmetic.)
  - Half-hour timezones: the TODO is at `ScheduleOverlap.vue:2317`. Sharpened finding: the
    half-hour case IS handled in the styling path (`:1845-1851` `isWeirdTimezone` →
    `timeOffset = -0.5`) but **not** in the specific-times computation — that asymmetry is the
    actual bug shape (India/Nepal/Newfoundland).

- [ ] **G2 (was A23) · Split the two remaining giants.** `L` · **P3** — updated numbers:
  `ScheduleOverlap.vue` 2,967 lines — the `computed` block (:1270-2265) is **996 lines**, now
  larger than `methods` (464); it's the single biggest extraction target left.
  `Event.vue` 983. `NewEvent.vue` 970 vs `NewSignUp.vue` 845 — **overlap confirmed heavy**
  (script sections differ by only ~164 lines; 13 of 15 methods/computeds are structurally
  identical, same names, same order) → a shared `newEventFormMixin` is the move.
  `date_utils.js` 1,119 lines / 46 exports — **but 9 exports have zero callers anywhere**
  (`getDateWithTime`, `dateFromObjectId`, `clampDateToTimeNum`, `getDaysInMonth`,
  `compareDateDay`, `isTimeNumBetweenDates`, `isDateInRange`, `isTimeWithinEventRange`,
  `getCurrentTimezone`) and 5 more are used only internally — delete/unexport first (free),
  which shrinks the real split surface to ~32 exports. Heed A11's caveat: verify splits with
  the app running, not blind.

- [ ] **G3 (was C8) · Web push.** `M` · **P3 — still deferred; reassess value first.**
  Premise unchanged (no service worker exists; reintroducing one reverses a deliberate
  removal; email reminders already cover iOS). **New sub-finding:** `kill-sw.js` is
  **not actually served** — it sits at the repo root, not `frontend/public/`, unreferenced by
  Caddyfile/deploy/compose, and its header comment targets the upstream's origin. Any client
  holding a stale pre-`f857320` SW never fetches it from this fork's domain. Either move it
  into `frontend/public/` (cheap, harmless) or mark it documentation-only. Also
  `frontend/.eslintrc.cjs:11`'s `serviceworker: true` env + comment are stale.

- [ ] **G4 (was D2) · Mongo DB name `schej-it` — rename is a data migration.** `L` · **P3**
  **Rewritten: this is now Mongo-only.** The GCP Cloud Tasks half of old D2 is gone — the
  entire `services/gcloud/` package was deleted in `49267959` (Listmonk drop); repo-wide grep
  confirms zero `cloudtasks`/`cloud.google.com` references outside `scripts/`. What remains:
  `db/init.go:44` `Db = Client.Database("schej-it")` + the dump/restore commands in docs.
  Renaming = `mongodump` old → `mongorestore` new name → cutover in a deploy window, human-run
  on the VM. Intentionally parked; zero user-facing benefit.

---

## PART H — New findings (2026-07-28 re-review)

None has a correctness or security symptom; all are cleanup/hygiene. Ranked by value:

- [x] **H1 · `NumEventsCreated`: dead aggregate on every profile fetch.** `S` · **P2**
  **DONE 2026-07-28** (`80a20905`). Sharper than written: the count's error path returned 500
  for the whole of `GET /user/profile`, so a transient Mongo failure on a dead field could
  bounce a user out on cold load. Writers were `events.go:329` + `event_import.go:230` (:229 is
  the comment). Also removed `signedIn := true` in `createEvent`, dead once its `$inc` went.
  `routes/user.go:65-70` runs `db.GetEventsCreatedThisMonth` on **every** `GET /user/profile`
  (incl. every cold-load router-guard call) and the frontend never reads `numEventsCreated`
  (zero grep hits). Paywall-quota leftover that survived E4. Also remove the two writers still
  `$inc`-ing it (`routes/events.go:329`, `routes/event_import.go:229`) and the model field.
  Same shape as A21's dead-marshal-on-hot-path finding.

- [ ] **H2 · Dead friend-request subsystem.** `S` · **P2**
  Deletable as one unit, zero callers: `db/utils.go:16` `GetFriendRequestById` + `:40`
  `DeleteFriendRequestById`, `models/friend_request.go`, `FriendRequestsCollection`
  (`db/init.go:19,48`), errs `FriendRequestNotFound`/`UserNotFriends` (`errs/errors.go:15-16`).
  Upstream social-graph cruft this fork never used (A2 already flagged the two db funcs as
  0-caller in July).

- [ ] **H3 · Dead exported server helpers.** `S` · **P2**
  Zero references outside their definitions: **all of `utils/db_utils.go`** (both funcs),
  `utils/utils.go` `PrintJson:27`, `EscapeRegExp:88`, `GetClientIdFromTokenOrigin:94`,
  `PrintHttpResponse:106`, `GetPrimaryAccountKey:191`, and `models/event.go:176`
  `RecurrenceLabel` (its comment claims email/log use; the callers went with Listmonk).
  Also unexport `IsRelease`/`GetDateString` (internal-only). A18-style batch.

- [x] **H4 · Clear the frontend lint warnings; flip eslint to fully blocking.** `S–M` · **P2**
  **DONE 2026-07-29.** The re-inventory confirmed the note below: `npm run lint` reported **54
  warnings / 0 errors** (not the 9 that `npx eslint src` shows, since eslint 8 skips `.vue`
  without `--ext`). All cleared, plus **H6 and H7 folded in** — both were literally items on
  the warning list. Notes:
  - **Rules are back at `error`** (inherited from `eslint:recommended` / `plugin:vue/essential`
    — the overrides are simply deleted, not restated), **and** `--max-warnings 0` is on the
    lint script so a *new* warning fails too. Verified with a canary: a stray unused const
    exits 1. `frontend-ci.yml`'s stale "~67 remain" comment is corrected.
  - **Three `vue/no-mutating-props` sites survive behind targeted `eslint-disable` comments
    that state why** — they are load-bearing, not sloppy. `OverflowGradient.vue` writes
    `scrollTop` on a raw `HTMLElement` prop (a false positive: DOM API, not Vue state).
    `CalendarAccount.vue:12` (`v-model="account.enabled"`) and `ScheduleOverlap.vue`'s
    `saveTempTimes` write through a shared object because **nothing refetches or emits** — the
    calendar toggle POSTs without refreshing `authUser`, so the write-through is what keeps the
    checkbox and the parent in sync. Fixing them properly is **G1** and **G2** respectively,
    not a lint pass. If either is picked up, delete the disable comment with it.
  - **A template `eslint-disable-next-line` disables the next *line*, not the next *element*.**
    In `CalendarAccount.vue` the violation is reported on the `v-model` attribute line, so a
    comment above `<v-checkbox>` silenced the wrong line and the warning survived. Use a
    `<!-- eslint-disable X -->` / `<!-- eslint-enable X -->` pair around the element instead.
  - `NewSignUp.vue`'s `EmailInput` import + registration were live only inside a commented-out
    "Email reminders" block; the block went with them rather than leave a comment referencing a
    deleted import. (`showEmailReminders`, `addedEmails` and `requestContactsAccess` are now
    orphaned Vue options — eslint can't see those, left for a G1-style sweep.)
  - Removing unused component registrations is safe here because the codebase has **no
    `<component :is>` anywhere** — checked before deleting. That's the one thing that would
    make a "registered but not used" warning a lie.

- [ ] **H5 · `CallApi` can't see a failed token refresh.** `S–M` · **P2**
  `services/services.go:19` calls `auth.RefreshUserTokenIfNecessary(user, nil)` which returns
  nothing — a failed refresh proceeds with the stale token and surfaces as an opaque upstream
  401. Adjacent to B8 (which fixed reporting *inside* the refresh; this caller is still
  blind). Second blind caller: `services/calendar/calendar.go:57`. Also `services.go:25`
  `json.Marshal` swallow — the function already returns `error`, propagating is a one-liner.

- [x] **H6 · `pluginMessagesMixin.js:165-171`: lying comment + abandoned validation.** `S` · **P3**
  **DONE 2026-07-29, with H4.** Subsumption confirmed: the range check really does happen
  downstream, but not the way the comment implied — it's the `coveredWidth < intWidth` test
  against the actual grid (`timeSlotToRowCol`), which is strictly better than the
  `eventDates`/`eventStartTime`/`eventDuration` approach the dead locals were reaching for.
  Deleted the four locals (incl. the never-pushed-to `convertedSlots`) and rewrote the comment
  to say what the loop does — parse + ordering checks — and where the range check lives.

- [x] **H7 · Unused `tenant` const trap.** `S` · **P3**
  **DONE 2026-07-29, with H4.** Interpolated rather than deleted: `${tenant}` now builds the
  URL it was always meant to build. Identical string today (`"common"`), but changing the
  const now takes effect instead of silently doing nothing, which is what made it a trap.

- [ ] **H8 · Verify `guestEvent` template arms against a real legacy ownerless event.** `S` ·
  **P3 — verify, don't delete.** The `guestEvent` prop threads through `NewEvent.vue`,
  `NewEventAdvancedOptions.vue`, `ToolRow.vue` and gates 5 template arms. Post-E3, new events
  always have owners, so it should only be reachable editing a **legacy** ownerless event — a
  documented keeper. Runtime-check before anyone assumes those arms are dead.

- [ ] **H9 · The cropper refuses a too-small photo without saying so.** `S` · **P3**
  *Found 2026-07-29 during the post-deploy browser pass on F6, not by lint/tests/build.*
  With a source image only a little larger than the 256x256 export (a 100x75 fixture
  reproduces it), cropperjs lays out a degenerate crop box, `getCroppedCanvas`
  (`AvatarEditorDialog.vue:209`) returns nothing, and the member gets **"That crop could not
  be saved. Try adjusting it."** (`:215`) — advice that cannot work, since no amount of
  dragging makes the source bigger. The refusal itself is right (better than uploading a
  smeared 256x256 upscale); only the diagnosis is wrong.
  Fix shape: measure the source **before** opening the dialog and reject with a message naming
  the real cause and the minimum, alongside the `image/*` and 10MB guards already in
  `onFileChosen` (`:158-165`). Note there is no cropper yet at that point — `getImageData()`
  is not available — so measure by loading the `FileReader` data URL into an `Image` and
  reading `naturalWidth/naturalHeight` before calling `openWith` (`:168` already hands it the
  data URL). A guard at pick time beats one at save time: it fails before the
  member has spent effort positioning a crop.
  Low priority because real photos are nowhere near the threshold — phone cameras start
  around 3000px. It costs someone scanning a small logo or a cropped screenshot, and the
  current wording sends them in circles.
  Note for whoever picks this up: `viewMode: 1` + `autoCropArea: 1` (`:191`,`:194`) are what
  make the box degenerate rather than merely small, so re-check the threshold empirically
  against those settings instead of assuming exactly 256.

---

## Suggested sequencing

1. **F1** (S, user-visible immediately) — then verify on the user's phone.
2. **F2 → F3** (nickname end-to-end), slotting **H1/H2/H3** between deploys as cheap wins.
3. **F4 → F5** (avatars end-to-end; F4 can start parallel to F3), then **H4** once the new
   frontend code is in (so the lint flip covers it).
4. ~~**F6** (admin editing — reuses everything above).~~ **Done 2026-07-29**, with H4 (and
   H6/H7) ahead of it.
5. ~~**F13 → F14** (Lists of Lists: backend → panel).~~ **Done 2026-07-29**, taken ahead of the
   mention track at the user's request. The two were independent, so nothing was rewritten.
6. **F7 → F8** (mentions: backend → emails). **Both done 2026-07-29.**
7. ~~**F15 → F16** (Lists v2: backend → tabbed band, tree, checklists).~~ **Done 2026-07-29**,
   taken ahead of F9 at the user's request for the same reason F13/F14 were: the two tracks share
   no code, so the order cost nothing. F16 landed as the three planned commits.
8. **F9** (the mention composer + rendering) **← next**, after which the feature track is
   complete.
9. **F10** close-out. Part G items remain background/P3, same as before; **H5** whenever
   calendar code is next touched; **H6–H9** opportunistic (**H9** is cheapest folded into the
   next change that touches `AvatarEditorDialog.vue`).

Workflow rules unchanged from `TODO.md`/`CLAUDE.md`: sync before changes, green commits to
trunk, deploys are human-run from the VM, cold-load signed-out testing after any router/auth
change (the E3 outage lesson).
