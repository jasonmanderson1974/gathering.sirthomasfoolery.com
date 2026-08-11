#!/usr/bin/env bash
#
# Is this checkout's *build output* actually the checkout? Prints what it found
# and exits non-zero, with the fixing command, when it is not.
#
# WHY THIS EXISTS (TODO3 M5): both halves of this machine were stale on arrival
# and neither said so. `frontend/node_modules` held the whole Vue 2 stack — vue
# 2.7.16, vuetify 2.7.2, vuex 3.6.2 — against a package.json asking for 3.x, so
# `npm run test:unit` failed 2 files with `(0 , createStore) is not a function`
# pointing at `src/store/index.js`. That reads exactly like a Vuex-migration
# regression on main. It was a stale install; CI was green on the same commit.
# Separately, the `timeful-dev` stack's images were built before the first commit
# of the Vue 3 migration and had been serving a Vue 2 bundle out of a Vue 3
# checkout for 18 hours.
#
# Both failures are silent, and that is the point: the wrong answer arrives
# looking exactly like a real one. This turns them into a message.
#
#   scripts/dev-doctor.sh            # deps + the dev stack's images + toolchain
#   scripts/dev-doctor.sh --deps     # just the install check (what CI-ish runs need)
#   PROJECT=timeful-check scripts/dev-doctor.sh   # inspect another compose project
#
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

DEPS_ONLY=""
[ "${1:-}" = "--deps" ] && DEPS_ONLY=1

PROJECT="${PROJECT:-timeful-dev}"
problems=0

say() { printf '%s\n' "$*"; }
fail() { # fail <what> <fix command>
  problems=$((problems + 1))
  say ""
  say "  ✗ $1"
  say "    fix: $2"
}

# ---------------------------------------------------------------- installs ---
# Majors only. A patch drift is normal and uninteresting; a major drift is the
# shape of the failure above — a whole framework generation behind what the
# source is written against.
say "--- frontend/node_modules vs frontend/package.json"
if [ ! -d frontend/node_modules ]; then
  fail "frontend/node_modules does not exist" "npm ci --prefix frontend"
else
  DRIFT=$(node -e '
    const fs = require("fs")
    const path = require("path")
    const pkg = JSON.parse(fs.readFileSync("frontend/package.json", "utf8"))
    const want = { ...pkg.dependencies, ...pkg.devDependencies }
    // The leading range operator is stripped rather than parsed: "^3.5.41",
    // "~5.0.0" and "7.4.47" all answer the only question asked here, which is
    // which major generation the source expects.
    const major = (v) => {
      const m = /(\d+)\./.exec(String(v).replace(/^[\^~>=<\s]+/, "") + ".")
      return m ? m[1] : null
    }
    const out = []
    for (const [name, range] of Object.entries(want)) {
      const p = path.join("frontend/node_modules", name, "package.json")
      if (!fs.existsSync(p)) { out.push(`${name}: not installed (want ${range})`); continue }
      const got = JSON.parse(fs.readFileSync(p, "utf8")).version
      const w = major(range), g = major(got)
      if (w && g && w !== g) out.push(`${name}: installed ${got}, package.json wants ${range}`)
    }
    process.stdout.write(out.join("\n"))
  ' 2>/dev/null)
  if [ -n "$DRIFT" ]; then
    say "$DRIFT" | sed 's/^/    /'
    fail "$(printf '%s' "$DRIFT" | grep -c '') dependencies are a major version off (or missing)" \
      "npm ci --prefix frontend"
  else
    say "    ok — every dependency's major matches"
  fi
fi

if [ -n "$DEPS_ONLY" ]; then
  say ""
  [ "$problems" -eq 0 ] && say "dev-doctor: ok" || say "dev-doctor: $problems problem(s)"
  exit $((problems == 0 ? 0 : 1))
fi

# ------------------------------------------------------------------ images ---
# compose.dev.yaml bakes the frontend bundle into the frontend image and the Go
# binary into the server image, so `docker compose restart` re-runs the OLD
# artifacts and every harness happily reports ALL PASS against code that
# predates the change under test.
say ""
say "--- $PROJECT images vs the source tree"
if ! command -v docker >/dev/null 2>&1; then
  say "    docker not on PATH — skipping"
elif ! docker compose -f compose.dev.yaml -p "$PROJECT" ps -aq 2>/dev/null | grep -q .; then
  say "    no $PROJECT stack — nothing to be stale"
else
  for svc in frontend server; do
    # Via the CONTAINER rather than `compose images`: that subcommand fails
    # outright ("No such image") when any one of the project's images has been
    # pruned out from under a still-running container, which takes the other
    # services' answers down with it.
    CID=$(docker compose -f compose.dev.yaml -p "$PROJECT" ps -aq "$svc" 2>/dev/null | head -1)
    [ -z "$CID" ] && { say "    $svc: no container"; continue; }

    IMAGE=$(docker inspect --format '{{.Image}}' "$CID" 2>/dev/null)
    BUILT=$(docker image inspect --format '{{.Created}}' "$IMAGE" 2>/dev/null)
    if [ -z "$BUILT" ]; then
      # The image is gone but the container is still running off it. Nothing can
      # be rebuilt from that, and the container's own start time says nothing
      # about when the bundle inside it was compiled.
      fail "the $PROJECT $svc container is running an image that no longer exists" \
        "docker compose -f compose.dev.yaml -p $PROJECT up -d --build $svc"
      continue
    fi

    # The source that ends up INSIDE each image, and only that: a server change
    # does not make the frontend image stale, and saying it does trains people
    # to ignore this.
    case "$svc" in
      frontend) WATCH=(frontend/src frontend/public frontend/package.json frontend/package-lock.json frontend/vue.config.js frontend/tailwind.config.js) ;;
      server) WATCH=(server) ;;
    esac
    NEWER=$(find "${WATCH[@]}" -newermt "$BUILT" -type f \
      -not -path '*/node_modules/*' -not -name 'logs.log' -print 2>/dev/null | head -5)

    if [ -n "$NEWER" ]; then
      say "    $svc image built $BUILT, but these are newer:"
      say "$NEWER" | sed 's/^/      /'
      fail "the $PROJECT $svc image predates the source it is supposed to be built from" \
        "docker compose -f compose.dev.yaml -p $PROJECT up -d --build $svc"
    else
      say "    $svc: ok — image ($BUILT) is newer than its sources"
    fi
  done
fi

# --------------------------------------------------------------- toolchain ---
# Printed, never failed on: the point is that a version lands in the run's log
# next to whatever it produced, so "it worked on the other box" has an answer.
say ""
say "--- toolchain"
ver() { # ver <label> <command...>
  local label="$1"; shift
  if command -v "$1" >/dev/null 2>&1; then
    say "    $label: $("$@" 2>&1 | head -1)"
  else
    say "    $label: NOT FOUND"
  fi
}
ver docker docker --version
ver go go version
ver node node --version
ver npm npm --version
ver mongosh mongosh --version
if [ -n "${CHROME_PATH:-}" ] && command -v "$CHROME_PATH" >/dev/null 2>&1; then
  ver chrome "$CHROME_PATH" --version
else
  ver chrome google-chrome --version
fi

say ""
if [ "$problems" -eq 0 ]; then
  say "dev-doctor: ok"
  exit 0
fi
say "dev-doctor: $problems problem(s) above — fix them before trusting a harness run"
exit 1
