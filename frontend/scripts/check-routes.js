#!/usr/bin/env node
/*
 * Every route, every band tab, signed in — does it render, and does it render
 * quietly?
 *
 * WHY THIS EXISTS: the unit suite cannot fail on any of this. All 23 test files
 * are pure JS extracted *out of* components — `vitest.config.mjs` sets
 * `environment: "node"`, nothing imports a `.vue` file, and `@vue/test-utils`
 * is not a dependency. The suite stays green through a total rendering failure.
 * Lint and the production build are no better: the repo has already shipped
 * `v-show` beaten by Tailwind's `important: true`, a purged class name built
 * from a template string, and a fifth band tab pushing a phone into horizontal
 * scroll — all three green everywhere except in a browser.
 *
 * It is written against the DOM rather than against Vue, so it is worth exactly
 * as much after the Vue 3 / Vuetify 3 migration as before it. That is the point:
 * it is the safety net the migration is carried out over, and the reason it
 * checks structure ("exactly one band panel is visible") rather than markup.
 *
 * Local sign-in needs SMTP (OTP) or Google OAuth, neither wired in dev, so mint
 * the cookie directly with `server/tools/mintsession` — see DEVELOPMENT.md. Use
 * a superAdmin: `/members` is gated on `canInvite`, and role-gated UI is only
 * exercised if the session has the role.
 *
 * Usage: node scripts/check-routes.js <baseUrl> <sessionCookie> <eventId>
 *                                     [--shots <dir>]
 *
 * `--shots` leaves a PNG of every page it visits in <dir> (TODO3 M1). A failing
 * run then hands over an image of the page that failed rather than only the name
 * of the assertion — which is the difference between "band tab Lists — exactly
 * one panel visible: FAIL" and seeing that two panels are stacked. For one page
 * in isolation use `scripts/shot.js`, which drives the same code.
 */
const path = require("path")
const {
  launch,
  evaluate,
  pageErrors,
  frameworkWarnings,
  screenshot,
  sleep,
} = require("./browser-check-lib")

const args = process.argv.slice(2)

// Pulled out before the positionals are read, so `--shots <dir>` can go
// anywhere on the line and the three required arguments keep their order.
let SHOTS = null
const shotsAt = args.indexOf("--shots")
if (shotsAt !== -1) {
  SHOTS = args[shotsAt + 1]
  if (!SHOTS) {
    console.error("--shots needs a directory")
    process.exit(2)
  }
  args.splice(shotsAt, 2)
}

const [rawBase, COOKIE, EVENT_ID] = args

if (!rawBase || !COOKIE || !EVENT_ID) {
  console.error(
    "usage: node scripts/check-routes.js <baseUrl> <sessionCookie> <eventId> [--shots <dir>]"
  )
  process.exit(2)
}
const BASE = rawBase.replace(/\/$/, "")
const HOST = new URL(BASE).hostname

const DESKTOP = { width: 1280, height: 900, mobile: false }
const PHONE = { width: 390, height: 844, mobile: true }

/* ---------- page-side expressions ---------- */

const hasText = (re) => `${re}.test(document.body.innerText)`

const buttonMatching = (re) =>
  `[...document.querySelectorAll('button')].some(b => ${re}.test(b.textContent.trim()))`

/** Clicks the first button/tab whose trimmed text matches, and says so. */
const clickButton = (re) => `(() => {
  const el = [...document.querySelectorAll('button')]
    .find(b => ${re}.test(b.textContent.trim()))
  if (!el) return 'NOT FOUND'
  el.click()
  return 'ok'
})()`

/**
 * How many band panels are actually visible on the event page.
 *
 * The band tabs toggle sibling divs with `v-show`, which is the exact construct
 * that Tailwind's `important: true` silently defeats when the toggled element
 * also carries a display utility (`tw-flex`/`tw-block`/`tw-grid` compile to
 * `display: … !important` and beat the inline `display: none`). Asserting
 * "exactly one" catches both directions: nothing rendering, and every panel
 * rendering at once with no error anywhere.
 */
const visibleBandPanels = `(() => {
  const btn = [...document.querySelectorAll('button')]
    .find(b => /^Discussion/.test(b.textContent.trim()))
  if (!btn) return 'NO TAB ROW'
  const row = btn.parentElement
  const band = row.parentElement
  return [...band.children]
    .filter((el) => el !== row)
    .filter((el) => el.offsetParent !== null && el.getBoundingClientRect().height > 0)
    .length
})()`

const noHorizontalScroll =
  "document.documentElement.scrollWidth <= window.innerWidth + 1"

/**
 * The MDI webfont actually arrived and painted.
 *
 * Every `mdi-*` name in the app is a glyph in one webfont; if that font fails,
 * all 69 of them render as blank squares with nothing logged anywhere — no
 * console error, no failed assertion, a build and a unit suite both green.
 * `document.fonts.check()` is the obvious API and the wrong one: with no
 * matching `@font-face` at all it reports *true* (system fonts are assumed
 * available), so it passes in exactly the case worth catching.
 *
 * `document.fonts.load()` is the one that works, because it ATTEMPTS the face
 * and reports the outcome: an empty array when nothing declares the family, a
 * rejection when the bytes are not a font — which is what the SPA fallback
 * serving `index.html` in place of a missing woff2 produces.
 *
 * It reads the state rather than inferring it, and that is the point (TODO3
 * M2). This assertion used to require a `resource` timing entry over 100KB, and
 * that inference has two ways of being wrong that have nothing to do with the
 * font: a webfont is only fetched when a glyph on the page needs it, and a
 * cached response that is revalidated (a 304, which is what a dev server sends
 * — the Go server sends `immutable` and is never asked) reports a zero body.
 * Both report a perfectly-painting font as missing, and the second one is
 * exactly what `--dev` walked into on its first run.
 */
const iconFontLoaded = `(async () => {
  const faces = [...document.fonts]
    .filter((f) => /Material Design Icons/.test(f.family))
  if (faces.length === 0 || faces.some((f) => f.status === 'error')) return false
  const loaded = await document.fonts.load("24px 'Material Design Icons'")
  return loaded.length > 0 && loaded.every((f) => f.status === 'loaded')
})()`

/**
 * ...and arrived from us, content-hashed, rather than from a CDN.
 *
 * L8: this used to be `@mdi/font@latest` on cdn.jsdelivr.net — an unpinned
 * third party that could change what renders here with no deploy on our side.
 * The hash in the filename is also what earns the font `immutable` caching from
 * the Go server (`contentHashedAsset`, main.go), so asserting on it keeps both
 * properties honest.
 *
 * Read off the `@font-face` rule rather than off resource timing, for the same
 * reason as above: the declaration is there on every page whether or not that
 * page happened to fetch anything. A cross-origin stylesheet throws on
 * `.cssRules`, and skipping it is right — it cannot be ours.
 */
const iconFontSelfHosted = `(() => {
  const srcs = []
  for (const sheet of document.styleSheets) {
    let rules
    try { rules = sheet.cssRules } catch { continue }
    for (const rule of rules) {
      if (rule instanceof CSSFontFaceRule &&
          /Material Design Icons/.test(rule.style.fontFamily)) {
        srcs.push(rule.style.src)
      }
    }
  }
  if (srcs.length === 0) return false
  const all = srcs.join(' ')
  const urls = [...all.matchAll(/url\\(["']?([^"')]+)["']?\\)/g)].map((m) => m[1])
  return urls.length > 0 &&
    urls.every((u) => new URL(u, location.href).origin === location.origin) &&
    /webfont\\.[0-9a-f]{8}\\.woff2/.test(all)
})()`

/* ---------- routes ---------- */

const routes = (eventId) => [
  {
    name: "landing",
    path: "/",
    assertions: [
      ["renders", "document.querySelectorAll('#app *').length > 80"],
      ["shows the club pitch", hasText("/The Gathering/")],
      ["offers the way in", buttonMatching("/to the club room/i")],
    ],
  },
  {
    name: "home",
    path: "/home",
    assertions: [
      ["renders", "document.querySelectorAll('#app *').length > 200"],
      ["offers gathering creation", buttonMatching("/call a gathering/i")],
      ["offers folder creation", buttonMatching("/new folder/i")],
      ["the icon webfont loaded", iconFontLoaded],
      ["the icon webfont is self-hosted", iconFontSelfHosted],
    ],
  },
  {
    name: "settings",
    path: "/settings",
    assertions: [
      ["renders", "document.querySelectorAll('#app *').length > 100"],
      ["offers avatar upload", buttonMatching("/add photo/i")],
      ["offers calendar linking", buttonMatching("/add calendar/i")],
      ["offers account deletion", buttonMatching("/delete account/i")],
    ],
  },
  {
    name: "admin (the roll)",
    path: "/members",
    assertions: [
      // Gated on `canInvite`; a session below member is redirected to /home,
      // which shows up here as the path assertion failing rather than this one.
      ["renders", "document.querySelectorAll('#app *').length > 150"],
      ["offers an invitation", buttonMatching("/extend invitation/i")],
    ],
  },
  {
    name: "fellowship",
    path: "/fellowship",
    assertions: [
      ["renders", "document.querySelectorAll('#app *').length > 80"],
      ["offers the roll export", buttonMatching("/export/i")],
    ],
  },
  {
    name: "chronicle",
    path: "/chronicle",
    assertions: [
      ["renders", "document.querySelectorAll('#app *').length > 100"],
    ],
  },
  {
    name: "event",
    path: `/e/${eventId}`,
    assertions: [
      ["renders", "document.querySelectorAll('#app *').length > 400"],
      ["offers availability entry", buttonMatching("/mark availability/i")],
      ["offers scheduling", buttonMatching("/schedule event/i")],
      ["shows the band tab row", buttonMatching("/^Discussion/")],
      ["exactly one band panel visible", `${visibleBandPanels} === 1`],
    ],
  },
  {
    name: "responded",
    path: `/e/${eventId}/responded`,
    // The ONLY route allowed to log an error, and the reason is in the view:
    // `Responded.vue` POSTs the confirmation with the `email` from the query
    // string, so visiting the bare path — which is what this check does — is a
    // request the server correctly rejects, and the component correctly reports.
    // Reaching it the real way needs a live confirmation link from an email.
    // Everything else about the route is still asserted below.
    expectConsoleErrors: /HTTP 400/,
    assertions: [
      // The view POSTs a confirmation on `created` and renders one of three
      // states. Which one depends on whether this event has a response pending
      // for this member, so assert that it reached *a* terminal state — the
      // route mounting and its conditional rendering working is the signal.
      [
        "reaches a confirmation state",
        hasText(
          "/confirming response|has been confirmed|something went wrong/i"
        ),
      ],
    ],
  },
  {
    name: "privacy-policy",
    path: "/privacy-policy",
    assertions: [
      // The whole view is one iframe onto a Google Doc, so element count is
      // meaningless here — the iframe either mounted or it did not.
      [
        "embeds the policy document",
        "!!document.querySelector('iframe[src*=\"docs.google.com\"]')",
      ],
    ],
  },
  {
    name: "404",
    path: "/definitely-not-a-route",
    assertions: [
      ["shows the not-found page", hasText("/404 - Page not found/")],
    ],
  },
]

/* ---------- band tabs ---------- */

// All five. The fifth ("Settle Up", added by F22) is the one that forced
// `tw-flex-wrap` onto the tab row, so it is also the reason the phone pass
// below asserts on horizontal scroll.
const BAND_TABS = ["Discussion", "Lists", "My Lists", "My Notes", "Settle Up"]

/* ---------- runner ---------- */

let failures = 0

function report(ok, label, detail) {
  if (!ok) failures++
  console.log(`${ok ? "PASS" : "FAIL"}  ${label}`)
  if (!ok && detail !== undefined) console.log(`      ${detail}`)
}

// Numbered in visit order so the directory reads as the run did, and slugged
// from the label so a FAIL line names its own picture.
let shotSeq = 0
async function shoot(cdp, label) {
  if (!SHOTS) return
  const name = `${String(++shotSeq).padStart(2, "0")}-${label
    .replace(/[^a-z0-9]+/gi, "-")
    .replace(/^-|-$/g, "")
    .toLowerCase()}.png`
  try {
    // Full-page, because the bug class this is aimed at is exactly the one that
    // lives below the fold — L2's "+" button was 1,300px down.
    await screenshot(cdp, path.join(SHOTS, name), { fullPage: true })
  } catch (e) {
    // A missing picture must never turn a passing run red: this is diagnosis,
    // not an assertion.
    console.log(`      (screenshot ${name} failed: ${e.message})`)
  }
}

async function setViewport(cdp, vp) {
  await cdp("Emulation.setDeviceMetricsOverride", {
    width: vp.width,
    height: vp.height,
    deviceScaleFactor: 1,
    mobile: vp.mobile,
  })
}

/** Navigates, waits for the SPA to settle, and reports console noise. */
async function visit(cdp, events, url, label, expected = null) {
  events.length = 0
  await cdp("Page.navigate", { url })
  await sleep(6000)

  // `expected` is a regexp for errors a route legitimately produces. Anything
  // not matching it still fails — muting a whole route would defeat the point.
  const errors = pageErrors(events).filter(
    (e) => !(expected && expected.test(e))
  )
  report(
    errors.length === 0,
    `${label} — no console errors`,
    errors.slice(0, 3)
  )
  const warnings = frameworkWarnings(events)
  report(
    warnings.length === 0,
    `${label} — no framework warnings`,
    warnings.slice(0, 3)
  )

  await shoot(cdp, label)
}

async function main() {
  const { cdp, events, close } = await launch({ port: 9444 })

  try {
    await cdp("Network.setCookie", {
      name: "session",
      value: COOKIE,
      domain: HOST,
      path: "/",
    })

    console.log("--- routes, desktop (1280x900) ---")
    await setViewport(cdp, DESKTOP)

    for (const route of routes(EVENT_ID)) {
      await visit(
        cdp,
        events,
        BASE + route.path,
        route.name,
        route.expectConsoleErrors
      )

      const path = await evaluate(cdp, "location.pathname")
      if (/sign-in/.test(path)) {
        report(
          false,
          `${route.name} — stayed off sign-in`,
          "the session cookie was not accepted. Check SESSION_SECRET matches " +
            "the running server, and that the user is on the allowlist — " +
            "AuthRequired enforces the roll on every request."
        )
        continue
      }

      for (const [name, check] of route.assertions) {
        report((await evaluate(cdp, check)) === true, `${route.name} — ${name}`)
      }
    }

    console.log("\n--- event band tabs ---")
    await visit(cdp, events, `${BASE}/e/${EVENT_ID}`, "event (tabs)")
    for (const tab of BAND_TABS) {
      const clicked = await evaluate(cdp, clickButton(`/^${tab}( |$)/`))
      if (clicked !== "ok") {
        report(false, `band tab "${tab}" — present`, clicked)
        continue
      }
      await sleep(1200)
      const visible = await evaluate(cdp, visibleBandPanels)
      report(
        visible === 1,
        `band tab "${tab}" — exactly one panel visible`,
        `saw ${visible}`
      )
      await shoot(cdp, `band-${tab}`)
    }

    console.log("\n--- dialogs ---")
    await visit(cdp, events, `${BASE}/home`, "home (dialog)")
    const opened = await evaluate(cdp, clickButton("/call a gathering/i"))
    report(opened === "ok", "New Gathering — activator present")
    await sleep(2500)
    // Asserted on the ARIA role rather than a Vuetify class. This line
    // originally read `.v-dialog--active`, which Vuetify 3 does not emit — so
    // the check failed on the framework upgrade while the dialog itself was
    // opening perfectly well. `[role=dialog]` is the app's contract with
    // assistive technology and survives the next upgrade too.
    report(
      (await evaluate(
        cdp,
        `[...document.querySelectorAll('[role=dialog]')]
           .filter((e) => e.getBoundingClientRect().height > 0).length === 1`
      )) === true,
      "New Gathering — dialog opens"
    )
    report(
      (await evaluate(cdp, hasText("/Dates and times/i"))) === true,
      "New Gathering — form renders"
    )
    await shoot(cdp, "new-gathering-dialog")

    console.log("\n--- phone (390x844) ---")
    await setViewport(cdp, PHONE)
    for (const [label, path] of [
      ["home", "/home"],
      ["event", `/e/${EVENT_ID}`],
    ]) {
      await visit(cdp, events, BASE + path, `${label} @390px`)
      report(
        (await evaluate(cdp, noHorizontalScroll)) === true,
        `${label} @390px — no horizontal scroll`
      )
    }
  } finally {
    close()
  }

  if (SHOTS) console.log(`\n${shotSeq} screenshot(s) in ${path.resolve(SHOTS)}`)
  console.log(failures === 0 ? "\nALL PASS" : `\n${failures} FAILURE(S)`)
  process.exit(failures === 0 ? 0 : 1)
}

main().catch((e) => {
  console.error("harness error:", e.message)
  process.exit(2)
})
