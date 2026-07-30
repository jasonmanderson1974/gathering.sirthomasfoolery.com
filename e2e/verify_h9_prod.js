// H9 (d99d18e): a too-small avatar source is refused at PICK time, with its real
// size named, and the cropper never opens. Boundary case (256x256) still opens.
//
// Assert-only + no-PII logging. Nothing is ever saved: the pass case is
// cancelled, so the member's real avatar is untouched.
//
// Usage: node verify_h9_prod.js   (needs a fresh prod_state.json)
const { chromium } = require("playwright");

const BASE =
  process.env.TIMEFUL_BASE || "https://gathering.sirthomasfoolery.com";

// Build a PNG of exactly w x h in the page, and hand the bytes back to Node so
// they can be fed to the hidden <input type=file>.
async function pngBuffer(page, w, h) {
  const b64 = await page.evaluate(({ w, h }) => {
    const c = document.createElement("canvas");
    c.width = w;
    c.height = h;
    const ctx = c.getContext("2d");
    ctx.fillStyle = "#7a5c2e";
    ctx.fillRect(0, 0, w, h);
    return c.toDataURL("image/png").split(",")[1];
  }, { w, h });
  return Buffer.from(b64, "base64");
}

const CASES = [
  { w: 100, h: 75, refuse: true, why: "the fixture H9 named" },
  { w: 2000, h: 10, refuse: true, why: "wide strip — square crop is bounded by 10px" },
  { w: 10, h: 2000, refuse: true, why: "tall strip" },
  { w: 255, h: 300, refuse: true, why: "one under the boundary" },
  { w: 256, h: 256, refuse: false, why: "the boundary, must still work" },
];

(async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    storageState: `${__dirname}/prod_state.json`,
    viewport: { width: 1280, height: 900 },
  });
  const page = await ctx.newPage();

  await page.goto(`${BASE}/settings`, { waitUntil: "networkidle" });
  if (page.url().includes("/sign-in")) {
    throw new Error("prod_state.json is stale — re-run prod_login.js");
  }

  // The snackbar from the previous case lingers (Vuetify's default 5s timeout),
  // so it has to be gone before the next pick or its text is read as this
  // case's verdict.
  const waitForNoSnackbar = async () => {
    for (let i = 0; i < 40; i++) {
      const visible = await page
        .locator(".v-snack__wrapper")
        .first()
        .isVisible()
        .catch(() => false);
      if (!visible) return true;
      await page.waitForTimeout(500);
    }
    return false;
  };

  const results = [];
  for (const c of CASES) {
    console.log(`-- case ${c.w}x${c.h}: waiting for a clear snackbar…`);
    if (!(await waitForNoSnackbar())) {
      throw new Error(`snackbar never cleared before case ${c.w}x${c.h}`);
    }
    const buf = await pngBuffer(page, c.w, c.h);

    await page.setInputFiles('input[type="file"]', {
      name: `probe_${c.w}x${c.h}.png`,
      mimeType: "image/png",
      buffer: buf,
    });
    await page.waitForTimeout(1200);

    const cropperOpen = await page
      .locator(".cropper-container")
      .isVisible()
      .catch(() => false);

    // The snackbar carries showError()'s text. Read only the VISIBLE ones:
    // Vuetify leaves the wrapper mounted after it hides, and allInnerTexts()
    // happily returns the previous case's message from that hidden node.
    const snackbar = await page.evaluate(() =>
      Array.from(document.querySelectorAll(".v-snack__wrapper"))
        .filter((el) => {
          const s = getComputedStyle(el);
          return (
            el.offsetParent !== null &&
            s.visibility !== "hidden" &&
            s.display !== "none" &&
            parseFloat(s.opacity) > 0.1
          );
        })
        .map((el) => el.innerText)
        .join(" ")
    );

    const namesSize = snackbar.includes(`${c.w}x${c.h}`);
    const saysTooSmall = /too small/i.test(snackbar);

    let ok;
    if (c.refuse) {
      ok = !cropperOpen && saysTooSmall && namesSize;
    } else {
      ok = cropperOpen && !saysTooSmall;
    }

    results.push({
      case: `${c.w}x${c.h}`,
      expect: c.refuse ? "REFUSE" : "OPEN",
      cropperOpen,
      saysTooSmall,
      namesSize,
      ok,
      why: c.why,
    });

    console.log(
      `   cropperOpen=${cropperOpen} saysTooSmall=${saysTooSmall} namesSize=${namesSize} -> ${ok ? "ok" : "NOT OK"}`
    );

    if (cropperOpen) {
      // Cancel — never save, so the real avatar is untouched.
      await page
        .locator(".v-dialog--active")
        .getByRole("button", { name: "Cancel" })
        .click({ timeout: 10000 });
      await page.waitForTimeout(800);
      console.log("   cropper cancelled (nothing saved)");
    }
    // Reset the input so the same-file change event fires next round.
    await page.evaluate(() => {
      const el = document.querySelector('input[type="file"]');
      if (el) el.value = "";
    });
  }

  await page.screenshot({ path: `${__dirname}/verify_h9_prod.png` });
  console.table(results);
  const failed = results.filter((r) => !r.ok);
  console.log(failed.length === 0 ? "H9 PASS" : `H9 FAIL (${failed.length})`);
  await browser.close();
  process.exit(failed.length === 0 ? 0 : 1);
})();
