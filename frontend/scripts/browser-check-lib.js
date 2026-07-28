/*
 * Minimal Chrome DevTools Protocol driver shared by the browser checks.
 *
 * Deliberately not Puppeteer: these checks exist to be run occasionally before
 * a deploy, and pulling a ~100MB browser download into devDependencies for that
 * is a poor trade. Chrome is already installed on the machines that run them.
 */
const { spawn } = require("child_process")
const http = require("http")
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

  let chrome, lastErr
  for (const bin of CHROME_BINARIES) {
    try {
      chrome = spawn(
        bin,
        [
          "--headless=new",
          `--remote-debugging-port=${port}`,
          `--user-data-dir=${profile}`,
          "--no-sandbox",
          "--disable-gpu",
          "about:blank",
        ],
        { stdio: "ignore" }
      )
      break
    } catch (e) {
      lastErr = e
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

/** Evaluates an expression in the page and returns its value. */
async function evaluate(cdp, expression) {
  const res = await cdp("Runtime.evaluate", { expression, returnByValue: true })
  return res?.result?.value
}

/** Console errors and uncaught exceptions recorded since `events` was cleared. */
function pageErrors(events) {
  const consoleErrors = events
    .filter(
      (e) => e.method === "Runtime.consoleAPICalled" && e.params.type === "error"
    )
    .map((e) => (e.params.args[0] || {}).value)
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

module.exports = { launch, evaluate, pageErrors, sleep }
