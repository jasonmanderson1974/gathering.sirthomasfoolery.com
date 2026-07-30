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
| `TIMEFUL_VM` | *(required for login)* | SSH target running the deployment |
| `TIMEFUL_VM_DIR` | `~/docker/timeful.app` | Repo path on that host |
| `TIMEFUL_BASE` | `https://gathering.sirthomasfoolery.com` | Instance under test |
| `TIMEFUL_EMAIL` | — | Alternative to passing the email as argv |

`prod_state.json` holds a live session cookie and is gitignored. So are the
screenshots the scripts drop here.

## The check

`smoke_prod.js` — `/home`, `/settings`, `/members` and `/new` render, with no
console errors, no failed requests and no 5xx. It exits non-zero on failure:

```bash
node smoke_prod.js
```

That's deliberately the whole of it. Checks pinned to one past fix pass forever
and then quietly rot; this repo keeps the broad post-deploy check tracked and
treats fix-specific ones as throwaway — write them when a fix needs verifying,
run them, don't commit them.

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
