// F22 (Settle Up) — standing PRODUCTION check.
//
// Runs against real member data, so this is ASSERT-ONLY: it reports counts and
// booleans and never prints a name, an email, or the contents of a ledger. It
// also WRITES NOTHING — the whole point of a standing prod check is that it can
// be re-run at any time without leaving anything behind.
//
//   TIMEFUL_VM=root@host node prod_login.js      # once, to seed prod_state.json
//   node verify_f22_prod.js [eventShortId]
const { chromium } = require("playwright");

const BASE = "https://gathering.sirthomasfoolery.com";
const STATE = `${__dirname}/prod_state.json`;

let pass = 0;
const failures = [];
const ok = (label, cond, detail = "") => {
  if (cond) {
    pass++;
    console.log(`  ✓ ${label}`);
  } else {
    failures.push(`${label}${detail ? ` — ${detail}` : ""}`);
    console.log(`  ✗ ${label}${detail ? ` — ${detail}` : ""}`);
  }
};

const domClick = (locator) => locator.evaluate((el) => el.click());

(async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    storageState: STATE,
    viewport: { width: 1400, height: 950 },
  });
  const page = await ctx.newPage();

  try {
    // Whichever gathering was named, else the first one on the home page — this
    // has to keep working as gatherings come and go.
    let event = process.argv[2];
    if (!event) {
      await page.goto(`${BASE}/home`, { waitUntil: "networkidle" });
      await page.waitForTimeout(2000);
      event = await page.evaluate(() => {
        const link = [...document.querySelectorAll("a")]
          .map((a) => a.getAttribute("href"))
          .find((h) => h && h.startsWith("/e/"));
        return link ? link.slice(3) : null;
      });
    }
    if (!event) throw new Error("no gathering found to check against");
    console.log(`checking a gathering (id withheld), build ${await version(page)}\n`);

    await page.goto(`${BASE}/e/${event}`, { waitUntil: "networkidle" });
    await page.waitForTimeout(2500);

    // --- the tab is there, and in the right place
    const tabs = await page.evaluate(() =>
      [...document.querySelectorAll("button")]
        .map((b) => b.textContent.trim())
        .filter((t) => /^(Discussion|Lists|My Lists|My Notes|Settle Up)/.test(t))
    );
    ok("the band still has its five tabs", tabs.length === 5, `${tabs.length}`);
    ok(
      "Settle Up sits to the right of My Notes",
      tabs.findIndex((t) => t.startsWith("Settle Up")) ===
        tabs.findIndex((t) => t.startsWith("My Notes")) + 1
    );

    // --- the ledger endpoint answers
    const ledger = await page.evaluate(async (id) => {
      const res = await fetch(`/api/events/${id}/expenses`, { credentials: "include" });
      const body = await res.json();
      return {
        status: res.status,
        isArray: Array.isArray(body),
        count: Array.isArray(body) ? body.length : -1,
        // Shape only — never the values.
        reconciles: Array.isArray(body)
          ? body.every(
              (e) =>
                (e.splits ?? []).reduce((sum, s) => sum + s.amountCents, 0) ===
                e.amountCents
            )
          : false,
        hasCanEdit: Array.isArray(body)
          ? body.every((e) => typeof e.canEdit === "boolean")
          : false,
      };
    }, event);

    ok("GET /expenses answers 200 with an array", ledger.status === 200 && ledger.isArray);
    ok(
      "every stored ledger reconciles to the cent",
      ledger.reconciles,
      `${ledger.count} row(s)`
    );
    ok("every row carries a computed canEdit", ledger.hasCanEdit);

    // --- the participant picker resolves real members
    const participants = await page.evaluate(async (id) => {
      const res = await fetch(`/api/events/${id}/expenses/participants`, {
        credentials: "include",
      });
      const body = await res.json();
      return {
        status: res.status,
        count: Array.isArray(body) ? body.length : -1,
        // Assert the slimming held: a picker entry must not carry an email.
        leaksEmail: Array.isArray(body) ? body.some((u) => !!u.email) : true,
      };
    }, event);

    ok("participants answers 200", participants.status === 200);
    ok(
      "and names at least the caller",
      participants.count >= 1,
      `${participants.count}`
    );
    ok("no email address is serialized into the picker", !participants.leaksEmail);

    // --- the panel renders
    await domClick(page.locator('button:has-text("Settle Up")').first());
    await page.waitForTimeout(1200);
    ok(
      "the Expenses panel renders",
      (await page.locator('text="Expenses"').count()) > 0
    );

    // Matched on the panel's heading being EXACTLY "Settle Up". Not on the text
    // merely starting with it — a gathering whose NAME begins with those words
    // has a title element that would count as a second panel — and not on the
    // heading being a <span>, which it briefly was.
    const panels = await page.evaluate(
      () =>
        [...document.querySelectorAll("div")].filter(
          (d) =>
            d.className.includes?.("tw-bg-leather") &&
            d.firstElementChild?.textContent.trim() === "Settle Up"
        ).length
    );
    ok("the balances render in exactly one place", panels === 1, `${panels} panels`);

    // --- no console errors on the way through
    const errors = [];
    page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
    await page.reload({ waitUntil: "networkidle" });
    await page.waitForTimeout(1500);
    await domClick(page.locator('button:has-text("Settle Up")').first());
    await page.waitForTimeout(1200);
    ok("no console errors", errors.length === 0, errors.slice(0, 2).join(" | "));

    // --- and nothing was written
    const after = await page.evaluate(async (id) => {
      const res = await fetch(`/api/events/${id}/expenses`, { credentials: "include" });
      return (await res.json()).length;
    }, event);
    ok("the check wrote nothing", after === ledger.count, `${ledger.count} → ${after}`);
  } finally {
    await browser.close();
  }

  console.log(`\n${pass} passed, ${failures.length} failed`);
  if (failures.length) {
    console.log("\nFAILURES:");
    failures.forEach((f) => console.log(`  - ${f}`));
    process.exit(1);
  }
})();

async function version(page) {
  return page.evaluate(async () => {
    try {
      return (await (await fetch("/api/health")).json()).version;
    } catch {
      return "unknown";
    }
  });
}
