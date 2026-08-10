# Timeful Deployment Guide

Production runs on **stf-thegathering** (192.168.24.56) in the isolated "Sir Tom"
VLAN, where hosting The Gathering is the machine's only job.

There is no Docker on that host, and there should never be. Three things run
under systemd — `mongod`, one static Go binary, and `cloudflared` — and the
build happens elsewhere.

> **Historical note:** this file used to describe a Caddy reverse proxy. That was
> never the actual setup; ingress has always been a Cloudflare Tunnel, and Caddy
> sat inactive on the old host. `Caddyfile.example` has been removed.

## Architecture

```
dev box                                  stf-thegathering (192.168.24.56)
  go build  ─┐                             /opt/thegathering/
  npm build ─┼─ rsync over SSH ──────────►   releases/<sha>/{server,dist/}
             │                               current -> releases/<sha>
  deploy.sh ─┘                               logs/      the app's cwd
                                             backups/   nightly dumps
                                           /etc/thegathering/env  (0600 root)

                                           systemd units:
                                             mongod                127.0.0.1:27017, auth on
                                             thegathering          127.0.0.1:3002
                                             cloudflared           outbound to Cloudflare
                                             thegathering-backup.timer
```

Ingress is a **Cloudflare Tunnel**: `cloudflared` dials out to Cloudflare, so
nothing needs to be forwarded inbound. This is what makes an egress-only VLAN
workable — the host can reach the internet, and nothing on the LAN can reach it.

## Deploying

From the **dev box** (not the server), in the repo root:

```bash
./deploy.sh
```

It refuses to run on a dirty tree or a checkout behind `origin/main`, runs the
tests, builds a static binary and the frontend bundle, rsyncs both to
`releases/<sha>`, flips the `current` symlink, restarts the service, and polls
`/api/health`. If health doesn't come good it re-points the symlink at the
previous release and restarts — leaving the bad release on disk to inspect.

`DEPLOY_HOST=...` targets another host; `SKIP_TESTS=1` skips the test run.

Rolling back by hand is a symlink flip:

```bash
ssh root@192.168.24.56 'ln -sfn /opt/thegathering/releases/<sha> /opt/thegathering/current.new \
  && mv -Tf /opt/thegathering/current.new /opt/thegathering/current \
  && systemctl restart thegathering'
```

### Why builds happen on the dev box

Production carries no toolchain — no Go, no Node, no Docker. That keeps
`node_modules` and build caches off the internet-facing host (a Docker build
cache once filled the old VM's disk and took the site down), makes rollback a
symlink flip rather than a rebuild, and means a failed build never reaches
production.

## Building a host from scratch

```bash
rsync -a deploy/ root@<host>:/tmp/deploy/
ssh root@<host> /tmp/deploy/install.sh        # packages, user, dirs, units, mongod, cloudflared
ssh root@<host> /tmp/deploy/mongo-bootstrap.sh # DB users + MONGODB_URI; ONCE, records a password
# copy secrets into /etc/thegathering/env (see below), then from the dev box:
./deploy.sh
```

`install.sh` is idempotent — re-run it after any change under `deploy/`.
`mongo-bootstrap.sh` is not, by design: it mints credentials, detects that it
has already run, and refuses to rotate a password by accident.

### Host requirements

- Ubuntu 24.04 (noble).
- **Linux kernel 7.0.14 or newer**, or older than 6.19. MongoDB 8.0+ crashes on
  kernels 6.19–7.0.13 (a TCMalloc/rseq bug, SERVER-121912) and refuses to start
  rather than crash 30 seconds in. On an LXC guest this is the *Proxmox host's*
  kernel — the guest cannot fix it.
- **MongoDB 8.2 or newer.** The kernel-side fix in 7.0.14 is only half of it:
  8.0.x still refuses on *any* kernel >= 6.19, because its startup guard is a
  version test written before the fix existed and never loosened. 8.0.28 is the
  last 8.0 release, so there is no 8.0 build that runs here.
- Egress to Cloudflare (`7844`), Gmail SMTP (`587`), and the apt/MongoDB repos.

## Services

| Unit                        | What it does                                    |
| --------------------------- | ----------------------------------------------- |
| `thegathering`              | The Go server on `127.0.0.1:3002`, as `gathering` |
| `mongod`                    | MongoDB 8.2, loopback only, SCRAM auth          |
| `cloudflared`               | Outbound tunnel; the public ingress             |
| `thegathering-backup.timer` | Nightly `mongodump` at 03:15                    |

```bash
systemctl status thegathering
journalctl -u thegathering -f          # stdout half of the logs
tail -f /opt/thegathering/logs/logs.log # file half (logrotate'd daily, 14 kept)
curl -s localhost:3002/api/health      # {"status":"ok","mongo":"ok","version":"<sha>"}
```

`/api/health` returns **503** when Mongo is unreachable, which is what tells a
deploy to roll back rather than retry. Note that on a cold start the server does
not open its listener until `db.Init` finishes — with Mongo down that means
roughly three minutes of index-creation timeouts before it answers at all.

## Backups

The old host had no backup cron; dumps were taken when someone remembered.

- **On the server:** `thegathering-backup.timer` runs `mongodump` nightly into
  `/opt/thegathering/backups`, keeping 14 days. It refuses to leave behind a
  dump under 1 KB, because an empty-but-successful dump is the failure that
  hides best.
- **On the dev box:** `deploy/pull-backups.sh` pulls them into
  `/root/backups/timeful/`. The direction matters — the VLAN is egress-only, so
  the server *cannot* push; the dev box must pull. Wired into root's crontab at
  04:30, after the host's 03:15 dump:

  ```
  30 4 * * * /root/projects/timeful.app/deploy/pull-backups.sh >> /var/log/timeful-backup-pull.log 2>&1
  ```

  It exits non-zero if the newest archive is over 48h old or implausibly small,
  so a stopped timer surfaces in that log rather than during a restore.

Restore into the live database — destructive, and what you'd run in a real
recovery:

```bash
mongorestore --uri="$MONGODB_URI" --archive=<file>.archive.gz --gzip --drop
```

### Testing a restore without touching prod

A backup nobody has restored is a guess. Restore into a throwaway container
instead, and diff it against the real thing:

```bash
docker run -d --name restore-test mongo:8.2          # 8.2, not 8.0 — see below
docker cp /root/backups/timeful/<file>.archive.gz restore-test:/tmp/b.gz
docker exec restore-test mongorestore --archive=/tmp/b.gz --gzip

# Compare CONTENT, not just counts. Both sides are 8.2, so dbHash is comparable.
H='const r=db.runCommand({dbHash:1}); Object.keys(r.collections).sort().forEach(c=>print(c+" "+r.collections[c]));'
docker exec restore-test mongosh --quiet schej-it --eval "$H"
ssh 192.168.24.56 "mongosh --quiet '<admin-uri>' --eval '$H'"

docker rm -f restore-test    # it holds every member's data — don't leave it up
```

Matching collection counts prove very little; matching `dbHash` per collection
proves every document survived. Use `mongo:8.2` and not `8.0`: containers share
the host kernel, so an 8.0 image hits the same SERVER-121912 refusal the host
does.

**Last verified: 2026-08-05** — all 11 collections and all 15 indexes restored
from the nightly archive, every content hash identical to production.

> **That run is now short of the database.** My Lists, My Notes and Settle Up
> landed the same day and added four collections (`personalLists`,
> `personalNotes`, `expenses`, `expenseReceipts`) and five named indexes, so the
> schema is **15 collections / 9 named indexes** and the verification above
> covered neither the new collections nor the expense receipts — which are the
> largest documents in the database, being embedded images. Worth re-running
> before trusting a restore.

## Configuration

Secrets live in `/etc/thegathering/env` (root-owned, `0600`) and are read by the
systemd unit via `EnvironmentFile`. This is not in git and never should be.

| Variable                                    | Notes                                                                                         |
| ------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `MONGODB_URI`                               | Written by `mongo-bootstrap.sh`. Moving Mongo to another host is this one line plus a restart. |
| `SESSION_SECRET`                            | ≥32 chars. The server panics at startup without it.                                            |
| `ENCRYPTION_KEY`                            | Exactly 16/24/32 chars — raw AES key bytes. Use `openssl rand -hex 16`, **not** `-base64 32` (44 chars, rejected). |
| `CLIENT_ID` / `CLIENT_SECRET`               | Google OAuth. Redirect URI: `https://<domain>/api/auth/callback`.                              |
| `INVITE_ONLY_ENFORCED`                      | The allowlist gate fails closed on this. Dropping it changes who can get in.                   |
| `SCHEJ_EMAIL_ADDRESS` / `GMAIL_APP_PASSWORD`| Gmail SMTP, the only mail transport. Without it nothing is sent — OTP codes included, so nobody can sign in. |
| `CORS_ORIGINS`                              | Same-origin in production, so rarely needed. `http://localhost:8080` for local dev.            |
| `FRONTEND_DIST`                             | `/opt/thegathering/current/dist` — absolute, so the release symlink can move underneath it.     |
| `TZ`                                        | Pinned to `UTC`. The old server ran in a container whose TZ was UTC even though its host was `America/Los_Angeles`; pinning stops the migration quietly changing how times are interpreted. |
| `MICROSOFT_CLIENT_ID` / `..._SECRET`        | Optional — Outlook calendars.                                                                  |

**Frontend build args** live in the **root `.env` on the dev box**, not on the
server: `CLIENT_ID`, `MICROSOFT_CLIENT_ID`, `GOOGLE_MAPS_API_KEY`. These are
baked into the bundle at build time, so changing one needs a redeploy — and an
empty `CLIENT_ID` ships a silently broken sign-in rather than failing. `deploy.sh`
asserts it is both set and present in the built bundle.

Only browser-side public values belong there. The OAuth client *secret* and the
Gmail password stay on the server.

## Cloudflare Tunnel

The tunnel is created in the Cloudflare dashboard; the host runs the connector.
The token is a credential — keep it in a root-owned `0600` file, not in a unit's
`ExecStart` where it shows up in `systemctl status` and every process listing.

Public hostname → `http://127.0.0.1:3002`.

Cutting a hostname over between hosts is a dashboard change, which also makes it
the rollback: point it back at the old tunnel.

## Google OAuth setup

1. [Google Cloud Console](https://console.cloud.google.com/) → project.
2. Enable: Google Calendar API, People API (Contacts), Admin SDK API (Directory).
3. OAuth 2.0 credentials, Web application.
4. Authorized redirect URIs:
   - `https://<domain>/api/auth/callback`
   - `http://localhost:3002/api/auth/callback` (development)

The redirect URI is domain-bound, so calendar-connect does not work on a
temporary hostname unless you add that hostname's callback too. OTP sign-in —
the primary login path — is unaffected.
