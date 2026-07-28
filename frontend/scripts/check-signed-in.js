#!/usr/bin/env node
/*
 * Signed-in pass over the event page.
 *
 * WHY THIS EXISTS: E3 phase 4 deleted the anonymous branches from components
 * whose remaining arms only ever ran for signed-in users. That shape of change
 * builds and lints clean while breaking at runtime — see TODO A11's caveat. It
 * caught a real one: removing the fields from SignUpForSlotDialog left a v-form
 * whose lazy-validation `formValid` never became true, which would have
 * permanently disabled the "Join slot" button.
 *
 * Local sign-in needs SMTP (OTP) or Google OAuth, neither of which is wired in
 * dev — so the session cookie is minted directly. See DEVELOPMENT.md for the
 * setup, including `server/tools/mintsession`.
 *
 * Usage: node scripts/check-signed-in.js <baseUrl> <sessionCookie> <eventId>
 */
const { launch, evaluate, pageErrors, sleep } = require("./browser-check-lib")

const [rawBase, COOKIE, EVENT_ID] = process.argv.slice(2)

if (!rawBase || !COOKIE || !EVENT_ID) {
  console.error(
    "usage: node scripts/check-signed-in.js <baseUrl> <sessionCookie> <eventId>"
  )
  process.exit(2)
}
const BASE = rawBase.replace(/\/$/, "")

const ASSERTIONS = [
  {
    name: "event page renders for a signed-in member",
    check: "document.querySelectorAll('#app *').length > 100",
  },
  {
    name: "no 'Your name' guest field anywhere (RSVP / polls)",
    check:
      "![...document.querySelectorAll('label')].some(l => /your name/i.test(l.textContent))",
  },
  {
    // These were previously gated on a guest name that no longer exists, so a
    // botched removal leaves them permanently disabled rather than absent.
    name: "RSVP buttons present and ENABLED (no leftover name gate)",
    check: `(() => {
      const btns = [...document.querySelectorAll('button')]
        .filter(b => /going|maybe|can't make it/i.test(b.textContent))
      return btns.length >= 3 && btns.every(b => !b.disabled)
    })()`,
  },
  {
    name: "'+ Add guest availability' (on-behalf entry) still offered",
    check:
      "[...document.querySelectorAll('button')].some(b => /add guest availability/i.test(b.textContent))",
  },
]

async function main() {
  const { cdp, events, close } = await launch({ port: 9333 })
  let failures = 0

  try {
    const { hostname } = new URL(BASE)
    await cdp("Network.setCookie", {
      name: "session",
      value: COOKIE,
      domain: hostname,
      path: "/",
    })

    events.length = 0
    await cdp("Page.navigate", { url: `${BASE}/e/${EVENT_ID}` })
    await sleep(6000)

    const path = await evaluate(cdp, "location.pathname")
    if (/sign-in/.test(path)) {
      console.log(
        "FAIL  the session cookie was not accepted — landed on sign-in.\n" +
          "      Check SESSION_SECRET matches the running server, and that the\n" +
          "      user exists AND is on the allowlist (AuthRequired enforces the\n" +
          "      roll on every request)."
      )
      process.exit(1)
    }

    for (const a of ASSERTIONS) {
      const ok = (await evaluate(cdp, a.check)) === true
      if (!ok) failures++
      console.log(`${ok ? "PASS" : "FAIL"}  ${a.name}`)
    }

    const errors = pageErrors(events)
    const noErrors = errors.length === 0
    if (!noErrors) failures++
    console.log(`${noErrors ? "PASS" : "FAIL"}  no console errors`)
    if (!noErrors) console.log("      errors:", errors.slice(0, 3))
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
