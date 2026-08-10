# e2e — post-deploy checks against a live instance

Playwright scripts run **by hand after a deploy**, against a running deployment
— not part of `backend-ci.yml` / `frontend-ci.yml`, and not a merge gate. The
unit suites (`npm run test:unit`, `go test`) cover logic; these cover the thing
unit tests structurally cannot: that the built dist actually shipped and behaves
in a real browser.

They are **assert-only**. Nothing here creates, edits, or saves anything — the
one case that opens an editor cancels out of it. Keep it that way: these run
against production data, with a real member's session.

## Setup

```bash
cd e2e
npm install
npx playwright install chromium
```

## Logging in

Every `verify_*` / `smoke_*` script reuses a session from `prod_state.json`,
produced by `prod_login.js`. Login is email-OTP-only, and the mail round trip
isn't scriptable, so the code is read from the deployment's Mongo over SSH —
hence an SSH host is required. It is not hardcoded (this repo is public):

```bash
TIMEFUL_VM=user@host node prod_login.js someone@example.com
```

| Env var | Default | Meaning |
| --- | --- | --- |
| `TIMEFUL_VM` | *(required for login)* | SSH target running the deployment. Must be a **root** login — the env file below is `0600` root-owned. |
| `TIMEFUL_ENV_FILE` | `/etc/thegathering/env` | Where `MONGODB_URI` is read from on that host. Mongo requires auth, and sourcing the URI there beats threading a password through argv — it would land in this box's shell history and the remote's process list. |
| `TIMEFUL_BASE` | `https://gathering.sirthomasfoolery.com` | Instance under test |
| `TIMEFUL_EMAIL` | — | Alternative to passing the email as argv |

`prod_state.json` holds a live session cookie and is gitignored. So are the
screenshots the scripts drop here.

## The checks

`smoke_prod.js` is the broad one — `/home`, `/settings`, `/members` and `/new`
render, with no console errors, no failed requests and no 5xx. It exits non-zero
on failure:

```bash
node smoke_prod.js
```

Beside it are three feature-specific checks, committed 2026-08-05 with the
features they cover: `verify_f19f20_prod.js` (My Lists / My Notes),
`verify_f22_prod.js` (the Settle Up ledger) and `verify_notes_quiet_prod.js`
(that the My Notes autosave stops firing when idle).

> **Unresolved policy question.** This file used to state that only the broad
> check is tracked and that fix-specific ones are throwaway — "write them when a
> fix needs verifying, run them, don't commit them" — because checks pinned to
> one past fix pass forever and then quietly rot. The three above were committed
> anyway. Either they earn their keep as standing checks or they should go; until
> someone decides, don't read either the old rule or these three files as
> settled precedent.

## Two gotchas, for when you write a throwaway one

Both cost real time and neither is obvious from a failing run:

- **Vuetify snackbars linger.** The wrapper stays mounted after it hides, so
  `allInnerTexts()` returns the *previous* case's message — a passing case reads
  as a failure. Filter to what is actually rendered:

  ```js
  const visibleSnackbarText = await page.evaluate(() =>
    Array.from(document.querySelectorAll(".v-snack__wrapper"))
      .filter((el) => {
        const s = getComputedStyle(el);
        return (
          el.offsetParent !== null &&
          s.visibility !== "hidden" &&
          s.display !== "none" &&
          parseFloat(s.opacity) > 0.1
        );
      })
      .map((el) => el.innerText)
      .join(" ")
  );
  ```

- **Scope dialog buttons.** A bare `getByRole("button", {name: "Cancel"})`
  matches buttons elsewhere on the page and hangs until the timeout. Scope it:
  `page.locator(".v-dialog--active").getByRole("button", {name: "Cancel"})`.
