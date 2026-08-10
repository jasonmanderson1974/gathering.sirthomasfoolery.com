<div align="center">

# The Fellowship · The Gathering

</div>
<div align="center">

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-orange.svg)](https://www.gnu.org/licenses/agpl-3.0)

</div>

A private, self-hosted group-scheduling app for **The Fellowship** — a small,
invite-only club. Members call a _Gathering_, cast their availability, and settle
upon the hour that suits the whole Order.

It is a hardened, rebranded derivative of [Timeful](https://github.com/schej-it/timeful.app)
(formerly Schej.it), an open-source availability/scheduling app. Built with
[Vue 2](https://github.com/vuejs/vue), [Go](https://github.com/golang/go),
[MongoDB](https://github.com/mongodb/mongo), and
[TailwindCSS](https://github.com/tailwindlabs/tailwindcss).

> **Working on this repo? Read [`DEVELOPMENT.md`](./DEVELOPMENT.md) first.** It covers the multi-machine
> workflow (always `git fetch` + sync to the latest `main` before making changes), deploys (`./deploy.sh`
> on the build box — manual and gate-kept), local dev (`compose.dev.yaml`), and testing/CI.
> **AI assistants:** the authoritative project + workflow instructions live in [`CLAUDE.md`](./CLAUDE.md)
> (auto-loaded by Claude Code); if your tool doesn't load it, read `CLAUDE.md` and `DEVELOPMENT.md` first.

## Features

Inherited from Timeful:

- See when everybody's availability overlaps
- Specify date + time ranges to meet between
- Google Calendar, Outlook, and Apple Calendar integration
- "Available" vs. "If needed" times
- Determine when a subset of people are available
- Schedule across time zones
- Duplicating polls, CSV export
- Only show responses to the event creator

Added in this build (for a ~40-person club):

- **Invite-only access control** — email-OTP login, a member roll with roles (super-admin / admin / member / guest). Every event page requires a session; there is no anonymous access.
- **Confirmed gatherings** — lock in a time, with automated pre-gathering reminder emails, and a repeat rule (weekly / fortnightly / monthly) that rolls the gathering forward on its own
- **RSVP + plus-ones** — going / maybe / can't, live headcount + roster, spouse/guest counts
- **Universal "add to calendar"** `.ics` export for confirmed gatherings
- **Shared lists** per gathering — nested three deep, checklists, drag to reorder within and across lists
- **Private per-gathering scratch space** — "My Lists" and a markdown "My Notes" document, visible only to you
- **Settle Up** — a shared expense ledger with receipts, even/by-amount/by-share splits and per-person balances
- **Per-gathering discussion threads** with @mentions (and mention emails), venue / location, and a printable directory roster
- **Nicknames and avatars**, resolved at read time so a rename updates past events too
- **The Chronicle** — every gathering that comes to pass is archived automatically, kept separately from the event
- Whole-app rebrand to The Fellowship's archaic gentleman's-club voice

## Plugin API

The frontend exposes a `get-slots` / `set-slots` `postMessage` API for browser plugins.
See the [Plugin API Docs](./PLUGIN_API_README.md).

## Self-hosting

See the [Deployment Guide](./DEPLOYMENT.md). Production runs no containers: MongoDB,
one static Go binary and a Cloudflare Tunnel connector, all under systemd. Docker is
used only for local development.

## Credits & license

Derived from [Timeful / Schej.it](https://github.com/schej-it/timeful.app) by its original
authors. Licensed under **AGPL-3.0** — see the license badge above; this project retains the
same license.
