// Post-deploy smoke on LIVE prod: each main screen renders and logs no console
// errors / failed requests. Read-only — navigates, never submits.
const { chromium } = require("playwright");

const BASE =
  process.env.TIMEFUL_BASE || "https://gathering.sirthomasfoolery.com";
const ROUTES = ["/home", "/settings", "/members", "/new"];

(async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    storageState: `${__dirname}/prod_state.json`,
    viewport: { width: 1280, height: 900 },
  });

  const rows = [];
  for (const route of ROUTES) {
    const page = await ctx.newPage();
    const errors = [];
    page.on("console", (m) => {
      if (m.type() === "error") errors.push(m.text().slice(0, 120));
    });
    page.on("requestfailed", (r) =>
      errors.push(`REQFAIL ${new URL(r.url()).pathname}`)
    );
    page.on("response", (r) => {
      if (r.status() >= 500) errors.push(`HTTP${r.status()} ${new URL(r.url()).pathname}`);
    });

    await page.goto(`${BASE}${route}`, { waitUntil: "networkidle", timeout: 45000 });
    await page.waitForTimeout(1500);

    const landed = new URL(page.url()).pathname;
    const bodyLen = (await page.locator("body").innerText()).trim().length;
    rows.push({
      route,
      landed,
      rendered: bodyLen > 50,
      consoleErrors: errors.length,
      first: errors[0] || "",
    });
    await page.close();
  }

  console.table(rows);
  const bad = rows.filter((r) => !r.rendered || r.consoleErrors > 0);
  console.log(bad.length === 0 ? "SMOKE PASS" : `SMOKE ISSUES (${bad.length})`);
  await browser.close();
})();
