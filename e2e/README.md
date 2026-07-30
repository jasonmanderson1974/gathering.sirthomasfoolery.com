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

## The checks

| Script | Covers |
| --- | --- |
| `smoke_prod.js` | `/home`, `/settings`, `/members`, `/new` render with no console errors, no failed requests, no 5xx |
| `verify_h5_prod.js` | H5 (`3aa2478`) — `/api/user/calendars` still returns a per-account map, and no account's error is the swallowed-unmarshal string H5 fixed |
| `verify_h9_prod.js` | H9 (`d99d18e`) — an avatar source under 256px on its shortest side is refused at pick time with its real size named; 256x256 still opens the cropper |

Each exits non-zero on failure, so they chain:

```bash
node smoke_prod.js && node verify_h5_prod.js && node verify_h9_prod.js
```

## Two gotchas worth keeping

- **Vuetify snackbars linger.** The wrapper stays mounted after it hides, so
  `allInnerTexts()` returns the *previous* case's message and a passing case
  reads as a failure. `verify_h9_prod.js` filters to visibly-rendered snackbars;
  copy that helper rather than re-deriving it.
- **Scope dialog buttons.** A bare `getByRole("button", {name: "Cancel"})`
  matches buttons elsewhere on the page and hangs. Scope to `.v-dialog--active`.

## Regression-specific scripts

`verify_h5` / `verify_h9` are pinned to specific fixes. They're kept because
they still pass and cost nothing to run, but they aren't a general suite — when
a check stops describing behaviour anyone relies on, delete it rather than
letting it rot.
