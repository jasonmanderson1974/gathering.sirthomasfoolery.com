// Log into a deployed instance via email OTP and save the browser session to
// prod_state.json, which every verify_*.js here reuses.
//
// The OTP is read straight out of the deployment's Mongo over SSH — the mail
// round trip isn't scriptable. That needs an SSH host, which is deliberately
// NOT hardcoded (this repo is public): set TIMEFUL_VM.
//
//   TIMEFUL_VM=root@host node prod_login.js [email]
//
// The credentials are never passed in: Mongo requires auth, and the connection
// string already exists on the host as MONGODB_URI in /etc/thegathering/env.
// Source it there rather than threading a password through argv (where it would
// land in this box's shell history and the remote's process list).
// That file is root-readable only, so TIMEFUL_VM needs to be a root login.
//
// prod_state.json holds a live session cookie and is gitignored. Don't commit it.
const { chromium } = require("playwright");
const { execSync } = require("child_process");

const EMAIL = process.argv[2] || process.env.TIMEFUL_EMAIL;
const BASE = process.env.TIMEFUL_BASE || "https://gathering.sirthomasfoolery.com";
const VM = process.env.TIMEFUL_VM;
const ENV_FILE = process.env.TIMEFUL_ENV_FILE || "/etc/thegathering/env";

if (!VM) {
  console.error(
    "Set TIMEFUL_VM=root@host (the box running the deployment) — the OTP is\n" +
      "read from that host's Mongo. Optionally TIMEFUL_ENV_FILE, TIMEFUL_BASE."
  );
  process.exit(2);
}
if (!EMAIL) {
  console.error("Pass the member's email as argv[1], or set TIMEFUL_EMAIL.");
  process.exit(2);
}

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
  await page.goto(`${BASE}/sign-in`, { waitUntil: "networkidle" });

  await page.fill('input[type="email"]', EMAIL);
  await page.keyboard.press("Enter");
  await page.waitForSelector('input[placeholder="Enter 6-digit code..."]', {
    timeout: 15000,
  });
  await page.waitForTimeout(1500);

  const code = execSync(
    `ssh ${VM} 'set -a; . ${ENV_FILE}; set +a; ` +
      `mongosh --quiet "$MONGODB_URI" --eval ` +
      `"db.otpCodes.find({email: \\"${EMAIL}\\"}).sort({_id:-1}).limit(1).toArray()[0].code"'`
  )
    .toString()
    .trim();
  if (!/^\d{6}$/.test(code)) throw new Error(`bad OTP from mongo: ${code}`);

  await page.fill('input[placeholder="Enter 6-digit code..."]', code);
  await page.keyboard.press("Enter");
  await page.waitForURL((u) => !u.pathname.startsWith("/sign-in"), {
    timeout: 15000,
  });
  await page.context().storageState({ path: `${__dirname}/prod_state.json` });
  console.log("logged in, landed on:", page.url());
  await browser.close();
})();
