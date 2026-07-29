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

      <!-- Join sign up slot dialog-->
      <SignUpForSlotDialog
        v-if="currSignUpBlock"
        v-model="signUpForSlotDialog"
        :signUpBlock="currSignUpBlock"
        @submit="signUpForBlock"
        :event="event"
      />

      <!-- Edit event dialog -->
      <NewDialog
        v-model="editEventDialog"
        :type="eventType"
        :event="event"
        :contactsPayload="contactsPayload"
        edit
        no-tabs
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
            <v-btn
              text
              color="primary"
              @click="
                () => {
                  saveChanges(true)
                  this.pagesNotVisitedDialog = false
                }
              "
              >Add anyways</v-btn
            >
          </v-card-actions>
        </v-card>
      </v-dialog>

      <div
        class="tw-mx-auto tw-mt-4 lg:tw-flex lg:tw-items-start lg:tw-justify-center lg:tw-gap-6"
      >
        <div class="tw-mx-auto tw-max-w-5xl tw-flex-1">
          <div v-if="!isSettingSpecificTimes" class="tw-mx-4">
            <!-- Title, chips, date, and action buttons -->
            <EventHeader
              :event="event"
              :canEdit="canEdit"
              :isSignUp="isSignUp"
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
              :event.sync="event"
              :canEdit="event.ownerId != 0 && canEdit"
            />

            <!-- Venue / location (C12) -->
            <EventLocation
              :event.sync="event"
              :canEdit="event.ownerId != 0 && canEdit"
            />

            <!-- RSVP to the confirmed gathering (C1) -->
            <GatheringRsvp
              v-if="event.scheduledEvent"
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
            :fromEditEvent="fromEditEvent"
            :loadingCalendarEvents="loading"
            :calendarEventsMap="calendarEventsMap"
            :calendarPermissionGranted="calendarPermissionGranted"
            :weekOffset.sync="weekOffset"
            :curGuestId="curGuestId"
            :initial-timezone="initialTimezone"
            :addingAvailabilityAsGuest="addingAvailabilityAsGuest"
            @addAvailability="addAvailability"
            @addAvailabilityAsGuest="addAvailabilityAsGuest"
            @refreshEvent="refreshEvent"
            @highlightAvailabilityBtn="highlightAvailabilityBtn"
            @deleteAvailability="deleteAvailability"
            @setCurGuestId="(id) => (curGuestId = id)"
            @signUpForBlock="initiateSignUpFlow"
          />

          <!-- Discussion thread (C7) alongside the shared lists (F14). Two
               columns from lg up; below that the lists stack under the
               discussion. -->
          <div
            v-if="!isSettingSpecificTimes"
            class="tw-mx-4 lg:tw-flex lg:tw-items-start lg:tw-gap-6"
          >
            <div class="tw-min-w-0 lg:tw-w-2/3">
              <EventComments
                :event="event"
                @add-comment="onAddComment"
                @edit-comment="onEditComment"
                @delete-comment="onDeleteComment"
                @tag-thread="onTagThread"
                @set-thread-members-only="onSetThreadMembersOnly"
                @untag-thread="onUntagThread"
              />
            </div>
            <div class="tw-min-w-0 lg:tw-w-1/3">
              <EventLists
                :event="event"
                @create-list="onCreateList"
                @rename-list="onRenameList"
                @delete-list="onDeleteList"
                @add-item="onAddListItem"
                @edit-item="onEditListItem"
                @delete-item="onDeleteListItem"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="tw-h-8"></div>
      <!-- Bottom bar for phones -->
      <EventBottomBar
        v-if="!isSettingSpecificTimes && isPhone && (!isSignUp || canEdit)"
        :event="event"
        :isSignUp="isSignUp"
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
  post,
  signInGoogle,
  signInOutlook,
  isPhone,
  processEvent,
  getCalendarEventsMap,
  getDateRangeStringForEvent,
  isDstObserved,
  doesDstExist,
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
import SignUpForSlotDialog from "@/components/sign_up_form/SignUpForSlotDialog.vue"
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
import EventComments from "@/components/event/EventComments.vue"
import EventPolls from "@/components/event/EventPolls.vue"
import EventLists from "@/components/event/EventLists.vue"
import {
  setRsvp,
  clearRsvp,
  addComment,
  editComment,
  deleteComment,
  tagThread,
  setThreadMembersOnly,
  untagThread,
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
} from "@/utils/services/EventService"
import pluginMessagesMixin from "@/components/event/pluginMessagesMixin"
export default {
  name: "Event",

  mixins: [pluginMessagesMixin],

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
    SignUpForSlotDialog,
    ScheduleOverlap,
    NewDialog,
    SignInNotSupportedDialog,
    MarkAvailabilityDialog,
    EventHeader,
    EventBottomBar,
    EventDescription,
    EventLocation,
    GatheringRsvp,
    EventComments,
    EventPolls,
    EventLists,
  },

  data: () => ({
    fromEditEvent: false,

    choiceDialog: false,
    webviewDialog: false,
    guestDialog: false,
    signUpForSlotDialog: false,
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
    currSignUpBlock: null,

    // Shared lists (F15): guards refreshLists against overlapping calls.
    refreshingLists: false,
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
    ...mapState(["authUser", "events"]),
    ...mapGetters(["canInvite"]),
    allowScheduleEvent() {
      return this.scheduleOverlapComponent?.allowScheduleEvent
    },
    calendarTypes() {
      return calendarTypes
    },
    dateString() {
      return getDateRangeStringForEvent(this.event)
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
    isSignUp() {
      return this.event?.isSignUpForm
    },
    eventType() {
      if (this.isSignUp) return "signup"
      else return "event"
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
      if (this.isSignUp) return "Edit slots"
      else if (this.userHasResponded) return "Edit availability"
      return "Mark availability"
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
        if (!this.userHasResponded && !this.isSignUp) {
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

      if (!this.isSignUp)
        this.scheduleOverlapComponent.resetCurUserAvailability()
      else this.scheduleOverlapComponent.resetSignUpForm()
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
        this.showError("Could not change who can see this thread. Please try again.")
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
        this.$set(this.event, "lists", await getLists(id))
      } catch (err) {
        this.showError("Could not refresh the lists. Please try again.")
      } finally {
        this.refreshingLists = false
      }
    },

    /** Refresh event details */
    async refreshEvent() {
      let sanitizedId = this.eventId.replaceAll(".", "")

      let resolvedLongId = this.event?._id || ""
      try {
        const ids = await get(`/events/${sanitizedId}/ids`)
        if (ids?.longId) {
          resolvedLongId = ids.longId
        }
      } catch (err) {
        // If ID resolution fails, continue with existing fallback behavior.
      }
      // The ?guestName= identity parameter is gone: the server keys the viewer
      // off the session now, and honouring a name from the client was how blind
      // availability could be read incognito.
      void resolvedLongId
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

      let changesPersisted = true

      if (this.isSignUp) {
        changesPersisted =
          await this.scheduleOverlapComponent.submitNewSignUpBlocks()
      } else {
        await this.scheduleOverlapComponent.submitAvailability()
      }

      if (changesPersisted) {
        this.showInfo("Changes saved!")
        this.scheduleOverlapComponent.stopEditing()
      }
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
    cancelScheduleEvent() {
      this.scheduleOverlapComponent?.cancelScheduleEvent()
    },
    saveScheduleEvent() {
      this.scheduleOverlapComponent?.saveScheduleEvent()
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

    // -----------------------------------
    //#region Sign Up Form
    // -----------------------------------

    initiateSignUpFlow(signUpBlock) {
      this.currSignUpBlock = signUpBlock
      this.signUpForSlotDialog = true
    },

    async signUpForBlock() {
      const payload = {
        guest: false,
        signUpBlockIds: [this.currSignUpBlock._id],
      }

      await post(`/events/${this.event._id}/response`, payload)
      await this.refreshEvent()

      this.scheduleOverlapComponent.resetSignUpForm()
      this.signUpForSlotDialog = false
    },

    //#endregion
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

  beforeDestroy() {
    window.removeEventListener("beforeunload", this.onBeforeUnload)
    window.removeEventListener("message", this.handleMessage)
  },

  watch: {
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
