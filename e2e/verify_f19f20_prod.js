// Hand-verify My Lists (F19) and My Notes (F20) against the DEPLOYED instance.
//
// Runs as ONE real member against a REAL gathering, so it is deliberately
// conservative: it writes only into that member's own private documents — which
// nobody else can see by construction — and removes everything it created
// before it exits. It never touches the shared lists or the discussion.
//
// ASSERT-ONLY OUTPUT. This runs against real member data, so nothing read from
// the page is ever printed: no names, no emails, no gathering titles. Every
// check prints its own label and a pass/fail, never the value it looked at.
//
//   TIMEFUL_VM=root@host node prod_login.js you@example.com   # once, for the session
//   node verify_f19f20_prod.js
const { chromium } = require("playwright");

const BASE = process.env.TIMEFUL_BASE || "https://gathering.sirthomasfoolery.com";
const MARKER = `f19-check-${Date.now()}`; // unique, so cleanup can't hit anything real

let pass = 0;
const failures = [];
const ok = (label, cond) => {
  if (cond) {
    pass++;
    console.log(`  PASS  ${label}`);
  } else {
    failures.push(label);
    console.log(`  FAIL  ${label}`);
  }
};

// The event page never reports "stable" to Playwright, so dispatch clicks on
// the element rather than going through actionability checks.
const domClick = (locator) => locator.evaluate((el) => el.click());
const tab = (page, label) => page.locator(`button:has-text("${label}")`).first();

// EVERY control lookup must be :visible-scoped. All four band panels are
// mounted at once (v-show, not v-if), so a bare .first()/.last() can resolve to
// the SHARED panel's hidden "Add list" field — which on a gathering you own
// means typing into the club's lists instead of your own. Found the hard way
// against production; locally it hid, because the guest account under test had
// no shared "Add list" form to collide with.
const visible = (page, selector) => page.locator(`${selector}:visible`);

(async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    storageState: `${__dirname}/prod_state.json`,
  });
  const page = await ctx.newPage();

  const errors = [];
  page.on("console", (m) => m.type() === "error" && errors.push(m.text()));

  // Pick a gathering from the member's own dashboard, via the API rather than
  // by scraping — and never print which one.
  await page.goto(`${BASE}/home`, { waitUntil: "networkidle" });
  const eventId = await page.evaluate(async () => {
    const r = await fetch("/api/user/events", { credentials: "include" });
    if (!r.ok) return null;
    const events = await r.json();
    const list = Array.isArray(events) ? events : Object.values(events ?? {});
    const live = list.filter((e) => e && !e.isDeleted);
    return live.length ? live[0].shortId ?? live[0]._id : null;
  });
  if (!eventId) throw new Error("no gathering available on this account to test against");
  console.log("  (using one of this account's own gatherings)\n");

  await page.goto(`${BASE}/e/${eventId}`, { waitUntil: "networkidle" });
  await page.waitForTimeout(2500);

  console.log("1. The tabs");
  ok("My Lists tab present", await tab(page, "My Lists").isVisible());
  ok("My Notes tab present", await tab(page, "My Notes").isVisible());
  ok("Discussion tab still present", await tab(page, "Discussion").isVisible());
  ok("shared Lists tab still present", await tab(page, "Lists").isVisible());

  console.log("\n2. A private list, created and read back");
  await domClick(tab(page, "My Lists"));
  await page.waitForTimeout(1800);
  ok("the private panel renders", (await page.locator("body").innerText()).includes("My Lists"));
  // The shared panel's refresh button is mounted-but-hidden, so this must ask
  // about VISIBLE ones — the private panel is handed collaborative:false and
  // renders none.
  ok(
    "the private panel offers no refresh control",
    (await visible(page, 'button[title="Refresh lists"]').count()) === 0
  );

  await domClick(visible(page, 'button:has-text("Add list")').first());
  await page.waitForTimeout(600);
  await visible(page, 'input[type="text"]').last().fill(MARKER);
  await domClick(visible(page, 'button:has-text("Create list")').first());
  await page.waitForTimeout(2500);
  ok("the list was created", (await page.locator("body").innerText()).includes(MARKER));

  const composer = visible(page, 'input[placeholder*="Add"], input[placeholder*="add"]').last();
  await composer.fill(`${MARKER}-entry`);
  await composer.press("Enter");
  await page.waitForTimeout(2500);
  ok("an entry was added", (await page.locator("body").innerText()).includes(`${MARKER}-entry`));

  console.log("\n3. Isolation, checked at the API");
  const isolation = await page.evaluate(async (id) => {
    const mine = await (await fetch(`/api/events/${id}/my-lists`, { credentials: "include" })).json();
    const shared = await (await fetch(`/api/events/${id}/lists`, { credentials: "include" })).json();
    const event = await (await fetch(`/api/events/${id}`, { credentials: "include" })).text();
    return {
      mineCount: Array.isArray(mine) ? mine.length : -1,
      sharedIsArray: Array.isArray(shared),
      eventBody: event,
    };
  }, eventId);
  ok("the private list is returned to its owner", isolation.mineCount >= 1);
  ok("the shared lists endpoint still answers", isolation.sharedIsArray);
  // The headline guarantee: private text must not ride out on the event.
  ok("the private list does NOT appear on the event document", !isolation.eventBody.includes(MARKER));

  console.log("\n4. Notes");
  await domClick(tab(page, "My Notes"));
  await page.waitForTimeout(2000);
  const ta = visible(page, "textarea").last();
  const preExisting = await ta.inputValue(); // never printed — may be a real note
  ok("the notes editor renders", (await ta.count()) > 0);

  // Only write if there is nothing there. A real note is never overwritten.
  const noteSafeToWrite = preExisting.trim() === "";
  if (noteSafeToWrite) {
    await ta.fill(`## ${MARKER}\n\n- **bold** entry\n<script>alert(1)</script>`);
    await page.waitForTimeout(600);
    await domClick(visible(page, 'button:has-text("Preview")').first());
    await page.waitForTimeout(1200);
    const html = await visible(page, ".flw-prose").last().innerHTML();
    ok("markdown renders as html", /<h2>/.test(html) && /<strong>/.test(html));
    ok("a script tag renders inert", !/<script/i.test(html));
    ok(
      "the toolbar is hidden in preview",
      (await visible(page, 'button[title="Bold"]').count()) === 0
    );
    await domClick(visible(page, 'button:has-text("Write")').first());
    await page.waitForTimeout(400);
    const saveBtn = visible(page, 'button:has-text("Save")').last();
    if (await saveBtn.isEnabled()) await domClick(saveBtn);
    await page.waitForTimeout(2500);
    ok("the note reports a save time", /Saved \d\d:\d\d/.test(await page.locator("body").innerText()));
  } else {
    console.log("  SKIP  note write — this account already has a real note here, left untouched");
  }

  console.log("\n5. Cleanup");
  const cleaned = await page.evaluate(
    async ({ id, marker, clearNote }) => {
      const lists = await (await fetch(`/api/events/${id}/my-lists`, { credentials: "include" })).json();
      let removed = 0;
      for (const l of lists) {
        // ONLY lists this run created — matched on the unique marker.
        if (l.name === marker) {
          const r = await fetch(`/api/events/${id}/my-lists/${l._id}`, {
            method: "DELETE",
            credentials: "include",
          });
          if (r.ok) removed++;
        }
      }
      if (clearNote) {
        await fetch(`/api/events/${id}/my-notes`, {
          method: "PUT",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ text: "" }),
        });
      }
      const after = await (await fetch(`/api/events/${id}/my-lists`, { credentials: "include" })).json();
      const note = await (await fetch(`/api/events/${id}/my-notes`, { credentials: "include" })).json();
      return {
        removed,
        leftovers: after.filter((l) => l.name === marker).length,
        noteHasMarker: (note.text ?? "").includes(marker),
      };
    },
    { id: eventId, marker: MARKER, clearNote: noteSafeToWrite }
  );
  ok("the test list was removed", cleaned.removed === 1 && cleaned.leftovers === 0);
  ok("no test text left in the note", !cleaned.noteHasMarker);

  ok("no console errors", errors.length === 0);

  await browser.close();
  console.log(`\n${pass} passed, ${failures.length} failed`);
  if (failures.length) {
    console.log("FAILURES:");
    failures.forEach((f) => console.log(" - " + f));
    process.exit(1);
  }
})().catch((e) => {
  console.error("HARNESS ERROR:", e.message);
  process.exit(2);
});
