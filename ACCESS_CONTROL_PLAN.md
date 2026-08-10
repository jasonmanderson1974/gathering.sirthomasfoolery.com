# The Fellowship — Invite-Only Access Control Plan

> **⚠️ HISTORICAL — the plan shipped; §5's last entry is 2026-07-21. Not a live plan.**
> §1 is still the best record of *why* access control works the way it does, and the
> four-tier role model it describes is what `models/roles.go` implements. But the
> mechanics below are pre-migration: commands like `docker compose up -d --build server`
> refer to the deleted Docker production stack, and "the dev box has no Go/Docker" is no
> longer true of either machine. For anything operational read `DEPLOYMENT.md` and
> `DEVELOPMENT.md`; `CLAUDE.md` is authoritative on the codebase.

Multi-day, phased plan to make the self-hosted instance **invite-only** (no open sign-ups).
Companion to `REDESIGN_PLAN.md`. Memory: `project-fellowship-access-control`.

> Resume tip: read **§1 Decisions** and **§5 Status** first. The OTP login flow already
> exists in the codebase — most of this is *wiring + gating*, not building from scratch.

---

## 1. Decisions (finalized 2026-06-20 with user)

- **Scale:** ~30–40 users total (≈12 men + wives). Many — especially wives — likely have **no
  Google account**, so the universal path must not require Google.
- **Primary auth = passwordless EMAIL CODE (OTP), gated by an allowlist.**
  Enter email → server checks the **`allowlist`** → on it: email a 6-digit code → verify → in.
  Not on it: show *"The Fellowship is by invitation only"*, **no account created, no code sent**.
- **Google OAuth = REMOVED as a LOGIN path (2026-07-21).** Login is now **email-OTP-only** — the
  Google/Outlook buttons were stripped from the sign-in surfaces (`SignIn.vue`, `SignInDialog.vue`, and
  the pure-login link in `groups/NotSignedIn.vue`). Google OAuth is **kept only for calendar autofill /
  contacts import** (Event availability, Settings → calendar accounts, create-event/group flows) — a
  post-login feature, still gated by the same allowlist. Rationale: user plans to go live with the
  Google consent screen open (allowlist is the real gate), and OTP is the sole desired auth path.
  NOTE: the autofill feature still uses the Google consent screen, so connecting a Google calendar
  still hits Google's test-user/verification limits — but that's opt-in and does not affect login.
- **Email transport = plain Gmail account via SMTP + app password** (`smtp.gmail.com`). Low volume
  (login codes for ~40 people) is well within Gmail's ~500/day limit. "From" = the gmail address.
- ~~**Guest (no-login) event responses: LEFT OPEN.** Members share Gathering links internally; we are
  gating *account sign-in*, not link responses.~~
  **REVERSED 2026-07-28 (TODO E3).** Every event route now requires a session; anonymous visitors
  see only the landing page, sign-in/auth, and the privacy policy. The reversal was driven by what
  "left open" turned out to mean in practice: because responses are keyed by display name for
  guests and by user-id for members *in the same map*, an unauthenticated caller could overwrite a
  member's response, delete any response by name, rename respondents, RSVP and vote as anyone, and
  read named availability through `?guestName=` — bypassing blind availability. Requiring sign-in
  closed all of it at once.
  Two things deliberately survive:
  - **On-behalf entry**, so a member can still fill in availability for a spouse without an account
    ("+ Add guest availability"). The server rejects ObjectID-shaped names, which is what makes it
    safe to leave open.
  - **The ICS feed** (`GET /api/events/:id/ics`), the one unauthenticated route: calendar apps poll
    it without cookies, it exposes only the confirmed gathering's name/time/venue, and obtaining
    the URL requires signing in.
  Legacy name-keyed data still renders read-only; it can no longer be created or impersonated.
- **Member management:**
  - **`canInvite` role** on users — designated members (the user + any they pick) can add emails to
    the allowlist ("invite"). Regular members cannot.
  - **Self-service contact info** — Settings page gains **email + phone** editing (name already there).
  - **Email change auto-adds to the allowlist** so a member who changes their email keeps access.
    Subtlety (accepted): this lets a member allowlist any address by setting it as their own — fine
    for a trusted ~40-person club; the explicit **Invite** action is the intended way to add others.
- **Branching:** build on a feature branch (off `redesign/fellowship-phase1`, the current deployed
  branch; or off `main` once the redesign merges). Backend = Go ⇒ compiles on the VM only
  (dev box has no Go/Docker) — watch the Docker Hub `golang` pull rate limit (see REDESIGN_PLAN §5).

## 2. What already exists (don't rebuild)

- **OTP login flow — frontend:** `frontend/src/views/SignIn.vue` (enter email → check-email →
  onboarding name for new users → send code → verify). Already themed.
- **OTP login flow — backend:** `server/routes/auth.go` handlers `checkEmail` (`/auth/otp/check-email`),
  `sendOtp` (`/auth/otp/send`), `verifyOtp` (`/auth/otp/verify`); `generateOtpCode()`;
  `models.OtpCode`; `db.OtpCodesCollection`.
- **Google OAuth:** working (`signInHelper` in `auth.go`; CLIENT_ID/SECRET configured). Calendar autofill.
- **Gap to fix:** `sendOtp` currently sends the code via **Listmonk**
  (`listmonk.SendEmailAddSubscriberIfNotExist`, `LISTMONK_OTP_EMAIL_TEMPLATE_ID`) — NOT configured on
  this instance. Swap to Gmail SMTP. (`server/utils/mail_utils.go` exists — check before adding net/smtp.)

## 3. Data model

- **`allowlist` collection** (new): `{ email (lowercased, unique), addedBy, addedAt }`. The gate.
- **`User`** (`server/models/user.go`): add **`canInvite bool`**; ensure email + phone are user-editable.
- Matching is by **email, lowercased/trimmed**, exact. (Google's verified email must equal the
  allowlisted email — the key cross-check; see §6 gotchas.)

## 4. Build phases

### Phase A — Allowlist + the gate  *(DONE 2026-06-20, deployed `5853a06`, verified live)*
- [x] `allowlist` collection (unique email index, `db/init.go`) + `db/allowlist.go` accessors
      (IsAllowlisted, **IsAccessAllowed** [fail-open when empty], Add/Remove/GetAllowlist).
- [x] `errs.NotInvited` + sentinel `errs.ErrNotInvited`.
- [x] Gate `checkEmail` → `{invited:false}` (200) when blocked; `sendOtp` → 403; OAuth
      `signInHelper` returns `ErrNotInvited` before account creation; `signIn`/`signInMobile`
      callers return 403 `not-invited`.
- [x] `SignIn.vue` shows "by invitation only" on the email path + via `?notInvited=1` query;
      `Auth.vue` bounces a not-invited Google login to `/sign-in?notInvited=1`.
- [x] VERIFIED live: empty list ⇒ fail-open (no lockout); add 1 email ⇒ enforces (listed=invited,
      others=invited:false); cleared ⇒ dormant. **Allowlist is currently EMPTY (dormant).**

### Phase B — Gmail SMTP for the codes  *(DONE 2026-07-21, code complete; awaits VM env + deploy)*
- [x] Replaced the Listmonk send in `sendOtp` with **`utils.SendEmail`** (existing Gmail SMTP helper,
      `smtp.gmail.com:587` via gomail). **Reused the existing env vars** `GMAIL_APP_PASSWORD` +
      `SCHEJ_EMAIL_ADDRESS` instead of inventing new `SMTP_*` names — the helper already existed.
- [x] Fellowship-themed HTML code email (`buildOtpEmailBody` in `routes/auth.go`): dark leather/brass,
      inline styles, 10-min-expiry copy. Subject: "NNNNNN is your Fellowship sign-in code".
- [x] Dropped `strconv` + `LISTMONK_OTP_EMAIL_TEMPLATE_ID` from the OTP path; `.env.template` updated
      (GMAIL vars now marked *required for email OTP auth*; Listmonk OTP template marked no-longer-used).
- [ ] **Manual (user, on VM):** set `GMAIL_APP_PASSWORD` + `SCHEJ_EMAIL_ADDRESS` in `server/.env`, then
      `docker compose up -d --build server`. Send a test code to confirm delivery (check spam).

### Phase C — Inviter role + admin UI  *(DONE 2026-07-21, code complete; awaits VM deploy + bootstrap)*
- [x] `User.canInvite *bool` (`models/user.go`) — returned by `/user/profile`, so `authUser.canInvite`
      drives UI. `db.SetUserCanInvite(email, bool)` accessor (`db/users.go`, case-insensitive).
- [x] `middleware.CanInviteRequired()` (chained after `AuthRequired`) → 403 `errs.NotAuthorized`.
- [x] `routes/admin.go` (`InitAdmin`, group `/api/admin`, both middlewares): `GET /allowlist`
      (enriched w/ hasAccount + name + canInvite), `POST /allowlist` (invite, validates email),
      `DELETE /allowlist` (remove; blocks self via `errs.CannotRemoveSelf`),
      `POST /member/can-invite` (promote/demote; blocks self-demote; 404 if no account yet).
      New errs: `NotAuthorized`, `InvalidEmail`, `CannotRemoveSelf`. Registered in `main.go`.
- [x] Frontend: **`views/MemberAdmin.vue`** ("The Roll", Fellowship-themed) at route `/members`
      (name `admin`, added to router auth-guard list); invite form + roll list w/ Member/Invited badge,
      inviter toggle, strike button. "Members" link in `AuthUserMenu.vue` shown only when
      `authUser.canInvite`. Client guard redirects non-inviters home (endpoints enforce server-side).
- [ ] **Manual (user, on VM after deploy):** bootstrap the first admin —
      `docker compose exec mongo mongosh schej-it --eval 'db.users.updateOne({email:"jason@jasonmanderson.com"},{$set:{canInvite:true}})'`
      then reload the site; the "Members" menu item appears. Promote others via the UI thereafter.

### Phase D — Self-service contact info  *(DONE 2026-07-22, deployed `b3dd960`)*
- [x] Settings page: edit **phone** (direct, `PATCH /user/phone`, `User.phone` field) + **email**.
- [x] **Email change is OTP-VERIFIED** (user chose this over direct, given per-request enforcement):
      `POST /user/email/request-change` (validate + conflict-check + rate-limit + code to NEW address) →
      `POST /user/email/verify-change` (confirm code, then update `user.email`).
- [x] On email change: **REPLACE** semantics — add new email to allowlist at the user's role FIRST
      (so per-request access keeps passing), then remove the old email (no dangling invite at the old
      address). New errs `email-unchanged`, `email-taken`.

### Phase E — Seed  *(DROPPED 2026-07-22 — not needed)*
- Decided not to build a seed script; members are added ad-hoc via **The Roll** (`/members`) or
  directly in Mongo. **Access control is COMPLETE** through Phase D.

## 5. Status / progress  (update each session)

- 2026-06-20: Design finalized + documented.
- 2026-06-20: **Phase A DONE & deployed** (`5853a06`) — allowlist + sign-in gate, fail-open while
  empty, verified live. Allowlist currently EMPTY ⇒ gate dormant, site behaves as before.
- 2026-07-21: **Phase B code DONE** — OTP codes now sent via `utils.SendEmail` (Gmail SMTP), themed
  HTML email, Listmonk OTP dependency removed. Reused `GMAIL_APP_PASSWORD`/`SCHEJ_EMAIL_ADDRESS`.
  Awaits: user sets those two vars in `server/.env` on the VM + rebuild/deploy + delivery test.
- 2026-07-21: **Login made email-OTP-only** (Google/Outlook sign-in buttons removed; Google kept for
  calendar autofill only). Commit `88772a1`, deployed.
- 2026-07-21: **Phase C code DONE** — `canInvite` role + `/api/admin` allowlist endpoints +
  `CanInviteRequired` middleware + "The Roll" admin page (`/members`) + gated menu link. Deployed;
  jason@ bootstrapped. **SUPERSEDED same day by the 4-tier role model below.**
- 2026-07-21: **4-TIER ROLE MODEL (Phase C v2) code DONE.** Replaced the binary `canInvite` flag with
  `User.role` (superAdmin/admin/member/guest — `models/roles.go`, empty⇒member). Decisions: **Admins
  can manage other Admins** (promote/demote/remove); **Super Admin is DB-only + immutable in app**
  (never grantable/removable via UI). Capabilities: create events = all except guest; invite guests =
  member+; invite members/admins + manage roles + strike = admin+; super admin untouchable by anyone
  via API. Allowlist entry now carries `role` (seeds new user's role at first sign-in; existing users
  keep their role). Endpoints: `GET/POST/DELETE /admin/allowlist` + `POST /admin/member/role`
  (renamed from /can-invite); guards block self-change, super-admin mods, and out-of-tier grants.
  Guest event-creation blocked in `createEvent` + UI (store `createNew` guard + hidden create buttons).
  Frontend: role constants/getters (`isGuest`,`canInvite`,`canManageUsers`,`canCreateEvents`), reworked
  "The Roll" (role badges + per-member role dropdown + adaptive member/admin view). **Awaits VM deploy
  + re-bootstrap jason@ to `role:"superAdmin"` (canInvite field now obsolete).**
- 2026-07-21: **4-tier role model deployed** (`9ccc6e7`); jason@ re-bootstrapped `role:"superAdmin"`.
- 2026-07-21: **SECURITY REVIEW + HARDENING deployed** (`e2d0b08` → `a21a020` → `ad0decf` → `7dca053`;
  branch HEAD `7dca053`, VM in sync). Full review of the access-control work → 15 findings, tracked in
  a **Todoist** parent "Timeful/Fellowship access-control — code review findings". **11/15 fixed:**
  - P1: `GET /admin/allowlist` gated to admin (roster-read authz); guest event-creation bypass via
    `duplicateEvent`/`importEvent` closed.
  - P2: OTP **rate limiting** (`utils.RateLimiter` — send 5/email/15m, check-email 60/IP/min);
    **session revocation** (`AuthRequired` re-checks `IsAccessAllowed` every request ⇒ allowlist now
    enforced per-request); **cookie hardening** (HttpOnly/SameSite=Lax/Secure-in-release/MaxAge=30d).
  - Tests: `models/roles_test.go` + `routes/admin_test.go` (run in `golang:1.25-alpine` container;
    dev box has no Go).
  - P3: admin 500s → `errs.Internal`; email normalization → `utils.NormalizeEmail`; `getAllowlist`
    batch query (no N+1). `checkEmail` isNewUser enumeration CLOSED (accepted; rate-limit-mitigated).
  - P4 (all fixed, `c64c82d`): verifyOtp re-checks allowlist (TOCTOU); `INVITE_ONLY_ENFORCED` flag
    (empty allowlist fails closed; set true in VM `server/.env`); setMemberRole writes account role
    first; unknown roles → 400 `invalid-role` (`models.IsKnownRole`); `canCreateEvents` no longer hides
    the create button from anon (matches backend).
  - **ALL 15 REVIEW FINDINGS CLOSED.** Security: invite-only enforced per-request + fail-closed; OTP
    rate-limited; cookies hardened; roles validated + unit-tested.
- 2026-07-22: **Phase D DONE & deployed** (`b3dd960`) — self-service Settings email/phone editing;
  email change is OTP-verified and replaces the allowlist entry (add-new-then-remove-old).
- **Next = Phase E** (seed script for the initial ~40 emails/admins) — the last access-control phase.

## 6. Needs-from-user / manual steps

- [ ] **Gmail app password:** on a plain @gmail.com account, enable 2-Step Verification, then create an
      **App Password** (Google Account → Security → App passwords). Add `SMTP_USER`(the gmail address) +
      `SMTP_PASS`(the 16-char app password) + `SMTP_FROM` to `server/.env` on the VM (like CLIENT_SECRET —
      never paste the password in chat). I'll give exact var names when Phase B lands.
- [ ] **Initial allowlist:** the ~30–40 emails (email required; name optional).
- [ ] **Initial admins:** which emails get `canInvite` (the user + anyone else).

## 7. Gotchas / decisions to revisit

- **Email-match risk:** the allowlisted email must equal the email the person actually signs in with
  (their Google email, or the exact address they enter for the code). If someone's Google email differs
  from what we have, they use the email-code path with their allowlisted address.
- **Gmail SMTP:** needs 2FA + app password; "less secure apps" is gone. Watch the daily send cap (~500;
  fine here). Deliverability from a plain gmail is usually OK at this volume but codes could land in spam
  — tell members to check spam on first login.
- **Backend deploys:** Go changes compile on the VM; if `docker compose up --build` hits the Docker Hub
  `golang` 429 pull limit, build only what changed / `docker login` on the VM (see REDESIGN_PLAN §5).
- Old Listmonk OTP env/template references can be removed once Gmail SMTP is in.
