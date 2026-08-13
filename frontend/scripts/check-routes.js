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

function takeFlag(name) {
  const at = args.indexOf(name)
  if (at === -1) return false
  args.splice(at, 1)
  return true
}

const SHOTS = takeOption("--shots")
// Set by browser-check.sh --dev. The service worker registers in production
// builds only (see utils/offline/sw.js), so the offline section has nothing to
// assert against a webpack-dev-server and says so rather than passing hollowly.
const DEV_SERVER = takeFlag("--dev-server")
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

// The hover card's own open delay (MemberHoverCard.OPEN_DELAY_MS). Nothing can
// appear before this, so it is a floor to wait out rather than a timeout.
const OPEN_DELAY_FLOOR_MS = 600

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
 * Hovers the nth member hover-card trigger on the page (N3).
 *
 * Aimed at `[data-member-hover]` rather than at the avatar or name inside it,
 * because `mouseenter` does not bubble: dispatched at a child it would reach
 * the listener on the wrapper not at all, and the check would report a broken
 * card on a page where hovering works perfectly.
 */
const hoverMember = (n = 0) => `(() => {
  const els = [...document.querySelectorAll('[data-member-hover]')]
  if (els.length <= ${n}) return 'ONLY ' + els.length + ' TRIGGER(S)'
  els[${n}].dispatchEvent(new MouseEvent('mouseenter', { bubbles: false }))
  return 'ok'
})()`

/**
 * How many hover-card triggers a real pointer could never reach.
 *
 * A trigger can sit perfectly in the DOM, wrap the right person, pass every
 * mount test — and be covered by something absolutely positioned on top of it,
 * at which point it can never open. That is not hypothetical: the respondents
 * list positions its select-respondent checkbox over the 16px avatar and fades
 * it in on row hover, so a card wrapped round that avatar was dead on arrival.
 * Nothing else in the repo can see this. The `dom` tier has no layout and
 * therefore no hit testing, and `dispatchEvent` on an element bypasses hit
 * testing by definition — which is why the check's own hover helper, which must
 * use it, cannot double as this assertion.
 *
 * `elementFromPoint` at the trigger's centre is the browser's own answer to
 * "what would the mouse hit here". Triggers scrolled outside the viewport are
 * skipped rather than counted: they have no meaningful centre, and being
 * below the fold is not the same defect.
 */
const hoverTriggerReachability = `(() => {
  const covered = []
  let tested = 0
  for (const el of document.querySelectorAll('[data-member-hover]')) {
    // Scrolled to the middle first, so the centre being tested is genuinely
    // inside the viewport. Without this the whole check degrades to "every
    // trigger was off-screen, nothing to test" and passes over the bug — which
    // is how the first version of this assertion managed to stay green while
    // the respondents-list avatar was provably unreachable.
    // behavior:'instant' is required, not tidiness: App.vue sets
    // scroll-behavior:smooth globally, so a plain scrollIntoView ANIMATES and
    // every measurement below is taken before the scroll lands — the element
    // reads as off-screen and gets skipped, quietly shrinking what this
    // assertion covers to almost nothing. (No backticks in this comment: it
    // lives inside a template literal.)
    el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' })
    const r = el.getBoundingClientRect()
    if (r.width === 0 || r.height === 0) continue
    const cx = r.left + r.width / 2
    const cy = r.top + r.height / 2
    if (cx < 0 || cy < 0 || cx >= window.innerWidth || cy >= window.innerHeight) continue
    tested++
    const top = document.elementFromPoint(cx, cy)
    if (!top || !(el === top || el.contains(top))) {
      covered.push(
        ((el.innerText || '').trim().slice(0, 16) || '(no text)') +
          ' under ' + (top ? top.tagName : 'nothing')
      )
    }
  }
  return JSON.stringify({ tested, covered })
})()`

/**
 * The text of the open hover card, or a reason there isn't one.
 *
 * Scoped to the overlay, and that scoping is the whole assertion: the
 * Fellowship page already prints every member's email and telephone in the
 * directory cards themselves, so `document.body.innerText` would contain the
 * phone number whether or not the card ever opened.
 */
const hoverCardText = `(() => {
  const panels = [...document.querySelectorAll('.v-overlay__content')]
    .filter((e) => e.getBoundingClientRect().height > 0)
  if (panels.length === 0) return 'NO CARD'
  if (panels.length > 1) return 'MULTIPLE CARDS'
  return panels[0].innerText.replace(/\\s+/g, ' ')
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
      // Either label — `Settings.vue` renders `hasAvatar ? "Change photo" :
      // "Add photo"`, so pinning to "Add photo" asserted the seed fixture's
      // avatar-less account rather than the control existing. It failed against
      // an account that has a photo, which is not a defect.
      ["offers avatar upload", buttonMatching("/(add|change) photo/i")],
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
      // An element count asserted the environment's DATA VOLUME, not the view.
      // `Chronicle.vue` has three states — loading, empty, entries — and 100 is
      // above what a real but modest chronicle produces: production renders
      // several entries in 71 elements and failed this, while the seed
      // fixture's chronicle clears 100 comfortably. So it passed on the fixture
      // and nowhere else.
      //
      // Assert the two things that are actually the view's job instead: it
      // mounted (the header is outside all three branches, so it is the one
      // stable marker), and it is no longer loading. That is state-independent
      // — empty and populated both pass, a page stuck on the spinner does not —
      // and it still works as this route's readiness signal (see `ready`
      // below), since the header only appears once the view mounts.
      //
      // It deliberately does NOT assert that entries exist: `Chronicle.vue`
      // catches a failed `/chronicle` fetch and sets `entries = []` without
      // logging, so an outage and an empty chronicle are the same DOM. Nothing
      // here can tell them apart — not even "no console errors" — and an
      // assertion implying otherwise would be false comfort.
      [
        "renders",
        `/A record of gatherings past/i` +
          `.test(document.body.innerText.replace(/\\s+/g, ' ')) &&` +
          ` document.querySelectorAll('.v-progress-circular').length === 0`,
      ],
    ],
  },
  {
    name: "event",
    path: `/e/${eventId}`,
    assertions: [
      ["renders", "document.querySelectorAll('#app *').length > 400"],
      // Either label — `Event.vue` renders `userHasResponded ? "Edit
      // availability" : "Mark availability"`. Pinning to "Mark" asserted that
      // the signed-in member had not yet responded to this event, which is
      // fixture state, not behaviour.
      [
        "offers availability entry",
        buttonMatching("/(mark|edit) availability/i"),
      ],
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

/**
 * Reports whether every hover-card trigger on the current page can actually be
 * hit by a pointer — and refuses to pass when it tested nothing.
 *
 * The "tested > 0" half is not belt-and-braces. The first version of this
 * assertion had no viewport set (the desktop-routes loop that sets one is
 * skipped under `--only`), so every trigger measured off-screen, none was
 * tested, and it reported PASS while the respondents-list avatar was provably
 * unreachable. An assertion that cannot fail is worse than no assertion,
 * because it gets quoted as evidence.
 */
async function reportReachability(cdp, label) {
  const raw = await evaluate(cdp, hoverTriggerReachability)
  let parsed
  try {
    parsed = JSON.parse(raw)
  } catch (e) {
    report(
      false,
      `${label} — every trigger is reachable by a pointer`,
      `${raw}`
    )
    return
  }
  report(
    parsed.tested > 0 && parsed.covered.length === 0,
    `${label} — every trigger is reachable by a pointer`,
    parsed.tested === 0
      ? "tested none — no trigger was on screen, so this proved nothing"
      : `${parsed.covered.length}/${
          parsed.tested
        } covered: ${parsed.covered.join("; ")}`
  )
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

/* ---------------------------------------------------------------- *
 * Offline
 *
 * The only tier that can check any of this. The unit tiers have no service
 * worker, no Cache Storage, no real IndexedDB and no navigation — so "the app
 * opens with the network switched off" is a claim only a real browser can
 * settle.
 *
 * The header assertions are not decoration either. A service worker served as
 * text/html fails its update check on the MIME type rather than updating, and
 * because this server answers any unmatched path with the SPA shell, that is
 * one deleted file away at all times. A client in that state is pinned to a
 * dead build and cannot be rescued by deploying anything except a replacement
 * at that exact URL. These two lines are the guard on it.
 * ---------------------------------------------------------------- */

const OFFLINE_ON = {
  offline: true,
  latency: 0,
  downloadThroughput: 0,
  uploadThroughput: 0,
}
const OFFLINE_OFF = {
  offline: false,
  latency: 0,
  downloadThroughput: -1,
  uploadThroughput: -1,
}

/** The worker is registered, active, and controlling this page. */
const swControlling = `(async () => {
  if (!("serviceWorker" in navigator)) return false
  const reg = await navigator.serviceWorker.ready
  return !!reg.active && !!navigator.serviceWorker.controller
})()`

/** The shell and the build's own chunks made it into Cache Storage. */
const shellCached = `(async () => {
  const names = await caches.keys()
  if (names.length === 0) return false
  const shell = await caches.match("/")
  return !!shell
})()`

const precachedChunkCount = `(async () => {
  const names = await caches.keys()
  let n = 0
  for (const name of names) {
    const keys = await (await caches.open(name)).keys()
    n += keys.filter((r) => /\\/js\\/.*\\.js/.test(new URL(r.url).pathname)).length
  }
  return n
})()`

/**
 * How /service-worker.js is actually served, read from the page so the real
 * response headers are what gets asserted rather than what the config says.
 */
const serviceWorkerHeaders = `(async () => {
  const res = await fetch("/service-worker.js", { cache: "no-store" })
  return {
    status: res.status,
    type: res.headers.get("content-type") || "",
    cache: res.headers.get("cache-control") || "",
  }
})()`

async function runOfflineChecks(cdp, events) {
  if (DEV_SERVER) {
    // Announced, never silent. A section that quietly passed zero assertions
    // is the failure mode this whole file is built to avoid.
    report(true, "offline — SKIPPED on the dev server (no worker registered)")
    return
  }

  // Warm everything the offline load will need: the worker installs and
  // precaches, and the app caches the gathering it reads.
  await visit(cdp, events, `${BASE}/e/${EVENT_ID}`, "event (warming)", {
    ready: buttonMatching("/^Discussion/"),
  })

  // How the worker is actually served. This has to come AFTER a navigation:
  // the fetch is issued from the page, and on a fresh browser the section
  // would otherwise run it against about:blank — which is how these three
  // assertions passed in a full run and failed under `--only offline`, the
  // exact order-dependence that makes a section untrustworthy.
  const headers = await evaluate(cdp, serviceWorkerHeaders)
  report(
    headers?.status === 200,
    "service worker — served",
    `status ${headers?.status}`
  )
  report(
    /javascript/i.test(headers?.type || "") &&
      !/text\/html/i.test(headers?.type || ""),
    "service worker — served as JavaScript, not the SPA shell",
    `content-type: ${headers?.type}`
  )
  report(
    /no-cache|no-store|max-age=0/i.test(headers?.cache || ""),
    "service worker — always revalidated, so it can be replaced",
    `cache-control: ${headers?.cache}`
  )

  const controlling = await evaluate(cdp, swControlling)
  report(controlling === true, "service worker — registered and controlling")

  report((await evaluate(cdp, shellCached)) === true, "app shell — cached")

  const chunks = await evaluate(cdp, precachedChunkCount)
  report(
    typeof chunks === "number" && chunks > 5,
    "route chunks — precached, so a cold offline load can boot",
    `found ${chunks}`
  )

  // ---- a write made with no connection, and what becomes of it ----
  //
  // The queue's own rules are unit-tested; what only a browser can settle is
  // that a comment typed offline appears on the page, survives the connection
  // coming back, and arrives at the server exactly once.
  // Unique per run. A constant marker counts every previous run's comment too
  // (the fixture is not reset between REUSE runs), which reads as "it arrived
  // three times" when it arrived once, three runs ago.
  const marker = `offline-check-${Date.now().toString(36)}`

  // Done here, on the page just warmed above, rather than after the cold-load
  // pass: this is about a write made while reading, which is when one actually
  // gets made. Discussion is the default tab on arrival.
  await cdp("Network.emulateNetworkConditions", OFFLINE_ON)
  await sleep(500)

  // Driven through the UI rather than through internals: the point is that the
  // existing, unmodified call sites work offline, and reaching past them into
  // the store would test the queue while skipping the thing being claimed.
  const composed = await evaluate(
    cdp,
    `(() => {
      // BY PLACEHOLDER, not "the first visible textarea" — the event
      // description editor is also a textarea and sits above this one on the
      // page, so the loose selector types the comment into the description and
      // then reports a broken feature.
      const box = [...document.querySelectorAll('textarea')].find(
        (el) =>
          el.offsetParent !== null &&
          /add a message/i.test(el.getAttribute('placeholder') || '')
      )
      if (!box) return 'NO COMPOSER'
      // The native setter, because Vue tracks the property rather than the
      // attribute: assigning box.value directly leaves the model untouched and
      // the Post button disabled.
      const setter = Object.getOwnPropertyDescriptor(
        window.HTMLTextAreaElement.prototype, 'value'
      )?.set
      setter.call(box, ${JSON.stringify(marker)})
      box.dispatchEvent(new Event('input', { bubbles: true }))
      return 'ok'
    })()`
  )

  if (composed !== "ok") {
    // Said out loud rather than skipped silently — a section that quietly
    // checks nothing is the failure mode this file exists to avoid.
    report(true, `offline write — SKIPPED (${composed} on the Discussion tab)`)
  } else {
    await sleep(400)
    const posted = await evaluate(cdp, clickButton("/^Post$/i"))
    report(posted === "ok", "offline write — the compose button is usable", posted)
    await sleep(1500)
    report(
      (await evaluate(cdp, hasText(new RegExp(marker)))) === true,
      "offline write — appears on the page straight away"
    )
    await shoot(cdp, "event-offline-write")

    // Reconnect, and let boot.js's drain run.
    await cdp("Network.emulateNetworkConditions", OFFLINE_OFF)
    await sleep(4000)

    // Read it back from the SERVER, bypassing every cache, and count it.
    const onServer = await evaluate(
      cdp,
      `(async () => {
        const res = await fetch("/api/events/${EVENT_ID}", { cache: "no-store" })
        const event = await res.json()
        const hits = (event.comments || [])
          .filter((c) => (c.text || "").includes(${JSON.stringify(marker)}))
        return hits.length
      })()`
    )
    report(onServer === 1, "offline write — reached the server exactly once", `found ${onServer}`)
  }


  // Let the dashboard's prefetch finish; it is what makes gatherings readable
  // that this browser never opened.
  await visit(cdp, events, `${BASE}/home`, "home (warming)", {
    ready: buttonMatching("/call a gathering/i"),
  })
  // The prefetch deliberately waits for an idle moment (up to 2s) before it
  // starts, then walks the gatherings one at a time. Long enough to cover both.
  await sleep(6000)

  // ---- the actual question ----
  await cdp("Network.emulateNetworkConditions", OFFLINE_ON)

  // A cold load: not a reload of a live page, but a fresh navigation with the
  // network switched off, which is what a member opening the app on a train
  // actually does.
  await visit(cdp, events, `${BASE}/e/${EVENT_ID}`, "event @offline", {
    ready: buttonMatching("/^Discussion/"),
    // The browser reports every blocked request. Those are the point of the
    // exercise, not a fault — but anything else still fails.
    expected:
      /Failed to fetch|ERR_INTERNET_DISCONNECTED|ERR_NETWORK_CHANGED|ERR_FAILED/i,
  })

  report(
    (await evaluate(cdp, "location.pathname")).includes(`/e/${EVENT_ID}`),
    "event @offline — stayed on the gathering, not bounced to sign-in"
  )
  report(
    (await evaluate(cdp, "document.querySelectorAll('#app *').length > 200")) ===
      true,
    "event @offline — renders"
  )
  report(
    (await evaluate(cdp, hasText(/Offline/i))) === true,
    "event @offline — says it is offline"
  )

  // The three surfaces this feature exists for.
  for (const tab of ["Discussion", "Lists", "Settle Up"]) {
    const clicked = await evaluate(cdp, clickButton(`/^${tab}( |$)/`))
    if (clicked !== "ok") {
      report(false, `${tab} @offline — reachable`, clicked)
      continue
    }
    await sleep(800)
    report(
      (await evaluate(cdp, visibleBandPanels)) === 1,
      `${tab} @offline — its panel is the one showing`
    )
  }
  await shoot(cdp, "event-offline")

  await visit(cdp, events, `${BASE}/home`, "home @offline", {
    ready: buttonMatching("/call a gathering/i"),
    expected:
      /Failed to fetch|ERR_INTERNET_DISCONNECTED|ERR_NETWORK_CHANGED|ERR_FAILED/i,
  })
  report(
    (await evaluate(cdp, "location.pathname")) === "/home",
    "home @offline — stayed signed in"
  )
  await shoot(cdp, "home-offline")

  await cdp("Network.emulateNetworkConditions", OFFLINE_OFF)
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

    if (selected("member hover card")) {
      console.log("\n--- member hover card ---")
      // Set explicitly: the desktop-routes loop that normally does this is
      // skipped under `--only`, and every geometry assertion below is
      // meaningless without a known viewport.
      await setViewport(cdp, DESKTOP)
      // The Fellowship is the one page guaranteed to render a trigger for every
      // member of the seeded club, whatever else is or isn't in the fixture.
      await visit(cdp, events, `${BASE}/fellowship`, "fellowship (hover)", {
        ready: `document.querySelectorAll('[data-member-hover]').length > 0`,
      })

      const hovered = await evaluate(cdp, hoverMember(0))
      report(hovered === "ok", "member hover card — trigger present", hovered)

      if (hovered === "ok") {
        // Polled rather than slept, with the component's own 500ms open delay
        // as a FLOOR and ~5s as the ceiling. A flat wait was enough in a
        // one-section run and not in a full one, where the card has the roll
        // fetch and an overlay transition to get through on a busier page —
        // which surfaced as an empty card and read like a broken feature.
        // Polled on THIS assertion's own predicate, not on a proxy for it
        // ("is there any text yet?"), which is what made it flaky: the card
        // renders its avatar before its text lays out, so a poll that stopped
        // at "non-empty" could still read an empty string on a slow pass. The
        // component's 500ms open delay is a floor to wait out first; ~6s is the
        // ceiling, after which a genuinely broken card fails as it should.
        await sleep(OPEN_DELAY_FLOOR_MS)
        let text = ""
        for (let waited = 0; waited < 6000; waited += 250) {
          text = await evaluate(cdp, hoverCardText)
          if (typeof text === "string" && /\+15550100/.test(text)) break
          await sleep(250)
        }
        report(
          typeof text === "string" && /\+15550100/.test(text),
          "member hover card — opens with the member's details",
          `card said: ${text}`
        )
        // The 96px avatar is the request; a card that rendered the 40px byline
        // avatar would satisfy every other line here.
        //
        // Scoped to the VISIBLE overlay, not to the first `.v-overlay__content`
        // in document order. Vuetify leaves overlay containers in the DOM, so
        // the loose selector could measure an avatar belonging to some other
        // (closed) overlay entirely — which is what it did on a full run while
        // passing on a single-section one.
        //
        // Polled until it settles, because Vuetify SCALES the overlay in and
        // `getBoundingClientRect` reports the transformed box. Measured the
        // instant the text appears it reads 68px mid-animation — a real
        // measurement of a real element at the wrong moment, which is the most
        // convincing kind of wrong number.
        const measureAvatar = `(() => {
            const panel = [...document.querySelectorAll('.v-overlay__content')]
              .filter((e) => e.getBoundingClientRect().height > 0)[0]
            if (!panel) return 'NO CARD'
            const av = panel.querySelector('.v-avatar')
            if (!av) return 'NO AVATAR'
            return Math.round(av.getBoundingClientRect().width)
          })()`
        let avatarPx = await evaluate(cdp, measureAvatar)
        for (let waited = 0; waited < 2000 && avatarPx !== 96; waited += 200) {
          await sleep(200)
          avatarPx = await evaluate(cdp, measureAvatar)
        }
        report(
          avatarPx === 96,
          "member hover card — avatar at the Settings size",
          `measured ${avatarPx}`
        )
        await shoot(cdp, "member-hover-card")
      }

      await reportReachability(cdp, "member hover card")

      // The event page is where triggers share rows with controls, so it is
      // where being covered by one is likely rather than theoretical.
      await visit(cdp, events, `${BASE}/e/${EVENT_ID}`, "event (hover)", {
        ready: buttonMatching("/^Discussion/"),
      })
      await reportReachability(cdp, "event page")
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

    if (selected("offline")) {
      console.log("\n--- offline ---")
      await setViewport(cdp, DESKTOP)
      await runOfflineChecks(cdp, events)
    }
  } finally {
    // Whatever happened above, hand the browser back connected. A throw
    // part-way through the offline section would otherwise leave the emulation
    // in place for anything that runs after it.
    try {
      await cdp("Network.emulateNetworkConditions", OFFLINE_OFF)
    } catch {
      // The page is already gone; nothing to restore.
    }
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
