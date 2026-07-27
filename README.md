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
> on the VM — manual and gate-kept), local dev (`compose.dev.yaml`), and testing/CI.
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

- **Invite-only access control** — email-OTP login, a member roll with roles (super-admin / admin / member / guest)
- **Confirmed gatherings** — lock in a time, with automated pre-gathering reminder emails
- **RSVP + plus-ones** — going / maybe / can't, live headcount + roster, spouse/guest counts
- **Universal "add to calendar"** `.ics` export for confirmed gatherings
- **Sign-up blocks** with capacity enforcement + waitlist
- **Per-gathering discussion threads**, venue / location, and a printable directory roster
- Whole-app rebrand to The Fellowship's archaic gentleman's-club voice

## Plugin API

The frontend exposes a `get-slots` / `set-slots` `postMessage` API for browser plugins.
See the [Plugin API Docs](./PLUGIN_API_README.md).

## Self-hosting

See the [Deployment Guide](./DEPLOYMENT.md) for Docker Compose setup.

## Credits & license

Derived from [Timeful / Schej.it](https://github.com/schej-it/timeful.app) by its original
authors. Licensed under **AGPL-3.0** — see the license badge above; this project retains the
same license.
