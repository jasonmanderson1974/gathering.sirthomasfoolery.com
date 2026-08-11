# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development workflow (READ FIRST — multi-machine)

This repo is developed from **more than one machine**, all pushing directly to `main`. Full details in
`DEVELOPMENT.md` (read it for deploy/local-dev/testing specifics). The rules that always apply:

- **Sync before making ANY changes.** Another machine may have pushed. Start every task with
  `git fetch origin` and confirm you're on the latest `origin/main` (`git pull --ff-only` if behind).
  Building on stale `main` causes divergence and rejected pushes.
- **`main` is the trunk; keep it green.** CI (GitHub Actions: `backend-ci.yml`, `frontend-ci.yml`) runs
  on every push but is *post-hoc*, not a merge gate — so build/test locally before pushing.
- **Deploys are manual and gate-kept, and only possible from the machine with SSH access to the prod
  host.** Deploy by running `./deploy.sh` **on the build box** — it builds here and rsyncs artifacts to
  `stf-thegathering` (192.168.24.56); it is NOT run on the server. Production runs no Docker: `mongod`,
  one static Go binary and `cloudflared` under systemd. If the current machine has no SSH access to
  that host, do NOT attempt to deploy — the human handles it. `origin/main` may be ahead of what's
  live; that's expected.
- **Local stack:** `docker compose -f compose.dev.yaml up` (dummy secrets; Mongo on :27017 for tests;
  SMTP/Google not wired, so login doesn't work locally). **Tests:** `npm run test:unit` (frontend) and
  `go test $(go list ./... | grep -v '/scripts')` with `MONGODB_URI` set (backend) — see
  `DEVELOPMENT.md`.

## Repository layout

Monorepo for Timeful (formerly Schej.it), a group availability/scheduling app.

- `frontend/` — Vue 3 + Vuetify 3 + Tailwind single-page app (Vue CLI 5 / webpack). Built output lands in `frontend/dist`. Migrated from Vue 2 on 2026-08-11 (TODO3 Part K). **Stay on Vue CLI / webpack — do not migrate to Vite**: its asset hashes (`index-DiwrgTda.js`, base64url, dash-separated) never match the immutable-cache regex J4 added for Vue CLI's `app.457eeeac.js`, so the caching would silently switch off with no error and no warning (TODO3 K, Phase 0).
- `server/` — Go (Gin) HTTP API backed by MongoDB. Also serves the built frontend as static files at the root.
- `deploy/` — the production host's configuration, version-controlled so the host is reproducible rather than remembered: systemd units, `mongod.conf`, logrotate, and the `install.sh` / `mongo-bootstrap.sh` bootstrap scripts. See `DEPLOYMENT.md`.
- `compose.dev.yaml` — the **local dev** stack (Mongo + a built frontend + the server). Docker is used *only* here; production runs no containers. The old Docker production stack (`compose.yaml`) was deleted on 2026-08-05 after the migration; recover it from git history if you ever need it.
- `PLUGIN_API_README.md` — `window.postMessage` API used by browser plugins to read/write availability on the frontend.
- `TODO3.md` — **the active backlog**, opened 2026-08-10. `TODO.md` (items `A*`–`E*`) and `TODO2.md` (`F*`–`H*`) are closed archives: still the place to read a cited item ID, never the place to add one. New items start at `J`.

The Go module is `sirtom/server` (renamed from `schej.it/server`, 2026-07-23). The Mongo DB name (`schej-it`) and the `SCHEJ_EMAIL_ADDRESS` env var are intentionally left unchanged (internal/infra — see TODO D0/D2). **Renaming the database was closed won't-do on 2026-08-10** (TODO2 G4, closure in `TODO3.md`): it is a human-run dump→restore→cutover with the backup chain as its only safety net, for no user-facing benefit. Don't re-raise it as a cleanup.

## Common commands

### Frontend (`cd frontend`)
- `npm run serve` — dev server with hot reload (port 8080).
- `npm run build` — production build into `frontend/dist`.
- `npm run test:unit` — Vitest (config in `vitest.config.mjs`, matches `src/**/*.test.js`, alias `@` → `src/`).
- `npm run test:unit:watch` — Vitest watch mode.
- Run a single test: `npx vitest run src/utils/date_utils.test.js` (or `-t "test name"`).
- `npm run check:vuetify-props` — diffs every prop bound on a `v-*` tag against Vuetify's own
  shipped `.d.ts` declarations, and fails on any that no longer exists. Runs in CI. **Vue 3 turns
  an unrecognised prop into a plain DOM attribute silently**, so a Vuetify 2 leftover renders at
  the wrong size, variant or position with lint, the unit suite, the build and `check:routes` all
  green — this is the only check that sees them (TODO3 L2/L3/L4).

### Browser check (repo root)
- `scripts/browser-check.sh` — boots its **own** copy of the dev stack (compose
  project `timeful-check` on `:3010`/`:27018`, so it can't touch a dev stack you
  have on `:3002`), seeds a populated club, mints a superAdmin cookie and runs
  `check:routes` against it, then tears it down. `KEEP_STACK=1` leaves it up.
  **This is the only thing in the repo that looks at a rendered page** — the
  unit suite is pure JS with `environment: "node"`, imports no `.vue` file and
  has no `@vue/test-utils`, so it stays green through a total rendering failure
  (TODO3 L5). Runs in CI as `browser-ci.yml`, on `frontend/**` *and* `server/**`.

### Backend (`cd server`)
- `air` — live-reload dev (install: `go install github.com/cosmtrek/air@latest`). Runs `main.go`, listens on `:3002` (`:3003` if `NODE_ENV=staging`).
- `go run main.go` — run without live reload. Pass `-release` to force `GIN_MODE=release`.
- `go test $(go list ./... | grep -v '/scripts')` — run all Go tests. **Not bare `go test ./...`**: it
  fails on the one-off migrations under `server/scripts/`, which reference model shapes from years ago
  and are deliberately not kept compiling (`server/scripts/README.md`). `/scripts` is the only
  exclusion, and `go vet`, `golangci-lint`, `govulncheck` and CI all use this same derived form —
  never spell the package list out (TODO E12).
- `go test ./db -run TestName` — run a single test (e.g. `./services/microsoftgraph`, `./services/reminders`).
- `govulncheck $(go list ./... | grep -v '/scripts')` — dependency vulnerability scan
  (`go install golang.org/x/vuln/cmd/govulncheck@latest`). Same exclusion as above, and here the
  failure mode is loud but misleading: a bare `govulncheck ./...` aborts in package loading on the
  `scripts/` migrations and looks like a broken checkout. **Expect exactly one finding** —
  GO-2026-5932, `x/crypto/openpgp` "unmaintained by design", which has no fixed version and is not
  imported by our code. Anything beyond that is new (TODO J12).
- `swag init --parseDependency --parseInternal` (in `server/`) — regenerate Swagger docs in `server/docs/` after editing route comments. **The two flags are required** — a bare `swag init` aborts with `cannot find type definition: primitive.DateTime` (swag can't introspect the Mongo driver types the allowlist models use); `--parseDependency` resolves them. Pin the CLI to the go.mod version (`go install github.com/swaggo/swag/cmd/swag@v1.16.1`; note its `--version` misreports as v1.8.12). Swagger UI is served at `http://localhost:3002/swagger/index.html`.
- MongoDB backup/restore: `mongodump --host=localhost:27017 --db=schej-it` / `mongorestore --uri mongodb://localhost:27017 ./dump --drop`.

### Required env vars for local server boot
`SESSION_SECRET` (≥32 chars) and `ENCRYPTION_KEY` (exactly 16/24/32 chars — raw AES key bytes) are both enforced at startup. `CLIENT_ID`/`CLIENT_SECRET` (Google OAuth) are required for most flows. See `server/.env.template` and `DEPLOYMENT.md` for the full list (Microsoft, Gmail, etc.).

For local frontend → local backend, set `CORS_ORIGINS=http://localhost:8080` in `server/.env`.

## Architecture

### Backend (Gin + MongoDB)
`server/main.go` wires everything: CORS, cookie sessions, Mongo init (`db.Init`), the email scheduler (`services/reminders.StartReminderScheduler`), then mounts API groups under `/api` via `routes.Init*`. After API routes, it walks `frontend/dist` and registers each file as a static route, loads `index.html` as a template, and falls back to a `NoRoute` handler (`noRouteHandler`, `server/main.go:327`) that serves that shell. **It does no DB lookup and injects no per-route meta tags** — it used to set per-event OG titles, and E3 deleted that on purpose, because it served gathering names to anyone who guessed a short id, with no session. Don't reintroduce it. The handler also sends `Cache-Control: no-cache, no-store, must-revalidate` on the shell, so a returning browser can't hold an `index.html` pointing at hashed chunks a later deploy removed.

- `routes/` — HTTP handlers grouped by domain, one file per area rather than one per model: `auth.go`, `user.go`, `users.go`, `admin.go`/`admin_profile.go`, `display_names.go`, `avatars.go`, `images.go`, `events.go`, `event_responses.go`, `event_emails.go`, `event_import.go`, `event_lists.go`, `personal_lists.go`, `personal_notes.go`, `expenses.go`, `expense_receipts.go`, `comments.go`, `mentions.go`, `mention_emails.go`, `polls.go`, `chronicle.go`, `folders.go`, `health.go`, `text.go`. Route comments use Swag annotations; regenerate `docs/` with the full `swag init` command above, flags included.
- `models/` — Mongo document structs. Core: `Event` (with `Rsvp`, `Poll`, `EventList`, `GatheringRecurrence`, `Remindee` nested in it), `User`, `Response`, `Folder`/`FolderEvent`, `CalendarAccount`, `Comment`, `Chronicle`, `Allowlist`, `Avatar`, `Otp`, `Location`, `DailyUserLog`, `Personal*` (My Lists / My Notes), `Expense`/`ExpenseSplit`/`ExpenseReceipt`, plus `Role` (`roles.go`), `EncryptedString` and the generic `Set[T]`.
- `db/` — Mongo accessors, one file per area (`events.go`, `users.go`, `folders.go`, `comments.go`, `chronicle.go`, `allowlist.go`, `avatars.go`, `event_lists.go`, `personal_lists.go`, `personal_notes.go`, `expenses.go`, `expense_receipts.go`, `health.go`, `utils.go`) plus `init.go` and `encryption_migration.go`. Treat this as the only layer that talks to Mongo.
- `services/` — external integrations. Notable: `calendar/` (Google, Outlook/Graph, Apple CalDAV via `jonyTF/go-webdav`, generic ICS), `auth/`, `contacts/`, `microsoftgraph/`, `reminders/` (in-process scheduler for every scheduled email). Note `services/gcloud/` and `services/listmonk/` are **deleted packages** — if either directory exists locally it holds nothing but a gitignored `logs.log`.
- `middleware/auth.go` — session-based auth middleware applied selectively by `routes.Init*`.
- `scripts/` — one-off Mongo migrations (dated folders like `20250417_responses_collection`). Run manually; don't import from runtime code. See `server/scripts/README.md`.
- `utils/` — generic helpers (`array_utils`, `mail_utils`, `email_layout`, `request_utils`, `response_utils`, `ratelimit`, `http`, `utils`).
- `logger/` — wraps log file (`logs.log`) + stdout via `gin.DefaultWriter`.

### Frontend (Vue 3 SPA)
- `src/router/index.js` — routes: `landing`, `home`, `event`, `settings`, `admin` (`MemberAdmin.vue`), `fellowship` (the member roll/directory), `chronicle`, `responded`, `sign-in`, `sign-up` (also `SignIn.vue`, with `initialIsSignUp`), `auth`, `privacy-policy`, `404`. Every route except the landing/auth surfaces is behind the guard — see `src/views/`.
- `src/store/index.js` — single (non-modular) Vuex store holding auth user, events, folders, the two remaining feature flags (`daysOnlyEnabled`, `overlayAvailabilitiesEnabled`), and dialog/snackbar state. The paywall and sign-up-sheet flags are gone with their features; don't reintroduce a flag for a feature that no longer exists.
- `src/components/` — organized by feature folder (`event/`, `home/`, `landing/`, `settings/`, `schedule_overlap/`, `calendar_permission_dialogs/`, `general/`) plus top-level shared components.
- `src/utils/` — date math (`date_utils.js`, **dayjs only** — `moment` and `spacetime` are long gone from `package.json` and from every import; it is 946 lines / 32 exports and **splitting it was closed won't-do on 2026-08-10**, TODO2 G2 — its size is not a defect, and a blind split has already been shown to break live behaviour), `fetch_utils.js` (API client), `plugin_utils.js` (handles the postMessage plugin API — see `PLUGIN_API_README.md`), `sign_in_utils.js`, `location_utils.js`, `markdown.js`, `services/` (`EventService.js`, `FolderService.js`, `ExpenseService.js`, `PersonalService.js` — thin wrappers over `fetch_utils`).
- Tailwind + Vuetify coexist; `tailwind.config.js` purges `src/**/*.{vue,js,...}`.
- **Icons are the MDI *webfont*, self-hosted and pinned** — `@mdi/font` at an exact version in
  `package.json`, imported in `src/main.js`, emitted by webpack into `dist/fonts/` (L8; it was an
  unpinned `@latest` CDN `<link>` until 2026-08-11). Keep the version exact, don't put it back on a
  CDN, and note `plugins/vuetify.js` selects the `mdi` icon set for this reason — `mdi-svg` wants
  SVG paths from `@mdi/js` instead. If the font ever fails to load, all 69 `mdi-*` names render as
  blank squares with nothing logged; `check:routes` asserts on `document.fonts` to catch that.
- **Vue 3 discards an unrecognised prop on a component silently** — no warning in dev, none in the
  build. That is the general rule the next three bullets are instances of, and it is why a Vuetify 2
  leftover renders at the wrong size, variant or position with lint, the unit suite, the build and
  `check:routes` all green. `npm run check:vuetify-props` is the only check that sees it (L2/L3/L4;
  75 real leftovers were found this way, not the 6 a read-through named). On a plain HTML tag the
  unknown prop is worse than dropped — it becomes a literal DOM attribute.
- **Vuetify 3's `@change` gives the native DOM event, not the new value** (Vuetify 2 gave the value).
  Use `@update:model-value` on Vuetify components; keep `@change` only where the element really is
  native (`<input type="file">` in `AvatarEditorDialog` / `ExpenseDialog`). K5 shipped this to
  production: a toggle POSTed an `Event` object where a `*bool` was bound, and four availability
  switches took `!!val` — `!!someEvent` is always `true`, so they could be turned on but never off.
  The wider lesson: on a framework bump, sweeping *one* event name is not sweeping the class.
- **`VForm.validate()` returns `Promise<{ valid, errors }>`**, and a Promise is always truthy, so
  `if (!this.$refs.form.validate()) return` is dead code (L1). Write
  `const { valid } = await this.$refs.form.validate(); if (!valid) return` and make the caller
  `async`. Don't "fix" it by adding `validate-on="submit"` — that installs neither the input nor the
  blur watcher, a pristine field reports `isValid === null`, and the `:disabled="!formValid"` both
  submit buttons carry would latch disabled forever. Note a rules change does not itself trigger
  validation, which is what makes GuestDialog's "install strict rules, then validate" pattern valid.
- **`tailwind.config.js` sets `important: true`, which breaks `v-show`.** `tw-flex`/`tw-block`/
  `tw-grid` compile to `display: … !important` and beat the inline `display: none` that `v-show`
  sets, so the element stays visible with no error anywhere. Use `v-if` on any element that both
  toggles and carries a Tailwind display utility (or move the utility to a child). Lint, unit
  tests and the build all pass with this bug present — only looking at the page catches it.
- **Tailwind purges on literal source text**, so a class name must appear whole in the source. Build
  a static map (`["", "tw-pl-6", "tw-pl-12"]`), never a template string like `` `tw-pl-${n}` `` —
  that emits no CSS at all.
- **Money is integer cents end to end** (Settle Up, F22) — `int64` in Go, `number` in JS, never a
  float in either. Parse with `parseAmount` in `components/event/expenseForm.js`, not `parseFloat`
  (`parseFloat("1.005") * 100` is `100.49999999999999`). An even split is computed by the server
  (`models.SplitEvenly`); `splitEvenlyPreview` mirrors it, remainder rule included, so the form's
  preview matches what gets stored — if one moves, move the other.
- **A fifth band tab does not fit across a phone**, so the tab row in `Event.vue` carries
  `tw-flex-wrap`. Adding a sixth without checking the page at 390px will put the whole document
  into a horizontal scroll, on every tab. Lint, unit tests and the build all pass with that bug.
- **`src/utils/index.js` is an `export *` barrel imported by ~40 components.** Don't add modules with
  heavy or DOM-dependent dependencies to it (e.g. `utils/markdown.js`, which pulls in DOMPurify);
  import those directly by path.
- There is **no service worker**, and no client of this origin can be holding one — the PWA was deliberately removed upstream (`f857320`, 2025-06-24), 13 months before this fork's first deploy (`cd1f103b`, 2026-07-22). Do not reintroduce one casually — **web push was closed won't-do on 2026-08-10** (TODO2 G3, closure
  recorded in `TODO3.md`), so bringing a worker back means reversing a decision, not picking up a plan. Upstream's `kill-sw.js` was deleted from the repo root in J11: it was never served, and it was aimed at *schej.it's* stale registrations, not ours. If a PWA is ever shipped **and then removed**, the kill switch has to be served at the **registered script URL** — Vue CLI's plugin registers `/service-worker.js` — because that is the only URL a stale worker ever re-fetches. A file at `/kill-sw.js` would unregister nothing, and the SPA fallback makes the do-nothing case permanent: `GET /service-worker.js` returns `index.html` as `text/html`, so the update check fails on MIME type and the stale worker survives instead of 404ing itself away.

### Frontend ↔ backend contract
- Same-origin in production: a **Cloudflare Tunnel** (`cloudflared` dialling out from the host) → Go on `127.0.0.1:3002`; Go serves `/api/*` and falls through to `index.html` for SPA routes. There is no reverse proxy on the host — Caddy was never the real setup, and `Caddyfile.example` is deleted. See `DEPLOYMENT.md`.
- Local dev: Vue CLI serves `:8080`, frontend calls `http://localhost:3002/api/*` (must whitelist via `CORS_ORIGINS`). Session cookie is `session` (cookie store, signed with `SESSION_SECRET`).
- Event IDs may be either the Mongo `_id` or a short ID; `db.GetEventByEitherId` handles both — prefer it when looking up events from route params.

### Plugin (browser extension) API
The frontend exposes `get-slots` / `set-slots` over `window.postMessage` with a `FILL_CALENDAR_EVENT` type and `requestId` for response matching. Implementation lives in `src/utils/plugin_utils.js`; spec in `PLUGIN_API_README.md`. Don't change message shapes without also updating that doc.

## Conventions worth knowing

- The Go module path is `sirtom/server`; imports use that prefix throughout.
- Mongo collection naming and indexes are established by the dated migration scripts in `server/scripts/` — when adding a new collection or index, follow the same dated-folder pattern.
- New API routes need Swag comments above the handler so `swag init` picks them up; otherwise they're invisible in `/swagger`.
- The server panics on startup if `SESSION_SECRET` is missing or shorter than 32 chars (`validateSessionSecret` in `main.go`).
- `frontend/dist` is consumed by the Go server at runtime — local server boot tries `./frontend/dist` then `../frontend/dist`, or honors `FRONTEND_DIST` env var.
