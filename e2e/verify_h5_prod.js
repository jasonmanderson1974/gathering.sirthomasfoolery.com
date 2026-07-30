// H5 (3aa2478) non-regression on LIVE prod: /api/user/calendars still returns a
// per-account map, and no account's error is the swallowed-unmarshal string the
// commit was fixing ("json: cannot unmarshal ... Response.error").
//
// The failure path itself (revoked consent) can't be forced on prod without
// breaking a real account — services_test.go / refresh_failures_test.go cover
// that. This asserts the healthy path still works after the signature change,
// and that any error present is a real reason.
//
// Assert-only, no PII: never logs event titles, emails, or account keys.
const { chromium } = require("playwright");

const BASE =
  process.env.TIMEFUL_BASE || "https://gathering.sirthomasfoolery.com";
const BAD = /cannot unmarshal|Response\.error/i;

(async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    storageState: `${__dirname}/prod_state.json`,
    viewport: { width: 1280, height: 900 },
  });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/home`, { waitUntil: "networkidle" });
  if (page.url().includes("/sign-in")) {
    throw new Error("prod_state.json is stale — re-run prod_login.js");
  }

  const timeMin = new Date();
  const timeMax = new Date(Date.now() + 7 * 24 * 3600 * 1000);

  const out = await page.evaluate(
    async ({ min, max }) => {
      const res = await fetch(
        `/api/user/calendars?timeMin=${encodeURIComponent(min)}&timeMax=${encodeURIComponent(max)}`,
        { credentials: "include" }
      );
      const body = await res.json().catch(() => null);
      if (!body || typeof body !== "object") return { status: res.status, accounts: [] };
      // Shape only — no titles, no emails.
      return {
        status: res.status,
        accounts: Object.values(body).map((v) => ({
          hasEventsArray: Array.isArray(v && v.calendarEvents),
          eventCount: Array.isArray(v && v.calendarEvents) ? v.calendarEvents.length : -1,
          error: v && v.error ? String(v.error) : null,
        })),
      };
    },
    { min: timeMin.toISOString(), max: timeMax.toISOString() }
  );

  const checks = {
    "HTTP 200": out.status === 200,
    "at least one calendar account": out.accounts.length > 0,
    "every account has a calendarEvents array": out.accounts.every((a) => a.hasEventsArray),
    "no account errored": out.accounts.every((a) => !a.error),
    "no swallowed-unmarshal error (the H5 symptom)": out.accounts.every(
      (a) => !a.error || !BAD.test(a.error)
    ),
  };

  console.log("status:", out.status, "| accounts:", out.accounts.length);
  console.log(
    "events per account:",
    out.accounts.map((a) => a.eventCount).join(", ")
  );
  for (const [k, v] of Object.entries(checks)) {
    console.log(`${v ? "PASS" : "FAIL"}  ${k}`);
  }
  const errs = out.accounts.filter((a) => a.error).map((a) => a.error);
  if (errs.length) console.log("account errors seen:", errs.join(" | "));

  await browser.close();
  const failed = Object.values(checks).filter((v) => !v).length;
  console.log(failed === 0 ? "H5 PASS" : `H5 FAIL (${failed})`);
  process.exit(failed === 0 ? 0 : 1);
})();
