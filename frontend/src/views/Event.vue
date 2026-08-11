<template>
  <span>
    <div v-if="event" class="tw-mt-8 tw-h-full">
      <!-- Mark availability option dialog -->
      <MarkAvailabilityDialog
        v-model="choiceDialog"
        :initialState="linkApple ? 'create_account_apple' : 'choices'"
        @signInLinkApple="signInLinkApple"
        @allowGoogleCalendar="
          () => setAvailabilityAutomatically(calendarTypes.GOOGLE)
        "
        @allowOutlookCalendar="
          () => setAvailabilityAutomatically(calendarTypes.OUTLOOK)
        "
        @setAvailabilityManually="setAvailabilityManually"
        @addedAppleCalendar="addedAppleCalendar"
        @addedICSCalendar="addedICSCalendar"
      />

      <!-- Google sign in not supported dialog -->
      <SignInNotSupportedDialog v-model="webviewDialog" />

      <!-- Guest dialog -->
      <GuestDialog
        v-model="guestDialog"
        @submit="handleGuestDialogSubmit"
        :event="event"
        :respondents="Object.keys(event.responses)"
      />

      <!-- Edit event dialog -->
      <NewDialog
        v-model="editEventDialog"
        :event="event"
        :contactsPayload="contactsPayload"
        edit
      />

      <!-- Pages Not Visited dialog -->
      <v-dialog
        v-model="pagesNotVisitedDialog"
        max-width="400"
        content-class="tw-m-0"
      >
        <v-card>
          <v-card-title>Are you sure?</v-card-title>
          <v-card-text
            ><span class="tw-font-medium"
              >You're about to add your availability without filling out all
              pages of this Gathering.</span
            >
            Click the left and right arrows at the top to switch between
            pages.</v-card-text
          >
          <v-card-actions>
            <v-spacer />
            <v-btn text @click="pagesNotVisitedDialog = false">Cancel</v-btn>
            <v-btn text color="primary" @click="confirmSaveWithPagesUnvisited"
              >Add anyways</v-btn
            >
          </v-card-actions>
        </v-card>
      </v-dialog>

      <div
        class="tw-mx-auto tw-mt-4 lg:tw-flex lg:tw-items-start lg:tw-justify-center lg:tw-gap-6"
      >
        <!-- The gathering at a glance, once a time is confirmed and the
             calendar has collapsed (F21). Only exists wide enough for two
             columns; narrower than that the same three panels render inline in
             the column below, under the title where they read in order. -->
        <div
          v-if="showGatheringSidebar"
          class="tw-mx-4 lg:tw-sticky lg:tw-top-16 lg:tw-order-2 lg:tw-w-72 lg:tw-flex-none"
        >
          <GatheringSummary
            :event="event"
            :canEdit="canEdit"
            @reschedule="rescheduleGathering"
            @cancel-gathering="cancelGathering"
          />

          <EventLocation
            v-model:event="event"
            :canEdit="event.ownerId != 0 && canEdit"
          />

          <GatheringRsvp
            :event="event"
            @set-rsvp="setRsvp"
            @clear-rsvp="clearRsvp"
          />
        </div>

        <div class="tw-mx-auto tw-max-w-5xl tw-flex-1 lg:tw-order-1">
          <div v-if="!isSettingSpecificTimes" class="tw-mx-4">
            <!-- Title, chips, date, and action buttons -->
            <EventHeader
              :event="event"
              :canEdit="canEdit"
              :isEditing="isEditing"
              :dateString="dateString"
              :actionButtonText="actionButtonText"
              :loading="loading"
              :userHasResponded="userHasResponded"
              :selectedGuestRespondent="selectedGuestRespondent"
              :availabilityBtnOpacity="availabilityBtnOpacity"
              @edit-event="editEvent"
              @copy-link="copyLink"
              @edit-guest-availability="editGuestAvailability"
              @add-availability="addAvailability"
              @cancel-editing="cancelEditing"
              @save-changes="saveChanges"
            />

            <!-- Description -->
            <EventDescription
              v-model:event="event"
              :canEdit="event.ownerId != 0 && canEdit"
            />

            <!-- The confirmed gathering, the venue (C12) and the RSVP panel
                 (C1). All three live in the sidebar once the calendar
                 collapses, and inline here when it doesn't — gated so they
                 render in exactly one place, never both. Expanding the
                 availability grid must not make the confirmed time disappear. -->
            <GatheringSummary
              v-if="isScheduled && !showGatheringSidebar"
              :event="event"
              :canEdit="canEdit"
              @reschedule="rescheduleGathering"
              @cancel-gathering="cancelGathering"
            />

            <EventLocation
              v-if="!showGatheringSidebar"
              v-model:event="event"
              :canEdit="event.ownerId != 0 && canEdit"
            />

            <GatheringRsvp
              v-if="event.scheduledEvent && !showGatheringSidebar"
              :event="event"
              @set-rsvp="setRsvp"
              @clear-rsvp="clearRsvp"
            />

            <!-- Venue / activity polls (C6) -->
            <EventPolls
              :event="event"
              @create-poll="onCreatePoll"
              @delete-poll="onDeletePoll"
              @vote-poll="onVotePoll"
            />
          </div>

          <!-- Calendar -->

          <ScheduleOverlap
            ref="scheduleOverlap"
            :event="event"
            :collapsed="calendarCollapsed"
            :fromEditEvent="fromEditEvent"
            :loadingCalendarEvents="loading"
            :calendarEventsMap="calendarEventsMap"
            :calendarPermissionGranted="calendarPermissionGranted"
            v-model:weekOffset="weekOffset"
            :curGuestId="curGuestId"
            :initial-timezone="initialTimezone"
            :addingAvailabilityAsGuest="addingAvailabilityAsGuest"
            @addAvailability="addAvailability"
            @addAvailabilityAsGuest="addAvailabilityAsGuest"
            @refreshEvent="refreshEvent"
            @highlightAvailabilityBtn="highlightAvailabilityBtn"
            @deleteAvailability="deleteAvailability"
            @setCurGuestId="(id) => (curGuestId = id)"
            @calendarsChanged="fetchAuthUserCalendarEvents"
          />

          <!-- Who owes whom (F22), in the content column's right-hand strip
               rather than the page sidebar — so it continues straight down from
               the calendar's own Responses / Options column, which is flush to
               the same right edge and the same width.

               Derived from the same rows the Settle Up tab renders, so it is
               right whether or not that tab has ever been opened. Below the
               breakpoint there is no strip and it falls back to the top of the
               tab; `showSettleUpColumn` is what the tab is told, so the two can
               never both render. -->
          <div v-if="showSettleUpColumn" class="tw-flex tw-justify-end">
            <!-- Flush to the same right edge as Responses, but wider than it:
                 at the Responses column's own 208px the panel truncated names
                 ("Jason Ander…") and wrapped every figure onto two lines. The
                 right edge is what reads as the alignment; the extra width
                 comes off the empty space to its left. -->
            <div class="tw-w-72">
              <SettleUpSummary :expenses="expenses" />
            </div>
          </div>

          <!-- The discussion (C7) and the shared lists (F14) as two tabs over
               one full-width band (F16). They were side by side at 2/3 + 1/3,
               which left both cramped and put the lists below the entire
               discussion on a phone. Panels use v-show, not v-if, so drafts and
               what you had expanded survive a switch back and forth. -->
          <div v-if="!isSettingSpecificTimes" class="tw-mx-4">
            <!-- Availability is still there once a time is set, just out of the
                 way. Anyone can look; only a manager can act on it. -->
            <v-btn
              v-if="isScheduled"
              text
              small
              class="tw-mb-1 tw-text-xs tw-text-parchment-dim"
              @click="calendarExpanded = !calendarExpanded"
            >
              <v-icon small left>{{
                calendarExpanded ? "mdi-chevron-up" : "mdi-chevron-down"
              }}</v-icon>
              {{ calendarExpanded ? "Hide availability" : "View availability" }}
            </v-btn>

            <!-- tw-flex-wrap, from F22 on: five tabs no longer fit across a
                 390px phone, and an unwrapped row pushed the whole page into a
                 horizontal scroll — visible on every tab, not just the new
                 one. Wrapping to a second line beats a scroll nobody would
                 find. -->
            <div class="tw-flex tw-flex-wrap tw-gap-1">
              <v-btn
                v-for="t in bandTabs"
                :key="t.value"
                text
                small
                :class="`tw-text-xs tw-transition-all ${
                  t.value === bandTab
                    ? 'tw-bg-brass/10 tw-text-brass'
                    : 'tw-text-parchment-dim'
                }`"
                @click="bandTab = t.value"
              >
                {{ t.title }}
              </v-btn>
            </div>

            <div v-show="bandTab === 'discussion'">
              <EventComments
                :event="event"
                :mentionables="mentionables"
                @add-comment="onAddComment"
                @edit-comment="onEditComment"
                @delete-comment="onDeleteComment"
                @tag-thread="onTagThread"
                @set-thread-members-only="onSetThreadMembersOnly"
                @untag-thread="onUntagThread"
              />
            </div>
            <div v-show="bandTab === 'lists'">
              <EventLists
                :lists="sharedLists"
                :can-manage="canManageLists"
                :refreshing="refreshingLists"
                @refresh="refreshLists"
                @create-list="onCreateList"
                @rename-list="onRenameList"
                @delete-list="onDeleteList"
                @add-item="onAddListItem"
                @edit-item="onEditListItem"
                @delete-item="onDeleteListItem"
                @move-item="onMoveListItem"
                @toggle-item-checked="onToggleListItemChecked"
              />
            </div>
            <!-- The two private tabs (F19/F20). Both panels own their own
                 fetching, so each is told only whether it is the one on screen.
                 v-if on authUser as well as the tab: without an account there
                 is nothing to key a private document to. -->
            <div v-show="bandTab === 'my-lists'">
              <PersonalLists
                v-if="authUser"
                :event-id="bandEventId"
                :active="bandTab === 'my-lists'"
                @loaded="(n) => (personalListCount = n)"
              />
            </div>
            <div v-show="bandTab === 'my-notes'">
              <PersonalNotes
                v-if="authUser"
                :event-id="bandEventId"
                :active="bandTab === 'my-notes'"
              />
            </div>
            <!-- The shared ledger (F22). Unlike the two private tabs this one
                 is readable by everyone signed in, guests included — the v-if
                 is on having an account at all, not on a role. -->
            <div v-show="bandTab === 'settle-up'">
              <EventExpenses
                v-if="authUser"
                :event-id="bandEventId"
                :expenses="expenses"
                :refreshing="refreshingExpenses"
                :has-sidebar="showSettleUpColumn"
                @create-expense="onCreateExpense"
                @edit-expense="onEditExpense"
                @delete-expense="onDeleteExpense"
                @delete-receipt="onDeleteReceipt"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="tw-h-8"></div>
      <!-- Bottom bar for phones -->
      <EventBottomBar
        v-if="!isSettingSpecificTimes && isPhone"
        :event="event"
        :isEditing="isEditing"
        :isScheduling="isScheduling"
        :numResponses="numResponses"
        :selectedGuestRespondent="selectedGuestRespondent"
        :availabilityBtnOpacity="availabilityBtnOpacity"
        :loading="loading"
        :userHasResponded="userHasResponded"
        :allowScheduleEvent="allowScheduleEvent"
        @schedule-event="scheduleEvent"
        @edit-guest-availability="editGuestAvailability"
        @add-availability="addAvailability"
        @cancel-editing="cancelEditing"
        @save-changes="saveChanges"
        @cancel-schedule-event="cancelScheduleEvent"
        @save-schedule-event="saveScheduleEvent"
      />
    </div>
  </span>
</template>

<script>
import {
  get,
  signInGoogle,
  signInOutlook,
  isPhone,
  processEvent,
  getCalendarEventsMap,
  getDateRangeStringForEvent,
  getStartEndDateString,
  isDstObserved,
  doesDstExist,
  viewportWidth,
} from "@/utils"
import { mapActions, mapState, mapMutations, mapGetters } from "vuex"
import dayjs from "dayjs"
import utcPlugin from "dayjs/plugin/utc"
import timezonePlugin from "dayjs/plugin/timezone"
dayjs.extend(utcPlugin)
dayjs.extend(timezonePlugin)

import NewDialog from "@/components/NewDialog.vue"
import ScheduleOverlap from "@/components/schedule_overlap/ScheduleOverlap.vue"
import GuestDialog from "@/components/GuestDialog.vue"
import {
  errors,
  authTypes,
  eventTypes,
  calendarTypes,
  isOwnerlessEvent,
} from "@/constants"
import isWebview from "is-ua-webview"
import SignInNotSupportedDialog from "@/components/SignInNotSupportedDialog.vue"
import MarkAvailabilityDialog from "@/components/calendar_permission_dialogs/MarkAvailabilityDialog.vue"
import EventHeader from "@/components/event/EventHeader.vue"
import EventBottomBar from "@/components/event/EventBottomBar.vue"
import EventDescription from "@/components/event/EventDescription.vue"
import EventLocation from "@/components/event/EventLocation.vue"
import GatheringRsvp from "@/components/event/GatheringRsvp.vue"
import GatheringSummary from "@/components/event/GatheringSummary.vue"
import EventComments from "@/components/event/EventComments.vue"
import EventPolls from "@/components/event/EventPolls.vue"
import EventLists from "@/components/event/EventLists.vue"
import PersonalLists from "@/components/event/PersonalLists.vue"
import PersonalNotes from "@/components/event/PersonalNotes.vue"
import EventExpenses from "@/components/event/EventExpenses.vue"
import SettleUpSummary from "@/components/event/SettleUpSummary.vue"
import { canManageEventLists } from "@/components/event/eventLists"
import {
  setRsvp,
  clearRsvp,
  addComment,
  editComment,
  deleteComment,
  tagThread,
  setThreadMembersOnly,
  untagThread,
  getMentionables,
  createPoll,
  deletePoll,
  votePoll,
  getLists,
  createList,
  renameList,
  deleteList,
  addListItem,
  editListItem,
  deleteListItem,
  moveListItem,
  setListItemChecked,
} from "@/utils/services/EventService"
import pluginMessagesMixin from "@/components/event/pluginMessagesMixin"
import expensesMixin from "@/components/event/expensesMixin"
export default {
  name: "Event",

  mixins: [pluginMessagesMixin, expensesMixin],

  props: {
    eventId: { type: String, required: true },
    fromSignIn: { type: Boolean, default: false },
    editingMode: { type: Boolean, default: false },
    linkApple: { type: Boolean, default: false },
    initialTimezone: { type: Object, default: () => ({}) },
    contactsPayload: { type: Object, default: () => ({}) },
  },

  components: {
    GuestDialog,
    ScheduleOverlap,
    NewDialog,
    SignInNotSupportedDialog,
    MarkAvailabilityDialog,
    EventHeader,
    EventBottomBar,
    EventDescription,
    EventLocation,
    GatheringRsvp,
    GatheringSummary,
    EventComments,
    EventPolls,
    EventLists,
    PersonalLists,
    PersonalNotes,
    EventExpenses,
    SettleUpSummary,
  },

  data: () => ({
    fromEditEvent: false,

    choiceDialog: false,
    webviewDialog: false,
    guestDialog: false,
    editEventDialog: false,
    pagesNotVisitedDialog: false,

    loading: true,
    calendarEventsMap: {},
    event: null,
    scheduleOverlapComponent: null,
    scheduleOverlapComponentLoaded: false,

    curGuestId: "", // Id of the current guest being edited
    calendarPermissionGranted: true,
    addingAvailabilityAsGuest: false, // Whether a signed in user is current adding availability as a guest

    weekOffset: 0,

    availabilityBtnOpacity: 1,
    hasRefetchedAuthUserCalendarEvents: false,

    // Sign Up Forms

    // Shared lists (F15/F16): the band's active tab, and a guard that keeps
    // refreshLists from overlapping itself.
    bandTab: "discussion",
    refreshingLists: false,

    // The private tabs (F19/F20). Only the count comes back up here — the
    // panels own everything else about themselves.
    personalListCount: 0,

    // Who the discussion may @mention (F9), fetched once per event.
    mentionables: [],

    // Whether the user has asked to see the availability grid again after it
    // collapsed on a confirmed gathering (F21). Not persisted — landing on the
    // page should always show the gathering, not the poll it came from.
    calendarExpanded: false,
  }),

  mounted() {
    // If coming from enabling contacts, show the dialog. Checks if contactsPayload is not an Observer.
    this.editEventDialog = Object.keys(this.contactsPayload).length > 0
    // If coming from signing in to link apple calendar, show the mark availability dialog
    if (this.linkApple) {
      this.choiceDialog = true
    }
  },

  computed: {
    ...mapState(["authUser"]),
    ...mapGetters(["canInvite", "canManageUsers"]),
    /**
     * A computed, not an inline `event.lists ?? []` in the template: an inline
     * fallback allocates a fresh array on every render, which changes the
     * prop's identity every time and would fire EventLists' `lists` watcher
     * continuously.
     */
    sharedLists() {
      return this.event?.lists ?? []
    },
    canManageLists() {
      return canManageEventLists({
        authUser: this.authUser,
        event: this.event,
        canManageUsers: this.canManageUsers,
        canInvite: this.canInvite,
      })
    },
    /** What the band's panels call the event — short id where there is one. */
    bandEventId() {
      return this.event?.shortId ?? this.event?._id ?? ""
    },
    /**
     * The band's tabs (F16, extended by F19/F20). Counts are the only sign that
     * there is anything behind the tab you aren't looking at, so they matter
     * more here than they would beside an always-visible panel. Omitted at zero.
     */
    bandTabs() {
      const count = (n) => (n ? ` (${n})` : "")
      const tabs = [
        {
          value: "discussion",
          title: `Discussion${count((this.event?.comments ?? []).length)}`,
        },
        {
          value: "lists",
          title: `Lists${count((this.event?.lists ?? []).length)}`,
        },
      ]

      // Any signed-in user, guests included. Absent when signed out — there is
      // no private document without an account to key it to.
      if (this.authUser) {
        tabs.push({
          value: "my-lists",
          title: `My Lists${count(this.personalListCount)}`,
        })
        // No count: a character total would be noise, and "you have a note" is
        // not news to the only person who can read it.
        tabs.push({ value: "my-notes", title: "My Notes" })
        // The shared ledger (F22), to the right of the private tabs. Signed-in
        // rather than member+: a guest reads the ledger, they just can't add
        // to it.
        tabs.push({
          value: "settle-up",
          title: `Settle Up${count(this.expenses.length)}`,
        })
      }

      return tabs
    },
    allowScheduleEvent() {
      return this.scheduleOverlapComponent?.allowScheduleEvent
    },
    calendarTypes() {
      return calendarTypes
    },
    /**
     * The line under the title. Once a time is confirmed it shows that time —
     * before this it kept showing the polling window ("Mar 3 - Mar 17") long
     * after the answer was known, which read as though nothing had been decided.
     */
    dateString() {
      const scheduled = this.event.scheduledEvent
      if (scheduled?.startDate && scheduled?.endDate) {
        return getStartEndDateString(
          new Date(scheduled.startDate),
          new Date(scheduled.endDate)
        )
      }
      return getDateRangeStringForEvent(this.event)
    },
    isScheduled() {
      return !!this.event?.scheduledEvent
    },
    /**
     * Once a gathering is confirmed the availability grid has done its job, so
     * it gets out of the way and the discussion and lists move up (F21).
     *
     * The three state guards matter: anything that drives the calendar —
     * editing availability, picking a time, setting specific times — must
     * reveal it, or the phone's bottom bar would be operating a grid nobody
     * can see.
     */
    calendarCollapsed() {
      return (
        this.isScheduled &&
        !this.calendarExpanded &&
        !this.isEditing &&
        !this.isScheduling &&
        !this.isSettingSpecificTimes
      )
    },
    /**
     * Whether the summary / venue / RSVP panels get their own column on the
     * right, or render inline under the title instead.
     *
     * Measured against Tailwind's `lg` (1024px) rather than a Vuetify
     * breakpoint, because the column itself is laid out by `lg:` classes and
     * the two scales don't line up — Vuetify's own `lg` starts at 1264.
     * Stacking the sidebar above everything on a narrow screen would put the
     * summary above the gathering's own title.
     */
    showGatheringSidebar() {
      return this.calendarCollapsed && viewportWidth(this.$vuetify) >= 1024
    },
    /**
     * Whether the balances get their own right-hand strip in the content
     * column, rather than sitting at the top of the Settle Up tab.
     *
     * Deliberately NOT tied to the gathering sidebar: money gets spent on a
     * gathering that is still being arranged, so the balances appear as soon as
     * there is a ledger, whether or not a time has been confirmed.
     *
     * Gated on there being expenses at all, so an empty panel doesn't conjure a
     * column onto every gathering; on `authUser`, because every expense route is
     * behind a session; and on the same 1024px breakpoint the sidebar uses,
     * below which a 208px strip would be unreadable.
     */
    showSettleUpColumn() {
      return (
        !!this.authUser &&
        this.expenses.length > 0 &&
        viewportWidth(this.$vuetify) >= 1024
      )
    },
    isEditing() {
      return this.scheduleOverlapComponent?.editing
    },
    isScheduling() {
      return this.scheduleOverlapComponent?.scheduling
    },
    isOwnerlessEvent() {
      return isOwnerlessEvent(this.event)
    },
    canEdit() {
      if (this.authUser?._id === this.event.ownerId) return true
      // Legacy ownerless events: manageable by member+, matching the server's
      // requireEventManager. Anonymous callers can't reach this page at all now.
      return this.isOwnerlessEvent && this.canInvite
    },
    isPhone() {
      return isPhone(this.$vuetify)
    },
    isSpecificDates() {
      return this.event?.type === eventTypes.SPECIFIC_DATES || !this.event?.type
    },
    isWeekly() {
      return this.event?.type === eventTypes.DOW
    },
    areUnsavedChanges() {
      return this.scheduleOverlapComponent?.unsavedChanges
    },
    userHasResponded() {
      return this.authUser?._id in this.event.responses
    },
    selectedGuestRespondent() {
      return this.scheduleOverlapComponent?.selectedGuestRespondent
    },
    numResponses() {
      return this.scheduleOverlapComponent?.respondents.length
    },
    actionButtonText() {
      return this.userHasResponded ? "Edit availability" : "Mark availability"
    },
    isSettingSpecificTimes() {
      return (
        this.scheduleOverlapComponent?.state ===
        this.scheduleOverlapComponent?.states.SET_SPECIFIC_TIMES
      )
    },
  },

  methods: {
    ...mapActions(["showError", "showInfo", "getEvents"]),
    ...mapMutations(["setAuthUser"]),

    /**
     * "Add anyways" on the unvisited-pages warning: save, then dismiss.
     *
     * Extracted from an inline handler that referred to `this` inside a
     * template expression. That worked under Vue 2 and still happens to work
     * under Vue 3's Options API, but only because the compiled render function
     * is invoked with the instance proxy — it is not something to rely on.
     */
    confirmSaveWithPagesUnvisited() {
      this.saveChanges(true)
      this.pagesNotVisitedDialog = false
    },

    /** Show choice dialog if not signed in, otherwise, immediately start editing availability */
    addAvailability() {
      if (!this.scheduleOverlapComponent) return

      // Start editing immediately if days only
      if (this.event?.daysOnly) {
        this.scheduleOverlapComponent.startEditing()
        return
      }

      // Start editing if calendar permission granted or user has responded, otherwise show choice dialog
      if (
        (this.authUser && this.calendarPermissionGranted) ||
        this.userHasResponded
      ) {
        this.scheduleOverlapComponent.startEditing()
        if (!this.userHasResponded) {
          this.scheduleOverlapComponent.setAvailabilityAutomatically()
        }
      } else {
        this.choiceDialog = true
      }
    },
    /** Add guest availability while signed in */
    addAvailabilityAsGuest() {
      this.addingAvailabilityAsGuest = true
      this.setAvailabilityManually()
    },
    cancelEditing() {
      /* Cancels editing and resets availability to previous */
      if (!this.scheduleOverlapComponent) return

      this.scheduleOverlapComponent.resetCurUserAvailability()
      this.scheduleOverlapComponent.stopEditing()
      this.curGuestId = ""
      this.addingAvailabilityAsGuest = false
    },
    copyLink() {
      /* Copies event link to clipboard */
      navigator.clipboard.writeText(
        `${window.location.origin}/e/${this.event.shortId ?? this.event._id}`
      )
      this.showInfo("Link copied to clipboard!")
    },
    async deleteAvailability() {
      if (!this.scheduleOverlapComponent) return

      if (this.addingAvailabilityAsGuest) {
        if (this.curGuestId) {
          await this.scheduleOverlapComponent.deleteAvailability(
            this.curGuestId
          )
          this.curGuestId = ""
        }
      } else {
        await this.scheduleOverlapComponent.deleteAvailability()
      }

      this.showInfo("Availability deleted!")
      this.scheduleOverlapComponent.stopEditing()
    },

    editEvent() {
      /* Show edit event dialog */
      this.editEventDialog = true
    },
    /** Persist an RSVP to the confirmed gathering, then refresh (C1). */
    async setRsvp(payload) {
      const id = this.event.shortId ?? this.event._id
      try {
        await setRsvp(id, payload)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not save your RSVP. Please try again.")
      }
    },
    /** Remove the caller's RSVP, then refresh (C1). */
    async clearRsvp() {
      const id = this.event.shortId ?? this.event._id
      try {
        await clearRsvp(id)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not update your RSVP. Please try again.")
      }
    },

    /** Discussion thread (C7): persist then refresh. */
    async onAddComment(payload) {
      const id = this.event.shortId ?? this.event._id
      try {
        await addComment(id, payload)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not post your message. Please try again.")
      }
    },
    async onEditComment({ commentId, payload }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await editComment(id, commentId, payload)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not save your edit. Please try again.")
      }
    },
    async onDeleteComment({ commentId }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await deleteComment(id, commentId)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not delete the message. Please try again.")
      }
    },

    /** Threads within the discussion (C13): persist then refresh. */
    async onTagThread({ commentId, payload }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await tagThread(id, commentId, payload)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not create the thread. Please try again.")
      }
    },
    async onSetThreadMembersOnly({ commentId, payload }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await setThreadMembersOnly(id, commentId, payload)
        await this.refreshEvent()
      } catch (err) {
        this.showError(
          "Could not change who can see this thread. Please try again."
        )
      }
    },
    async onUntagThread({ commentId }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await untagThread(id, commentId)
        await this.refreshEvent()
      } catch (err) {
        // 409 means someone replied between the page load and the click.
        this.showError(
          err?.error === "thread-has-replies"
            ? "This thread has replies, so it can no longer be un-tagged."
            : "Could not un-tag the thread. Please try again."
        )
      }
    },

    /** Venue / activity polls (C6): persist then refresh. */
    async onCreatePoll(payload) {
      const id = this.event.shortId ?? this.event._id
      try {
        await createPoll(id, payload)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not create the poll. Please try again.")
      }
    },
    async onDeletePoll(pollId) {
      const id = this.event.shortId ?? this.event._id
      try {
        await deletePoll(id, pollId)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not delete the poll. Please try again.")
      }
    },
    async onVotePoll({ pollId, payload }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await votePoll(id, pollId, payload)
        await this.refreshEvent()
      } catch (err) {
        this.showError("Could not save your vote. Please try again.")
      }
    },

    /**
     * Shared lists (F14): persist then refresh.
     *
     * Unlike comments and polls these refresh through refreshLists, not
     * refreshEvent — several people work a list at once and the whole-event
     * fetch resolves a user per response before it ever reaches the lists.
     */
    async onCreateList(payload) {
      const id = this.event.shortId ?? this.event._id
      try {
        await createList(id, payload)
        await this.refreshLists()
      } catch (err) {
        this.showError("Could not create the list. Please try again.")
      }
    },
    async onRenameList({ listId, payload }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await renameList(id, listId, payload)
        await this.refreshLists()
      } catch (err) {
        this.showError("Could not rename the list. Please try again.")
      }
    },
    async onDeleteList(listId) {
      const id = this.event.shortId ?? this.event._id
      try {
        await deleteList(id, listId)
        await this.refreshLists()
      } catch (err) {
        this.showError("Could not delete the list. Please try again.")
      }
    },
    async onAddListItem({ listId, payload }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await addListItem(id, listId, payload)
        await this.refreshLists()
      } catch (err) {
        this.showError("Could not add that entry. Please try again.")
      }
    },
    async onEditListItem({ listId, itemId, payload }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await editListItem(id, listId, itemId, payload)
        await this.refreshLists()
      } catch (err) {
        this.showError("Could not save that entry. Please try again.")
      }
    },
    async onDeleteListItem({ listId, itemId }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await deleteListItem(id, listId, itemId)
        await this.refreshLists()
      } catch (err) {
        this.showError("Could not remove that entry. Please try again.")
      }
    },

    /**
     * Persist a drag (F18). The refetch is what makes the new arrangement real:
     * vuedraggable has already moved the row in the DOM, so a failure here has
     * to be corrected by the truth coming back from the server rather than by
     * putting the row back by hand.
     */
    async onMoveListItem({ listId, itemId, payload }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await moveListItem(id, listId, itemId, payload)
        await this.refreshLists()
      } catch (err) {
        this.showError("Could not move that entry. Please try again.")
        await this.refreshLists()
      }
    },

    async onToggleListItemChecked({ listId, itemId, checked }) {
      const id = this.event.shortId ?? this.event._id
      try {
        await setListItemChecked(id, listId, itemId, checked)
        await this.refreshLists()
      } catch (err) {
        this.showError("Could not update that entry. Please try again.")
      }
    },

    /**
     * Fetch the people this caller may @mention (F9).
     *
     * Once per page load, not per keystroke: the roll is ~40 people and the
     * picker has to open on the first `@` without waiting on the network. A
     * failure is swallowed — the composer still posts comments, and a
     * hand-written token still notifies, so a missing picker is a degraded
     * convenience rather than a broken discussion.
     */
    async fetchMentionables() {
      const id = this.event?.shortId ?? this.event?._id
      if (!id) return
      try {
        this.mentionables = (await getMentionables(id)) ?? []
      } catch (err) {
        this.mentionables = []
      }
    },

    /**
     * Refresh only the lists (F15).
     *
     * Splicing into this.event is safe: processEvent never touches lists, and
     * EventLists reads them straight off the prop. The busy flag drops
     * overlapping calls — selecting the Lists tab and expanding a list can
     * easily land in the same moment.
     */
    async refreshLists() {
      if (this.refreshingLists) return
      this.refreshingLists = true
      const id = this.event.shortId ?? this.event._id
      try {
        this.event = { ...this.event, lists: await getLists(id) }
      } catch (err) {
        this.showError("Could not refresh the lists. Please try again.")
      } finally {
        this.refreshingLists = false
      }
    },

    /** Refresh event details */
    async refreshEvent() {
      let sanitizedId = this.eventId.replaceAll(".", "")

      // The ?guestName= identity parameter is gone: the server keys the viewer
      // off the session now, and honouring a name from the client was how blind
      // availability could be read incognito.
      //
      // No /ids call first (J3): its result was assigned and then discarded,
      // left over from the guestName-era code, and because the two awaits were
      // sequential it put a whole round-trip in front of the grid's first paint.
      // The server's GET /events/:eventId/ids endpoint stays — this was its last
      // caller in the app, but it is a documented part of the API surface.
      this.event = await get(`/events/${sanitizedId}`)
      processEvent(this.event)
    },

    setAvailabilityAutomatically(calendarType = calendarTypes.GOOGLE) {
      /* Prompts user to sign in when "set availability automatically" button clicked */
      if (isWebview(navigator.userAgent)) {
        // Show dialog prompting user to use a real browser
        this.webviewDialog = true
      } else {
        // Or sign in if user is already using a real browser
        let signInParams
        if (this.authUser) {
          // Request permission if calendar permissions not yet granted
          signInParams = {
            state: {
              type: authTypes.EVENT_ADD_AVAILABILITY,
              eventId: this.eventId,
            },
            selectAccount: false,
            requestCalendarPermission: true,
          }
        } else {
          // Ask the user to select the account they want to sign in with if not logged in yet
          signInParams = {
            state: {
              type: authTypes.EVENT_ADD_AVAILABILITY,
              eventId: this.eventId,
            },
            selectAccount: true,
            requestCalendarPermission: true,
          }
        }

        if (calendarType === calendarTypes.GOOGLE) {
          signInGoogle(signInParams)
        } else if (calendarType === calendarTypes.OUTLOOK) {
          signInOutlook(signInParams)
        }
      }
      this.choiceDialog = false
    },
    setAvailabilityManually() {
      /* Starts editing after "set availability manually" button clicked */
      if (!this.scheduleOverlapComponent) return

      this.$nextTick(() => {
        this.scheduleOverlapComponent.startEditing()
      })
      this.choiceDialog = false
    },
    editGuestAvailability() {
      /* Edits the selected guest's availability */
      if (!this.scheduleOverlapComponent) return

      this.curGuestId = this.selectedGuestRespondent
      this.scheduleOverlapComponent.startEditing()
      this.$nextTick(() => {
        this.scheduleOverlapComponent.populateUserAvailability(
          this.selectedGuestRespondent
        )
      })
    },

    async saveChanges(ignorePagesNotVisited = false) {
      /* Saves the viewer's availability, or — when they've deliberately started
         an on-behalf entry — prompts for the name it belongs to. */
      if (!this.scheduleOverlapComponent) return

      // If user hasn't responded and they haven't gone to the next page, show pages not visited dialog
      if (
        !this.userHasResponded &&
        this.curGuestId.length === 0 &&
        !this.scheduleOverlapComponent.pageHasChanged &&
        !ignorePagesNotVisited &&
        this.scheduleOverlapComponent.hasPages
      ) {
        this.pagesNotVisitedDialog = true
        return
      }

      if (this.addingAvailabilityAsGuest) {
        if (this.curGuestId) {
          this.saveChangesAsGuest({
            name: this.curGuestId,
            email: this.event.responses[this.curGuestId].email,
          })
          this.curGuestId = ""
          this.addingAvailabilityAsGuest = false
        } else {
          this.guestDialog = true
        }
        return
      }

      await this.scheduleOverlapComponent.submitAvailability()

      this.showInfo("Changes saved!")
      this.scheduleOverlapComponent.stopEditing()
    },
    async saveChangesAsGuest(payload) {
      /* On-behalf entry: a signed-in member submitting availability under a
         plain name, for someone without an account. The server validates the
         name (it rejects ObjectID-shaped ones, which could overwrite a member). */
      if (!this.scheduleOverlapComponent) return

      if (payload.name.length > 0) {
        await this.scheduleOverlapComponent.submitAvailability(payload)

        this.showInfo("Changes saved!")
        this.scheduleOverlapComponent.resetCurUserAvailability()
        this.scheduleOverlapComponent.stopEditing()
        this.guestDialog = false
        this.addingAvailabilityAsGuest = false
      }
    },

    scheduleEvent() {
      this.scheduleOverlapComponent?.scheduleEvent()
    },
    /**
     * Reschedule from the summary card. The grid has to be back in the DOM
     * before SCHEDULE_EVENT is entered, hence the nextTick — otherwise the user
     * lands in a picking state with nothing to pick on.
     */
    rescheduleGathering() {
      this.calendarExpanded = true
      this.$nextTick(() => this.scheduleOverlapComponent?.scheduleEvent())
    },
    cancelGathering() {
      this.scheduleOverlapComponent?.cancelGathering()
      this.calendarExpanded = false
    },
    cancelScheduleEvent() {
      this.scheduleOverlapComponent?.cancelScheduleEvent()
    },
    saveScheduleEvent() {
      this.scheduleOverlapComponent?.saveScheduleEvent()
      // Confirming a time is the moment the grid stops being the point of the
      // page, whether the user got here from "View availability" or Reschedule.
      this.calendarExpanded = false
    },

    highlightAvailabilityBtn() {
      this.availabilityBtnOpacity = 0.1
      setTimeout(() => {
        this.availabilityBtnOpacity = 1
        setTimeout(() => {
          this.availabilityBtnOpacity = 0.1
          setTimeout(() => {
            this.availabilityBtnOpacity = 1
          }, 100)
        }, 100)
      }, 100)
    },

    /** Sign in with google to link apple calendar */
    signInLinkApple() {
      if (isWebview(navigator.userAgent)) {
        // Show dialog prompting user to use a real browser
        this.webviewDialog = true
      } else {
        signInGoogle({
          state: {
            type: authTypes.EVENT_SIGN_IN_LINK_APPLE,
            eventId: this.eventId,
          },
          selectAccount: true,
        })
      }
    },
    /** Called when user adds apple calendar account */
    addedAppleCalendar() {
      this.choiceDialog = false
      this.scheduleOverlapComponent?.startEditing()
      this.scheduleOverlapComponent?.setAvailabilityAutomatically()
    },
    /** Called when user adds ICS calendar account */
    addedICSCalendar() {
      this.choiceDialog = false
      this.scheduleOverlapComponent?.startEditing()
      this.scheduleOverlapComponent?.setAvailabilityAutomatically()
    },

    /** Fetch current user's calendar events */
    async fetchAuthUserCalendarEvents() {
      if (!this.authUser) {
        this.calendarPermissionGranted = false
        return
      }

      const curWeekOffset = this.weekOffset
      return getCalendarEventsMap(this.event, { weekOffset: curWeekOffset })
        .then((eventsMap) => {
          // Don't set calendar events / set availability if user has already
          // selected a different weekoffset by the time these calendar events load
          if (curWeekOffset !== this.weekOffset) return

          this.calendarEventsMap = eventsMap

          // Fix DST bug
          if (this.event.type === eventTypes.DOW) {
            for (const calendarId in this.calendarEventsMap) {
              for (const index in this.calendarEventsMap[calendarId]
                .calendarEvents) {
                const event =
                  this.calendarEventsMap[calendarId].calendarEvents[index]
                const startDate = new Date(event.startDate)
                const endDate = new Date(event.endDate)
                if (doesDstExist(startDate) && !isDstObserved(startDate)) {
                  startDate.setHours(startDate.getHours() - 1)
                  endDate.setHours(endDate.getHours() - 1)
                }
                this.calendarEventsMap[calendarId].calendarEvents[
                  index
                ].startDate = startDate.toISOString()
                this.calendarEventsMap[calendarId].calendarEvents[
                  index
                ].endDate = endDate.toISOString()
              }
            }
          }

          // Set user availability automatically if we're in editing mode and they haven't responded
          if (
            this.authUser &&
            this.isEditing &&
            !this.userHasResponded &&
            !this.areUnsavedChanges &&
            this.scheduleOverlapComponent
          ) {
            this.$nextTick(() => {
              this.scheduleOverlapComponent?.setAvailabilityAutomatically()
            })
          }

          // calendar permission granted is false when every calendar in the calendar map has an error, true otherwise
          this.calendarPermissionGranted = !Object.values(
            this.calendarEventsMap
          ).every((c) => Boolean(c.error))

          if (!this.hasRefetchedAuthUserCalendarEvents) {
            const hasAtLeastOneError = Object.values(
              this.calendarEventsMap
            ).some((c) => Boolean(c.error))

            // Refetch calendar if there is an error
            if (hasAtLeastOneError) {
              this.hasRefetchedAuthUserCalendarEvents = true
              setTimeout(() => {
                this.fetchAuthUserCalendarEvents()
              }, 1000)
            }
          }
        })
        .catch((err) => {
          console.error(err)
          this.calendarPermissionGranted = false
        })
    },

    /** Refreshes calendar avaliabilities and fetches current user calendar events */
    refreshCalendar() {
      const promises = []
      promises.push(this.fetchAuthUserCalendarEvents())

      const curWeekOffset = this.weekOffset
      this.loading = true
      Promise.allSettled(promises).then(() => {
        // Only set loading to false if promises resolved at the same week offset they were fetched at
        // i.e. no new promises are currently being run
        if (curWeekOffset === this.weekOffset) {
          this.loading = false
        }
      })
    },

    /** Resets week offset to 0 */
    resetWeekOffset() {
      this.weekOffset = 0
    },

    onBeforeUnload(e) {
      if (this.areUnsavedChanges) {
        e.preventDefault()
        e.returnValue = ""
        return
      }

      delete e["returnValue"]
    },

    handleGuestDialogSubmit(guestPayload) {
      this.saveChangesAsGuest(guestPayload)
    },
  },

  async created() {
    window.addEventListener("beforeunload", this.onBeforeUnload)
    window.addEventListener("message", this.handleMessage)

    // Get event details
    try {
      await this.refreshEvent()

      const fromEditEvent = localStorage.getItem(
        `from-edit-event-${this.event._id}`
      )
      if (fromEditEvent) {
        localStorage.removeItem(`from-edit-event-${this.event._id}`)
        this.fromEditEvent = true
      }
    } catch (err) {
      switch (err.error) {
        case errors.EventNotFound:
          this.showError("The specified event does not exist!")
          this.$router.replace({ name: "home" })
          return
      }
    }

    // Not awaited and not in `promises`: the mention picker is a convenience on
    // one tab of the page, so it must never hold up the calendar grid.
    this.fetchMentionables()

    // Nor is the ledger (F22) — but it IS read on arrival rather than when its
    // tab is opened, because the sidebar summary has to be right for someone
    // who never opens it. Guarded on already knowing who the viewer is; the
    // profile fetch below covers the case where we don't yet, via the authUser
    // watcher.
    if (this.authUser) this.refreshExpenses()

    const promises = []
    promises.push(this.fetchAuthUserCalendarEvents())

    this.loading = true
    Promise.allSettled(promises).then(() => {
      this.loading = false
    })

    get("/user/profile")
      .then((authUser) => {
        this.setAuthUser(authUser)
      })
      .catch(() => {
        this.setAuthUser(null)
      })
  },

  beforeUnmount() {
    window.removeEventListener("beforeunload", this.onBeforeUnload)
    window.removeEventListener("message", this.handleMessage)
  },

  watch: {
    // Selecting the Lists tab refetches them. The panels are kept alive with
    // v-show, so there is no created() hook to hang this off — and someone
    // opening the tab is exactly when the data most wants to be current.
    bandTab(tab) {
      if (tab === "lists") {
        this.refreshLists()
      }
      // The ledger is loaded on arrival for the sidebar's sake, so this is a
      // refresh rather than the first read — someone opening the tab is still
      // when it most wants to be current.
      if (tab === "settle-up") {
        this.refreshExpenses()
      }
      // The two private panels refresh themselves off their `active` prop —
      // they own their own data, so there is nothing to fetch from here.
    },
    authUser(now, before) {
      // Signing out, or a session expiring, while a signed-in-only tab is
      // selected must not leave the band pointing at a panel that no longer
      // exists.
      if (
        !now &&
        ["my-lists", "my-notes", "settle-up"].includes(this.bandTab)
      ) {
        this.bandTab = "discussion"
        this.personalListCount = 0
      }
      // The ledger goes with the session: every expense route needs one, and
      // leaving the rows behind would keep stale balances in the sidebar.
      // Only fetched on the falsy→truthy edge, so learning who the viewer is
      // after arrival loads it once rather than twice.
      if (!now) {
        this.expenses = []
      } else if (!before) {
        this.refreshExpenses()
      }
    },
    event() {
      if (this.event) {
        this.resetWeekOffset()
        this.$nextTick(() => {
          this.scheduleOverlapComponent = this.$refs.scheduleOverlap
        })
        document.title = `${this.event.name} · The Fellowship`
      }
    },
    scheduleOverlapComponent() {
      if (!this.scheduleOverlapComponentLoaded) {
        this.scheduleOverlapComponentLoaded = true

        // Put into editing mode if just signed in
        if (this.fromSignIn || this.editingMode) {
          this.scheduleOverlapComponent.startEditing()
        }
      }
    },
    weekOffset() {
      this.refreshCalendar()
    },
    [`authUser.calendarAccounts`]() {
      this.fetchAuthUserCalendarEvents()
    },
  },
}
</script>
