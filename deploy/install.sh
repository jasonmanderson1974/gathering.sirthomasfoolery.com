#!/usr/bin/env bash
#
# Bootstrap stf-thegathering from the repo, so the host is reproducible rather
# than remembered. Idempotent: safe to re-run after any change to deploy/.
#
#     scp -r deploy root@192.168.24.56:/tmp/  &&  ssh root@192.168.24.56 /tmp/deploy/install.sh
#
# What it does NOT do, on purpose:
#   - create the Mongo users (needs a password; see mongo-bootstrap.sh)
#   - write /etc/thegathering/env (holds secrets; copied in by hand, once)
#   - install the app itself (that is deploy.sh, from the dev box)
#
# There is no Go, Node, or Docker on this host and there should never be. The
# build happens on the dev box; this machine only runs artifacts.
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root on the target host." >&2
  exit 1
fi

HERE="$(cd "$(dirname "$0")" && pwd)"
APP_DIR=/opt/thegathering
ETC_DIR=/etc/thegathering

MONGO_SERIES=8.0

# Drop any MongoDB apt source from a different series before touching apt. A
# stale one is not inert: `apt-get update` fails hard on a repo with no Release
# file, which breaks every install below with an error pointing at MongoDB
# rather than at the leftover file actually causing it.
for f in /etc/apt/sources.list.d/mongodb-org-*.list; do
  [ -e "$f" ] || continue
  case "$f" in
    */mongodb-org-$MONGO_SERIES.list) ;;
    *) echo "==> Removing stale apt source $(basename "$f")"; rm -f "$f" ;;
  esac
done

echo "==> Base packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq ca-certificates curl gnupg rsync logrotate unattended-upgrades >/dev/null

echo "==> Service account and directories"
if ! id gathering >/dev/null 2>&1; then
  useradd --system --home-dir "$APP_DIR" --shell /usr/sbin/nologin gathering
fi
# releases/  built artifacts, one dir per commit
# logs/      the app's cwd; logs.log lives here, stable across releases
# backups/   nightly dumps, pulled from the dev box
# bin/       host-side scripts (backup.sh)
mkdir -p "$APP_DIR"/{releases,logs,backups,bin}
chown -R gathering:gathering "$APP_DIR"
# Backups contain every member's data; keep them off other users' radar.
chmod 0750 "$APP_DIR/backups"

mkdir -p "$ETC_DIR"
chown root:root "$ETC_DIR"
chmod 0750 "$ETC_DIR"

echo "==> MongoDB 8.0"
# 8.0, not the 7.0 the old host runs. Not a preference: MongoDB never published
# 7.0 packages for Ubuntu 24.04 (noble) — 8.0 is the first line that supports
# it. The alternative was pinning the jammy repo on a noble host, which trades a
# supported major version for an unsupported distro pairing. Worse deal.
#
# Verified rather than assumed, before choosing it: the full Go test suite
# (integration tests included) passes against 8.2 on the current driver, and the
# real 7.0 production dump restores into it with every collection count and all
# 15 indexes intact. mongodump/mongorestore is a logical BSON dump, which is
# supported across major versions.
if ! command -v mongod >/dev/null 2>&1; then
  curl -fsSL https://pgp.mongodb.com/server-$MONGO_SERIES.asc \
    | gpg -o /usr/share/keyrings/mongodb-server-$MONGO_SERIES.gpg --dearmor --yes
  echo "deb [ arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-$MONGO_SERIES.gpg ] https://repo.mongodb.org/apt/ubuntu noble/mongodb-org/$MONGO_SERIES multiverse" \
    > /etc/apt/sources.list.d/mongodb-org-$MONGO_SERIES.list
  apt-get update -qq
  apt-get install -y -qq mongodb-org >/dev/null
fi

# Install our config, but only enable auth once the users actually exist —
# otherwise mongo-bootstrap.sh would be locked out of the database it needs to
# create them in. install.sh can therefore run before or after bootstrapping.
#
# The test is a file check, deliberately: asking mongod whether it has users
# requires mongod to be running, and the first time through it isn't. That
# version of this check silently took the wrong branch and enabled auth on an
# empty database.
if grep -q '^MONGODB_URI=' "$ETC_DIR/env" 2>/dev/null; then
  install -m 0644 "$HERE/mongod.conf" /etc/mongod.conf
else
  echo "    not bootstrapped yet — leaving auth disabled until mongo-bootstrap.sh runs"
  sed 's/^  authorization: enabled$/  authorization: disabled/' "$HERE/mongod.conf" > /etc/mongod.conf
fi
systemctl enable mongod >/dev/null

# Deliberately tolerant: mongod cannot start on Linux kernels 6.19–7.0.13 (a
# TCMalloc/rseq bug, SERVER-121912) and refuses rather than crashing 30s in.
# LXC guests share the Proxmox host kernel, so that is a host-level fix (7.0.14+)
# and not something this script can do anything about. Say so plainly and carry
# on installing the rest, rather than aborting the whole bootstrap over it.
if ! systemctl restart mongod 2>/dev/null; then
  echo "    !! mongod failed to start. If this host's kernel ($(uname -r)) is between"
  echo "       6.19 and 7.0.13, that is SERVER-121912 — upgrade the Proxmox host"
  echo "       kernel to 7.0.14+ and re-run. Continuing with the rest of the install."
fi

echo "==> cloudflared"
if ! command -v cloudflared >/dev/null 2>&1; then
  curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg \
    -o /usr/share/keyrings/cloudflare-main.gpg
  echo "deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared noble main" \
    > /etc/apt/sources.list.d/cloudflared.list
  apt-get update -qq
  apt-get install -y -qq cloudflared >/dev/null
fi

echo "==> Host scripts, units and log rotation"
install -m 0750 "$HERE/backup.sh" "$APP_DIR/bin/backup.sh"
install -m 0644 "$HERE/thegathering.service"        /etc/systemd/system/thegathering.service
install -m 0644 "$HERE/thegathering-backup.service" /etc/systemd/system/thegathering-backup.service
install -m 0644 "$HERE/thegathering-backup.timer"   /etc/systemd/system/thegathering-backup.timer
install -m 0644 "$HERE/logrotate.thegathering"      /etc/logrotate.d/thegathering
systemctl daemon-reload
systemctl enable thegathering-backup.timer >/dev/null
systemctl start thegathering-backup.timer

# thegathering.service is enabled but NOT started here: there is no release to
# run until deploy.sh has shipped one, and a failed start would only be noise.
systemctl enable thegathering >/dev/null

echo
echo "==> Host ready."
echo "    Next: mongo-bootstrap.sh (creates the DB users, writes MONGODB_URI)"
echo "          then ./deploy.sh from the dev box to ship a release."
