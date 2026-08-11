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
> **Part K is complete as of 2026-08-11 — every item shipped and deployed.** Production serves the
> Vue 3 stack. **K5's calendar-linking check is still outstanding and is not a code item**: it needs
> a human with a real OAuth consent flow. Nothing in the migration has ever exercised linking a
> Google/Microsoft/Apple calendar or importing availability from one.
>
> **Part L (opened 2026-08-11) is the post-migration review. L1–L8 are closed; L9–L15 are open**
> and are all P2/P3 — dependency and hardening work, nothing user-facing left in it.
>
> **Part M (opened 2026-08-11) is COMPLETE — M1–M7 all shipped 2026-08-11.** It is not a
> code review: it is a review of what can be verified on a dev box without a deploy, run after
> everything in the repo was confirmed green on the WSL box (395 unit tests, lint,
> `check:vuetify-props`, the Go suite, and `browser-check.sh` ALL PASS in 3m19s). Seven items in,
> this box can now do things it could not on the morning of the 11th: take a screenshot (M1), fail
> on a framework warning (M2), **sign in** (M3), hold a club worth clicking through (M4), say
> so when it is stale (M5), **mount a component** (M6), and re-run one section of the browser check
> in eleven seconds instead of rebuilding everything (M7).
>
> The numbers moved: **411 unit tests in 7.3s** (was 395), and the full browser check is **1m58s**
> with `--dev` at **57s** (was 3m19s / 2m) — same 64 assertions.
>
> Four of the seven found something the item had not predicted, all by testing rather than reading:
> M2 exposed an icon-font assertion that inferred "painted" from network shape and reported a
> working font as missing; **M3's proposed gate did not work at all** — the dev image ran
> `-release=true`, so `gin.Mode()` had been silently claiming production on every dev stack for
> months; M6 found that happy-dom lets a mounted component open a real socket to whatever is
> listening on the machine; and M7 found that the script's own failure path buried its
> one-sentence diagnosis under sixty lines of container log. None of the four was visible from the
> source.
>
> **The backlog was the Vue 3 migration (Part K).** The three inherited `P3` items (`TODO2.md` G2, G3, G4) were **closed
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

### K1 — two dead components · **P2 · S** — DONE 2026-08-10 (`d8a4fa8`)

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

**Shipped as `d8a4fa8` on 2026-08-10** — 425 deletions across three files: both components plus the
five `isLgAndUp` lines in `general_utils.js`, exactly as the entry called for. Lint, 395 unit tests,
the production build and `check:routes` were all green, and the work went out with the Part K deploys
(`d8a4fa8` is an ancestor of the live build). So it landed *before* the cutover, which is the point
of the item: neither component's ~15 Vuetify-2 constructs were ever migrated.

**This heading nevertheless read `OPEN` until 2026-08-11 — the commit shipped, the record didn't
move.** Recorded because of how it fails on re-reading: a stale `OPEN` on completed work does not
look stale. Checking it out the obvious way — grep `src/` for referrers, grep `dist/` — returns
*exactly* the emptiness the entry predicts, because the files are already gone; the verification
that was supposed to confirm the item is actionable confirms it just as well when the item is
finished. **`git log -- <the paths>` is the check that distinguishes the two**, and for a deletion
item it is the only one that does.

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

### K9 — light-theme leftovers, swept · **P2 · M** — DONE 2026-08-11

Three separate reports (K7's unreadable sub-calendar list, K8's white-veiled responses list) all
traced to the same source: colours the Fellowship redesign never converted. This is the deliberate
pass rather than one bug report at a time.

Method: inventory every hardcoded light value plus every use of the legacy Schej palette
(`off-white`, `light-gray`, `light-gray-stroke`, `gray` #BDBDBD, `very-dark-gray`), then judge each
against the surface it sits on. Fixed: the specific-times grid (white/grey blocks → translucent
brass / leather, legend updated to match), disabled cells, grid cell borders, eight grey button
outlines, and **ZigZag** — the torn-paper day-break edge, which drew its mask in `white` and was
the bright sawtooth running down the availability grid.

**Deliberately left**, each checked: the Google/Apple/Outlook/ICS brand buttons (white is the brand
requirement and index.css already darkens their labels), `SignInGoogleBtn`'s border, ExpenseDialog's
`ctx.fillStyle = "#ffffff"` (flattens receipt images before upload — a data concern, not a theme
one), and every `tw-text-white`, which is correct on green/brass.

**Evidence the sweep is complete for these:** `#BDBDBD`, `#dfdfdf` and `#DDDDDD99` no longer appear
anywhere in the compiled CSS. Compiled values were read rather than assumed, because Tailwind
purges on literal source text and these classes are built by string concatenation in
`timeslotStylingMixin` — precisely where that bites.

**Not visually verified: the specific-times editor**, a creation-time flow the harness could not
drive. Confirmed at the CSS level only.

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

### K4 — merged, pushed and DEPLOYED · 2026-08-11

`vue3` fast-forwarded into `main`, pushed (`2612696..d263f10`), and deployed with `./deploy.sh`.
Production serves **`d263f10`**; the previous release `2612696` is still on disk, so a rollback is
the symlink flip `deploy.sh` already documents.

**Verified against live production**, not just locally — `/root/tools/browser/verify_vue3_prod.js`,
33 assertions, all passing: every route, all five band tabs (exactly one panel each), the New
Gathering dialog, the date picker rendering a month, no `[object Object]` anywhere, no horizontal
scroll at 390px, and a clean console throughout. `check:signed-out` is ALL PASS against the live
URL too, so the E3 redirect-loop class is clear.

**J4's immutable caching survived the migration** — `/js/app.5db61c86.js` still comes back
`public, max-age=31536000, immutable`, which is the check that would have caught a bundler change
silently defeating `contentHashedAsset`.

Two things worth knowing for the next post-deploy run:

- **`git fetch` on the dev box failed with `server certificate verification failed. CAfile: none`**
  — git was not finding the system CA bundle, while curl was fine. Fixed with
  `git config --global http.sslCAInfo /etc/ssl/certs/ca-certificates.crt`. `deploy.sh` runs its own
  `git fetch` in preflight, so nothing could deploy until this was sorted.
- **The deploy health gate cannot see a broken frontend.** It pings `/api/health` and matches the
  version string, and the Go side is untouched by all of Part K — so it would have passed with
  every page blank. The browser check is the gate that matters for a frontend release.

### K5 — `@change` on Vuetify 3 gives the DOM event, not the value · **P0 · S** — DONE 2026-08-11

**The first real user-visible breakage from Part K**, reported from production: toggling a calendar
answered "There was a problem with toggling your calendar account!".

Vuetify 2's `change` on a checkbox/switch/select emitted the new **value**; Vuetify 3's is the
**native DOM event**. The calendar toggle POSTs that value to `/user/toggle-sub-calendar`, which
binds `Enabled *bool` with `binding:"required"` — an `Event` object fails binding, the server
answers 400, and the catch shows that message.

**K2c converted `@input` and never looked at `@change`, so the whole class survived the migration.**
Sixteen sites; fourteen were on Vuetify components and became `@update:model-value`. The two that
stayed are `<input type="file">` in `AvatarEditorDialog` and `ExpenseDialog`, where `change` really
is the native event — converting those would have broken avatar and receipt uploads.

Also silently broken, none of it reported: role changes on The Roll, expense split selection,
buffer time, working hours, reminder lead time, recurrence, and the four availability-grid
switches. Those four take `!!val`, and **`!!someEvent` is always `true`, so they could be turned ON
but never OFF** — which is what made this verifiable without a real calendar: the same bug, in a
control local dev can reach. Deployed as `f32178c`.

**The lesson for the next framework bump:** sweeping one event name is not sweeping the class.
Enumerate every handler bound on a library component and check each against the new API, which is
now done — what remains is `@click`, `@keyup*`, `@keydown*`, `@blur` and one `@click:outside`.

**STILL UNVERIFIED: the Google/Microsoft/Apple calendar paths.** `compose.dev.yaml` has no OAuth,
and the account used for the live check has no calendar linked, so nothing in this migration has
ever exercised calendar linking or availability import — despite `ScheduleOverlap` carrying 39 of
the 56 `.sync` conversions and J8's toggle→refetch chain. Linking a calendar needs a real consent
flow, i.e. a human. **That is the one outstanding risk from Part K**: link a calendar on the live
site and confirm events still appear on the availability grid.

---

## PART L — 2026-08-11 post-migration review (opened 2026-08-11)

> A full-codebase review run at `422241f`, immediately after Part K shipped. **All tooling is
> green** and stayed green through every finding below: 395 frontend unit tests pass, eslint is
> clean, the production build succeeds, `go vet`, `golangci-lint` (0 issues) and the Go suite all
> pass, and `govulncheck` reports exactly the one expected module-level finding (GO-2026-5932,
> the J12 baseline). Nothing here was caught by a check that already runs.
>
> **The theme is the one K5 named and did not finish.** K5's own lesson was "sweeping one event
> name is not sweeping the class" — it enumerated `@change` and left the *prop* surface
> unexamined. Vue 3 passes an unrecognised prop straight through as a DOM attribute and says
> nothing, so a Vuetify 2 prop that no longer exists is invisible to lint, to the unit suite, to
> the build, and to `check:routes` (which asks whether a route renders, not whether a button is
> the right size or in the right place). **L1–L4 are all that one class**, and L4 is the check
> that ends it.
>
> Findings are ordered by what they cost a user, not by effort.
>
> **Status 2026-08-11: L1–L8 are closed.** The check L4 asked for exists and is in CI; running it
> for real turned L3's six props into **75 across 25 files**, all swept. L5 put `check:routes` in
> CI against a booted stack, L6 fixed the docs, L7 declared all 111 emits, L8 self-hosted the icon
> font off jsdelivr. L9–L15 are untouched.

### L1 — `VForm.validate()` returns a Promise, so the submit guard never fires · **P0 · S** — DONE 2026-08-11

`if (!this.$refs.form.validate()) return` at `frontend/src/components/GuestDialog.vue:109` and
`frontend/src/components/NewEvent.vue:447`.

Vuetify 2's `validate()` returned a **boolean**. Vuetify 3's returns
`Promise<{ valid, errors }>` — confirmed in
`node_modules/vuetify/lib/components/VForm/VForm.d.ts:102` (`validate: () => Promise<{…}>`).
**A Promise is always truthy, so `!promise` is always `false` and the guard is dead code.**

It is currently *masked*, not harmless: both submit buttons also carry `:disabled="!formValid"`,
and that binding is the only thing enforcing validity today. **GuestDialog is the one where the
mask does not cover the hole.** Its design is deliberately "install the strict rules at submit
time, then validate": `submit()` assigns `nameRules` (name required · name already taken · not
ObjectID-shaped) and `emailRules`, then calls `validate()` on `$nextTick`. The button was already
enabled when it was clicked, so those rules never gate anything — the `submit` event is emitted
regardless. The ObjectID-shaped-name rule exists *because the server rejects that name* (a guest
response keyed by an ObjectID-shaped string can collide with a member's), and its comment says it
is "mirrored here so the guest sees why rather than a generic failure". That mirror is off.

Fix in both places: `const { valid } = await this.$refs.form.validate(); if (!valid) return` —
and make the caller `async`. While there, drop `lazy-validation` (see L3): it was Vuetify 2's way
of saying "don't validate until I ask", which is precisely what this pattern depends on, and in
Vuetify 3 the replacement is `validate-on="submit"`.

**Done as described, except for `validate-on="submit"` — that replacement would have broken both
forms.** `lazy-validation` was simply deleted instead (it was inert, so deleting it changes
nothing), leaving Vuetify 3's default `validate-on="input"`. The reason is in
`vuetify/lib/composables/validation.js`: with `validate-on="submit"` neither the `input` nor the
`blur` watcher is installed, so **nothing re-validates until `validate()` is called by hand**. A
pristine field with rules reports `isValid === null`, the form's `v-model` aggregates to `null`,
and `:disabled="!formValid"` — which both submit buttons carry — would have latched the button
disabled forever, with no keystroke able to clear it. Keeping the default preserves today's
behaviour exactly, and the awaited `validate()` is what makes the submit-time rules bite.

That is also why the pattern works at all: **a rules change does not trigger validation** (the
only watchers are on the field's value and its focus), so GuestDialog's "install strict rules,
then validate on `$nextTick`" is still the right shape — it just has to await the result.

**Browser-verified** on the local stack, signed in, against the real dialog
(`/root/tools/browser/verify_L1_L4.js`):

```
PASS  Continue is enabled with an empty name (so the rules are the only gate)
PASS  blank name refused (messages: ["Name is required"])
PASS  no response POSTed on blank submit (0)
PASS  ObjectID-shaped name refused (messages: ["That name isn't allowed"])
PASS  no response POSTed for the ObjectID-shaped name (0)
PASS  valid name submits (POSTs: 1) · dialog closed after a valid submit
```

### L2 — the phone's "+" button is a rectangle 1,300px below the fold · **P1 · S** — DONE 2026-08-11

`frontend/src/components/BottomFab.vue` — the floating create button on `/home`, phone only.

It relies on two Vuetify 2 `VBtn` props that no longer exist: **`fab`** (round icon shape) and
**`fixed`** (`position: fixed`). Both now fall through as inert DOM attributes. The Tailwind
classes on the same element — `tw-bottom-4 tw-left-0 tw-right-0 tw-mx-auto` — were only ever
*positioning* for the `fixed` the prop supplied; with static positioning they do nothing at all.
There is no `tw-fixed`.

**Browser-verified** against the running local build at 390×844, signed in, on `/home`:

```
position: relative · border-radius: 6px · rect.top: 2186 · viewport height: 844
```

So the button is square, statically positioned, and sits 1,300px past the bottom of the screen —
reachable only by scrolling the entire dashboard. `check:routes` cannot see this: the element
renders, and it renders quietly.

Fix: `<v-btn icon position="fixed" class="tw-fixed …">`, then re-check at 390px.

Fixed as `<v-btn icon size="large" class="tw-fixed … tw-z-30 …">` — `tw-fixed` alone rather than
also `position="fixed"`, because Tailwind's `important: true` makes the utility win outright and
one source of truth beats two. `size="large"` restores the 56px touch target v2's `fab` gave, and
`tw-z-30` restores the stacking `.v-btn--fixed` used to carry (under the header's `tw-z-40`).
`OverflowGradient.vue:9` had the same dead `fab` and is now `icon` too.

**Browser-verified** at 390×844, signed in, on `/home` — the same probe that found it:

```
position: fixed · border-radius: 50% · rect 772–828 · viewport height: 844
```

### L3 — six more Vuetify 2 props survived the K2d styling pass · **P1 · S** — DONE 2026-08-11

Same mechanism as L2, smaller blast radius — each renders at the wrong size or variant, silently.
Found by the L4 check; all confirmed against Vuetify 3's shipped prop declarations.

| Site | Prop | What it should be |
|---|---|---|
| `event/EventDescription.vue:59,62` | `:small="isPhone"` on `v-btn` | `:size="isPhone ? 'small' : 'default'"` — the description editor's ✓/✕ buttons never shrink on a phone |
| `event/EventHeader.vue:52` | `:outlined="!isPhone"` on `v-btn` | `:variant="isPhone ? 'text' : 'outlined'"` — renders elevated on desktop instead |
| `event/GatheringRsvp.vue:20` | `:outlined="myStatus !== opt.value"` on `v-btn` | `:variant="…"` — unselected RSVP buttons lose their outline (the Tailwind `:class` still carries the colour, so this one degrades rather than breaks) |
| `LocationInput.vue:9` + `:48` | a declared `solo` prop, forwarded as `:solo` to `v-combobox` | `variant="solo"`; note the prop is **LocationInput's own** and its one caller (`NewEvent.vue:47`) still passes it, so both ends move together |
| `NewEvent.vue:191` | `solo` on `v-btn-toggle` | not a v3 prop at all — drop it |
| `GuestDialog.vue:17`, `NewEvent.vue:26` | `lazy-validation` on `v-form` | `validate-on="submit"` (and see L1 — this is load-bearing for GuestDialog) |

Two harmless leftovers to sweep at the same time: `:dark="formValid"` on `GuestDialog.vue:49`
(the theme is global in Vuetify 3) and a bare `dark` on a plain `<div>` at `App.vue:14`, which
was never a Vue thing.

**It was seventy-five, not six.** The L4 check as actually written — every `.vue` file, every
`v-*` tag, diffed per component against that component's own declared props — printed **96 lines
on a clean tree**, of which 21 were `title` and `required`, both genuine native attributes that
Vuetify routes onto the DOM (`filterInputAttrs` sends everything except class/style/id/inert/data-*
to the inner `<input>`). Those are now on the check's allowlist. **The remaining 75 were all real**,
across 25 files, and every one of them is fixed here. The six in the table above were a sample, not
the set — the review reported what one narrower pass had surfaced, and the entry should have said
so.

The full class, by translation:

| Vuetify 2 | Vuetify 3 | Sites |
|---|---|---|
| `small` / `x-small` on `v-btn`, `v-chip` | `size="small"` / `size="x-small"` | 14 |
| `dense` on fields, lists, checkboxes, switches | `density="compact"` | 15 |
| `left` / `right` on `v-icon` | `start` / `end` | 11 |
| `left` on `v-menu` | `location="bottom end"` | 2 |
| `right` on `v-menu` | dropped (Vuetify 3's default placement is what they already render as); the one real submenu — `EventItem.vue`'s hover menu — became `location="end"` | 5 |
| `top` on `v-tooltip` / `v-snackbar` | `location="top"` | 3 |
| `outlined` on `v-btn` | `variant="outlined"` | 2 |
| `off-icon` on `v-checkbox` | `false-icon` | 3 |
| `row` on `v-radio-group` | `inline` | 2 |
| `absolute` / `fixed` / `fab` on `v-btn` | `tw-absolute` / `tw-fixed` / `icon` | 5 |
| `two-line` on `v-list-item` | `lines="two"` | 1 |
| `background-color` on `v-text-field` | `bg-color` | 1 |
| `solo` on `v-combobox` / `v-btn-toggle` | `variant="solo"` / dropped (never a v3 prop) | 2 |
| `lazy-validation`, `dark`, `:menu-props` on a `v-text-field` | dropped — no v3 meaning here | 5 |

`LocationInput`'s own `solo`/`dense` props kept their names — they are that component's API, not
Vuetify's — and are translated to `variant`/`density` on the way through, so its callers did not
have to move with it.

**Verified**: the checker is green, `check:routes` passes end to end (every route, every band tab,
both viewports, no console errors and no framework warnings), and the visibly-changed surfaces
were looked at — desktop "Copy link" renders `v-btn--variant-outlined`, the snackbar lands at
`top: 8px`, the new-list radios share a row, phone Settings fields are `v-input--density-compact`,
the description editor's ✓/✕ are `v-btn--size-small`, and the location field is
`v-field--variant-solo`.

*Noticed while verifying, not fixed here:* the location combobox renders **no `placeholder`
attribute at all** ("Where? (optional)" never appears), while the plain `v-text-field` beside it
does. Independent of `variant` — it wants its own look, in Part L's tail or a Part M.

### L4 — the check that ends this class: lint Vuetify props against Vuetify · **P1 · S** — DONE 2026-08-11

**L2 and L3 are not six mistakes, they are one missing check**, and the check is about forty
lines. Parse every `.vue` template with `@vue/compiler-sfc`, collect the attribute names bound on
any `v-*` tag, and diff them against the prop names declared across
`node_modules/vuetify/lib/components/**/*.d.ts`. Skip native passthroughs (`maxlength`,
`inputmode`, `autocomplete`, `data-*`, `aria-*`, `class`/`style`/`key`/`ref`).

Run from a clean tree during this review it printed exactly six lines and nothing else — every
finding in L2 and L3, no false positives. It is worth having as `npm run check:vuetify-props`
in `frontend-ci.yml`, because it is the only thing in the repo that would have failed on them,
and because it costs nothing and keeps working across the next Vuetify minor.

The same shape generalises: the authority for "does this prop exist" is the library's own type
declarations, which are already on disk. Nothing else in the pipeline consults them.

Shipped as `frontend/scripts/check-vuetify-props.js` → `npm run check:vuetify-props`, and wired
into `frontend-ci.yml` between Lint and the unit tests. 150 lines rather than forty, most of it
the native-attribute allowlist and the comment explaining why the check exists at all.

How it reads Vuetify: each component's **full, flattened** prop list is the `Defaults` constraint
of its `makeVXxxProps` factory in the shipped `.d.ts` (`makeVBtnProps: <Defaults extends { density?:
unknown; size?: unknown; … }>`), so the check brace-matches that object and takes its depth-1 keys.
153 components resolve this way; the handful built by `createSimpleFunctional` (`VSpacer`,
`VCardTitle`, …) declare no props of their own and are skipped rather than guessed at. Tags are
mapped by name (`v-btn-toggle` → `VBtnToggle`), only `:foo`/`v-bind:foo` and plain attributes are
considered (never `v-model`, `v-if`, `v-slot`, `v-on` or `v-bind="$attrs"`), and non-Vuetify tags
are left alone.

**Its one real design decision is the native allowlist**, because that is the only place it can
produce a false positive — and getting it wrong is how a check like this gets switched off. The
rule it encodes: Vuetify hands unmatched attributes to the DOM, and for the input components
`filterInputAttrs` hands them to the inner `<input>`, so `maxlength`, `required`, `inputmode`,
`title` and friends land exactly where they are meant to and are not findings.

### L5 — `check:routes` is the safety net and CI does not run it · **P1 · M** — DONE 2026-08-11

`frontend/scripts/check-routes.js` states the gap in its own header, and it is right: the unit
suite is 395 pure-JS tests with `environment: "node"`, **no `.vue` file is imported anywhere and
`@vue/test-utils` is not a dependency**, so the suite stays green through a total rendering
failure. Lint and the build are no better. `check:routes` is the only thing that looks at a page —
and it runs by hand, because it needs a booted stack, a session cookie from
`server/tools/mintsession`, and an event id.

Every browser-only bug this repo has paid for is in that gap: `v-show` beaten by Tailwind's
`important: true`, a purged class name built from a template string, a fifth tab putting a phone
into horizontal scroll, K3's shipped dialog crash, K5's silently-broken toggles, and now L2.

Make CI do it: `docker compose -f compose.dev.yaml up -d --build`, mint a superAdmin cookie, seed
one event, run `check:routes` against it. The two halves are complementary and both are needed —
`check:routes` catches "it did not render", L4 catches "it rendered wrong".

Shipped as `scripts/browser-check.sh` (boot → seed → mint → check → tear down) and
`.github/workflows/browser-ci.yml`. **61 assertions, ALL PASS**, ~1m45s locally on a warm layer
cache. Triggered by `frontend/**` **and** `server/**`: the frontend renders what the API returns,
so a handler change breaks pages as easily as a component change does.

**Validated by breaking the app on purpose**, which is the only evidence worth having for a check
like this: adding `class="tw-block"` to the `v-show`-toggled Lists panel in `Event.vue` — the exact
Tailwind-`important` interaction that this repo has already shipped once — turned five assertions
red with `saw 2`, and exit 1. Reverting turned them green again.

**"Seed one event" was the wrong instinct, and the failures said so.** A minimal fixture fails the
check for reasons that have nothing to do with the app:

- `hasSpecificTimes: true` with `times` empty puts `ScheduleOverlap`'s `mounted()` into
  `SET_SPECIFIC_TIMES` — the creator's click-and-drag screen. Correct behaviour, and the event page
  then has no band tabs, no "Mark availability" and no "Schedule event" for the check to find.
- "Schedule event" needs `numResponses > 0`, so the fixture casts one availability — as a *second*
  member, because the action button reads "Edit availability" once the signed-in user has responded.
- An empty Fellowship and an empty Chronicle render their empty states perfectly, and assert nothing
  about the list rendering that actually breaks. The fixture is a populated club: five members, ten
  Chronicle entries, three gatherings. The alternative was tuning the thresholds down to whatever an
  empty database happens to produce, which would have quietly weakened the check for everyone.

The gathering is created **over the API**, not inserted into Mongo, so the fixture cannot drift away
from what the app writes; its dates are relative to today, because a hardcoded date eventually falls
into the past and changes what renders. The Chronicle entries are the one hand-built fixture — the
alternative is waiting on the reminder scheduler's tick, which is not something a CI run should do.

Two bugs in the script itself, both found by running it rather than reading it, both recorded in the
comments where they happened:

- The teardown used a **relative** `-f compose.dev.yaml` and ran from the EXIT trap, by which point
  the script had `cd`'d into `frontend/`. It failed, its own `|| true` swallowed the failure, the
  stack stayed up, and the *next* run seeded on top of the old database and died on a duplicate-key
  error on the allowlist's unique email index — a message that points nowhere near the cause. Now an
  absolute path, `npm --prefix` instead of `cd`, and a `down -v` on the way *in* as well as out.
- **`mongosh` prints a failed insert on stdout and still exits 0**, so a broken seed produced empty
  ids, every request 401'd, and the first thing to complain was a JSON parse error three steps
  later. The ids are now checked against `^[0-9a-f]{24}$` before anything uses them.

`compose.dev.yaml`'s two published ports became `${DEV_HTTP_PORT:-3002}` / `${DEV_MONGO_PORT:-27017}`
so the check can run its own project on `:3010`/`:27018`. Defaults unchanged, so nothing else moves —
and on these machines the dev stack on `:3002` usually holds a restored production dump, which this
must never seed into, and certainly never `down -v`.

Docs updated with it: `DEVELOPMENT.md` no longer says these checks are "**not** in CI" (it said
"two" checks where there are three), and now records what the fixture needs and why, each line of it
an assertion that failed first.

### L6 — the docs still describe a Vue 2 app · **P1 · S** — DONE 2026-08-11

Same class as J9, and it matters more than usual because `CLAUDE.md` is what the next session
reads before touching anything.

- `CLAUDE.md:30` — "`frontend/` — Vue 2 + Vuetify + Tailwind"; `CLAUDE.md:85` — "### Frontend
  (Vue 2 SPA)". `README.md:18` links Vue 2.
- `CLAUDE.md`'s backend section still says the `NoRoute` handler "walks … falls back to a
  `NoRoute` handler that injects per-route OG meta tags (e.g. for `/e/:eventId` it looks up the
  event to set the title and OG image)". **E3 deleted that** — `noRouteHandler` in
  `server/main.go:327` now serves a static shell with no DB lookup, on purpose, because per-event
  OG titles leaked gathering names to anyone who guessed a short id.
- The frontend gotchas list should absorb what Part K and Part L cost: `@change` gives the DOM
  event now (K5), `VForm.validate()` is async (L1), and **an unknown prop on a Vuetify component
  is silently discarded** (L2/L3) — that last one is the general rule the other two are instances
  of.

All three done. `CLAUDE.md` now reads "Vue 3 + Vuetify 3 + Tailwind (Vue CLI 5 / webpack)" and
"### Frontend (Vue 3 SPA)"; `README.md` links `vuejs/core`. `DEVELOPMENT.md` needed nothing — it
was written after the migration and already refers to Vue 3.

Two things were added beyond the item as filed, both because they are decisions a future session
would otherwise re-litigate from scratch:

- **Don't migrate to Vite**, with the reason (its base64url dash-separated hashes never match the
  immutable-cache regex J4 added for Vue CLI's `app.<hash>.js`, so caching switches off silently).
  That was reasoned out in Part K's Phase 0 and lived only in the backlog.
- The `noRouteHandler` correction records the **`Cache-Control: no-cache` on the shell** as well as
  E3's deletion of the OG lookup — same handler, and it is the fix for the stale-`index.html`
  breakage that once took out the OTP login after a run of deploys.

The gotchas list leads with the general rule (**Vue 3 discards an unrecognised prop silently, and
on a plain HTML tag it becomes a literal DOM attribute**), then K5 and L1 under it, each with the
trap that makes the naive fix wrong: for K5, `<input type="file">` must keep `@change`; for L1,
`validate-on="submit"` would latch every submit button disabled forever.

### L7 — 111 emits are undeclared, which in Vue 3 also makes them DOM listeners · **P2 · S** — DONE 2026-08-11

`vue/require-explicit-emits` reports 111 sites. This is not style: in Vue 3 an event that is not
in `emits` stays in `$attrs` and is **additionally** bound as a native listener on the
component's root element, so a component emitting a DOM-named event without declaring it fires
its parent's handler twice.

**Nothing is live today** — the two components that emit DOM-named events (`NewEvent` emits
`input`, `GuestDialog` emits `submit`) both declare `emits`, and every other emit uses a custom
name that no DOM element listens for. It is a trap rather than a bug: the next `$emit("click")`
written without a declaration double-fires, and nothing warns. Turn the rule on and clear it.

Done: 111 events declared across **30 components**, and `vue/require-explicit-emits` is now
`"error"` in `.eslintrc.cjs` (cherry-picked out of `vue3-recommended` exactly as L13 recommends,
so the rule that closes this item is what keeps it closed). No behaviour was expected to change —
`$emit` finds the parent's handler in `$attrs` whether or not the event is declared, so the
declaration only removes the *extra* native binding.

**One did change, and it is the finding.** `SignInNotSupportedDialog`'s root is a bare
`<v-dialog :model-value="modelValue">` with no `@update:model-value` — its only `$emit` is on the
"Ok" button. Esc and a click on the scrim closed it **through the fallthrough**: the parent's
`v-model` listener sat in `$attrs`, landed on the root `v-dialog`, and its dismiss went straight
to the parent. Declaring `emits` takes that listener out of `$attrs`, and because the dialog is
prop-controlled it cannot close itself — so both dismiss gestures would have died silently, with
lint, the unit suite, the build and `check:routes` all green. The root now forwards explicitly
(`@update:model-value="(e) => $emit('update:modelValue', e)"`), which is what every other dialog
in the tree already did.

So the general shape to check when declaring an event on any component: **is the root a component
that emits the same event, and does the template forward it?** If it doesn't, the declaration is
what breaks it. Roots audited: the other nine `update:modelValue` emitters all either forward
explicitly (`UnsavedChangesDialog`, `MarkAvailabilityDialog`, `AvailabilityTypeToggle`,
`LocationInput`), route through a local `computed` setter (`FeatureNotReadyDialog`,
`TeamsNotReadyDialog`), or have a plain-element root where the fallthrough was never live
(`DatePicker`, `ExpandableSection`, `SlideToggle`, `TimezoneSelector`).

Verified with the full local gate plus `scripts/browser-check.sh` — 44 assertions, all pass,
which is the only check that would have caught the dialog. Note it needs a Chrome: on this box
there is none on `PATH`, so it wants
`CHROME_PATH=/root/.cache/ms-playwright/chromium-1234/chrome-linux64/chrome`.

Shipped and verified live on `8149dcb` (`tools/browser/verify_L7_prod.js`, off-repo): the New
Gathering dialog opens, renders and **closes** again, an event page renders clean, and the
timezone selector still opens its menu — the `update:modelValue` paths, end to end, on
production. One thing that surfaced while writing it, worth knowing before someone asserts on it:
`NewDialog`'s root is `persistent`, so **Escape does not close it** and is not meant to — it
closes on a click outside, through `handleDialogInput`, so the unsaved-changes dialog gets a say.

### L8 — the icon font is `@latest`, from a CDN, unpinned and unverified · **P2 · S** — DONE 2026-08-11

`frontend/public/index.html:34`:

```html
href="https://cdn.jsdelivr.net/npm/@mdi/font@latest/css/materialdesignicons.min.css"
```

**`@latest`.** Every page load resolves whatever jsdelivr is serving under that tag at that
moment — a third party can change what renders in this app with no deploy on our side, and there
is no SRI hash to catch it. It is also load-bearing: `src/plugins/vuetify.js` selects the font
icon set (`mdi`, not `mdi-svg`) precisely because the glyphs arrive this way, and all 69 `mdi-*`
names in the app render as blank squares if the request fails, with nothing logged.

This sits badly beside the deliberate strip-third-party-scripts work. Pin the version as the
minimum fix; self-hosting the woff2 removes the dependency and the request.

Done: **self-hosted, which is the fix that also pins it.** `@mdi/font` is now a dependency at an
exact version (`"7.4.47"`, no caret — a floating icon font is the thing this item was about),
imported in `src/main.js`, and the `<link>` to jsdelivr is gone from `public/index.html`. Webpack
emits the font as `dist/fonts/materialdesignicons-webfont.<8hex>.woff2`, which is same-origin and —
because the hash matches `contentHashedAsset` in `server/main.go` — served `immutable` for a year,
where the CDN copy was revalidated against a third party on every page load. One fewer DNS lookup,
TLS handshake and off-origin request on the critical path; the app no longer renders whatever
jsdelivr decides `@latest` means this week.

Two things worth knowing before touching this again:

- **The CSS is 340 KB of it** (7,447 icon classes for the 69 we use), and it landed in
  `chunk-vendors.css`, which went 348 KB → 683 KB raw, 97.8 KB gzipped. Subsetting it is possible
  and deliberately not done: Vuetify's own `aliases` name glyphs no grep of our source will find, so
  a subset is the Tailwind-purge trap (`tw-pl-${n}` emitting no CSS) with blank squares instead of
  missing padding. Cloudflare compresses it at the edge; the CDN copy cost about the same over the
  wire.
- **`dist` grew 7.3 MB → 12 MB** because the package's `@font-face` also lists eot, ttf and woff, so
  webpack emits all four. Only the woff2 is ever fetched by a browser that can run this app. The
  hashes are content-derived and so stable across builds, which is why `deploy.sh`'s rsync sends
  them exactly once.

`check:routes` now asserts both halves of this on `/home`, because the failure mode is silent —
every `mdi-*` name renders as a blank square with nothing logged: **the icon webfont loaded** and
**the icon webfont is self-hosted** (every `materialdesignicons` resource entry is same-origin, and
one of them is a content-hashed woff2 — so a regression to *any* CDN fails, not just to jsdelivr).

Writing the first of those is the part with a lesson in it. `document.fonts.check()` is the obvious
API and the wrong one: with no matching `@font-face` at all it returns **true**, so it passes in
exactly the case worth catching. Reading the face out of `document.fonts` and asserting
`status === 'loaded'` is the next thing you'd write, and it is **flaky** — the live verification
caught it failing on `/home` while the glyphs were plainly painting: on the second navigation of a
session the font comes from cache and the face was still reporting `unloaded`. What survives
caching is the response size — a cache hit reports `transferSize` 0 but still reports
`decodedBodySize` — and it is diagnostic too, since a missing font hits the SPA fallback and
"decodes" to a couple of KB of `index.html` rather than 400. So the assertion is: the face is
declared, none errored, and the hashed woff2 decoded to >100 KB.

Verified with the full local gate — eslint, `check:vuetify-props` (153 components, no unknown
props), 395 unit tests, the production build — plus `scripts/browser-check.sh`: 63 assertions, all
pass, the two new ones included. Shipped on `946d825` and verified live
(`tools/browser/verify_L8_prod.js`, off-repo, twice): on landing and `/home` the font loads from
our origin, an icon paints, and no request to jsdelivr/unpkg/cdnjs is made at all. `curl` agrees —
`/fonts/materialdesignicons-webfont.fbaef2a9.woff2` comes back `200 font/woff2`, 403,216 bytes,
`cache-control: public, max-age=31536000, immutable`.

### L9 — three Google Fonts families on the critical path · **P3 · S**

`frontend/public/index.html:36–44` — `preconnect` to `fonts.googleapis.com` / `fonts.gstatic.com`
plus two stylesheet requests (DM Sans; Cormorant Garamond + EB Garamond + Cinzel). Every page load
of an invite-only, self-hosted club app sends every member's IP and UA to Google, and blocks first
paint on a third party. Self-hosting the woff2 files is mechanical and removes both. Lower
priority than L8 only because the version is not floating.

### L10 — four build-toolchain vulnerabilities, none shipped · **P2 · S** — DONE 2026-08-11

`npm audit`: 3 moderate, 1 high — webpack's `AutoPublicPathRuntimeModule` DOM-clobbering XSS, two
`buildHttp` allowlist-bypass SSRF advisories, and `serialize-javascript` RCE/CPU-exhaustion
reached via `terser-webpack-plugin`. **All are build-time only** — none of this code is in `dist`,
and the webpack XSS gadget needs `AutoPublicPath`, which this build does not use. `npm audit fix`
resolves all four without a major bump. Worth doing because J12's whole point was that nobody had
audited what we depend on, and the Go side is currently clean (govulncheck: 0 called, the one
expected module-level GO-2026-5932).

While here: add `npm audit` and `govulncheck` to CI as a scheduled job, so J12 and L10 are not
both "someone remembered to look".

**DONE 2026-08-11 — and the count above was wrong by an order of magnitude.**

`npm audit` reported **48** (6 low, 21 moderate, 18 high, 3 critical), not "3 moderate, 1 high".
The item was written against a stale reading. The *conclusion* survived — every one of the 48 is
build- or dev-time (webpack, babel, vitest/vite dev server, webpack-dev-server's
express/http-proxy-middleware, svgo, postcss) and none of it is in `dist` — but "S" was sized
against four advisories and the real tree had twelve times that.

`npm audit fix` (no `--force`), run to convergence, takes **48 → 14** and **changes no line of
`package.json`**: it is a lockfile-only change, 2,194 insertions / 2,993 deletions. All three
criticals and 16 of the 18 highs are gone. Trialled first on a copy of `package.json` +
`package-lock.json` under `--package-lock-only`, so the shape was known before anything in the
repo moved.

**The remaining 14 must not be "fixed", and this is the part worth keeping.** All of them are
rooted in `@vue/cli-service` and its plugins — Vue CLI 5, unmaintained upstream. npm's offered
remediation for nearly every one of them is `@vue/cli-plugin-babel@3.12.1`: a **downgrade to Vue
CLI 3**, flagged `isSemVerMajor`. `npm audit fix --force` here is not a fix, it is an outage — it
would take the build with it, and K2 already settled that this app stays on Vue CLI / webpack
(repo layout, TODO3 K Phase 0). Anyone who runs `--force` on a red audit and pushes the result
will find out in CI at best.

So the gate is not "zero advisories". `scripts/dependency-audit.sh` blocks on the three things
that can actually reach a user, and prints the fourth as information:

- **a vulnerability in a SHIPPED frontend dependency** — `npm audit --omit=dev --audit-level=low`,
  which drops the entire Vue CLI toolchain and leaves what ends up in `dist`. Currently **0**. This
  is the assertion with real signal, and it is stable: it does not churn every time a new
  webpack-dev-server advisory lands.
- **a Go vulnerability our code CALLS** — govulncheck's own exit code, which is non-zero only for
  called symbols.
- **a change to the module-level Go findings** — govulncheck exits 0 for these, so a new one would
  otherwise arrive silently. J12's "expect exactly one finding, anything beyond that is new" is now
  the `GO_ALLOWLIST` in that script, with the reason for the entry sitting next to it rather than
  in prose in CLAUDE.md.
- **the dev-toolchain total**, printed every run, failing never.

Both blocking assertions were shown capable of failing before being trusted — the M2 lesson, and
it earned its keep immediately. Perturbing `GO_ALLOWLIST` fails as intended. Moving
`@vue/cli-service` into `dependencies` fires the shipped-dependency gate with the full postcss
chain printed. And running the script from outside the repo revealed that a failed `cd server` (or
any package-load abort — **exactly how J12's bare `./...` fails**) came back as *"our code CALLS a
vulnerable symbol"*, sending the reader after a CVE that does not exist. The script now separates
"the scan found something" from "the scan never ran".

CI: `.github/workflows/dependency-audit.yml` — **weekly cron** (Mondays 07:00 UTC),
`workflow_dispatch`, plus push/PR on the four dependency files and the script itself. The cron is
the load-bearing trigger: a new advisory is published against code that has not changed, so a
push-triggered job would have caught neither J12 nor L10. One job runs the same script you run
locally, so the pass/fail policy lives in the script rather than in the workflow file where nobody
running it by hand would see it.

Verified with the full local gate on the new lockfile: eslint clean, `check:vuetify-props` clean
(153 components), **411** unit tests green across both tiers, the production build clean (asset
hashes still Vue CLI's `app.193e6bbb.css` shape, so J4's immutable-cache regex still matches), and
`scripts/browser-check.sh` **ALL PASS**.

Left open deliberately: the 14 dev-tree advisories, which close when Vue CLI does — not a task,
a consequence. `vue-template-compiler` (Vue 2's compiler, pulled in by `vue-loader` under
`@vue/cli-service`) is among them and is not used by anything: this app compiles through
`@vue/compiler-sfc`.

### L11 — two collections' indexes exist only in migration scripts · **P3 · S**

`server/db/init.go` creates every other invariant-bearing index at boot through `ensureIndex`,
with a comment explaining what each one guarantees. Missing: **`comments`** (queried by `eventId`,
sorted by `createdAt` — `db/comments.go:26`) and **`eventResponses`**, both of which exist only
because a dated script under `server/scripts/` created them on the live database.

At 30–40 members this is not a performance problem and should not be filed as one. It is a
reproducibility gap: a fresh install, or the restored-dump dev box, silently runs collection scans
where production does not — so a query plan verified locally is not the query plan in production.
Move them into `ensureIndex` alongside the rest.

### L12 — Swagger UI is served publicly in production · **P3 · S**

`server/main.go:228` mounts `router.GET("/swagger/*any", …)` unconditionally, outside every auth
group. On an app where **all** event access requires sign-in (E3), the complete API surface —
every route, every model shape, every field name — is readable by anyone who visits
`/swagger/index.html`. Gate it on `gin.Mode() != gin.ReleaseMode`, or behind the admin role.

### L13 — `vue3-recommended` is not worth adopting; three of its rules are · **P3 · S**

Recorded so the follow-up noted in `.eslintrc.cjs` ("tightening to `vue3-recommended` is a
follow-up worth doing on a quiet diff") can be closed with a decision rather than re-litigated.

Measured: `vue3-recommended` produces **1,941 warnings, 0 errors, 1,712 of them auto-fixable** —
and every one sampled is formatting that fights prettier (`html-indent`,
`singleline-html-element-content-newline`, `attributes-order`, `max-attributes-per-line`). Zero
correctness signal for a very large diff.

The signal is in individual rules, so cherry-pick instead: `vue/require-explicit-emits` (L7, 111
real hits) and `vue/no-unused-properties` (61 hits, worth a look). Note `vue/no-unused-refs`
reports 4 and **all four are false positives** — `name-field`, `emailInput`, `datePicker` and
`calendar` are read from mixins, which the rule cannot see. Don't enable it.

### L14 — a transition that has never had any CSS · **P3 · S**

`schedule_overlap/RespondentsList.vue:85` — `<transition-group tag="div" name="list">`. No
`list-enter*` / `list-leave*` rule exists in any `.vue`, `.css` or `.scss` in the tree, nor in the
compiled bundle, so the respondent list has never animated. Pre-dates the migration.
(`CalendarEventBlock`'s `:name="transitionName"` is fine — it resolves to `fade-transition`, which
Vuetify ships and which is present in the built CSS.) Either add the rules or drop the wrapper.

### L15 — dependency drift and a Go version skew · **P3 · S**

- `sass-loader` is on **10.5.2** against a latest of 17 — six majors behind, and the one dep here
  whose age is likely to bite during a future toolchain change. `core-js` 3.42→3.50,
  `dayjs` 1.11.10→1.11.21, `tailwindcss` 3.4.1→3.4.19, `autoprefixer`, `ws` are all routine patch
  drift. Explicitly **not** recommended: tailwind 4, vuetify 4, vue-router 5, eslint 10 — those
  are majors and belong in their own scoped piece of work, not a sweep.
- `backend-ci.yml` pins Go **1.25** (matching `go.mod`'s `go 1.25.0`); the dev box runs
  **1.26.5**. Local builds and CI builds are on different compilers, which is exactly the
  arrangement that produces a green local run and a red CI one. Pick one and state it.
  **Correction 2026-08-11:** that is one box, not both. The WSL box runs **go1.25.5**, matching
  `go.mod` and CI exactly. The skew is on the other machine only — so "pick one" means bringing
  the 1.26.5 box down to 1.25, not moving CI.

---

## PART M — local testability (opened 2026-08-11)

> **Not a code review — a review of what can be verified here, without a deploy.** Run at
> `3db21b5a`, on the WSL box, immediately after Part L. Everything in the repo is green on this
> machine and was measured rather than assumed: **395** frontend unit tests (1.5s), eslint clean,
> `check:vuetify-props` clean (153 components), the full Go suite green against a throwaway Mongo,
> and `scripts/browser-check.sh` **ALL PASS in 3m19s** — 62 assertions, images built from scratch,
> stack seeded, torn down. Toolchain present: Docker 29.7.2, Go 1.25.5, Node 20.20.2/npm 10.8.2,
> `google-chrome`, `mongosh`. **Nothing below is blocked on a missing tool.**
>
> **The theme is the tier that is missing, and the two things that are silently untrue.** The repo
> has a 1.5-second tier that cannot see a rendered page and a 200-second tier that sees everything;
> there is nothing in between, and the top workflow rule — "look at the page" — has no
> implementation an agent can run at all, because nothing here can take a screenshot (M1). Of the
> assertions that *do* run, twelve are incapable of failing (M2). And both halves of this machine
> were stale on arrival, silently, in a way that manufactures fake regressions (M5).
>
> Ordered by how much each one widens what can be checked before a deploy.

### M1 — nothing local can take a screenshot · **P1 · S** — DONE 2026-08-11

The one workflow rule the repo puts above the others is **"look at the page"**, and there is no
way to do it here except by being a human with a browser. `browser-check-lib.js` already has the
page, the session cookie and viewport control; `Page.captureScreenshot` is one CDP call it does
not make. The only screenshot code in the tree is `frontend/scripts/verify_f9_prod.js:296,326` —
Playwright, against **production**, needing a real signed-in prod session. So the rule as written
today reads: to look at the page, deploy it.

Add `frontend/scripts/shot.js <url> [--phone] [--cookie <c>] [--out <dir>]` on the existing driver
— a dozen lines against what `launch()` already returns. Then add `--shots <dir>` to
`check-routes.js` so every route it visits leaves a PNG, and a failing run leaves **an image of
the page that failed** rather than only the name of an assertion.

That is the whole change and it is small, but it is the one that changes who can do the checking:
a rendered PNG is readable by an agent, and "does this look right" stops being a question that
only a deploy can answer.

**DONE 2026-08-11.** `screenshot()` in `browser-check-lib.js` (one `Page.captureScreenshot`, full
page via `captureBeyondViewport`), `frontend/scripts/shot.js` on top of it, and `--shots <dir>` in
`check-routes.js`.

- `npm run shot -- <url> [--cookie c] [--phone] [--viewport WxH] [--full] [--click <text>]
  [--out p] [--wait ms]`. `--click` was not in the plan and earns its place: the New Gathering
  dialog is a page state no URL reaches, and it is where K3's crash lived.
- `--shots` writes a full-page PNG per navigation, numbered in visit order and slugged from the
  assertion label — `07-event.png`, `16-band-settle-up.png`, `18-new-gathering-dialog.png`. Full
  page rather than the fold on purpose: L2's off-screen "+" was 1,300px down. A screenshot failure
  is caught and logged, never turned into a red run — this is diagnosis, not an assertion.
- CI runs `--shots shots` on both legs and uploads `browser-check-shots-{prod,dev}`, 7-day
  retention. `frontend/shots/` and `/shots` are gitignored (pictures of seeded or real members).
- Verified: 20 PNGs from a full local run, and the event page and the open dialog were read back
  and are legible — icons painted, band tabs and below-the-fold content in frame.

**It found something on its first use, and it is not fixed here:** the New Gathering dialog shows
**"Please fix form errors before continuing"** in red the moment it opens, before anything has been
typed, with the submit button disabled. `check:routes` asserts "the dialog opens" and "the form
renders" and passes — the picture is the only thing that saw it. Wants its own item: the L1/K5
family (`VForm.validate()`, `validate-on`, `:disabled="!formValid"` on a pristine form) is the
first place to look, and CLAUDE.md already warns that a pristine field reports `isValid === null`.

### M2 — twelve assertions that cannot fail: framework warnings are stripped from what we test · **P1 · S** — DONE 2026-08-11

`compose.dev.yaml` builds the frontend image through `frontend/Dockerfile:31` — `npm run build`, a
**production** build — and Vue and Vuetify compile their warnings out of those entirely. Every
`— no framework warnings` line in a `scripts/browser-check.sh` run (twelve of them, one per
navigation) therefore reports PASS whatever the app does. `browser-check-lib.js` says so in its
own docstring, and `DEVELOPMENT.md` repeats it; both then say "point it at `npm run serve`
instead", and nothing in the repo does that or makes it easy.

This is worth fixing *now* specifically because of what just landed. On the far side of a Vue 3 /
Vuetify 3 migration, a removed API usually **warns** rather than throwing — that is the channel
K5, L1, L3 and L7 were all found through, by hand — and it is the one channel the automated check
is deaf to.

Two ways, and the first is better because it also gives hot reload:

- **(a)** `--dev` on `browser-check.sh`: boot the stack for the API and Mongo, run `npm run serve`
  on :8080, point the check there.
- **(b)** a build arg on the frontend image selecting `vue-cli-service build --mode development`.

(a) needs one small unblock first: `frontend/src/constants.js:3` hardcodes
`http://localhost:3002/api` whenever `NODE_ENV === "development"`, so a dev server cannot be aimed
at the check stack on :3010. Make it `process.env.VUE_APP_API_URL || <current expression>`. The
rest of that path is already in place and was verified: `fetch_utils.js:66` sends
`credentials: "include"`, `main.go:131` sets `AllowCredentials: true`, and `compose.dev.yaml`
already whitelists `http://localhost:8080`. A cookie minted for `localhost` covers both ports —
ports are not part of a cookie's origin — so the same `mintsession` cookie works across the split.

**DONE 2026-08-11, option (a).** `scripts/browser-check.sh --dev`. `src/constants.js` now reads
`process.env.VUE_APP_API_URL || <the old expression>`; `CORS_ORIGINS` became a variable in
`compose.dev.yaml`; the script exports it, brings up **only** Mongo and the API (`--no-deps`, so
the frontend image is not built at all), and serves the app from `npm run serve` on `:8080`
(`SERVE_PORT` to move it) pointed at `:3010`. The dev server is launched under `set -m` so the
teardown can kill the whole process group — killing `$!` alone leaves webpack holding the port,
and the *next* run then fails on a port nothing visible owns.

**The premise was measured, not assumed.** With a deliberate `<v-deliberately-not-a-component />`
in `Landing.vue`:

| build | framework warnings seen | console errors | check says |
|---|---|---|---|
| production (`npm run build`, the frontend image) | **0** | 0 | ALL PASS |
| dev (`npm run serve`, `--dev`) | **1** `[Vue warn]: Failed to resolve component` | 0 | FAIL |

So the twelve lines were reporting on nothing, exactly as the item said, and now they report on
something. Both modes are green on the real app: `--dev` **ALL PASS in 1m59s**, the production mode
**ALL PASS in 3m07s** — the new mode is the *faster* one, because it builds no frontend image.

**CI runs both**, as a two-leg matrix (`prod`, `dev`) in `browser-ci.yml` with `fail-fast: false`.
Manual-only would have left the twelve assertions still unable to fail where it counts. The path
filter widened from `scripts/browser-check.sh` to `scripts/**`.

**One assertion had to be fixed to make `--dev` usable, and it was wrong rather than
mode-specific.** `iconFontLoaded` (L8) required a `resource` timing entry over 100KB, which infers
"the font painted" from network shape — and that inference breaks two ways that have nothing to do
with the font: a webfont is only fetched when a glyph on the page needs it, and a revalidated
cache hit (a 304, which is what a dev server sends; the Go server sends `immutable` and is never
asked) reports a zero-length body. It now calls `document.fonts.load()`, which *attempts* the face
and reports the outcome, and `iconFontSelfHosted` now reads the `@font-face` rule's own `src` out
of the CSSOM instead of resource timing. Both were re-validated in the repo's usual way — blocking
`*materialdesignicons*` makes the first FAIL, injecting a jsdelivr-hosted face makes the second
FAIL — and both now pass on the first navigation, on a repeat navigation, and on a page with no
icons on it, in both modes. `evaluate()` in `browser-check-lib.js` now always passes
`awaitPromise: true`; without it an async assertion returns `"[object Promise]"` and silently
fails its `=== true`.

### M3 — nothing can sign in locally, so the real auth path is verifiable only in production · **P2 · S** — DONE 2026-08-11

`server/routes/auth.go:474–486`: when `utils.SendEmail` fails, `sendOtp` **deletes the code it just
stored** and returns 500. Locally `SendEmail` always fails — no `GMAIL_APP_PASSWORD`, and
`utils/mail_utils.go:27` dials `smtp.gmail.com` unconditionally — so the code is gone before
anyone could read it out of Mongo. Local sign-in is not merely inconvenient, it is impossible, and
`otpCodes` on the dev stack is empty for that reason rather than for want of trying.

Everything consequently runs on `tools/mintsession`, which starts *after* auth: `/auth/otp/check-email`,
`sendOtp`, `verifyOtp`, `SignIn.vue` in both its modes, and the post-sign-in redirect have **never
been exercised anywhere but production**. That is an odd hole to have next to `check-signed-out`,
which exists because an auth-shaped change once made the site unreachable for exactly the people
who could not already get in.

Fix: when the SMTP credentials are absent **and** `gin.Mode() != gin.ReleaseMode`, keep the code
and log it (`logger.StdErr.Println("DEV: otp for", email, code)`) instead of mailing it. Gate on
both conditions, so no production configuration can reach the branch even with a broken mailbox.
Then a local browser signs in for real, and a check that walks the OTP flow becomes writable.

**DONE 2026-08-11 — and the proposed gate did not work, for a reason worth keeping.**

The branch went in as specified (`server/routes/auth.go`, with `smtpConfigured()` reading exactly
the two variables `SendEmail` reads). It did nothing: the first local sign-in attempt still 500'd
with `gomail: invalid address ""`. **`server/Dockerfile` ran `CMD ["./server", "-release=true"]`**,
which calls `gin.SetMode` and therefore beat `GIN_MODE: debug` in `compose.dev.yaml` — so the dev
stack had been claiming to be production, and `gin.Mode()` was a signal that had been silently
wrong for as long as anyone has been running it. Nothing depended on it before this, except the
session cookie's `Secure` flag, which browsers exempt `localhost` from. So it never showed.

That CMD is a leftover: it dates from when the Docker image *was* production, and the Docker
production stack was deleted on 2026-08-05. `compose.dev.yaml` is now its only consumer — checked,
it is the single reference in the repo — while production is systemd running the bare binary with
`-release=true` in `deploy/thegathering.service`, untouched by this. The flag is gone from the
Dockerfile.

Because the failure mode here is *a wrong answer from a signal nobody was watching*, `main.go` now
logs at boot when the branch is live:
`DEV MODE: no SMTP credentials — sign-in codes will be LOGGED, not emailed`. A server about to log
credentials should say so on line one rather than leave it to be inferred.

**Verified end to end in a browser, not just at the API.** `POST /auth/otp/send` → 200, the code
logged and *kept* in `otpCodes` (1 document, where it used to be 0) → `/auth/otp/verify` → a real
session cookie that authenticates `GET /api/user/profile` as the superAdmin. Then the same flow
driven through the UI on the CDP harness: `/sign-in` → type the email → Continue → the "Enter
verification code" screen → read the code from the server log → Verify → lands on `/home` with the
dashboard rendered, **no console errors anywhere in the flow**. That is `SignIn.vue` in its
sign-in mode, `sendOtp`, `verifyOtp` and the post-sign-in redirect all exercised outside production
for the first time.

`server/routes/auth_dev_otp_test.go` asserts the gate in both directions across five
configurations, the load-bearing row being **release mode with a broken mailbox must not log
codes**. Follow-up worth having, deliberately not done here: a `check:signin` alongside
`check:signed-out` that walks this flow on every run. It is now writable — the walk above was a
throwaway script — but M3 asked for the capability, not the check.

### M4 — the fixture is trapped inside the script that throws it away · **P2 · S** — DONE 2026-08-11

The five-member club, ten Chronicle entries, three gatherings and the one cast availability live
as a `mongosh` heredoc inside `scripts/browser-check.sh`, in a stack that is deleted at the end of
the run. Nothing else can use it.

Meanwhile the dev stack on :3002 holds, measured today: **0 users, 0 events, 0 allowlist rows**
(94 chronicle documents and 30 folders survive from a partial dump). There is nothing to click
through even after a rebuild, and no way to sign in if there were (M3). The consequence is that
"bring up the app and look at it" is, in practice, a five-minute manual setup nobody does — which
is why the checks are the only thing that ever looks.

Extract the seed to `scripts/seed-club.js`, taking a Mongo URI and a base URL, and call it from
both `browser-check.sh` and a new `scripts/dev-up.sh --seed`. One fixture with two consumers, and
the interactive stack stops being able to drift away from the one CI asserts against.

**DONE 2026-08-11**, as two files rather than one, because the fixture has two halves with
different needs:

- **`scripts/seed-club.js`** — the Mongo documents (five members, their allowlist rows, ten
  Chronicle entries), run through mongosh, printing two user ids.
- **`scripts/seed-club.sh`** — the orchestration: runs that JS, mints the cookies, then creates
  the three gatherings and casts the one availability **over the API**, so the fixture cannot
  drift from what the app actually writes. Prints one line, `<userId> <responderId> <eventId>`,
  with everything else on stderr so a caller can `read` it.

The split is forced by where mongosh lives. A pure `.js` taking a Mongo URI would need a host
mongosh, and **the CI runner has none** — only the container does. So the mongosh invocation is a
caller-supplied command (`seed-club.sh <baseUrl> [mongosh argv...]`), defaulting to a host mongosh
against `$SEED_MONGO_URI`; `browser-check.sh` and `dev-up.sh` both pass a `docker compose exec`
form. The two knobs reach the JS as prepended globals *or* env vars, because `docker compose exec`
does not forward the environment and this script does not get to add `-e` to a command line it was
handed.

**`scripts/dev-up.sh`** is the second consumer: build, start, wait for health, optionally seed, and
then print how to sign in — which is only useful because M3 landed in the same pass. It also
carries `--no-build`, `--force` and `--down`.

**Re-seeding is the part that needed care.** Seeding twice collides on the allowlist's unique email
index and reports as a duplicate-key error rather than "you already have a club in here", so the JS
detects an existing fixture and refuses, naming the fix. `SEED_FORCE=1` replaces it, deleting only
what it can prove it created: the five `@example.test` users, everything owned by those ids, and
chronicle entries carrying its own marker field. That scoping is load-bearing — the dev Mongo
volume on these machines habitually holds a restored production dump, and `dev-up.sh` never runs
`down -v`. Verified: after a forced re-seed the fixture is still exactly 5 users / 5 allowlist rows
/ 10 chronicle entries, and **the 94 pre-existing chronicle documents from the partial dump were
untouched**.

One bug found by testing the guard rather than the happy path: the refusal exited 1 **silently**,
because `set -e` kills the script at a failing command substitution and threw away the explanation
mongosh had just printed. A diagnosis-free exit 1 is exactly what this script exists to prevent, so
`set -e` is now lifted across that one assignment and stderr is captured separately, so a
connection warning can never land on the line the ids are read from.

`scripts/browser-check.sh` lost ~90 lines to the extraction and passes unchanged: ALL PASS in both
modes.

### M5 — a stale checkout is silent, and manufactures fake regressions · **P1 · S** — DONE 2026-08-11

Both halves of this machine were stale on arrival and neither said so. This is not a hypothetical;
it is what the first twenty minutes here were spent on.

- **`frontend/node_modules` held the entire Vue 2 stack** — vue 2.7.16, vuetify 2.7.2, vuex 3.6.2,
  vue-router 3.6.5 — against a `package.json` asking for 3.x of all four. `npm run test:unit`
  failed 2 of 23 files with `(0 , createStore) is not a function`, pointing at
  `src/store/index.js:12`. That reads precisely like a Vuex-migration regression on `main`. It was
  a stale install; CI was green on the same commit, and `npm ci` fixed it (23/23, 395 tests).
- **The `timeful-dev` stack's images were built 2026-08-10 14:47** — before the first commit of
  Part K. It had been up for 18 hours serving a Vue 2 bundle out of a Vue 3 checkout.

The existing workflow rule covers the second case ("rebuild the dev containers before trusting a
harness run") and not the first, and in both cases the failure is silent: the wrong answer arrives
looking exactly like a real one.

Add `scripts/dev-doctor.sh`: compare each installed dependency's major against `package.json`;
compare the running images' creation time against the newest mtime under `frontend/src` and
`server/`; print the Go, Node, Chrome and mongosh versions it found. Exit non-zero with the exact
command that fixes each thing it complains about. Call it from the top of `browser-check.sh`, so a
run cannot silently report on artifacts that predate the change under test.

**DONE 2026-08-11.** `scripts/dev-doctor.sh`, as specified, plus `--deps` (the install check
alone) and `PROJECT=` to point it at another compose project.

- Per-image sources rather than one tree-wide mtime: frontend sources make the frontend image
  stale and `server/` makes the server image stale, never both. A check that cries wolf is a check
  people learn to skip.
- It reaches the image through the **container** (`ps -aq` → `docker inspect`), not through
  `docker compose images` — that subcommand fails outright with "No such image" when *any one* of
  the project's images has been pruned from under a running container, taking every other service's
  answer down with it. That is not hypothetical: it is the state this box was in, and the first
  version of the script reported "frontend: not running" because of it.
- `browser-check.sh` calls `--deps` before it builds anything. Only the deps half, because that
  script rebuilds its own images every run and so cannot be caught by the image check — but it
  *does* run `npm run serve` out of `node_modules` in `--dev` mode.

**Verified in both directions.** With `vue` and `vuex` majors bumped in `package.json` and a
nonexistent package added, it named all three and exited 1; unmodified, it exits 0. And on its
first real run it caught this machine: the `timeful-dev` server image was built
**2026-08-10T14:47** — before the first commit of Part K — with `server/routes/events.go` and four
other files newer than it, and the frontend container was running an image that no longer exists
at all. That is the 18-hours-of-Vue-2-out-of-a-Vue-3-checkout state, reported in one line instead
of being discovered by debugging a fake regression.

### M6 — no fast render tier: a component regression costs 3m19s · **P2 · M** — DONE 2026-08-11

Measured on this box: `scripts/browser-check.sh` is **3m19s** end to end (two image builds, boot,
seed, ~14 navigations at a flat 6s each), and it is the **only** thing in the repo that renders a
component. The unit suite runs 395 tests in **1.5s** and cannot render anything at all —
`vitest.config.mjs` sets `environment: "node"`, no test imports a `.vue` file, and
`@vue/test-utils` is not a dependency.

Those are the only two tiers, and the gap between them is where this repo's bugs actually live.
K3's dialog that threw on every open, L1's validation guard that never fired, L2's off-screen
button, K5's toggles that would not turn off — every one of them is a *component* fault that a
mounted component would have caught in milliseconds, and every one of them instead needed either a
full stack or a deploy to find.

The pure-node choice was a deliberate Vue 2 decision, and Vue 3 removes its premise: `@vue/test-utils`
v2 plus `happy-dom` are small, stable and need no browser download.

Scope it tightly, because the failure mode here is a 23-file backfill nobody asked for: add the two
dependencies, one vitest project on `environment: "happy-dom"`, and **three** mount tests as the
proof — the New Gathering dialog opening, the event band tabs switching, one form's submit guard.
It does not replace `check:routes`: it cannot see layout, real CSS, the icon webfont or a 390px
viewport. It is the missing middle, not a substitute for either end.

**DONE 2026-08-11**, at the scope asked for: two dependencies, one vitest project, three specs.
**411 tests now, up from 395; the whole suite is 7.3s.** The tiers are split by *filename* —
`*.test.js` is `node`, `*.spec.js` is `dom` — so the two globs cannot overlap and a file's tier is
readable from its name.

**The console guard is the part that would actually have caught K3, and it is not an assertion any
spec makes.** `src/test/setup.dom.js` fails every test in the tier on an unasserted
`console.error` or `[Vue warn]`, because a component that throws from a lifecycle hook does not
fail a test on its own: Vue catches it, reports it to the console and returns a wrapper that looks
fine. `expectConsole(/…/)` opts one line out, mirroring `expectConsoleErrors` on a route.
Verified in both directions — a bad prop type and a hook that throws each fail a test that asserts
nothing else.

**Verified against the bug it is named after.** With L1's guard put back
(`if (!this.$refs.form.validate()) return`, a Promise, always truthy), 4 of the 6 GuestDialog specs
fail. That is the assertion this repo could not make before.

Three things the item did not anticipate, all found by running it rather than reading:

- **`fetch` had to be faked for the whole tier, not per spec.** `NewEvent` mounts `EmailInput`,
  which warms a contacts cache in `mounted()`, and happy-dom will open a real socket to
  localhost:3000 to do it — a suite whose result depends on what is listening on the machine.
  `src/test/api.js` fakes it at `fetch` rather than at `@/utils`, so `fetch_utils.js` (the layer
  every call site's error handling is written against) stays real code under test.
- **Four browser APIs Vuetify calls unconditionally are absent from happy-dom** — `ResizeObserver`,
  `visualViewport`, `matchMedia`, `Element.prototype.animate`. The `visualViewport` one is read
  from inside a watcher, so it surfaced as an unhandled rejection rather than at the mount.
- **`@vitejs/plugin-vue` is needed to compile `.vue`,** and vite does not resolve extensionless
  `.vue` imports the way Vue CLI's webpack config does. The app has ~20 of them, and the first one
  in a mounted tree failed the whole spec file at collection, pointing at a file that exists.
  `resolve.extensions` now matches the app. **None of this moves the app toward Vite** — the plugin
  is the SFC compiler wired into the transform vitest uses, and never runs over the shipped bundle.

One thing the tier found on its own: `Event.vue`'s `scheduleOverlapComponent?.states.SET_SPECIFIC_TIMES`
(and `?.respondents.length`) guard the ref being absent but not the property being absent on it.
Harmless in the app, since the real child always has both — so the spec's stub carries that
contract rather than the assertion being weakened, and the coupling is now written down somewhere.

### M7 — `browser-check.sh` has exactly one mode: everything, from scratch · **P3 · S** — DONE 2026-08-11

`KEEP_STACK=1` leaves a stack up and there is no way to then *use* it — no `REUSE=1`, so
reproducing one failing assertion means another full rebuild and reseed. There is no way to run a
single route while fixing it. And `visit()` sleeps a flat 6 seconds per navigation: roughly 90 of
the 199 seconds are spent waiting on pages that settled in one.

`REUSE=1`, `ONLY=event`, and a poll on the route's own first assertion with the 6s as a ceiling
rather than a floor. None of it changes what is checked — it changes whether the check is
reachable in the middle of fixing something, which is the only time it is worth the most.

**DONE 2026-08-11**, all three, measured on this box: a full run **1m58s** (was 3m19s), `REUSE=1`
**31s**, `REUSE=1 ONLY=event` **11s**. `--dev` is **57s**. ALL PASS in both modes, 64 assertions,
unchanged.

- **The poll's grace period is deliberate and is not padding.** Returning the instant the route's
  first assertion goes true would quietly shrink the window the two console assertions watch — a
  panel that fetches on mount can warn a beat after the element the readiness check looks for
  exists. That is a check getting weaker while appearing to get faster, which is the exact failure
  M2 was about. 900ms after ready; the full 6s whenever the page needs it.
- **The readiness signal is the route's own first assertion**, so no route maintains a second
  description of "has it rendered", and the wait can never pass on a page the run is about to fail.
- **`REUSE` records the fixture ids in `${TMPDIR}/browser-check-<project>.state`, never the session
  cookie** — that is a credential, and this file exists to be read by a later process. Re-minting
  costs one `go run`.
- **The mode guard is the load-bearing part of `REUSE`.** `CORS_ORIGINS` is read by the server at
  boot, so reusing a non-`--dev` stack under `--dev` means every API call from :8080 is rejected
  and the app renders as a completely empty club — no error, no warning, every assertion failing
  for a reason that has nothing to do with the code. It refuses in one line instead.
- **A partial run says so on its own verdict line**, not only in a header: `ALL PASS` is the string
  that gets pasted into a commit message, so one that checked a single route has to be unable to
  pass for the real thing. A pattern matching nothing exits **2** and lists the sections, rather
  than printing a green ALL PASS over zero assertions.
- One thing the item did not anticipate: the failure path drowned its own diagnosis. Every non-zero
  exit dumped 60 lines of stack log, which is right when the stack died and useless when the script
  has just explained in one sentence that you asked to reuse a stack booted in the other mode. A
  `fail()` helper marks those as self-explained, and exit 2 from the check is treated the same way.

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
  filenames. **`scripts/dev-doctor.sh` now says so out loud** (M5) — that and a stale
  `node_modules` are the two ways this box lies to you, and both are silent otherwise.
- **Look at the page — and since M1 that is an instruction anyone here can follow.**
  `scripts/browser-check.sh --shots <dir>` leaves a full-page PNG of every page it visits;
  `npm run shot -- <url> [--cookie] [--phone] [--click]` shoots one. Lint, unit tests and the
  build have all been green over a bug that only appears in a browser — `v-show` beaten by
  Tailwind's `important: true`, a purged class name built from a template string, a fifth tab
  putting a phone into horizontal scroll. The `CLAUDE.md` frontend section lists the ones already
  paid for. **Run `--dev` when the change is framework-shaped** (M2): a production build has no
  warnings in it to find, so the twelve warning assertions only mean something there.
- **Reach for the right tier** (M6/M7). A component fault — a dialog that throws on open, a guard
  that never fires, a toggle that will not turn off — belongs in a `*.spec.js` mount test, which
  costs a second. A *rendered-page* fault — layout, real CSS, the icon webfont, 390px — can only
  be seen by `check:routes`, and no mount test will ever catch one. When fixing something the
  browser check found, run it once with `KEEP_STACK=1` and then `REUSE=1 ONLY=<section>` for each
  attempt; a partial run stamps `(PARTIAL: …)` on its verdict line, so don't quote one as a full
  run.
