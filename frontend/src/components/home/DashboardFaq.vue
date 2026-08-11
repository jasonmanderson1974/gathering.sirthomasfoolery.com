<template>
  <div class="tw-mt-8">
    <div
      class="tw-flex tw-cursor-pointer tw-select-none tw-items-center tw-justify-between"
      @click="open = !open"
    >
      <div class="tw-flex tw-items-center tw-gap-2">
        <v-icon class="tw-text-brass">mdi-help-circle-outline</v-icon>
        <span class="tw-font-head tw-text-lg tw-text-parchment sm:tw-text-xl">
          How gatherings work
        </span>
      </div>
      <v-btn icon size="small" class="tw-text-parchment-dim">
        <v-icon
          :class="`tw-transition-transform ${open ? 'tw-rotate-180' : ''}`"
          >mdi-chevron-down</v-icon
        >
      </v-btn>
    </div>

    <v-expand-transition>
      <div v-show="open" class="tw-mt-2">
        <p class="tw-mb-3 tw-text-sm tw-italic tw-text-parchment-dim">
          A short primer on calling a Gathering, gathering everyone's
          availability, and settling on the hour — including how repeating
          gatherings keep themselves going.
        </p>

        <v-expansion-panels
          v-model="openPanels"
          multiple
          variant="accordion"
          class="faq-panels"
        >
          <!-- 1. Overview -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              The scheduling process, start to finish
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <ol class="tw-list-decimal tw-space-y-1 tw-pl-5">
                <li>
                  <strong>Call a Gathering.</strong> Give it a name and lay out
                  the candidate days (and, if you like, times).
                </li>
                <li>
                  <strong>Members cast availability.</strong> Everyone opens the
                  event and paints the hours they're free.
                </li>
                <li>
                  <strong>Settle on a time.</strong> Once responses are in, the
                  organiser picks the slot that suits the most people and
                  schedules it.
                </li>
                <li>
                  <strong>RSVP &amp; remind.</strong> The confirmed gathering
                  collects RSVPs, and reminder emails go out before it begins.
                </li>
                <li>
                  <strong>Into the Chronicle.</strong> After it passes, the
                  gathering is archived to the club's history automatically.
                </li>
              </ol>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 2. Calling a gathering -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              Calling a Gathering — the date &amp; time settings
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <p class="tw-mb-2">
                Press <strong>Call a Gathering</strong> (or the
                <v-icon size="small" class="tw-text-brass">mdi-plus</v-icon>
                beside a folder). There are <em>two</em> separate questions to
                answer, and they combine freely.
              </p>
              <p class="tw-mb-2">
                <strong>First, how precisely people answer:</strong>
              </p>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>Dates and times</strong> — collect exact hours. Then
                  choose <em>Same Times Every Day</em> to offer one start-to-end
                  window across every date, or
                  <em>Custom Times Every Day</em> to paint a different window
                  for each date in the next step.
                </li>
                <li>
                  <strong>Dates w/ time blocks</strong> — you draw the blocks
                  yourself in the next step, and members can only take a whole
                  block, never part of one. Good when the slots are fixed — a
                  two-hour sitting, a shift — and half answers are no use.
                </li>
                <li>
                  <strong>Dates only</strong> — poll which whole days work, with
                  no times at all, for when the hour doesn't matter. Offered
                  when creating a gathering, not when editing one: switching an
                  existing gathering in or out of it would change what its dates
                  mean.
                </li>
              </ul>
              <p class="tw-mb-2 tw-mt-3">
                <strong>Then, which days you're asking about:</strong>
              </p>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>Specific dates</strong> — drag across the calendar to
                  select particular calendar dates.
                </li>
                <li>
                  <strong>Days of the week</strong> — poll a generic “which
                  weekdays suit you” (Mon, Wed, Fri…) rather than fixed dates —
                  handy for standing meet-ups. <em>Start on Monday</em> just
                  reorders the week.
                </li>
              </ul>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 3. Casting availability -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              Casting your availability
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <p class="tw-mb-2">
                Open the event and drag across the grid to mark the times you're
                free. As people respond, the grid darkens where more members
                overlap, and <strong>best times</strong> highlights the slots
                that suit the most people. If your calendar is connected, your
                busy times can be overlaid so you don't double-book.
              </p>
              <p class="tw-mb-2">
                Each cell also carries a <strong>response count</strong> — how
                many members are free then — coloured green, amber or red as
                that share of respondents falls. Turn it off with
                <em>Show response counts</em> in the event's options; it's on by
                default.
              </p>
              <p class="tw-mb-2">
                If the gathering was called with <strong>time blocks</strong>,
                you don't paint freely: clicking anywhere inside a block takes
                the entire block.
              </p>
              <p>
                You must be signed in to open a gathering at all — there is no
                anonymous access. For someone without an account (a wife, a
                visitor), any member can press
                <strong>+ Add guest availability</strong> in the respondents
                list and enter it on their behalf; edit it later with the pencil
                beside their name.
              </p>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 4. Settling on a time -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              Settling on the final time (scheduling)
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <p class="tw-mb-2">
                When enough availability is in, the organiser presses
                <strong>Schedule event</strong>, drags to select the winning
                slot on the grid, then opens <strong>Schedule</strong> and
                presses <strong>Save gathering</strong>. Before saving you can
                set:
              </p>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>Email reminder to respondents</strong> and how far
                  ahead it goes out (12, 24, or 48 hours before).
                </li>
                <li>
                  <strong>Repeat</strong> — whether this becomes a recurring
                  gathering (see below).
                </li>
              </ul>
              <p class="tw-mt-2">
                Saving confirms the gathering in Gathering itself — no external
                calendar needed. To add it to your own calendar afterwards, use
                <strong>Add to calendar</strong> on the confirmed gathering,
                which downloads a universal <code>.ics</code> file (and carries
                the repeat rule if it's recurring).
              </p>
              <p class="tw-mt-2">
                Once saved, an <strong>RSVP</strong> panel appears so members
                can mark themselves going / maybe / can't (with optional
                plus-ones). The organiser can <em>Reschedule</em> or
                <em>Cancel</em> the gathering later from the same menu.
              </p>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 5. Recurring — the headline -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              How recurring gatherings work
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <p class="tw-mb-2">
                When you schedule a gathering, the <strong>Repeat</strong>
                setting turns it into a standing fixture. The choices are:
              </p>
              <ul class="tw-list-disc tw-space-y-1 tw-pl-5">
                <li>
                  <strong>Does not repeat</strong> — a one-off (the default).
                </li>
                <li>
                  <strong>Weekly</strong> — same day &amp; time every week.
                </li>
                <li>
                  <strong>Every 2 weeks</strong> — the same slot on a
                  fortnightly cadence.
                </li>
                <li><strong>Monthly</strong> — the same date each month.</li>
              </ul>
              <p class="tw-mb-2 tw-mt-3">
                <strong>It advances itself.</strong> Once an occurrence's start
                time passes, the gathering automatically rolls forward to the
                next date — keeping the same time of day — so the event page
                always shows the <em>next</em> meeting without anyone re-booking
                it. This happens even on quiet weeks; you don't have to touch
                it.
              </p>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>RSVPs reset each occurrence.</strong> When it rolls
                  forward, the previous week's going/maybe/can't answers clear
                  so everyone can RSVP fresh for the new date.
                </li>
                <li>
                  <strong>Reminders repeat too.</strong> If you enabled a
                  pre-gathering reminder, it's sent ahead of every occurrence,
                  not just the first.
                </li>
                <li>
                  <strong>Your calendar keeps up.</strong> The exported event
                  carries a proper repeat rule, so Google Calendar / Outlook /
                  the <code>.ics</code> file show the whole series natively.
                </li>
                <li>
                  <strong>Monthly is date-safe.</strong> A gathering on the 31st
                  simply lands on the last day of shorter months.
                </li>
                <li>
                  <strong>Each occurrence is chronicled.</strong> Every meeting
                  is captured into the Chronicle in turn, just before it rolls
                  forward — so the history records each time you actually met.
                </li>
              </ul>
              <p class="tw-mt-3">
                To stop or change the cadence, use <em>Reschedule</em> on the
                confirmed gathering and set <strong>Repeat</strong> back to
                <em>Does not repeat</em> (or a different frequency).
              </p>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 6. Advanced options -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              What the Advanced options do
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>Time increment</strong> — the grid's resolution (15,
                  30 or 60 minutes per cell).
                </li>
                <li>
                  <strong>Collect respondents' email addresses</strong> — ask
                  each responder for an email so you can reach them.
                </li>
                <li>
                  <strong>Hide responses from respondents</strong> (blind
                  availability) — people can't see others' answers until they've
                  submitted their own, so nobody just copies the crowd.
                </li>
                <li>
                  <strong>Email me after N responses</strong> — get a nudge once
                  a chosen number of people have replied.
                </li>
                <li>
                  <strong>Timezone</strong> — the timezone the event's times are
                  anchored to; each viewer still sees the grid in their own
                  zone.
                </li>
              </ul>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 7. Reminders & notifications -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              Reminders &amp; notifications
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>Email me when someone joins</strong> — a note to the
                  organiser each time a new availability comes in.
                </li>
                <li>
                  <strong>Remind people to fill out the event</strong> — invite
                  emails nudging respondents; sent the day the event is created,
                  one day later, and three days later, plus a note to you when
                  everyone has responded.
                </li>
                <li>
                  <strong>Pre-gathering reminder</strong> — set at scheduling
                  time, this emails respondents 12 / 24 / 48 hours before the
                  confirmed gathering (and before every occurrence, if it
                  repeats).
                </li>
              </ul>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 8. RSVPs, Polls, Chronicle -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              RSVPs, Polls, Discussion &amp; The Chronicle
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>RSVPs</strong> — once a gathering has a confirmed
                  time, members mark <em>going</em>, <em>maybe</em>, or
                  <em>can't</em>, and can add plus-ones.
                </li>
                <li>
                  <strong>Polls</strong> — the organiser can attach venue or
                  activity polls to an event so the party can also decide the
                  <em>where</em> and the <em>what</em>, not just the when.
                </li>
                <li>
                  <strong>Discussion</strong> — every gathering carries a
                  comment thread beneath it, so the plan can be argued out in
                  the open rather than scattered across email. Type
                  <strong>@</strong> to mention someone: pick them from the list
                  and they get an email pointing back at the comment.
                </li>
                <li>
                  <strong>The Chronicle</strong> — every gathering that comes to
                  pass is archived automatically (with who attended), forming
                  the club's running history. It's kept separately, so the
                  record survives even if the original event is later removed.
                </li>
              </ul>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 9. The tabs beneath a gathering -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              The tabs beneath a gathering — Lists, My Lists, My Notes, Settle
              Up
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <p class="tw-mb-2">
                Under the calendar sits a row of tabs. The first two are shared
                with everyone on the gathering; the next two are yours alone.
              </p>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>Lists</strong> — shared lists for the gathering: what
                  to bring, who's driving, a running kit list. Items nest three
                  deep, a list can be a <em>checklist</em> with tick-boxes, and
                  you can drag entries into order or across to another list. The
                  organiser and admins make and remove the lists themselves;
                  anyone can add entries, you may edit only your own words, and
                  members and up can remove anyone's.
                </li>
                <li>
                  <strong>My Lists</strong> — the same idea, but
                  <em>private to you</em> and tied to this one gathering. For
                  what you personally need to pack or remember. Nobody else can
                  see it, not even an admin.
                </li>
                <li>
                  <strong>My Notes</strong> — one private notebook per
                  gathering, written in Markdown, saved as you type. Also
                  invisible to everyone else.
                </li>
                <li>
                  <strong>Settle Up</strong> — the shared ledger, for when money
                  changed hands. See below.
                </li>
              </ul>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 10. Settle Up -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              Settling the bill (Settle Up)
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <p class="tw-mb-2">
                Log what was spent and the ledger works out who owes whom. Add
                an expense with what it was, what it cost,
                <strong>who paid</strong>
                and who it's shared between; attach a photo of the receipt if
                you have one.
              </p>
              <p class="tw-mb-2">Split it three ways:</p>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>Evenly</strong> — divided between everyone you tick,
                  with any odd penny handled for you.
                </li>
                <li>
                  <strong>By amount</strong> — type each person's share; it
                  won't save until the parts add up to the total, to the penny.
                </li>
                <li>
                  <strong>By share</strong> — for "he had two courses, I had
                  one".
                </li>
              </ul>
              <p class="tw-mt-2">
                A running summary shows what each person <em>paid</em>, with the
                gathering's total beneath it; click a row to expand what they
                actually <em>owe</em> and the items behind it. Everyone on the
                gathering can read the ledger — guests included, since a guest
                at the dinner has a fair interest in the bill — but only members
                and up can add to it, and only the person who entered a row (or
                an admin) can change it. Every edit is kept, with who made it,
                and a deleted expense leaves its trail behind.
              </p>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 11. Folders -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              Folders on your dashboard
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <p class="tw-mb-2">
                Your dashboard files everything into folders. Two are made for
                you and can't be renamed or deleted:
              </p>
              <ul class="tw-list-disc tw-space-y-2 tw-pl-5">
                <li>
                  <strong>Invites created</strong> — gatherings you called
                  yourself.
                </li>
                <li>
                  <strong>Invites received</strong> — gatherings someone else
                  called and invited you to.
                </li>
              </ul>
              <p class="tw-mt-2">
                Events land in the matching one automatically, but nothing is
                stuck there: drag an event between the two defaults, or into any
                folder you make yourself, and the placement is remembered.
              </p>
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>
      </div>
    </v-expand-transition>
  </div>
</template>

<script>
export default {
  name: "DashboardFaq",

  data() {
    return {
      open: false,
      openPanels: [],
    }
  },
}
</script>

<style scoped>
/* Match the Fellowship dark theme — Vuetify panels default to a light card. */
.faq-panels :deep() .v-expansion-panel {
  background-color: transparent !important;
  border-bottom: 1px solid #8a7333; /* brass-dim */
}
.faq-panels :deep() .v-expansion-panel::before {
  box-shadow: none !important;
}
.faq-panels :deep() .v-expansion-panel-title {
  color: #ede4d3; /* parchment */
  font-weight: 500;
  padding: 14px 4px;
  min-height: 48px;
}
.faq-panels :deep() .v-expansion-panel-title .v-icon {
  color: #b8ad97; /* parchment-dim */
}
.faq-panels :deep() .v-expansion-panel-text {
  color: #b8ad97; /* parchment-dim */
  font-size: 0.875rem;
  line-height: 1.5;
}
.faq-panels :deep() .v-expansion-panel-text strong {
  color: #ede4d3; /* parchment */
  font-weight: 600;
}
.faq-panels :deep() .v-expansion-panel-text code {
  background-color: rgba(138, 115, 51, 0.2);
  color: #e3c578; /* brass-bright */
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 0.8em;
}
</style>
