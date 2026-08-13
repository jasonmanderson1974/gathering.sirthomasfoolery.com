<template>
  <!--
    No detail to show ⇒ render the trigger exactly as it arrived, with no
    wrapper and no behaviour. A guest respondent whose `_id` is their own name,
    a comment whose author has been deleted, a legacy name-keyed RSVP — all of
    them land here, and none of them should gain a card offering a 96px
    monogram and nothing else.
  -->
  <slot v-if="!detail" />
  <v-menu
    v-else
    :open-on-hover="!touch"
    :open-on-click="touch"
    :open-delay="OPEN_DELAY_MS"
    :close-delay="CLOSE_DELAY_MS"
    :close-on-content-click="false"
    location="top"
    origin="auto"
  >
    <template v-slot:activator="{ props }">
      <!--
        The activator needs a real box: hover is dispatched at a layout box, and
        the slot content may be a bare avatar, a text span, or several nodes.
        `inline-flex` is the least invasive shape that still has one — it sits
        inside the flex rows these triggers live in without changing their
        baseline, which `block` would.
      -->
      <!--
        `data-member-hover` is a hook for `check:routes`, which has to dispatch
        a synthetic mouseenter at the element carrying the listener — and
        `mouseenter` does not bubble, so it cannot aim at the avatar inside.
        Same purpose as the `id="roster-export-btn"` hooks on the Fellowship
        export menu.
      -->
      <span
        v-bind="props"
        data-member-hover
        class="tw-inline-flex tw-items-center tw-align-middle"
        @mouseenter="prefetch"
        @focusin="prefetch"
      >
        <slot />
      </span>
    </template>

    <v-card
      class="tw-border tw-border-brass-dim tw-bg-leather tw-p-4"
      :max-width="280"
      elevation="8"
    >
      <div class="tw-flex tw-flex-col tw-items-center tw-gap-2 tw-text-center">
        <UserAvatarContent :user="detail.avatarUser" :size="AVATAR_PX" />

        <div class="tw-min-w-0 tw-max-w-full">
          <div class="tw-truncate tw-font-medium tw-text-parchment">
            {{ heading }}
          </div>
          <!--
            Only when it says something the heading didn't. With no nickname the
            heading IS the real name, and repeating it reads as a rendering bug.
          -->
          <div
            v-if="subheading"
            class="tw-truncate tw-text-sm tw-text-parchment-dim"
          >
            {{ subheading }}
          </div>
        </div>

        <!--
          Absent fields are not rendered at all rather than shown blank: a guest
          only ever gets the names, and a member with no telephone on the roll
          should not get an empty row implying one is missing.
        -->
        <a
          v-if="detail.email"
          :href="`mailto:${detail.email}`"
          class="tw-flex tw-max-w-full tw-items-center tw-gap-1 tw-text-sm tw-text-parchment-dim hover:tw-text-brass"
        >
          <v-icon size="x-small" color="brass">mdi-email-outline</v-icon>
          <span class="tw-truncate">{{ detail.email }}</span>
        </a>
        <a
          v-if="detail.phone"
          :href="`tel:${detail.phone}`"
          class="tw-flex tw-max-w-full tw-items-center tw-gap-1 tw-text-sm tw-text-parchment-dim hover:tw-text-brass"
        >
          <v-icon size="x-small" color="brass">mdi-phone-outline</v-icon>
          <span class="tw-truncate">{{ formatPhone(detail.phone) }}</span>
        </a>
      </div>
    </v-card>
  </v-menu>
</template>

<script>
/*
 * Who is this? — on hover, anywhere a member is shown (N3).
 *
 * The card cannot read what it shows off the object beside it. The server
 * strips `Phone` from every event payload on purpose
 * (`stripSensitiveUserFields`) and `slimUserForDisplay` drops the email too, so
 * a comment byline knows a name and an id and nothing else. The details come
 * from the store's people cache, which is filled from the Fellowship roll for
 * member+ and from the public profile for guests — see `store/people.js`.
 *
 * WHERE THIS GOES: around a person being *displayed*, never around a person
 * being *selected*. That is what keeps it out of the assignee picker, the
 * mention autocomplete, the invite composer and the expense form, where hover
 * and click are already spoken for by the act of choosing someone.
 *
 * `RespondentsList` is the one place where it shares a gesture, and it works
 * because the two listeners are different events: that row highlights a
 * respondent's availability on `@mouseover`, which BUBBLES, so it still fires
 * when the pointer is over the card's wrapper; the card opens on `mouseenter`,
 * which does not bubble and so is never triggered by anything inside the row.
 * The grid highlight and the card are both live at once, by design.
 */
import { mapGetters } from "vuex"
import UserAvatarContent from "@/components/UserAvatarContent.vue"
import { formatPhone, isTouchEnabled, userFromDisplayName } from "@/utils"
import { accountId, personDetail } from "@/utils/directory"

// Long enough that crossing a row of avatars on the way somewhere else opens
// nothing, short enough to feel like an answer rather than a wait.
const OPEN_DELAY_MS = 500
// Not decoration: without it, moving the pointer off the trigger and onto the
// card to click the email dismisses it before you arrive.
const CLOSE_DELAY_MS = 150
// The Settings page's own avatar size, so the face here is the face members
// recognise from their profile rather than a third size.
const AVATAR_PX = 96

export default {
  name: "MemberHoverCard",

  components: { UserAvatarContent },

  props: {
    /** Account id to look up. Absent ⇒ no card, and that is a valid state. */
    userId: { type: String, default: "" },
    /** Whatever user object the call site already had, if any. */
    fallback: { type: Object, default: null },
    /**
     * The name already on screen, for the many sites that hold a stored name
     * snapshot and an id but no user object — an expense split, a checklist
     * assignee. Split into first/last the same way every other snapshot
     * fallback in the app is, so the monogram matches the one beside it.
     */
    name: { type: String, default: "" },
    /**
     * A pre-resolved record, for the two views that already hold the roll —
     * they render allowlist rows, so the row IS the record and there is nothing
     * to look up.
     */
    person: { type: Object, default: null },
  },

  data: () => ({ OPEN_DELAY_MS, CLOSE_DELAY_MS, AVATAR_PX }),

  computed: {
    ...mapGetters(["personById", "canInvite"]),

    /**
     * The id, once — filtered through `accountId`, so no call site has to
     * remember that a zero ObjectID serializes as 24 zeros and a legacy row can
     * hold a name where an id belongs. Both would otherwise open a card over
     * somebody who has no account.
     */
    accountId() {
      return accountId(this.userId)
    },

    touch() {
      // Hover does not exist on a touch device, so the same card opens on tap.
      // Matches how RespondentsList swaps its hover affordance on phones.
      return isTouchEnabled()
    },

    detail() {
      const record =
        this.person ?? (this.accountId ? this.personById(this.accountId) : null)
      // An id is only ever an account's, so seeding it here is what lets a call
      // site that holds `assigneeId` and a name string — with no user object at
      // all — resolve to a card. Without it those sites would render the slot
      // bare, never mount a hover target, and so never trigger the lookup that
      // would have filled them in: a card that can only appear once it has
      // already appeared.
      const base =
        this.fallback ?? (this.name ? userFromDisplayName(this.name) : null)
      const seed = this.accountId
        ? { ...base, _id: base?._id ?? this.accountId }
        : base
      return personDetail(record, seed)
    },

    /** The nickname when there is one — that is what the club calls them. */
    heading() {
      if (!this.detail) return ""
      return this.detail.nickname || this.detail.realName || this.detail.email
    },

    subheading() {
      if (!this.detail) return ""
      const { nickname, realName } = this.detail
      return nickname && realName ? realName : ""
    },
  },

  /*
   * A member+ viewer warms the roll as soon as any card exists on the page.
   *
   * It is one request per SESSION however many cards mount — `ensureDirectory`
   * dedupes on both the in-flight promise and the loaded flag — and it is the
   * same call /fellowship and /members already make. Doing it here rather than
   * only on hover also closes the bootstrap case: a site that knows an id and a
   * name but holds no user object needs the roll before it can show anything
   * worth hovering.
   *
   * Guests are deliberately excluded: their path is one request per person, so
   * ten respondents on a page would be ten calls at load. They fetch on hover.
   */
  created() {
    if (this.canInvite) this.prefetch()
  },

  methods: {
    formatPhone,

    /*
     * Start the lookup on mouseenter, not on open.
     *
     * The request rides the 500ms delay, so by the time the card is drawn the
     * details are usually already there. Dispatching on open instead would show
     * every card as a name and a monogram first and then reflow as the phone
     * number arrived.
     */
    prefetch() {
      if (this.person || !this.accountId) return
      this.$store.dispatch("ensurePerson", this.accountId)
    },
  },
}
</script>
