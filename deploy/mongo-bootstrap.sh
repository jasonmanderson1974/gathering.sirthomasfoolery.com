#!/usr/bin/env bash
#
# Create the MongoDB users, write MONGODB_URI, and turn authorization on.
#
# Split out from install.sh because it is the one step that mints a secret and
# is therefore not freely re-runnable: run it once, when the host is new.
# Re-running is safe (it detects existing users and stops), but it will not
# rotate a password — do that deliberately, by hand.
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root on the target host." >&2
  exit 1
fi

ENV_FILE=/etc/thegathering/env
APP_DB=schej-it
APP_USER=gathering

# Check the config before touching the database. Once authorization is on, an
# anonymous connection can't answer "are there users?" either way — so asking
# Mongo first would mint two passwords and only then discover it was too late.
if grep -q '^  authorization: enabled$' /etc/mongod.conf 2>/dev/null; then
  echo "Authorization is already enabled in /etc/mongod.conf — this host is bootstrapped."
  echo "To rotate the app password, do it deliberately: db.changeUserPassword(), then update $ENV_FILE."
  exit 0
fi

if mongosh --quiet "mongodb://127.0.0.1:27017/admin" \
     --eval 'db.system.users.countDocuments({}) > 0' 2>/dev/null | grep -q true; then
  echo "Mongo already has users but authorization is off — finish that by hand rather than guessing." >&2
  exit 1
fi

# Hex, not base64: the password goes into a URI, and hex needs no
# percent-encoding. A password that has to be escaped is a password that will
# one day be escaped wrongly.
ADMIN_PW=$(openssl rand -hex 24)
APP_PW=$(openssl rand -hex 24)

echo "==> Creating users"
mongosh --quiet "mongodb://127.0.0.1:27017/admin" --eval "
  db.createUser({
    user: 'admin',
    pwd: '$ADMIN_PW',
    roles: [{role: 'userAdminAnyDatabase', db: 'admin'}, {role: 'readWriteAnyDatabase', db: 'admin'}]
  });
  db.getSiblingDB('$APP_DB').createUser({
    user: '$APP_USER',
    pwd: '$APP_PW',
    // readWrite on its own database and nothing else. The app has no business
    // reading admin, and no business anywhere outside $APP_DB.
    roles: [{role: 'readWrite', db: '$APP_DB'}]
  });
" >/dev/null

echo "==> Enabling authorization"
sed -i 's/^  authorization: disabled$/  authorization: enabled/' /etc/mongod.conf
grep -q '^  authorization: enabled$' /etc/mongod.conf || {
  echo "authorization is not enabled in /etc/mongod.conf — check the file by hand." >&2
  exit 1
}
systemctl restart mongod
sleep 3

APP_URI="mongodb://$APP_USER:$APP_PW@127.0.0.1:27017/$APP_DB?authSource=$APP_DB"

echo "==> Verifying"
# Prove auth is actually on: an anonymous connection must now be refused. If
# this still succeeds, the restart didn't take the new config and the database
# is open.
if mongosh --quiet "mongodb://127.0.0.1:27017/$APP_DB" --eval 'db.getCollectionNames()' >/dev/null 2>&1; then
  echo "!! An unauthenticated connection still works — authorization is NOT in force." >&2
  exit 1
fi
mongosh --quiet "$APP_URI" --eval 'db.runCommand({ping: 1})' >/dev/null

echo "==> Writing $ENV_FILE"
mkdir -p "$(dirname "$ENV_FILE")"
touch "$ENV_FILE"
chown root:root "$ENV_FILE"
chmod 0600 "$ENV_FILE"
if grep -q '^MONGODB_URI=' "$ENV_FILE"; then
  sed -i "s|^MONGODB_URI=.*|MONGODB_URI=$APP_URI|" "$ENV_FILE"
else
  echo "MONGODB_URI=$APP_URI" >> "$ENV_FILE"
fi

echo
echo "==> Mongo ready, auth in force, MONGODB_URI written."
echo "    The admin password is NOT stored anywhere. Record it now or lose it:"
echo
echo "      admin / $ADMIN_PW"
echo
