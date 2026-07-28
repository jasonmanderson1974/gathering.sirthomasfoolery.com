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

The event location field is plain free text by default. Setting
`VUE_APP_GOOGLE_MAPS_API_KEY` in `frontend/.env` adds Google address
suggestions on top; with no key the inputs behave exactly as they did before,
so this is never required to build or run anything.

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

**Backend** (`cd server`) — needs a Go toolchain and a reachable Mongo for the
`db` integration tests (`compose.dev.yaml` provides one on `localhost:27017`):
```bash
MONGODB_URI=mongodb://localhost:27017 go test ./models/ ./routes/ ./utils/ ./db/
```
> `go test ./...` fails on the stale one-off `server/scripts/` (outdated model
> fields) — build/test the specific packages listed above instead.

If you have no local Go toolchain, run the tests in a container (matches CI):
```bash
docker run --rm -e MONGODB_URI=mongodb://host.docker.internal:27017 \
  -v "$PWD/server:/src" -w /src golang:1.25-alpine \
  sh -c "go build . && go test ./models/ ./routes/ ./utils/ ./db/"
```

**What's covered today:** role/permission logic (`models`), the admin
permission guards (`routes`, handler-level), the rate limiter + email/phone
helpers (`utils`), the allowlist gate CRUD (`db`, integration), and the frontend
role getters + phone formatter. **Not yet covered:** most HTTP handlers'
happy-path, email-change/OTP flows, middleware. `go test` passing means "compiles
+ these pass" — not full correctness.

## CI (GitHub Actions)

- **`backend-ci.yml`** — on `server/**` changes: `go build .` + `go test` for
  `models/ routes/ utils/ db/`, with an ephemeral Mongo service for the `db`
  integration tests.
- **`frontend-ci.yml`** — on `frontend/**` changes: `npm run test:unit` + build.
- Both run on push to `main` and PRs. `gh run list` targets this repo directly
  (it was detached from the schej-it fork network), so no `--repo` flag is needed.

## Conventions

- Go module path is `sirtom/server` (renamed 2026-07-23); Mongo DB stays `schej-it` — internal
  names keep the old branding, don't rename.
- New API routes need Swag comments; run `swag init` in `server/` to regenerate
  `docs/`.
- Server panics at startup if `SESSION_SECRET` is missing or < 32 chars.
