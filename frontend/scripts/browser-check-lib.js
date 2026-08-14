/*
 * Minimal Chrome DevTools Protocol driver shared by the browser checks.
 *
 * Deliberately not Puppeteer: these checks exist to be run occasionally before
 * a deploy, and pulling a ~100MB browser download into devDependencies for that
 * is a poor trade. Chrome is already installed on the machines that run them.
 */
const { spawn } = require("child_process")
const fs = require("fs")
const http = require("http")
const path = require("path")
const WebSocket = require("ws")

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

const CHROME_BINARIES = [
  process.env.CHROME_PATH,
  "google-chrome",
  "chromium",
  "chromium-browser",
].filter(Boolean)

function getJSON(port, path) {
  return new Promise((resolve, reject) => {
    http
      .get({ host: "127.0.0.1", port, path }, (res) => {
        let d = ""
        res.on("data", (c) => (d += c))
        res.on("end", () => {
          try {
            resolve(JSON.parse(d))
          } catch (e) {
            reject(e)
          }
        })
      })
      .on("error", reject)
  })
}

/**
 * Launches headless Chrome with a THROWAWAY profile and attaches to its page
 * target. The fresh profile is the point: these checks are about the cold-load,
 * no-cookie state, which a browser you're already signed into never reaches.
 *
 * @returns {Promise<{cdp, events, close}>} `events` accumulates CDP events;
 *   clear it between navigations to scope assertions to one page load.
 */
async function launch({ port = 9222 } = {}) {
  const profile = `/tmp/browser-check-${process.pid}-${Date.now()}`
  const args = [
    "--headless=new",
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profile}`,
    "--no-sandbox",
    "--disable-gpu",
    // KEEP THE RENDERER IN THE FOREGROUND. Not tuning — without these three,
    // Chrome is free to decide this window is backgrounded or occluded and stop
    // delivering frames to it, and a headless window has nothing to argue
    // otherwise. That is not merely slow, it is a permanent wrong answer:
    // EVERY Vuetify overlay opens through VDialogTransition, which sets
    // `visibility: hidden` in onBeforeEnter and clears it two
    // `requestAnimationFrame`s later. With no frames the second step never
    // runs, so a menu or dialog sits in the DOM — active, full-size, its card
    // rendered inside it — and invisible for as long as the page lives. It
    // cost two days as TODO3 N4: an intermittent hover-card failure that only
    // ever appeared on long runs, survived a 25-second poll, and read like a
    // broken feature. Puppeteer passes the same three by default, for the same
    // reason.
    "--disable-background-timer-throttling",
    "--disable-backgrounding-occluded-windows",
    "--disable-renderer-backgrounding",
    "about:blank",
  ]

  // A missing binary makes `spawn` emit an asynchronous 'error' event rather
  // than throw, so a plain try/catch around it never fires and the fallback
  // list below it is decorative — the first name always "succeeds" and the
  // process dies later with an unhandled ENOENT. Wait a beat for that event
  // instead, so the list actually falls through to chromium.
  let chrome, lastErr
  for (const bin of CHROME_BINARIES) {
    const child = spawn(bin, args, { stdio: "ignore" })
    const launched = await new Promise((resolve) => {
      const onError = (e) => {
        lastErr = e
        resolve(false)
      }
      child.once("error", onError)
      setTimeout(() => {
        child.removeListener("error", onError)
        resolve(true)
      }, 300)
    })
    if (launched) {
      chrome = child
      break
    }
  }
  if (!chrome) {
    throw new Error(
      `could not launch Chrome (tried: ${CHROME_BINARIES.join(", ")}). ` +
        `Set CHROME_PATH to override. ${lastErr?.message ?? ""}`
    )
  }

  let target
  for (let i = 0; i < 40; i++) {
    try {
      const list = await getJSON(port, "/json/list")
      target = list.find((t) => t.type === "page")
      if (target) break
    } catch {
      /* devtools not up yet */
    }
    await sleep(500)
  }
  if (!target) {
    chrome.kill()
    throw new Error("Chrome devtools never became reachable")
  }

  const ws = new WebSocket(target.webSocketDebuggerUrl, {
    perMessageDeflate: false,
  })
  let msgId = 0
  const pending = new Map()
  const events = []

  ws.on("message", (raw) => {
    const msg = JSON.parse(raw)
    if (msg.id && pending.has(msg.id)) {
      const { resolve } = pending.get(msg.id)
      pending.delete(msg.id)
      resolve(msg.result ?? msg.error)
    } else if (msg.method) {
      events.push(msg)
    }
  })
  await new Promise((r) => ws.on("open", r))

  const cdp = (method, params = {}) =>
    new Promise((resolve) => {
      const id = ++msgId
      pending.set(id, { resolve })
      ws.send(JSON.stringify({ id, method, params }))
    })

  await cdp("Page.enable")
  await cdp("Runtime.enable")
  await cdp("Network.enable")

  return {
    cdp,
    events,
    close: () => {
      ws.close()
      chrome.kill()
    },
  }
}

/**
 * Evaluates an expression in the page and returns its value.
 *
 * `awaitPromise` is always on: a non-promise result comes back unchanged, and
 * an assertion that needs to *do* something before it can answer — the icon
 * font one calls `document.fonts.load()` — would otherwise come back as the
 * string "[object Promise]" and quietly fail its `=== true`.
 */
async function evaluate(cdp, expression) {
  const res = await cdp("Runtime.evaluate", {
    expression,
    returnByValue: true,
    awaitPromise: true,
  })
  return res?.result?.value
}

/**
 * Console errors and uncaught exceptions recorded since `events` was cleared.
 *
 * Every argument is read, and each one falls back from `value` to
 * `description`. That fallback is the whole point: CDP only sets `value` for
 * primitives, so `console.error(someError)` — which is exactly how Vue reports
 * a throw from a lifecycle hook — arrives with `value` undefined and the real
 * message in `description`. Reading `args[0].value` alone reported those as no
 * error at all, and did: a `TypeError` thrown on every open of the New
 * Gathering dialog passed this check for the whole Vue 3 migration.
 */
function pageErrors(events) {
  const consoleErrors = events
    .filter(
      (e) =>
        e.method === "Runtime.consoleAPICalled" && e.params.type === "error"
    )
    .map((e) =>
      (e.params.args || [])
        .map((a) => a.value ?? a.description ?? "")
        .join(" ")
        .trim()
    )
    .filter(Boolean)
  const exceptions = events
    .filter((e) => e.method === "Runtime.exceptionThrown")
    .map(
      (e) =>
        e.params.exceptionDetails?.exception?.description ||
        e.params.exceptionDetails?.text
    )
    .filter(Boolean)
  return [...consoleErrors, ...exceptions]
}

/**
 * Framework warnings recorded since `events` was cleared — `[Vue warn]`,
 * `[Vuetify]` and friends, across every console level.
 *
 * Vue 2 emits its warnings through `console.error` and Vue 3 through
 * `console.warn`, so both levels are scanned rather than one; and every
 * argument is joined, because the component trace that says *where* the
 * warning came from is never the first one.
 *
 * IMPORTANT: production builds strip these warnings entirely. A run against a
 * `npm run build` bundle (which is what compose.dev.yaml serves) will report
 * zero no matter what. Point the check at `npm run serve` when the warnings are
 * the thing you care about.
 */
function frameworkWarnings(events) {
  return events
    .filter((e) => e.method === "Runtime.consoleAPICalled")
    .map((e) =>
      (e.params.args || [])
        .map((a) => a.value ?? a.description ?? "")
        .join(" ")
        .trim()
    )
    .filter((text) =>
      /\[Vue warn\]|\[Vuetify\]|\[vue-router\]|\[vuex\]/i.test(text)
    )
}

/**
 * Writes a PNG of the current page to `filePath` and returns the path.
 *
 * WHY THIS EXISTS (TODO3 M1): the workflow rule this repo puts above the others
 * is "look at the page", and until this call there was no way to do it here
 * except by being a human with a browser — the only screenshot code in the tree
 * was `verify_f9_prod.js`, Playwright, against production, needing a real
 * signed-in prod session. So the rule read: to look at the page, deploy it. One
 * CDP call the driver already had the socket for changes who can do the
 * checking, because a PNG is readable by an agent.
 *
 * `captureBeyondViewport` is how the full page is taken rather than the fold.
 * It cooperates with `Emulation.setDeviceMetricsOverride` — the width stays
 * whatever the viewport was set to, so a phone-width full-page shot is one
 * 390px-wide column and not a desktop page squeezed.
 */
async function screenshot(cdp, filePath, { fullPage = false } = {}) {
  const res = await cdp("Page.captureScreenshot", {
    format: "png",
    captureBeyondViewport: fullPage,
  })
  if (!res || !res.data) {
    throw new Error(
      `Page.captureScreenshot returned no data: ${JSON.stringify(res)}`
    )
  }
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, Buffer.from(res.data, "base64"))
  return filePath
}

module.exports = {
  launch,
  evaluate,
  pageErrors,
  frameworkWarnings,
  screenshot,
  sleep,
}
