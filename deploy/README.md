# Production host configuration

Everything the production host is, kept in git so the machine is reproducible
rather than remembered. `DEPLOYMENT.md` at the repo root is the guide; this is
the reference for what is in here and why.

| File | Goes to | Purpose |
| --- | --- | --- |
| `install.sh` | run on host | Idempotent bootstrap: packages, user, dirs, mongod, cloudflared, units |
| `mongo-bootstrap.sh` | run on host | Creates DB users, writes `MONGODB_URI`, turns auth on. Once. |
| `mongod.conf` | `/etc/mongod.conf` | Loopback bind, SCRAM auth, pinned WiredTiger cache |
| `thegathering.service` | `/etc/systemd/system/` | The app |
| `thegathering-backup.{service,timer}` | `/etc/systemd/system/` | Nightly dump at 03:15 |
| `backup.sh` | `/opt/thegathering/bin/` | The dump itself |
| `logrotate.thegathering` | `/etc/logrotate.d/thegathering` | Rotates `logs.log` |
| `pull-backups.sh` | **runs on the dev box** | Pulls backups off the host |

## Things this host teaches you the hard way

**MongoDB 8.0+ will not start on Linux kernels 6.19–7.0.13.** A vendored
TCMalloc violates the kernel's rseq ABI and the process SIGSEGVs about 30
seconds in, so MongoDB detects those kernels and refuses to start at all
(SERVER-121912). Kernel 7.0.14+ fixes it. This host is an **LXC guest**, so the
kernel is the *Proxmox host's* — nothing inside the guest can fix it. If mongod
won't start, check `uname -r` first.

**...but upgrading the kernel is only half the fix — you also need 8.2+.**
MongoDB 8.0.x checks the kernel version, not whether the bug is actually
present, and every 8.0 build refuses on anything >= 6.19 however new. That guard
predates the 7.0.14 fix and was never loosened, and 8.0.28 is the last 8.0
release, so no 8.0 build will ever run on this host. 8.2 knows about 7.0.14+.
Seen for real here: on kernel 7.0.14-8-pve, 8.0.28 refused and 8.2.12 started.

**MongoDB 7.0 was never packaged for Ubuntu 24.04 (noble).** 8.0 is the first
line that supports it. That is why this doesn't simply match the old host's
version.

**MongoDB never published a GPG key for the 8.2 series.**
`pgp.mongodb.com/server-8.2.asc` 404s, while `server-8.0.asc` resolves — the
same key (`41DE058A4E7DCA05`) signs both repos. `install.sh` therefore fetches
the 8.0 URL and stores it as `mongodb-server.gpg`, without a series in the name.

**The app's working directory is the log directory, not the release
directory.** `main.go` opens `logs.log` relative to cwd and `log.Fatal`s if it
can't. Pointing cwd at the release would put logs inside `releases/<sha>`, where
pruning old releases would quietly delete them.

**The server does not listen until `db.Init` finishes.** It opens 15 collections,
creates nine named indexes and runs the token-encryption sweep first, each
blocking on a 30s server-selection timeout when Mongo is unreachable — so with
the database down the app takes ~3 minutes to answer anything at all, then
reports `503`. This is why the deploy health gate waits 90s rather than 40s. The
count grows as features land (it was four indexes before My Lists / My Notes /
Settle Up), so treat it as "several, each with its own timeout".

**Backups are pulled, never pushed.** The Sir Tom VLAN is egress-only: this host
reaches the internet and nothing on the LAN. `backup.sh` writes locally;
`pull-backups.sh` runs on the dev box and fetches. Anything written to push from
the host side fails in the way that is only discovered during a restore.

**Ingress is a Cloudflare Tunnel, not a reverse proxy on the host.** `cloudflared`
dials out, which is what makes an egress-only VLAN workable. Nothing is
forwarded inbound and no certificate is managed here.

## Rebuilding the host

```bash
rsync -a deploy/ root@<host>:/tmp/deploy/
ssh root@<host> /tmp/deploy/install.sh
ssh root@<host> /tmp/deploy/mongo-bootstrap.sh   # records an admin password — keep it
# copy secrets into /etc/thegathering/env, then from the dev box:
./deploy.sh
```

`install.sh` is safe to re-run after any change here. `mongo-bootstrap.sh` is
not idempotent by design — it mints credentials, and detects and refuses rather
than rotating a password by accident.
