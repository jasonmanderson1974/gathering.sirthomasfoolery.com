# Timeful / The Fellowship — Backlog 3 (active)

> Opened **2026-08-10**, at `be30197d`. `TODO.md` and `TODO2.md` are both **closed archives** as
> of this file — nothing in either needs updating again. The three items that were still open in
> TODO2.md Part G are restated below as pointers rather than copied, and all three are now closed
> won't-do; their `TODO2.md` entries were deliberately left untouched.
>
> **Status: Part J complete (2026-08-10), no loose ends.** A fresh full-codebase review pass —
> improvements and optimizations only. **J1–J11 all shipped**, and **J8's browser check has now been
> done against the deployed build** (2026-08-10) — the toggle→refetch path works, so nothing in Part
> J is left pending a deploy. **J12 (dependency vulnerabilities) and J13 (dead code) were added and
> shipped later the same day** — J12 came out of a `govulncheck`/`npm audit` sweep, not the review
> pass, because all three prior waves audited *our* code and nobody had ever audited what we depend
> on. **Nothing in Part J is open.**
>
> **The backlog is the Vue 3 migration (Part K).** The three inherited `P3` items (`TODO2.md` G2, G3, G4) were **closed
> won't-do on 2026-08-10** by user decision — see the section below for the reasoning, which is
> what a future review pass needs to read before re-filing any of them. Nothing else is open in any
> of the three backlog files.
>
> **Vue 2 is EOL, and as of 2026-08-11 the decision is to migrate.** It carries an unfixable ReDoS
> (no 2.x release will ever fix it — see J12), it will receive no further security fixes, and
> Vuetify 2, Vuex 3, vue-router 3, vue-meta 2 and vuedraggable 2 are all pinned behind it.
> `npm audit fix --force` "resolves" this by installing Vue 3 and breaking the app. **Part K below
> is that migration.** Phase 0 (J14–J17) is done and shipped on `main`; the cutover itself is a
> single branch, because Vuetify 2 does not run on Vue 3 even under `@vue/compat`.
>
> Context unchanged: self-hosted, invite-only fork for a ~30–40 person club. Reliability and
> small-club utility over scale. All event access requires sign-in; roles are
> superAdmin > admin > member > guest. Production runs no Docker — `mongod`, one static Go
> binary and `cloudflared` under systemd on `stf-thegathering`; see `DEPLOYMENT.md`.

Priority legend: **P0** = do first · **P1** = high value · **P2** = moderate · **P3** = nice-to-have.
Effort: **S** ≈ <½ day · **M** ≈ 1–2 days · **L** ≈ 3+ days.

**New items start at `J`** (A–E are TODO.md, F–H are TODO2.md; `I` is skipped so it can't be read
as a `1`). Never reuse a retired ID — an old commit message or code comment citing `F22` or `E12`
must keep resolving to one thing.

---

## Archives — where a cited item ID lives

Older commits, code comments and `CLAUDE.md`/`DEVELOPMENT.md` all cite backlog IDs. The letter
tells you which file to read:

| IDs | File | Covered |
|---|---|---|
| `A*` `B*` `C*` `D*` `E*` | [`TODO.md`](TODO.md) — archived 2026-07-28 | The first full-codebase review (2026-07-22) and the 2026-07-27 re-review: the security/cleanup wave, headlined by **E3** (sign-in required for all event access) and the deletion sweep. |
| `F*` `G*` `H*` | [`TODO2.md`](TODO2.md) — archived 2026-08-10 | The feature track **F1–F22** (nicknames, avatars, mentions, Lists v1–v3, My Lists, My Notes, Settle Up), Part G carried forward from `TODO.md`, and Part H's 2026-07-28 findings. |

Both archives keep their full write-ups, including what each item's implementation *found* that
the plan didn't predict. Read them before re-opening anything — several entries record defects
that were described wrongly by the finding that raised them, and the corrections are in the
entries, not in the code.

## Inherited — CLOSED 2026-08-10, won't do

**All three were closed by user decision on 2026-08-10: not worth the effort.** They had been
parked for weeks rather than scheduled, which is the same answer with more bookkeeping. This is a
decision, not an oversight — recorded here because each one is the kind of thing a future
full-codebase review will happily rediscover and file as new.

**If a later pass re-raises any of these, it is not a new finding.** Re-opening one means arguing
against the reasoning below, not merely noticing the code again. Give it a fresh `J*` ID if that
argument succeeds; the `TODO2.md` entries keep the full technical detail and stay untouched.

- **`TODO2.md` G2 — split `date_utils.js`** (`L`) — **won't do.** 946 lines / 32 exports, and the
  rest of G2 (eleven dead exports, `newEventFormMixin`, the `ScheduleOverlap` computed block) already
  shipped 2026-07-29/30, so what remains is pure file-shuffling with no user-visible payoff. The
  original entry's caveat is what kills it: a split has to be verified with the app running, because
  the earlier passes found three live bugs precisely by exercising them. That is real risk and real
  effort against zero benefit. A large file is not by itself a defect.
- **`TODO2.md` G3 — web push** (`M`) — **won't do.** Email reminders already cover iOS, so the
  feature's value was never established, and it reverses the deliberate PWA removal in `f857320`.
  J11 raised the cost of getting it wrong: because the SPA fallback serves `index.html` for
  `/service-worker.js`, a mis-registered worker **cannot 404 itself out of existence** — it fails its
  update check on the MIME type and pins that client to a cached build indefinitely. Only revisit if
  the push notifications are actually wanted; the kill-switch lesson in `CLAUDE.md` stays regardless.
- **`TODO2.md` G4 — rename the `schej-it` Mongo database** (`L`) — **won't do.** Zero user-facing
  benefit, and since the old host was decommissioned (2026-08-05) the backup chain is the only safety
  net under a human-run dump→restore→cutover. The name is internal and already documented as
  intentional in `CLAUDE.md` (with `SCHEJ_EMAIL_ADDRESS`, TODO D0/D2). Risk without upside.

---

## PART J — 2026-08-10 review findings (improvements & optimizations)

From a full review pass at `38b613f7`. The codebase has been through two prior waves (A–E, H), so
these are what's left: mostly performance and robustness, no known user-facing breakage except J2.
All eleven are done (J11 was added on 2026-08-10, picked up out of the parked G3). Each entry keeps
its original finding followed by what the implementation
actually found — including the three cases where the finding was wrong or incomplete (J3's
`getEventIds` note, J6's understated disclosure, J10's "the official client always sends a sane
value").

### J1 — `getEvent` looks up users one at a time · **P1 · S** — DONE 2026-08-10

`routes/events.go:521` — the "populate user fields" loop calls `db.GetUserById(userId)` once per
responder, on the hottest route in the app (every event page load). With a full-club gathering
that's ~30–40 sequential Mongo round-trips before the page can render. `db.GetUsersByIds` already
exists and is what every newer surface (display names, mentions, expenses) uses — collect the
responder ids first, do one `$in` query, then walk the map. Bonus: `resolveEventDisplayNames`
(called later in the same handler, `events.go:617`) does its own batched lookup over a heavily
overlapping id set; folding the two into one query is fair game while in there. The guest/deleted-
user branches in the loop must survive exactly as they are — that logic is subtle (guest keys
switch from id to name) and tested behavior.

**Found on implementation.** The loop body is unchanged; only the lookup moved, so the guest and
deleted-user branches are byte-for-byte what they were. Keys that don't parse as ObjectIDs are
simply left out of the `$in` — `GetUserById` already returned `nil, nil` for a malformed id, so a
name-keyed guest row reaches the same branch by the same route.

Two things the entry didn't predict:

- **The error branch is gone, and that's a small improvement.** `GetUserById` returned an error only
  on a decode failure, and the old `continue` skipped the availability-stripping below it — so a
  single undecodable user document leaked that responder's raw availability arrays to the client.
  `GetUsersByIds` logs and omits instead, which drops the row into the deleted/guest branch and
  always strips.
- **Do NOT fold `resolveEventDisplayNames` into this query**, despite the entry calling it fair
  game. It runs deliberately *after* `visibleComments`, so a hidden thread's authors are never
  looked up at all; hoisting its lookup to the top of the handler would re-introduce exactly the
  leak that ordering prevents. The overlap costs one extra `$in` and is worth paying.

Verified against the running stack with Mongo profiling on: an event with five responders (three
members, a name-keyed guest, a deleted account) went from five `_id` queries to one `$in`. The two
remaining single-id lookups in the trace are the auth middleware's session user and the
comment-viewer check — both outside this loop.

### J2 — a duplicated event inherits last time's RSVPs, poll votes and lists · **P2 · S** — DONE 2026-08-10

`routes/events.go:846` — `duplicateEvent` takes the event document as read, swaps `Id`/`Name`/
`NumResponses`, and inserts it. But `Rsvps`, `Polls` (votes included) and `Lists` (items included)
are all embedded in that document now (`models/event.go:364–371`), so the copy starts life with
everyone's old RSVPs, their poll votes, and the old menu — presented as if they answered for the
new gathering. Only availability copying is opt-in; the rest rides along invisibly. Clear all
three on the duplicate (or make each opt-in like availability). While in there: the
`copyAvailability` branch inserts responses in a per-document `InsertOne` loop
(`events.go:856–866`) — one `InsertMany` does it.

**Found on implementation.** Cleared, but not to the letter of "clear all three": the split is
**participation vs. scaffolding**. A poll keeps its title and options and loses its votes; a list
keeps its name and kind and loses its items; RSVPs go entirely, since an RSVP is nothing but a
response. Re-using the structure is the whole point of duplicating an event — nuking the polls and
lists outright would have removed a feature to fix a bug. If that call is ever revisited, the
alternative is making each opt-in alongside `copyAvailability`.

**Two more inherited fields, same defect class, not in the original finding:**
`GatheringReminder.SentAt` (the copy would never send its reminder email, because the *original's*
already went out) and `Chronicled` (the scheduler would never capture the new gathering). Both
cleared here.

`InsertMany` done, guarded on a non-empty batch — an empty `docs` slice is an error, not a no-op,
so a duplicate-with-copy-availability of an event nobody had answered would have 500'd. Regression
tests in `routes/events_duplicate_db_test.go` cover all three; confirmed they fail against the old
handler (`inherited 1 RSVP`, `inherited 1 vote`, `inherited 1 item`) and that the **source** event
keeps its own answers.

### J3 — every event page load pays a dead `/ids` round-trip · **P2 · S** — DONE 2026-08-10

`views/Event.vue:1003–1019` (`refreshEvent`) — the handler awaits `GET /events/:id/ids`, stores
the result in `resolvedLongId`, then discards it (`void resolvedLongId`, a leftover from the
guestName-era code the E-wave removed) and fetches the event by the original id anyway. Because
the two awaits are sequential, the discarded call adds a full round-trip of latency to the
page's critical path — first paint of the grid waits on it. Delete the `/ids` fetch; nothing
consumes it. (`getEventIds` itself stays — the router redirect still uses it.)

**Correction to the entry.** "The router redirect still uses it" is wrong on two counts: `getEventIds`
is the **Go handler** (`routes/events.go:44`), not a frontend function — there is no
`getEventIds` in `frontend/src` and there was none before this change either. And `refreshEvent`
was the endpoint's *last caller anywhere in the app*: `GET /events/:eventId/ids` is now unused by
the official client. It stays regardless — it's a documented, auth-gated part of the API surface
(`routes/event_auth_gate_test.go` covers it) — but nothing calls it, which is worth knowing before
someone "fixes" a caller that doesn't exist. (The only "redirect" in the frontend router is the
`?redirect` open-redirect guard in `router/index.js`, covered by `src/router/redirect.test.js` —
nothing to do with resolving event ids.)

Verified on the shipped artifact, not just the source: the string `/ids` appears zero times in the
built `dist/js/*.js`.

### J4 — hashed frontend assets are served with no cache headers · **P2 · S** — DONE 2026-08-10

`main.go:173–186` — every file in `frontend/dist` is registered via `router.StaticFile` with no
`Cache-Control`, so browsers revalidate each bundle on every visit and the Cloudflare edge is left
to guess. Vue CLI content-hashes its filenames (`app.[hash].js`), which is precisely the case for
`Cache-Control: public, max-age=31536000, immutable` — a returning member's load drops to just
`index.html` (already `no-cache` via the NoRoute handler, `main.go:316`, so deploys still
propagate instantly). Gate the header on the path carrying a content hash (`/js/`, `/css/`,
hashed `img/` names) rather than blanket-applying it — `favicon.ico` and friends aren't hashed.

**Found on implementation.** The gate ended up being the **content hash itself**
(`\.[0-9a-f]{8}\.`, `contentHashedAsset` in `main.go`) rather than a directory allowlist, because
`img/` is mixed: it holds hashed build output (`apple_logo.e6bf682d.svg`) *and* files copied
verbatim from `public/` (`ogImage.png`, `when2meetOgImage2.png`). Matching on the hash sorts both
cases correctly with no list to maintain.

`gin.StaticFile` gives no way to set a header, so hashed files are registered as explicit
`GET`+`HEAD` handlers (the two verbs `StaticFile` itself registers) that set `Cache-Control` and
then `c.File`. Unhashed files still go through `StaticFile` untouched.

Verified against the running dev stack, per-path:

| Path | `Cache-Control` |
|---|---|
| `/js/app.457eeeac.js`, `/css/416.953510a1.css`, `/img/apple_logo.e6bf682d.svg` | `public, max-age=31536000, immutable` |
| `/favicon.ico`, `/robots.txt`, `/img/ogImage.png` | *(none — unchanged default)* |
| `/index.html` and `/` (SPA routes) | `no-cache, no-store, must-revalidate` |

That last row is the one that matters for deploys: `index.html` is never registered by the static
walk, so it keeps `noRouteHandler`'s `no-cache` and a deploy still propagates instantly even though
the bundles it points at are pinned for a year.

**Testing gotcha, cost a few confused minutes:** `compose.dev.yaml` bakes its *own* frontend build
into the image, so the container's asset hashes differ from a local `npm run build`. Curling a
locally-built filename returns `200` with `Content-Type: text/html` — the SPA fallback serving
`index.html`, which looks like the header simply failed to apply. Read the filenames out of the
container before testing.

### J5 — a non-`error` panic in a calendar provider kills the whole server · **P2 · S** — DONE 2026-08-10

`services/calendar/calendar.go:21,41` — both async helpers recover with `c <- …{Error:
err.(error)}`. If any provider code panics with a non-error value (`panic("boom")`, an int — and
these are goroutines wrapping three external APIs plus a CalDAV library), the type assertion
itself panics inside the deferred function, the recovery is lost, and the unrecovered goroutine
panic takes down the process. Replace with `fmt.Errorf("%v", r)` (and recover into a plain
`interface{}` first). Two-line fix, removes a latent crash-on-weird-input in the one subsystem
that talks to the most third-party code.

**Found on implementation.** Done via a shared `recoveredError(r interface{}) error` helper rather
than inline, because there are two identical sites and a third would be easy to get wrong.

**A second defect at the same two lines, not in the finding: neither panic path set
`CalendarAccountKey`.** `GetUsersCalendarEvents` keys its results map by exactly that field, so a
recovered panic filed the error against a phantom `""` account and left the real account looking
like it had simply returned no calendars — the error was recovered and then lost. Both paths set it
now.

The helper also logs the panic value with `debug.Stack()`. Without that the panic is invisible: the
caller only ever renders it as a per-account error string on the calendar list, with nothing
naming the provider that blew up.

Covered by `services/calendar/panic_recovery_test.go` (string, int and error panics, both
helpers). Note the test's real assertion is structural — a panic escaping these goroutines would
take the *test binary* down, so the suite completing at all is the regression signal.

### J6 — the dashboard ships every event's full embedded document · **P2 · S/M** — DONE 2026-08-10

`routes/user.go:441` (`getEvents`) — the query returns whole event documents: embedded `rsvps`,
`polls`, `lists` (items and all), `dates`, `remindees`, for every event the member ever touched,
with no projection and no pagination. The dashboard renders a name, a date range and a folder.
Payload grows with club history and every F-track feature made each document fatter; it also
leaks data the dashboard has no business holding client-side (other people's RSVP emails are
stripped only by `getEvent`'s per-event logic, not here — worth confirming what `remindees`
serializes on this path while fixing it). Add a `Find` projection down to the fields the
dashboard and folder logic actually use.

**Found on implementation. The disclosure suspicion was correct, and worse than "worth
confirming".** Both leaks were reproduced against the running server before the fix: an event
carrying one RSVP and one remindee shipped `"rsvps":{"Zoe":{...,"email":"zoe@example.com"}}` and
`"remindees":[{"email":"invitee@example.com"}]` to the dashboard. So every member's dashboard load
handed them other people's RSVP email addresses *and* the owner's full invite roll — the two things
`getEvent` takes specific care to strip and hide, bypassed simply by asking a different endpoint.

The projection is the union of two consumers, and the comment in `routes/user.go` names both so the
next person to add a dashboard field knows where to add it (it would otherwise read as `undefined`
with nothing failing):

- **client** — `EventItem.vue`: `_id`, `shortId`, `ownerId`, `name`, `isArchived`, `type`,
  `daysOnly`, `numResponses`; plus `dates`, which `getDateRangeStringForEvent` needs.
- **server** — `AssignUnfiledEventsToDefaults`, which files unfiled events by `Id` and `OwnerId`.

**Two dead things found while auditing the consumers, deliberately left alone** (neither is a bug,
both are cleanup someone might otherwise "fix" into existence):

- `event.hasResponded` **does not exist on the server at all** — no field on `models.Event`, nothing
  that sets it. `EventItem.vue:231` and `Home.vue:86` both read `event.hasResponded ?? false`, so
  the dashboard's responded indicator is permanently false. Correctly absent from the projection;
  adding it there would be cargo-culting a field with no source.
- `Event.vue:504` maps `events` from the store and never reads it.

Verified live, not just in tests: with the projection in place the same seeded event returns all
nine dashboard fields intact and `rsvps`/`polls`/`lists`/`remindees` all `null`. Note they
serialize as `null` rather than vanishing — `models.Event`'s json tags carry no `omitempty` — so
tests must assert null-or-absent, not key absence. `routes/user_events_projection_db_test.go`
asserts both directions and fails without the projection.

### J7 — DB accessors that report an outage as "not found" · **P3 · S** — DONE 2026-08-10

`db/expenses.go:70` (`GetExpenseById`) — any `Decode` failure returns `nil, nil`, so a Mongo
outage mid-request answers 404 `expense-not-found` instead of 500, and the comment says
`GetCommentById` set the pattern (same shape in `db/comments.go`). A malformed id as "not found"
is right; a connection error is not. Distinguish `mongo.ErrNoDocuments` from everything else and
return real errors — the route layers already have the 500 branch wired for it.

**Found on implementation.** Both accessors fixed. The claim that the route layers already handle
the error was checked rather than taken on faith — all five call sites do
(`routes/expenses.go:130`, `routes/comments.go:152/196/318`, and `mention_emails.go:271`, which
withholds notification emails on either an error or a nil).

The contract now, pinned in `db/not_found_vs_error_test.go` in both directions:

| case | returns |
|---|---|
| absent row | `(nil, nil)` — a real 404 |
| malformatted id | `(nil, nil)` — also a real 404 |
| undecodable row / Mongo failure | `(nil, error)` — a 500 |

**The sharpest consequence was in `deleteComment`, not the 404s.** `routes/comments.go:318` treats
`comment == nil` as "already gone" and answers `200`, which is right for an idempotent delete and
exactly wrong when nothing was actually read: while Mongo was unreachable, the client was told the
comment had been deleted. That path now 500s.

Testing note for anyone extending this: a real outage can't be staged against the shared test Mongo,
so the tests use the other member of the same error class — a stored document whose field types
don't match the model (`userId` as an int, `description` as a sub-document). That fails `Decode`
with a type error rather than `ErrNoDocuments`, which is precisely the distinction the fix
introduced; before it, the two were indistinguishable.

### J8 — disabled calendars are still fetched from the providers · **P3 · S→M** — DONE 2026-08-10

`services/calendar/calendar.go:142` — after the calendar list resolves, events are fetched for
*every* sub-calendar, including those the member toggled off (`SubCalendar.Enabled` false) and
accounts toggled off wholesale (`CalendarAccount.Enabled`). Each is a live round-trip to
Google/Microsoft/CalDAV. Before skipping them, check what the frontend expects: if toggling a
calendar on merely re-filters client-side (instant today), skipping the fetch changes that to a
refetch — acceptable, but a product call to make knowingly, not a silent optimization.

**Found on implementation. The frontend check the entry demanded came back worse than "acceptable",
and the answer was a user decision** (taken 2026-08-10: do it properly, with a refetch).

The server never reads `Enabled` at all — it fetches everything and `currentAvailabilityMixin.js`
filters client-side. And `CalendarAccount.vue` carries an explicit comment saying **"the toggle
POSTs but never refetches"**: the write-through into `account.enabled` is what updates the UI.
Worse, `CalendarAccounts` is mounted *inside* `ScheduleOverlap.vue` — the availability grid itself.
So skipping the fetch on its own would mean toggling a calendar **on** while looking at the grid
showed nothing at all until a page reload. Not a trade-off; a regression.

Shipped as both halves together:

- **Server** — a wholesale-disabled account is skipped entirely (its calendar-list call *and* the
  one events call per sub-calendar it would have spawned); a disabled sub-calendar skips its events
  call. Its map entry is still created, so the response shape doesn't depend on which accounts are
  enabled.
- **Client** — a successful toggle POST emits `calendarsChanged`, re-emitted up
  `CalendarAccount` → `CalendarAccounts` → `ScheduleOverlap` → `Event.vue`, which calls
  `fetchAuthUserCalendarEvents`. Emitted on toggle-off too: cheap, and it keeps one path.

**The trap here is `nil`, and it is the reason `isDisabled` exists** rather than a bare `!*Enabled`.
Both flags are `*bool`. Nil means "never toggled" and appears on legacy rows; every current path
sets the flag outright (the Google provider mirrors the calendar's `Selected` state, the others
default to true). **Nil is treated as ENABLED, deliberately fail-open** — a skip is only safe when
the client would certainly have discarded the result, and guessing wrong the other way silently
removes real events from someone's availability. Note the client's own filter treats `undefined` as
*disabled*, so the two rules differ on purpose: the server's job is only to avoid fetching what is
certainly unwanted.

**Accepted trade-off:** a disabled account's stored `SubCalendars` list stops being refreshed, so a
calendar added provider-side while the account is off won't appear until it's switched back on — at
which point the toggle's refetch catches it up.

**Runtime verification — done, 2026-08-10, against the deployed build.** This could not be
exercised locally (a real Google/Microsoft account and a working login are both needed, and
`compose.dev.yaml` has neither SMTP nor OAuth), so it shipped on build/lint/unit-tests plus a
hop-by-hop check of the emit chain — a combination that has been green over a browser-only bug
before. The check written into this entry has since been carried out on the deploy: opening an
event, toggling a sub-calendar off and back on, and confirming the events return without a reload.
They do. **No loose end remains.**

### J9 — CLAUDE.md still says date math uses three libraries · **P3 · S** — DONE 2026-08-10

`CLAUDE.md` (frontend utils bullet) claims `date_utils.js` "uses `dayjs`/`moment`/`spacetime`".
`moment` and `spacetime` are gone — not in `frontend/package.json`, not imported anywhere;
`date_utils.js` is dayjs-only. One-line doc fix; flagged because the 2026-08-10 doc audit
(`3972f71e`) missed it, and a stale claim in CLAUDE.md steers every future session.

**Verified before fixing, not assumed:** `frontend/package.json` lists `dayjs` and neither of the
others, and every date import across `src/` is `dayjs` (or a dayjs plugin). The bullet now says
"dayjs only" and names the two that are gone, so the next reader doesn't re-add the claim from
memory.

### J10 — an expense accepts any date a client sends · **P3 · S** — DONE 2026-08-10

`routes/expenses.go:380–384` — `payload.Date` is trusted verbatim (unix ms, unbounded). The
ledger sorts by date descending (`db/expenses.go:48`), so a hand-rolled client can stamp an
expense with year 9999 and pin it above every real row forever, or go negative and bury it. The
official client always sends a sane value; clamp server-side anyway (say, within a year around
now) to the same standard the amount cap already sets — "a guard against nonsense, not a
business rule".

**Found on implementation. Rejected (400 `invalid-date`) rather than clamped**, despite the entry
saying clamp. Two reasons: the neighbouring amount cap 400s rather than pinning the value, so
rejecting is "the same standard" in the sense that matters; and silently rewriting a date someone
deliberately picked is the worse failure — a clamped date looks like the ledger lost the entry,
with nothing said.

**"The official client always sends a sane value" was NOT true, and that turned a server-only
guard into a two-sided change.** `v-date-picker` in `ExpenseDialog.vue` had no `min`/`max`, so a
member could navigate to any year and get a save failure — and `expenseErrorMessage`'s code→message
map (which exists precisely so a validation error says what to fix) had no arm for the new code, so
they'd get the generic "Could not save that expense. Please try again." and retry the same date
forever. Three parts shipped together:

1. `expenseDateWindow` in `routes/expenses.go` — ±365 days, rejecting outside it.
2. `expenseDateMin`/`expenseDateMax` bound the picker to the same window, so nobody can produce a
   rejectable date. `EXPENSE_DATE_WINDOW_DAYS` in `expenseForm.js` mirrors the Go constant, and a
   unit test asserts the number so the two can't drift silently.
3. An `invalid-date` arm in `expenseErrorMessage`, for the hand-rolled-client case the guard is
   actually aimed at.

### J11 — the service-worker kill switch that was never ours to run · **P3 · S** — DONE 2026-08-10

Picked up out of the parked `TODO2.md` G3, which is where this was raised as "one cheap loose end,
independent of the rest": `kill-sw.js` sits at the repo root, so it is **never actually served** and
can't unregister anything — move it to `frontend/public/` or mark it documentation-only.
`frontend/.eslintrc.cjs`'s `serviceworker: true` env is stale too. The rest of G3 (web push itself)
stays parked; only this is done.

**Found on implementation. Both of the options the finding offered were wrong, and the file was
deleted instead.** The finding assumed there are stale registrations out there to kill and that the
only thing standing in the way was the file's location. Neither half holds:

- **Moving it to `frontend/public/` would have produced a served file that still does nothing.** A
  stale worker only ever re-fetches **its own registered script URL**, and the deleted
  `registerServiceWorker.js` registered `${BASE_URL}service-worker.js` → **`/service-worker.js`**. A
  kill switch at `/kill-sw.js` is a URL nothing ever asks for. Serving it would have looked like the
  fix and changed nothing — the worst of the three outcomes, because the next reader would believe
  it was handled.
- **This origin cannot have a stale worker at all.** The PWA was removed upstream in `f857320` on
  **2025-06-24**; the fork's own deploy tooling doesn't appear until `cd1f103b`, **2026-07-22**. No
  build this origin has ever served contained a service worker, so no client of it holds a
  registration. `kill-sw.js` was upstream's kill switch for **schej.it's** clients — its own header
  comment says `// /schej.it/kill-sw.js`, and it was committed (`e8deeee4`) fifteen minutes before
  the PWA removal it was paired with. We inherited someone else's remediation for someone else's
  users.

So it went, along with the eslint env. Nothing else referenced either: `register-service-worker`,
`workbox` and the PWA plugin are all long gone from `frontend/package.json`, and `frontend/public/`
has no worker or manifest. The env was doubly dead — `lint` runs `eslint .` from `frontend/`, which
never reached a file at the repo root in the first place.

**Kept, in the CLAUDE.md bullet, because it is the part worth knowing:** if a PWA is ever shipped
here and then removed, the kill switch must be served at the **registered script URL**, and the SPA
fallback makes getting that wrong permanent rather than merely useless. `GET /service-worker.js`
against this server returns `index.html` as `text/html`, so a stale worker's update check fails on
the MIME type — it does **not** 404 itself out of existence, which is the outcome people assume when
they simply delete the file. The worker survives indefinitely, pinning that client to a cached build.

Frontend lint (`--max-warnings 0`), unit tests and the production build all pass with the env
removed — worth checking rather than assuming, since `serviceworker` also defines globals; `self`
and `caches` survive because `browser: true` supplies them, and nothing in `src/` uses `clients` or
`registration`. No Go file was touched.

### J12 — six reachable vulnerabilities in Go dependencies · **P1 · S** — DONE 2026-08-10

Not from the Part J review pass. Raised on 2026-08-10 by running `govulncheck` and `npm audit` for
the first time: **A–E, H and J all reviewed the code we wrote, and nothing had ever checked the code
we pull in.** `govulncheck` reported six vulnerabilities that our code actually *reaches* (not
merely has in the module graph), across four modules:

| Module | Was | Fixed in | Advisory |
|---|---|---|---|
| `github.com/gin-contrib/cors` | 1.4.0 | 1.6.0 | GO-2024-2955 — wildcard mishandled in the origin string |
| `go.mongodb.org/mongo-driver` | 1.12.1 | 1.17.7 | GO-2026-5327 — heap OOB read in GSSAPI error handling, reached via `db.Init` |
| `golang.org/x/net` | 0.23.0 | 0.53.0/0.55.0 | GO-2026-4918 (HTTP/2 infinite loop), GO-2026-5026 (idna), GO-2025-3595 |
| `golang.org/x/text` | 0.14.0 | 0.39.0 | GO-2026-5970 — infinite loop on invalid input |

**Result: 6 reachable → 0.** Vulnerabilities in imported packages also went 8 → 0. Backend suite,
`go vet` and golangci-lint are all green on the new versions, and the built binary was smoke-booted
against local Mongo (health `{"status":"ok","mongo":"ok"}`, no panic) — a driver bump is exactly the
kind of change that compiles and tests clean and then fails at connect time.

**Found on implementation.**

- **The `cors` fix is a no-op for us, and it was still right to take.** `main.go` passes
  `AllowOrigins` an explicit list (`CORS_ORIGINS`, or a two-entry default) and never a wildcard, so
  GO-2024-2955 was not exploitable here. Bumped anyway: it costs nothing, and the next person to add
  a wildcard origin should not be the one who discovers the version was known-vulnerable.
- **`go get golang.org/x/net@v0.55.0 golang.org/x/text@v0.39.0` fails outright** — x/text 0.39.0
  requires x/net **0.56.0**, so the pair must move together. The trap is that **a failed `go get`
  applies nothing at all**, so running `go mod tidy` straight afterwards looks like tidy "reverted"
  the upgrade when in truth it never happened. The versions hold through `tidy` once the bump
  actually lands; if an indirect bump appears to keep reverting, check that the `go get` succeeded
  before blaming MVS.
- **`mongo-driver` v1 is deprecated** — `go get` prints `use go.mongodb.org/mongo-driver/v2 instead`.
  1.17.7 is the fix on the v1 line and is what shipped. **v2 is a breaking API change and is
  deliberately out of scope**; it wants its own item, not a drive-by during a security bump.
- The x/net bump normalised the go directive from `go 1.25` to `go 1.25.0`. Cosmetic, left as-is.
- `github.com/klauspost/compress` 1.16.7 → 1.19.2 was taken as well (GO-2026-5841, OOB read in
  `s2`). **Not reachable** — it arrives under the Mongo driver and we never call it — but the bump is
  free and it keeps future scans from carrying permanent noise someone has to re-triage.

**One advisory remains and cannot be cleared by bumping: GO-2026-5932**, `golang.org/x/crypto/openpgp`
is "unmaintained and unsafe by design" and **has no fixed version**. We do not import it; it is
flagged at the module level only. Expect `govulncheck` to keep reporting exactly one uncalled
advisory — that is the steady state, not a regression.

**`govulncheck` needs the same `/scripts` exclusion as `go test`.** A bare `govulncheck ./...` does
not run at all — it fails during package loading on the deliberately-uncompilable migrations
(`unknown field AccessToken`, `undefined: models.PrintJson`, …), which reads like a broken checkout
rather than a known exclusion. Use
`govulncheck $(go list ./... | grep -v '/scripts')`, same derived form as everything else (E12).
Added to `CLAUDE.md` alongside the other commands.

**Frontend (`npm audit`) — assessed, deliberately not acted on.** Five findings, and the severity
ranking is misleading:

- `postcss` and `nanoid` (both "high") enter **only** through `@vue/cli-plugin-babel` and
  `@vue/cli-service` — build-time tooling. Neither ships in the bundle, so neither is reachable by
  any user. `npm audit --omit=dev` still lists them, which is why they look like production risk;
  read the tree (`npm ls postcss`) rather than the severity column.
- The `vue` ReDoS (GHSA-5j4c-8p2g-v4jx) is real and affects every Vue 2.x release. There is **no
  fix on the 2.x line** — Vue 2 is EOL — so `npm audit fix --force` "resolves" it by installing
  Vue 3, which breaks the entire app (Vuetify 2 and Vuex 3 both go with it). **Do not run
  `npm audit fix --force` in `frontend/`.** A Vue 3 migration is a project of its own; if it is ever
  wanted it needs a fresh item and a plan, not a security bump.

### J13 — dead code left behind by J3 and J6 · **P3 · S** — DONE 2026-08-10

Small, safe, and already scoped — split out of J12's sweep rather than done inside it, to keep a
security bump free of unrelated edits. All four were found by following J6's "two dead things"
note and re-checking it:

- `EventItem.vue:230` (`userHasResponded`) and `Home.vue:85` (`userRespondedToEvent`) read
  `event.hasResponded`, which **no server code sets** — J6 established that. What J6 did not
  record: **neither is referenced by any template**, so this is not a permanently-false indicator,
  it is three pieces of dead code. Verify with a template grep before deleting, not just a
  definition grep.
- `Event.vue:504` maps `events` from the store and never reads it (J6 noted this and left it).
- `GET /events/:eventId/ids` (`routes/events.go`) has had **no caller anywhere in the app** since
  J3. Keep or drop is a judgement call — it is auth-gated and covered by
  `routes/event_auth_gate_test.go`, so it is not a liability; it is just surface. Decide
  deliberately rather than deleting it by reflex.

**Found on implementation.**

**`userHasResponded` is two different things sharing a name, and that is the whole danger of this
item.** A repo-wide grep returns ~20 hits and they look like one heavily-used property, which
argues against deleting anything. They are unrelated:

| | reads | status |
|---|---|---|
| `EventItem.vue:230` | `event.hasResponded` — **a field the server never sets** | dead, deleted |
| `Event.vue:669` | `authUser._id in event.responses` | **live**, drives the availability button |
| `availabilityMixin.js:88` | `authUser._id in parsedResponses` | **live**, drives `ScheduleOverlap` |

Anyone doing this by grep-and-delete either removes the working ones or, seeing the traffic on the
name, concludes the item is wrong and closes it. Match on **what the computed reads**, not its name.

**The `/ids` endpoint was kept.** It is published Swagger surface, auth-gated and tested, and this
app documents a third-party client API (`PLUGIN_API_README.md`) — so "nothing in `frontend/src`
calls it" is *not* "nothing calls it", and there is no way to audit unofficial clients of a
self-hosted app. Deleting a documented contract to save ~15 lines is the wrong trade. A comment now
says so at the handler, because previously only `Event.vue` recorded it and the note a server-side
reader needs was on the wrong side of the codebase.

**That comment exposed a swag trap worth knowing.** Swag treats a non-annotation line in a
function's doc comment as the endpoint's **description**, so an internal note written directly above
`@Summary` gets published into the public API docs. It is kept as a separate comment group with a
blank line before `@Summary`. Verified rather than assumed: `swag init --parseDependency
--parseInternal` regenerates `docs/` **byte-identically** with the comment in place.

**Browser-verified** (`/root/tools/browser/verify_j13_local.js`, screenshots `j13_home.png` /
`j13_event.png`), because deleting a computed that a template still referenced is a Vue *runtime*
warning — lint, unit tests and the build pass either way, which is exactly the failure class the
"look at the page" rule exists for. Dashboard cards and the event page both render with **zero
console errors or Vue warnings**, and the event page still shows "Mark availability" — the string
produced by the *live* `userHasResponded` ternary, which is the direct evidence the surviving twin
still works.

**Two local-harness gotchas, both of which cost a cycle:**

- **Rebuilding the dev stack without the secrets overlay silently breaks OTP login.**
  `docker compose -f compose.dev.yaml up -d --build` drops the untracked
  `compose.dev.secrets.yaml`, and the symptom is the sign-in page never advancing to the OTP field
  — it reads as a frontend regression from the change under test. Always pass **both** `-f` files.
- **The local allowlist is not the prod one.** `login.js` defaulted to
  `jason@sirthomasfoolery.com`, which is not on the local roll; the invite gate refuses it at the
  email step, which again looks like the OTP screen failing rather than a rejected address. The
  local superAdmin is `jason@jasonmanderson.com`. Default fixed in `login.js`, with the mongosh
  one-liner to list the roll.

**Reconfirmed the golangci-lint no-op** (already in `CLAUDE.md`, worth restating because it bit
again): run from the repo root instead of `server/`, it prints a typechecking error **and then
`0 issues`**, so a green-looking line can mean it linted nothing. Read the line above the count.

---

## PART K — the Vue 3 / Vuetify 3 migration (opened 2026-08-11)

Vue 2 went EOL on 2023-12-31. The full plan, including the target versions and the reasoning
behind each decision, is the migration plan agreed on 2026-08-11; what follows is the item record.

**The shape of the thing, and the fact that drives everything else: `@vue/compat` buys nothing
here.** The official migration build exists so Vue 2 code can run on Vue 3 while deprecations are
fixed incrementally — but **Vuetify 2 does not run on Vue 3 in any configuration**, compat
included. With ~800 Vuetify tags across 41 distinct components there is no meaningful subset of
this app that renders without it. So Vue 3 and Vuetify 3 land together, in one branch, and all the
incrementalism has to come from Phase 0 instead.

Two more findings from the survey that shape the work:

- **The theme survives; the overrides do not.** There are **zero** Vuetify SASS variable overrides
  — the Fellowship identity lives in Tailwind, and `plugins/vuetify.js` is 41 lines reading its
  colours *from* `tailwind.config.js`. But **41 selector lines override Vuetify's internal DOM with
  `!important`, and 34 name classes Vuetify 3 does not emit** (`.v-input__slot`, `.v-menu__content`,
  `.v-input--switch__track`, `.v-btn--is-elevated`, `.error--text`, …), across five files of which
  only one is `<style scoped>`.
- **The biggest template change is the activator pattern** — 21 slots, including *every* `v-menu` in
  the app: `v-slot:activator="{ on, attrs }"` + `v-on="on" v-bind="attrs"` becomes
  `v-slot:activator="{ props }"` + `v-bind="props"`.

**Stay on Vue CLI 5 / webpack; do not migrate to Vite as part of this.** Beyond not wanting to
change framework and bundler at once: **Vite's asset hashes would silently disable J4's immutable
caching.** `server/main.go:52` gates `Cache-Control: … immutable` on `\.[0-9a-f]{8}\.` — eight hex
characters between dots, which is Vue CLI's `app.457eeeac.js`. Vite emits `index-DiwrgTda.js`:
base64url, dash-separated. It never matches, with no error and no warning. If Vite is ever wanted
it is a separate item whose first line is "move that regex".

### Phase 0 — prep on Vue 2 · DONE 2026-08-11

Four items, all shipped to `main` on the existing stack. Each shrinks the cutover diff and none is
wasted if the migration stalls.

#### K/J14 — a route-level browser check to migrate over · **P0 · M** — DONE 2026-08-11

`frontend/scripts/check-routes.js` + `npm run check:routes`. Every route in the router, all five
event band tabs, the New Gathering dialog, and a 390px pass; asserts render, an identifying
control, and a clean console for each.

**Why it had to come first.** Nothing in the repo could fail on a rendering bug. All 23 unit-test
files are pure JS deliberately extracted *out of* components — `vitest.config.mjs` sets
`environment: "node"`, no test imports a `.vue` file, `@vue/test-utils` isn't even a dependency —
so the suite stays green through a total rendering failure. Written against the DOM rather than
against Vue, so it is worth the same after the migration as before it.

**Found on implementation.**

- **The `browser-check-lib.js` Chrome fallback list was decorative.** `spawn` reports a missing
  binary by emitting an asynchronous `'error'` event, not by throwing, so the `try/catch` around it
  never fired: the first name always "succeeded" and the process died later with an unhandled
  ENOENT rather than falling through to `chromium`. Fixed.
- `frameworkWarnings` scans **both** console levels, because Vue 2 warns through `console.error`
  and Vue 3 through `console.warn`, and joins every argument — the component trace naming the
  culprit is never the first one. **These warnings are stripped from production builds**, which is
  what `compose.dev.yaml` serves, so run against `npm run serve` when the warnings are the point.
  That will matter during the cutover, where removed APIs warn rather than throw.
- **Validated by breaking each assertion class on purpose** and confirming it flipped to FAIL: a
  rejected cookie, an injected `display: block !important` on the band panels, a synthetic
  `console.error`, a `[Vue warn]` on both console levels, and an overflowing element. A check that
  has only ever passed is worth nothing.

#### K/J15 — replace `vue-worker` · **P1 · S** — DONE 2026-08-11

Vue 2-only plugin (`Vue.prototype.$worker`), one call site, replaced by `src/utils/worker.js`.

**Found on implementation.** The caller passes `Set`s in and gets a `Map` of `Set`s back, so the
transport had to stay **structured clone, not JSON** — a JSON round trip would have turned all of
them into `{}` with no error, producing an availability grid where nobody is ever free. The
replacement also terminates the worker on the *error* path, which `simple-web-worker` did not: it
ended with `close()` inside the worker body, which only runs when the work succeeds.

#### K/J16 — drop `$set` / `$delete` (15 sites) · **P1 · S** — DONE 2026-08-11

Removed in Vue 3; all 15 work identically on Vue 2 given whole-object *reassignment* rather than
in-place mutation.

**Found on implementation. `emailSuggestions` is an ARRAY, not a keyed object** — `$set(arr, i, v)`
there was the array-index workaround, and it becomes `splice(i, 1, v)`, which stays reactive on
both versions and keeps the array identity `respondents.map` establishes in `created`. Two loops
were rebuilt rather than transliterated (Dashboard's `folders` watcher, RespondentsList's
added/removed sweep) because reassigning per id would have made them quadratic.

#### K/J17 — remove `.native`, funnel the breakpoint reads · **P1 · S** — DONE 2026-08-11

**Found on implementation.** `.native` was dropped rather than rehomed: VTextField spreads
`$listeners` straight onto the inner element (VTextarea reuses that `genInput` and only swaps the
tag), so `@click`/`@keyup` now bind to the `<textarea>` itself — closer to the event, not further.
`.native`'s failure mode is *silent*, which is why it was worth clearing before the cutover rather
than during it. The three direct `$vuetify.breakpoint` reads moved behind `viewportWidth()` /
`isLgAndUp()` in `general_utils.js`, so Vuetify 3's rename to `$vuetify.display` — where
`thresholds` also flip from upper bounds to lower — is one file.

### K1 — two dead components · **P2 · S** — OPEN

Surfaced while verifying J17, confirmed by a sweep over all 86 components (no others):

| file | lines | status |
|---|---|---|
| `components/EventType.vue` | 128 | no referrer in `src/`, **absent from `dist/`** |
| `components/schedule_overlap/ConfirmDetailsDialog.vue` | 292 | no referrer in `src/`, **absent from `dist/`** |

Both have been carried through at least three sweeps (the nickname display pass, the eslint backlog
clear, a contacts fix) without anyone noticing nothing renders them. Between them they carry ~15
Vuetify-2 constructs — `v-combobox`, `v-list-item-content`, `v-list-item-avatar`, two
expansion-panel pairs, an activator slot — that would otherwise be migrated for nothing.
`isLgAndUp` in `general_utils.js` exists only for `EventType.vue` and goes with it.

Absence from the built bundle is the decisive evidence: webpack never reached them, so no import
exists anywhere, dynamic or otherwise (there is no `require.context` or `<component :is>` in the
app). Unlike J13's `/ids` endpoint there is no external-contract argument — a Vue component is not
API surface.

### K2 — the cutover · **P0 · L** — DONE on branch `vue3` (2026-08-11), not yet merged

Five commits on `vue3`. The app runs on **Vue 3.5.41 / Vuetify 3.13.1 / vue-router 4.6.4 /
Vuex 4.1.0**, with `vue-meta`, `vue-worker`, `vue-template-compiler`, `vuetify-loader` and
`vue-cli-plugin-vuetify` all gone. `check:routes` and `check-signed-out` are ALL PASS, lint is
clean and all 395 unit tests are green.

**The point of the exercise is achieved: `npm audit` no longer lists `vue`.** The unfixable ReDoS
is cleared. What remains (4 under `--omit=dev`, 50 including dev) is Vue CLI's own webpack/babel
toolchain — the same class J12 assessed and left alone, and read by `npm ls`, not by the severity
column.

**Vuetify 3, not 4**, though 4.1.8 is `latest`: both lines ship on the same days and `v3-stable` is
maintained, the v2→v3 migration is the documented path, and v3→v4 later is a well-trodden second
hop. **Router 4, not 5**: vue-router 5 lists `pinia` among its peers and is a different design.

Per-phase notes are in the commit messages; the ones with the longest half-life:

- **`$vuetify.breakpoint` → `$vuetify.display` was a one-file fix**, exactly as J17 intended — but
  its failure mode is the thing to remember. `breakpoint` is `undefined` on v3, so `.name` threw
  inside a *computed*, and Vue 3 responds by logging and leaving the subtree blank. Three routes
  rendered nothing while the harness reported zero console errors, because **production builds
  strip framework warnings**. Every subsequent phase was driven from `npm run serve`.
- **Vuetify 3 wraps slot items**: in an `item`/`selection` slot the real object is `item.raw`.
  Silent when wrong — the row just renders empty.
- **v3's default item field is `item-title`, defaulting to `title`**, where v2's was `item-text`
  defaulting to `text`. Every `:items` of `{ text, value }` objects rendered `[object Object]`.
- **`v-date-picker` is a reimplementation, not a port.** v3 removed the `@mousedown:date` /
  `@mouseover:date` events drag-to-select was built on, and renders a `readonly` picker greyed out.
  Vuetify now owns the click and the component owns the drag. Two bugs there are worth knowing:
  `stopPropagation` on mouseup stops the browser synthesising the `click` Vuetify toggles on, and
  two toggles in one tick discard each other because each rebuilds from a `modelValue` the parent
  has not written back yet.
- **Vue 3 removed `el.__vue__` and `$children`**, so every instance-walking probe in
  `/root/tools/browser/` is dead. Read the DOM.

### K3 — the "dev-only" crash was a shipped bug · **P1 · S** — DONE 2026-08-11

Traced, and the label was wrong twice over.

`OverflowGradient` declares `scrollContainer` an `HTMLElement`. `NewEvent` passes a ref on a
**`<v-card-text>`** — and Vuetify 2's `VCardText` was a *functional* component (a ref gave the DOM
node) where Vuetify 3's is a real one (a ref gives the component proxy). In dev, prop validation
caught the mismatch and then **threw while formatting its own error message**, because `${proxy}`
cannot reach a primitive — so the mount aborted and the real fault never surfaced. In production,
validation is skipped, the panel mounted, and `mounted` threw
`TypeError: this.scrollContainer.addEventListener is not a function` **on every open of the
dialog**.

**The harness had been hiding it.** `pageErrors` read `args[0].value`, and CDP only sets `value`
for primitives — `console.error(someError)`, which is how Vue reports a throw from a lifecycle
hook, carries its message in `description`. Every argument is read now, with that fallback.

Switching it on surfaced one more error, which is **not** a regression: `/e/:id/responded` 400s on
a bare visit because `Responded.vue` POSTs the `email` from the query string. Recorded as a
per-route `expectConsoleErrors` regexp with its reason; anything not matching still fails.

Dev-build framework warnings are now **zero** (21 at the start of K2c).

Still open before this merges:

- **Nothing has been exercised against real Google/Microsoft calendar accounts**, which
  `compose.dev.yaml` cannot do — same gap J8 had, and it needs the same post-deploy check.
- Neither `main` nor `vue3` has been pushed.

---

## Workflow rules

Unchanged from `CLAUDE.md` and the two archives — these are the durable part, and every one of
them exists because ignoring it cost a debugging session:

- **Sync before changes.** Two machines push to `main`; start with `git fetch origin` and
  `git pull --ff-only`.
- **Green commits to trunk.** CI is post-hoc, not a merge gate: run the frontend unit tests,
  eslint, the production build, the backend suite, `go vet` and golangci-lint locally first.
- **Deploys are human-run** from the box with SSH access to `stf-thegathering`, via `./deploy.sh`
  on the build box. `origin/main` is ahead of what's live until then, and that's expected.
- **Cold-load signed-out** after any router or auth change (the E3 outage lesson).
- **Rebuild the dev containers before trusting a harness run.** `compose.dev.yaml` bakes the
  frontend bundle and the Go binary into their images, so `docker compose restart` re-runs the
  *old* artifacts: `docker compose -f compose.dev.yaml up -d --build frontend server`. The server
  registers its static routes only at boot, so a `dist` swap needs the restart to see new hashed
  filenames.
- **Look at the page.** Lint, unit tests and the build have all been green over a bug that only
  appears in a browser — `v-show` beaten by Tailwind's `important: true`, a purged class name
  built from a template string, a fifth tab putting a phone into horizontal scroll. The
  `CLAUDE.md` frontend section lists the ones already paid for.
