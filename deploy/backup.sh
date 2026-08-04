#!/usr/bin/env bash
#
# Nightly MongoDB backup, run by thegathering-backup.timer.
#
# The old production host had no backup cron at all — dumps were taken by hand
# when someone remembered. This closes that.
#
# Note the direction of travel: this host cannot reach the LAN (the Sir Tom VLAN
# is egress-only), so it cannot push backups anywhere. It writes them locally and
# the dev box PULLS them. Anything here that tries to push will fail silently in
# the way that only gets discovered when a restore is needed.
set -euo pipefail

BACKUP_DIR=/opt/thegathering/backups
RETAIN_DAYS=14
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
ARCHIVE="$BACKUP_DIR/schej-it-$STAMP.archive.gz"

# MONGODB_URI carries the credentials; it is root-readable only, which is why
# this runs as root rather than as the app user.
# shellcheck disable=SC1091
set -a
source /etc/thegathering/env
set +a

if [ -z "${MONGODB_URI:-}" ]; then
  echo "MONGODB_URI is unset — refusing to take a backup that would silently be empty." >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"

# Write to a temp name and move into place only on success, so a partial dump
# from an interrupted run can never be mistaken for a good backup by the pull
# on the other end.
TMP="$ARCHIVE.partial"
mongodump --uri="$MONGODB_URI" --db=schej-it --archive="$TMP" --gzip --quiet
mv "$TMP" "$ARCHIVE"

# An empty-but-successful dump is the failure mode that hides best: it exits 0
# and leaves a file. A real dump of this database is comfortably over 1 KB.
SIZE=$(stat -c %s "$ARCHIVE")
if [ "$SIZE" -lt 1024 ]; then
  echo "Backup $ARCHIVE is only ${SIZE} bytes — that is not a real dump." >&2
  exit 1
fi

find "$BACKUP_DIR" -name 'schej-it-*.archive.gz' -mtime "+$RETAIN_DAYS" -delete
find "$BACKUP_DIR" -name '*.partial' -mtime +1 -delete

echo "Backed up to $ARCHIVE (${SIZE} bytes); retaining ${RETAIN_DAYS} days."
