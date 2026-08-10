# Timeful / The Fellowship — Backlog 3 (active)

> Opened **2026-08-10**, at `be30197d`. `TODO.md` and `TODO2.md` are both **closed archives** as
> of this file — nothing in either needs updating again, and the three items still open in
> TODO2.md Part G are restated below as pointers rather than copied.
>
> **Status: Part J in progress (2026-08-10).** A fresh full-codebase review pass — improvements and
> optimizations only. **J1–J4 shipped**; J5–J10 still queued, each waiting for a go-ahead.
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

## Inherited, still open — all `P3`, all parked by the user

Carried across as pointers only; the detail stays in `TODO2.md` and is still accurate. None of
these has a known failure mode in production. If one is picked up, give it a fresh `J*` ID here
and leave the old entry alone.

- **`TODO2.md` G2 — split `date_utils.js`** (`L`). The only part of G2 left; everything else
  (the eleven dead exports, `newEventFormMixin`, the `ScheduleOverlap` computed block) shipped
  2026-07-29/30. 946 lines / 32 exports. The entry's caveat stands: verify a split with the app
  running, not blind — the earlier passes found three live bugs precisely because they were
  exercised.
- **`TODO2.md` G3 — web push** (`M`). Deferred pending a value reassessment; reintroducing a
  service worker reverses a deliberate removal (`f857320`) and email reminders already cover iOS.
  One cheap loose end inside it, independent of the rest: `kill-sw.js` sits at the repo root, so
  it is **never actually served** and can't unregister anything — move it to `frontend/public/` or
  mark it documentation-only. `frontend/.eslintrc.cjs:11`'s `serviceworker: true` env is stale too.
- **`TODO2.md` G4 — rename the `schej-it` Mongo database** (`L`). A data migration (dump →
  restore under the new name → cutover in a deploy window), human-run. Zero user-facing benefit,
  which is why it's parked.

---

## PART J — 2026-08-10 review findings (improvements & optimizations)

From a full review pass at `38b613f7`. The codebase has been through two prior waves (A–E, H), so
these are what's left: mostly performance and robustness, no known user-facing breakage except J2.
J1–J4 are done; everything from J5 down waits for a go-ahead.

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

### J5 — a non-`error` panic in a calendar provider kills the whole server · **P2 · S**

`services/calendar/calendar.go:21,41` — both async helpers recover with `c <- …{Error:
err.(error)}`. If any provider code panics with a non-error value (`panic("boom")`, an int — and
these are goroutines wrapping three external APIs plus a CalDAV library), the type assertion
itself panics inside the deferred function, the recovery is lost, and the unrecovered goroutine
panic takes down the process. Replace with `fmt.Errorf("%v", r)` (and recover into a plain
`interface{}` first). Two-line fix, removes a latent crash-on-weird-input in the one subsystem
that talks to the most third-party code.

### J6 — the dashboard ships every event's full embedded document · **P2 · S/M**

`routes/user.go:441` (`getEvents`) — the query returns whole event documents: embedded `rsvps`,
`polls`, `lists` (items and all), `dates`, `remindees`, for every event the member ever touched,
with no projection and no pagination. The dashboard renders a name, a date range and a folder.
Payload grows with club history and every F-track feature made each document fatter; it also
leaks data the dashboard has no business holding client-side (other people's RSVP emails are
stripped only by `getEvent`'s per-event logic, not here — worth confirming what `remindees`
serializes on this path while fixing it). Add a `Find` projection down to the fields the
dashboard and folder logic actually use.

### J7 — DB accessors that report an outage as "not found" · **P3 · S**

`db/expenses.go:70` (`GetExpenseById`) — any `Decode` failure returns `nil, nil`, so a Mongo
outage mid-request answers 404 `expense-not-found` instead of 500, and the comment says
`GetCommentById` set the pattern (same shape in `db/comments.go`). A malformed id as "not found"
is right; a connection error is not. Distinguish `mongo.ErrNoDocuments` from everything else and
return real errors — the route layers already have the 500 branch wired for it.

### J8 — disabled calendars are still fetched from the providers · **P3 · S**

`services/calendar/calendar.go:142` — after the calendar list resolves, events are fetched for
*every* sub-calendar, including those the member toggled off (`SubCalendar.Enabled` false) and
accounts toggled off wholesale (`CalendarAccount.Enabled`). Each is a live round-trip to
Google/Microsoft/CalDAV. Before skipping them, check what the frontend expects: if toggling a
calendar on merely re-filters client-side (instant today), skipping the fetch changes that to a
refetch — acceptable, but a product call to make knowingly, not a silent optimization.

### J9 — CLAUDE.md still says date math uses three libraries · **P3 · S**

`CLAUDE.md` (frontend utils bullet) claims `date_utils.js` "uses `dayjs`/`moment`/`spacetime`".
`moment` and `spacetime` are gone — not in `frontend/package.json`, not imported anywhere;
`date_utils.js` is dayjs-only. One-line doc fix; flagged because the 2026-08-10 doc audit
(`3972f71e`) missed it, and a stale claim in CLAUDE.md steers every future session.

### J10 — an expense accepts any date a client sends · **P3 · S**

`routes/expenses.go:380–384` — `payload.Date` is trusted verbatim (unix ms, unbounded). The
ledger sorts by date descending (`db/expenses.go:48`), so a hand-rolled client can stamp an
expense with year 9999 and pin it above every real row forever, or go negative and bury it. The
official client always sends a sane value; clamp server-side anyway (say, within a year around
now) to the same standard the amount cap already sets — "a guard against nonsense, not a
business rule".

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
