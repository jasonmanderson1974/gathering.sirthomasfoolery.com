#!/usr/bin/env bash
#
# Brings up the local dev stack and, optionally, puts a club in it worth looking
# at — the SAME club `scripts/browser-check.sh` asserts against.
#
# WHY THIS EXISTS (TODO3 M4): the dev stack on :3002 held 0 users, 0 events and
# 0 allowlist rows, and the one populated fixture this repo knows how to build
# was locked inside browser-check.sh, in a stack deleted at the end of every
# run. So there was nothing to click through even after a rebuild — and, before
# M3, no way to sign in if there had been. "Bring up the app and look at it" was
# a five-minute manual setup nobody did, which is why the automated checks were
# the only thing that ever looked at this app.
#
#   scripts/dev-up.sh                 # build and start, leave the data alone
#   scripts/dev-up.sh --seed          # ...and seed a club (refuses if seeded)
#   scripts/dev-up.sh --seed --force  # ...replacing any club already in there
#   scripts/dev-up.sh --no-build      # start what is already built (fast)
#   scripts/dev-up.sh --down          # stop it, KEEPING the Mongo volume
#
# This is the `timeful-dev` project on :3002/:27017 — your ordinary dev stack,
# whose Mongo volume on these machines habitually holds a restored production
# dump. Nothing here ever runs `down -v`, and the seed only ever deletes
# documents it can prove it created. The throwaway stack is browser-check.sh's,
# and that one is a different compose project on different ports.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PROJECT="${PROJECT:-timeful-dev}"
DEV_HTTP_PORT="${DEV_HTTP_PORT:-3002}"
BASE="http://localhost:${DEV_HTTP_PORT}"

# Must match what the server runs with, or the cookies `seed-club.sh` mints are
# silently rejected. Same default as compose.dev.yaml, and exported so compose
# interpolates this one rather than each side picking its own.
export SESSION_SECRET="${SESSION_SECRET:-dev-session-secret-not-for-prod-change-me!!}"

SEED=""
FORCE=""
BUILD=1
DOWN=""
while [ $# -gt 0 ]; do
  case "$1" in
    --seed) SEED=1; shift ;;
    --force | --reseed) FORCE=1; SEED=1; shift ;;
    --no-build) BUILD=""; shift ;;
    --down) DOWN=1; shift ;;
    -h | --help) awk 'NR > 1 { if (!/^#/) exit; sub(/^# ?/, ""); print }' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

compose() { docker compose -f "$ROOT/compose.dev.yaml" -p "$PROJECT" "$@"; }

if [ -n "$DOWN" ]; then
  echo "--- stopping $PROJECT (the Mongo volume is kept)"
  compose down --remove-orphans
  exit 0
fi

if [ -n "$BUILD" ]; then
  echo "--- building and starting $PROJECT on $BASE"
  # --build is not optional, and this is the trap dev-doctor exists for: the
  # frontend bundle is baked into the frontend image and the Go binary into the
  # server image, so `docker compose restart` re-runs the OLD artifacts and the
  # app you then look at is not the app you are working on.
  compose up -d --build
else
  echo "--- starting $PROJECT on $BASE (no rebuild — you asked)"
  compose up -d
fi

echo "--- waiting for the server"
for i in $(seq 1 90); do
  if curl -fsS "$BASE/api/health" >/dev/null 2>&1; then break; fi
  if [ "$i" = 90 ]; then
    echo "server never became healthy; last 50 lines:" >&2
    compose logs --tail=50 server >&2
    exit 1
  fi
  sleep 2
done

if [ -n "$SEED" ]; then
  # Through the container's mongosh, not the host's, so this works on a machine
  # that has never installed one — and so it reaches the same Mongo the server
  # is actually using rather than whatever :27017 happens to be.
  SEED_FORCE="${FORCE:+1}" \
    scripts/seed-club.sh "$BASE" \
    docker compose -f "$ROOT/compose.dev.yaml" -p "$PROJECT" \
    exec -T mongo mongosh --quiet "mongodb://localhost:27017/schej-it"
fi

cat <<EOF

--- $PROJECT is up at $BASE

To sign in as a real member (TODO3 M3 made this possible locally):

  1. open $BASE/sign-in
  2. enter  harness@example.test    (superAdmin; ambrose@ is admin, and
                                     cornelius@ / percival@ / reginald@ are
                                     members — all @example.test)
  3. read the code out of the server log:

       docker compose -f compose.dev.yaml -p $PROJECT logs server | grep 'DEV: otp'

The code is logged instead of mailed because no SMTP is configured here. That
branch is gated on debug mode AND absent credentials, so no production
configuration can reach it.

Screenshots without a browser of your own:

  cd frontend && npm run shot -- $BASE/sign-in

Stop it with \`scripts/dev-up.sh --down\` (keeps the data) — never \`down -v\`,
which would take the Mongo volume with it.
EOF
