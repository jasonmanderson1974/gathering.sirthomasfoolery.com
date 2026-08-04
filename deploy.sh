#!/usr/bin/env bash
#
# One-command deploy for The Gathering. Run from the repo root ON THE DEV BOX:
#
#     ./deploy.sh
#
# This is the inverse of the old Docker deploy, which ran on the VM and pulled
# git. Now the dev box builds and pushes artifacts, and production carries no
# toolchain at all — no Go, no Node, no Docker. That keeps node_modules and
# build caches off the internet-facing host (a Docker build cache filled the old
# VM's disk and took the site down once), makes rollback a symlink flip rather
# than a rebuild, and means a broken build never reaches production.
#
# Layout on the host:
#   /opt/thegathering/releases/<sha>/{server,dist/}   one directory per release
#   /opt/thegathering/current -> releases/<sha>       what systemd runs
#   /opt/thegathering/logs/                           the app's cwd, stable
#
# Environment:
#   DEPLOY_HOST   override the target (default: the production host below)
#   SKIP_TESTS=1  skip the test run — for a hotfix you have already tested
set -euo pipefail

cd "$(dirname "$0")"

DEPLOY_HOST=${DEPLOY_HOST:-root@192.168.24.56}
APP_DIR=/opt/thegathering
KEEP_RELEASES=5

say() { printf '==> %s\n' "$1"; }
die() { printf '!! %s\n' "$1" >&2; exit 1; }

# ---------------------------------------------------------------- preflight --

# Two machines push to main, so a deploy from a stale checkout would quietly
# ship yesterday's code. Refuse rather than guess.
say "Checking the working tree"
[ -z "$(git status --porcelain)" ] || die "Working tree is dirty. Commit or stash before deploying."
git fetch origin --quiet
if [ -n "$(git log --oneline HEAD..origin/main)" ]; then
  die "Behind origin/main. Run 'git pull --ff-only' first — deploying now would ship stale code."
fi

SHA=$(git rev-parse --short HEAD)
say "Deploying ${SHA} to ${DEPLOY_HOST}"

# The frontend bakes these in at BUILD time, so an empty value doesn't fail —
# it ships a subtly broken site (a blank Google client ID means sign-in is dead
# on arrival). Assert before spending time on a build.
[ -f .env ] || die "No root .env — it holds the VUE_APP_* build args. See DEPLOYMENT.md."
set -a; . ./.env; set +a
[ -n "${CLIENT_ID:-}" ] || die "CLIENT_ID is empty in .env — the build would ship a broken sign-in."

if [ "${SKIP_TESTS:-}" != "1" ]; then
  say "Running tests"
  # Unquoted on purpose: the package list must word-split into many arguments.
  # MONGODB_URI is deliberately NOT set here. The DB-backed tests skip without
  # it, which is the outcome we want: the dev box's Mongo holds a restored copy
  # of production, and the db/ encryption sweep test WRITES to what it finds.
  # Pointing a deploy gate at real member data is not a trade worth making.
  # shellcheck disable=SC2046
  (cd server && go test $(go list ./... | grep -v '/scripts') >/dev/null) \
    || die "Backend tests failed."
  (cd frontend && npm run test:unit --silent >/dev/null 2>&1) \
    || die "Frontend tests failed."
fi

# ------------------------------------------------------------------- build --

STAGING=$(mktemp -d)
trap 'rm -rf "$STAGING"' EXIT

say "Building the server (static, no cgo)"
(cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -buildvcs=false \
  -ldflags "-s -w -X sirtom/server/routes.Version=${SHA}" \
  -o "$STAGING/server" .) || die "Go build failed."

say "Building the frontend"
(cd frontend && VUE_APP_GOOGLE_CLIENT_ID="${CLIENT_ID:-}" \
                VUE_APP_MICROSOFT_CLIENT_ID="${MICROSOFT_CLIENT_ID:-}" \
                VUE_APP_GOOGLE_MAPS_API_KEY="${GOOGLE_MAPS_API_KEY:-}" \
                npm run build >/dev/null 2>&1) || die "Frontend build failed."
cp -r frontend/dist "$STAGING/dist"

# Prove the client ID actually made it into the bundle rather than trusting that
# the env var was read. This is the failure that looks like a healthy deploy.
# -F: the client ID is full of dots, which are not regex wildcards here.
grep -rqF "${CLIENT_ID}" "$STAGING/dist/js/" \
  || die "CLIENT_ID is not present in the built bundle — sign-in would be broken."

# ------------------------------------------------------------------ deploy --

RELEASE="$APP_DIR/releases/$SHA"

# Plain readlink, NOT readlink -f: -f canonicalizes a path that doesn't exist
# yet, so on the very first deploy it returns the symlink's own path. Rolling
# back to that points `current` at itself, and systemd fails with 203/EXEC on a
# symlink loop. Accept a rollback target only if it is a real directory under
# releases/.
PREVIOUS=$(ssh "$DEPLOY_HOST" "readlink '$APP_DIR/current' 2>/dev/null" || true)
case "$PREVIOUS" in
  "$APP_DIR/releases/"*) ssh "$DEPLOY_HOST" "[ -d '$PREVIOUS' ]" || PREVIOUS="" ;;
  *) PREVIOUS="" ;;
esac

say "Shipping to ${RELEASE}"
ssh "$DEPLOY_HOST" "mkdir -p '$RELEASE'"
rsync -a --delete "$STAGING/server" "$STAGING/dist" "$DEPLOY_HOST:$RELEASE/"
ssh "$DEPLOY_HOST" "chown -R gathering:gathering '$RELEASE'"

say "Activating and restarting"
ssh "$DEPLOY_HOST" "ln -sfn '$RELEASE' $APP_DIR/current.new && mv -Tf $APP_DIR/current.new $APP_DIR/current"
ssh "$DEPLOY_HOST" "systemctl restart thegathering"

# --------------------------------------------------------------- health gate --

# The real /api/health: it pings Mongo and answers 503 when it can't, so this
# distinguishes "serving" from "serving but unable to reach the database" —
# which the old check, falling through to index.html, could not.
say "Waiting for health"
HEALTHY=false
for _ in $(seq 1 20); do
  # No curl -f: a 503 here is the interesting case, and -f would throw away the
  # body that says which dependency is down.
  BODY=$(ssh "$DEPLOY_HOST" "curl -sS --max-time 5 http://127.0.0.1:3002/api/health" 2>/dev/null || true)
  if [ -n "$BODY" ] && printf '%s' "$BODY" | grep -q '"status":"ok"'; then
    # Confirm the release that answered is the one just shipped, rather than an
    # old process that survived the restart and is happily serving stale code.
    printf '%s' "$BODY" | grep -q "\"version\":\"$SHA\"" \
      || die "Health is OK but reports a different version than ${SHA}: ${BODY}"
    HEALTHY=true
    break
  fi
  sleep 2
done

if [ "$HEALTHY" != true ]; then
  printf '!! Health check failed after 40s. Last response: %s\n' "${BODY:-<none>}" >&2
  if [ -n "$PREVIOUS" ] && [ "$PREVIOUS" != "$RELEASE" ]; then
    say "Rolling back to $(basename "$PREVIOUS")"
    ssh "$DEPLOY_HOST" "ln -sfn '$PREVIOUS' $APP_DIR/current.new && mv -Tf $APP_DIR/current.new $APP_DIR/current && systemctl restart thegathering"
    die "Rolled back to $(basename "$PREVIOUS"). The bad release is still on disk at ${RELEASE} for inspection."
  fi
  die "No previous release to roll back to. Investigate on the host: journalctl -u thegathering -n 50"
fi

say "Healthy on ${SHA}"

# Keep a handful of releases so a rollback never needs a rebuild.
ssh "$DEPLOY_HOST" "cd $APP_DIR/releases && ls -1dt */ 2>/dev/null | tail -n +$((KEEP_RELEASES + 1)) | xargs -r rm -rf" || true

say "Deployed ${SHA}."
