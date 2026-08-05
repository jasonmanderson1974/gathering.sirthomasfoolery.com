#!/usr/bin/env bash
#
# Pull nightly backups off the production host onto the dev box.
#
# Runs HERE, not there, and that is the whole point. The Sir Tom VLAN is
# egress-only: stf-thegathering can reach the internet but nothing on the LAN,
# so it cannot push a backup anywhere. Anything written to push from that side
# fails in the way that is only discovered when a restore is needed.
#
# Suggested cron on the dev box (after the 03:15 dump on the host):
#   30 4 * * *  /root/projects/timeful.app/deploy/pull-backups.sh >> /var/log/timeful-backup-pull.log 2>&1
set -euo pipefail

DEPLOY_HOST=${DEPLOY_HOST:-root@192.168.24.56}
REMOTE_DIR=/opt/thegathering/backups
LOCAL_DIR=${LOCAL_DIR:-/root/backups/timeful}

mkdir -p "$LOCAL_DIR"

# --ignore-existing rather than a plain sync: these archives are immutable once
# written, so re-transferring them is waste, and never deleting locally means
# the host's 14-day retention doesn't silently become the whole retention
# policy. Pruning here is a separate, deliberate decision.
rsync -a --ignore-existing \
  --include='schej-it-*.archive.gz' --exclude='*' \
  "$DEPLOY_HOST:$REMOTE_DIR/" "$LOCAL_DIR/"

# Lock the directory down AFTER rsync, not before. A trailing-slash source makes
# rsync -a apply the *remote* directory's own permissions to this one, so a
# chmod above here gets silently overwritten with the host's 0750 on every run
# — which was exactly what happened, unnoticed, until someone stat'd it. These
# archives are every member's data; 0700 is the point of keeping them here.
chmod 0700 "$LOCAL_DIR"
chmod 0600 "$LOCAL_DIR"/schej-it-*.archive.gz 2>/dev/null || true

LATEST=$(find "$LOCAL_DIR" -name 'schej-it-*.archive.gz' -printf '%T@ %p\n' 2>/dev/null \
         | sort -rn | head -1 | cut -d' ' -f2-)

if [ -z "$LATEST" ]; then
  echo "No backups pulled — check that the timer ran on $DEPLOY_HOST." >&2
  exit 1
fi

# A backup nobody has looked at is a guess. Assert the newest archive is both
# recent and non-trivial, so a silently-empty dump or a stopped timer surfaces
# here rather than during a restore.
AGE_HOURS=$(( ( $(date +%s) - $(stat -c %Y "$LATEST") ) / 3600 ))
SIZE=$(stat -c %s "$LATEST")

echo "Latest: $(basename "$LATEST") — ${SIZE} bytes, ${AGE_HOURS}h old"
[ "$SIZE" -ge 1024 ] || { echo "!! Latest backup is implausibly small." >&2; exit 1; }
[ "$AGE_HOURS" -le 48 ] || { echo "!! Latest backup is over 48h old — is the timer running?" >&2; exit 1; }
