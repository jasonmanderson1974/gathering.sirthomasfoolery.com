# Timeful / The Fellowship — Improvement & Feature Backlog

> Compiled 2026-07-22 from a full-codebase review. Context that shaped priorities:
> this is a **self-hosted, invite-only fork** for a **~30–40 person club** (≈12 men + wives).
> That means: **reliability, maintainability, and small-club utility** matter far more
> than horizontal scale, multi-tenant concerns, or growth/monetization features.
> Companion docs: `REDESIGN_PLAN.md`, `ACCESS_CONTROL_PLAN.md`.
>
> **2026-07-27: second full review** added items **A16–A23**, **B4–B5**, **E3–E10** — headlined by
> **E3** (require sign-in for ALL event access; remove the anonymous guest flow), which reverses the
> "guest responses left open" decision and subsumes a batch of IDOR findings.
>
> **2026-07-28:** cleared the cheap, E3-independent security batch — **B4** (CI was skipping 3 test
> files), **E5** (`session.Save()` errors), **E6** (phone/role/RSVP-email/remindee leaks in
> `getEvent`) — then the **deletion sweep**: **E4** (Stripe/paywall), **E7** (slackbot +
> discord_bot), **A18** (dead code), **A19** (unused npm deps). About **2,500 lines removed**, two
> unauthenticated surfaces gone. Then **E8**+**E11** (payload validation) and **E3 — now complete,
> all five phases**, which is the invite-only posture this fork was always meant to have.
> **Read E3's outage note before touching the router guard**: phase 3 took the site down for
> signed-out visitors, and the lesson (cold-load, no-cookie testing) is now a harness.

Priority legend: **P0** = do first (correctness / risk / cheap-and-high-value) ·
**P1** = high value · **P2** = moderate · **P3** = nice-to-have.
Effort: **S** ≈ <½ day · **M** ≈ 1–2 days · **L** ≈ 3+ days.

---

## PART A — Refactoring & Code-Health

### P0 — Correctness & risk (do first)

- [x] **A1 · Standardize `c.Bind` failure responses.** `S` — **DONE 2026-07-22 (events.go).**
  Correction to the original finding: the `events.go` sites use `c.Bind` (not `c.ShouldBind`), and
  Gin's `c.Bind` **already** calls `AbortWithError(400)` internally — so these returned a **400 with
  an empty body**, not a silent 200. (A true 200-on-bad-input would only occur at `ShouldBind`
  sites, which already return proper JSON.) So A1 was a *consistency* issue, not a silent-success
  bug. Fixed all **11** `events.go` bind handlers to return `c.JSON(http.StatusBadRequest,
  responses.Error{Error: err.Error()})` (removing the lone `fmt.Println` debug print and the
  bare/`c.Status`-only variants). **Follow-up:** the other route files' bind handlers already use
  either `ShouldBindJSON` with a JSON error body or `BindJSON` (also auto-400) — spot-check for
  consistency but no silent-200 bug found.

- [x] **A2 · Stop panicking inside request handlers on DB errors.** `M` — **FULLY DONE 2026-07-22
  (route handlers + `db/` + `services/`).** Only intentional fail-fast panics remain (`db/init.go`
  startup, `auth.go generateOtpCode` crypto/rand — see below). Converted every handler `Panicln` to
  `logger.StdErr.Println(err)` +
  `c.JSON(500, responses.Error{Error: errs.Internal})` + `return`: `events.go` (12),
  `user.go` (16), `auth.go` (5). `signInHelper` (a helper returning `(models.User, error)`)
  propagates the error instead — `return models.User{}, err`. Two handler `Panicln`s that were
  actually *bind* errors (`toggleCalendar`, `toggleSubCalendar`) now return 400. The
  `importEvent` response loop logs + `continue`s (event already inserted). `admin.go`,
  `folders.go`, `stripe.go`, `users.go` had none.
  **Intentionally left:** `auth.go generateOtpCode()` — a `crypto/rand` failure (not a DB error),
  in a helper returning `string` with no context; converting needs a signature change and the
  failure is astronomically rare.
  **`db/` + `services/` — partially DONE 2026-07-22 (the safe, error-returning subset):**
  Converted the panics in functions that *already return an `error`* (or error-like) so the error
  now flows through the return value instead of panicking — no signature change, no caller change:
  `db/folders.go CreateFolder` (the `return …, err` after it was dead code), `services/calendar/
  google_calendar.go` `GetCalendarList`/`GetCalendarEvents` (4 sites), `services/contacts/
  contacts.go SearchContacts` (2 sites — returns `*errs.GoogleAPIError{Code: 500}` so the caller's
  `c.JSON(googleError.Code, …)` stays valid). This also fixes a **latent goroutine crash**: the
  calendar async wrappers `recover()` with `err.(error)`, but `log.Panicln` panics with a *string*,
  so that assertion would itself panic and take down the process — now moot, since these no longer
  panic.

  **Deliberately NOT refactored (assessment, not laziness):** the ~40 remaining `Panicln`s in
  value-returning/void `db/` getters (`GetUserById`, `GetEventById`, …) and `services/`
  (`GetTokensFromAuthCode`, `RefreshAccessToken`, `CallApi`, `GetUserInfo`, `CreateEmailTask`).
  Reasons: (1) these getters already `return nil`/empty on *not-found* (callers handle it) and only
  panic on an *unexpected* DB error; (2) **every** such path is already contained — request handlers
  by `gin.Recovery()` (→ 500, the correct status), and all db/service-calling **goroutines** by
  their own `defer recover()` (verified: `events.go` 1005/1053/1422, calendar + auth async wrappers).
  So there is no crash bug left to fix. A full signature refactor would touch **80+ call sites**
  (`GetUserById` alone has 20) with **no local Go compiler** to verify — high risk, low benefit.
  If desired later, do it **incrementally, one function at a time, gated by Backend CI** — not as a
  single sweep. `db/init.go` panics are startup/fail-fast and should stay.

  **Incremental refactor — batch 1 DONE 2026-07-22 (CI-green):** de-panicked the lowest-caller
  getters — `db/utils.go` (`GetDailyUserLogByDate`→`(_, error)`, `UpdateDailyUserLog`,
  `GetFriendRequestById`/`DeleteFriendRequestById` — the last two have 0 callers, i.e. **dead code**,
  candidates for deletion), `GetEventsCreatedThisMonth`→`(int, error)`, `GetUserByStripeCustomerId`
  →`(*User, error)`. **Remaining tiers (heads-up — entangled/high-caller):** the event getters
  (`GetEventById`, `GetEventByShortId`, `GetEventByEitherId`, `GenerateShortEventId`) are a **single
  cluster** — `GetEventByEitherId` (11 callers) calls the other two, so they must move together
  (~17 call sites). `GetUserById` (20 callers) and `GetUserByEmail` (8) are the other big ones, plus
  `GetEventResponses`/`GetAttendees` (slices) and the `services/` functions (`CallApi`,
  `GetTokensFromAuthCode`, `RefreshAccessToken`, `GetUserInfo`, `CreateEmailTask`). These are the
  high-effort/low-benefit end (all already recovered → 500); decide whether they're worth it.

  **Batch 2 DONE 2026-07-22 (CI-green):** the event-getter cluster —
  `GetEventById`/`GetEventByShortId`/`GetEventByEitherId` → `(*Event, error)`, ~17 call sites
  updated (11 handlers → 500; `main.go` + db-internal callers keep nil-checks). `GenerateShortEventId`
  kept its `string` signature (handles the error internally).

  **Batch 3 DONE 2026-07-22:** the user-getter cluster — `GetUserById` → `(*User, error)` (20 callers)
  and `GetUserByEmail` → `(*User, error)` (8 callers) in `db/users.go`. Call-site handling by context:
  request handlers → 500 (`middleware/auth.go`, `routes/users.go` ×2, `routes/user.go` ×3,
  `routes/auth.go` ×3, `routes/events.go` handlers); `signInHelper` propagates
  `return models.User{}, err`; async goroutines (email-send blocks in `updateEventResponse`) and
  counted loops (`getResponses` populate loops, calendar-fetch loop) log + `continue`/`return` so they
  degrade gracefully rather than aborting; pure helpers that fall back safely ignore the error
  (`db/events.go isNameBlocked`, `routes/admin.go effectiveTargetRole`/invite email-check,
  `shouldKeepGroupResponseUserEmails` → treats a fetch error as "not a member"); `routes/stripe.go`
  fulfillment helper logs + returns.

  **Batch 4 DONE 2026-07-22 (final — A2 complete):** the slice getters + `services/` tail.
  - Slice getters in `db/events.go`: `GetEventResponses` → `([]EventResponse, error)` (8 callers) and
    `GetAttendees` → `([]Attendee, error)` (3 callers). All `routes/events.go` handler callers → 500;
    the `shouldKeepGroupResponseUserEmails` helper ignores the error (safe empty-slice fallback).
  - `services/services.go CallApi` → `(*http.Response, error)` (also fixed a latent nil-`req` deref by
    checking the previously-ignored `http.NewRequest` error). Callers propagate: outlook
    `GetCalendarList`/`GetCalendarEvents` (already `…, error`), contacts `SearchContacts` (2 sites →
    `*errs.GoogleAPIError{Code:500}`), and `microsoftgraph.GetUserInfo` → `(UserInfo, error)`.
  - `services/auth/auth.go`: `GetTokensFromAuthCode` → `(TokenResponse, error)` (3 handler callers in
    `user.go`/`auth.go` → 500; the OAuth-error branch now returns an error instead of panicking on the
    marshaled body) and `RefreshAccessToken` → `(AccessTokenResponse, error)` (its only caller,
    `RefreshAccessTokenAsync`, feeds the error into the existing `RefreshAccessTokenData.Error` channel
    field). `microsoftgraph.GetUserInfo` callers: `user.go` handler → 500, `auth.go signInHelper` →
    `return models.User{}, err`.
  - `services/gcloud/tasks.go CreateEmailTask` kept its `[]string` signature (no caller changes):
    reminder-email scheduling is a best-effort side effect of event create/edit, so a failure must not
    500 the event op — env-var/template-id parse errors log + `return []string{}`, and per-task
    marshal/CreateTask errors log + `continue` (partial scheduling still succeeds).
  - **Deliberately left panicking:** `db/init.go:39` (Mongo connect at startup — fail-fast) and
    `auth.go generateOtpCode` crypto/rand (astronomically rare, helper returns bare `string`).

  **Not verified locally (no Go toolchain on this machine) — Backend CI is the gate.**

- [x] **A3 · Unchecked writes in loops.** `S` — **DONE 2026-07-22 (the 3 listed sites).**
  `createEvent` now builds an `[]interface{}` and uses a single `InsertMany` with an error check
  (returns 500 on failure — it runs before the event is inserted, so no partial event). The
  `editEvent` added-attendees insert and the `updateEventResponse` new-response insert now capture
  and log the error (the latter only increments `NumResponses` on success). **Follow-up:** this is a
  subset of a broader pattern — many `UpdateOne`/`UpdateByID` calls across the routes ignore their
  error too (e.g. `updateEventResponse:947`); worth a dedicated unchecked-write sweep.

- [x] **A4 · Remove duplicate `refreshAuthUser` store action.** `S` — **DONE 2026-07-22.**
  Deleted the second (shadowing) definition in `frontend/src/store/index.js`; the original at the
  top of `actions` remains.

### P1 — Structural debt that slows every future change

- [x] **A5 · Break up `ScheduleOverlap.vue` (was 4,638 lines, ~99 methods).** `L` — **DONE 2026-07-22
  (checkbox flipped 2026-07-23; the body's final bullet already recorded "A5 is now DONE" after the
  Tier-2 child split — the box was just never ticked). Component 4,638 → ~3.1k lines; remaining size is
  inherent grid complexity.**
  This is the single largest maintenance liability in the repo — a god-component mixing drag-select
  grid math, availability animation, calendar-account plumbing, sign-up-block editing, and
  respondent hover/selection.
  - **Step 1 DONE:** grid geometry + drag lifecycle (`normalizeXY`, `clampRow`, `clampCol`,
    `getRowColFromXY`, `endDrag`, `inDragRange`, `moveDrag`, `startDrag`) →
    `schedule_overlap/dragGridMixin.js`. 4,638 → 4,303.
  - **Step 2 DONE:** "Aggregate user availability" (`fetchResponses`, `getResponsesFormatted`,
    `getRespondentsForHoursOffset`, `showAvailability`) → `availabilityMixin.js`. 4,303 → 4,125.
  - **Step 3 DONE:** "Current user availability" incl. the animate cluster (`refreshAuthUser`,
    `resetCurUserAvailability`, `populateUserAvailability`, `getIsTimeBlockInFirstSplit`,
    `getTimeBlockStyle`, `getAvailabilityFromCalendarEvents`, `setAvailabilityAutomatically`,
    `animateAvailability`, `stopAvailabilityAnim`) → `currentAvailabilityMixin.js`. 4,125 → 3,744.
  - All three are verbatim Vue 2 mixin moves (behavior-preserving: methods run on the same instance
    `this`; template bindings and cross-`this.*` calls resolve unchanged). Verified per step via
    `npm build`, **eslint `no-undef`** (the real gate — it caught a `dayjs` free-reference in step 3
    that `npm build` bundled silently), and unit tests (23/23).
  - **Steps 4–6 DONE 2026-07-22 (Tier 1 slices, runtime-verified via headless Chromium against the
    local stack):** respondent hover/selection → `respondentSelectionMixin.js`; the whole Timeslot
    region (484 lines: sizing, class/style maps, von handlers, valid-time-ranges) →
    `timeslotStylingMixin.js`; Options-panel handlers → `optionsMixin.js`. Component now **3,166**
    lines (was 4,638 pre-A5). Verified in-browser: grid renders, respondent hover/click switches
    single/subset availability views, best-times toggle re-renders + persists.
  - **Sign-up-block child split DONE 2026-07-22 (Tier 2):** the per-day grid overlay (dragged block +
    saved blocks + blocks-to-add) → `SignUpBlocksOverlay.vue` (presentational; state stays in the
    parent since dragGridMixin shares it; parent handles `block-click`). Runtime-verified end-to-end
    in headless Chromium: created a sign-up event, dragged a slot out (dragged branch renders live),
    saved, and as guest clicked the block → Join-slot dialog. **A5 is now DONE** — remaining
    ScheduleOverlap size (~3.1k lines) is inherent grid complexity; further splits optional.
    B3 (grid-math tests) can extract the pure bits of the geometry logic for real coverage.

- [x] **A6 · Split `server/routes/events.go` (1,925 lines).** `M` — **DONE 2026-07-22 (CI-green,
  in 3 incremental commits).** Pure reorg, no behavior change; `InitEvents` (route registration)
  stays in `events.go`. All handlers/helpers moved within `package routes`, so cross-file references
  resolve without changes. Final layout:
  - `events.go` (946 lines) — CRUD/read: `InitEvents`, `createEvent`, `editEvent`, `getEventIds`,
    `getEvent`, `deleteEvent`, `duplicateEvent`, `archiveEvent`.
  - `event_responses.go` (837) — `getResponses`, `updateEventResponse`, `deleteEventResponse`,
    `renameUser`, `userResponded`, `declineInvite` + helpers `findResponse`,
    `shouldKeepGroupResponseUserEmails`, `stripSensitiveUserFields`, `getResponsesMap`.
  - `event_import.go` (226) — `importEvent`.
  - `event_calendar.go` (150) — `getCalendarAvailabilities`.
  Each file's import block was hand-curated (no local Go — verified per-commit by Backend CI, since
  Go errors on unused imports). No route/comment content changed, so Swagger `docs/` need no regen.
  **Now testable in isolation → unblocks B1.**

- [x] **A7 · Consolidate date libraries (drop `moment`, ideally `spacetime`).** `S` — **DONE
  2026-07-22.** Both removed from package.json + lockfile (`npm ci` verified). By removal time
  **neither** had any import left in the frontend — moment was always dead, and spacetime's single
  TimezoneSelector usage had already been rewritten to dayjs (only a stale comment remained; fixed).
  dayjs is now the sole date lib. Browser-verified: timezone selector switches Pacific → Eastern,
  grid re-renders clean.

- [x] **A8 · Add linting to CI (nothing lints today).** `S` — **DONE 2026-07-22 (warnings-first).**
  All lint steps use `continue-on-error: true`, so findings surface in the CI log without blocking
  merges on the existing backlog. **Backend** (`backend-ci.yml`): added `go vet` (scoped
  `go vet $(go list ./... | grep -v '/scripts')` so it skips the non-compiling migration scripts) and
  `golangci-lint` via `golangci/golangci-lint-action@v6` **pinned to v1.61.0** (v2 changed the config
  schema) with a v1-format `server/.golangci.yml` (default linter set; `skip-dirs: [scripts]`).
  **Frontend** (`frontend-ci.yml`): added `eslint@^8.57` + `eslint-plugin-vue@^9.27` as devDeps
  (lockfile regenerated; `npm ci` verified green), a `.eslintrc.cjs` (Vue 2 preset
  `plugin:vue/essential` + `eslint:recommended`; `vue/multi-word-component-names` off since view
  components are intentionally single-word; noisiest rules set to `warn`), `.eslintignore`, a `lint`
  npm script, and a non-blocking `Lint` CI step. Baseline: **102 problems (41 errors, 61 warnings)** —
  that's the backlog to work down before flipping the steps to blocking.
  - **Backlog pass 2026-07-22 (later same day):** frontend eslint **errors 34 → 0** (all 34 fixed,
    incl. a real DatePicker `!= NaN` bug and an in-place Vuex sort in Dashboard.orderedFolders;
    screens browser-verified) → **frontend `Lint` step now BLOCKING** (fails on errors; ~67 warnings
    remain and still pass — mostly `no-unused-vars`, `vue/no-unused-components`, 6
    `vue/no-mutating-props` that need real design fixes). **`go vet` now BLOCKING** (clean; also
    fixed a broken `microsoftgraph_test.go` signature it caught). **golangci-lint was silently dead**
    — the pinned v1.61.0 binary (go1.23) refused to load the Go 1.25 module and continue-on-error
    hid it; upgraded to v2.12.2 with a migrated v2 config + package-list scripts exclusion, which
    surfaced the real backend backlog: **112 issues (98 errcheck, 11 staticcheck, 2 ineffassign,
    1 govet)** — stays warnings-first until worked down. Note: staticcheck SA1019 flags the CFB
    encryption in `utils/utils.go` as deprecated — do NOT swap ciphers casually; stored data is
    encrypted with it (needs a migration plan).

### P2 — Cleanup & smaller components

- [x] **A9 · Delete dead code.** `S` — **DONE 2026-07-22 (CI-green).**
  - ✅ `server/main.go` `splitPath()` — removed (recursive helper, no external callers).
  - ✅ `createEvent` commented-out "add owner to group by default" block — removed (referenced the
    long-gone `event.Responses` field).
  - ✅ `pricingPageConversion` A/B state — removed (write-only Vuex state; its only mutation caller was
    already commented out and the value is never read; also dropped the mutation + mapMutations reg).
  - **Left intentionally:** `isPremiumUser` is NOT dead — the store getter is still wired via
    `mapGetters` in `ScheduleOverlap.vue`, `Event.vue`, `ToolRow.vue`. Removing it needs confirming
    those template/logic uses are truly inert, which can't be verified without running the app; folded
    into the [A11]/paywall-cleanup consideration rather than removed blind.

- [x] **A10 · Normalize `fetch_utils.js` error handling.** `S` — **CORE DONE 2026-07-22 (CI-green);
  timeout/interceptor deferred.** Fixed the inconsistent style (the stray semicolons/indentation from
  line 60) and **standardized the error shape** — which also fixed a live regression: the Aug-2025
  "better debug logs" change had rewritten `throw returnValue` into a wrapped Error exposing only
  `.parsed`, silently breaking the `err.error` contract 6 call sites still use (`switch (err.error)`,
  `err.error?.code`, `err.error === …`), while 2 sites had migrated to `err.parsed?.error`. The
  thrown error now exposes **both** `err.error` (server code, or raw body if not an object) and
  `err.parsed` (full body), plus `err.status`/message; dropped the unused `.url`/`.responseBody`/
  `.headers`. Locked with `fetch_utils.test.js` (6 tests mocking `fetch`; suite 32 → 38).
  **Deferred (behavior change, needs app-run verification — not done blind):** the shared
  timeout/abort (a default timeout could kill legitimately-slow calls like calendar fetches) and the
  centralized snackbar-on-error interceptor (auto-dispatching `showError` in the client would
  double-show or override the ~58 call sites that handle errors themselves).

- [x] **A11 · Trim remaining large components.** `M` — **DONE 2026-07-22 (all slices browser-verified
  via the headless-Chromium loop; see per-item notes).**
  After A5: `Event.vue` (now 1,776), `NewEvent.vue` (1,010), `RespondentsList.vue` (844),
  `NewSignUp.vue` (827). Candidates for extracting presentational children and moving pure helpers
  into `utils/`.
  - **Done:** removed `Event.vue`'s dead `interceptPluginResponses` debug method (listener was
    commented out) → 1,815 → 1,776.
  - **Done 2026-07-22:** `pluginMessagesMixin` extracted (`components/event/pluginMessagesMixin.js`
    — `handleMessage`/`setSlots`/`getSlots`, 567 lines, verbatim; orphaned + pre-existing unused
    imports pruned) → Event.vue **1,175**. Plugin API runtime-verified via headless Chromium:
    get-slots/set-slots round-trip on a real event (guest write + readback + UI), no console errors.
  - **Done 2026-07-22 (Tier 2 child splits, both browser-verified):** `EventHeader.vue` (title/chips/
    date + 8-event action-button block; helpDialog moves in) and `EventBottomBar.vue` (phone action
    bar + mobile button-text computeds; 7 events) → Event.vue **1,006** (was 1,815 pre-A11).
  - **Done 2026-07-22 (final Tier 2 splits, both browser-verified):** `NewEventAdvancedOptions.vue`
    (Advanced-options panel content, 6 `.sync`-bound fields; verified by setting every field through
    the UI and confirming the created event's API payload) → NewEvent.vue **887** (was 1,010); and
    `ExportCsvMenu.vue` (kebab menu + dialog + whole CSV build/download feature; both export formats
    verified by downloading and checking content) → RespondentsList.vue **677** (was 844).
    **A11 complete** — remaining component sizes are inherent feature complexity; further splits
    would be churn, not payoff.
  - **⚠️ Verification caveat (learned the hard way here):** `Event.vue` is mostly `this`-coupled
    action handlers, not the pure/geometry code A5 had. The only large "method" appeared to be ~595
    lines but was actually THREE methods — `interceptPluginResponses` (dead) **plus the active
    `setSlots`/`getSlots` plugin handlers** (an `async`-method detection gap hid them). `npm build`
    and eslint do NOT catch deleting an actively-`this`-called method, so a wrong boundary silently
    breaks runtime (here: the plugin API). Remaining A11 extractions (a `pluginMessagesMixin` for
    `handleMessage`/`setSlots`/`getSlots`, or child-component splits) should be done **with the app
    running to smoke-test** — do not do them blind. Pre-existing unused imports in `Event.vue` (~7)
    are separate baseline cruft, safe to prune later.

- [x] **A12 · Remove stray `console.log` and backend `fmt.Println` debug prints.** `S` — **DONE
  2026-07-22 (CI-green).** Dropped the stray frontend logs: `SignUpForSlotDialog` (logged the block
  on submit), `FeatureNotReadyDialog` (empty-feedback else that only logged — removed the branch),
  `NewEvent` edit-error catch (kept user-facing `showError`, dropped unused `err`). **Left
  intentionally:** the structured `[PLUGIN RESPONSE]` logging in `Event.vue` (deliberate plugin-API
  dev tooling). Backend: the only remaining `fmt.Println` is `utils.PrintJson`, a named debug utility
  whose print IS its purpose (only called from a non-compiling script) — not a stray print; the stray
  handler prints were already removed back in A1/A3.

### P3 — Housekeeping

- [x] **A13 · Align Go toolchain version.** `S` — **DONE 2026-07-22.**
  Bumped `server/go.mod` `go 1.20` → `go 1.25` to match the CI toolchain (`setup-go` with
  `go-version: "1.25"` in `backend-ci.yml`). Verified green by CI (no local Go toolchain).

- [x] **A14 · Prune legacy CORS origins.** `S` — **DONE 2026-07-23 (folded into D1).** Once D0 settled
  the domain, `main.go`'s fallback default became
  `https://gathering.sirthomasfoolery.com,http://localhost:8080` (was `schej.it`/`www.schej.it`/
  `timeful.app` set). Prod still sets `CORS_ORIGINS` explicitly; this is just the sane fallback now.

- [x] **A15 · Clean up / document migration scripts.** `S` — **DONE 2026-07-22.**
  Added `server/scripts/README.md`: explains each dated folder is a run-once manual migration (kept
  for history), warns against re-running destructive ones, and documents that they intentionally
  don't compile / are excluded from CI. Used a single README listing the folders in date order rather
  than fabricating per-folder run-date/status (each `main.go` is its own record).

### 2026-07-27 review additions

- [x] **A16 · Lost-update race: whole-document `$set` on event writes.** `M` · **P1 — DONE
  2026-07-28** (build/vet clean, full suite green against both an empty Mongo and a restored prod
  replica). Finding confirmed and slightly wider than reported: **four** handlers wrote the whole
  document back — `updateEventResponse`, `deleteEventResponse`, `userResponded` and `editEvent`.
  The last is the worst of them: an edit reverted any response, RSVP, poll or comment made while
  the dialog was open.
  New scoped writers in `db/events.go`, so Mongo access stays in `db/`:
  - `IncrementNumResponses` — `$inc`, not read-modify-write. A 20-goroutine test counts 20.
  - `SetSignUpResponse` / `DeleteSignUpResponse` — target the one map key by dotted path so two
    people claiming different slots don't overwrite each other. Keys are user ids (safe) or
    guest-supplied names, so a key containing `.` or leading `$` can't be addressed and falls back
    to rewriting the map; both paths are tested.
  - `DisarmSendEmailAfterXResponses` — compare-and-set on the threshold. This is what makes the
    "N people have responded" email fire **exactly once**: the guard used to be flipped in memory
    and persisted by the very write that raced. Ten concurrent callers, one winner.
  - `MarkRemindeeResponded` — guarded positional update. Also makes "everyone has responded"
    send once; `userResponded` now re-reads before deciding "everyone", rather than judging it
    from a snapshot that may be seconds stale.
  - `UpdateEditableEventFields` — `$set` limited to the 17 fields the edit form owns. It marshals
    and filters rather than naming values, because every one is `omitempty`: under the old write a
    nil pointer was omitted and kept its stored value, and naming them would have written nulls
    instead. Pinned by a test.
  11 new DB-gated tests, several of which fail against the old whole-document write.
  **Not addressed (deliberate):** `routes/events.go` `scheduleEvent`/`archiveEvent` and the RSVP
  handlers already write scoped `$set`s; the remaining read-modify-write window on
  `signUpResponses` capacity assignment is narrower but real — a slot can still be overfilled by
  two simultaneous joins, which wants a different fix (conditional update on the block's count)
  and is not what A16 describes.

- [x] **A17 · Rune-safe text truncation.** `S` · **P2 — DONE 2026-07-28** (build/vet clean,
  golangci **0 issues**, suite green over 6 consecutive runs).
  All truncation in `routes` now goes through one helper. `truncateRunes` moved out of
  `events.go` into a new **`routes/text.go`** — with four callers across four files it no longer
  belonged beside the event payload code — joined by `trimAndTruncate`, which pairs trim-then-bound
  in that order so padding can't eat the budget.
  - **The A17 sites**: `comments.go` (`maxCommentLength`) and `polls.go` (`maxPollTitleLength`,
    `maxPollOptionLength`) — byte slicing → `trimAndTruncate`.
  - **Also unified**: `sanitizeResponderName` (`event_responses.go`) was already rune-aware but had
    the logic inlined, and `sanitizeEventText` called trim+truncate by hand. Both delegate now, so
    there is exactly one truncation implementation to get right. **-37 lines, +8.**
  - **Semantic note:** these caps now bound **characters**, not bytes — which is what "2,000
    characters" means to a member, and still bounds storage (≤4 bytes per rune).
  - **Tests** (`routes/text_test.go`): the load-bearing one sweeps every cap from 0 past the string
    length over emoji/accented inputs and asserts `utf8.ValidString` throughout — plus a case where
    two poll options identical after truncation must collapse and fail the ≥2-distinct rule rather
    than storing duplicates. **Verified by reverting the helper to byte slicing: 6 tests fail**,
    including the pre-existing `TestTruncateRunes`.

- [x] **A18 · Dead-code deletion batch (frontend + backend).** `S` · **P2 — DONE 2026-07-28**
  (backend build/vet + full suite green; frontend eslint 0 errors, 80/80 tests, build OK).
  Deleted: `Friends.vue` + `FriendItem.vue` + `UserItem.vue`; `SignInDialog.vue` and its two
  orphaned `App.vue` handlers (`_signIn`/`_emailSignIn`) plus the imports they were the last users
  of; `App.vue` scroll tracking (`handleScroll`/`scrollY` + two window listeners); `Event.vue`'s
  empty `beforeMount()`, the identical-branch `resetWeekOffset`, and three commented blocks; and
  `utils/mail_utils.go`'s `AddUserToMailchimp`/`AddUserToMailjet` (which also cleared the 8 `, _ :=`
  sites, and their env vars left `.env.template`). `discord_bot/` went with **E7**.
  Also collapsed `App.vue signIn()` — both branches pushed the same route, only the webview guard
  differed.
  **⚠️ One A18 claim was WRONG:** `store/index.js` `refreshAuthUser` is **not** "never dispatched" —
  it is live, dispatched from six components (`ICSCredentials`, `AppleCredentials`,
  `CalendarAccounts`, `EventItem`, `ScheduleOverlap`, `Settings`). **A4** had already deleted the
  duplicate definition that finding was really aimed at. Left in place.

- [x] **A19 · Drop 8 unused npm dependencies.** `S` · **P2 — DONE 2026-07-28** (clean
  `npx npm@10 ci` from an empty `node_modules` for CI parity; 80/80 tests, eslint 0 errors, build
  OK). Removed all 8 listed (`vue-github-button`, `vue-vimeo-player`, `html2canvas`,
  `vue-html2canvas`, `copy-image-clipboard`, `canvas-confetti`, `@rive-app/canvas`, `ua-parser-js`)
  **plus `register-service-worker`** — not on A19's list, but the same finding: **C8** already
  recorded it as unused after the PWA was deliberately removed (`f857320`). The stale CLAUDE.md line
  claiming a service worker is registered was corrected at the same time.
  The two flagged for verification are **both real and were kept**: `vue-worker` is registered in
  `main.js` *and* used via `this.$worker` in `availabilityMixin.js:57`; `vuedraggable` is a live
  component in `Dashboard.vue`.

- [x] **A20 · Router guard does a network round-trip on every navigation; `/user/profile` fetched
  from 5 places.** `S` · **P2 — DONE 2026-07-28, folded into E3 phase 3** (same guard, same file —
  doing it separately would have meant editing `router/index.js` twice and resolving a conflict).
  The guard now reads `store.state.authUser` and only dispatches `refreshAuthUser` when it's empty
  (cold load / after sign-out), instead of awaiting `GET /auth/status` on every navigation.
  `refreshAuthUser` returns the user now as well as committing it. **Still open (minor):** the
  remaining independent `/user/profile` fetches in `App.vue`, `Home.vue`, `Event.vue` and
  `currentAvailabilityMixin.js` — the per-navigation round-trip was the expensive part and is gone;
  consolidating those four is cheap follow-up whenever they're next touched.

- [ ] **A21 · Calendar-service error-handling leftovers.** `S` · **P3**
  `services/calendar/google_calendar.go:22,79` — `req, _ := http.NewRequest` (the same latent
  nil-deref A2 batch 4 fixed in `services.go CallApi`). `google_calendar.go:136-137`,
  `apple_calendar.go:107-108`, `ics_calendar.go:69-70` — `time.Parse` errors discarded, so an
  unparseable all-day date silently becomes year-0001 and renders as a bogus availability block
  instead of being skipped.

- [ ] **A22 · Small cleanup batch.** `S` · **P3**
  - Byte-identical toggle→PATCH `/user/calendar-options`→emit logic in
    `WorkingHoursToggle.vue:75-85`, `BufferTimeSwitch.vue:65-79`, `CalendarAccount.vue:200,224` —
    extract one mixin.
  - PostHog is a no-op `Proxy` stub (`plugins/posthog.js`) yet `$posthog.capture` call sites remain
    scattered — remove both.
  - `getEvent` dead debug scaffolding (`events.go:568-573`): marshals the whole privatized response
    to indented JSON on every call, assigns it to `_`. Hot read path; delete.
  - `errs/errors.go:10` TODO — error codes are bare strings; make them a type.
  - `Event.vue:981` / `SignUp.vue:57` — EventNotFound bounces to `home`, which re-bounces
    non-members; go direct to the right destination (mostly moot after E3, still a double redirect).
  - `ScheduleOverlap.vue:2334` TODO — half-hour timezones (India/Nepal/Newfoundland) unhandled.

- [ ] **A23 · Split the two remaining giants (continuation of A5/A11).** `L` · **P3**
  `ScheduleOverlap.vue` 2,986 lines (the `computed` block alone is ~1,000 — :1275-2282);
  `date_utils.js` 1,119 lines / 46 exports (split formatting / arithmetic / timezone);
  `Event.vue` 1,036; `NewEvent.vue` 975 vs `NewSignUp.vue` 845 — likely heavy overlap, **diff them
  first**. Continue the extract-pure-module pattern (and heed A11's caveat: verify splits with the
  app running, not blind).

---

## PART B — Test Coverage (its own track — currently thin)

- [x] **B1 · Cover the core `events.go` handlers.** `M` · **P1** — **DONE 2026-07-22 (CI-green,
  3 incremental commits).** Went from zero event tests to 20, split into DB-free unit tests and
  DB-backed integration tests.
  **Pure logic** (`routes/event_responses_test.go`, 17 tests, no Mongo): the easy-to-regress privacy
  rules — `getResponsesMap` (keying/empty/duplicate-id last-wins), `findResponse`,
  `stripSensitiveUserFields` (clears calendar/billing, preserves identity, nil-safe),
  `shouldKeepGroupResponseUserEmails` (DB-free guard branches), and **blind-availability filtering**:
  extracted `getResponses`' inline logic into a pure helper `filterResponsesForBlindAvailability`
  (behavior-preserving) and covered the full matrix (blind off → all; blind on → owner sees all,
  non-owner only own, guest only theirs, anon nothing).
  **DB-backed handlers** (option (a): `routes/event_responses_db_test.go`, 3 tests): `TestMain` calls
  `db.Init()` when `MONGODB_URI` is set (`mongo.Connect` is lazy, so safe); tests gate on that via
  `requireDB(t)` so `go test ./routes/` still passes without Mongo (they skip) and run in CI (Mongo
  service). Drive the handlers through a real gin engine + session middleware: `getResponses` 404 on
  missing event and blind-off happy path (returns all); `updateEventResponse` guest POST persists a
  new `EventResponse`. Fixtures inserted under a fresh ObjectID, cleaned up per-test.
  **Optional follow-ups (not blocking):** the per-response email-visibility loop (`showEmails` +
  `User.Email` stripping, entangled with the `shouldKeep` DB call) and `updateEventResponse`'s
  signed-in / GROUP / sign-up-form branches are still uncovered.

- [x] **B2 · Cover access-control end to end.** `S` · **P1** — **DONE 2026-07-22 (CI-green).**
  New `routes/access_control_test.go` exercises the invite-only gate end to end (not just the
  `models/roles.go` unit level). `AuthRequired` is driven through a real gin engine + session
  middleware, with the session cookie minted via a `test-login` helper route (the way a real sign-in
  would): not-signed-in → 401; **struck-off member** (valid session, email no longer allowlisted) →
  401 (a sentinel allowlist entry keeps the list non-empty so `IsAccessAllowed` can't fail open);
  allowlisted member → 200. `CanInviteRequired` (pure): guest → 403 + aborted, member → passes.
  Handler role check: a **guest POSTing `/events` → 403** before any event is built. DB-backed cases
  gate on `requireDB` (skip without Mongo; run in CI).
  *(Fixed one CI-caught bug: `primitive.DateTime` binds from an RFC3339 string, not epoch-ms.)*

- [x] **B3 · Frontend: cover the grid/availability math extracted in A5.** `M` · **P2** — **DONE
  2026-07-22 (CI-green).** The A5 mixin still held the geometry as `this`-dependent methods, so first
  extracted the pure computational core into `schedule_overlap/gridGeometry.js` (`clampRow`,
  `clampCol`, `getRowColFromXY` as pure functions of their inputs) and made `dragGridMixin` delegate
  to it (exact transcription → behavior unchanged; dropped the now-unused `clamp` import). Added
  `gridGeometry.test.js` — 9 vitest cases covering row/col clamping in both daysOnly and time-grid
  views, `columnOffsets`-based column derivation, past-last-offset clamping, and the split-gap row
  adjustment. Frontend suite 23 → 32 tests. **Remaining frontend test gap:** the availability
  fetch/format/animate logic (now in `availabilityMixin`/`currentAvailabilityMixin`) is still
  `this`-dependent and would need the same extract-pure-core treatment to be unit-testable.

- [x] **B4 · Three existing Go test files never run in CI.** `S` · **P0 — DONE 2026-07-28.**
  `backend-ci.yml` now also tests `./services/calendar/ ./services/contacts/
  ./services/microsoftgraph/` (all three pass locally). The old comment's "need external
  credentials" was true of the *packages*, not their tests — all three are pure logic. The
  `DEVELOPMENT.md` local-test command had drifted further still (it was also missing
  `./services/reminders/`); both lists are now identical, with a note to keep them in sync.

- [x] **B5 · Work down the golangci-lint errcheck backlog; flip lint to blocking.** `M` · **P2 —
  DONE 2026-07-28** (0 issues; lint is blocking; build/vet + full suite green on both an empty
  Mongo and a restored prod replica).
  **First finding: the lint step was linting nothing.** golangci-lint takes *directories* and
  `go list ./...` emits *import paths*, so every package resolved against the wrong root — CI
  printed a few "typechecking error: directory not found" lines and then `0 issues`, which reads
  exactly like a clean run. Confirmed in the workflow log for 67d8bcd, not just locally. Fixed with
  `go list -f '{{.Dir}}'` and a comment, because the failure mode is so quiet.
  With it actually running: **119 issues**, not the ~112 estimated blind. errcheck is now relaxed
  for `_test.go` only (~50 were teardown where the error has nowhere to go); the remaining 66 were
  all production and are now zero.
  **Four were real bugs, not noise:**
  - `services/calendar/apple_calendar.go` — `loc, err :=` declared a **second `err`** scoped to the
    TZID branch, so the `ParseInLocation` failure landed in the shadow and the outer check read a
    nil. An unparseable time with a TZID returned the zero time and no error. A
    `//lint:ignore SA4006 err is in fact used later` comment sat on top asserting the opposite.
    Four regression tests; the first fails against the old code with `0001-01-01`.
  - `main.go` — `router.Run`'s error was discarded, so a server that couldn't bind its port exited
    silently with status 0. A failed start was indistinguishable from a clean shutdown.
  - `db/init.go` — four indexes created unchecked (Chronicle at-most-once, OTP expiry, allowlist
    uniqueness, one default folder per kind). Existing duplicate data makes creation fail
    permanently while the code carries on assuming the invariant holds. Now logged loudly, naming
    the guarantee that is not in force; deliberately not fatal.
  - `services/auth/auth.go` — both OAuth token responses were decoded with the error dropped, so a
    malformed body became a zero-valued token that failed confusingly later.
  Plus: `GetAllowlist` swallowed its `Find` and `cursor.All` errors and returned an empty slice, so
  a broken query rendered as "the Fellowship has no members" (**closes the allowlist half of E10**);
  the response write in `updateEventResponse` and both response deletes answered 200 on a failed
  write; removing a calendar account reported success while still linked; and the OTP attempt
  counter, whose failure silently un-caps the 5-try brute-force guard.
  **`continue-on-error` is gone** — anything the linter reports now fails the build.

- [x] **B6 · AES-CFB is deprecated and unauthenticated.** `M` · **P2 — DONE 2026-07-28, all four
  steps deployed.**
  `utils/utils.go` used `cipher.NewCFBEncrypter/Decrypter` for `ENCRYPTION_KEY`. CFB is
  unauthenticated: a tamperer with database write access can flip plaintext bits undetected, and a
  wrong key yields plausible garbage rather than an error. Deprecated in Go 1.24.

  **⚠️ The original finding's premise was wrong.** It said "every stored calendar token is CFB".
  Not so: `utils.Encrypt` has exactly **one** caller (`routes/user.go` `addAppleCalendarAccount`)
  and `utils.Decrypt` exactly one (`services/calendar/apple_calendar.go` `getClients`). The only
  encrypted value in the system is the **Apple calendar app-specific password**. Google/Outlook
  OAuth tokens are stored in plaintext — a bigger problem, filed as **[B7]**.

  - **1/4 DONE** (`1cd4ac36`): `Encrypt` writes AES-256-GCM; `Decrypt` reads both formats.
    Version marker is the textual prefix `v2:`, **not** a leading version byte — a v1 blob starts
    with a 16-byte random IV whose first byte can be any value, so a numeric marker is ambiguous
    with real data. `:` is outside the base64 alphabet, so the discriminator is exact.
    Both versions keep using `ENCRYPTION_KEY`'s bytes as the raw AES key: an earlier draft derived
    the v2 key with SHA-256 and that was **dropped as wrong** — the key is already a valid 32-byte
    AES-256 key, so derivation bought nothing and left two key concepts coexisting mid-migration.
    `Decrypt` also stopped panicking on malformed input, which orphaned the exported `Decode`
    (an **E10** bare-panic item) — removed with the rewrite rather than left as dead panicking code.
  - **2/4 DONE** (`4c6f0bfc`): `validateEncryptionKey()` at startup beside `validateSessionSecret`.
    Nothing validated the key before, so a wrong length failed per-request deep inside a calendar
    call. What made it worth a guard: **`DEPLOYMENT.md` told you to generate the key with
    `openssl rand -base64 32`, which produces 44 characters — a length AES rejects outright.**
    Prod's key is a valid 32 bytes so this deployment was never affected, but the next would have
    been. Corrected in `DEPLOYMENT.md`, `server/.env.template` and `CLAUDE.md` (`openssl rand
    -hex 16`). `SESSION_SECRET`'s base64 generation is fine and untouched — it only needs ≥32 chars.
  - **3/4 DONE** (`37ea8330`): `db.ReEncryptLegacyCalendarSecrets()` runs at startup, before the
    router serves, and rewrites any v1 value as v2. Idempotent; best-effort (a failure logs and the
    boot continues, since values stay readable via v1); an **undecryptable value is left untouched**
    rather than overwritten. It rewrites the whole `calendarAccounts` field rather than a dotted
    path, because the map key is `email_TYPE` and emails contain dots — Mongo would read
    `a.b@c.com_apple` as four levels of nesting. Verified end-to-end against genuine v1 ciphertext:
    seed → boot → stored `v2:` → decrypts back to the original secret.
    Also fixed `db`'s `TestMain`, which never called `logger.Init`, so any db function logging on an
    error path **panicked inside tests** instead of returning its error (`routes` was already fixed).
  - **4/4 DONE** (separate deploy, as required): `decryptV1CFB`, its `//nolint:staticcheck`,
    `IsLegacyCiphertext`, `db/encryption_migration.go` + its test, and the `main.go` sweep call are
    all gone. `Decrypt` is GCM-only and **refuses** an untagged value rather than handing it to a
    mode that cannot detect tampering.
    **Gate satisfied before deleting anything:** 1–3 deployed at `7722dc7`; prod holds **1 user with
    calendar accounts and zero Apple accounts**, so there was never any v1 ciphertext — the sweep
    correctly logged nothing (a success, per the step-3 note), and the verification aggregation
    returned empty both before and after. The `//nolint:staticcheck` that B5 added to `utils/utils.go`
    is now gone from the repo entirely; no CFB call remains, only comments explaining why.
    Tests: the three v1 tests and the `encryptV1CFB` fixture went with the path, replaced by
    `TestDecrypt_RejectsUntaggedCiphertext` — the check that fails if CFB is ever reintroduced.
    (The handover listed two v1 tests; there were three — `TestV1CiphertextIsNeverMistakenForV2`
    also used the fixture.)

- [x] **B7 · Google/Outlook OAuth tokens are stored unencrypted.** `M` · **P1 — DONE 2026-07-28,
  all four steps deployed (1–3 together, 4 separately as required).**
  Surfaced while scoping **B6**. `OAuth2CalendarAuth.AccessToken` / `.RefreshToken` were written
  straight to the user document at four live sites — `routes/auth.go` (`signIn`, `signInMobile`)
  and `routes/user.go` (add Google / add Outlook) — with no encryption. A **refresh token is a
  long-lived credential** granting ongoing read access to a member's calendar; anyone with a copy
  of the database had them all. The inconsistency was the tell: the Apple password path already
  encrypted, so this was an omission rather than a decision.

  - **1/4 DONE** (`0a4ac8f`): encryption is a property of the **type**, not of the call sites —
    `models.EncryptedString` encrypts in `MarshalBSONValue` and decrypts in `UnmarshalBSONValue`.
    The call sites are the problem: four write paths, six read paths, and any one of them
    forgetting leaves a credential in the clear. It also covers the three handlers that persist
    the whole user document and never name a token at all.
    `Encrypt`/`Decrypt` **moved out of `utils` into a new leaf package `encryption`** — they had
    to, because models needs them and `utils` imports `models`, so leaving them was an import
    cycle. Verbatim apart from unexporting `Encode`; new `IsCiphertext` is what lets the codec
    tell ciphertext from legacy plaintext. Two dangling comments in the moved tests, left when B6
    step 4 deleted the v1 tests out from under them, went at the same time.
    **The judgement call:** a value that is tagged but fails to decrypt errors rather than
    degrading to `""`. It has to — `getCalendars`, `addCalendarAccount` and
    `RefreshUserTokenIfNecessary` all read the document, change one field and write it back, so a
    silent `""` would let a wrong `ENCRYPTION_KEY` destroy every stored refresh token on the next
    calendar fetch. Failing the decode means nothing is written back at all.
  - **2/4 DONE** (`fa9be3d`): `db.EncryptPlaintextOAuthTokens()` at startup, before the router
    serves. Walks **raw BSON**, not `models.User`: decoding into the model returns plaintext
    either way (the legacy passthrough), so the model cannot tell you which documents still need
    migrating. Structure lifted from B6's sweep at `37ea8330`, as B7 anticipated; whole
    `calendarAccounts` field again, because `email_TYPE` keys contain dots. A token that can't be
    encrypted is left in the clear rather than blanked.
  - **3/4 DONE** (`e9247cf`) — the fold-in this item called for. All three sites that wrote
    `$set: user` now use `db.SetUserCalendarAccounts`; **`addCalendarAccount` also returns its
    error**, and its four callers answer 500 instead of reporting a calendar connected when
    nothing was stored (the class of bug **B5** fixed for `removeCalendarAccount`).
  - **4/4 DONE** (separate deploy, as required): the plaintext passthrough is gone, so an untagged
    token is refused rather than used. `encryption.Decrypt` already refuses one, so this was a
    deletion, not a new check; `TestEncryptedString_RefusesUntaggedValue` is what fails if it comes
    back.
    **The sweep stays**, unlike B6 step 4, and that is the point rather than an oversight: it walks
    raw BSON and so does not depend on the read path it feeds. Restore a pre-B7 backup and the next
    boot re-encrypts it before the router serves a request, where under B6's arrangement the
    refusal would simply lock those members out. Nothing can write a plaintext token after boot —
    every write goes through the codec.
    Not folded in: the Apple password onto the same type, so there is one mechanism rather than
    two. It is a fair follow-up but not a regression risk either way now; **B6's path is already
    strict**, and it was only unsafe to move it while this type still passed plaintext through.

  **Gate satisfied before deleting anything:** 1–3 deployed at `f039fe7`; the boot log showed
  `encrypted stored OAuth tokens for 1 of 1 users with calendar accounts`, and prod then held zero
  untagged tokens (254 → 379 and 103 → 179 chars, exactly base64(12-byte nonce + n + 16-byte tag)
  plus the prefix). Checked the rest of the database too, not just `users`: `DailyUserLog.Users
  []User` embeds whole user documents, which would have been a second copy of every token — but
  `UpdateDailyUserLog` only ever appends `userIds`, and prod has **0** logs carrying a `users`
  field. No other collection holds one.

  **Verified end to end on prod, not only in tests:** real OTP login, then
  `/api/user/calendars` → **200 with 21 events** — the whole loop, since that path decrypts the
  stored refresh token, exchanges it with Google for a fresh access token and writes the new one
  back. Confirmed the write landed tagged with an expiry an hour out, which also exercises step 3's
  scoped writer. (Locally only the read path is provable: `CLIENT_ID`/`CLIENT_SECRET` aren't set on
  the dev box, so the refresh returns `invalid_client` — which is how **B8** surfaced.)

- [x] **B8 · A failed OAuth token refresh reports the wrong reason.** `S` · **P3 — DONE
  2026-07-28** (build/vet/lint clean, full suite green). Found while verifying B7.
  `services/auth/types.go` typed `AccessTokenResponse.Error` as `bson.M`, but Google and Microsoft
  both return `"error": "invalid_grant"` — a *string*. So the decode in `RefreshAccessToken` failed
  and the caller was told `json: cannot unmarshal string into Go struct field
  AccessTokenResponse.error of type primitive.M`, while the actual reason (revoked consent, expired
  refresh token, bad client credentials) was thrown away. `TokenResponse.Error` next door was
  already correctly a `string` + `error_description`. Pre-existing, unrelated to B7 — the encryption
  work just made it visible.

  **Fixed as reported, plus the three things that made the fix worth nothing on its own** — the type
  was only the first of four links in the chain, and repairing any one alone still loses the reason:
  1. `Error string` + `ErrorDescription string`, matching `TokenResponse` (the `bson` import in
     `types.go` goes away with it).
  2. **`RefreshAccessToken` never checked the field.** Even decoding correctly, a refused refresh
     returns HTTP 400 with a *well-formed* JSON body, so it decoded cleanly and the function
     returned an empty access token and `nil` error. Now returns
     `access token endpoint error: invalid_grant: Token has been expired or revoked.`, mirroring
     `GetTokensFromAuthCode`.
  3. **`RefreshUserTokenIfNecessary` silently `continue`d** on every refresh failure. A correct
     error that reaches no log is still invisible — a user with revoked consent just sees an account
     with no events. It logs the account and reason now.
  4. **`RefreshAccessTokenAsync` left `Email`/`CalendarType` zero on both failure paths** (the error
     return *and* the panic-recover), so that new log line would have read `for  ()`. Both now
     carry the account.

  Dropped the two bare `logger.StdErr.Println(err)` calls in `RefreshAccessToken` — they logged an
  error the function also returns, and the caller-side log has the account context they lacked. One
  line per failure, not two.

  **Tests:** `services/auth` had none, so this adds `auth_test.go` (5) on a stubbed
  `http.RoundTripper` — no network. The error-code case asserts the reason *and* explicitly guards
  that the message no longer contains `cannot unmarshal`; the success case asserts a good refresh
  still returns its token, so the new check can't pass by rejecting everything; plus a no-description
  variant, a non-JSON body (must stay distinguishable from a refusal), and one that the async
  wrapper carries the account on failure. Verified the guard is real by decoding Google's actual
  error body into the old struct shape — it reproduces the `primitive.M` message verbatim.

  **`services/auth` was not in the CI test list**, so these would never have run — the same gap
  **B4** found. Added to `backend-ci.yml` and to all three package lists in `DEVELOPMENT.md`, two of
  which had also drifted (missing `./encryption/` since B6).

---

## PART C — New Features (fit for a ~40-person invite-only club)

### P1 — High value, leverages infrastructure already present

- [x] **C1 · RSVP / attendance tracking for a *confirmed* gathering.** `M` — **DONE 2026-07-23
  (CI-green; backend build/vet/tests + frontend build/lint/tests pass; RSVP endpoints +
  RSVP→reminder pipeline verified live against local Mongo).** Adds a 3-state RSVP
  (Going / Maybe / Can't make it) to any event with a confirmed gathering (**[C2]**'s
  `scheduledEvent`), a live headcount + roster, and wires the result into C2's reminder targeting.
  - **Storage** (`models/event.go`): a new `Rsvps map[string]*Rsvp{status,name,email,userId,
    respondedAt}` on the Event doc, keyed by guest-name / user-id — mirrors `SignUpResponses`.
    **Not** the `Attendee` model (that's group-invite-specific: Email + Declined only). No new
    collection / migration.
  - **Endpoints** (`routes/event_responses.go`): `POST /events/:eventId/rsvp` (status +
    guest/signed-in identity, keyed like `updateEventResponse`; signed-in RSVPs backfill
    name/email from the account) and `DELETE …/rsvp` to un-RSVP. Requires a confirmed gathering
    (400 `gathering-not-scheduled`) and validates the status enum.
  - **C2 integration** (`services/reminders`): `processDueReminders` now prefers RSVPs — new
    `collectRsvpRecipientEmails` reminds `going`+`maybe` (decliners excluded); if **no** RSVP
    exists yet it falls back to all availability respondents, so reminders keep working before
    anyone RSVPs.
  - **Frontend**: `GatheringRsvp.vue` (shown when `event.scheduledEvent` exists) — headcount
    ("N going · M maybe · K can't"), a roster grouped by status (visible to all — it's a club),
    and 3 RSVP buttons highlighting the viewer's choice (re-click to clear). Signed-in users
    RSVP directly; guests enter a name first (same trust model as guest availability). Mounted in
    `Event.vue` between the description and the calendar; `EventService.setRsvp/clearRsvp` persist
    then `refreshEvent`.
  - **Tests**: `collectRsvpRecipientEmails` unit test (going/maybe in, no out, dedupe, signed-in
    lookup) + a DB-gated integration test (RSVPs present → only going+maybe emailed, availability
    responders ignored). Live-verified the full endpoint flow (pre-schedule 400 → RSVP →
    change → un-RSVP).
  - **Independently re-verified live on prod 2026-07-23 (this machine, via headless Chromium
    against gathering.sirthomasfoolery.com):** BOTH UI paths end-to-end on a throwaway
    scheduled event — **guest** (name field → Going, roster "Going: <name>") and **signed-in**
    (no name field, identity backfilled → "Going: Jason Anderson"), plus the [C4] plus-one
    stepper and the decliner-forces-0-guests rule, status changes, and clear-on-re-click. All
    assertions green, no console errors; test events deleted afterward. (See PART E for two
    incidental findings this surfaced.)
  - **Non-goal:** guest plus-ones/spouse headcount is **[C4]** — the `Rsvp` struct leaves room
    for a `GuestCount` without migration.

- [x] **C2 · Automated pre-gathering reminder emails.** `S–M` — **DONE 2026-07-23 (CI-green;
  backend build/vet/tests + frontend build/lint/tests all pass; API round-trip + DB-backed
  pipeline verified against a local Mongo).**
  The TODO's premise turned out stale: (1) **no confirmed time was ever persisted** — the
  "Schedule event" button only opened a Google/Outlook *template URL* and wrote nothing back;
  (2) the **Cloud Tasks + listmonk path is dead on this fork** (all `# optional`, points at
  the upstream's GCP project `schej-it`; OTP already moved to Gmail SMTP). So instead of
  extending that path, the feature persists the locked-in time and sends via the fork's real
  mail transport (Gmail SMTP), on a **self-contained in-process scheduler** — no GCP/listmonk.
  - **Persist the gathering** (`POST /events/:eventId/schedule`, owner-gated like `editEvent`):
    reuses the existing (previously-unwritten) `Event.ScheduledEvent *CalendarEvent` for the
    time, plus a new `GatheringReminder{Enabled, LeadTimeHours, Timezone, SentAt}` struct
    (`models/event.go`). `scheduled:false` cancels (unsets both). Lead time clamped 1..168h,
    default 24; `SentAt` reset to nil on every (re)schedule so it re-arms.
  - **Scheduler** (`services/reminders/`): a ticker goroutine started in `main.go`
    (`REMINDER_SCHEDULER_INTERVAL`, default 5m) that no-ops with a log if the Gmail vars are
    unset (mirrors `gcloud.InitTasks`). Each tick: `db.GetEventsWithPendingReminders()` →
    Go-side lead-time window (`isReminderDue`) → recipients = all availability respondents with
    an email (`collectRecipientEmails`: guest `Response.Email`, else signed-in via
    `db.GetUserById`, deduped) → inline-HTML email (Fellowship style, time formatted in the
    saved tz) → **mark `SentAt` regardless of per-send failures** so it never loops. Sender is
    injected (`SendFunc`) for testability.
  - **Frontend**: `EventService.setScheduledEvent`; `ScheduleOverlap.confirmScheduleEvent` now
    persists (keeps opening the organizer's own calendar URL) + `cancelGathering`; `ToolRow`
    gains a reminder toggle + lead-time select in the Schedule menu and a "Gathering set"
    indicator (shows time + reminder summary) with Reschedule / Cancel actions. Mobile
    (`EventBottomBar`) uses the defaults (reminder on, 24h).
  - **Tests**: `services/reminders` pure unit tests (`isReminderDue`, `collectRecipientEmails`
    guest/signed-in/dedupe, `buildReminderEmail` tz + UTC fallback) + a DB-gated integration
    test (`requireDB`/`TestMain`) driving the whole pipeline with a mock `SendFunc`.
  - **Notes / non-goals:** single-VM scheduler (no distributed lock — fine for this fork);
    recipients = respondents-with-email until **C1 (RSVP)** lands, then swap
    `collectRecipientEmails` for the confirmed-attendee list. **Swagger `docs/` regenerated
    2026-07-23** (the dedicated docs-regen pass): resynced with the D1 `@title` ("The Fellowship
    API") + all the new routes (rsvp/ics/comments/schedule/…). Requires
    `swag init --parseDependency --parseInternal` — a bare `swag init` aborts on the allowlist
    models' `primitive.DateTime`; the flags resolve it (guidance in CLAUDE.md).

- [x] **C3 · "Add to calendar" / `.ics` export for confirmed gatherings.** `S` — **DONE
  2026-07-23 (CI-green; backend build/vet/tests + frontend build/lint/tests pass; live .ics
  download verified against local Mongo).** Builds directly on **[C2]**'s persisted
  `scheduledEvent`: a universal, no-OAuth "add to calendar" that works for everyone (incl.
  spouses without a Google account).
  - **Generation** (`services/calendar/ics_generate.go` — the mirror of `ics_calendar.go`'s
    parsing, same `emersion/go-ical` lib): `GenerateEventICS(event)` builds a `VCALENDAR` +
    one `VEVENT` from `event.ScheduledEvent` — stable `UID` (`{id}@timeful.app`), UTC
    DTSTART/DTEND, SUMMARY, DESCRIPTION (+ event link), URL, `STATUS:CONFIRMED`,
    `METHOD:PUBLISH`. Errors if the event has no confirmed gathering.
  - **Endpoint** (`routes/events.go`): public `GET /events/:eventId/ics` — no auth so any
    invitee can add it; returns `text/calendar` with `Content-Disposition: attachment;
    filename="<slug>.ics"`. 404 (`gathering-not-scheduled`, new `errs` code) until a time is
    locked in.
  - **Reminder email** (`services/reminders`): the C2 pre-gathering email now carries an
    **"Add to calendar"** button pointing at `/api/events/{id}/ics` — closes the loop for the
    no-Google-account members right in the reminder.
  - **Frontend**: an "Add to calendar" chip on `EventHeader.vue` (visible to **everyone** who
    opens the event once it's scheduled) + an item in the owner's `ToolRow` "Gathering set"
    menu; both `:href` the `.ics` URL (`serverURL` + `/events/{id}/ics`), a plain download —
    no JS, works for guests.
  - **Tests**: `ics_generate_test.go` (structure/UTC formatting/escaping + no-gathering error).
    Live-verified end to end: 404 before scheduling → 200 with correct headers + a valid,
    comma-escaped `VEVENT` after. **Note:** `createEvent` still doesn't accept a `description`
    (only `editEvent` does) — pre-existing, unrelated; the generator includes it when present.

- [x] **C4 · Plus-one / guest handling on responses.** `S–M` — **DONE 2026-07-23 (CI-green;
  backend build/vet/tests + frontend build/lint/tests pass; plus-one persist + clamp verified
  live).** A small extension of **[C1]**: a respondent can indicate how many extra people
  (spouse/guests) they're bringing, so the headcount is accurate for the "≈12 men + wives" reality
  without every spouse needing an account.
  - **Model** (`models/event.go`): added `GuestCount int` to `Rsvp` — the number of *additional*
    people (headcount for an RSVP = 1 + GuestCount). The room the C1 struct left, now filled; no
    migration.
  - **Endpoint** (`routes/event_responses.go`): `rsvpToEvent` accepts `guestCount`, clamped by
    `clampGuestCount` (0..20; forced to 0 for `no` — decliners can't bring guests).
  - **Frontend** (`GatheringRsvp.vue`): a "Bringing guests: [− N +]" stepper that appears once
    you're going/maybe and re-submits the RSVP on change; the headcount now reads
    "N going (+G) · M maybe (+g) · K can't" and the roster shows "Alice (+2)".
  - **Tests**: `clampGuestCount` unit test (negative→0, over-max→20, decliner→0). Live-verified:
    going +2 / maybe +1 persist; `no +5`→0; `going +999`→20.

### P2 — Strong quality-of-life

- [x] **C5 · Recurring gatherings.** `M` — **DONE 2026-07-23 (CI-green; backend build/vet/tests +
  frontend build/lint/tests pass; schedule-with-recurrence + .ics RRULE verified live against a
  local server + Mongo; auto-advance covered by DB-gated integration tests).** Rather than a
  spawn-a-copy model (the TODO's `duplicateEvent` hint), this makes a *single* confirmed gathering
  repeat — one permanent event link + calendar series — which fits "First-Friday poker night"
  better than minting a new event/URL each cycle. Builds on **[C2]**'s `scheduledEvent` +
  reminder pipeline and **[C3]**'s `.ics`.
  - **Model** (`models/event.go`): `GatheringRecurrence{Frequency, Until}` on the Event (paired with
    `ScheduledEvent`; nil = one-off). Frequencies weekly / biweekly / monthly. No migration. Pure
    methods live with the type (unit-tested): `IsRecurring`, `Step` (monthly uses `addMonthsClamped`
    so Jan 31 → Feb 28/29, not Mar 3), `NextOccurrenceAfter` (skips a long outage — jumps to the next
    occurrence after `now`, doesn't replay missed ones), and `RRULE` (RFC 5545 string).
  - **Schedule endpoint** (`routes/events.go`): `scheduleEvent` accepts `recurrenceFrequency`
    (+ optional `recurrenceUntil`), validates the enum (bad value → 400), stores/clears
    `gatheringRecurrence`; `none` and cancel both unset it.
  - **.ics** (`services/calendar/ics_generate.go`): emits an `RRULE` (`FREQ=WEEKLY` /
    `FREQ=WEEKLY;INTERVAL=2` / `FREQ=MONTHLY`, + `UNTIL`) via `Props.Set` (not `SetText`, which would
    tag it `VALUE=TEXT` — wrong for a RECUR value), so a single "Add to calendar" covers the whole
    series in members' calendars.
  - **Auto-advance** (`services/reminders`): the scheduler now rolls a recurring gathering forward to
    its next future occurrence once the current one has *ended* — clearing that cycle's RSVPs (fresh
    headcount) and re-arming the reminder (`sentAt` unset). Guarded by a conditional `AdvanceGathering`
    update (keyed on the expected current start) so concurrent ticks can't double-advance; stops once
    the next occurrence would fall after `Until`. **Decoupled from email**: `StartReminderScheduler`
    now always runs the advance pass (only the *send* is gated on Gmail creds), so the event page shows
    the next occurrence even on an email-less instance.
  - **Frontend**: recurrence selector ("Does not repeat / Weekly / Every 2 weeks / Monthly") in the
    `ToolRow` Schedule menu (`.sync` through `ScheduleOverlap.confirmScheduleEvent`), a "Repeats …"
    line in the owner's "Gathering set" summary, and a **"Repeats …" chip on `EventHeader`** so
    *everyone* (not just the owner) sees the cadence next to "Add to calendar".
  - **Tests**: `models/event_recurrence_test.go` (Step, monthly clamp incl. leap-year + year boundary,
    skip-outage, RRULE incl. UNTIL), `ics_generate_test.go` (RRULE present for recurring / absent for
    one-off), and DB-gated `recurrence_integration_test.go` (rolls forward + clears RSVPs + re-arms +
    idempotent; respects `Until`; skips a still-ongoing occurrence).
  - **Swagger `docs/` regenerated** (pinned `swag@v1.16.1 --parseDependency --parseInternal`):
    documents the new schedule params + `GatheringRecurrence` model. The regen also normalized some
    pre-existing `primitive.DateTime` string/integer drift in the committed baseline (it was already
    internally inconsistent) — expected, not from this change.
  - **Known limitations (v1, documented in code):** monthly recurrence on day 29–31 clamps to the
    month's last day and can compound across short months (fine for a club — meetings fall on normal
    days), which may diverge slightly from a strict RRULE reader; no "nth-weekday" (e.g. "2nd
    Saturday") rule; comments accumulate across occurrences (single rolling event). Non-goal: per-
    occurrence history — that's **[C10]** (The Chronicle).

- [x] **C6 · Venue / activity poll (not just time).** `M` — **DONE 2026-07-23 (CI-green; backend
  build/vet/tests + frontend build/lint/tests pass; create/vote/switch/clear/delete verified live
  against a local server + Mongo).** Lightweight multiple-choice polls on an event ("Where should we
  meet?", "What should we do?") so the club can vote on venue/activity. Mirrors the RSVP/comments
  trust model rather than the sign-up-block editor (which is heavier, tied to the availability grid).
  - **Model** (`models/event.go`): `Polls []Poll` on the Event (mirrors `SignUpBlocks`; no migration).
    `Poll{Id, Title, AllowMultiple, Options []PollOption}`; `PollOption{Id, Label, Votes map[string]
    string}` — votes are keyed by responder (guest name / signed-in user-id hex) → display name, so a
    count is `len(Votes)` and the roster is its values, rendered straight from the event (no extra
    fetch, like RSVP). Supports **multiple polls per event** (venue *and* activity).
  - **Endpoints** (`routes/polls.go`, registered in `InitEvents`): `POST /events/:id/polls` (create —
    owner), `DELETE /events/:id/polls/:pollId` (delete — owner), `POST /events/:id/polls/:pollId/vote`
    (vote — members + guests). Owner gating via new `requireEventManager` (mirrors `scheduleEvent`:
    owner-only when the event has an owner; owner-less/guest events require a signed-in member on
    enforced instances). Voting reuses the shared `responderKey`. Pure, unit-tested helpers:
    `sanitizePollInput` (trim, dedupe, drop empty, require title + ≥2 options, cap at 20) and
    `applyPollVote` (validates option ids, enforces single-choice, replaces the voter's prior pick,
    empty ids = un-vote).
  - **Frontend**: `EventPolls.vue` — each poll shows its options with radio (single) / checkbox
    (multi) indicators, live counts, and a per-option voter roster; clicking votes (single-choice
    switches, re-click clears). Guests vote by name (shared field, same as RSVP). The **owner** gets
    an inline "Add poll" form (title, dynamic options, allow-multiple toggle) and a delete per poll.
    Mounted in `Event.vue` under the RSVP block; `EventService.createPoll/deletePoll/votePoll` persist
    then `refreshEvent`. The card hides entirely for non-owners when there are no polls.
  - **Tests**: `routes/polls_test.go` — pure (`sanitizePollInput` incl. dedupe/cap; `applyPollVote`
    single-replace / multi / invalid-option / single-choice-rejects-multiple / empty-clears) + DB-gated
    (owner creates → guest votes → persisted; non-owner create → 403; owner delete → removed).
  - **Swagger `docs/` regenerated** (pinned `swag@v1.16.1 --parseDependency --parseInternal`) — the 3
    poll routes + `models.Poll`/`PollOption`.
  - **Follow-ups (not v1):** no **edit** of an existing poll's title/options (delete + recreate before
    votes land; a proper edit that preserves votes on kept options is deferred); no result-close /
    winner-highlight; guest-name vote collisions share a key (accepted trust model, same as
    RSVP/comments).

- [x] **C7 · Per-gathering discussion thread / comments.** `M` — **DONE 2026-07-23 (CI-green;
  backend build/vet/tests + frontend build/lint/tests pass; post/edit/delete verified live).** A
  discussion thread on every event for coordinating details ("I'll bring cigars", "parking's out
  back"), keeping it off scattered group texts.
  - **Decisions (confirmed):** members **and** guests (by name, same trust model as RSVP/
    availability — guest posting stays open on enforced instances); **full** management (edit +
    delete-own, owner deletes any).
  - **Storage:** a dedicated `comments` collection (mirrors `eventResponses` — many-per-event,
    append-heavy), keyed by `eventId`; `models/comment.go` + `db/comments.go` + registered in
    `db/init.go`. `getEvent` attaches `event.Comments` (like it does `ResponsesMap`/`Attendees`),
    so the existing `refreshEvent()` surfaces them with no extra fetch.
  - **Endpoints** (`routes/comments.go`, registered in `InitEvents`): `POST …/comments`,
    `PUT …/comments/:id` (own-only), `DELETE …/comments/:id` (own OR event owner). Text trimmed +
    capped at 2000; empty → 400. Reused the guest/signed-in key helper — renamed `rsvpKey` →
    `responderKey` (generic) and shared it.
  - **Frontend:** `EventComments.vue` — thread with author/time/"edited", inline edit + delete
    controls on your own (delete also on any when you're the owner), and a composer (members post
    directly; guests enter a name first, like `GatheringRsvp`). Mounted below the calendar in
    `Event.vue`; `EventService.addComment/editComment/deleteComment` persist then `refreshEvent`.
  - **Tests:** `sanitizeCommentText` unit test + DB-gated integration (guest post→appears in
    getEvent; edit sets `updatedAt`; other-guest delete→403; owner deletes another's). Live-verified
    the full post/edit/delete/authz flow.
  - **Non-goal (v1):** no new-comment notifications (email/web-push) — follow-up tying into
    **[C2]**/**[C8]**. Optional later polish: enrich member comments with account avatars at read time.

- [ ] **C8 · Web push notifications for "new gathering" / "you were invited."** `M` —
  **DEFERRED 2026-07-23 (premise was wrong — reassess value before picking up).**
  **Correction:** the original note ("a service worker is already registered … client half partly
  there") is **false**. `git log` shows `f857320 "remove pwa"` (the SW/PWA was *deliberately
  removed*) then `e8deeee "Create kill-sw.js"` (a kill switch that *unregisters* the SW from clients).
  `main.js` registers nothing; `register-service-worker` is an unused dependency. So there is **no
  active service worker** — C8 would **reintroduce** one, reversing a deliberate decision (a bad SW
  can brick the app for all members — the likely reason it was pulled).
  - **If revived, do it safely:** a **push-only SW** (`push` + `notificationclick` handlers **only**,
    NO fetch interception / caching) to avoid the caching footgun that got the PWA removed.
  - **Needs infra:** a VAPID key pair — private key on the VM (`server/.env`, like
    `GMAIL_APP_PASSWORD`), public key baked into the frontend build. A Go webpush lib
    (e.g. `SherClockHolmes/webpush-go`) + a `pushSubscriptions` store + subscribe/unsubscribe routes.
  - **iOS gap:** Safari delivers web push only to home-screen-installed PWAs (iOS 16.4+) — most of the
    club's iPhone users won't get pushes unless they install the site. **[C2]'s email reminders already
    cover the "gathering is set" need for everyone incl. iOS**, which is why value here is now
    questionable. Reassess whether it's worth the SW risk before building.

- [x] **C9 · Sign-up-block capacity + waitlist.** `S` — **DONE 2026-07-23 (CI-green; backend
  build/vet/tests + frontend build/lint/tests pass; capacity+waitlist verified live).** The
  `SignUpBlock.Capacity` field was only *displayed* (client hid the join link when full) and **not
  enforced server-side**, and there was no waitlist. Now capacity is authoritative and overflow is
  waitlisted.
  - **Model** (`models/event.go`): added `WaitlistBlockIds []ObjectID` to `SignUpResponse`
    (`SignUpBlockIds` = confirmed, within capacity; `WaitlistBlockIds` = waitlisted). No migration.
  - **Enforcement** (`routes/event_responses.go`): new `assignSignUpBlocks(event, user, requested)`
    splits requested blocks into confirmed/waitlisted by each block's `Capacity` (nil = unlimited),
    excluding the user's own prior signup from the count and **preserving an already-confirmed spot
    on re-submit**. The sign-up branch of `updateEventResponse` now routes through it — a direct API
    call can no longer overfill a slot.
  - **Frontend**: `resetSignUpForm` (`ScheduleOverlap.vue`) populates a per-block `waitlist`;
    `handleSignUpBlockClick` now lets a **full** block be clicked (→ server waitlists);
    `SignUpBlock.vue` shows a "Waitlist" roster and the join link reads **"+ Join waitlist"** when
    full instead of disappearing. (The compact calendar-tile variant is joinable via the same
    handler; detailed waitlist lives in the list view.)
  - **Tests**: `TestAssignSignUpBlocks` (full→waitlist, unlimited→confirmed, already-confirmed keeps
    spot, fresh user on full block→waitlist). Live-verified: 3 guests → capacity-2 block → first two
    confirmed, third waitlisted.
  - **Follow-up (not v1):** auto-promotion — when a confirmed signup is removed, the earliest
    waitlisted user isn't auto-promoted (they get confirmed on their next re-submit, since a spot is
    now free). Proper promotion needs a signup timestamp/order on `SignUpResponse`; deferred.

### P3 — Nice-to-have / thematic

- [x] **C10 · Members-only gathering archive ("The Chronicle").** `M` — **DONE 2026-07-23 (CI-green;
  backend build/vet/tests + frontend build/lint/tests pass; auto-capture + member-gating verified live
  against a local server + Mongo).** An internal, roll-gated history of past gatherings (date, venue,
  attendees) — **auto-captured**, no manual upkeep. Resolves the C5/C10 tension flagged in C5: a
  recurring gathering rolls forward and discards its past occurrence, so the Chronicle is where that
  history is preserved.
  - **Capture engine** (`services/reminders`): the C5 gathering scheduler now, each tick, snapshots
    completed gatherings into a new `chronicle` collection. **Recurring** occurrences are captured in
    `advanceRecurringGatherings` **before** the roll-forward clears their RSVPs (one entry per
    occurrence); **one-off** gatherings are captured by a new `archivePastGatherings` pass once their
    time passes (guarded by a `Chronicled` flag on the event so they're not re-snapshotted — this also
    backfills already-past gatherings on first deploy). Runs email-independent (like C5's advance).
  - **Storage** (`models/chronicle.go` + `db/chronicle.go`): `ChronicleEntry{eventId, shortId, name,
    description, location, startDate, endDate, attendees []ChronicleAttendee, headCount, capturedAt}`
    in its own collection so history survives even if the source event is deleted. Attendees =
    going/maybe RSVPs (decliners excluded), headCount = Σ(1 + guestCount). A **unique index on
    (eventId, startDate)** + dup-key-as-no-op in `InsertChronicleEntry` makes capture idempotent under
    racing scheduler ticks / re-runs.
  - **Endpoint** (`routes/chronicle.go`, registered as `InitChronicle` in `main.go`): `GET /chronicle`
    behind `middleware.AuthRequired()` — **members only** (signed-in + allowlisted), most-recent-first.
    Kept off the `/events/:id` group to avoid the wildcard-vs-static route conflict.
  - **Frontend**: `Chronicle.vue` (role-gated like `Fellowship.vue` — non-members redirected home),
    a timeline of past gatherings (name → event link, date, venue, "N attended: …" roster). New
    `/chronicle` route + a "The Chronicle" item in `AuthUserMenu` (v-if `canInvite`, next to The
    Fellowship / The Roll).
  - **Tests**: pure (`chronicleAttendees` going/maybe/decliner/nameless/nil) + DB-gated
    (`archivePastGatherings` captures once + skips future + marks chronicled + no-dup + shows via
    `GetChronicleEntries`; recurring advance captures the occurrence before clearing RSVPs) + endpoint
    401-gating. **Swagger regenerated** (pinned `swag@v1.16.1 + flags`) — `/chronicle` + the models.
  - **Follow-ups (not v1):** no photos/notes per entry (the original "a photo or two" idea — would need
    upload/storage; the free-text venue + description carry over for now); no per-year grouping / paging
    beyond the 200-entry cap; RSVP-derived attendance only (pre-RSVP historical gatherings show
    "no attendance recorded").

- [x] **C11 · Printable / exportable roster of the Fellowship directory.** `S` — **DONE 2026-07-23
  (browser-verified; frontend-only).** Added an **Export** menu to `Fellowship.vue` with **Print /
  PDF** (opens a clean light serif print document — name/role/email/telephone + count + date — in a
  separate window so it doesn't fight the dark app theme; Save-as-PDF from the print dialog) and
  **Download CSV** (`The Fellowship Roster.csv`, quoted/escaped). Both operate on the
  currently-filtered roll (search + Show-guests) — export what you see. No backend change (reuses
  `GET /admin/allowlist`). `MemberAdmin.vue` left as-is (the same roll, admin-managed).

- [x] **C12 · Venue / location on an event.** `S–M` — **DONE 2026-07-23 (CI-green; backend
  build/vet/tests + frontend build/lint/tests pass; venue create/edit + .ics LOCATION verified
  live).** **Correction to the finding:** the existing `models/location.go` / `location_utils.js`
  are **IP-geolocation of the user** (country/city/lat-long), not a venue — and the Event had no
  location field. So this added a real venue field, not just wiring.
  - **Model** (`models/event.go`): `Location *string` on Event (free-text venue/address).
  - **Endpoints** (`routes/events.go`): `location` accepted in `createEvent` + `editEvent`
    (persists via the existing `$set: event` path).
  - **Surfaced everywhere a gathering appears:** `services/calendar/ics_generate.go` sets the .ics
    `LOCATION`; the C2 reminder email shows a "📍 venue" line linking to Google Maps; the schedule
    Google/Outlook calendar URLs pass `&location=`.
  - **Frontend**: new `EventLocation.vue` — inline-editable venue on the event page (mirrors
    `EventDescription`); everyone sees the venue + an **"open in Google Maps"** link, the owner can
    add/edit it. Mounted under the description in `Event.vue`.
  - **Design choice:** keyless — a free-text address + a plain `google.com/maps/search` link (no
    maps-provider API key, which this fork doesn't have). Tests: `.ics` `LOCATION` assertion.
    Live-verified: set at create → persists; edit via PUT → updates; `.ics` carries `LOCATION`.
  - **Follow-up (not v1):** an embedded **static-map image** needs a maps-provider API key
    (Google Static Maps / Mapbox) + config; add if a key is ever provisioned.

---

## Suggested sequencing

1. **P0 correctness** (A1–A4) — small, safe, removes silent-failure and crash-on-error footguns.
2. **A8 lint-as-warnings + A13** — cheap guardrails so the rest of the cleanup stays clean.
3. **A6 (split events.go) → B1/B2 (tests)** — reorganize the backend core, then lock it with tests.
4. **A5 (split ScheduleOverlap.vue) → B3** — the biggest frontend win; tackle in slices.
5. **Feature track in parallel:** C2 → C3 → C1 (reminder infra → universal calendar → RSVP) are the
   highest-leverage, lowest-new-infrastructure wins for an active club.
6. **2026-07-27 wave — COMPLETE through A16/B5/E9 (2026-07-28).** Landed in this order: **B4** →
   **E3** (all five phases) → **E5/E6** → **E4** → **E7** → **A18/A19/A20** → **E8/E11** →
   **A16** → **B5** → **E9**.
   Then **A17**, **B6** and **B7** — all closed 2026-07-28, four steps each — then **B8** (the OAuth
   refresh error type B7 filed), **E10** (misc hardening) and **E12** (derive the test-package list,
   filed by E10) the same day.
   **What's left is five items, and Parts A, B and E are done to P3.** Every P0/P1/P2 in the
   refactoring, testing and security tracks is closed. Remaining: **A21**, **A22**, **A23** (`P3`),
   **D2** (`L`/P3, infra-coupled rebranding — not a code-only change), and **C8**, which is `P2` but
   **deferred on a false premise** and needs its value reassessed before anyone picks it up, since
   it would reintroduce a service worker that was deliberately removed.
   Nothing remaining blocks anything else. Natural next pick is **A21** — it's the last item with a
   user-visible wrong answer (an unparseable all-day date silently becomes a year-0001 availability
   block) rather than a cleanup.

---

## PART D — Rebranding (remove all `schej-it` / `schej.it` and `timeful.app` references)

> **Supersedes the CLAUDE.md caveat** ("internal identifiers … still use the old name — leave them
> alone unless rebranding is the explicit task"). This item *is* that explicit task. Scope from a
> 2026-07-22 survey: **~290 `schej*` matches across ~50 files** (234 `schej.it`, 44 `schej-it`, plus
> bare `schej`) and **~69 `timeful*` matches**. Split into a safe/mechanical tier and a
> dangerous/infra tier — **do NOT treat this as one find-replace.**

- [x] **D0 · Decide the target name(s) first.** `S` · **P3 — DECIDED 2026-07-23.**
  - **Go module path:** `schej.it/server` → **`sirtom/server`**.
  - **Public domain:** **`gathering.sirthomasfoolery.com`** (final; for CORS/nginx/email links/ICS UIDs).
  - **Brand string** ("Schej.it"/"Timeful" shown to users) → **"The Fellowship"** (org) / **"The
    Gathering"** (the scheduling/event concept). Stray "Timeful(s)" in code comments → "gathering/event".
  - **`timeful.app` refs:** technical/URL (e.g. ICS UID `@timeful.app`) → `@gathering.sirthomasfoolery.com`;
    brand → The Fellowship/The Gathering.
  - **Contact email:** `contact@timeful.app` → **`sirthomasfoolery24@gmail.com`**.
  - **Remove** the upstream author's cal.com link in `TeamsNotReadyDialog.vue`.
  - **Mongo DB name `schej-it`** → **LEAVE as-is** (internal/invisible; renaming is a risky live-data
    migration for no user benefit). **GCP project id `schej-it`** → **LEAVE** (that Cloud Tasks path is
    dead on this fork). Both are D2 and intentionally not changed.

- [x] **D1 · Safe code/brand renames (mechanical, CI-gated).** `M` · **P3 — DONE 2026-07-23 (two
  commits; verified locally with Go 1.26.5 + eslint/build, both blocking-clean).**
  - **Go module path** `schej.it/server` → **`sirtom/server`**: `go.mod` module directive + the import
    prefix in **74 `.go` files** (the survey's "59" undercounted; the other machine had since added
    comments/waitlist/location routes). Isolated commit. `docs/` doesn't import the module path, so no
    swag regen was needed. (The `no local Go toolchain` caveat is stale — dev box now has Go 1.26.5, so
    this was `go build`/`vet`/`test`-verified locally before push, not just on CI.)
  - **User-facing brand + domain/URL**: OG event title, Swagger title, CORS default origins, email/
    event `baseUrl`, slackbot urls, ICS UID/ProductID, the Settings contact email
    (`sirthomasfoolery24@gmail.com`), removed the upstream cal.com link, dropped dead commented Timeful
    OG block in index.html, `package.json` name, maintenance page, stray code comments, and factual
    doc fixes (CLAUDE.md/DEVELOPMENT.md now say `sirtom/server`).
  - **Follow-ups since done:** the unused upstream `deploy_scripts/` + `deploy.yml` were **deleted**
    (see D2), and the root `README.md` was **rebranded** to The Fellowship (+ orphaned Timeful
    `hero.jpg`/`logo.svg` assets removed), both 2026-07-23.
  - **Intentionally LEFT (see D0/D2):** Mongo DB name `schej-it`, `SCHEJ_EMAIL_ADDRESS` env var, GCP
    project id in the dead Cloud Tasks code, Discord channel names, dead Stripe/paywall log strings.
    The only remaining `schej`/`timeful` hits are exactly these leaves + historical plan docs
    (REDESIGN_PLAN/ACCESS_CONTROL_PLAN, kept as history).

- [ ] **D2 · Dangerous / infra-coupled references (NOT a code-only change).** `L` · **P3**
  These are tied to live infrastructure and data — changing the string in code without the matching
  infra change will break prod. Each needs a coordinated migration, ideally done by the human with VM
  access:
  - **Mongo DB name `schej-it`** (`db/init.go`, `mongodump/mongorestore` commands in docs): renaming
    the database is a **data migration** (`mongodump` old → `mongorestore` into new name → cutover),
    not a code edit. Sequence with a deploy window.
  - **GCP Cloud Tasks project `schej-it`** (`services/gcloud/tasks.go`:
    `projects/schej-it/locations/us-central1/queues/SendReminderEmail`): this is a real **GCP project
    id**. It can only change if the project itself is renamed/recreated — coordinate with whoever owns
    the GCP project or leave as-is.
  - **Domains/CORS/nginx**: `main.go`'s default CORS origins — **DONE via A14/D1**
    (→ `gathering.sirthomasfoolery.com`). The `deploy_scripts/` + `.github/workflows/deploy.yml`
    ("Deploy Schej") were the **upstream's** screen-based auto-deploy to the old schej.it AWS VPS
    (`workflow_dispatch`-only, targets `secrets.SSH_HOST` this fork doesn't have) — **DELETED
    2026-07-23** rather than rewritten, since this fork deploys via `./deploy.sh` (docker compose +
    Caddy) on the VM. So D2's domain/nginx tail is resolved. **Still open (intentionally):** Mongo DB
    name `schej-it` (data migration) + GCP project id (dead code) — both LEFT per D0.

- [x] **D3 · Historical migration scripts — leave or annotate, don't rename.** `S` · **P3 — DONE
  2026-07-23 (resolved via A15).** Decision: **leave** the `schej` identifiers in `server/scripts/*` —
  they intentionally **don't compile** (reference outdated models; excluded from CI per
  `backend-ci.yml`) and are run-once history, so renaming is pointless and risks implying they're live.
  The "annotate" side is already satisfied by **A15**'s `server/scripts/README.md`, which documents that
  each dated folder is a manual, run-once migration kept for history. No code change needed here.

---

## PART E — Security & Access-Control follow-ups

> Companion doc: `ACCESS_CONTROL_PLAN.md`. E1–E2 came out of the 2026-07-23 live RSVP
> verification (see [C1]); none is a regression in the RSVP feature itself.
> **E3–E10 come from the 2026-07-27 full review.** E3 is the product change of record and
> supersedes E1's "RSVPs stay guest-open" carve-out.

- [x] **E1 · Gate `createEvent` / `scheduleEvent` behind auth on enforced invite-only instances.**
  `S–M` · **P2 · DONE 2026-07-23 (decision: gate via the existing `INVITE_ONLY_ENFORCED` flag).**
  Closes the anonymous-write surface without a new config knob or breaking guest flows:
  - `db.AccessControlEnforced()` exported (was private `accessControlEnforced`).
  - New `middleware.AuthRequiredIfInviteOnly()` — delegates to `AuthRequired` when
    `INVITE_ONLY_ENFORCED` is on, else passes through. Applied to `POST /events` (createEvent).
  - `scheduleEvent`: the owner-less-event branch now requires a signed-in caller when enforced
    (the existing owner-check already covers owned events).
  - **Verified live (enforced=true):** anonymous `POST /events` → 401; anonymous schedule of an
    owner-less event → 401; guest availability/RSVP endpoints still reachable (404, not 401).
    Not-enforced (dev/open) preserves the guest create/schedule path. Tests:
    `TestAuthRequiredIfInviteOnly_{NotEnforcedPassesAnonymous,EnforcedBlocksAnonymous}` (DB-free)
    + existing `TestCreateEvent_GuestForbidden` (signed-in guest → 403, unchanged). `.env.template`
    documents the expanded flag semantics.
  - **Left open (intentional, per decision):** RSVP `POST/DELETE …/rsvp` stay guest-open by design;
    if abuse becomes a concern, prefer rate-limiting / a per-event toggle over blanket auth.

  <details><summary>Original finding (for history)</summary>

  · **P2 · OPEN — needs discussion before any change.**
  **Finding:** the invite-only allowlist is enforced *inside* `middleware.AuthRequired()`, which is
  applied **per-route**. `POST /events` (create), `POST /events/:id/schedule`, and the RSVP endpoints
  are **not** behind it, so they're reachable by an **unauthenticated** caller who can hit the API. In
  the 2026-07-23 verification this was used deliberately (the guest path is *supposed* to be open — it
  mirrors guest availability responses), but it also means an anonymous party who reaches the API can
  **create and schedule arbitrary events**. Not exploitable for data disclosure (no member data is
  exposed), but it is an unauthenticated write surface.
  - **Why it's not a simple flip:** guest, no-account interaction is a genuine product requirement
    for this club (guests RSVP and respond to availability by name). Locking *all* of these behind
    `AuthRequired` would break the guest flows. The real question is which **writes** should require a
    member session vs. which must stay open, e.g.:
    - `createEvent` — does an anonymous visitor ever legitimately create an event on this private
      instance? If not, gate it (members create; guests only *respond* to existing events). Watch the
      existing **guest-created event** path (`ownerId == 0`) — some flows rely on it; confirm none are
      user-facing on this fork before gating.
    - `scheduleEvent` — already **owner-gated when the event has an owner**; the gap is
      **owner-less (guest-created) events**, where anyone can schedule. Likely fine to require auth
      unconditionally (only an owner should lock in a time), but confirm against the guest-event UX.
    - RSVP `POST/DELETE …/rsvp` — intentionally open (guest RSVP by name). Leave open; if abuse is a
      concern, prefer rate-limiting / a per-event toggle over blanket auth.
  - **Decide & discuss:** whether to (a) leave as-is (guest-open by design), (b) gate `createEvent`
    +`scheduleEvent` for owner-less events behind `AuthRequired`, or (c) add a config flag
    (`GUEST_EVENT_CREATION_ENABLED`) defaulting to off for invite-only instances. No code change until
    this is settled.

  *(Resolved with option (b), scoped to enforced instances via the existing flag — see above.)*
  </details>

- [x] **E2 · `deleteEvent` only accepts the Mongo `_id`, not the short id.** `S` · **P3 — DONE
  2026-07-23 (backend build/vet + full suite green).** `deleteEvent` now resolves via
  `db.GetEventByEitherId` up front and drives every DB op (responses lookup + the ownerId-scoped
  delete/soft-delete + folder cleanup) off the resolved `_id`; unknown id now **404**s instead of
  400/500. DB-gated tests added (`events_delete_db_test.go`): delete-by-short-id → 200 + gone;
  unknown id → 404.
  <details><summary>original finding</summary>

  Pre-existing, not RSVP-related: `DELETE /events/:eventId` called `primitive.ObjectIDFromHex(eventId)`
  directly, so a **short id** returned **400** (every other event route uses `db.GetEventByEitherId`).
  The real UI always deletes by `_id`, so no user-facing bug — but an inconsistency / sharp edge for
  API scripting. (Surfaced when API-cleaning up an RSVP test event by short id fell back to a direct
  Mongo delete.)
  </details>

- [x] **E3 · Require sign-in (minimum role `guest`) for ALL event access; remove the anonymous
  guest flow.** `L` · **P0 — the 2026-07-27 product change of record.**
  **Decisions (user, 2026-07-27):** anonymous visitors see **only the landing page** (+ sign-in /
  auth / privacy). This **reverses** `ACCESS_CONTROL_PLAN.md` §1 ("Guest (no-login) event responses:
  LEFT OPEN") and E1's "RSVPs stay guest-open" carve-out. Confirmed choices:
  - **On-behalf guest entry KEPT, behind sign-in** — a signed-in user can still enter availability
    under a plain name (e.g. a spouse without an account) via RespondentsList's "Add guest
    availability". Only *anonymous* use is removed; name-spoofing holes get closed.
  - **ICS feed stays open** (`GET /api/events/:id/ics`) — calendar apps fetch without cookies; it
    only exposes the scheduled name/time/venue, and obtaining the URL now requires sign-in. Mark it
    in code as the one deliberate exception. (Signed-token URLs = optional future hardening.)
  - **Legacy name-keyed guest data KEPT, rendered read-only** — existing responses / `Rsvps` /
    poll `Votes` whose key is a display name (not a 24-hex ObjectID) still display (precedent:
    `Comment.IsGuest`); no migration. They age out as events archive.
  - **Stripe is NOT part of this item** — logged separately as E4.

  **Subsumes / closes these review findings** (fixed by the phases below): guest-name-as-ObjectID
  overwrites a member's response (`event_responses.go:223,253`); anonymous delete-any-response
  (guest branch `:441-455` has zero auth — `{"guest":true,"name":""}` deletes the first member row);
  RSVP/vote spoofing via `responderKey` (`:724-741` → `events.go:811`, `polls.go:92`);
  unauthenticated `rename-user` (`:515-547`) and `userResponded` (`:557-626` — anyone can silence
  reminders for any email); ownerless-event takeover (ownership check `events.go:277-282` only runs
  when `OwnerId != Nil`, and anonymous creation `events.go:139` mints such events); event-name leak
  to crawlers via OG tags; unanchored `/e/` regex in NoRoute; `chronicle` missing from the router
  `authRoutes`; `canEdit` granting anonymous edit of ownerless events (`Event.vue:347-351`); lost
  `?redirect` params on sign-in (`router/index.js:98-99`); event enumeration via unauthenticated
  `GET /:eventId/ids` + predictable short ids (E9 covers the generator itself).

  **DONE 2026-07-28 — all five phases.** Deployed in two units: phases 1–3 together (`3dc85e56`,
  + hotfix `9482e489`), then phase 4 (`9dc24518`).

  **⚠️ Phase 3 caused a production outage — read this before touching the router guard.**
  Deploying phases 1–3 made the site unreachable for anyone WITHOUT a session: `/` and `/sign-in`
  ping-ponged and neither rendered, so nobody could even log in. Cause: the guard probes
  `GET /user/profile` to decide whether anyone is signed in; for a signed-out visitor that 401s with
  `not-signed-in`, which is the *expected answer* — but it also matched the new central
  session-ended handler, which pushed to `/sign-in` while `beforeEach` was still resolving,
  cancelling the navigation and re-running the guard forever. The handler's `publicRoutes` bail-out
  couldn't save it either: before the first navigation resolves, `router.currentRoute` is Vue
  Router's placeholder with `name: null`, so even the landing page didn't match.
  **Fix** (`9482e489`): the auth probe is exempt from the central handler (the guard already handles
  that failure in its own try/catch), and the handler refuses to push before the first navigation
  resolves. Three regression tests in `fetch_utils.test.js`.
  **Why local testing missed it:** it only reproduces on a **cold load with no session cookie**. A
  dev browser that is already signed in never enters that state. → **Any change touching the router
  guard, `fetch_utils`' error path, or auth-dependent rendering needs a signed-out pass in a fresh
  browser profile.** A headless harness for exactly this now exists (see the note under phase 4);
  it was validated by reverting the fix and confirming all five routes fail, then restoring.

  **PROGRESS 2026-07-28 — phases 1–3 DONE (deployable as a unit), 4–5 remaining.**
  - **Phase 1 DONE** (`79c14890`): `InitEvents` registers `GET /:eventId/ics` bare and everything
    else in an `AuthRequired()` sub-group. The three divergent ownership checks (`editEvent`,
    `scheduleEvent`, `polls.requireEventManager`) unified on one helper — ownerless events are now
    member+ only, closing the takeover. `createEvent` takes its owner from the session
    unconditionally. `AuthRequiredIfInviteOnly` retired with its two tests. Table-driven gate test
    drives the REAL `InitEvents` and cross-checks against the router's own route list, so a route
    added outside the authed group fails the build instead of shipping open.
    *Two pre-existing test problems fixed en route:* `logger.StdOut/StdErr` were nil in tests (any
    handler error path that logged would nil-deref) — `routes` `TestMain` now inits to `io.Discard`,
    protecting the whole package; and several tests registered bare handlers that now need
    `AuthRequired`, so they drive the real chain via `insertTestUser`.
  - **Phase 2 DONE** (`4bb20702`): every hole listed below closed. On-behalf entry survives but
    rejects ObjectID-shaped names; guest-branch `deleteEventResponse` + `renameUser` are owner/admin+
    (both previously had NO authorization); RSVP and poll votes are session-keyed with identity from
    the account, never the body; `?guestName=` gone (with it the acknowledged blind-availability
    incognito bypass); the `/e/:id` OG-title lookup gone; `eventsToLink` gone.
  - **Phase 3 DONE** (`65f1d6df`): router guard inverted to `publicRoutes`, so new routes are gated
    by default (this fixes the `chronicle` gap by construction). Deep links round-trip through
    `?redirect`, same-origin only — unit-tested against protocol-relative and `javascript:`/`data:`
    forms. Central 401 handler in `fetch_utils` for mid-session revocation, wired as a registration
    hook to avoid the `fetch_utils → router → store → fetch_utils` cycle. **[A20] folded in here.**
    *Verified against a locally-run server:* anonymous curl over all 20 event routes → 401 on every
    one, ICS 404s; `/e/<id>` and `/e/<shortId>` serve the static title with the event name absent
    from the shell (confirmed the leak was real first — the template reads `{{ or .title … }}`).
  - **Phase 4 DONE** (`9dc24518`, frontend-only + one server fix; **-554 lines**): removed the
    localStorage guest identity and every reader, the `?guestName=` fetch parameters, the guest name
    fields in RSVP/polls/sign-up, the unreachable `!authUser` arms in EventHeader/EventBottomBar,
    `eventsToLink`/`eventsCreated` + helpers, and anonymous `canCreateEvents`. **Kept on-behalf
    entry** (GuestDialog survives, reworded, and now mirrors the server's ObjectID-shaped-name
    rejection client-side). Unified the two no-owner sentinels behind `isOwnerlessEvent`
    (`ownerId == guestUserId` ×3 and `ownerId == 0` ×2, the latter working only via JS coercion),
    and corrected the NewEvent alert still claiming "anybody can edit this event".
    **Server fix — an inconsistency introduced in phase 2:** `requireResponseManager` compared
    `user.Id` to `event.OwnerId` directly, locking members out of legacy **ownerless** events (a nil
    OwnerId never equals a real id) even though those same members can edit the event. It delegates
    to `requireEventManager` + admin override now, so "who manages this event" has one definition;
    3 tests. `canEditGuestName` in the UI mirrors it — it returned `true` unconditionally, so a
    non-owner saw the rename pencil and got a 403 on submit.
    **Verified in a real browser both signed-out and signed-in** (see the outage note above for why
    that matters), including an RSVP round-trip: body is exactly `{"status":"going","guestCount":0}`
    and it persists keyed by user id with name/email backfilled from the account. That pass caught a
    regression before commit — removing the fields from `SignUpForSlotDialog` left a `v-form` whose
    lazy-validation `formValid` never became true, which would have permanently disabled "Join slot".
  - **Phase 5 DONE** (folded into phase 4): `ACCESS_CONTROL_PLAN.md` §1 records the reversal with
    what "left open" turned out to mean in practice; `PLUGIN_API_README.md` carries a
    breaking-change notice (the plugin acts as the signed-in user; `guestName`/`guestEmail` are
    ignored). Grep sweep clean — the only surviving hits are the sentinel definition, an unrelated
    monthly event counter, and the `db.GuestNameExists`/`UpdateGuestResponseName` helpers behind the
    now-owner-gated rename.
  - **Test harnesses — landed in-repo** (`7f345e22`), documented in `DEVELOPMENT.md`:
    `frontend/scripts/check-signed-out.js` (cookie-less Chrome over the public/gated routes,
    asserting both where you land *and* that the destination rendered — run it after **any** change
    to the router guard, `fetch_utils`' error path, or auth-dependent rendering) and
    `check-signed-in.js`, backed by `server/tools/mintsession` (mints a session cookie so the
    signed-in UI can be driven without SMTP or Google OAuth). Not in CI — they need a built
    frontend + running server + Mongo — but both exit non-zero, so wiring them up stays open.

  **Phase 1 — backend gating** (`server/routes/events.go:26-57`): register `GET /:eventId/ics`
  bare; every other event route goes in an `AuthRequired()` sub-group
  (`authed := eventRouter.Group("", middleware.AuthRequired())`) — includes `POST ""` (create),
  edit, get, ids, responses, response CRUD, rename-user, **responded** (reminder-email links pass
  through the SPA sign-in redirect, so the flow survives), schedule, rsvp, polls; drop the
  now-redundant per-route `AuthRequired()` on delete/duplicate/archive/import/comments.
  **Retire `AuthRequiredIfInviteOnly`** (`middleware/auth.go:57-72`, sole prod use is create) + its
  two tests, same commit. `createEvent` takes the owner from the session unconditionally
  (`events.go:139` nil-owner path dies). Legacy ownerless events: manageable by **member+** only
  (`user.EffectiveRole().CanInvite()`) — unify `editEvent` (`events.go:277-282`), `scheduleEvent`
  (`events.go:878-891`), `requireEventManager` (`polls.go:105-121`) on one shared helper.

  **Phase 2 — remove anonymous guest semantics + close the holes**
  (`event_responses.go`, `polls.go`, `events.go`, `main.go`, `auth.go`):
  - `updateEventResponse`: keep the `guest:true` branch as *authenticated on-behalf* entry, but
    reject names matching `^[0-9a-fA-F]{24}$` (and empty/whitespace) — closes the member-overwrite
    IDOR. Same validation in the sign-up-form branch.
  - Guest-branch `deleteEventResponse` (`:441-455`) and `renameUser` (`:515-547`): event owner (or
    admin+) only — mirrors the member-branch ownership rule at `:466-470`.
  - `responderKey` (`:724-741`): drop the anonymous path — RSVP (`rsvpToEvent`/`deleteRsvp`) and
    `votePoll` become session-keyed only; remove `guest`/`name` from those payloads. Legacy
    name-keyed entries still render but can no longer be created or spoofed.
  - Remove the `?guestName=` identity/filter params (`events.go:474,545-556`,
    `event_responses.go:97,138-144`, `filterResponsesForGuest`) — the blind-availability incognito
    bypass acknowledged in-code at `event_responses.go:192-209` dies with them.
  - `main.go:225-263` NoRoute: delete the `/e/:eventId` regex + `db.GetEventByEitherId` OG-title
    lookup; serve the static default title (event names must not leak to anonymous crawlers, and
    per-event OG titles are pointless behind a login).
  - `routes/auth.go`: remove `EventsToLink` claim logic (anonymous creation no longer exists).

  **Phase 3 — frontend gating + deep-link return**
  (`frontend/src/router/index.js:92-110`, `SignIn.vue`, `fetch_utils.js`):
  - Invert the guard to `publicRoutes = ["landing","sign-in","sign-up","auth","privacy-policy",
    "404"]`, everything else auth-required by default (fixes the `chronicle` gap by construction;
    new routes become secure-by-default). Unauthed hit on a gated route →
    `/sign-in?redirect=<fullPath>`.
  - `SignIn.vue handlePostAuthRedirect` (:392-409): after the existing branches, honor a
    same-origin `redirect` (starts with `/`, not `//`) so a shared `/e/:id` link round-trips
    through OTP login back to the event. Already-signed-in users hitting sign-in keep `redirect`
    too (fixes the dropped-params bug at `router/index.js:98-99`).
  - `fetch_utils.js`: central 401 handling (`not-signed-in` / `user-does-not-exist` /
    `not-invited` → clear `authUser`, redirect to sign-in with `redirect`; skip on public routes).
    This is the scoped version of the interceptor A10 deferred. A mid-session strike-off
    (`middleware/auth.go:41-48`) currently leaves the SPA in a stale signed-in state.

  **Phase 4 — remove anonymous guest UI** (`frontend/src/`):
  - `Event.vue`: anonymous GuestDialog trigger (:707), `saveChangesAsGuest` (:726-739, submit
    :922-924); fix `canEdit` (:347-351) — drop `ownerId == 0` anonymous-edit, ownerless-edit for
    member+ to match Phase 1 (note the TWO no-owner sentinels: `0` and `constants.js:200
    guestUserId` — unify).
  - `currentAvailabilityMixin.js` (:220-273, :317-348): remove anonymous branches; `{guest:true}`
    submission survives only for the signed-in on-behalf path. `RespondentsList.vue` (:220-241,
    :313-332) "Add guest availability" stays; owner-gate its delete/rename in UI to match backend.
  - Remove anonymous name fields/prompts: `GatheringRsvp.vue:16-22,190-205`,
    `EventPolls.vue:10-17,193-198,254-256`, `SignUpForSlotDialog.vue:10,28,113`,
    `EventHeader.vue:79-91`, `EventBottomBar.vue:19`; all `localStorage["<eventId>.guestName"]`
    reads/writes and `guestName` query params; `pluginMessagesMixin.js:85` `forceGuestMode`.
  - `store/role_getters.js:30-35`: `canCreateEvents` no longer true for anonymous (that was
    deliberate — flip it + the comment + `role_getters.test.js`).
  - `Auth.vue:44-51`: remove `eventsToLink` / `localStorage.eventsCreated` (+ `getEventsCreated`
    helper). `GuestDialog.vue` itself stays (on-behalf use); reword copy.

  **Phase 5 — docs + sweep:** update `ACCESS_CONTROL_PLAN.md` §1 (record the reversal + date);
  `PLUGIN_API_README.md` — the `guestName`-forces-guest-mode capability is gone; grep sweep for
  `guestName` / `guest: true` / `guestUserId` / `eventsCreated` stragglers (keep `guestUserId`
  only where legacy rendering needs it).

  **Tests:** table-driven "anonymous → 401" over every gated event route (pattern:
  `routes/access_control_test.go` test-login helper + allowlisted users per `comments_test.go`);
  ICS stays 200/404 without a session; hex-guest-name rejection; owner-gated guest delete/rename;
  RSVP/vote ignore spoofed names; frontend `role_getters.test.js` + `fetch_utils.test.js`.
  **Verification:** backend `MONGODB_URI=… go test ./models/ ./routes/ ./utils/ ./db/
  ./services/reminders/`; frontend `npm run test:unit` + build; curl matrix (anonymous 401
  everywhere except ICS; `view-source:/e/:id` shows the generic title); manual: anonymous `/e/:id`
  → sign-in → lands back on the event; signed-in `guest`-role user can view/respond/RSVP/vote but
  not create; legacy name-keyed data still displays; on-behalf entry works, 24-hex names rejected.
  **Deploy note:** land as ~5 green commits (one per phase); between Phase 1 and Phase 3 anonymous
  visitors get raw 401s instead of a redirect — keep those commits/deploys close together.

- [x] **E4 · Delete the Stripe/paywall subsystem.** `M` · **P1 — DONE 2026-07-28** (backend
  build/vet + full suite green; frontend eslint 0 errors, 80/80 tests, build OK). **-696 lines.**
  The anonymously-reachable nil deref was confirmed exactly as described (`stripe.go:214`
  `cs, _ := session.Get(...)` → unconditional `cs.PaymentStatus`, on an unauthenticated route, and
  with no API key the `Get` always errors — `gin.Recovery` made it a free 500 rather than a crash).
  - **Backend:** `routes/stripe.go`, the `main.go` wiring (`InitStripe` + `stripe.Key`), the
    `stripe-go/v82` dep, `db.GetUserByStripeCustomerId` (only callers were in `stripe.go`), and
    `User.StripeCustomerId` / `User.IsPremium`. Dropping those fields also retired
    `GET /users/:userId/is-premium`, whose entire implementation was `StripeCustomerId != nil`.
    Existing Mongo docs keep the fields; nothing reads them. **Swagger regenerated.**
  - **Frontend:** checkout plumbing in `SignIn.vue`/`Auth.vue` (`authTypes.UPGRADE`), the
    `isPremiumUser` util + store getter, `upgradeDialogTypes`, `numFreeEvents`, and `Event.vue`'s
    `ownerIsPremium` — including `checkOwnerPremium()`, which fired a `/users/:id/is-premium`
    request on **every event page load**. Every surviving `isPremiumUser` reference turned out to be
    inert (unused import / unused `mapGetters` / inside a comment). Also removed `ToolRow.vue`'s
    commented AdSense block and its orphaned `initializeAd()`.
  - **TODO was stale on two points:** the `StripeRedirect` route/view and the `enablePaywall` store
    flag were already gone from an earlier pass.
  - Resolves **A9**'s deferred `isPremiumUser` question.

- [x] **E5 · `session.Save()` errors discarded at 5 sites.** `S` · **P1 — DONE 2026-07-28**
  (build/vet clean, full suite green with DB-gated tests actually running). Handled by impact
  rather than uniformly:
  - `middleware/auth.go` (struck-off-member revocation) — **log**. The request is denied either
    way; the risk is that an unpersisted `Delete` leaves the revoked cookie live for the *next*
    request, which must not be silent. Changing the response here would be wrong (it's already 401).
  - `auth.go signInHelper` — **propagates** (`return models.User{}, err`); it already returns an
    error, so no signature change.
  - `auth.go` OTP sign-in and `signOut` — **500**. Both previously returned 200 while the cookie
    state was the opposite of what the response claimed.
  - `user.go deleteAccount` — **log only**: the account row is already gone, so `AuthRequired`
    401s the stale cookie on its next use.
  Knocks 5 off the errcheck backlog **B5** tracks.

- [x] **E6 · PII leaks in `getEvent` to non-owners.** `S` · **P1 — DONE 2026-07-28** (build/vet
  clean, full suite green). All three channels confirmed present and fixed:
  - `stripSensitiveUserFields` now also clears **`Phone`** and **`Role`** — unconditionally, since
    an event response only needs to identify the respondent. Checked first that nothing on the
    frontend reads either *from event responses*: the directory reads `member.phone` from
    `/admin/allowlist`, `Settings` reads `authUser.phone`, and `role` is only consumed for the
    signed-in `authUser` (`store/role_getters.js`) and in `MemberAdmin`.
  - **`Rsvps`**: `Rsvp.Email` now follows the same `showEmails` (`isOwner && collectEmails`) rule as
    responses. The RSVP itself still serializes — `GatheringRsvp.vue` renders status/name/guestCount
    and never reads `.email`.
  - **`Remindees`**: now owner-only (`event.Remindees = nil` otherwise). The owner still needs it —
    `NewEvent.vue:565` prefills the invite field from it when editing. The json tag has no
    `omitempty`, so non-owners get an explicit `null`, which that call site already handles.
  - **Tests**: `routes/events_pii_db_test.go` — DB-gated matrix over non-owner / anonymous / owner
    with `collectEmails` / owner without, plus the phone+role strip. **5 of the 6 fail without the
    change** (verified by stashing the fix); the 6th is deliberately green both ways as the guard
    against over-stripping the owner's own view.

- [x] **E7 · Slackbot endpoint: no Slack signature verification (or just delete it).** `S` ·
  **P2 — DONE 2026-07-28 (deleted).** Finding confirmed: `execCommand` was unauthenticated, did no
  signing-secret verification, and handed the caller-supplied `payload.ResponseUrl` straight to
  `command.Execute`, which POSTs to it. Took the recommended option and **deleted** rather than
  hardened — after **E4**, the outbound `SendTextMessageWithType` had zero callers, and the two
  remaining ones (event-created, user-joined) were already commented out.
  `discord_bot/` went with it (the A18 half): `Init()` was never called from `main.go` and it had
  zero references outside its own package, so it has never run on this fork. `go mod tidy` dropped
  `discordgo`. Also removed the dangling commented call sites and their leftover scaffolding, the
  `SLACK_*`/`DISCORD_*` entries in `.env.template` + `DEPLOYMENT.md`, and the CLAUDE.md references.
  **Swagger regenerated.**

- [x] **E8 · Input caps on event creation.** `S` · **P2 — DONE 2026-07-28** (build/vet clean, full
  suite green). Done together with **E11** — same payload, same two handlers. New shared helpers in
  `routes/events.go` (`validateEventPayload` / `sanitizeEventText`), called from both `createEvent`
  and `editEvent` so one rule set covers both.
  - **Rejected with 400** (`errs.PayloadTooLarge`): `Dates` > 366, `Times` > 366, `Remindees` > 200,
    `SignUpBlocks` > 200. Cardinality is rejected rather than truncated on purpose — silently
    dropping dates would store a subtly-wrong event that looks fine to the organizer.
  - **Truncated**: `Name` → 200 runes, `Description` → 2,000 runes (matching the poll-title and
    comment caps), and guest/on-behalf display names → 100 runes via `sanitizeResponderName`, applied
    in both `updateEventResponse` and `responderKey` (so RSVP/vote/comment paths are covered too).
    Free text is truncated because an over-long name is harmless once bounded.
  - Caps are deliberately generous — the frontend builds one date per selected day (DOW tops out at
    7) and never sends `times` at all, so these only bite on direct API use. A test asserts that a
    payload sitting **exactly** at each cap still passes.
  - Truncation is **rune-aware** (`truncateRunes`), so a cut can't land mid-character. That's the
    same defect **A17** tracks for comments/polls — this establishes the helper A17 can reuse.

- [x] **E9 · Short event IDs: predictable generator + collision fallback returns the duplicate.**
  `S` · **P2 — DONE 2026-07-28** (0 lint issues, build/vet + full suite green on both an empty
  Mongo and a restored prod replica).
  All three parts of the finding confirmed. The same-second collision is not theoretical —
  running the old generator twice against one Unix second returns the identical id (`8aEF4`
  both times, `feAf6` a second later), because `rand.NewSource(eventId.Timestamp().Unix())` is
  seeded at one-second granularity and two events created in the same second draw the same
  sequence.
  Now: `crypto/rand` with **rejection sampling** — `% 20` over a 256-value byte would have made
  the first 16 letters of the alphabet measurably likelier than the last 4 — probe errors
  propagated instead of read as "no collision", and a hard error after 10 attempts rather than
  returning an id it knows is taken. `GenerateShortEventId` drops its `eventId` argument (it was
  only ever the seed) and returns `(string, error)`; the three call sites 500 on failure.
  Four tests: no repeat across 50 draws, shape and alphabet, every letter reachable across 2,000
  drawn characters (the bias check), and that a planted id is skipped — with an assertion that the
  planted id is findable, so the collision probe can't pass vacuously.

- [x] **E10 · Misc hardening batch.** `S` · **P3 — DONE 2026-07-28** (build/vet/lint clean, full
  suite green, race-clean, routes+reminders run 12× for the known flake).
  All five sub-items resolved; two had already been closed by later work.
  - **`CORS_ORIGINS` split on `,` without trimming** — `"a.com, b.com"` silently yielded a
    never-matching `" b.com"` origin. Extracted `parseCorsOrigins` in `main.go`: trims each entry
    and drops empties. The empty-entry handling also fixes the *fallback* — a whitespace-only or
    trailing-comma value used to produce a garbage origin list where it should fall through to the
    defaults, so that check is now `len(origins) == 0` rather than `raw == ""`. Worth knowing why
    this one was invisible: a CORS rejection surfaces only in the browser console, and nothing
    server-side logs it.
  - ~~`db/allowlist.go` `GetAllowlist` ignores the `cursor.All` error~~ — **DONE 2026-07-28** in
    **B5**: it returns `([]AllowlistEntry, error)` now, logs both the `Find` and `cursor.All`
    failures, and its one caller (`routes/admin.go` `getAllowlist`) 500s instead of rendering a
    broken query as "the Fellowship has no members".
  - ~~`utils/utils.go:212`: bare `panic(err)` on base64 decode~~ — **already gone**, closed by
    **B6/B7** when encryption moved out of `utils` into the `encryption` package. The helper is
    `encryption.go:58` now and returns `([]byte, error)`; its caller wraps it as `ciphertext is not
    valid base64: %w` (`:109`). Verified there is no remaining `panic(` anywhere outside
    `server/scripts/`.
  - **`deleteEvent` / `archiveEvent` returned 500 to a non-owner.** Both used `ownerId: user.Id` as
    a Mongo *filter* rather than an authorization check, so a non-owner matched no document and
    `Decode` returned `ErrNoDocuments` — reported as `internal-error`. Both now resolve the event
    first and call the existing `requireEventManager` (403 `user-not-event-owner`), and a genuinely
    missing document 404s via `errors.Is(err, mongo.ErrNoDocuments)`. `archiveEvent` also resolved
    its id with `ObjectIDFromHex` directly, so short ids 400'd — it uses `GetEventByEitherId` now,
    the same sharp edge **E2** fixed for delete.

    **Third thing the filter was hiding:** a legacy ownerless event (created before **E3** removed
    anonymous creation) had `ownerId = NilObjectID`, which no user's id can equal — so those events
    were undeletable and unarchivable by *anyone*, always 500. `requireEventManager` gives them the
    member-or-above rule the edit and schedule routes already use. Tested from both sides: a member
    can now delete one, a guest still gets 403, so the ownerless path didn't become a hole.

    **Prod data follow-up, 2026-07-28 — the last ownerless event now has an owner.** Prod held
    exactly one: **The Inaugural Gathering** (`_id 6a36a3c9f875bc934af9c62f`, shortId `FE2dd`), 1 of
    16, and its `ownerId` field was *missing* rather than `NilObjectID` — Go unmarshals both to the
    zero value, so it took the ownerless branch either way. The effect after this fix was backwards:
    the one event nobody would want casually deleted was the only one any member could delete, and
    the frontend agreed — `canEdit` (`frontend/src/views/Event.vue:346`) mirrors the server rule, so
    members actually *saw* the edit/delete controls on it. Set `ownerId` to the superAdmin
    (`6a360308f1c73bc282cced51`) with a one-document `updateOne`; the collection is now 16/16
    `objectId` and the event still resolves by both `_id` and short id (`GET /api/events/FE2dd` →
    401 unauthenticated, the expected **E3** gate, not a 500). Rollback is `$unset: {ownerId: ""}`,
    which restores the exact prior shape.

    No code changed, and deliberately so: the ownerless branch of `requireEventManager` now matches
    no document in prod, but it stays as correct handling for a shape that could reappear (a
    restored old backup, a migration). Deleting it would reintroduce the always-500 bug above.
  - **`utils/ratelimit.go` janitor had no stop.** Added `Stop()` (a stop channel closed under
    `sync.Once`, so it's idempotent) and `defer ticker.Stop()`; the janitor now selects on both.
    Not hypothetical — `ratelimit_test.go` already built five limiters, each leaking a goroutine
    and a ticker for the rest of the run; all five now `defer rl.Stop()`. Also split the sweep body
    out as `evictStale(age)` so eviction is testable without waiting on a 10-minute ticker.

  **Tests (13 new).** Six DB-gated handler tests in `routes/events_archive_db_test.go` — short-id
  archive, non-owner 403 on both handlers, 404, and the ownerless member/guest pair — each
  asserting the *side effect* too (a refused delete must leave the event present; a refused archive
  must leave it unarchived), since a handler returning 403 while still writing would otherwise
  pass. Confirmed all six fail against the pre-fix handlers with exactly the reported symptoms
  (`got 500, want 403`; `got 400, want 200`). Seven table cases for `parseCorsOrigins`, plus
  `Stop`-halts-the-janitor (goroutine count) and `evictStale`.

  **`.` (the main package) was not in the CI test list**, so `parseCorsOrigins` coverage wouldn't
  have run — the third instance of the **B4** gap in three items (B8 hit it with `services/auth`).
  Added to `backend-ci.yml` and the three `DEVELOPMENT.md` lists. **This keeps recurring: a single
  source for the package list is worth doing** — filed as **E12**.

- [x] **E11 · `createEvent` / `editEvent` accept an unvalidated `EventType`.** `S` · **P2 — DONE
  2026-07-28** (build/vet clean, full suite green). Confirmed exactly as reported. Added
  `models.IsKnownEventType` mirroring `models.IsKnownRole`, and both handlers now reject an unknown
  type with 400 `errs.InvalidEventType` via the shared `validateEventPayload` (see **E8**).
  Covered by unit tests over the enum (including the camelCase `"specificDates"` that surfaced it)
  **plus two DB-gated end-to-end tests** driving the real `createEvent` — the pure tests can't catch
  a handler that forgets to call the validator, and one of the pair asserts the valid spelling still
  returns 201 so the guard isn't rejecting everything.

- [x] **E12 · The backend test-package list is duplicated and keeps drifting.** `S` · **P2 — DONE
  2026-07-28** (build/vet/lint clean, full suite green with and without Mongo, race-clean,
  routes+reminders 12×). Filed by **E10**.
  `go test ./...` can't be used because the one-off migrations under `server/scripts/` no longer
  compile, so every caller spelled the package list out instead. It had drifted three times in two
  days: **B4** found CI skipping three test files; **B8** found `services/auth` missing, so a new
  test package ran nowhere; **E10** found `.` (main) missing and two `DEVELOPMENT.md` lists still
  lacking `./encryption/` from **B6**. Each was caught only because someone happened to look.

  **It was five copies, not four** — `CLAUDE.md` held a sixth-form variant too, and the stalest of
  the lot: `go test ./models/ ./routes/ ./utils/ ./db/`, missing five packages. Its command
  reference also advertised bare `go test ./...`, which has never worked.

  **Fixed by deriving it:** every backend command is now
  `go test $(go list ./... | grep -v '/scripts')` — the same shape `go vet` and `golangci-lint` in
  the same workflow already used. `backend-ci.yml`, all three `DEVELOPMENT.md` spots and both
  `CLAUDE.md` spots quote that one form, so there is no list left to keep in sync. Verified the
  derived set is a **strict superset** of the old one — nothing that was being tested stopped being
  tested — and it picks up seven packages that were previously invisible (`docs`, `errs`, `logger`,
  `middleware`, `responses`, `services`, `tools/mintsession`); they have no tests today, but now a
  test added to any of them runs without anyone editing a list.

  **Did NOT delete or repair `server/scripts/`**, which this item had floated as the tidier fix.
  `server/scripts/README.md` already documents the opposite as a deliberate decision — *"They
  intentionally do not compile as part of the build … Don't try to make them build again"* — and it
  is the right call: they are historical records of migrations already run against prod, pinned to
  the model shapes of their time. Repairing them would falsify the record; deleting them would
  discard it. The exclusion is the correct permanent shape, so the README now also explains that it
  is *why* every command is written this way, and that adding a folder there needs no change
  elsewhere.

  **Found while verifying the no-Mongo path: `routes` did not actually skip cleanly.**
  `TestEventRoutes_IcsStaysPublic` had no `requireDB`, and unlike its neighbour it is not DB-free —
  passing the auth gate is the point, so the request reaches a handler that queries Mongo. Without
  `MONGODB_URI` it panicked on a nil `EventsCollection` rather than skipping. Invisible in CI, which
  always sets the var, but it broke `go test ./routes/` on any machine without Mongo — exactly what
  `DEVELOPMENT.md` promised would work. Confirmed pre-existing (reproduced at `1cead24d`, before
  E10) and fixed. The whole suite now exits 0 both with and without Mongo.

  **Docs corrected while in here:** `DEVELOPMENT.md`'s container command used `host.docker.internal`,
  which does not resolve on Linux/WSL — noted the `--network host` form, and **both container
  commands are now verified to run as written**, not just written down. Added the race-detector
  recipe (needs `CGO_ENABLED=1` and the Debian image, since the dev boxes have no gcc and Alpine
  ships none) that E10 had to work out from scratch. `scripts/README.md` listed
  `20240909_multiple_calendar_support_groups`, deleted in `c7d19b15` with the Availability Group
  feature. Dropped the stale `//nolint` example from `backend-ci.yml` too — same one E10 removed
  from `DEVELOPMENT.md`; the tree has no `nolint` directives at all since **B6**.
