# Timeful / The Fellowship — Backlog 3 (active)

> Opened **2026-08-10**, at `be30197d`. `TODO.md` and `TODO2.md` are both **closed archives** as
> of this file — nothing in either needs updating again, and the three items still open in
> TODO2.md Part G are restated below as pointers rather than copied.
>
> **Status: nothing queued.** This is a blank slate awaiting the next round of work.
>
> Context unchanged: self-hosted, invite-only fork for a ~30–40 person club. Reliability and
> small-club utility over scale. All event access requires sign-in; roles are
> superAdmin > admin > member > guest. Production runs no Docker — `mongod`, one static Go
> binary and `cloudflared` under systemd on `stf-thegathering`; see `DEPLOYMENT.md`.

Priority legend: **P0** = do first · **P1** = high value · **P2** = moderate · **P3** = nice-to-have.
Effort: **S** ≈ <½ day · **M** ≈ 1–2 days · **L** ≈ 3+ days.

**New items start at `J`** (A–E are TODO.md, F–H are TODO2.md; `I` is skipped so it can't be read
as a `1`). Never reuse a retired ID — an old commit message or code comment citing `F22` or `E12`
must keep resolving to one thing.

---

## Archives — where a cited item ID lives

Older commits, code comments and `CLAUDE.md`/`DEVELOPMENT.md` all cite backlog IDs. The letter
tells you which file to read:

| IDs | File | Covered |
|---|---|---|
| `A*` `B*` `C*` `D*` `E*` | [`TODO.md`](TODO.md) — archived 2026-07-28 | The first full-codebase review (2026-07-22) and the 2026-07-27 re-review: the security/cleanup wave, headlined by **E3** (sign-in required for all event access) and the deletion sweep. |
| `F*` `G*` `H*` | [`TODO2.md`](TODO2.md) — archived 2026-08-10 | The feature track **F1–F22** (nicknames, avatars, mentions, Lists v1–v3, My Lists, My Notes, Settle Up), Part G carried forward from `TODO.md`, and Part H's 2026-07-28 findings. |

Both archives keep their full write-ups, including what each item's implementation *found* that
the plan didn't predict. Read them before re-opening anything — several entries record defects
that were described wrongly by the finding that raised them, and the corrections are in the
entries, not in the code.

## Inherited, still open — all `P3`, all parked by the user

Carried across as pointers only; the detail stays in `TODO2.md` and is still accurate. None of
these has a known failure mode in production. If one is picked up, give it a fresh `J*` ID here
and leave the old entry alone.

- **`TODO2.md` G2 — split `date_utils.js`** (`L`). The only part of G2 left; everything else
  (the eleven dead exports, `newEventFormMixin`, the `ScheduleOverlap` computed block) shipped
  2026-07-29/30. 946 lines / 32 exports. The entry's caveat stands: verify a split with the app
  running, not blind — the earlier passes found three live bugs precisely because they were
  exercised.
- **`TODO2.md` G3 — web push** (`M`). Deferred pending a value reassessment; reintroducing a
  service worker reverses a deliberate removal (`f857320`) and email reminders already cover iOS.
  One cheap loose end inside it, independent of the rest: `kill-sw.js` sits at the repo root, so
  it is **never actually served** and can't unregister anything — move it to `frontend/public/` or
  mark it documentation-only. `frontend/.eslintrc.cjs:11`'s `serviceworker: true` env is stale too.
- **`TODO2.md` G4 — rename the `schej-it` Mongo database** (`L`). A data migration (dump →
  restore under the new name → cutover in a deploy window), human-run. Zero user-facing benefit,
  which is why it's parked.

---

## PART J — (empty)

Nothing queued. New work goes here.

---

## Workflow rules

Unchanged from `CLAUDE.md` and the two archives — these are the durable part, and every one of
them exists because ignoring it cost a debugging session:

- **Sync before changes.** Two machines push to `main`; start with `git fetch origin` and
  `git pull --ff-only`.
- **Green commits to trunk.** CI is post-hoc, not a merge gate: run the frontend unit tests,
  eslint, the production build, the backend suite, `go vet` and golangci-lint locally first.
- **Deploys are human-run** from the box with SSH access to `stf-thegathering`, via `./deploy.sh`
  on the build box. `origin/main` is ahead of what's live until then, and that's expected.
- **Cold-load signed-out** after any router or auth change (the E3 outage lesson).
- **Rebuild the dev containers before trusting a harness run.** `compose.dev.yaml` bakes the
  frontend bundle and the Go binary into their images, so `docker compose restart` re-runs the
  *old* artifacts: `docker compose -f compose.dev.yaml up -d --build frontend server`. The server
  registers its static routes only at boot, so a `dist` swap needs the restart to see new hashed
  filenames.
- **Look at the page.** Lint, unit tests and the build have all been green over a bug that only
  appears in a browser — `v-show` beaten by Tailwind's `important: true`, a purged class name
  built from a template string, a fifth tab putting a phone into horizontal scroll. The
  `CLAUDE.md` frontend section lists the ones already paid for.
