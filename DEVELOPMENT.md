# Development & Workflow

How this repo is developed and shipped. Read this first when setting up a new
machine. (Deployment mechanics for the server itself are in `DEPLOYMENT.md`.)

## The setup

- **Two dev machines**, both push directly to `main`:
  - **VM-adjacent box** — has SSH access to the production VM, so it deploys.
  - **Other machine(s)** — no VM access; cannot deploy.
- **Production** is a single VM (`gathering.sirthomasfoolery.com`) behind a
  Cloudflare tunnel, running Docker Compose (`compose.yaml`: mongo + frontend
  build + Go server). It tracks `main`.
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

## Deploying (VM-adjacent box only)

SSH to the VM and run the deploy script from the repo root:

```bash
./deploy.sh
```

It pulls `main`, rebuilds only the service(s) whose code changed (`server/`
and/or `frontend/`), recreates the server when the frontend changed (the Go
server registers the frontend's static files at startup), health-checks
`/api/health`, and prunes the Docker build cache (the 30G disk fills fast).
Docs/config-only changes skip the rebuild.

## Local development

Neither dev machine holds prod secrets, so use the self-contained dev stack:

```bash
docker compose -f compose.dev.yaml up -d --build
open http://localhost:3002
```

It boots mongo + frontend + server with dummy secrets and exposes Mongo on
`:27017`. **Caveat:** external-service auth does NOT work locally — Gmail SMTP
(email OTP codes) and Google OAuth (calendar) aren't configured. Use it for
build/boot/UI smoke tests and for running the backend integration tests, not for
full login.

### Event location address lookup (optional)

The event location field is plain free text by default. A Maps key adds Google
address suggestions on top; with no key the inputs behave exactly as they did
before, so this is never required to build or run anything.

Where the key goes depends on how you're running the frontend:

| Running via                        | Put the key in                                    |
| ---------------------------------- | ------------------------------------------------- |
| `npm run serve` / `npm run build`  | `frontend/.env` as `VUE_APP_GOOGLE_MAPS_API_KEY`  |
| `compose.dev.yaml` (local Docker)  | root `.env` as `GOOGLE_MAPS_API_KEY`              |
| production (`compose.yaml`)        | root `.env` as `GOOGLE_MAPS_API_KEY`              |

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
npm run test:unit
```

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

If you have no local Go toolchain, run the tests in a container (matches CI):
```bash
docker run --rm -e MONGODB_URI=mongodb://host.docker.internal:27017 \
  -v "$PWD/server:/src" -w /src golang:1.25-alpine \
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
  -e CGO_ENABLED=1 -v "$PWD/server:/src" -w /src golang:1.25 \
  sh -c "go test -race \$(go list ./... | grep -v '/scripts')"
```

**What's covered today:** role/permission logic (`models`), the admin
permission guards (`routes`, handler-level), the rate limiter + email/phone
helpers (`utils`), the allowlist gate CRUD (`db`, integration), and the frontend
role getters + phone formatter. **Not yet covered:** most HTTP handlers'
happy-path, email-change/OTP flows, middleware. `go test` passing means "compiles
+ these pass" — not full correctness.

### Browser checks (manual, before deploying auth/UI changes)

Two headless-Chrome checks live in `frontend/scripts/`. They are **not** in CI —
they need a built frontend, a running server and Mongo — so run them by hand
when a change warrants it. Both exit non-zero on failure.

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

Both use a small CDP driver (`scripts/browser-check-lib.js`) over `ws` rather
than Puppeteer — these run occasionally before a deploy, and a ~100MB browser
download in devDependencies is a poor trade when Chrome is already installed.
Set `CHROME_PATH` if yours is not on `PATH`.

Remember to delete the seeded documents afterwards.

### Live production verification (`verify_f9_prod.js`)

A third, heavier check sits beside them: `frontend/scripts/verify_f9_prod.js`
drives the **deployed** site to prove the @mention composer and rendering (F9)
actually work there — the picker opening/filtering/inserting, the token
surviving the round trip to a real `mentions` entry, the name rendering instead
of the markup, thread headers flattening, mobile.

It writes only to a throwaway gathering it creates and deletes in a `finally`
(the delete is itself asserted), and the one mention it writes names the
signed-in account — `mentionRecipients` drops a comment's own author, so the run
mails nobody. Keep both properties if you copy it as a template for the next
feature.

Unlike the two checks above it uses **Playwright**, which is deliberately not a
dependency here, so run it from a box that has one plus a signed-in production
storage state (`prod_login.js`, kept out of the repo because it reads OTPs from
prod Mongo):

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
- **`frontend-ci.yml`** — on `frontend/**` changes: `npm run test:unit` + build.
- Both run on push to `main` and PRs. `gh run list` targets this repo directly
  (it was detached from the schej-it fork network), so no `--repo` flag is needed.

## Conventions

- Go module path is `sirtom/server` (renamed 2026-07-23); Mongo DB stays `schej-it` — internal
  names keep the old branding, don't rename.
- New API routes need Swag comments; run `swag init` in `server/` to regenerate
  `docs/`.
- Server panics at startup if `SESSION_SECRET` is missing or < 32 chars.
