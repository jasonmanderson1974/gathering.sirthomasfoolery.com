// LIVE (production) verification of F9 — the @mention composer and rendering.
//
// SAFETY: every write goes to a THROWAWAY gathering this harness creates under
// the signed-in account and deletes again at the end (also on failure). No
// existing gathering is touched. The only mention written names the signed-in
// account itself, and `mentionRecipients` drops the comment's own author, so
// NO mail is sent to anybody. Member data read from /mentionables is asserted
// on, never printed.
//
// Unlike the two standing checks beside it, this one drives Playwright rather
// than raw CDP, and Playwright is deliberately NOT a repo dependency (it pulls
// a browser download that CI has no use for). Run it from a box that has one
// installed:
//
//   NODE_PATH=/root/tools/browser/node_modules \
//     PROD_STATE=/root/tools/browser/prod_state.json \
//     node frontend/scripts/verify_f9_prod.js
//
// PROD_STATE is a Playwright storage state for a signed-in production session
// (see the out-of-repo `prod_login.js`, which fishes the OTP from prod Mongo);
// it defaults to ./prod_state.json. SHOT_DIR (default: the OS temp dir) is
// where the two screenshots land — they are artefacts, not repo content.
const { chromium } = require("playwright");
const fs = require("fs");
const os = require("os");

const BASE = process.env.PROD_BASE ?? "https://gathering.sirthomasfoolery.com";
const STATE = process.env.PROD_STATE ?? `${process.cwd()}/prod_state.json`;
const SHOT_DIR = process.env.SHOT_DIR ?? os.tmpdir();
const GHOST = "000000000000000000000000";
const TAG = `zz-harness-f9-${Date.now()}`;

const results = [];
function check(name, ok, detail = "") {
  results.push({ name, ok });
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? ` — ${detail}` : ""}`);
}

(async () => {
  if (!fs.existsSync(STATE))
    throw new Error(`no signed-in prod session at ${STATE} — run prod_login.js first, or set PROD_STATE`);
  const browser = await chromium.launch();
  const consoleErrors = [];
  let page = null;
  let eventId = null;

  const api = (method, path, body) =>
    page.evaluate(
      async ([method, path, body]) => {
        const res = await fetch(path, {
          method,
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          ...(body === undefined ? {} : { body: JSON.stringify(body) }),
        });
        const text = await res.text();
        let json = null;
        try {
          json = JSON.parse(text);
        } catch (e) {
          /* non-JSON body */
        }
        return { status: res.status, json };
      },
      [method, path, body]
    );

  try {
    // ============ 1. The mentionables route is live and gated ============
    const signedOut = await fetch(`${BASE}/api/events/${GHOST}/mentionables`);
    check(
      "GET /events/:id/mentionables 401s when signed out",
      signedOut.status === 401,
      `${signedOut.status}`
    );

    // ============ 2. A throwaway gathering ============
    const ctx = await browser.newContext({
      storageState: STATE,
      viewport: { width: 1440, height: 1000 },
    });
    page = await ctx.newPage();
    page.on("console", (m) => {
      if (m.type() === "error") consoleErrors.push(m.text());
    });
    await page.goto(`${BASE}/home`, { waitUntil: "networkidle" });
    check("signed-in home renders", !page.url().includes("/sign-in"));

    const tomorrow = new Date(Date.now() + 86400000);
    tomorrow.setUTCHours(17, 0, 0, 0);
    const created = await api("POST", "/api/events", {
      name: TAG,
      duration: 1,
      dates: [tomorrow.toISOString()],
      type: "specific_dates",
    });
    eventId = created.json?.eventId ?? created.json?.shortId ?? created.json?._id;
    check("throwaway gathering created", !!eventId, `${created.status}`);
    if (!eventId) throw new Error("no event id returned; cannot continue");

    const E = `/api/events/${eventId}`;

    // ============ 3. The candidate list ============
    const me = await api("GET", "/api/user/profile");
    const myId = me.json?._id;
    // The picker and the token both use `displayName`, which prefers a
    // nickname over the real name — filtering on firstName finds nobody for an
    // account that has one.
    const myName =
      (me.json?.nickname ?? "").trim() ||
      [me.json?.firstName, me.json?.lastName].filter(Boolean).join(" ").trim();
    check("the signed-in account resolves", !!myId && !!myName, `${me.status}`);

    const cands = await api("GET", `${E}/mentionables`);
    check(
      "GET /mentionables returns a list of candidates",
      cands.status === 200 && Array.isArray(cands.json) && cands.json.length > 0,
      `${cands.status} n=${Array.isArray(cands.json) ? cands.json.length : "n/a"}`
    );
    check(
      "every candidate carries the id + name the token format needs",
      (cands.json ?? []).every(
        (c) => /^[0-9a-f]{24}$/.test(c._id ?? "") && (c.firstName || c.lastName)
      )
    );
    check(
      "the owner is among their own candidates (self-mention is possible)",
      (cands.json ?? []).some((c) => c._id === myId)
    );

    // ============ 4. The composer: picker opens, filters, inserts ============
    await page.goto(`${BASE}/e/${eventId}`, { waitUntil: "networkidle" });
    await page.waitForTimeout(2500);
    check("the event page loads", page.url().includes(`/e/${eventId}`));

    const composer = page
      .locator('textarea[placeholder*="Add a message"]')
      .first();
    check("the composer renders", (await composer.count()) > 0);
    check(
      "the composer placeholder advertises mentions",
      /@ to mention/.test((await composer.getAttribute("placeholder")) ?? ""),
      (await composer.getAttribute("placeholder")) ?? ""
    );

    const picker = page.locator("div.tw-absolute.tw-z-20");
    await composer.click();
    await composer.type("hello ");
    check("the picker is shut while typing ordinary prose", (await picker.count()) === 0);

    await composer.type("@");
    await page.waitForTimeout(600);
    check("typing @ opens the picker", (await picker.count()) > 0);
    const openCount = await picker.locator("> div").count();
    check("the picker is capped at 8 candidates", openCount > 0 && openCount <= 8, `${openCount}`);

    // Filter down to my own display name — the only person this harness will
    // name, so that `mentionRecipients` (which drops the author) mails nobody.
    const firstName = myName.split(" ")[0];
    await composer.type(firstName);
    await page.waitForTimeout(600);
    const filtered = await picker.locator("> div").allInnerTexts();
    check(
      "typing a partial name filters the picker to matches only",
      filtered.length > 0 &&
        filtered.every((t) => t.toLowerCase().includes(firstName.toLowerCase())),
      `${filtered.length} candidate(s)`
    );

    // Enter picks the highlighted candidate rather than breaking the line.
    await composer.press("Enter");
    await page.waitForTimeout(400);
    const afterPick = await composer.inputValue();
    check("Enter closes the picker", (await picker.count()) === 0);
    check(
      "Enter inserts a well-formed token and does not add a newline",
      /@\[[^\]\n]{1,60}\]\([0-9a-f]{24}\)/.test(afterPick) && !afterPick.includes("\n"),
      JSON.stringify(afterPick)
    );
    check(
      "the inserted token names the candidate that was picked",
      afterPick.includes(`](${myId})`)
    );

    // ============ 5. Escape stays dismissed, a later @ still opens ============
    await composer.type("and ");
    await composer.type("@");
    await page.waitForTimeout(400);
    check("a second @ opens the picker again", (await picker.count()) > 0);
    await composer.press("Escape");
    await page.waitForTimeout(400);
    check("Escape dismisses the picker", (await picker.count()) === 0);
    await composer.type("x");
    await page.waitForTimeout(400);
    check(
      "the dismissed mention stays dismissed as it is typed into",
      (await picker.count()) === 0
    );

    // Clear the trailing junk, keeping the token.
    await composer.fill(afterPick + " — F9 harness");

    // ============ 6. Posting: stored token, rendered name ============
    await page.getByRole("button", { name: /^Post$/ }).first().click();
    await page.waitForTimeout(2500);

    // Comments ride along on the event; there is no standalone GET.
    const fetched = await api("GET", E);
    const comments = fetched.json?.comments ?? [];
    const posted = comments.find((c) => (c.text ?? "").includes("F9 harness"));
    check("the comment posted", !!posted, `${fetched.status}`);
    check(
      "the stored text keeps the token verbatim (the server's parse contract)",
      !!posted && posted.text.includes(`](${myId})`)
    );
    check(
      "the server parsed it into a real mentions entry",
      !!posted &&
        Array.isArray(posted.mentions) &&
        posted.mentions.map(String).includes(String(myId)),
      JSON.stringify(posted?.mentions ?? null)
    );

    const discussion = page.locator(".tw-whitespace-pre-wrap");
    const rendered = (await discussion.allInnerTexts()).find((t) => t.includes("F9 harness"));
    check("the posted comment renders in the discussion", !!rendered, rendered ?? "not found");
    check(
      "it renders the name, not the token markup",
      !!rendered && rendered.includes(`@${myName}`) && !rendered.includes("@["),
      rendered ?? ""
    );

    // Being named yourself is highlighted.
    const ownMention = page.locator("span.tw-bg-brass\\/20", { hasText: `@${myName}` });
    check(
      "a mention of the reader is highlighted",
      (await ownMention.count()) > 0,
      `${await ownMention.count()} span(s)`
    );
    const bg = await page
      .locator("span", { hasText: `@${myName}` })
      .last()
      .evaluate((el) => getComputedStyle(el).backgroundColor)
      .catch(() => "n/a");
    check(
      "the highlight class survived the Tailwind purge (a real background)",
      bg !== "rgba(0, 0, 0, 0)" && bg !== "transparent" && bg !== "n/a",
      bg
    );

    // ============ 7. Thread header flattening + the reply composer ============
    // A reply composer only exists inside a tagged thread, so tag the comment
    // we just posted (which starts with a mention — exactly the case whose
    // header would otherwise be mostly token markup).
    if (posted) {
      const tagged = await api("POST", `${E}/comments/${posted._id}/thread`, {
        membersOnly: false,
      });
      check("the mention comment can be tagged as a thread", tagged.status === 200, `${tagged.status}`);

      await page.reload({ waitUntil: "networkidle" });
      await page.waitForTimeout(2500);

      const header = page.locator("span.tw-font-medium", { hasText: "F9 harness" }).first();
      const headerText = (await header.count()) > 0 ? await header.innerText() : "";
      check(
        "the thread header flattens the token to @Name",
        headerText.includes(`@${myName}`) && !headerText.includes("@["),
        headerText || "header not found"
      );

      // Click the thread header row itself — the bare chevron locator also
      // matches collapsed chevrons elsewhere on the page, which are hidden.
      await page.locator("div.tw-cursor-pointer", { hasText: "F9 harness" }).first().click();
      await page.waitForTimeout(1200);
      const reply = page.locator('textarea[placeholder*="Reply"]').first();
      if ((await reply.count()) > 0) {
        await reply.click();
        await reply.type("@" + firstName);
        await page.waitForTimeout(800);
        check("the reply composer opens a picker too", (await picker.count()) > 0);
        await reply.press("Escape");
        await reply.fill("");
      } else {
        check("the reply composer is reachable", false, "no Reply textarea found");
      }
    }

    // ============ 8. No raw markup leaks anywhere ============
    check(
      "no raw token markup leaks anywhere on the page",
      !(await page.locator("body").innerText()).includes("@["),
      "found '@[' in rendered text"
    );

    await page.screenshot({ path: `${SHOT_DIR}/f9_desktop.png`, fullPage: false });

    // ============ 9. Mobile ============
    const mctx = await browser.newContext({
      storageState: STATE,
      viewport: { width: 390, height: 844 },
      isMobile: true,
      hasTouch: true,
    });
    const mobile = await mctx.newPage();
    await mobile.goto(`${BASE}/e/${eventId}`, { waitUntil: "networkidle" });
    await mobile.waitForTimeout(2500);
    const mComposer = mobile.locator('textarea[placeholder*="Add a message"]').first();
    if ((await mComposer.count()) > 0) {
      await mComposer.click();
      await mComposer.type("@" + firstName.slice(0, 2));
      await mobile.waitForTimeout(800);
      check(
        "mobile: the picker opens under the composer",
        (await mobile.locator("div.tw-absolute.tw-z-20").count()) > 0
      );
    } else {
      check("mobile: the composer renders", false, "not found");
    }
    check(
      "mobile: no horizontal overflow",
      await mobile.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth + 1
      )
    );
    await mobile.screenshot({ path: `${SHOT_DIR}/f9_mobile.png`, fullPage: false });

    check(
      "no console errors throughout",
      consoleErrors.length === 0,
      consoleErrors.slice(0, 3).join(" | ")
    );
  } catch (err) {
    check(`harness error: ${err.message}`, false);
  } finally {
    // ============ 10. Clean up the throwaway gathering ============
    if (page && eventId) {
      try {
        const gone = await api("DELETE", `/api/events/${eventId}`);
        const after = await api("GET", `/api/events/${eventId}`);
        check(
          "the throwaway gathering was deleted from production",
          gone.status === 200 && after.status !== 200,
          `delete=${gone.status} refetch=${after.status}`
        );
      } catch (e) {
        check(`CLEANUP FAILED — remove ${TAG} by hand: ${e.message}`, false);
      }
    }
    await browser.close();
  }

  const failed = results.filter((r) => !r.ok);
  console.log(`\n${results.length - failed.length}/${results.length} checks passed`);
  process.exit(failed.length ? 1 : 0);
})();
