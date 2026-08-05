// Confirm the My Notes autosave goes quiet on the DEPLOYED instance.
//
// Walks the sequence that surfaced the loop — write, format, preview, edit,
// preview — then sits idle and counts PUTs. Writes only into the signed-in
// member's own private note, and only when that note is empty, so a real note
// is never touched. Restores it to empty on the way out.
//
// ASSERT-ONLY OUTPUT: nothing read from the page is printed.
//
//   TIMEFUL_VM=root@host node prod_login.js you@example.com   # once
//   node verify_notes_quiet_prod.js
const { chromium } = require("playwright");

const BASE = process.env.TIMEFUL_BASE || "https://gathering.sirthomasfoolery.com";
const IDLE_MS = 12000;

const domClick = (locator) => locator.evaluate((el) => el.click());
const visible = (page, selector) => page.locator(`${selector}:visible`);

(async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    storageState: `${__dirname}/prod_state.json`,
  });
  const page = await ctx.newPage();

  await page.goto(`${BASE}/home`, { waitUntil: "networkidle" });
  // Find a gathering whose note is EMPTY. Real notes are never written over,
  // and the account running this may well have one — the loop this checks for
  // is exactly the sort of thing you notice while writing a real note.
  const eventId = await page.evaluate(async () => {
    const r = await fetch("/api/user/events", { credentials: "include" });
    if (!r.ok) return null;
    const events = await r.json();
    const list = Array.isArray(events) ? events : Object.values(events ?? {});
    for (const e of list.filter((x) => x && !x.isDeleted)) {
      const id = e.shortId ?? e._id;
      const note = await (
        await fetch(`/api/events/${id}/my-notes`, { credentials: "include" })
      ).json();
      if ((note.text ?? "").trim() === "") return id;
    }
    return null;
  });
  if (!eventId) {
    console.log("SKIP — every gathering on this account already has a note; none touched");
    await browser.close();
    return;
  }

  await page.goto(`${BASE}/e/${eventId}`, { waitUntil: "networkidle" });
  await page.waitForTimeout(2500);
  await domClick(page.locator('button:has-text("My Notes")').first());
  await page.waitForTimeout(2000);

  const ta = visible(page, "textarea").last();
  if ((await ta.inputValue()).trim() !== "") {
    console.log("SKIP — this account already has a real note here; left untouched");
    await browser.close();
    return;
  }

  let puts = 0;
  page.on("request", (r) => {
    if (r.method() === "PUT" && r.url().includes("/my-notes")) puts++;
  });

  await ta.fill("quiet check");
  await page.waitForTimeout(400);
  // The divider leaves a trailing newline — the input that caused the loop.
  await ta.evaluate((el) => el.setSelectionRange(el.value.length, el.value.length));
  await domClick(visible(page, 'button[title="Divider"]').first());
  await page.waitForTimeout(500);
  await domClick(visible(page, 'button:has-text("Preview")').first());
  await page.waitForTimeout(1200);
  await domClick(visible(page, 'button:has-text("Write")').first());
  await page.waitForTimeout(500);
  await ta.fill((await ta.inputValue()) + "\nand more\n");
  await page.waitForTimeout(500);
  await domClick(visible(page, 'button:has-text("Preview")').first());

  await page.waitForTimeout(4000); // let the real save land
  const saved = /Saved \d\d:\d\d/.test(await page.locator("body").innerText());
  console.log(saved ? "  PASS  the edit saved" : "  FAIL  the edit did not save");

  puts = 0;
  await page.waitForTimeout(IDLE_MS);
  const quiet = puts === 0;
  console.log(
    quiet
      ? `  PASS  no saves while idle (0 PUTs in ${IDLE_MS / 1000}s)`
      : `  FAIL  autosave still looping (${puts} PUTs in ${IDLE_MS / 1000}s)`
  );

  // Put the note back the way it was found.
  await page.evaluate(async (id) => {
    await fetch(`/api/events/${id}/my-notes`, {
      method: "PUT",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text: "" }),
    });
  }, eventId);
  console.log("  (note restored to empty)");

  await browser.close();
  if (!saved || !quiet) process.exit(1);
})().catch((e) => {
  console.error("HARNESS ERROR:", e.message);
  process.exit(2);
});
