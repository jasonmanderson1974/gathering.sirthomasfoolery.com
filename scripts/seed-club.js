/*
 * The club fixture's Mongo half: five members, their allowlist rows, and ten
 * Chronicle entries. Run through mongosh by `scripts/seed-club.sh`, which then
 * creates the gatherings over the API. Prints two user ids on stdout.
 *
 * WHY THIS IS A FILE (TODO3 M4): it used to be a heredoc inside
 * `scripts/browser-check.sh`, in a stack that is deleted at the end of every
 * run — so the one populated club this repo knows how to build existed for
 * three minutes at a time and nothing else could use it. Meanwhile the dev
 * stack on :3002 held 0 users, 0 events and 0 allowlist rows, which is why
 * "bring up the app and look at it" was a five-minute manual setup nobody did.
 * Now `scripts/dev-up.sh --seed` builds the same club CI asserts against, and
 * the interactive stack cannot drift away from the checked one.
 *
 * THE FIXTURE IS POPULATED ON PURPOSE, and that is the whole point of its
 * shape. An empty Fellowship renders its one row and an empty Chronicle renders
 * its "nothing recorded yet" line — both perfectly, and neither exercises the
 * entry rendering, which is the code most likely to break. check-routes'
 * thresholds were written against a real database; seeding to match keeps them
 * meaningful instead of tuning them down to whatever an empty database happens
 * to produce.
 *
 * Two knobs, each readable either as a mongosh env var or as a global that
 * seed-club.sh prepends. Both, because mongosh runs on the HOST for a dev stack
 * (where the environment is simply inherited) and inside the CONTAINER for the
 * browser check (where it is not, and `docker compose exec` would need a `-e`
 * this script has no way to add to a caller-supplied command line):
 *
 *   SEED_FORCE=1   remove a previous seed first, instead of refusing
 *   SEED_ADDED_BY  the `addedBy` stamped on allowlist rows (default "seed-club")
 */

const SEED_MARKER = "seed-club"
const env = (typeof process !== "undefined" && process.env) || {}
const force =
  String(typeof SEED_FORCE !== "undefined" ? SEED_FORCE : env.SEED_FORCE) === "1"
const addedBy =
  (typeof SEED_ADDED_BY !== "undefined" ? SEED_ADDED_BY : env.SEED_ADDED_BY) ||
  SEED_MARKER

// The signed-in user is a superAdmin because /members is gated on `canInvite`,
// and a lesser role is redirected to /home — which surfaces as the route
// failing rather than as a seeding mistake. The allowlist rows matter too:
// AuthRequired enforces the roll on every request, not just at sign-in.
const members = [
  ["harness@example.test", "Harness", "Check", "superAdmin"],
  ["ambrose@example.test", "Ambrose", "Fenwick", "admin"],
  ["cornelius@example.test", "Cornelius", "Blackwood", "member"],
  ["percival@example.test", "Percival", "Thorne", "member"],
  ["reginald@example.test", "Reginald", "Ashcombe", "member"],
]
const emails = members.map(([email]) => email)

/*
 * Seeding twice fails on the allowlist's unique email index, and reports as a
 * duplicate-key error rather than as "you already have a club in here" — which
 * is a bad five minutes the first time you meet it. browser-check.sh never hits
 * this (it starts from an empty volume every run); dev-up.sh does, every time
 * after the first.
 *
 * The cleanup is scoped to documents this file can PROVE it created: the five
 * @example.test users, their allowlist rows, everything owned by those user ids,
 * and chronicle entries carrying the marker below. The dev stack on these
 * machines habitually holds a restored production dump, so a broad delete here
 * would be a genuinely expensive mistake.
 */
const existing = db.users.find({ email: { $in: emails } }, { _id: 1 }).toArray()
if (existing.length > 0) {
  if (!force) {
    print(
      "SEED_ERROR the club is already seeded (" +
        existing.length +
        " fixture users). Re-run with SEED_FORCE=1, or " +
        "`scripts/dev-up.sh --seed --force`, to replace it."
    )
    quit(1)
  }
  const stale = existing.map((u) => u._id)
  db.events.deleteMany({ ownerId: { $in: stale } })
  db.eventResponses.deleteMany({ userId: { $in: stale } })
  db.folderEvents.deleteMany({ userId: { $in: stale } })
  db.folders.deleteMany({ userId: { $in: stale } })
  db.allowlist.deleteMany({ email: { $in: emails } })
  db.users.deleteMany({ _id: { $in: stale } })
  db.chronicle.deleteMany({ seededBy: SEED_MARKER })
}

const ids = []
for (const [email, firstName, lastName, role] of members) {
  const uid = new ObjectId()
  ids.push(uid)
  db.users.insertOne({
    _id: uid,
    email,
    firstName,
    lastName,
    role,
    phone: "+15550100",
    timezoneOffset: 0,
  })
  db.allowlist.insertOne({ email, addedBy, role, addedAt: new Date() })
}

// Chronicle entries are inserted directly rather than produced by letting the
// reminder scheduler archive a past gathering: that path ticks on its own
// timer, which is not something a CI run should be waiting on. The cost is a
// hand-built fixture — if ChronicleEntry's shape ever moves, this renders wrong
// and reads like a regression, so check the seed before the app.
const day = 24 * 60 * 60 * 1000
for (let i = 1; i <= 10; i++) {
  const start = new Date(Date.now() - i * 30 * day)
  db.chronicle.insertOne({
    eventId: new ObjectId(),
    name: "Gathering the " + i,
    location: "The Club Room",
    startDate: start,
    endDate: new Date(start.getTime() + 3 * 60 * 60 * 1000),
    attendees: [
      { name: "Ambrose Fenwick", status: "going", guestCount: 1 },
      { name: "Cornelius Blackwood", status: "going", guestCount: 0 },
      { name: "Percival Thorne", status: "maybe", guestCount: 0 },
    ],
    headCount: 4,
    capturedAt: new Date(),
    // Only so the re-seed above can find exactly these again. The app never
    // reads it.
    seededBy: SEED_MARKER,
  })
}

// The first id is who the caller signs in as; the second is a fellow member,
// who exists so a gathering can have a response by somebody OTHER than the
// signed-in user. Both halves matter: "Schedule event" needs numResponses > 0,
// and the "Mark availability" assertion needs the signed-in user to be the one
// who has NOT responded (it reads "Edit availability" once they have).
print(ids[0].toString() + " " + ids[1].toString())
