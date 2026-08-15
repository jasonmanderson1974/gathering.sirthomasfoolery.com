# Development & Workflow

How this repo is developed and shipped. Read this first when setting up a new
machine. (Deployment mechanics for the server itself are in `DEPLOYMENT.md`.)

## The setup

- **Two dev machines**, both push directly to `main`:
  - **Build box** — has SSH access to the production host, so it deploys.
  - **Other machine(s)** — no access; cannot deploy.
- **Production** is `stf-thegathering` (192.168.24.56), a single host in the
  isolated "Sir Tom" VLAN, behind a Cloudflare tunnel. **No Docker:** `mongod`,
  one static Go binary and `cloudflared`, all under systemd. It does not track
  `main` — it runs whatever release was last pushed to it.
- **CI** runs on GitHub Actions on every push to `main` and on PRs.

## Golden rules

1. **Sync before you change anything.** Because two machines push to `main`,
   your local can be behind. Always start with:
   ```bash
   git fetch origin
   git log --oneline HEAD..origin/main   # anything here? you're behind
   git pull --ff-only                    # fast-forward to latest
   ```
   Building on stale `main` creates divergence and rejected pushes.

2. **`main` is the trunk. Keep it green.** CI runs *after* the push (it's not a
   merge gate), so don't push code you haven't at least built/tested locally.
   Check the CI result before relying on a commit.

3. **Deploys are manual and gate-kept.** `main` being updated does NOT auto-ship
   to prod. A human deploys, from the VM-adjacent box. `origin/main` can be
   ahead of what's live — that's expected.

## Deploying (build box only)

Run the deploy script **here**, from the repo root — not on the server:

```bash
./deploy.sh
```

It refuses a dirty tree or a checkout behind `origin/main`, runs the tests,
builds a static Go binary and the frontend bundle, rsyncs both to
`/opt/thegathering/releases/<sha>` on the server, flips the `current` symlink,
restarts the service, and polls `/api/health`. If health doesn't come good it
re-points the symlink at the previous release and restarts.

The direction is the opposite of the old Docker deploy, which ran on the VM and
pulled git. Production now carries no toolchain at all. Full mechanics, and how
to build a host from scratch, are in `DEPLOYMENT.md`.

## Local development

Neither dev machine holds prod secrets, so use the self-contained dev stack.
**Prefer `scripts/dev-up.sh`** over driving compose by hand — it rebuilds (the
trap that makes a harness run meaningless), waits for health, and can put a club
in the database worth looking at:

```bash
scripts/dev-up.sh --seed          # build, start, seed a populated club
scripts/dev-up.sh --seed --force  # ...replacing a club already in there
scripts/dev-up.sh --no-build      # start what is already built
scripts/dev-up.sh --down          # stop, KEEPING the Mongo volume
open http://localhost:3002
```

It boots mongo + frontend + server with dummy secrets and exposes Mongo on
`:27017`. Out of the box that is enough for build/boot/UI smoke tests and the
backend integration tests, but not for anything touching Google OAuth.

### A club worth looking at, and signing in to it

Two things used to make "bring up the app and look at it" a five-minute setup
nobody did, so the automated checks were the only thing that ever looked.

**There was nothing in the database** (TODO3 M4). The dev stack held 0 users, 0
events and 0 allowlist rows, while the one populated fixture this repo knows how
to build was a heredoc inside `scripts/browser-check.sh` — in a stack deleted at
the end of every run. It now lives in `scripts/seed-club.js` (the Mongo half:
five members, their allowlist rows, ten Chronicle entries) driven by
`scripts/seed-club.sh` (the API half: three gatherings and one cast
availability, created over HTTP so the fixture cannot drift from what the app
actually writes). Both `browser-check.sh` and `dev-up.sh --seed` call it, so the
stack you click through is the one CI asserts against.

```bash
scripts/seed-club.sh http://localhost:3002          # host mongosh
scripts/seed-club.sh http://localhost:3010 \
  docker compose -f compose.dev.yaml -p timeful-check \
  exec -T mongo mongosh --quiet mongodb://localhost:27017/schej-it
```

It prints one line, `<userId> <responderId> <eventId>`. Seeding twice refuses
rather than colliding on the allowlist's unique index; `SEED_FORCE=1` replaces,
and deletes only documents it can prove it created (the five `@example.test`
users, anything owned by them, and chronicle entries carrying its marker). That
scoping is load-bearing: this Mongo volume habitually holds a restored
production dump. **Never `down -v` the dev stack.**

**And nothing could sign in** (TODO3 M3). `utils.SendEmail` dials Gmail
unconditionally and fails with no `GMAIL_APP_PASSWORD`, and `sendOtp`'s failure
path *deleted the code it had just stored* — so the code was gone from Mongo
before anyone could read it, and `otpCodes` on a dev stack was always empty.
Everything therefore ran on `tools/mintsession`, which starts *after* auth,
leaving `/auth/otp/*`, `SignIn.vue` in both modes and the post-sign-in redirect
exercised nowhere but production. Now, with no SMTP configured **and** gin not
in release mode, the code is logged and kept:

```bash
scripts/dev-up.sh --seed
# open http://localhost:3002/sign-in, enter harness@example.test, then:
docker compose -f compose.dev.yaml -p timeful-dev logs server | grep 'DEV: otp'
```

The seeded roll is `harness@` (superAdmin), `ambrose@` (admin), and `cornelius@`
/ `percival@` / `reginald@` (members), all `@example.test`.

> **`gin.Mode()` was silently wrong here, and that is why the server announces
> this branch at boot.** `server/Dockerfile` ran `CMD ["./server", "-release=true"]`
> — a leftover from the Docker production stack deleted in 2026-08, when that
> image was production. Today `compose.dev.yaml` is its only consumer, and the
> flag calls `gin.SetMode` and so beat `GIN_MODE: debug` in the environment:
> the dev stack had been claiming to be production. The flag is gone from the
> Dockerfile (production is systemd + `-release=true` in
> `deploy/thegathering.service`, which is untouched), and `main.go` now logs
> `DEV MODE: ... codes will be LOGGED, not emailed` at startup, so a server
> about to log credentials says so on line one instead of leaving it to be
> inferred.

### Testing the real calendar path locally

The calendar code was the one area nothing could exercise — no OAuth locally —
which is how two Vue 3 regressions reached production before anyone saw them
(K5, K6). It *can* be exercised, and without a browser OAuth round trip:

**A stored refresh token is enough.** `services/auth` sends `client_id`,
`client_secret`, `refresh_token` and `grant_type=refresh_token` — **no
`redirect_uri`** (that appears only on the initial code exchange). So a token
already in a restored production dump can be replayed locally, and **no Google
Cloud Console change is needed**. Only the initial *linking* flow would need
`http://localhost:3002/auth` added to the OAuth client's redirect URIs.

Put the three values in `server/.env.dev.local` (untracked, chmod 600 — it
already holds `ENCRYPTION_KEY` for decrypting a restored dump), then **source it
before bringing the stack up**:

```bash
set -a; . server/.env.dev.local; set +a
docker compose -f compose.dev.yaml -f compose.dev.secrets.yaml up -d
```

`ENCRYPTION_KEY` must match the production key or the stored token will not
decrypt; `CLIENT_ID` and `CLIENT_SECRET` are what exchange it.

> **Sourcing it is not optional, and forgetting is silent-ish.** Compose gives
> `environment:` precedence over `env_file:`, so for a long time the secrets
> overlay could not override the dummy `ENCRYPTION_KEY` hardcoded in
> `compose.dev.yaml` — it just ran on the dummy. The symptom is
> `GET /api/user/profile` returning **500** with
> `decrypt: cipher: message authentication failed` in the server log, and a
> completely blank-looking app. `compose.dev.yaml` now writes these as
> `${VAR:-dummy}` so a sourced value actually wins.

Two things to know once it works. The **CalendarAccounts panel only renders
while editing availability** — it is not on the read-only grid, so "I see no
calendars" there is expected. And calendar events only draw when the
"Show my calendar events" switch is on (or while editing).

**Caveat:** email OTP still needs Gmail SMTP credentials in the same file; see
the browser-checks section for minting a session directly instead.

### Event location address lookup (optional)

The event location field is plain free text by default. A Maps key adds Google
address suggestions on top; with no key the inputs behave exactly as they did
before, so this is never required to build or run anything.

Where the key goes depends on how you're running the frontend:

| Running via                        | Put the key in                                    |
| ---------------------------------- | ------------------------------------------------- |
| `npm run serve` / `npm run build`  | `frontend/.env` as `VUE_APP_GOOGLE_MAPS_API_KEY`  |
| `compose.dev.yaml` (local Docker)  | root `.env` as `GOOGLE_MAPS_API_KEY`              |
| production (`./deploy.sh`)         | root `.env` as `GOOGLE_MAPS_API_KEY`, **on the build box** |

**`frontend/.env` does not work inside Docker.** The Dockerfile always sets
`ENV VUE_APP_GOOGLE_MAPS_API_KEY` (empty when no build arg is passed), and
dotenv never overwrites an already-set variable — so a key in `frontend/.env`
is silently ignored in any image build. Both compose files pass it as a build
arg from the root `.env` instead. It is baked in at build time either way, so
changing it needs a frontend rebuild.

Local dev also needs `http://localhost:8080/*` and `http://localhost:3002/*`
on the key's HTTP-referrer allowlist, or Google rejects the request with
`RefererNotAllowedMapError`. Use a separate unrestricted dev key if you'd
rather not widen the production one.

The key must be a Maps Platform **browser** key with **Places API (New)** and
**Maps JavaScript API** enabled, restricted by HTTP referrer and to those two
APIs. Note that `google.maps.places.Autocomplete` (the old widget) has not been
available to new Maps customers since 2025-03-01 — `utils/maps_utils.js` uses
the current `AutocompleteSuggestion.fetchAutocompleteSuggestions` data API and
renders predictions in our own themed input, so free-form venues like "Greg's
back garden" stay first-class. Suggestions are billed per session, so the
input holds one session token per pick.

## Testing

Run before pushing.

**Frontend** (`cd frontend`):
```bash
npm run test:unit          # both tiers
npm run test:unit:node     # just the pure-JS tier
npm run test:unit:dom      # just the mounted-component tier
```

**There are two tiers, split by filename** (TODO3 M6), and the split is worth
understanding before adding a test:

| | `node` | `dom` |
|---|---|---|
| files | `src/**/*.test.js` | `src/**/*.spec.js` |
| environment | `node` | `happy-dom` |
| what it sees | pure JS, extracted *out of* components | a mounted component: template, props, events, watchers, slots |
| what it cannot see | anything rendered | real CSS, layout, the icon webfont, a 390px viewport |

The `node` tier is the original suite and is unchanged: ~395 tests in 1.5s,
importing no `.vue` file. That was a deliberate Vue 2 decision and it stays.

The `dom` tier is the missing middle between it and `scripts/browser-check.sh`,
which was the only other thing in the repo that rendered a component and cost
two minutes to say so. Every browser-only bug this repo has paid for that was
*not* a layout bug lived in that gap — K3's dialog that threw on every open,
L1's validation guard that never fired, K5's toggles that could be switched on
but not off. Mount tests find those in milliseconds.

Three things to know before writing one:

- **`src/test/setup.dom.js` fails every test on an unasserted `console.error` or
  `[Vue warn]`.** That guard, not the assertions, is what would have caught K3:
  Vue catches a throw from a lifecycle hook, reports it to the console and
  carries on, so the mount succeeds and the page looks fine. Opt a single
  expected line out with `expectConsole(/…/)` — never a whole test.
- **`fetch` is faked for the tier** (`src/test/api.js`). An unmocked call gets
  `{}` and a 200 rather than a socket; `mockApi(route, body)` answers one, and
  `apiCalls()` / `calledApi(route)` assert that a call happened. It is faked at
  `fetch` rather than at `@/utils` so `fetch_utils.js` — the layer every call
  site's error handling is written against — is real code under test.
- **Mount through `src/test/mount.js`** (`mountApp`), which assembles Vuetify, a
  store with the app's real role getters, and a memory router carrying the app's
  real route names. It attaches to a live element because Vuetify teleports
  every overlay to `document.body`; use `openDialogs()` to find one.

`@vitejs/plugin-vue` is a devDependency purely so this tier can compile `.vue`.
**It is not a step toward Vite** — see the repo-layout note in `CLAUDE.md` for
why the app stays on Vue CLI / webpack.

**Backend** (`cd server`) — needs a Go toolchain, plus a reachable Mongo if you
want the integration tests to actually run (`compose.dev.yaml` provides one on
`localhost:27017`):
```bash
MONGODB_URI=mongodb://localhost:27017 go test $(go list ./... | grep -v '/scripts')
```
> **`/scripts` is the only exclusion, and it is derived — never spell the
> package list out.** Plain `go test ./...` fails on the one-off migrations
> under `server/scripts/`, which reference model shapes from years ago and are
> *deliberately* not kept compiling (see `server/scripts/README.md`). Everything
> else is in, automatically. `go vet` and `golangci-lint` use the same
> `go list | grep -v '/scripts'` shape, and so does `backend-ci.yml` — there is
> nothing left to keep in sync. A hand-written list drifted three times in two
> days (TODO B4, B8, E10) before this replaced it.
>
> Dropping `MONGODB_URI` is fine — the DB-backed tests call `requireDB` and skip
> — but then you are running roughly half the backend suite, so don't read a
> green run as a green build.

> **Careful running `./db/` against a restored prod dump.** The B7 sweep test
> calls `db.EncryptPlaintextOAuthTokens()`, which walks *every* user in whatever
> database `MONGODB_URI` points at — so any real OAuth token still stored in the
> clear gets encrypted with the **test** key, and the local server (which boots
> with the real `ENCRYPTION_KEY` via `compose.dev.secrets.yaml`) can then no
> longer read it. Harmless on the throwaway `compose.dev.yaml` Mongo; on a
> restored dump, re-restore it, or decrypt with the test key and write the
> plaintext back so the next boot re-encrypts it correctly.

**Lint (backend) — BLOCKING since 2026-07-28 (TODO B5).** The backlog is at
zero, so anything the linter reports is new and will fail the build. Run it
before pushing:

```bash
cd server
curl -sL https://github.com/golangci/golangci-lint/releases/download/v2.12.2/golangci-lint-2.12.2-linux-amd64.tar.gz | tar xz -C /tmp
/tmp/golangci-lint-2.12.2-linux-amd64/golangci-lint run $(go list -f '{{.Dir}}' ./... | grep -v '/scripts')
```

> **Keep the `-f '{{.Dir}}'`.** golangci-lint takes *directories*; plain
> `go list` emits *import paths*. Passing those makes it resolve everything
> against the wrong root, print a few "typechecking error: directory not found"
> lines, and then report `0 issues` — which is indistinguishable from a clean
> run. CI silently linted nothing this way until 2026-07-28.

If something is genuinely not worth fixing, suppress it narrowly **with a
reason** — `//nolint:<linter> // why`, on the one line — rather than restoring
`continue-on-error`. There are currently **no** `nolint` directives in the tree
(the last one, on AES-CFB in `utils/utils.go`, went away with B6), so keep that
the exception it should be. errcheck is already relaxed for `_test.go`
teardown; see `server/.golangci.yml`.

**Cross-package test isolation.** `go test` runs packages in parallel against
one Mongo, and `services/reminders` sweeps *every* eligible event in the
database. Fixtures in other packages must not look nudgeable (set
`NudgeStage: 3`), and assertions about "how many were sent" must filter to
their own recipients. This was a real CI failure that reproduces roughly one
run in three — if you touch either package, run them together a dozen times,
not once.

If you have no local Go toolchain, run the tests in a container. **Keep the tag
in step with `.go-version`** — that file is the toolchain production is built
with, and all three Go workflows read it (P1/P2); a container on another patch
is testing a standard library we do not ship.
```bash
docker run --rm -e MONGODB_URI=mongodb://host.docker.internal:27017 \
  -v "$PWD/server:/src" -w /src golang:1.26.6-alpine \
  sh -c "go build . && go test \$(go list ./... | grep -v '/scripts')"
```
> On Linux/WSL, `host.docker.internal` may not resolve — use
> `--network host` with `MONGODB_URI=mongodb://localhost:27017` instead.

**Race detector.** Worth running when you touch anything concurrent (the rate
limiter's janitor, the token-refresh goroutines, the reminder scheduler). It
needs cgo and a C compiler, which the dev boxes don't have, so use the Debian
image rather than Alpine:
```bash
docker run --rm --network host -e MONGODB_URI=mongodb://localhost:27017 \
  -e CGO_ENABLED=1 -v "$PWD/server:/src" -w /src golang:1.26.6 \
  sh -c "go test -race \$(go list ./... | grep -v '/scripts')"
```

**What's covered today** (as of 2026-08-10: **432** Go test functions, **391**
frontend unit tests): role/permission logic and money splitting (`models`); the
handler layer broadly (`routes`, 288 functions) — admin permission guards, the
event auth gate, responses, comments/mentions, polls, event lists, My Lists / My
Notes, expenses + receipts, avatars, display names, chronicle, health, and the
PII-leak and scoped-write regressions; the rate limiter and email/phone helpers
(`utils`); allowlist CRUD, encryption migration and scoped writes (`db`,
integration); calendar/contacts/Graph and the reminder scheduler (`services`).
**Not yet covered:** `middleware` has no test files at all, and the
email-change/OTP flows are only covered where a handler test happens to reach
them. `go test` passing still means "compiles + these pass", not full
correctness — and note that roughly half the backend suite skips without
`MONGODB_URI`.

### Browser checks (before deploying auth/UI changes)

Three headless-Chrome checks live in `frontend/scripts/`. They need a built
frontend, a running server and Mongo, so each one takes a base URL and (for two
of them) a session cookie. All three exit non-zero on failure.

**The routes check now runs in CI** — `.github/workflows/browser-ci.yml`, on
every push and PR touching `frontend/**` or `server/**` (both halves: the
frontend renders what the API returns), in **two legs**, `prod` and `dev` (see
"Framework warnings" below for why the second one exists). It gets its stack
from `scripts/browser-check.sh`, which is also how you run it here:

```bash
scripts/browser-check.sh                 # build, seed, check, tear down (~2m)
scripts/browser-check.sh --dev           # ...served by `npm run serve` (~57s)
scripts/browser-check.sh --shots out/    # leave a PNG of every page it visits
KEEP_STACK=1 scripts/browser-check.sh    # leave it up on :3010 to poke at
REUSE=1 scripts/browser-check.sh         # ...and check again against it (~31s)
REUSE=1 ONLY=event scripts/browser-check.sh   # ...one section only (~11s)
```

It refuses to start on a stale checkout: the first thing it runs is
`scripts/dev-doctor.sh --deps` (below).

**`REUSE` and `ONLY` are for the middle of fixing something** (TODO3 M7). A full
run is two image builds, a boot, a seed and fourteen navigations, and until
these existed, reproducing one failing assertion cost all of it every time. Run
once with `KEEP_STACK=1`, then `REUSE=1 ONLY=event` for each attempt after that
— about eleven seconds.

- `REUSE=1` skips the teardown, the build, the boot and the seed, and implies
  `KEEP_STACK=1` (tearing down a stack you asked to reuse would break the next
  run). The fixture's ids are recorded in
  `${TMPDIR:-/tmp}/browser-check-<project>.state` — never the session cookie,
  which is a credential and is re-minted each run.
- It refuses, in one line and without dumping sixty lines of container log, if
  there is no recorded fixture, if nothing healthy is answering on :3010, or if
  the stack was booted in the **other mode**. That last one matters:
  `CORS_ORIGINS` is read by the server at boot, so a stack booted without
  `--dev` rejects every API call from the :8080 dev server — and the app then
  renders as a completely empty club, with no error anywhere, failing every
  assertion for a reason that has nothing to do with the code.
- `ONLY=<regexp>` is matched case-insensitively against section names (the
  route names, plus `event band tabs`, `dialogs`, `home phone`, `event phone`).
  It changes nothing about what is asserted, and it stamps `(PARTIAL: …)` on the
  verdict line so a filtered run cannot be quoted later as a full one. A pattern
  matching nothing exits 2 and lists the sections, rather than printing a green
  `ALL PASS` over zero assertions.
- Independently of both: a navigation now polls the route's own first assertion
  instead of sleeping a flat six seconds, with the six as a **ceiling**. A short
  grace still follows, because the two assertions every visit makes are about
  console output and some of it arrives after the page is usable — returning the
  instant the page renders would shrink the window those watch, which is a check
  getting weaker while appearing to get faster.

That script brings up its **own** compose project (`timeful-check`) on **:3010**
and **:27018**, so it never touches a dev stack you have running on :3002 — nor
its Mongo volume, which on these machines usually holds a restored production
dump. It seeds a five-member club, ten Chronicle entries, three gatherings and
one cast availability, mints a superAdmin cookie, runs the check and tears the
whole thing down. On failure it prints the stack logs before it does.

The other two still run by hand.

**Run the signed-out check after ANY change to** the router guard
(`router/index.js`), `fetch_utils`' error path, or auth-dependent rendering:

```bash
npm run check:signed-out                       # defaults to http://localhost:3002
npm run check:signed-out -- http://localhost:8080
```

It launches Chrome with a **throwaway profile** so every run is cookie-less, and
asserts for each route both *where you land* and *that the destination actually
rendered*.

> **Why it exists.** E3 phase 3 shipped a redirect loop that made the site
> unreachable for anyone without a session — `/` and `/sign-in` ping-ponged and
> neither rendered, so nobody could log in at all. It only reproduces on a **cold
> load with no session cookie**, which a browser you are already signed into
> never reaches; the whole suite was green and the bug shipped anyway. The check
> was validated by reverting the fix and confirming all five routes fail.
>
> Note the render assertion is the load-bearing one, not the navigation count:
> the original loop produced *cancelled* navigations, which emit no
> `frameNavigated`, and every route still had ~31 elements of `App.vue` chrome —
> so "did anything render" reported the broken pages as fine.

**The signed-in check** covers the event page as a real member. Local sign-in
needs SMTP (OTP) or Google OAuth, neither wired in dev, so mint a session cookie
directly with `server/tools/mintsession`. It grants nothing `SESSION_SECRET`
does not already grant — guard the secret, not the tool — and lives inside the
server module so its codec cannot drift from the server's.

```bash
# 1. seed a member + an event with a confirmed gathering
mongosh "mongodb://localhost:27017/schej-it" --eval '
  const uid = new ObjectId();
  db.users.insertOne({_id:uid, email:"harness@example.test",
    firstName:"Harness", lastName:"Check", role:"member"});
  db.allowlist.insertOne({email:"harness@example.test", addedBy:"tester",
    role:"member", addedAt:new Date()});
  const eid = new ObjectId();
  db.events.insertOne({_id:eid, name:"Harness Event", type:"specific_dates",
    ownerId:uid, duration:2, dates:[new Date("2026-08-01T18:00:00Z")],
    numResponses:0, scheduledEvent:{startDate:new Date("2026-08-01T18:00:00Z"),
    endDate:new Date("2026-08-01T20:00:00Z")}});
  print(uid.toString() + " " + eid.toString());'

# 2. mint a cookie (SESSION_SECRET must match the running server)
cd server && COOKIE=$(SESSION_SECRET=... go run ./tools/mintsession <userId>)

# 3. run it
cd ../frontend && npm run check:signed-in -- http://localhost:3002 "$COOKIE" <eventId>
```

The allowlist row matters: `AuthRequired` enforces the roll on **every** request,
so a user who exists but is not on it gets 401 and the check reports the session
as rejected.

> **Why it exists.** E3 phase 4 deleted anonymous branches from components whose
> surviving arms only ever ran for signed-in users — a change shape that builds
> and lints clean while breaking at runtime (see TODO A11). It caught one:
> removing the fields from `SignUpForSlotDialog` left a `v-form` whose
> lazy-validation `formValid` never became true, permanently disabling the
> "Join slot" button.

**The routes check** (`npm run check:routes`) is the broad one: every route in
`src/router/index.js`, all five event band tabs, the New Gathering dialog, and a
390px pass — asserting for each that it rendered, that an identifying control is
present, and that the console stayed clean.

```bash
npm run check:routes -- http://localhost:3002 "$COOKIE" <eventId>
```

Prefer `scripts/browser-check.sh` (above), which supplies all three arguments.
Run it this way only against a stack you already have. **What the fixture needs
is not obvious**, and each of these was a failing assertion before it was a
known requirement:
>
> - A **superAdmin** session. `/members` is gated on `canInvite`, so a lesser
>   role silently redirects to `/home` and the route is never exercised.
> - A gathering with **`hasSpecificTimes: false`** (or with `times` filled in).
>   With it true and `times` empty, `ScheduleOverlap`'s `mounted()` opens
>   `SET_SPECIFIC_TIMES` — the creator's click-and-drag screen — so the page has
>   no band tabs, no "Mark availability" and no "Schedule event" at all.
> - **At least one response, by somebody else.** "Schedule event" needs
>   `numResponses > 0`, and the action button reads "Edit availability" rather
>   than "Mark availability" once the signed-in user has responded.
> - **Members and Chronicle entries.** An empty club renders its empty states
>   perfectly and asserts nothing about the list rendering that actually breaks.

> **Why it exists.** Nothing else in the repo can fail on a *rendered-page* bug.
> The `node` unit tier is pure JS deliberately extracted *out of* components and
> renders nothing; the `dom` tier (M6) mounts components, but under happy-dom,
> where there is no real CSS, no layout, no icon webfont and no viewport — so
> everything the two assertions below are aimed at is invisible to it by
> construction. This check is written against the DOM rather than against Vue so
> that it keeps its value across a framework upgrade.
>
> Two assertions are worth knowing about. **"exactly one band panel visible"**
> is aimed at `v-show` being defeated by Tailwind's `important: true`, which
> beats the inline `display: none` and shows every panel at once with no error
> anywhere; asserting *exactly one* catches that as well as nothing rendering.
> **The 390px pass** asserts no horizontal scroll, which is how a sixth band tab
> would announce itself.
>
> It was validated by breaking each assertion on purpose and confirming it
> flipped to FAIL — a rejected cookie, an injected `display: block !important`
> on the band panels, a synthetic `console.error`, a `[Vue warn]` on both
> `console.warn` and `console.error`, and an overflowing element.

**Framework warnings only appear in a dev build**, which is what `--dev` is for
(TODO3 M2). `npm run build` compiles every `[Vue warn]` and `[Vuetify]` out, and
that is what the frontend image serves — so in a normal run the twelve
"— no framework warnings" lines report PASS *whatever the app does*. Measured,
with a deliberate `<v-deliberately-not-a-component />` in `Landing.vue`: the dev
build reports one `[Vue warn]` and fails; the production build of the same
source reports zero warnings and zero errors, and the check says ALL PASS.

```bash
scripts/browser-check.sh --dev
```

That boots the same throwaway stack for its API and Mongo (`:3010`), skips the
frontend image entirely, and serves the app from `npm run serve` on `:8080`
pointed at it — which is also why it is the *faster* loop, 2m rather than 3m.
Two things make the split work, and both are load-bearing:

- `VUE_APP_API_URL` in `src/constants.js`. It used to hardcode
  `http://localhost:3002/api` whenever `NODE_ENV === "development"`, so a dev
  server could only ever talk to a stack on `:3002`.
- Ports are **not** part of a cookie's origin, so the one minted `session`
  cookie is accepted across `:8080` → `:3010`. The dev server's origin does have
  to be in `CORS_ORIGINS`, which the script exports for compose.

Use `SERVE_PORT=8081 scripts/browser-check.sh --dev` if you already have
something on 8080.

**Screenshots** (TODO3 M1). `--shots <dir>` leaves a full-page PNG of every page
the check visits, numbered in visit order — so a failing run hands over a picture
of the page that failed and not just the name of an assertion. CI uploads them as
`browser-check-shots-{prod,dev}`. For one page on a stack you already have:

```bash
cd frontend
npm run shot -- http://localhost:3002/                       # → frontend/shots/
npm run shot -- http://localhost:3002/home --cookie "$COOKIE" --phone --full
npm run shot -- http://localhost:3002/home --cookie "$COOKIE" \
  --click "call a gathering" --out /tmp/dialog.png           # open a dialog first
```

`frontend/shots/` and `/shots` are gitignored: these are pictures of a seeded
club or of real member data.

All three use a small CDP driver (`scripts/browser-check-lib.js`) over `ws`
rather than Puppeteer — these run occasionally before a deploy, and a ~100MB
browser download in devDependencies is a poor trade when Chrome is already
installed. Set `CHROME_PATH` if yours is not on `PATH`; the driver tries
`google-chrome`, `chromium` and `chromium-browser` in turn.

Remember to delete the seeded documents afterwards.

> **Rebuild the containers first, or the run means nothing.** `compose.dev.yaml` bakes the
> frontend bundle into the frontend image and the Go binary into the server image — `docker
> compose restart` re-runs the *old* artifacts, and both harnesses will happily report ALL PASS
> against code that predates your change. Before a pre-deploy run:
>
> ```bash
> docker compose -f compose.dev.yaml up -d --build frontend server
> ```
>
> The server registers a static route per file in `frontend/dist` **at startup**, so it needs
> restarting even when only the frontend changed — otherwise the new hashed filenames 404.
> This bit during the F11 close-out: the harnesses passed 5/5 against a stack older than every
> change in them, and the give-away was an API response missing a newly added field while the
> code beside it behaved as expected. When a check passes but a hand-inspection of the response
> disagrees, suspect the image before the code.

### `scripts/dev-doctor.sh` — is this machine actually the checkout?

The rule above is one of two stale-artifact traps on these boxes, and both are
silent: the wrong answer arrives looking exactly like a real one. `dev-doctor`
turns each into a message with the fixing command attached.

```bash
scripts/dev-doctor.sh          # installs + the dev stack's images + toolchain
scripts/dev-doctor.sh --deps   # just the install check (what browser-check runs)
PROJECT=timeful-check scripts/dev-doctor.sh   # some other compose project
```

It checks three things and exits non-zero on the first two:

1. **`frontend/node_modules` against `frontend/package.json`**, by major version.
   On 2026-08-11 this box's install held the entire Vue 2 stack — vue 2.7.16,
   vuetify 2.7.2, vuex 3.6.2 — against a `package.json` asking for 3.x, and
   `npm run test:unit` failed two files with `(0 , createStore) is not a
   function` pointing at `src/store/index.js`. That reads *precisely* like a
   Vuex-migration regression on `main`. It was a stale install: CI was green on
   the same commit and `npm ci` fixed it.
2. **Each running image's build time against the sources baked into it**, so the
   trap in the blockquote above announces itself. Frontend sources make the
   frontend image stale and server sources the server image — not both, because
   a check that cries wolf gets ignored.
3. **The toolchain versions it found** (docker, Go, node, npm, mongosh, Chrome),
   printed, never failed on — so a version lands in the log next to whatever the
   run produced, and "it worked on the other box" has somewhere to start.

`scripts/browser-check.sh` calls `--deps` before it builds anything. Only the
deps half: that script rebuilds its own images every run, so it can't be caught
by (2) — but it *does* run `npm run serve` from `node_modules` in `--dev` mode,
where a stale install is not a subtle wrongness, it is what executes.

### Post-deploy checks against the live site (`e2e/`)

**`e2e/` is the current home for anything that drives the deployed site**, and it
has its own guide — read [`e2e/README.md`](./e2e/README.md) before writing one.
It uses **Playwright** (a dependency of `e2e/package.json` only, deliberately not
of `frontend/`), reuses a session minted by `e2e/prod_login.js`, and is
**assert-only**: these run against production data as a real member, so nothing
there may create, edit or save. `e2e/smoke_prod.js` is the broad check.

`prod_login.js` reads the OTP out of the deployment's Mongo over SSH, since the
mail round trip isn't scriptable. It is in the repo — the *session file* it
writes (`prod_state.json`) is what's gitignored, along with the screenshots.

### Live production verification (`verify_f9_prod.js`)

An older, heavier check predates `e2e/` and still sits beside the two above:
`frontend/scripts/verify_f9_prod.js` drives the **deployed** site to prove the
@mention composer and rendering (F9) actually work there — the picker
opening/filtering/inserting, the token surviving the round trip to a real
`mentions` entry, the name rendering instead of the markup, thread headers
flattening, mobile. New work of this kind belongs in `e2e/`, not here.

It writes only to a throwaway gathering it creates and deletes in a `finally`
(the delete is itself asserted), and the one mention it writes names the
signed-in account — `mentionRecipients` drops a comment's own author, so the run
mails nobody. Keep both properties if you copy it as a template for the next
feature.

Unlike the two checks above it uses **Playwright**, which is not a dependency of
`frontend/`, so run it from a box that has one (`e2e/node_modules` will do) plus
a signed-in production storage state from `e2e/prod_login.js`:

```bash
NODE_PATH=/path/to/playwright/node_modules \
  PROD_STATE=/path/to/prod_state.json \
  node frontend/scripts/verify_f9_prod.js     # 35 checks, non-zero on failure
```

`PROD_BASE` overrides the target and `SHOT_DIR` (default: the OS temp dir) is
where its two screenshots land.

## CI (GitHub Actions)

- **`backend-ci.yml`** — on `server/**` changes: `go build` + **`go vet`** +
  **`golangci-lint` (blocking since 2026-07-28)** + `go test`, with an ephemeral
  Mongo service for the DB-backed tests. All three analysis steps run over
  `go list ./... | grep -v '/scripts'` — every package except the stale one-off
  migrations. Nothing here enumerates packages, so nothing can drift out of sync
  with the Testing section above (TODO E12).
- **`frontend-ci.yml`** — on `frontend/**` changes: lint + `check:vuetify-props`
  + `npm run test:unit` + build. `test:unit` runs **both** vitest projects, so
  the mounted-component tier (M6) is covered by the step that was already there.
- Both run on push to `main` and PRs. `gh run list` targets this repo directly
  (it was detached from the schej-it fork network), so no `--repo` flag is needed.

## Conventions

- Go module path is `sirtom/server` (renamed 2026-07-23); Mongo DB stays `schej-it` — internal
  names keep the old branding, don't rename.
- New API routes need Swag comments. Regenerate `docs/` from `server/` with
  `swag init --parseDependency --parseInternal` — **both flags are required**, a
  bare `swag init` aborts on `primitive.DateTime`. Pin the CLI to `@v1.16.1`.
- Server panics at startup if `SESSION_SECRET` is missing or < 32 chars.
