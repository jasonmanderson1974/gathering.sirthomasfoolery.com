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
> **Status: FEATURE TRACK COMPLETE (2026-07-29); PART H CLOSED (2026-07-30).** F1–F16 have all
> landed, along with **F11** (RSVP half; poll voters closed as won't-do), **F12**, **G1** in
> full and **G2's** dead-export prune on 2026-07-29 — and **H5**, **H8** and **H9** on
> 2026-07-30, which empties Part H. Verified green on the H pass: swag not re-run (no route
> comments changed), full backend suite + `go vet` + golangci-lint (0 issues), frontend lint +
> **188** unit tests + production build, and both headless harnesses 5/5 **against a rebuilt
> dev stack** — see the note under "Workflow rules" about why that rebuild matters.
>
> **What remains is only what was deliberately set aside:** **G2's** actual splits, and **G3**
> and **G4** (deferred and parked by the user on 2026-07-29). Nothing is queued.
>
> **Worth reading if you are picking up an old finding:** two of the three H items closed on
> 2026-07-30 were described *wrongly* by the entries that recorded them, and neither error was
> visible from the code — H5's real symptom was a JSON decode error, not the 401 it predicted,
> and H9 had its defect exactly inverted (there was no guard, not a badly-worded one). Both
> were caught by reproducing the finding before fixing it. History below.
>
> F1, H1, F2, F3, F4 and F5 landed 2026-07-28 (`0b5f2272`, `80a20905`,
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
> for a whole-event refetch. **F9 landed 2026-07-29** (`b9633400`), closing the mention track:
> tokens are now written by a picker and rendered as names rather than markup.
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
UI reads defensively). **Everything here is done**, F11 and F12 included as of 2026-07-29 —
F11 scoped to its RSVP half, with the poll-voter half closed as won't-do. See their entries.
The sequencing that got there is at the bottom.

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

- [x] **F10 · Close-out sweep.** `S` — after the feature track (F1–F9 + F13/F14): final
  `swag init --parseDependency --parseInternal`, full backend suite
  (`go test $(go list ./... | grep -v '/scripts')` with `MONGODB_URI`),
  `npm run test:unit` + build + lint, both headless harnesses
  (`check-signed-out.js` / `check-signed-in.js`), tick items off here.
  **DONE 2026-07-29.** Everything green, nothing to fix:
  - `swag init --parseDependency --parseInternal` regenerated `docs/` with **no diff** — the
    route comments and the committed spec were already in step (F9 added no endpoint).
  - Backend against the dev stack's Mongo: `go build`, `go vet`, `golangci-lint` (**0 issues**)
    and `go test -count=1`, all over `go list ./... | grep -v '/scripts'`. `-count=1` on
    purpose: a cached "ok" from a run without `MONGODB_URI` means the DB-gated tests *skipped*,
    which is not what a close-out wants to record.
  - Frontend: `npm run lint` (`--max-warnings 0`), 167 unit tests across 16 files, production
    build.
  - Both harnesses: `check:signed-out` 5/5, `check:signed-in` 5/5 against `localhost:3002`.
  - Seeded harness documents (3 users, 3 allowlist rows, the event and its comments) deleted
    afterwards, per DEVELOPMENT.md.

- [x] **F11 · Avatars on the RSVP roster.** `M` · **P3**
  **DONE 2026-07-29** (`2d8678d6`). **Scoped to the RSVP half on the user's call** — the
  poll-voter half is closed as won't-do, not deferred (see below). The RSVP half built as
  planned:
  - `Rsvp.User` mirrors `Comment.Author`, including **`bson:"-"`, which is load-bearing here**
    and was not in the plan: the RSVP write path `$set`s the whole `rsvps` map from the
    in-memory struct, so an untagged field would be persisted and then go stale. Pinned by its
    own test (`TestRsvpUserIsNeverPersisted`).
  - No PII change — `slimUserForDisplay` clears the email unconditionally, so the attached
    account carries none regardless of the `collectEmails` gate.
  - The monogram-from-name fallback for legacy name-keyed rows was already inline in
    `CommentRow`; it is now `userFromDisplayName` in `general_utils` and shared by both.
  - Verified in a browser against a seeded fixture, not just unit tests: account row renders
    avatar + "Harness Check (+1)", guest row falls back to a monogram with no `<img>`.
    (At 22px `UserAvatarContent` renders ONE initial by design, not two.)
  - **Poll votes: won't-do.** `Votes map[string]string` stores a bare display name per voter
    key, so an avatar there needs a stored-shape change (`{name, user}`) that drags in the write
    path and legacy rows — not worth it for a voter roster. Reopen only if the vote shape
    changes for some other reason.

- [x] **F12 · `getEvent` nil-derefs on an event response with no `response` field.** `S` · **P3**
  **DONE 2026-07-29** (`1b1ad6d3`). Two corrections to the plan:
  - **Guarded at the source, not at the call site.** The plan said one
    `if response == nil { continue }` beside the deleted-user branch. `getResponsesMap` and
    `ConvertEventToOldFormat` skip the nil row instead, so the map never contains one — two
    lines total, and the invariant holds for any future caller.
  - **A second caller had the identical bug**, unmentioned in the plan: `getResponses`
    (`routes/event_responses.go:62`) dereferences `response.Availability` in the same way. The
    source guard covers it; a call-site guard would not have.
  - The DB-gated test inserts the bad row as **raw bson**, because the model's `response` tag
    carries no `omitempty` — marshalling the struct writes an explicit null rather than omitting
    the key. Confirmed it panics with the guard reverted.

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

- [x] **G1 (was A22) · Small cleanup batch.** `S` · **P3**
  **DONE 2026-07-29** (`f1a98d67`). All five items done. What differed:
  - The toggle mixin is a **factory** (`calendarOptionSync(optionName, errorLabel)`), because
    the option's name is needed three times — the prop, the PATCH body key, and the `update:`
    event. `CalendarAccount.vue` stayed out of it as planned and got its own local helper.
  - **PostHog removal stranded more than the call sites.** App.vue's `setFeatureFlags` survived
    only to write `featureFlagsLoaded`, state with **no readers anywhere**, from a watcher
    gating on `this.$posthog` being truthy — which the stub always was. The whole chain went.
    `signUpFormEnabled` is kept: `NewDialog` still reads it (and, as before, nothing sets it).
    **Superseded 2026-07-30** — the flag and the whole sign-up-sheet feature were removed
    (`cc002d1`). See the note below.
    Two methods that existed only to report analytics (`trackTimezoneChange`,
    `trackExportCsvClick`) went with their template bindings.
  - **The half-hour timezone symptom was worse than written.** The finding called it an
    asymmetry in the specific-times computation. It is: a specific-times event matches each cell
    against stored instants *by timestamp*, and the grid was shifted 30 minutes off them — so a
    viewer in Kolkata/Kathmandu/Newfoundland got an **entirely empty grid**, not a mislabelled
    one. Stored instants win; those viewers now correctly see the event's real half-hour local
    times. Decision extracted to `gridTimeOffset.js` + tests, following the
    gridGeometry/responseCounts pattern; the flag is scoped so the ordinary grid is untouched by
    construction, which one test pins.
  - Error codes are now `errs.Code`. Underlying type stays `string`, so **the JSON on the wire
    is unchanged**; the two sentinels that wrap a code in `errors.New` need `string()` now.

  Original notes, for reference:
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

- [ ] **G2 (was A23) · Split the two remaining giants.** `L` · **P3**
  **FREE PART DONE 2026-07-29** (`9831d303`); **`newEventFormMixin` DONE 2026-07-30;
  the `ScheduleOverlap` computed block DONE 2026-07-30.**
  The nine zero-caller exports were re-verified repo-wide and deleted — and deleting them
  **stranded two more** (`splitTime`, `getDateWithTimeNum`), whose only callers were among the
  nine, so eleven went rather than nine. Three of the internal-only five are unexported
  (`getDateRangeString`, `processTimeBlocks`, `stdTimezoneOffset`). `date_utils.js` is now
  **946 lines / 32 exports** (from 1,119 / 46) — the predicted ~32 surface, so the numbers below
  for the split still stand. **Still to do:** only the `date_utils` split itself, which wants the
  app running, per A11's caveat.

  **What the computed pass did.** `ScheduleOverlap.vue` **2,713 → 1,960**; its `computed` block
  **929 → 180 lines**. Two new mixins, plus computed blocks added to four that were methods-only
  — the point being that each moved computed landed next to the methods it already served, so no
  mixin is a grab-bag:
  - `calendarDaysMixin` (279) — the **day axis**: `allDays`/`days`/`monthDays`, `daysOfWeek`,
    `dayOffset`, `monthDayIncluded`, `curMonthText`, the four pagination computeds, `columnOffsets`.
  - `timeGridMixin` (183) — the **time axis**: `splitTimes` (131 lines by itself) and `times`,
    `timeslotDuration`/`timeslotHeight`, `timezoneOffset`, `timezoneReferenceDate`.
  - `timeslotStylingMixin` += the six per-cell fan-out maps (`timeslotClassStyle`,
    `dayTimeslotClassStyle`, `timeslot`/`dayTimeslotCounts`, `timeslot`/`dayTimeslotVon`) — they
    do nothing but call that mixin's own per-cell methods once per cell.
  - `availabilityMixin` += the read side of responses: `parsedResponses`, `respondents`, `max`,
    `userHasResponded`.
  - `respondentSelectionMixin` += `curRespondentsSet`, `curRespondentsMax`, `selectedGuestRespondent`.
  - `currentAvailabilityMixin` += `availabilityArray`, `ifNeededArray`, `calendarEventsByDay`,
    `overlaidAvailability`.

  Eight now-unused imports left the component. **No behaviour change was intended and none was
  made** — every one of the 63 function-bodied computeds is byte-identical to its original, checked
  mechanically, and the 65-name set is unchanged with nothing duplicated (a duplicate would mean a
  mixin/component shadow). Verified beyond that with a **before/after runtime snapshot**:
  `verify_g2_overlap.js` dumps all 67 merged computeds (65 + the two Vuex spreads expanding to four
  keys) plus a DOM fingerprint for four fixture gatherings — paged/overnight/days-only/weekly — in
  five UI states each (initial, next page, back, and two timezones), 16 snapshots per run. The
  extracted build reproduces the pre-extraction build **byte for byte**, and the snapshot is stable
  across repeated runs on one build. Note when re-running it: the dev **server registers its static
  routes at boot**, so a dist swap needs a `restart server`, and confirm which build is actually
  live via `$options.mixins.length` (6 = before, 8 = after) — the bundle hash is not a reliable
  discriminator and quietly cost one bogus "identical" result.

  **What the mixin pass found.** NewEvent 937→585, NewSignUp 776→417 (761 lines deleted for 50
  added), against `src/mixins/newEventForm.js` and a pure `src/components/newEventDates.js` (122)
  + 18 tests. **NewSignUp was deleted outright later the same day** (see the sign-up note below),
  so the mixin now has one consumer and its factory generality is gone (`154cc3b`); NewEvent sits
  at 531 lines, which is what the split was for. It is a factory like `calendarOptionSync`, taking a *function* of default
  overrides so `startOnMonday` can read `localStorage` per instance; it seeds `data()` and
  `reset()` from that one source. Three field lists drive what used to be hand-written parallel
  code: `contactsFields` (the OAuth round-trip), `trackedFields` (the unsaved-changes check),
  and the defaults themselves. Components override the two lists to extend them.

  **The duplication was hiding three real bugs**, all fixed by folding the copies together:
  - A dates-only **sign-up sheet could never be saved**: its branch set `type` from
    `eventTypes.SIGNUP`, which does not exist, so the field went out as `undefined` and the API
    (`type` is `binding:"required"`) rejected it. Reachable — editing a sign-up sheet is live
    even though *creating* one is not.
  - The sign-up copy of the day-of-week loop **omitted the June-2018 DST correction** the event
    copy has, so a recurring sheet made in the opposite half of the year was stored an hour out.
    Pinned now by two fake-clock tests (winter/summer), and the winter one fails without it.
  - `resetToEventData()` called `this.$refs.emailInput.reset()` unguarded, so closing the edit
    dialog on an **ownerless gathering** — where that section isn't rendered — threw. Now `?.`.

  **Two deliberate behaviour changes**, both improvements, worth knowing if something looks off:
  `timeIncrement` was snapshotted but never compared, so changing only the time increment and
  closing discarded it with no prompt; it is now in `trackedFields`. And the snapshot copies
  arrays instead of storing them by reference. Also deleted `contactsAccessGranted` from both —
  dead in each, and it wrote to `curScheduledEvent`/`confirmDetailsDialog`, which are
  ScheduleOverlap's state and exist in neither component.

  Verified: 206 unit tests, eslint clean, production build, and two headless harnesses against a
  rebuilt dev stack — `verify_g2_neweventform.js` 11/11 (both create paths, the payloads, the
  unsaved-changes prompt in both directions, reset-on-reopen) and `verify_g2_newsignup.js` 6/6
  (the sign-up edit path).

  Original notes, for reference:
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

### Sign-up sheets REMOVED (2026-07-30, `cc002d1`) — not a backlog item, recorded here

Switched on and then removed the same day, both at the user's request. The middle step is only
worth knowing because the reasoning is easy to get wrong twice:

- **A "sign up" here was a sign-up _sheet_** — an event with `isSignUpForm: true`, carrying named
  blocks (name + time range + capacity) that people claimed slots in, with a waitlist past
  capacity. **It was never account registration**, which is the allowlist/OTP gate in
  `ACCESS_CONTROL_PLAN.md` and is untouched. The names invite exactly the wrong inference — it was
  drawn twice in one session. If something here reads like "signup", check which one it means.
- **It was half-dead when found:** `signUpFormEnabled` was initialised `false` and never set (its
  mutation was PostHog-driven and lost its caller in G1), so the tab never rendered and no sheet
  could be created through the UI — while the *edit* path stayed live. Switching it on
  (`5ae6901`) surfaced a folder bug and made the dates-only sheet reachable, which is the path
  whose `type` came from the non-existent `eventTypes.SIGNUP` (fixed in `dc1d133`).
- **Then it was cut entirely**, on the user's decision that the club won't use slot sheets and the
  feature isn't returning. ~1,335 lines: the components, the `/s/:signUpId` route, the model
  types, `assignSignUpBlocks`, the db helpers and their tests. Prod held **zero** events with
  `isSignUpForm`, so nothing was migrated; a legacy document carrying the field is ignored by the
  model and renders as an ordinary gathering (verified against the one on the dev stack).
- **Three things fell out as dead once it went:** NewDialog's whole tab mechanism, the `eventOnly`
  flag that existed only to suppress those tabs, and `HelpDialog.vue`, whose only job was
  explaining the difference between the two tabs. `newEventForm`'s factory generality went with
  it (`154cc3b`) — one consumer left, so the parameterisation described nothing.

- [ ] **G3 (was C8) · Web push.** `M` · **P3 — still deferred; reassess value first.**
  **Reconfirmed deferred 2026-07-29** — offered to the user alongside G1/G2 and not taken, so
  even the cheap `kill-sw.js` housekeeping below is untouched.
  Premise unchanged (no service worker exists; reintroducing one reverses a deliberate
  removal; email reminders already cover iOS). **New sub-finding:** `kill-sw.js` is
  **not actually served** — it sits at the repo root, not `frontend/public/`, unreferenced by
  Caddyfile/deploy/compose, and its header comment targets the upstream's origin. Any client
  holding a stale pre-`f857320` SW never fetches it from this fork's domain. Either move it
  into `frontend/public/` (cheap, harmless) or mark it documentation-only. Also
  `frontend/.eslintrc.cjs:11`'s `serviceworker: true` env + comment are stale.

- [ ] **G4 (was D2) · Mongo DB name `schej-it` — rename is a data migration.** `L` · **P3**
  **Reconfirmed parked 2026-07-29** — put to the user with G1/G2 and left parked, no runbook
  written. Unchanged below.
  **Rewritten: this is now Mongo-only.** The GCP Cloud Tasks half of old D2 is gone — the
  entire `services/gcloud/` package was deleted in `49267959` (Listmonk drop); repo-wide grep
  confirms zero `cloudtasks`/`cloud.google.com` references outside `scripts/`. What remains:
  `db/init.go:44` `Db = Client.Database("schej-it")` + the dump/restore commands in docs.
  Renaming = `mongodump` old → `mongorestore` new name → cutover in a deploy window, human-run
  on the VM. Intentionally parked; zero user-facing benefit.

---

## PART H — New findings (2026-07-28 re-review)

None has a correctness or security symptom; all are cleanup/hygiene. Ranked by value:

> **All of Part H is closed as of 2026-07-30.** H5, H8 and H9 went in one pass. Two of the
> three turned out to be described wrongly in ways only a runtime check could show — H5's
> symptom was a JSON decode error rather than the 401 it predicted, and H9 had the defect
> exactly inverted. Both entries record the corrected version.

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

- [x] **H2 · Dead friend-request subsystem.** `S` · **P2**
  **DONE 2026-07-29** (`6ef72fd`, batched with H3). Went as written, with two notes: the errs
  entries below were **already gone** — the G1 errs reshape took them, so only the model, the
  two db funcs and the collection var remained. And the `friendrequests` **collection is left
  in Mongo**: removing the accessor doesn't drop stored data, and dropping it is a separate
  deliberate call nobody has made. Verified no frontend references either (the error codes were
  never part of the client contract).
  Deletable as one unit, zero callers: `db/utils.go:16` `GetFriendRequestById` + `:40`
  `DeleteFriendRequestById`, `models/friend_request.go`, `FriendRequestsCollection`
  (`db/init.go:19,48`), errs `FriendRequestNotFound`/`UserNotFriends` (`errs/errors.go:15-16`).
  Upstream social-graph cruft this fork never used (A2 already flagged the two db funcs as
  0-caller in July).

- [x] **H3 · Dead exported server helpers.** `S` · **P2**
  **DONE 2026-07-29** (`6ef72fd`, batched with H2). All confirmed still 0-caller before
  deleting. One nuance worth recording: **`PrintJson` was not strictly callerless** — it had
  three references, all inside `scripts/20240721_apple_calendar_test`, which is excluded from
  the build and deliberately not kept compiling (`server/scripts/README.md`). Deleted anyway on
  that basis, but it needed a deliberate call rather than a silent one. `IsRelease`/
  `GetDateString` were unexported, not deleted, as planned. Removing the five helpers left five
  now-unused imports in `utils.go` (`bytes`, `encoding/json`, `io`, `net/http`, `regexp`) —
  trimmed with them. 152 deletions against 4 insertions; no behaviour change.
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

- [x] **H5 · `CallApi` can't see a failed token refresh.** `S–M` · **P2**
  **DONE 2026-07-30** (`3aa2478`). Built as planned — `RefreshUserTokenIfNecessary` now returns
  `map[string]error` keyed by calendar account key — with four notes:
  - **The symptom was worse than recorded.** The plan said a failed refresh "surfaces as an
    opaque upstream 401". Reverting the fix under test showed what the member actually got:
    `json: cannot unmarshal string into Go struct field Response.error of type
    errs.GoogleAPIError` — Google's 401 body doesn't fit the struct that parses it, so even the
    401 was lost. Same shape as the B8 bug one layer down, and the reason the regression test
    asserts on the *reason text* rather than merely on "an error".
  - **`CallApi` matches the failure by POINTER IDENTITY**, because an `OAuth2CalendarAuth`
    carries no account key and no email — there is nothing else to match on. Both real callers
    (`contacts.SearchContacts`, `microsoftgraph.GetUserInfo`) hand over the very pointer stored
    on `user.CalendarAccounts`, which is also what lets the refresh write a new token through it.
    A copied auth deliberately does not match; there is a test for that.
  - **Persist only when a token actually changed.** The old `numAccountsToUpdate > 0` counted
    *attempts*, so an all-failed round wrote the user document back unchanged. Fixing that is
    also what keeps the failure path off Mongo entirely, which is why the new tests are pure
    rather than DB-gated.
  - `GetUsersCalendarEvents` puts the reason in the account's slot in the map — **the same slot
    the 401 filled**, so `Event.vue`'s `calendarPermissionGranted` / refetch logic sees exactly
    what it saw before — and skips the doomed round trip.
  - The `json.Marshal` swallow went too: it was sending an empty body upstream and making the
    result look like the provider's fault.
  - Tests: 5 pure in `services/auth` (failure keyed by account, unexpired not reported, the
    `accounts` filter honoured, `asRefreshError` preserving a real error vs wrapping a panic
    value) + 5 in `services` (the skip, the non-skip, per-account isolation, identity-not-value,
    the empty map) + 1 in `services/calendar` asserting the calendar URL is never requested.
    Both new packages needed a `TestMain` calling `logger.Init(io.Discard)` — they are the first
    tests there to reach a log line, and `logger.StdErr` is nil until `main.go` runs.

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

- [x] **H8 · Verify `guestEvent` template arms against a real legacy ownerless event.** `S` ·
  **P3 — verify, don't delete.** **VERIFIED 2026-07-30 — keeper, no code change.** Driven in
  headless Chrome against the dev stack, as a signed-in member who is **not** the owner:
  - **Both legacy shapes reach it.** `isOwnerlessEvent` accepts `ownerId ==
    "000000000000000000000000"` *and* a missing `ownerId`; seeded one of each and `guestEvent`
    computed `true` for both. A third event owned by a different member computed `false`, so the
    arms are gated rather than always-on — which is the half that would have made a "still
    reachable" result meaningless.
  - **The arms render.** On a legacy event the edit dialog shows the "created before sign-in was
    required" `AlertText`, and the "Email me each time someone joins" checkbox and the "Email
    reminders" section are correctly absent. On the owned event: no alert, both present.
  - **`ToolRow`'s arm needed a response to exercise.** `showScheduleEventButton` also requires
    `numResponses > 0`, and the server recomputes `numResponses` from the `eventResponses`
    collection — seeding the field on the event document does nothing. With a real response
    inserted, the confirm/schedule button appears on the legacy event (via `guestEvent`) and not
    on the one owned by someone else (`isOwner` false). That is the arm actually doing work: it
    is what lets a member confirm a gathering nobody owns.
  - Not exercised, and dead for a *different* reason: the two `v-else-if="!guestEvent"`
    signed-out variants (`NewEvent.vue:240`, `NewEventAdvancedOptions.vue:33,73`) need
    `!authUser`, which E3 made unreachable on this page. Removing those is a G1-style sweep, not
    this item — and they are not `guestEvent`'s doing.
  - Seeded documents (2 users, 2 allowlist rows, 3 events, 3 responses) deleted afterwards.

- [x] **H9 · The cropper refuses a too-small photo without saying so.** `S` · **P3**
  **DONE 2026-07-30** (`d99d18e`). The fix shape below was right; **the diagnosis was
  backwards, and taking its own advice — re-check the threshold empirically — is what showed
  it.** Recorded in full because the corrected finding is worse than the original:
  - **There was no refusal.** `getCroppedCanvas` returned a canvas for *every* source driven
    through the real dialog: 1x1, 8x8, 32x24, 100x75 (the fixture this item named), 150x150,
    2000x10 and 10x2000 strips, 256x256, 4000x3000 — and also when Save was clicked on the
    first frame it was enabled, testing the one path that genuinely can return null (cropperjs
    v1 returns null only while its own async `ready` flag is false, and
    `AvatarEditorDialog` clears `loading` synchronously after the constructor). Every one of
    those uploaded and was stored.
  - **So the item had it inverted**: it was not a right refusal with wrong wording, it was no
    guard at all, and the "smeared 256x256 upscale" it credited the refusal with preventing was
    what actually got saved. The strips are the worst case — a square crop is bounded by the
    **shortest** side, so a 2000x10 image contributed a 1–2px selection blown up to 256x256.
  - **The threshold is the export size, and the shortest side is what it applies to.** New pure
    `avatarSourceError(width, height)` in `general_utils.js` (F5's pattern — the vitest env is
    node with no jsdom, so nothing testable can live in the `.vue`), rejecting under
    `AVATAR_EXPORT_PX = 256`. It returns a *message*, not a boolean, so it can name the actual
    size: naming it is the whole point of the item.
  - Measured at pick time from an `Image`'s `naturalWidth/naturalHeight`, as planned — there is
    no cropper yet, so no `getImageData()` to ask.
  - The save-time branch stays as a backstop, **reworded**: "Try adjusting it" named an action
    that cannot help. It is now believed unreachable, but it is two lines and the alternative is
    a bare `return`.
  - `MemberAdmin`'s admin-on-behalf upload reuses the dialog, so it is covered by the same guard
    with no change.
  - Verified in headless Chrome against a **rebuilt** dev stack (per the note at the bottom of
    this file): 100x75 and 2000x10 are refused by name and size and the dialog never opens;
    256x256 (the boundary) and 4000x3000 crop and save as before. 7 vitest cases pin the helper.

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
8. ~~**F9** (the mention composer + rendering).~~ **Done 2026-07-29** (`b9633400`) — the
   feature track is complete.
9. ~~**F10** close-out.~~ **Done 2026-07-29.**

**What's left, and nothing here is urgent:** ~~**F11** and **F12**~~ and ~~**G1**~~ are done
(2026-07-29), as is **G2's** free half, and ~~**H2**/**H3**~~ went as one deletion batch the
same day (`6ef72fd`). ~~**H5**, **H8** and **H9**~~ closed 2026-07-30, which empties **Part H**
and leaves nothing anywhere with a known failure mode.

**Everything still open is P3 and was already deliberately set aside:** **G2's** two remaining
splits (`ScheduleOverlap.vue`'s computed block and `date_utils.js` — its `newEventFormMixin`
went in 2026-07-30, and found three live bugs on the way), and **G3**/**G4**, which the user
deferred and parked respectively on 2026-07-29. Nothing is queued. Deploy is the human's call from the VM-adjacent box —
`origin/main` is ahead of what's live until then.

Workflow rules unchanged from `TODO.md`/`CLAUDE.md`: sync before changes, green commits to
trunk, deploys are human-run from the VM, cold-load signed-out testing after any router/auth
change (the E3 outage lesson).

**Rebuild the dev containers before trusting a harness run.** `compose.dev.yaml` bakes the
frontend bundle into the frontend image and the Go binary into the server image; `docker
compose restart` re-runs the *old* artifacts. During the F11/G1 close-out the harnesses first
passed 5/5 against a stack that predated every change in them — the give-away was `rsvp.user`
missing from the API while the nickname re-resolution beside it worked. `docker compose -f
compose.dev.yaml up -d --build frontend server` (the server also re-registers its static routes
only at startup, so it needs the restart to see new hashed filenames).
