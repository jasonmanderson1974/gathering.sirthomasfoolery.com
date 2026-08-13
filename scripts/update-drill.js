#!/usr/bin/env node
/*
 * Can a member actually take an update?
 *
 * Driven by scripts/update-drill.sh. The service worker deliberately does NOT
 * take over mid-session — a deploy deletes the previous build's hashed chunks,
 * so swapping under a running app could leave it asking for a file that is
 * gone. The cost of that choice is that taking an update becomes a thing the
 * app has to OFFER, and an offer that fails to appear leaves someone stranded
 * on an old build with no way through short of force-quitting.
 *
 * That is not hypothetical. It shipped: the prompt was wired only to
 * `updatefound`, which fires for a worker installing while the page is open.
 * Refresh — which is what anyone does when something looks wrong — installed
 * the new build, and every refresh after that found it ALREADY waiting, so no
 * event fired and no prompt ever appeared again. Found by asking this question
 * of a real browser rather than reasoning about it.
 *
 * usage: node update-drill.js <baseUrl> <cookie> [distVolume]
 */
const { execSync } = require("child_process")
const {
  launch,
  evaluate,
  sleep,
} = require("../frontend/scripts/browser-check-lib")

const [BASE, COOKIE, VOLUME = "timeful-check_frontend_dev_dist"] =
  process.argv.slice(2)

let failures = 0
const report = (ok, label, detail) => {
  console.log(`${ok ? "PASS" : "FAIL"}  ${label}`)
  if (!ok) {
    failures++
    if (detail !== undefined) console.log(`      ${JSON.stringify(detail)}`)
  }
}

/** Ships a byte-different worker, which is what any real deploy produces. */
const shipNewBuild = () => {
  execSync(
    `docker run --rm -v ${VOLUME}:/dist alpine sh -c ` +
      `"cp /dist/service-worker.js /tmp/sw && (cat /tmp/sw; echo '//new-build') > /dist/service-worker.js"`,
    { stdio: "ignore" }
  )
}

const workerState = `(async () => {
  const reg = await navigator.serviceWorker.getRegistration()
  if (!reg) return { none: true }
  return { waiting: !!reg.waiting, active: !!reg.active }
})()`

const promptShowing = `/new version of The Fellowship is ready/i.test(document.body.innerText)`

async function main() {
  const { cdp, close } = await launch({ port: 9490 })
  try {
    await cdp("Network.setCookie", {
      name: "session",
      value: COOKIE,
      domain: new URL(BASE).hostname,
      path: "/",
    })

    await cdp("Page.navigate", { url: `${BASE}/home` })
    await sleep(9000)
    const first = await evaluate(
      cdp,
      `(async () => { const r = await navigator.serviceWorker.ready; return !!r.active })()`
    )
    report(first === true, "the running build has a worker")

    shipNewBuild()
    console.log("--- a new build has been deployed")

    // ---- 1. the update installs, and does NOT take over ----
    await evaluate(
      cdp,
      `(async () => { const r = await navigator.serviceWorker.getRegistration(); await r.update(); return "ok" })()`
    )
    await sleep(4000)
    let state = await evaluate(cdp, workerState)
    report(
      state.waiting === true && state.active === true,
      "the new build installs and WAITS — the running app is not swapped under",
      state
    )
    report(
      (await evaluate(cdp, promptShowing)) === true,
      "in-session — the member is offered the reload"
    )

    // ---- 2. and the offer survives a refresh ----
    //
    // The regression this drill exists for. On a refreshed page nothing is
    // installing any more, so `updatefound` never fires; the prompt has to come
    // from noticing a worker that is ALREADY waiting.
    await cdp("Page.navigate", { url: `${BASE}/home` })
    await sleep(8000)
    state = await evaluate(cdp, workerState)
    report(
      state.waiting === true,
      "after a refresh — the new build is still only waiting",
      state
    )
    report(
      (await evaluate(cdp, promptShowing)) === true,
      "after a refresh — the offer is STILL there (nobody gets stranded)"
    )

    // ---- 3. and taking it works ----
    const clicked = await evaluate(
      cdp,
      `(() => {
        const b = [...document.querySelectorAll('button')].find((b) => /^Reload$/i.test(b.textContent.trim()))
        if (!b) return 'NO BUTTON'
        b.click()
        return 'ok'
      })()`
    )
    report(clicked === "ok", "the Reload button is there to press", clicked)
    await sleep(7000)
    state = await evaluate(cdp, workerState)
    report(
      state.waiting === false && state.active === true,
      "after reload — the new build is the one serving",
      state
    )
    report(
      (await evaluate(cdp, "document.querySelectorAll('#app *').length > 50")) ===
        true,
      "after reload — the app still renders"
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
