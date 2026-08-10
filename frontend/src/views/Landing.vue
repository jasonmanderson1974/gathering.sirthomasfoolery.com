<template>
  <div class="tw-bg-transparent">
    <div
      class="tw-relative tw-m-auto tw-mb-12 tw-flex tw-max-w-6xl tw-flex-col tw-px-4 sm:tw-mb-20"
    >
      <!-- Header -->
      <div class="tw-mb-12 sm:tw-mb-20">
        <div class="tw-flex tw-items-center tw-pt-5">
          <router-link
            :to="{ name: 'landing' }"
            class="tw-flex tw-items-center tw-gap-3 tw-no-underline"
          >
            <SirThomasFoolery :size="44" />
            <span
              class="tw-font-display tw-text-lg tw-font-bold tw-tracking-[0.16em] tw-text-parchment sm:tw-text-xl"
              >THE FELLOWSHIP</span
            >
          </router-link>

          <v-spacer />

          <LandingPageHeader>
            <div v-if="authUser" class="tw-ml-2">
              <AuthUserMenu />
            </div>
            <v-btn
              v-else
              text
              class="tw-font-display tw-tracking-widest tw-text-brass"
              :to="{ name: 'sign-in' }"
              >Enter</v-btn
            >
          </LandingPageHeader>
        </div>
      </div>

      <div class="tw-flex tw-flex-col tw-items-center">
        <header
          class="tw-mb-8 tw-flex tw-flex-col tw-items-center tw-text-center"
        >
          <SirThomasFoolery :size="104" class="tw-mb-4" />
          <p class="flw-eyebrow tw-mb-2">The Fellowship Presents</p>
          <h1
            id="header"
            class="flw-title tw-text-5xl sm:tw-text-6xl xl:tw-text-7xl"
          >
            The Gathering
          </h1>
          <div class="flw-rule tw-mt-4"><span>&#9670;</span></div>
          <p class="flw-sub tw-mt-5 tw-text-xl sm:tw-text-2xl">
            Cast thy vote, good sir &mdash; when shall we convene?
          </p>
        </header>

        <div class="tw-mb-12 tw-flex tw-flex-col tw-items-center tw-gap-2">
          <button
            class="flw-btn tw-text-sm sm:tw-text-base"
            @click="authUser ? openDashboard() : signIn()"
          >
            {{ authUser ? "To the Club Room" : "Call a Gathering" }}
          </button>
        </div>
      </div>
    </div>

    <!-- How a Gathering comes to pass -->
    <div
      id="how-it-works"
      class="tw-grid tw-place-content-center tw-px-4 tw-pt-16"
    >
      <div class="tw-mb-8 tw-text-center">
        <p class="flw-eyebrow tw-mb-2">The Order of Things</p>
        <h2 class="flw-title tw-text-3xl sm:tw-text-4xl">
          How a Gathering Comes to Pass
        </h2>
        <div class="flw-rule tw-mt-4"><span>&#9670;</span></div>
      </div>
      <div class="tw-mx-auto tw-flex tw-max-w-xl tw-flex-col tw-gap-4">
        <div
          v-for="(step, i) in howItWorksSteps"
          :key="i"
          class="tw-flex tw-items-center tw-gap-3"
        >
          <NumberBullet>{{ i + 1 }}</NumberBullet>
          <div class="tw-font-head tw-text-lg tw-text-parchment md:tw-text-xl">
            <div v-html="step"></div>
          </div>
        </div>
      </div>
      <div
        class="flw-sub tw-mb-6 tw-mt-12 tw-text-center tw-text-3xl tw-text-brass md:tw-mb-12 md:tw-mt-16 md:tw-text-5xl"
      >
        And so, &rsquo;tis done.
      </div>
      <SirThomasFoolery
        :size="isPhone ? 150 : 210"
        class="tw-mx-auto -tw-mb-12"
      />
    </div>

    <!-- FAQ -->
    <div class="tw-flex tw-justify-center tw-pt-12">
      <div class="tw-mx-4 tw-mb-12 tw-max-w-3xl tw-flex-1 sm:tw-mx-16">
        <div id="faq-section" class="tw-text-center lg:tw-pt-3">
          <p class="flw-eyebrow tw-mb-2">Should You Wonder</p>
          <h2 class="flw-title tw-text-3xl sm:tw-text-4xl">
            Matters of Procedure
          </h2>
          <div class="flw-rule tw-mb-8 tw-mt-4"><span>&#9670;</span></div>
          <div
            class="tw-grid tw-grid-cols-1 tw-gap-3 tw-text-left sm:tw-text-xl lg:tw-text-2xl"
          >
            <FAQ
              v-for="faq in faqs"
              :key="faq.question"
              @signIn="signIn"
              v-bind="faq"
            />
          </div>
        </div>
      </div>
    </div>

    <AppFooter />
  </div>
</template>

<style scoped>
@media screen and (min-width: 375px) and (max-width: 640px) {
  #header {
    font-size: 1.875rem !important; /* 30px */
    line-height: 2.25rem !important; /* 36px */
  }
}
</style>
<style>
.rdt-h {
  @apply tw-rounded tw-bg-brass/20 tw-px-px tw-text-parchment;
}
</style>

<script>
import { isPhone } from "@/utils"
import FAQ from "@/components/FAQ.vue"
import NumberBullet from "@/components/NumberBullet.vue"
import LandingPageHeader from "@/components/landing/LandingPageHeader.vue"
import AppFooter from "@/components/Footer.vue"
import { mapState } from "vuex"
import AuthUserMenu from "@/components/AuthUserMenu.vue"
import SirThomasFoolery from "@/components/general/SirThomasFoolery.vue"

export default {
  name: "Landing",

  metaInfo: {
    title: "The Fellowship · The Gathering",
  },

  components: {
    SirThomasFoolery,
    FAQ,
    NumberBullet,
    LandingPageHeader,
    AppFooter,
    AuthUserMenu,
  },

  data: () => ({
    howItWorksSteps: [
      "Call a Gathering and propose the candidate evenings",
      "Circulate the summons, that each man may mark his availability",
      "Behold where the whole Order's free hours align &mdash; and set the date",
    ],
    faqs: [
      {
        question: "Does the Fellowship account for timezones?",
        answer:
          "Indeed, good sir. All hours are rendered in each viewer's own timezone automatically. Should you wish to set it yourself, a timezone selector sits amongst the Gathering's controls, above the grid — save upon a dates-only Gathering, where no hours are in question.",
      },
      {
        question: "In what manner may I put the question to the Order?",
        answer: `Three, as the occasion demands. "Dates and times" collects exact hours — one window across every date, or a different window for each. "Dates w/ time blocks" lets you set the sittings yourself, and each man may take only a whole block, never a part of one. "Dates only" asks merely which days serve, when the hour is of no consequence.`,
      },
      {
        question: "Which calendars may I consult?",
        answer:
          "You may draw your availability from Google Calendar, Outlook, Apple Calendar, or an ICS feed URL. Further provisions are forthcoming.",
      },
      {
        question: "Must I grant calendar access to take part?",
        answer:
          "Not at all. You may enter your availability by hand — though we heartily recommend granting access, that you may consult your engagements as you mark them.",
      },
      {
        question: "Will others spy upon my calendar?",
        answer:
          "Never. Your fellows see only the availability you choose to enter for a given Gathering — nothing more.",
      },
      {
        question: "How do I amend my availability?",
        answer:
          'Simply press the "Edit availability" button. Should you have entered the hours of a guest on their behalf, hover upon their name in the list of respondents and press the pencil beside it.',
      },
      {
        question: "Must I be a member to answer a summons?",
        answer:
          "You must. Every Gathering lies behind the door — none may view or answer one without entering first, and the Fellowship admits by invitation only. Should you wish to record the availability of a lady or a visitor who holds no account, any member may enter it upon their behalf.",
      },
      {
        question: "May a Gathering be made a standing fixture?",
        answer: `It may. When you settle upon the hour, set "Repeat" to weekly, fortnightly, or monthly, and the Gathering advances itself — once an occasion has passed it rolls forward to the next date of its own accord, with RSVPs cleared and reminders dispatched afresh. No man need book it again.`,
      },
      {
        question: "What becomes of a Gathering once the hour is settled?",
        answer: `Your fellows mark themselves going, maybe, or unable — plus-ones and all. You may attach polls for the venue or the entertainment, and a discussion thread sits beneath for the arguing of details; write <strong>@</strong> and a man's name to summon his attention, and word is sent to him by post. Once the day has passed, the Gathering is entered into The Chronicle, the Order's standing record of what came to pass.`,
      },
      {
        question: "May we keep lists against a Gathering?",
        answer: `Indeed — beneath the calendar sit tabs for the purpose. <strong>Lists</strong> are shared with the whole party: what to bring, who drives, a kit list. They nest three deep, may be made tick-boxes, and entries may be dragged into whatever order pleases you, or across into another list. <strong>My Lists</strong> and <strong>My Notes</strong> are yours alone against that one Gathering — a private packing list and a private notebook — and no other man may read them, not even an Admin.`,
        authRequired: true,
      },
      {
        question: "How are the reckonings settled when coin has changed hands?",
        answer: `Under <strong>Settle Up</strong>, each man logs what he paid, who he paid it for, and a likeness of the receipt should he have one — divided evenly, by exact amounts, or by shares, as the case demands. A summary then declares what each man laid out and what he truly owes. All may read the ledger; members and above may add to it; and every amendment is recorded against the man who made it.`,
        authRequired: true,
      },
      {
        question: `I should like only myself to see the replies.`,
        answer: `But of course — tick "Hide responses from respondents" under Advanced Options when calling your Gathering, and your fellows shall see neither names nor availability of one another until they have answered themselves.`,
        authRequired: true,
      },
      {
        question: `May I be notified when a man answers?`,
        answer: `Most certainly. Tick "Email me each time someone joins my event" when calling a Gathering. <br><br>For word only after a certain number (X) of replies, tick "Email me after X responses" under Advanced Options.`,
        authRequired: true,
      },
      {
        question: `How do I prompt the laggards to reply?`,
        answer: `Open the "Email reminders" section when calling your Gathering and enter each man's address. Reminders are dispatched on the day of creation, the day following, and three days hence. <br><br>You shall also be notified once the whole Order has answered.`,
        authRequired: true,
      },
    ],
  }),

  computed: {
    ...mapState(["authUser"]),
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },

  methods: {
    signIn() {
      this.$router.push({ name: "sign-in" })
    },
    openDashboard() {
      this.$router.push({ name: "home" })
    },
  },
}
</script>
