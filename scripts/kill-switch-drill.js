#!/usr/bin/env node
/*
 * Kill-switch drill.
 *
 * Proves that deploying deploy/kill-service-worker.js in place of the real
 * worker actually removes it from a client that already has one — the rollback
 * the whole service worker decision rests on. Run against a stack that has
 * already served the real worker.
 *
 * Driven by scripts/kill-switch-drill.sh, which mints the cookie. See that
 * script's header for why this exists at all.
 *
 * usage: node kill-switch-drill.js <baseUrl> <cookie> [distVolume]
 */
const path = require("path")
const { execSync } = require("child_process")
const {
  launch,
  evaluate,
  sleep,
} = require("../frontend/scripts/browser-check-lib")

const [BASE, COOKIE, VOLUME = "timeful-check_frontend_dev_dist"] =
  process.argv.slice(2)
const DEPLOY_DIR = path.resolve(__dirname, "..", "deploy")
const HOST = new URL(BASE).hostname

let failures = 0
const report = (ok, label, detail) => {
  console.log(`${ok ? "PASS" : "FAIL"}  ${label}`)
  if (!ok) {
    failures++
    if (detail !== undefined) console.log(`      ${JSON.stringify(detail)}`)
  }
}

const state = `(async () => {
  const regs = await navigator.serviceWorker.getRegistrations()
  const names = await caches.keys()
  let entries = 0
  for (const n of names) entries += (await (await caches.open(n)).keys()).length
  return { registrations: regs.length, caches: names.length, entries }
})()`

async function main() {
  const { cdp, close } = await launch({ port: 9455 })
  try {
    await cdp("Network.setCookie", {
      name: "session",
      value: COOKIE,
      domain: HOST,
      path: "/",
    })

    // --- 1. get into the state the drill is about: a client holding a worker
    await cdp("Page.navigate", { url: `${BASE}/home` })
    await sleep(6000)

    const before = await evaluate(cdp, state)
    report(
      before.registrations > 0,
      "before — this client holds a service worker",
      before
    )
    report(before.entries > 20, "before — it has precached the build", before)

    // --- 2. deploy the kill switch over the real worker
    // Written straight into the volume the server reads, which is what
    // `cp deploy/kill-service-worker.js frontend/dist/service-worker.js`
    // followed by a deploy does on the real host.
    execSync(
      `docker run --rm ` +
        `-v ${VOLUME}:/dist ` +
        `-v ${DEPLOY_DIR}:/src:ro ` +
        `alpine cp /src/kill-service-worker.js /dist/service-worker.js`,
      { stdio: "inherit" }
    )
    console.log("--- kill switch deployed over the worker")

    // --- 3. the client's next update check finds it
    const updated = await evaluate(
      cdp,
      `(async () => {
        const reg = await navigator.serviceWorker.getRegistration()
        if (!reg) return "NO REGISTRATION"
        await reg.update()
        return "ok"
      })()`
    )
    report(updated === "ok", "update check — reached the new script", updated)

    // The kill switch skipWaiting()s, unregisters, clears caches and navigates
    // its clients. Give it room to do all four.
    await sleep(8000)

    // --- 4. and the client comes back clean
    const after = await evaluate(cdp, state)
    report(
      after.registrations === 0,
      "after — no service worker registered",
      after
    )
    report(after.caches === 0, "after — every cache emptied", after)

    // The app itself must still work, with no worker in front of it.
    await cdp("Page.navigate", { url: `${BASE}/home` })
    await sleep(5000)
    const renders = await evaluate(
      cdp,
      "document.querySelectorAll('#app *').length > 200"
    )
    report(renders === true, "after — the app still renders, workerless")
    const stillClean = await evaluate(cdp, state)
    report(
      stillClean.registrations === 0,
      "after — it does not re-register itself",
      stillClean
    )
  } finally {
    close()
  }

  console.log(failures === 0 ? "\nDRILL PASSED" : `\n${failures} FAILURE(S)`)
  process.exit(failures === 0 ? 0 : 1)
}

main().catch((e) => {
  console.error(e)
  process.exit(2)
})
