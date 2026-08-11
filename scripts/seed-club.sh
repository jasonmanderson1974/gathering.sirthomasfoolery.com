#!/usr/bin/env bash
#
# Seeds one populated club — five members, ten Chronicle entries, three
# gatherings, one cast availability — and prints the ids it made.
#
# WHY THIS EXISTS (TODO3 M4): this fixture used to live as a heredoc inside
# `scripts/browser-check.sh`, in a stack that is torn down at the end of every
# run. The one populated club this repo knows how to build existed for three
# minutes at a time and nothing else could use it — while the dev stack on
# :3002 sat at 0 users, 0 events and 0 allowlist rows. So "bring up the app and
# look at it" was a five-minute manual setup nobody did, which is why the checks
# were the only thing that ever looked. One fixture, two consumers, and the
# interactive stack can no longer drift away from the one CI asserts against.
#
#   scripts/seed-club.sh <baseUrl> [mongoshCommand ...]
#
#   <baseUrl>          API base, e.g. http://localhost:3002 (no trailing /api)
#   [mongoshCommand]   how to reach Mongo. Defaults to a host mongosh against
#                      $SEED_MONGO_URI. browser-check.sh passes a
#                      `docker compose exec` form instead, because the CI runner
#                      has no mongosh of its own — only the container does.
#
# Env: SESSION_SECRET (must match the running server), SEED_MONGO_URI,
#      SEED_FORCE=1 (replace an existing seed rather than refuse).
#
# stdout is exactly one line, `<userId> <responderId> <eventId>`, so a caller
# can `read` it. Everything else goes to stderr.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

BASE="${1:-}"
if [ -z "$BASE" ]; then
  echo "usage: scripts/seed-club.sh <baseUrl> [mongoshCommand ...]" >&2
  exit 2
fi
shift
BASE="${BASE%/}"

SEED_MONGO_URI="${SEED_MONGO_URI:-mongodb://localhost:27017/schej-it}"
if [ "$#" -gt 0 ]; then
  MONGOSH=("$@")
else
  MONGOSH=(mongosh --quiet "$SEED_MONGO_URI")
fi

: "${SESSION_SECRET:?SESSION_SECRET must be set, and must match the running server}"

say() { echo "$@" >&2; }

# ---------------------------------------------------------------- documents ---

say "--- seeding members and the Chronicle"
# The two knobs are prepended as globals rather than passed as environment,
# because the mongosh command may be a `docker compose exec` that this script
# does not get to add `-e` flags to. seed-club.js reads either form.
SEED_PRELUDE="const SEED_FORCE = ${SEED_FORCE:-0}; const SEED_ADDED_BY = '${SEED_ADDED_BY:-seed-club}';"
SEED_ERR=$(mktemp)
trap 'rm -f "$SEED_ERR"' EXIT

# `set -e` is turned off across this one assignment on purpose. seed-club.js
# calls quit(1) when the club is already seeded, and under `set -e` a failing
# command substitution kills the script *at the assignment* — so the explanation
# it just printed is thrown away and the user gets a silent exit 1, which is
# precisely the diagnosis-free failure this whole script exists to avoid.
set +e
SEED_OUT=$("${MONGOSH[@]}" --eval "$SEED_PRELUDE
$(cat "$ROOT/scripts/seed-club.js")" 2>"$SEED_ERR" | tr -d '\r')
SEED_STATUS=$?
set -e

# stderr is captured separately rather than folded in, so a connection warning
# cannot end up on the line the ids are read from.
if [[ "$SEED_OUT" == *SEED_ERROR* ]]; then
  say "${SEED_OUT#SEED_ERROR }"
  exit 1
fi
if [ "$SEED_STATUS" -ne 0 ]; then
  say "mongosh failed (exit $SEED_STATUS):"
  say "$SEED_OUT"
  [ -s "$SEED_ERR" ] && say "$(cat "$SEED_ERR")"
  exit 1
fi

read -r USER_ID RESPONDER_ID <<<"$SEED_OUT"

# Checked, because mongosh reports a failed insert on stdout and still exits 0.
# Without this the ids come through empty, every request 401s, and the first
# thing that actually complains is a JSON parse error three steps downstream.
if ! [[ "$USER_ID" =~ ^[0-9a-f]{24}$ && "$RESPONDER_ID" =~ ^[0-9a-f]{24}$ ]]; then
  say "seeding did not produce two user ids. mongosh said:"
  say "$SEED_OUT"
  exit 1
fi
say "    signed in as $USER_ID, responder $RESPONDER_ID"

# ----------------------------------------------------------------- sessions ---

say "--- minting session cookies"
# mintsession lives inside the server module so its gorilla/sessions version
# cannot drift from the server's and mint cookies that look right but are
# rejected — which surfaces as every route landing on /sign-in.
COOKIE=$(cd "$ROOT/server" && go run ./tools/mintsession "$USER_ID")
RESPONDER_COOKIE=$(cd "$ROOT/server" && go run ./tools/mintsession "$RESPONDER_ID")

# --------------------------------------------------------------- gatherings ---

say "--- creating gatherings"
# Created over the API rather than inserted into Mongo, so the fixture cannot
# drift away from what the app actually writes — a hand-built document that no
# longer matches the model would fail the render assertions and read exactly
# like a real regression. The dates are relative to today for the same reason:
# a hardcoded date eventually falls into the past and changes what renders.
#
# `hasSpecificTimes` is false on purpose. With it true and `times` empty,
# ScheduleOverlap's mounted() opens SET_SPECIFIC_TIMES — the creator's
# click-and-drag screen — instead of the normal heatmap, and the event page then
# has no band tabs, no "Mark availability" and no "Schedule event" for the check
# to find. That is correct app behaviour and a wrong fixture.
DATES=$(node -e '
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + 14);
  d.setUTCHours(18, 0, 0, 0);
  const out = [];
  for (let i = 0; i < 3; i++) {
    const day = new Date(d);
    day.setUTCDate(d.getUTCDate() + i);
    out.push(day.toISOString());
  }
  process.stdout.write(JSON.stringify(out));')

jsonField() { # jsonField <field> — reads a JSON object on stdin
  node -e '
    let s = ""; process.stdin.on("data", (c) => (s += c)).on("end", () => {
      const v = JSON.parse(s)[process.argv[1]];
      if (!v) { console.error("no " + process.argv[1] + " in: " + s); process.exit(1); }
      process.stdout.write(String(v));
    });' "$1"
}

createEvent() { # createEvent <name>
  curl -fsS -X POST "$BASE/api/events" \
    -H 'Content-Type: application/json' \
    -H "Cookie: session=$COOKIE" \
    -d "{\"name\":\"$1\",\"duration\":4,\"type\":\"specific_dates\",
         \"dates\":$DATES,\"hasSpecificTimes\":false,\"notificationsEnabled\":false,
         \"blindAvailabilityEnabled\":false,\"daysOnly\":false,
         \"sendEmailAfterXResponses\":-1,\"timeIncrement\":15}" | jsonField eventId
}

# Three, because the dashboard assertion is about a populated dashboard — one
# lonely row does not exercise the list.
EVENT_ID=$(createEvent "Harness Gathering")
createEvent "Michaelmas Dinner" >/dev/null
createEvent "The Quarterly Smoker" >/dev/null
say "    event $EVENT_ID"

say "--- casting one availability, as a fellow member"
# As the responder, never as the signed-in user: numResponses > 0 is what puts
# "Schedule event" on the page, while the signed-in user staying unresponded is
# what keeps the action button reading "Mark availability".
SLOTS=$(printf '%s' "$DATES" | node -e '
  let s = ""; process.stdin.on("data", (c) => (s += c)).on("end", () => {
    // Four 30-minute slots from the start of the first day, which is inside the
    // 4-hour window the gathering was created with.
    const start = new Date(JSON.parse(s)[0]).getTime();
    const out = [];
    for (let i = 0; i < 4; i++) out.push(new Date(start + i * 30 * 60 * 1000).toISOString());
    process.stdout.write(JSON.stringify(out));
  });')
curl -fsS -X POST "$BASE/api/events/$EVENT_ID/response" \
  -H 'Content-Type: application/json' \
  -H "Cookie: session=$RESPONDER_COOKIE" \
  -d "{\"guest\":false,\"availability\":$SLOTS,\"ifNeeded\":[]}" >/dev/null

# The one line of stdout this whole script produces.
echo "$USER_ID $RESPONDER_ID $EVENT_ID"
