#!/usr/bin/env bash
#
# Wire this host to a Cloudflare Tunnel connector.
#
#     ./cloudflared-setup.sh <connector-token>
#     printf '%s' "$TOKEN" | ./cloudflared-setup.sh      # preferred
#
# Prefer stdin. The token is the whole credential — anyone holding it can serve
# traffic for the hostname — and an argument is visible in `ps` to every user on
# the box for as long as the script runs.
#
# The tunnel itself is created in the Cloudflare dashboard, with a public
# hostname pointing at http://127.0.0.1:3002. This only installs the connector
# that dials out to it.
#
# Ingress is a tunnel rather than a reverse proxy on purpose: cloudflared makes
# an OUTBOUND connection, so nothing has to be forwarded inbound and no
# certificate is managed here. That is what makes an egress-only VLAN workable.
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root on the target host." >&2
  exit 1
fi

TOKEN=${1:-}
if [ -z "$TOKEN" ] && [ ! -t 0 ]; then
  read -r TOKEN || true
fi
if [ -z "$TOKEN" ]; then
  echo "Usage: $0 <connector-token>   (or pipe the token on stdin)" >&2
  echo "Create the tunnel in the Cloudflare dashboard and copy its token." >&2
  exit 1
fi

command -v cloudflared >/dev/null 2>&1 || {
  echo "cloudflared is not installed — run install.sh first." >&2
  exit 1
}

CRED=/etc/cloudflared/token

echo "==> Storing the connector token"
# The token is a credential. It goes in a root-only file rather than the unit's
# ExecStart, where it would be visible in `systemctl status`, `ps`, and the
# journal to anyone who can read them.
mkdir -p "$(dirname "$CRED")"
umask 077
printf 'TUNNEL_TOKEN=%s\n' "$TOKEN" > "$CRED"
chown root:root "$CRED"
chmod 0600 "$CRED"

echo "==> Installing the service"
cat > /etc/systemd/system/cloudflared.service <<'UNIT'
[Unit]
Description=Cloudflare Tunnel connector
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
EnvironmentFile=/etc/cloudflared/token
ExecStart=/usr/bin/cloudflared --no-autoupdate tunnel run
Restart=on-failure
RestartSec=5s
User=root
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable cloudflared >/dev/null
systemctl restart cloudflared

echo "==> Waiting for the connector to register"
for _ in $(seq 1 15); do
  if journalctl -u cloudflared --since "-1 min" --no-pager 2>/dev/null \
     | grep -q "Registered tunnel connection"; then
    echo "==> Connector registered."
    exit 0
  fi
  sleep 2
done

echo "!! No 'Registered tunnel connection' yet. Check: journalctl -u cloudflared -n 30" >&2
exit 1
