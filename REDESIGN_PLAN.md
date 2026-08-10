# The Fellowship — Redesign Plan

> **⚠️ HISTORICAL — completed 2026-06-20, last updated then. Not a live plan.**
> The redesign shipped and the theme it describes is the app's current look, so
> the design system in §2 and the gotchas in §5 are still worth reading. Do not
> trust anything here about *process or infrastructure* — three things below are
> now wrong: the work is on **`main`**, not `redesign/fellowship-phase1`;
> production does **not** track a branch (a release is rsynced to it — see
> `DEPLOYMENT.md`); and the Go module was renamed to **`sirtom/server`** on
> 2026-07-23, reversing the "internal identifiers STAY" decision in §1. Only the
> Mongo DB name (`schej-it`) kept the old branding. `CLAUDE.md` is authoritative.

Plan for rebranding/redesigning the self-hosted Timeful instance into
**The Fellowship / The Gathering** (a vintage gentleman's-club theme for a small
men's event-planning club).

> Resume tip: read this top-to-bottom, then check **Current state** for exactly
> where we left off. Design source of truth: `samples/the-gathering.html` + `.css`.

---

## Parking Lot / Captured asides
Items the user dropped as "btw ..." asides (see [[feedback-btw-asides-are-todos]]).
Acted-on items get checked off; open ones get surfaced at the next checkpoint.

Captured 2026-06-20 (user-reported during review — for future sessions):
- [x] **Remove all front-end "Schej" references.** DONE 2026-06-20 (commit deployed):
      deleted FormerlyKnownAs/PronunciationMenu + schej_logo*/schej_character/schejie
      assets + schej_pronunciation.mp3; schejie mascot → crest on AccessDenied/PageNotFound;
      dropped unused redditComments data + dead Rive schej.riv. Backend schej.it/schej-it kept. User-facing only — leave backend/internal
      identifiers (Go module `schej.it`, Mongo `schej-it`) per repo convention. Covers:
      `schej_logo*`/`schej_character.png` assets, any "Schej"/"formerly Schej" copy,
      `FormerlyKnownAs.vue` (already removed from Landing+Event templates; delete component
      + remaining imports), alt text, the index.html title still has "(formerly Schej)"? (no —
      already "The Fellowship · The Gathering"). Grep `-i schej` under `frontend/src` + `public`.
- [x] **Remove paywall items (front-end).** DONE 2026-06-20 (deployed): deleted pricing/
      (UpgradeDialog + AlreadyDonated/StudentProof), StripeRedirect (+route), Test.vue (+route);
      removed createNew paywall gate, App Premium badge + UpgradeDialog, Dashboard + EventType
      free-events/upgrade UI, Settings billing portal, and the orphaned store plumbing
      (enablePaywall, upgradeDialog*, show/hideUpgradeDialog). isPremiumUser getter kept (inert).
- [x] **Remove the Reddit upvote message** DONE 2026-06-20 (deployed): deleted
      `UpvoteRedditSnackbar.vue` + its `App.vue` usage.
- [x] **Login bug: Google/Outlook buttons white-on-white — FIXED 2026-06-20.** Added global
      `.v-application .v-btn.tw-bg-white { color: #1c1410 }` in index.css → dark labels on all
      white brand buttons (login + Apple/ICS autofill). Deployed & verified on /sign-in.
- [x] **/home: grey backgrounds wash out text** DONE 2026-06-20: around the **Dashboard** and **Tools** sections.
      Currently grey (likely a leftover Vuetify default / unconverted surface) → low contrast on
      the light text. Re-tone to wood/leather (or transparent over the damask) so text reads.
      Look in `views/Home.vue` + `components/home/Dashboard.vue` (and any section wrapper/cards
      for the Tools row) for grey surfaces — possibly Vuetify `theme--dark` default surface
      (rgb 30,30,30) or a missed `tw-bg-*` / explicit background. Logged-in review finding.

Captured 2026-06-20 (batch 2):
- [x] **Remove YouTube videos.** DONE 2026-06-20: The landing "The proceedings, in motion" section
      (`Landing.vue` ~154-173, the `youtube.com/embed/...` iframe) AND the `HowItWorksDialog.vue`
      YouTube embed (and any other YouTube embeds — grep `youtube` under `frontend/src`).
- [x] **Remove these links:** DONE 2026-06-20 (standalone links; inline sign-in 'agree to privacy policy' kept): "Privacy Policy" (footer `Footer.vue`; also the sign-in
      "agree to our privacy policy" link in `SignIn.vue`/`SignInDialog.vue` — confirm scope),
      "The Manner of It" (landing nav → opens HowItWorksDialog) and "The Chronicle" (landing
      nav → `/blog`), both in `Landing.vue` header nav (and the `LandingPageHeader` mobile menu).
- [x] **Remove landing hero text** DONE 2026-06-20: "No dues · no login required." (`Landing.vue` hero, under the
      "Call a Gathering" button).
- [x] **Site icon** DONE 2026-06-20 (regenerated binary favicons from crest + cache-bust ?v=2): (confirmed by user across cleared cache +
      different browser/computer). Cause: only `favicon.svg` was rebranded; browsers/bookmarks
      still fetch the binary `public/favicon.ico` + `favicon-16x16.png` + `favicon-32x32.png`,
      which are the old green Timeful calendar. FIX: regenerate those from the crest — render
      PNGs at 16/32/180/512 via Playwright (same trick as the OG image, `/tmp/shot`), build a
      real multi-size `favicon.ico` (needs ImageMagick/png-to-ico — not on dev box, may need a
      tool/online or `npx png-to-ico`), add apple-touch-icon, and bump references in index.html.
      Also Cloudflare may cache favicons hard — purge/cache-bust (e.g. `favicon.ico?v=2`).

## Next-session discussion (not a code task yet)
- **Restrict sign-up / access to an allowlist (invite-only, NOT public).** User has a list of
  **names, email addresses, and phone numbers**; members added infrequently but manually; the
  site must not be open to the general public. Need to design this next session — see
  [[project-fellowship-access-control]]. Covers: Google OAuth sign-in + the email/OTP path,
  and guest (no-login) event responses. Decide allowlist storage + matching key (email most
  reliable; phone for SMS-OTP?), admin flow to add members, and what unauthorized users see.

---

## 1. Goal & scope (decided 2026-06-20)

- **Whole-app** re-skin — every screen, not just the landing.
- **Rebrand the UI** to The Fellowship + the Sir Thomas Foolery hare crest.
  ~~Internal code identifiers (Go module `schej.it`, Mongo `schej-it`) STAY.~~
  **Half-reversed 2026-07-23:** the Go module became `sirtom/server`. Only the
  Mongo DB name `schej-it` and `SCHEJ_EMAIL_ADDRESS` still carry the old
  branding, and renaming the database is parked (TODO2 G4) as a data migration
  with no user-facing benefit.
- **Full archaic "gentleman's club" voice** across all copy (buttons, labels,
  empty states, errors), while keeping things usable.
- Mascot: **Sir Thomas Foolery** = "Eric the Hare" (Eric + Eloise collection)
  with monocle + handlebar mustache.

## 2. Design system (source of truth = `samples/`)

**Palette** (now in `frontend/tailwind.config.js` as Tailwind tokens, prefix `tw-`):

| token | hex | use |
|---|---|---|
| `wood-deep` | #1c1410 | page base |
| `wood` | #241a13 | panels |
| `leather` | #2e2117 | raised surfaces |
| `green-felt` | #16261d | billiard-green accent |
| `green-deep` | #0f1c15 | |
| `brass` | #c9a44c | primary gold |
| `brass-bright` | #e3c578 | highlight |
| `brass-dim` | #8a7333 | hairlines/borders |
| `parchment` | #ede4d3 | primary text |
| `parchment-dim` | #b8ad97 | secondary text |
| `oxblood` | #6e2b2b | danger |

**Fonts** (loaded in `frontend/public/index.html`; Tailwind families):
`tw-font-display` = Cinzel (spaced ALL-CAPS titles), `tw-font-head` = Cormorant
Garamond (ornamental italic heads), `tw-font-body` = EB Garamond (running text).

**Reusable classes** (in `frontend/src/index.css`, `@layer components`):
`.flw-panel` (brass-keyline wood card), `.flw-btn` (brass CTA), `.flw-rule`
(centered ornamental rule — wrap a `<span>◆</span>`), `.flw-eyebrow` (spaced
all-caps brass label), `.flw-title` (display title), `.flw-sub` (italic lead).

**Crest component:** `frontend/src/components/general/SirThomasFoolery.vue`
(`:size` prop, unique gradient id per instance).

**Motifs:** damask diamond lattice + walnut background (global, on `.v-application`
in `index.css`), brass keyline frames, ◆ diamond rule dividers.

**Voice examples:** "Cast thy vote, good sir — when shall we convene?",
"The Manner of It" (How it works), "The Chronicle" (Blog), "Enter" (Sign in),
"Call a Gathering" (Create event), "To the Club Room" (Open dashboard),
"No dues · no login required."

## 3. Current state (updated 2026-06-20)

**The redesign is FUNCTIONALLY COMPLETE and deployed** on branch
`redesign/fellowship-phase1` (VM tracks this branch; live at gathering.sirthomasfoolery.com).
The whole app is on the dark Fellowship theme — landing, event/availability page, create
dialogs, sign-in/auth, the logged-in app (dashboard/groups/friends/settings), sub-dialogs,
plus meta title/description, SVG+binary crest favicons, and the OG link-share image.

- **All Parking-Lot review items (1–8) done** (Schej refs, paywall, Reddit snackbar, login
  button contrast, /home grey bg, YouTube, links, hero "No dues", site icon).
- **Additional user-requested removals done:** the "Send Word" feedback button, the /home
  **Tools** section (+ When2meet/Timeful import dialogs), and the boba "calendar sipping a
  drink" animation. Hero product-demo media removed (no on-theme replacement wanted).

**Invite-only access control:** separate effort, see **`ACCESS_CONTROL_PLAN.md`**. **Phase A
DONE & deployed** (allowlist + sign-in gate, fail-open while empty ⇒ currently dormant).
Phases B–E pending (B = Gmail SMTP for the OTP email).

**Branch/VM state:** VM is on `redesign/fellowship-phase1`, **NOT `main` — not yet merged**.
Finalize = merge to `main` + `git checkout main` on the VM + redeploy, when ready.

## 4. Phased task list

### Phase 1 — Foundation + Landing + global chrome  *(mostly done; deployed)*
- [x] Fonts, palette, Vuetify dark theme, global CSS, `flw-*` classes, crest.
- [x] Landing header + hero.
- [x] Landing: How-it-works section (archaic steps; crest replaces `schej_character.png`).
- [x] Landing: Reddit testimonials block — REMOVED (external SaaS social proof, N/A for club).
- [x] Landing: `FAQ.vue` (dark/brass panels) + rebranded/archaic FAQ + how-it-works copy.
- [x] `Footer.vue` → club colophon (crest, THE FELLOWSHIP, Sir Thomas Foolery, Privacy).
- [x] Global chrome in `App.vue`: crest + wordmark header, dark wood bar, archaic buttons.
- [x] Deployed to VM (commit `9c61b34`); landing reads coherently end-to-end.
- [x] `NumberBullet.vue` — already brass (`tw-bg-brass tw-text-wood-deep`); item was stale (2026-07-12).
- [x] Hero demo media — removed earlier (no on-theme replacement wanted). 2026-07-12: deleted the
      orphaned dead imports/registrations it left behind in `Landing.vue` (`LandingPageCalendar`,
      `vueVimeoPlayer`, `Header`, `NewEvent`, `GithubButton`, `Logo`) + dead data/method
      (`githubSnackbar`, `isVideoPlaying`, `onPlay`).
- [x] `LandingPageCalendar.vue` — was imported but unused everywhere; DELETED (2026-07-12).
- [x] `HowItWorksDialog.vue` — already deleted; item was stale (2026-07-12).

### Phase 2 — Event / availability page (core guest screen)  *(in progress; deployed)*
- [x] `src/views/Event.vue` shell: parchment title/text, brass accents+buttons,
      leather chips; removed FormerlyKnownAs. Verified on live test event `/e/FE2dd`.
- [x] `ScheduleOverlap.vue` chrome: parchment text, brass accents, dark wood
      day-header strips, leather surfaces. The EMPTY grid reads great on dark.
- [x] Heatmap CELL colors VERIFIED OK on dark (checked in edit mode on `/e/FE2dd`):
      green `#00994C` "available" reads well on dark; red→maroon "unavailable" lands
      on-theme (oxblood-ish); single `#00994C88` visible. NO remap needed. The
      Available/If-needed toggle + legend panel already theme correctly.
- [x] Archaic copy pass on Event.vue (2026-07-12): "Add availability"→"Mark availability"
      (echoes landing/FAQ "mark his availability"; "Edit availability" kept — FAQ cites it
      literally); doc title already `· The Fellowship`. Also rebranded the two `ScheduleOverlap`
      calendar-export strings "scheduled with Timeful: https://timeful.app/e/" →
      "called with The Fellowship: ${window.location.origin}/e/" (also fixes the invite linking
      members to the wrong host).
- [x] `RespondentsList`/`MarkAvailabilityDialog`/`GuestDialog`/`SignUpForSlotDialog`/pickers/`ToolRow`
      already themed by the Phase-5 sweep. 2026-07-12: fixed the two remaining off-theme blue
      surfaces — the `ToolRow` "Schedule" confirm button and the on-grid scheduled-event preview
      block (`tw-bg-blue tw-text-white` → brass on `tw-text-wood-deep`).
      `AvailabilityTypeToggle` green left intentionally (semantic, matches grid).
- [ ] Test event for review: **`/e/FE2dd`** ("The Inaugural Gathering").

### Phase 3 — Auth + creation flows  *(create dialogs done; deployed)*
- [x] RESOLVED the "dialogs render light" red herring: dialogs were ALWAYS dark via
      the theme (`theme--dark`, rgb 30,30,30) — my earlier white screenshot was a flash
      of unstyled content during load. Real gaps were just accents + tone.
- [x] `NewEvent`/`NewGroup`/`NewSignUp`/`NewDialog`: parchment text, brass accents/tabs,
      brass primary buttons, archaic headers ("Call a Gathering"/"Amend the Gathering").
- [x] `SlideToggle.vue`: green active → brass (fixes create-dialog tabs everywhere).
- [x] `index.css`: global tint of `.v-dialog/.v-menu` card surfaces → wood + brass keyline.
- [x] `App.vue` global elevated-button rule: green box-shadow/border → brass (was the
      green halo on the "Call a Gathering" button + all primary buttons).
- [x] `AvailabilityTypeToggle.vue` LEFT green/amber on purpose (semantic, matches grid).
- [x] `SignIn.vue` (also covers SignUp via prop): crest+wordmark, dark page, brass
      links/buttons, archaic copy ("Welcome Back, Good Sir"/"Join the Fellowship"/
      "Seek admittance"). `SignInDialog.vue` brass accents. `Auth.vue` already fine.
      Google/Outlook brand buttons kept white. Deployed & verified.
- [ ] Archaic copy pass on the create dialogs' field labels (deferred to Phase 5 copy pass).

### Phase 4 — Logged-in app  *(DONE; deployed; user-reviewed)*
- [x] `Home.vue` dashboard + `home/Dashboard.vue`, `EventItem.vue`, `EventType.vue`.
- [x] Groups (`groups/`), Friends, `Settings` + `settings/` components, `AuthUserMenu.vue`.
- [x] Converted via the repo-wide token sweep; user reviewed while logged in and reported
      issues (grey /home bg, etc.) which were fixed in the Parking-Lot pass.

### Phase 5 — Misc & cross-cutting  *(meta/favicon/OG done; deployed)*
- [x] index.html meta defaults → Fellowship title/description, dark theme-color.
- [x] SVG crest favicon (`public/favicon.svg`); old `favicon.ico`/`-16`/`-32` PNGs
      remain (need binary art) — SVG covers modern browsers.
- [x] OG image regenerated (`public/img/ogImage.png`) via Playwright render of the crest.
- [x] Removed obsolete `public/ads.txt`.
- [x] Per-view metaInfo titles → "· The Fellowship" (Home/Settings/PrivacyPolicy/404,
      Event doc.title). `PrivacyPolicy.vue` was a thin wrapper, inherits dark theme.
- [x] Repo-wide GUARDED token sweep across the long-tail components (event sub-components,
      sign-up forms, calendar/permission dialogs, misc): parchment/brass/leather. Settings
      switch, mobile speed-dial, sign-up borders → brass. Card bg-white → leather (brand
      logo buttons kept white). Build clean, deployed, event page re-verified.
- [x] User-facing "Timeful" copy rebranded (Import a Gathering, Seek admittance, Fellowship
      account, etc.). Unused redditComments data in Landing.vue still says Timeful (not rendered).
- [x] Old binary favicons regenerated from the crest (Parking-Lot #8): favicon-16/32 PNGs,
      multi-size `favicon.ico`, apple-touch-icon, `?v=2` cache-bust.
- [x] Phase 4 logged-in screens VISUALLY REVIEWED by user; issues fixed.
- [ ] Accessibility/contrast audit on dark; mobile pass at 390px (not done — optional).

### Phase 6 — Finalize  *(pending)*
- [x] `schej_*` assets removed (Parking-Lot #1). Hero media removed (no product demo wanted).
- [x] Removed unused `Logo.vue` (replaced by the crest everywhere) + its orphaned assets
      `timeful_logo_with_text.png` and `april_fools_logo.png` (2026-07-12).
- [ ] **Merge `redesign/fellowship-phase1` → `main`; `git checkout main` on the VM; redeploy.**
      (Deferred — keep deploying the branch while access-control phases B–E land, then merge once.)

## 5. Conventions & gotchas

- **Deploy = commit + push + deploy** (standing rule). VM repo at
  `/home/jasonmanderson/docker/timeful.app`, `ssh jasonmanderson@192.168.54.68`.
- **Docker Hub pull rate limit (429):** `docker compose up -d --build` rebuilds the
  Go server too, which pulls `golang:1.25-alpine` and can hit the anonymous pull limit.
  When ONLY the frontend changed, deploy with:
  `docker compose up -d --build frontend && docker compose restart server`
  (the server must restart to re-read the new `dist` — it registers static routes by
  walking `frontend/dist` at startup). Longer-term fix: `docker login` on the VM.
- **Dual-purpose Tailwind tokens:** legacy tokens like `white`/`light-gray` are used as
  BOTH backgrounds and text in different places — do NOT global-remap them; convert each
  screen to the new `flw`/Fellowship tokens explicitly. Legacy tokens retained for now.
- **No local Go/Docker on the dev box** — backend can't be compiled locally; rely on the
  VM build (safe failure: bad build keeps old container running).
- **Verifying a live route:** Go `NoRoute` serves index.html (200 text/html) for any
  unregistered path — check content-type, not status, to tell removed vs live API routes.
- **Review/screenshots:** Playwright+Chromium installed at `/tmp/shot` on the dev box;
  serve a local build with `python3 -m http.server 8099` from `frontend/dist`.
- Build check: `cd frontend && npm run build` (no backend needed; API calls just fail).

## 6. Open questions / decisions to revisit

- Hero demo media: keep a product demo, or replace with bespoke club art?
- Footer: keep Blog/Articles/Reddit columns, or replace with club-appropriate links?
- How literal should the archaic voice be on functional errors (usability vs theme)?
- Light-mode toggle, or dark-only? (Currently dark-only.)
