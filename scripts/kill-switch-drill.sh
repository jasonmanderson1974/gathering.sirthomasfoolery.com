#!/usr/bin/env bash
# The rollback drill for the service worker.
#
# WHY THIS IS A SCRIPT AND NOT AN INSTRUCTION. Shipping a service worker is the
# one change in this repo that can put a client beyond reach: a worker served as
# text/html fails its update check on the MIME type rather than updating, and
# because the SPA fallback answers any unmatched path with index.html, DELETING
# the worker does not undo it — it makes it permanent. The only way out is
# serving a replacement at the registered URL, which is
# `deploy/kill-service-worker.js`.
#
# So "we have a kill switch" is not a claim worth making on the strength of
# having written the file. This proves it against a real browser holding a real
# worker: it deploys the kill switch over the live one, forces the update check
# a client would make, and asserts the registration and every cache are gone and
# the app still renders without one.
#
# RUN IT BEFORE SHIPPING ANY CHANGE TO THE WORKER, not after something breaks.
#
#   ./scripts/browser-check.sh          # once, with KEEP_STACK=1
#   KEEP_STACK=1 ./scripts/kill-switch-drill.sh
#
# It leaves the check stack's worker replaced by the kill switch, so tear the
# stack down afterwards rather than reusing it.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

STATE_FILE="/tmp/browser-check-timeful-check.state"
BASE="${BASE:-http://localhost:3010}"
VOLUME="${VOLUME:-timeful-check_frontend_dev_dist}"

if [ ! -f "$STATE_FILE" ]; then
  echo "No stack fixture at $STATE_FILE."
  echo "Run 'KEEP_STACK=1 ./scripts/browser-check.sh' first (NOT --dev: the"
  echo "worker is only built and registered in a production build)."
  exit 2
fi

# shellcheck disable=SC1090
. "$STATE_FILE"

# Must match the secret the running stack was booted with, or the cookie it
# mints will be rejected and the drill will look like a worker failure.
export SESSION_SECRET="${SESSION_SECRET:-browser-check-session-secret-not-for-prod!!}"
COOKIE=$(cd server && go run ./tools/mintsession "$USER_ID")

exec node "$ROOT/scripts/kill-switch-drill.js" "$BASE" "$COOKIE" "$VOLUME"
