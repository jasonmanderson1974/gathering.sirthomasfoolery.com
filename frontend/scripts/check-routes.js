#!/usr/bin/env node
/*
 * Every route, every band tab, signed in — does it render, and does it render
 * quietly?
 *
 * WHY THIS EXISTS: no unit test can fail on any of this. The `node` tier is
 * pure JS extracted *out of* components and renders nothing at all; the `dom`
 * tier (M6) mounts components but under happy-dom, with no real CSS, no layout,
 * no icon webfont and no viewport — so `v-show` beaten by an `!important`
 * display utility, an element 1,300px below the fold and a fifth tab that
 * wraps at 390px are all invisible to it by construction. This is the only
 * thing in the repo that sees a page the way a person does.
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
 *                                     [--shots <dir>] [--only <pattern>]
 *
 * `--shots` leaves a PNG of every page it visits in <dir> (TODO3 M1). A failing
 * run then hands over an image of the page that failed rather than only the name
 * of the assertion — which is the difference between "band tab Lists — exactly
 * one panel visible: FAIL" and seeing that two panels are stacked. For one page
 * in isolation use `scripts/shot.js`, which drives the same code.
 *
 * `--only <pattern>` runs just the sections whose name matches (TODO3 M7) — a
 * case-insensitive regexp, so `--only event` is the event page, its band tabs
 * and its phone pass, and nothing else. It changes NOTHING about what is
 * asserted; it exists so that fixing one page does not cost a full run each
 * time. A filtered run says so on every summary line it prints, because a green
 * "ALL PASS" that only checked one route is exactly the kind of thing that gets
 * quoted later as if it were a full one.
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
function takeOption(name) {
  const at = args.indexOf(name)
  if (at === -1) return null
  const value = args[at + 1]
  if (!value) {
    console.error(`${name} needs a value`)
    process.exit(2)
  }
  args.splice(at, 2)
  return value
}

const SHOTS = takeOption("--shots")
const ONLY_SOURCE = takeOption("--only")
// Built once so a bad pattern fails here, naming itself, rather than on the
// first section it is tested against.
let ONLY = null
if (ONLY_SOURCE) {
  try {
    ONLY = new RegExp(ONLY_SOURCE, "i")
  } catch (e) {
    console.error(`--only ${ONLY_SOURCE} is not a valid regexp: ${e.message}`)
    process.exit(2)
  }
}

const [rawBase, COOKIE, EVENT_ID] = args

if (!rawBase || !COOKIE || !EVENT_ID) {
  console.error(
    "usage: node scripts/check-routes.js <baseUrl> <sessionCookie> <eventId> " +
      "[--shots <dir>] [--only <pattern>]"
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

/**
 * The four TEXT faces arrived and painted, too.
 *
 * Same failure mode as the icon font above, and a quieter one: a text face that
 * fails to load does not leave blank squares, it silently falls through to the
 * next entry in the stack. `serif` and `sans-serif` are always available, so the
 * app renders in Times and Arial and looks merely *wrong* rather than broken —
 * no console error, no failed build, nothing to grep for. The whole visual
 * identity of the club is these four families.
 *
 * The `Variable` suffix is load-bearing (L9): these are the variable cuts from
 * `@fontsource-variable/*`, and that is the family name they declare. A stale
 * `"EB Garamond"` in a stylesheet matches no face and falls straight through to
 * `serif` — which is precisely the silent failure this asserts against, so the
 * names are spelled out here rather than derived from the CSS.
 */
const FONT_FAMILIES = [
  "DM Sans Variable",
  "Cinzel Variable",
  "Cormorant Garamond Variable",
  "EB Garamond Variable",
]

const textFontsLoaded = `(async () => {
  const want = ${JSON.stringify(FONT_FAMILIES)}
  for (const family of want) {
    const faces = [...document.fonts].filter((f) => f.family.replace(/["']/g, '') === family)
    if (faces.length === 0 || faces.some((f) => f.status === 'error')) return false
    const loaded = await document.fonts.load("16px '" + family + "'")
    if (loaded.length === 0 || !loaded.every((f) => f.status === 'loaded')) return false
  }
  return true
})()`

/**
 * ...and all of them from us, not from Google.
 *
 * L9: these were two \`<link rel="stylesheet">\`s to fonts.googleapis.com, which
 * blocked first paint on a third party and handed Google the IP and User-Agent
 * of every member of an invite-only club on every page load.
 *
 * Asserted two ways, because they fail differently. The \`@font-face\` src check
 * catches a face served from somewhere else; the stylesheet-origin check catches
 * a re-added Google stylesheet *even when its rules are unreadable* — a
 * cross-origin sheet throws on \`.cssRules\`, so a font arriving that way is
 * invisible to the first check by construction. Origin-checking only what we
 * can read would quietly pass the exact regression this exists to stop.
 *
 * The second check walks \`@import\` rules recursively, and that is not
 * hypothetical thoroughness: the third Google request in this app was an
 * \`@import url(...)\` in App.vue's style block, which L9 had not counted and
 * which the first draft of this assertion missed. An imported sheet is a
 * CSSImportRule inside its parent — it is NOT a top-level entry in
 * \`document.styleSheets\`, so iterating that list alone cannot see it.
 */
const textFontsSelfHosted = `(() => {
  const want = ${JSON.stringify(FONT_FAMILIES)}
  const bad = /fonts\\.(googleapis|gstatic)\\.com/

  for (const link of document.querySelectorAll('link[rel="stylesheet"], link[rel="preconnect"], link[rel="preload"]')) {
    if (bad.test(link.href)) return false
  }

  const seen = new Set()
  let clean = true
  const walk = (sheet) => {
    if (!clean) return
    if (sheet.href && bad.test(sheet.href)) { clean = false; return }
    let rules
    try { rules = sheet.cssRules } catch { return }
    for (const rule of rules) {
      if (rule instanceof CSSImportRule) {
        if (bad.test(rule.href || '')) { clean = false; return }
        if (rule.styleSheet) walk(rule.styleSheet)
        continue
      }
      if (!(rule instanceof CSSFontFaceRule)) continue
      const family = rule.style.fontFamily.replace(/["']/g, '')
      if (!want.includes(family)) continue
      const urls = [...rule.style.src.matchAll(/url\\(["']?([^"')]+)["']?\\)/g)].map((m) => m[1])
      if (urls.length === 0) { clean = false; return }
      if (!urls.every((u) => new URL(u, location.href).origin === location.origin)) { clean = false; return }
      // Content-hashed, same as the icon font — that is what earns it
      // \`immutable\` from the Go server (\`contentHashedAsset\`, main.go).
      if (!urls.every((u) => /\\.[0-9a-f]{8}\\.woff2/.test(u))) { clean = false; return }
      seen.add(family)
    }
  }
  for (const sheet of document.styleSheets) walk(sheet)

  return clean && want.every((f) => seen.has(f))
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
      ["the four text faces loaded", textFontsLoaded],
      ["the text faces are self-hosted", textFontsSelfHosted],
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

/*
 * How long a navigation is given to settle (TODO3 M7).
 *
 * This used to be a flat `sleep(6000)` per navigation — about 90 of the run's
 * 199 seconds spent waiting on pages that were done in one. It is now a
 * CEILING: poll the route's own first assertion until it is true, then allow a
 * short grace and move on.
 *
 * The grace is not padding. The two assertions every visit makes are about
 * console output, and some of it arrives after the page is usable — a panel
 * that fetches on mount can warn a beat after the element the readiness check
 * looks for exists. Returning the instant `ready` goes true would quietly
 * shrink the window those two assertions watch, which is a check getting
 * weaker while appearing to get faster. Six seconds of console is still six
 * seconds of console whenever the page needs it.
 */
const SETTLE_CEILING_MS = 6000
const SETTLE_GRACE_MS = 900
const POLL_MS = 250

/**
 * Navigates, waits for the SPA to settle, and reports console noise.
 *
 * `ready` is a page-side expression that is true once the route has rendered —
 * in practice the route's own first assertion, so nothing new has to be written
 * per route and the wait cannot pass on a page the run is about to fail.
 */
async function visit(cdp, events, url, label, { expected = null, ready } = {}) {
  events.length = 0
  await cdp("Page.navigate", { url })

  if (ready) {
    const deadline = Date.now() + SETTLE_CEILING_MS
    let settled = false
    while (Date.now() < deadline) {
      await sleep(POLL_MS)
      try {
        settled = (await evaluate(cdp, ready)) === true
      } catch {
        // Evaluating mid-navigation can fail outright; that is a "not yet".
        settled = false
      }
      if (settled) break
    }
    // Not settled means the full ceiling has already elapsed, so there is
    // nothing left to grant — the assertions below run on six seconds of page,
    // exactly as they always did.
    if (settled) await sleep(SETTLE_GRACE_MS)
  } else {
    await sleep(SETTLE_CEILING_MS)
  }

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

// Everything `--only` can select, counted so a filtered run can say what it
// left out rather than leaving the reader to infer it from a short log.
let skipped = 0
let ran = 0
const sectionNames = []
function selected(name) {
  sectionNames.push(name)
  if (!ONLY || ONLY.test(name)) {
    ran++
    return true
  }
  skipped++
  return false
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

    if (ONLY) console.log(`--- ONLY /${ONLY_SOURCE}/i — a PARTIAL run ---`)

    const desktopRoutes = routes(EVENT_ID).filter((r) => selected(r.name))

    if (desktopRoutes.length > 0) {
      console.log("--- routes, desktop (1280x900) ---")
      await setViewport(cdp, DESKTOP)
    }

    for (const route of desktopRoutes) {
      await visit(cdp, events, BASE + route.path, route.name, {
        expected: route.expectConsoleErrors,
        // The route's own first assertion doubles as its readiness signal, so
        // no route has to maintain a second description of "has it rendered".
        ready: route.assertions[0]?.[1],
      })

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

    if (selected("event band tabs")) {
      console.log("\n--- event band tabs ---")
      await visit(cdp, events, `${BASE}/e/${EVENT_ID}`, "event (tabs)", {
        ready: buttonMatching("/^Discussion/"),
      })
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
    }

    if (selected("dialogs")) {
      console.log("\n--- dialogs ---")
      await visit(cdp, events, `${BASE}/home`, "home (dialog)", {
        ready: buttonMatching("/call a gathering/i"),
      })
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
    }

    const phonePages = [
      ["home", "/home", buttonMatching("/call a gathering/i")],
      ["event", `/e/${EVENT_ID}`, buttonMatching("/^Discussion/")],
    ].filter(([label]) => selected(`${label} phone`))

    if (phonePages.length > 0) {
      console.log("\n--- phone (390x844) ---")
      await setViewport(cdp, PHONE)
      for (const [label, routePath, ready] of phonePages) {
        await visit(cdp, events, BASE + routePath, `${label} @390px`, { ready })
        report(
          (await evaluate(cdp, noHorizontalScroll)) === true,
          `${label} @390px — no horizontal scroll`
        )
      }
    }
  } finally {
    close()
  }

  if (SHOTS) console.log(`\n${shotSeq} screenshot(s) in ${path.resolve(SHOTS)}`)

  // The filter is repeated on the verdict line, not just announced at the top.
  // "ALL PASS" is the string that gets pasted into a commit message or read off
  // a scrollback hours later, and one that checked a single route has to be
  // unable to pass for the real thing.
  const partial = ONLY
    ? ` (PARTIAL: --only /${ONLY_SOURCE}/i, ${skipped} section(s) skipped)`
    : ""

  // A pattern matching nothing would otherwise print a perfectly green "ALL
  // PASS" over zero assertions, which is the single worst thing this option
  // could do. Exit 2 — the harness-error code, not the assertion-failure one.
  if (ran === 0) {
    console.error(
      `\n--only /${ONLY_SOURCE}/i matched no section, so NOTHING was checked.` +
        `\nThe sections are:\n  ${sectionNames.join("\n  ")}`
    )
    process.exit(2)
  }

  console.log(
    failures === 0
      ? `\nALL PASS${partial}`
      : `\n${failures} FAILURE(S)${partial}`
  )
  process.exit(failures === 0 ? 0 : 1)
}

main().catch((e) => {
  console.error("harness error:", e.message)
  process.exit(2)
})
