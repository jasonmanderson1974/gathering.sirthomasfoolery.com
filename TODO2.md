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
> `719cb4b8`, `0a9f450d`, `6c281faf`, F5 this commit) — see their entries for what differed from
> the plan. F11 and F12 were opened out of F5. Everything else is still planned-not-started. The
> feature designs in Part F were reviewed against the codebase on 2026-07-28 (file:line
> references verified that day) and the three product decisions in "Confirmed decisions" were
> made by the user. **Line numbers below shift as items land — re-verify before starting one.**

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

---

## PART F — Feature track (P1 unless noted)

Ten work items, each independently green-deployable to trunk (new fields `omitempty`, endpoints
additive, UI reads defensively). Suggested order: F1 first (quick win), then F2→F9 in order;
F4 can run in parallel with F2/F3. Cheap Part-H items slot between deploys.

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
  - `GET /users/:userId/avatar` in `routes/users.go` — unauthenticated (consistent with
    `getPublicUserProfile` exposing `Picture`): `Cache-Control: public, max-age=31536000,
    immutable`, `ETag` = updatedAt, `If-None-Match` → 304, 404 when none.
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

- [ ] **F6 · Admin+ edits a Member/Guest's name, nickname, photo.** `M` — needs F2, F4/F5.
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

- [ ] **F7 · @Mentions backend: parsing, storage, mentionables endpoint.** `M` — needs F2.
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

- [ ] **F8 · Mention notification emails.** `M` — needs F7.
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

- [ ] **F9 · Mention composer UI + rendering.** `L` — needs F7 (F8 independent of this).
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

- [ ] **F10 · Close-out sweep.** `S` — after F1–F9: final `swag init --parseDependency
  --parseInternal`, full backend suite (`go test $(go list ./... | grep -v '/scripts')` with
  `MONGODB_URI`), `npm run test:unit` + build + lint, both headless harnesses
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

- [ ] **H4 · Clear the frontend lint warnings; flip eslint to fully blocking.** `S–M` · **P2**
  **The count is wrong because the command is wrong.** `npx eslint src` under eslint 8 lints
  only `.js` — every `.vue` file is skipped. The real figure is what CI runs, `npm run lint`
  (`eslint . --ext .js,.vue`): **55 warnings / 0 errors**, not 9. (`frontend-ci.yml:37`'s
  "~67 remain" is stale in the other direction.) Re-inventory with `npm run lint` before
  starting; the effort is bigger than `S`. The original 9-warning `.js` list still stands as
  the subset below:
  `npx eslint src` → 9 warnings / 0 errors across 5 files (`pluginMessagesMixin.js:166-171,237`
  — see H6; `availabilityMixin.js:40` swallowed `err`; `respondentSelectionMixin.js:21`;
  `store/index.js:131`; `sign_in_utils.js:53` — see H7). Clear them and remove the
  warnings-tolerance, mirroring what B5 did for golangci.

- [ ] **H5 · `CallApi` can't see a failed token refresh.** `S–M` · **P2**
  `services/services.go:19` calls `auth.RefreshUserTokenIfNecessary(user, nil)` which returns
  nothing — a failed refresh proceeds with the stale token and surfaces as an opaque upstream
  401. Adjacent to B8 (which fixed reporting *inside* the refresh; this caller is still
  blind). Second blind caller: `services/calendar/calendar.go:57`. Also `services.go:25`
  `json.Marshal` swallow — the function already returns `error`, propagating is a one-liner.

- [ ] **H6 · `pluginMessagesMixin.js:165-171`: lying comment + abandoned validation.** `S` · **P3**
  Four dead locals under a comment announcing a date-range check that was never written; a
  later `isBrokenBounds` (:222+) may subsume it. Confirm subsumption, then delete the block
  (also clears 5 of H4's warnings).

- [ ] **H7 · Unused `tenant` const trap.** `S` · **P3**
  `sign_in_utils.js:53` declares `const tenant = "common"` but `:69` hardcodes `/common/` in
  the Outlook OAuth URL — changing the variable does nothing. Delete or interpolate.

- [ ] **H8 · Verify `guestEvent` template arms against a real legacy ownerless event.** `S` ·
  **P3 — verify, don't delete.** The `guestEvent` prop threads through `NewEvent.vue`,
  `NewEventAdvancedOptions.vue`, `ToolRow.vue` and gates 5 template arms. Post-E3, new events
  always have owners, so it should only be reachable editing a **legacy** ownerless event — a
  documented keeper. Runtime-check before anyone assumes those arms are dead.

---

## Suggested sequencing

1. **F1** (S, user-visible immediately) — then verify on the user's phone.
2. **F2 → F3** (nickname end-to-end), slotting **H1/H2/H3** between deploys as cheap wins.
3. **F4 → F5** (avatars end-to-end; F4 can start parallel to F3), then **H4** once the new
   frontend code is in (so the lint flip covers it).
4. **F6** (admin editing — reuses everything above).
5. **F7 → F8 → F9** (mentions: backend → emails → composer).
6. **F10** close-out. Part G items remain background/P3, same as before; **H5** whenever
   calendar code is next touched; **H6–H8** opportunistic.

Workflow rules unchanged from `TODO.md`/`CLAUDE.md`: sync before changes, green commits to
trunk, deploys are human-run from the VM, cold-load signed-out testing after any router/auth
change (the E3 outage lesson).
