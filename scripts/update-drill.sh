#!/usr/bin/env bash
# Can a member actually take an update?
#
# The worker deliberately does not take over mid-session, so taking an update is
# something the app has to OFFER — and an offer that fails to appear strands
# someone on an old build with no way through short of force-quitting. That is
# not hypothetical: the prompt was originally wired only to `updatefound`, so it
# appeared once and never again after a refresh. This drill is what found it.
#
#   ./scripts/browser-check.sh          # once, with KEEP_STACK=1
#   ./scripts/update-drill.sh
#
# It mutates the check stack's worker, so tear that stack down afterwards.
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

exec node "$ROOT/scripts/update-drill.js" "$BASE" "$COOKIE" "$VOLUME"
