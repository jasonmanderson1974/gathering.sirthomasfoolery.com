#!/usr/bin/env node
/*
 * Take a PNG of one page. The missing half of "look at the page".
 *
 * WHY THIS EXISTS (TODO3 M1): that workflow rule sits above the others in this
 * repo and had no implementation anybody but a human with a browser could run.
 * `check-routes.js` already drove a real Chrome, held a session cookie and set
 * viewports; it just never took a picture, so a failing assertion reported a
 * name and never the page. This is the same driver with `Page.captureScreenshot`
 * on the end, so "does this look right" stops being a question only a deploy
 * can answer.
 *
 * Usage:
 *   node scripts/shot.js <url> [options]
 *
 *   --cookie <value>   session cookie value (mint one: server/tools/mintsession)
 *   --phone            390x844 mobile viewport (default: 1280x900 desktop)
 *   --viewport <WxH>   any other size, e.g. --viewport 768x1024
 *   --out <path>       PNG path, or a directory (default: frontend/shots/)
 *   --wait <ms>        settle time after navigation (default 6000, as check:routes)
 *   --full             the whole page, not just the fold
 *   --click <text>     click the first button whose text matches, then settle
 *                      again and shoot — for dialogs and band tabs
 *
 * Examples:
 *   node scripts/shot.js http://localhost:3002/ --out /tmp/landing.png
 *   node scripts/shot.js http://localhost:3010/home --cookie "$COOKIE" --phone --full
 *   node scripts/shot.js http://localhost:3010/home --cookie "$COOKIE" \
 *     --click "call a gathering" --out /tmp/new-gathering-dialog.png
 */
const fs = require("fs")
const path = require("path")
const { launch, evaluate, screenshot, sleep } = require("./browser-check-lib")

/* ---------- args ---------- */

const argv = process.argv.slice(2)
const opts = { wait: 6000, viewport: { width: 1280, height: 900, mobile: false } }
let url = null

for (let i = 0; i < argv.length; i++) {
  const a = argv[i]
  const next = () => {
    const v = argv[++i]
    if (v === undefined) {
      console.error(`${a} needs a value`)
      process.exit(2)
    }
    return v
  }
  if (a === "--cookie") opts.cookie = next()
  else if (a === "--out") opts.out = next()
  else if (a === "--wait") opts.wait = Number(next())
  else if (a === "--click") opts.click = next()
  else if (a === "--full") opts.full = true
  else if (a === "--phone") opts.viewport = { width: 390, height: 844, mobile: true }
  else if (a === "--viewport") {
    const m = /^(\d+)x(\d+)$/.exec(next())
    if (!m) {
      console.error("--viewport wants WxH, e.g. 768x1024")
      process.exit(2)
    }
    opts.viewport = { width: +m[1], height: +m[2], mobile: false }
  } else if (a === "--help" || a === "-h") {
    console.log(fs.readFileSync(__filename, "utf8").split("*/")[0])
    process.exit(0)
  } else if (a.startsWith("-")) {
    console.error(`unknown option ${a}`)
    process.exit(2)
  } else if (!url) url = a
  else {
    console.error(`unexpected argument ${a}`)
    process.exit(2)
  }
}

if (!url) {
  console.error("usage: node scripts/shot.js <url> [--cookie c] [--phone] [--out p] [--full] [--click text]")
  process.exit(2)
}

/** A filename from the URL's path, so a directory of shots is readable. */
function defaultName() {
  const p = new URL(url).pathname.replace(/^\/|\/$/g, "") || "index"
  const tag = opts.viewport.width === 390 ? "-phone" : ""
  return `${p.replace(/[^a-z0-9]+/gi, "-")}${tag}.png`
}

// `--out` is a file if it ends in .png, otherwise a directory. Default lives in
// frontend/shots/, which .gitignore covers — these are pictures of seeded or
// real member data and none of them belong in a commit.
let outPath
if (!opts.out) outPath = path.join(__dirname, "..", "shots", defaultName())
else if (/\.png$/i.test(opts.out)) outPath = opts.out
else outPath = path.join(opts.out, defaultName())

/* ---------- run ---------- */

async function main() {
  const { cdp, close } = await launch({ port: 9445 })
  try {
    if (opts.cookie) {
      await cdp("Network.setCookie", {
        name: "session",
        value: opts.cookie,
        domain: new URL(url).hostname,
        path: "/",
      })
    }
    await cdp("Emulation.setDeviceMetricsOverride", {
      width: opts.viewport.width,
      height: opts.viewport.height,
      deviceScaleFactor: 1,
      mobile: opts.viewport.mobile,
    })

    await cdp("Page.navigate", { url })
    await sleep(opts.wait)

    if (opts.click) {
      const re = JSON.stringify(opts.click)
      const clicked = await evaluate(
        cdp,
        `(() => {
          const el = [...document.querySelectorAll('button')]
            .find((b) => new RegExp(${re}, 'i').test(b.textContent.trim()))
          if (!el) return 'NOT FOUND'
          el.click()
          return 'ok'
        })()`
      )
      if (clicked !== "ok") {
        console.error(`no button matching ${opts.click} — shooting the page anyway`)
      }
      await sleep(2500)
    }

    // Reported so a caller (a human or an agent) has the path to open without
    // having to reconstruct the naming rule.
    console.log(await screenshot(cdp, outPath, { fullPage: !!opts.full }))
  } finally {
    close()
  }
}

main().catch((e) => {
  console.error("shot error:", e.message)
  process.exit(2)
})
