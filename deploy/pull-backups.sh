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
# recent and actually full of member data, so a silently-empty dump or a stopped
# timer surfaces here rather than during a restore.
#
# This deliberately does NOT key on the archive's total byte count, which used to
# be the whole check (`SIZE -ge 1024`). Total bytes stopped tracking member data
# the moment expense receipts shipped: they are JPEGs stored as Binary, they do
# not shrink under gzip the way the BSON around them does, and one 275KB photo
# uploaded on 2026-08-07 took the nightly archive from 35KB to 318KB overnight.
# 78% of the archive was then a single image. Two failure modes follow, and a
# byte threshold gets both backwards:
#
#   - delete that one expense and the archive drops back to ~36KB, which any
#     meaningful byte floor would report as a catastrophe. It isn't one.
#   - with receipts present, a byte floor tuned to ~300KB would pass an archive
#     that had lost every user, event and response, because the photo alone
#     clears it.
#
# So assert on structure and content instead. All three checks below are blob
# independent: adding or removing receipts moves none of them.
AGE_HOURS=$(( ( $(date +%s) - $(stat -c %Y "$LATEST") ) / 3600 ))
SIZE=$(stat -c %s "$LATEST")

# 1. Integrity. Decompressing the whole stream is what the old byte floor was
#    really groping at — a truncated or half-written archive fails here, at any
#    size. Everything downstream also reads the stream, so this runs first.
gzip -t "$LATEST" 2>/dev/null || {
  echo "!! Latest backup is corrupt or truncated (gzip -t failed)." >&2; exit 1; }

# 2. Every collection that must survive a restore is present in the archive.
#    Read from the collection metadata mongodump writes ahead of each block, via
#    the JSON `collectionName` key rather than the raw BSON field: the binary
#    form needs a length prefix matched with `.` , and PCRE's `.` does not match
#    a newline byte, so names whose length byte happens to be 0x0A go missing —
#    which silently dropped `allowlist` and `chronicle` from a first version of
#    this check. A stray match from document content could only ever ADD a name,
#    never hide a missing one, so presence-testing against this list is safe.
#
#    otpCodes is intentionally absent: it is ephemeral and legitimately empty.
#    avatars/expenseReceipts are absent too — they are the blob collections, and
#    keeping them out of the assertions is the point of this rewrite.
REQUIRED_COLLECTIONS="users events eventResponses expenses comments folders folderEvents allowlist chronicle"
FOUND=$( { gunzip -c "$LATEST" | grep -aoP '"collectionName":"\K[^"]+' || true; } | sort -u )
MISSING=""
for c in $REQUIRED_COLLECTIONS; do
  printf '%s\n' "$FOUND" | grep -qxF "$c" || MISSING="$MISSING $c"
done

# 3. How much member data is in there. Counts BSON ObjectId field markers
#    (\x07 _id), which is a structural marker count and NOT a document count: it
#    runs well above the true total (244 vs 176 on 2026-08-10) because embedded
#    subdocuments — expense splits, responses, list items — carry their own
#    ObjectIds. That is fine, because it is only ever compared against itself.
#    It is stable where bytes are not: across the receipt upload it moved
#    236 -> 241 -> 244 while the byte count went 35,731 -> 318,226.
MARKERS=$( { gunzip -c "$LATEST" | grep -aoP '\x07_id\x00' || true; } | wc -l )

# Compare against the previous archive, so a collapse is caught even if it stays
# above the absolute floor. A modest decline is normal — events and comments do
# get deleted — so only a sharp drop fails.
PREV=$(find "$LOCAL_DIR" -name 'schej-it-*.archive.gz' -printf '%T@ %p\n' 2>/dev/null \
       | sort -rn | sed -n 2p | cut -d' ' -f2-)
PREV_MARKERS=0
if [ -n "$PREV" ] && gzip -t "$PREV" 2>/dev/null; then
  PREV_MARKERS=$( { gunzip -c "$PREV" | grep -aoP '\x07_id\x00' || true; } | wc -l )
fi

MIN_MARKERS=${MIN_MARKERS:-50}            # backstop against an all-but-empty dump
MIN_RETAINED_PCT=${MIN_RETAINED_PCT:-80}  # vs. the previous archive

echo "Latest: $(basename "$LATEST") — ${SIZE} bytes, ${AGE_HOURS}h old, \
$(printf '%s\n' "$FOUND" | grep -c .) collections, ${MARKERS} id markers (prev ${PREV_MARKERS})"

[ -z "$MISSING" ] || { echo "!! Latest backup is missing collections:$MISSING" >&2; exit 1; }
[ "$MARKERS" -ge "$MIN_MARKERS" ] || {
  echo "!! Latest backup holds only ${MARKERS} id markers (floor ${MIN_MARKERS}) — near-empty dump." >&2
  exit 1; }
if [ "$PREV_MARKERS" -gt 0 ] \
   && [ $(( MARKERS * 100 )) -lt $(( PREV_MARKERS * MIN_RETAINED_PCT )) ]; then
  echo "!! Latest backup dropped to ${MARKERS} id markers from ${PREV_MARKERS} — possible data loss." >&2
  exit 1
fi
[ "$AGE_HOURS" -le 48 ] || { echo "!! Latest backup is over 48h old — is the timer running?" >&2; exit 1; }
