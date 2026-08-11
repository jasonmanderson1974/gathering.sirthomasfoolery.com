<template>
  <div v-if="showAccount" class="tw-flex tw-flex-col">
    <div
      class="tw-group tw-flex tw-h-10 tw-flex-row tw-items-center tw-justify-between tw-text-parchment"
    >
      <div
        :class="`tw-gap-${toggleState ? '0' : '2'}`"
        class="tw-flex tw-w-full tw-flex-row tw-items-center"
      >
        <div v-if="toggleState" class="tw-flex tw-items-center">
          <!--
            The write-through into `account.enabled` is load-bearing: the toggle
            POSTs but never refetches, so the shared reference is what keeps the
            checkbox (and the parent's copy) in sync until the next authUser
            refresh. Untangling it means owning the toggle state properly —
            tracked as part of the toggle-mixin extraction (TODO2 G1).
          -->
          <!-- eslint-disable vue/no-mutating-props -->
          <v-checkbox
            v-model="account.enabled"
            @update:model-value="(enabled) => toggleCalendarAccount(enabled)"
            color="primary"
            hide-details
          />
          <!-- eslint-enable vue/no-mutating-props -->
          <div
            v-if="hasSubCalendars"
            class="-tw-ml-2 tw-h-fit tw-w-fit tw-cursor-pointer"
            @click="
              () => {
                showSubCalendars = !showSubCalendars
              }
            "
          >
            <!-- Make sure tailwind classes are compiled -->
            <div class="tw-rotate-0 tw-rotate-90"></div>

            <v-icon
              :class="`tw-rotate-${showSubCalendars ? 90 : 0}`"
              class="tw-text-parchment-dim tw-transition-all"
              >mdi-chevron-right</v-icon
            >
          </div>
        </div>
        <UserAvatarContent v-else :size="24" :user="account" />
        <div
          :class="toggleState && !fillSpace ? 'tw-w-[180px]' : ''"
          class="tw-align-text-middle tw-inline-block tw-break-words tw-text-sm"
        >
          {{ account.email }}
        </div>
        <v-tooltip location="top" v-if="accountHasError">
          <template v-slot:activator="{ props }">
            <v-btn icon v-bind="props" @click="reauthenticateCalendarAccount">
              <v-icon>mdi-alert-circle</v-icon>
            </v-btn>
          </template>
          <span>{{ reauthenticateBtnText }}</span>
        </v-tooltip>
      </div>
      <!-- Needed to make sure tailwind classes compile -->
      <span class="tw-hidden tw-opacity-0 tw-opacity-100"></span>

      <!-- Delete account button -->
      <v-btn
        icon
        :class="`tw-opacity-${
          account.email == selectedRemoveEmail && removeDialog ? '100' : '0'
        } ${!allowDelete ? 'tw-hidden' : ''}`"
        class="group-hover:tw-opacity-100"
        @click="openRemoveDialog"
        ><v-icon color="#b8ad97">mdi-close</v-icon></v-btn
      >
    </div>

    <!-- Sub-calendar accounts -->

    <v-expand-transition>
      <!-- Inset a shade darker than the panel's leather, so the sub-calendars
           read as nested under the account. This was `tw-bg-[#EBF7EF]` — a pale
           mint left over from the pre-Fellowship light theme, which put
           parchment text on a near-white background and made the list
           effectively unreadable. -->
      <div
        v-if="hasSubCalendars && showSubCalendars"
        class="tw-space-y-2 tw-rounded tw-bg-wood-deep tw-py-2 tw-text-parchment"
      >
        <div
          v-for="(subCalendar, id) in account.subCalendars"
          :key="id"
          class="tw-flex tw-flex-row tw-items-start"
        >
          <v-checkbox
            v-model="subCalendar.enabled"
            @update:model-value="
              (enabled) => toggleSubCalendarAccount(enabled, id)
            "
            color="primary"
            class="-tw-mt-px"
            hide-details
          />
          <div
            :class="!fillSpace ? 'tw-w-40' : ''"
            class="tw-align-text-middle tw-ml-8 tw-inline-block tw-break-words tw-text-sm"
          >
            {{ subCalendar.name }}
          </div>
        </div>
      </div>
    </v-expand-transition>
  </div>
</template>

<script>
import { mapState, mapActions } from "vuex"
import { authTypes, calendarTypes } from "@/constants"
import { post, signInGoogle, getCalendarAccountKey } from "@/utils"
import UserAvatarContent from "@/components/UserAvatarContent.vue"

export default {
  name: "CalendarAccount",

  props: {
    toggleState: { type: Boolean, default: false },
    account: { type: Object, default: () => {} },
    eventId: { type: String, default: "" },
    calendarEventsMap: { type: Object, default: () => {} }, // Object of different users' calendar events
    removeDialog: { type: Boolean, default: false },
    selectedRemoveEmail: { type: String, default: "" },
    syncWithBackend: { type: Boolean, default: true },
    fillSpace: { type: Boolean, default: false },
  },

  emits: ["calendarsChanged", "openRemoveDialog"],

  components: {
    UserAvatarContent,
  },

  data: () => ({
    showSubCalendars: false,
  }),

  computed: {
    ...mapState(["authUser"]),
    allowDelete() {
      return !(
        (this.account.calendarType == calendarTypes.GOOGLE &&
          this.account.email == this.authUser.email) ||
        this.toggleState
      )
    },
    hasSubCalendars() {
      return this.account.calendarType !== calendarTypes.ICS
    },
    accountHasError() {
      const account =
        this.calendarEventsMap?.[
          getCalendarAccountKey(this.account.email, this.account.calendarType)
        ]
      return account?.error && account?.calendarEvents?.length === 0
    },
    /** don't show account if in toggle state and account has an error */
    showAccount() {
      return !(this.toggleState && this.accountHasError)
    },
    reauthenticateBtnText() {
      if (this.account.calendarType == calendarTypes.GOOGLE) {
        return "Calendar access not granted, click to reauthenticate"
      } else if (this.account.calendarType == calendarTypes.APPLE) {
        return "Error with Apple Calendar account, click to remove"
      } else if (this.account.calendarType == calendarTypes.OUTLOOK) {
        return "Error with Outlook Calendar account, click to remove"
      }
      return ""
    },
  },

  methods: {
    ...mapActions(["showError"]),
    addCalendarAccount() {
      signInGoogle({
        state: {
          type: this.toggleState
            ? authTypes.ADD_CALENDAR_ACCOUNT_FROM_EDIT
            : authTypes.ADD_CALENDAR_ACCOUNT,
          eventId: this.eventId,
        },
        requestCalendarPermission: true,
        selectAccount: true,
      })
    },
    reauthenticateCalendarAccount() {
      if (this.account.calendarType == calendarTypes.GOOGLE) {
        signInGoogle({
          state: {
            type: this.toggleState
              ? authTypes.ADD_CALENDAR_ACCOUNT_FROM_EDIT
              : authTypes.ADD_CALENDAR_ACCOUNT,
            eventId: this.eventId,
          },
          requestCalendarPermission: true,
          selectAccount: false,
          loginHint: this.account.email,
        })
      } else if (this.account.calendarType == calendarTypes.APPLE) {
        this.openRemoveDialog()
      } else if (this.account.calendarType == calendarTypes.OUTLOOK) {
        this.openRemoveDialog()
      }
    },
    /**
     * The shared half of the two toggles below (G1): identify the account, then
     * either persist the change or hand it to the parent.
     *
     * `syncWithBackend` is the whole difference between the two modes — in
     * Settings this component owns the account and POSTs; in the event flow the
     * parent is collecting changes to apply later, so it only emits.
     *
     * Kept local to this component rather than folded into the
     * calendarOptionSync mixin the two switches share: those PATCH one route
     * with a merged object, these POST two different routes with if/else
     * semantics, so a common helper would be parameters all the way down.
     */
    toggleAccount(route, event, fields) {
      const payload = {
        email: this.account.email,
        calendarType: this.account.calendarType,
        ...fields,
      }
      if (this.syncWithBackend) {
        post(route, payload)
          .then(() => {
            // The server no longer fetches events for a calendar that is
            // toggled off (J8), so re-enabling one has nothing to re-filter —
            // the events were never fetched. Tell whoever owns the calendar
            // events to go and get them. Emitted on every toggle, not just on
            // enable: turning one OFF is still cheaply re-fetched, and the
            // parent decides whether it cares at all (Settings does not).
            this.$emit("calendarsChanged")
          })
          .catch(() => {
            this.showError(
              "There was a problem with toggling your calendar account! Please try again later."
            )
          })
      } else {
        this.$emit(event, payload)
      }
    },
    toggleSubCalendarAccount(enabled, subCalendarId) {
      this.toggleAccount(
        "/user/toggle-sub-calendar",
        "toggleSubCalendarAccount",
        {
          enabled,
          subCalendarId,
        }
      )
    },
    toggleCalendarAccount(enabled) {
      // Collapsing the sub-calendar list is local state, so it happens either
      // way — including in the emit-only mode, where nothing is persisted.
      if (!enabled) this.showSubCalendars = false

      this.toggleAccount("/user/toggle-calendar", "toggleCalendarAccount", {
        enabled,
      })
    },
    openRemoveDialog() {
      this.$emit("openRemoveDialog", {
        email: this.account.email,
        calendarType: this.account.calendarType,
      })
    },
  },
}
</script>
