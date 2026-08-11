#!/usr/bin/env bash
#
# Boots a throwaway copy of the dev stack, seeds a superAdmin and a gathering,
# mints a session cookie, and runs the routes check against it.
#
# WHY THIS EXISTS (TODO3 L5): `frontend/scripts/check-routes.js` is the only
# thing in this repo that looks at a rendered page. The unit suite is pure JS
# with `environment: "node"` — no `.vue` file is imported anywhere and
# `@vue/test-utils` is not a dependency — so it stays green through a total
# rendering failure, and lint and the build are no better. Every browser-only
# bug this repo has paid for lived in that gap: `v-show` beaten by Tailwind's
# `important: true`, a purged class name built from a template string, a fifth
# band tab pushing a phone into horizontal scroll, K3's shipped dialog crash,
# K5's silently-broken toggles, L2's off-screen button. The check only ever ran
# by hand because it needs a booted stack, a session cookie and an event id.
# This script produces all three, so CI can run it.
#
# It is not CI-only: run it here to reproduce a CI failure exactly, or just to
# get the check run without wiring up a stack by hand.
#
#   scripts/browser-check.sh                 # build, seed, check, tear down
#   KEEP_STACK=1 scripts/browser-check.sh    # leave it up to poke at
#   scripts/browser-check.sh --dev           # ...against a DEV build (see below)
#   scripts/browser-check.sh --shots out/    # leave a PNG of every page visited
#
# --dev EXISTS BECAUSE TWELVE ASSERTIONS CANNOT FAIL WITHOUT IT (TODO3 M2). The
# frontend image runs `npm run build` — a PRODUCTION build — and Vue and Vuetify
# compile their warnings out of those entirely. Every "— no framework warnings"
# line in a normal run therefore reports PASS whatever the app does. That is the
# one channel a framework upgrade speaks through: a removed API usually WARNS
# rather than throwing, which is how K5, L1, L3 and L7 were all found, by hand.
# `--dev` boots the same stack for its API and Mongo but serves the app from
# `npm run serve` on :8080, where the warnings are real — and, incidentally,
# where the frontend image is not built at all, so it is also the faster loop.
#
# Needs: docker compose, Go (for tools/mintsession), node + `npm ci` already run
# in frontend/, and a Chrome or Chromium on PATH (or CHROME_PATH set).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INVOKED_FROM="$PWD" # for --shots, before the cd below moves the goalposts
cd "$ROOT"

DEV_BUILD=""
SHOTS_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --dev) DEV_BUILD=1; shift ;;
    --shots) SHOTS_DIR="${2:?--shots needs a directory}"; shift 2 ;;
    -h | --help) awk 'NR > 1 { if (!/^#/) exit; sub(/^# ?/, ""); print }' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

# A relative --shots is relative to where YOU ran this, which is the only
# reading that isn't a surprise — and it has to be made absolute here, because
# the check is started with `npm --prefix frontend`, and npm runs a script with
# its cwd set to the PACKAGE directory. `--shots shots` therefore wrote to
# frontend/shots, and CI's artifact upload looked at ./shots and found nothing:
# both legs green, twenty screenshots taken, none of them kept.
if [ -n "$SHOTS_DIR" ]; then
  case "$SHOTS_DIR" in
    /*) ;;
    *) SHOTS_DIR="$INVOKED_FROM/$SHOTS_DIR" ;;
  esac
  mkdir -p "$SHOTS_DIR"
fi

# Before anything is built, because both things it checks make this run report
# on artifacts that are not the checkout (TODO3 M5) — and in --dev mode a stale
# `frontend/node_modules` is not a subtle wrongness, it is what `npm run serve`
# actually executes.
echo "--- checking this machine is not stale"
scripts/dev-doctor.sh --deps

# Its OWN compose project and its OWN ports. A dev stack on :3002 with a
# restored production dump in it is a normal thing to have running on these
# machines, and seeding test documents into that — or worse, `down -v`ing it on
# the way out — would be an unpleasant surprise. Separate project name means
# separate containers and separate volumes.
PROJECT="${PROJECT:-timeful-check}"
export DEV_HTTP_PORT="${DEV_HTTP_PORT:-3010}"
export DEV_MONGO_PORT="${DEV_MONGO_PORT:-27018}"

# API_BASE is where the stack is; CHECK_BASE is where the browser is pointed.
# They are the same thing in a normal run and deliberately different under
# --dev, where the pages come from a Vue CLI dev server and only the /api calls
# go to the stack. Ports are not part of a cookie's origin, so the ONE minted
# `session` cookie is valid across the split — that is what makes this work at
# all, and it is why the dev server does not need a proxy.
API_BASE="http://localhost:${DEV_HTTP_PORT}"
SERVE_PORT="${SERVE_PORT:-8080}"
if [ -n "$DEV_BUILD" ]; then
  CHECK_BASE="http://localhost:${SERVE_PORT}"
  # Read by compose (see compose.dev.yaml): the dev server's origin must be
  # whitelisted or every API call from it is rejected and the app renders as an
  # empty database rather than as an error.
  export CORS_ORIGINS="$CHECK_BASE"
else
  CHECK_BASE="$API_BASE"
fi

# Must be >=32 chars, and mintsession must be given the SAME one the server is
# running with or the cookie it produces is silently rejected — which surfaces
# as every route landing on /sign-in. Exported so compose interpolates it into
# the server too, rather than the two sides each picking their own default.
export SESSION_SECRET="${SESSION_SECRET:-browser-check-session-secret-not-for-prod!!}"

# An ABSOLUTE path to the compose file, because this runs from the EXIT trap as
# well as from the body. An earlier version used a relative one and the teardown
# silently did nothing — the failure was swallowed by its own `|| true`, the
# stack stayed up, and the NEXT run seeded on top of the old database and died
# on a duplicate-key error that looked like anything but a missing teardown.
compose() { docker compose -f "$ROOT/compose.dev.yaml" -p "$PROJECT" "$@"; }

SERVE_PID=""
SERVE_LOG="$(mktemp -t browser-check-serve.XXXXXX.log)"

cleanup() {
  local status=$?

  # On any failure, the stack logs before it goes away. The check prints
  # PASS/FAIL per assertion, which says WHICH page broke but not why — and when
  # it is the stack that failed rather than a page, nothing else says anything
  # at all. Printing here rather than in the CI job means a local run is just as
  # informative, and means the teardown can't race the diagnosis.
  if [ "$status" -ne 0 ]; then
    echo "--- FAILED (exit $status) — last 60 lines of the stack:"
    compose logs --tail=60 2>&1 || true
    if [ -n "$SERVE_PID" ]; then
      echo "--- last 40 lines of the dev server:"
      tail -40 "$SERVE_LOG" 2>/dev/null || true
    fi
  fi

  # Killed by process GROUP, not by pid: `npm run serve` execs vue-cli-service as
  # a child, so killing $! alone leaves webpack holding :8080 and the NEXT --dev
  # run fails on a port nothing visible owns. The group exists because the launch
  # site turns on job control (`set -m`) for exactly that reason.
  if [ -n "$SERVE_PID" ]; then
    kill -- "-$SERVE_PID" 2>/dev/null || kill "$SERVE_PID" 2>/dev/null || true
    wait "$SERVE_PID" 2>/dev/null || true
  fi
  rm -f "$SERVE_LOG"
  if [ -n "${KEEP_STACK:-}" ]; then
    echo "--- KEEP_STACK set: leaving $PROJECT up on $API_BASE"
    return
  fi
  echo "--- tearing down $PROJECT"
  compose down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Start from nothing, every time. A stack left behind by an interrupted run (or
# by KEEP_STACK) keeps its Mongo volume, and seeding a second club into it fails
# on the allowlist's unique email index — which reports as a duplicate-key error
# rather than as "you have a stale stack".
echo "--- clearing any previous $PROJECT stack"
compose down -v --remove-orphans >/dev/null 2>&1 || true

echo "--- building and starting the stack ($PROJECT on $DEV_HTTP_PORT)"
# --build is not optional: the frontend bundle is baked into the frontend image
# and the Go binary into the server image, so without it the whole run reports
# on artifacts that predate the change under test.
if [ -n "$DEV_BUILD" ]; then
  # Only Mongo and the API. The frontend image is not built at all, because in
  # this mode nothing is served from it — the pages come from `npm run serve`.
  # `--no-deps` on the server is what skips it (compose.dev.yaml has the server
  # depending on the frontend service, to get the dist volume populated first).
  #
  # The server then boots with an empty dist and logs "index.html not found",
  # which is correct and harmless here: nothing asks it for a page. It still
  # serves every /api route, which is all this mode wants from it.
  compose up -d --wait mongo
  compose up -d --build --no-deps server
else
  compose up -d --build
fi

echo "--- waiting for the server"
for i in $(seq 1 90); do
  if curl -fsS "$API_BASE/api/health" >/dev/null 2>&1; then break; fi
  if [ "$i" = 90 ]; then
    echo "server never became healthy; last 50 lines:" >&2
    compose logs --tail=50 server >&2
    exit 1
  fi
  sleep 2
done

# One fixture, shared with `scripts/dev-up.sh` (TODO3 M4). mongosh is given as
# an explicit command rather than left to the default host one: the CI runner has
# no mongosh installed, only the container does.
SEED_OUT=$(scripts/seed-club.sh "$API_BASE" \
  docker compose -f "$ROOT/compose.dev.yaml" -p "$PROJECT" \
  exec -T mongo mongosh --quiet "mongodb://localhost:27017/schej-it")
read -r USER_ID RESPONDER_ID EVENT_ID <<<"$SEED_OUT"
if ! [[ "$EVENT_ID" =~ ^[0-9a-f]{24}$ ]]; then
  echo "seed-club.sh did not produce an event id; it said: $SEED_OUT" >&2
  exit 1
fi

# Minted here rather than returned by seed-club.sh: a session cookie is a
# credential, and that script's stdout is echoed into a CI log.
COOKIE=$(cd server && go run ./tools/mintsession "$USER_ID")

if [ -n "$DEV_BUILD" ]; then
  echo "--- starting the dev server on $CHECK_BASE (this is the slow part now)"
  # VUE_APP_API_URL is the whole reason this mode works: `src/constants.js` used
  # to hardcode http://localhost:3002/api whenever NODE_ENV was development, so a
  # dev server could only ever talk to a stack on :3002 — never to this one
  # (TODO3 M2).
  # `set -m` gives this background job a process group of its own, whose pgid is
  # $!. Without it every background job shares the script's group and the
  # teardown has no way to reach vue-cli-service, which is npm's child.
  set -m
  VUE_APP_API_URL="$API_BASE/api" \
    npm --prefix "$ROOT/frontend" run --silent serve -- --port "$SERVE_PORT" \
    >"$SERVE_LOG" 2>&1 &
  SERVE_PID=$!
  set +m

  # Waited on by polling the port rather than by grepping the log for "Compiled
  # successfully": webpack prints that once and this can start after it.
  for i in $(seq 1 120); do
    if curl -fsS "$CHECK_BASE" >/dev/null 2>&1; then break; fi
    if ! kill -0 "$SERVE_PID" 2>/dev/null; then
      echo "the dev server exited before it served anything:" >&2
      tail -40 "$SERVE_LOG" >&2
      exit 1
    fi
    if [ "$i" = 120 ]; then
      echo "the dev server never came up on $CHECK_BASE:" >&2
      tail -40 "$SERVE_LOG" >&2
      exit 1
    fi
    sleep 2
  done
fi

echo "--- running the routes check against $CHECK_BASE"
# `--prefix` rather than `cd frontend`: the EXIT trap runs wherever the script
# left the shell, and it needs to still be somewhere compose and the repo make
# sense.
SHOT_ARGS=()
[ -n "$SHOTS_DIR" ] && SHOT_ARGS=(--shots "$SHOTS_DIR")
npm --prefix "$ROOT/frontend" run --silent check:routes -- \
  "$CHECK_BASE" "$COOKIE" "$EVENT_ID" "${SHOT_ARGS[@]}"
