#!/usr/bin/env node
/*
 * Signed-out cold-load check.
 *
 * WHY THIS EXISTS: E3 phase 3 shipped a redirect loop that made the site
 * unreachable for anyone without a session — `/` and `/sign-in` ping-ponged and
 * neither rendered, so nobody could even log in. The router guard probes
 * GET /user/profile to decide whether anyone is signed in; for a signed-out
 * visitor that 401s with `not-signed-in`, which is the EXPECTED answer, but it
 * also matched the central session-ended handler, which pushed to /sign-in
 * while beforeEach was still resolving — cancelling the navigation, re-running
 * the guard, forever.
 *
 * It only reproduces on a cold load with no session cookie, which a browser
 * you're already signed into never reaches. Hence: throwaway profile, every run.
 *
 * Run this after ANY change to the router guard, fetch_utils' error path, or
 * auth-dependent rendering. See DEVELOPMENT.md.
 *
 * Usage: node scripts/check-signed-out.js [baseUrl]
 */
const { launch, evaluate, pageErrors, sleep } = require("./browser-check-lib")

const BASE = (process.argv[2] || "http://localhost:3002").replace(/\/$/, "")

// A gated route redirects whether or not the event exists, so this needs no
// seeded data — the id is deliberately fake.
const FAKE_EVENT_ID = "000000000000000000000000"

// Each case asserts BOTH where you land and that the destination actually
// rendered. The `check` matters: during the loop every route still had ~31
// elements of App.vue chrome, so an element count alone reported the landing
// and sign-in pages as fine while neither was usable.
const CASES = [
  {
    path: "/",
    expect: (u) => u === "/",
    check: "document.body.innerText.length > 200",
    rendered: "landing content",
    desc: "landing stays public",
  },
  {
    path: "/sign-in",
    expect: (u) => u.startsWith("/sign-in"),
    check:
      "!!document.querySelector('input[type=email], input[inputmode=email], form input')",
    rendered: "an email field",
    desc: "sign-in reachable and stable",
  },
  {
    path: `/e/${FAKE_EVENT_ID}`,
    expect: (u) => u.startsWith("/sign-in") && u.includes("redirect="),
    check:
      "!!document.querySelector('input[type=email], input[inputmode=email], form input')",
    rendered: "an email field",
    desc: "gated deep link -> sign-in carrying its destination",
  },
  {
    path: "/home",
    expect: (u) => u.startsWith("/sign-in") && u.includes("redirect="),
    check:
      "!!document.querySelector('input[type=email], input[inputmode=email], form input')",
    rendered: "an email field",
    desc: "gated route -> sign-in carrying its destination",
  },
  {
    path: "/privacy-policy",
    expect: (u) => u.startsWith("/privacy-policy"),
    // This page is a single cross-origin iframe, so its innerText is
    // legitimately empty — assert the frame, not the text.
    check: "!!document.querySelector('iframe')",
    rendered: "the policy iframe",
    desc: "privacy policy stays public",
  },
]

async function main() {
  const { cdp, events, close } = await launch({ port: 9222 })
  let failures = 0

  try {
    for (const c of CASES) {
      events.length = 0
      await cdp("Network.clearBrowserCookies")
      await cdp("Page.navigate", { url: BASE + c.path })
      await sleep(4000) // let the guard settle — or loop

      const url = await evaluate(cdp, "location.pathname + location.search")

      // A loop shows up as a pile of navigations for one Page.navigate. Note it
      // does NOT always fire: the original bug produced *cancelled* navigations,
      // which emit no frameNavigated — which is why the render check above is
      // the load-bearing assertion, not this.
      const navs = events.filter(
        (e) =>
          e.method === "Page.frameNavigated" ||
          e.method === "Page.navigatedWithinDocument"
      ).length

      const rendered = (await evaluate(cdp, c.check)) === true
      const errors = pageErrors(events)

      const ok = c.expect(url) && navs < 8 && rendered && errors.length === 0
      if (!ok) failures++

      console.log(
        `${ok ? "PASS" : "FAIL"}  ${c.path.padEnd(30)} -> ${url.padEnd(46)} ` +
          `navs=${navs} rendered=${rendered ? "y" : "N"} errs=${errors.length}` +
          `  (${c.desc})`
      )
      if (!ok && !rendered) console.log(`      expected to find: ${c.rendered}`)
      if (!ok && errors.length) console.log("      errors:", errors.slice(0, 3))
    }
  } finally {
    close()
  }

  console.log(failures === 0 ? "\nALL PASS" : `\n${failures} FAILURE(S)`)
  process.exit(failures === 0 ? 0 : 1)
}

main().catch((e) => {
  console.error("harness error:", e.message)
  process.exit(2)
})
