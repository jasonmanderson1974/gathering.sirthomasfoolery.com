<template>
  <div>
    <div class="tw-flex tw-items-center tw-font-medium">
      <template v-if="!isOwner && event.blindAvailabilityEnabled">
        Your response
      </template>
      <template v-else>
        <div class="tw-mr-1 tw-text-lg">Responses</div>
        <div class="tw-font-normal">
          <template v-if="curRespondents.length === 0">
            {{
              isCurTimeslotSelected
                ? `(${numUsersAvailable}/${respondents.length})`
                : `(${respondents.length})`
            }}
          </template>
          <template v-else>
            {{
              isCurTimeslotSelected
                ? `(${numCurRespondentsAvailable}/${curRespondents.length})`
                : `(${curRespondents.length})`
            }}
          </template>
        </div>
        <template v-if="allowExportCsv">
          <v-spacer />
          <ExportCsvMenu
            :event="event"
            :eventId="eventId"
            :respondents="respondents"
            :parsedResponses="parsedResponses"
            :timezone="timezone"
          />
        </template>
      </template>
      <template v-if="isPhone">
        <v-spacer />
        <div
          class="tw-mt-2 tw-text-sm tw-font-normal tw-text-parchment-dim"
          :class="showIfNeededStar ? 'tw-visible' : 'tw-invisible'"
        >
          * if needed
        </div>
      </template>
    </div>
    <div
      v-if="isOwner && !isPhone && event.blindAvailabilityEnabled"
      class="tw-mb-2 tw-mt-1 tw-text-xs tw-italic tw-text-parchment-dim"
    >
      Responses are only visible to {{ isOwner ? "you" : "event creator" }}
    </div>
    <div
      ref="scrollableSection"
      class="tw-flex tw-flex-col"
      :style="
        maxHeight
          ? `max-height: ${maxHeight}px !important;`
          : !isPhone
          ? `max-height: ${respondentsListMaxHeight}px !important;`
          : ''
      "
    >
      <div
        ref="respondentsScrollView"
        class="-tw-ml-2 tw-pl-2 tw-pt-2 tw-text-sm"
        :class="
          isPhone && !maxHeight
            ? 'tw-overflow-hidden'
            : 'tw-overflow-y-auto tw-overflow-x-hidden'
        "
      >
        <div v-if="respondents.length === 0" class="tw-mb-6">
          <span
            class="tw-text-parchment-dim"
            v-if="!isOwner && event.blindAvailabilityEnabled"
          >
            No response yet!
          </span>
          <span class="tw-text-parchment-dim" v-else>No responses yet!</span>
        </div>
        <template v-else>
          <!-- `tag` is required in Vue 3: without it a transition-group renders
               a fragment, which has no single root to put `class` on — the grid
               classes were being dropped with only a warning to say so. -->
          <transition-group
            tag="div"
            name="list"
            class="tw-grid tw-grid-cols-2 tw-gap-x-2 sm:tw-block"
          >
            <div
              v-for="user in orderedRespondents"
              :key="user._id"
              class="tw-group tw-relative tw-flex tw-cursor-pointer tw-items-center tw-py-1"
              @mouseover="(e) => $emit('mouseOverRespondent', e, user._id)"
              @mouseleave="$emit('mouseLeaveRespondent')"
              @click="(e) => clickRespondent(e, user._id)"
            >
              <div class="tw-relative tw-flex tw-items-center">
                <div class="tw-ml-1 tw-mr-3">
                  <UserAvatarContent
                    v-if="!isGuest(user)"
                    :user="user"
                    :size="16"
                  ></UserAvatarContent>
                  <v-avatar v-else :size="16">
                    <v-icon size="small">mdi-account</v-icon>
                  </v-avatar>
                </div>

                <v-checkbox-btn
                  @click="(e) => $emit('clickRespondent', e, user._id)"
                  color="primary"
                  :model-value="respondentSelected(user._id)"
                  class="tw-absolute -tw-top-[2px] tw-left-0 tw-rounded tw-bg-wood-deep tw-opacity-0 group-hover:tw-opacity-100 group-[&:has(.email-hover-target:hover)]:!tw-opacity-0"
                  :class="
                    respondentSelected(user._id)
                      ? 'tw-opacity-100'
                      : 'tw-opacity-0'
                  "
                />
              </div>
              <div class="tw-flex tw-flex-col">
                <div
                  class="tw-mr-1 tw-transition-all"
                  :class="respondentClass(user._id)"
                >
                  {{
                    displayName(user) +
                    (respondentIfNeeded(user._id) ? "*" : "")
                  }}
                </div>
                <div
                  v-if="isOwner && event.collectEmails"
                  class="email-hover-target tw-flex tw-items-center tw-rounded-sm tw-p-px tw-text-xs tw-text-parchment-dim tw-transition-all hover:tw-bg-leather"
                  :class="respondentClass(user._id)"
                  @mouseover.stop
                  @click.stop="copyEmailToClipboard(user.email)"
                >
                  {{ user.email }}
                  <v-icon class="tw-ml-1 tw-text-xs">mdi-content-copy</v-icon>
                </div>
              </div>
              <div
                class="tw-absolute tw-right-0 tw-transition-none group-hover:tw-opacity-100 group-[&:has(.email-hover-target:hover)]:!tw-opacity-0"
                :class="isPhone ? 'tw-opacity-100' : 'tw-opacity-0'"
              >
                <template v-if="isPhone && (isGuest(user) || isOwner)">
                  <v-menu>
                    <template v-slot:activator="{ props }">
                      <v-btn icon v-bind="props">
                        <v-icon size="small" color="#b8ad97"
                          >mdi-dots-vertical</v-icon
                        >
                      </v-btn>
                    </template>
                    <v-list class="tw-py-1" density="compact">
                      <v-list-item
                        v-if="isGuest(user)"
                        @click="$emit('editGuestAvailability', user._id)"
                      >
                        <v-list-item-title class="tw-flex tw-items-center">
                          <v-icon size="small" class="tw-mr-2" color="#b8ad97"
                            >mdi-pencil</v-icon
                          >
                          Edit
                        </v-list-item-title>
                      </v-list-item>
                      <v-list-item
                        v-if="isOwner"
                        @click="() => showDeleteAvailabilityDialog(user)"
                      >
                        <v-list-item-title class="tw-flex tw-items-center">
                          <v-icon size="small" class="tw-mr-2" color="#b8ad97"
                            >mdi-delete</v-icon
                          >
                          Delete
                        </v-list-item-title>
                      </v-list-item>
                    </v-list>
                  </v-menu>
                </template>
                <template v-else>
                  <v-btn
                    v-if="isGuest(user)"
                    size="small"
                    icon
                    class="tw-bg-leather"
                    @click="$emit('editGuestAvailability', user._id)"
                    ><v-icon size="small" color="#b8ad97"
                      >mdi-pencil</v-icon
                    ></v-btn
                  >
                  <v-btn
                    v-if="isOwner"
                    size="small"
                    icon
                    class="tw-bg-leather"
                    @click="() => showDeleteAvailabilityDialog(user)"
                    ><v-icon
                      size="small"
                      class="hover:tw-text-red"
                      color="#b8ad97"
                      >mdi-delete</v-icon
                    ></v-btn
                  >
                </template>
              </div>
            </div>
          </transition-group>
          <div class="tw-h-2"></div>
        </template>
      </div>
      <div class="tw-relative">
        <OverflowGradient
          v-if="hasMounted && !isPhone"
          class="tw-h-16"
          :scrollContainer="$refs.respondentsScrollView"
          :showArrow="false"
        />
      </div>

      <div
        v-if="!isPhone && respondents.length > 0"
        class="tw-col-span-full tw-mb-2 tw-mt-1 tw-text-sm tw-text-parchment-dim"
        :class="showIfNeededStar ? 'tw-visible' : 'tw-invisible'"
      >
        * if needed
      </div>
      <template v-if="!isPhone">
        <v-btn
          v-if="!event.blindAvailabilityEnabled || isOwner"
          variant="text"
          color="primary"
          class="-tw-ml-2 tw-mb-4 tw-w-min tw-px-2"
          @click="$emit('addAvailabilityAsGuest')"
        >
          {{ "+ Add guest availability" }}</v-btn
        >
        <v-switch
          v-if="respondents.length > 1"
          class="tw-mb-4"
          inset
          id="show-best-times-toggle"
          :model-value="showBestTimes"
          @update:model-value="(val) => $emit('update:showBestTimes', !!val)"
          hide-details
        >
          <template v-slot:label>
            <div class="tw-text-sm tw-text-parchment">
              Show best {{ event.daysOnly ? "days" : "times" }}
            </div>
          </template>
        </v-switch>
        <EventOptions
          :event="event"
          :showEventOptions="showEventOptions"
          @toggleShowEventOptions="$emit('toggleShowEventOptions')"
          :showBestTimes="showBestTimes"
          @update:showBestTimes="(val) => $emit('update:showBestTimes', val)"
          :hideIfNeeded="hideIfNeeded"
          @update:hideIfNeeded="(val) => $emit('update:hideIfNeeded', val)"
          :showResponseCounts="showResponseCounts"
          @update:showResponseCounts="
            (val) => $emit('update:showResponseCounts', val)
          "
          :startCalendarOnMonday="startCalendarOnMonday"
          @update:startCalendarOnMonday="
            (val) => $emit('update:startCalendarOnMonday', val)
          "
          :showCalendarEvents="showCalendarEvents"
          @update:showCalendarEvents="
            (val) => $emit('update:showCalendarEvents', val)
          "
          :hasCalendarEvents="hasCalendarEvents"
          :numResponses="respondents.length"
        />
      </template>
    </div>

    <div
      v-if="(!isOwner || isPhone) && event.blindAvailabilityEnabled"
      class="tw-mt-2 tw-text-xs tw-italic tw-text-parchment-dim"
    >
      Responses are only visible to {{ isOwner ? "you" : "event creator" }}
    </div>

    <v-dialog v-model="deleteAvailabilityDialog" width="500" persistent>
      <v-card>
        <v-card-title>Are you sure?</v-card-title>
        <v-card-text class="tw-text-sm tw-text-parchment-dim"
          >Are you sure you want to delete
          <strong>{{ displayName(userToDelete) }}</strong
          >'s availability from this event?</v-card-text
        >
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="deleteAvailabilityDialog = false"
            >Cancel</v-btn
          >
          <v-btn
            variant="text"
            color="error"
            @click="
              () => {
                deleteAvailability(userToDelete)
                deleteAvailabilityDialog = false
              }
            "
            >Delete</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-btn
      v-if="
        !maxHeight && isPhone && (!event.blindAvailabilityEnabled || isOwner)
      "
      variant="text"
      color="primary"
      class="-tw-ml-2 tw-mt-4 tw-w-min tw-px-2"
      @click="$emit('addAvailabilityAsGuest')"
    >
      + Add guest availability</v-btn
    >
  </div>
</template>

<style scoped>
.list-move {
  transition: transform 0.5s;
}
</style>

<script>
import { _delete, isPhone, displayName } from "@/utils"
import UserAvatarContent from "../UserAvatarContent.vue"
import { mapState, mapActions } from "vuex"
import EventOptions from "./EventOptions.vue"
import OverflowGradient from "@/components/OverflowGradient.vue"
import ExportCsvMenu from "./ExportCsvMenu.vue"

export default {
  name: "RespondentsList",

  components: {
    UserAvatarContent,
    EventOptions,
    OverflowGradient,
    ExportCsvMenu,
  },

  props: {
    eventId: { type: String, required: true },
    event: { type: Object, required: true },
    days: { type: Array, required: true },
    times: { type: Array, required: true },
    curDate: { type: Date, required: false }, // Date of the current timeslot
    curRespondent: { type: String, required: true },
    curRespondents: { type: Array, required: true },
    curTimeslot: { type: Object, required: true },
    curTimeslotAvailability: { type: Object, required: true },
    respondents: { type: Array, required: true },
    parsedResponses: { type: Object, required: true },
    isOwner: { type: Boolean, required: true },
    maxHeight: { type: Number },
    responsesFormatted: { type: Map, required: true },
    timezone: { type: Object, required: true },
    showBestTimes: { type: Boolean, required: true },
    hideIfNeeded: { type: Boolean, required: true },
    showResponseCounts: { type: Boolean, default: true },
    startCalendarOnMonday: { type: Boolean, default: false },
    // Passed straight through to EventOptions. ScheduleOverlap has bound
    // `showCalendarEvents` here since before the Vue 3 migration, but no
    // component ever declared it, so the binding was inert and the value could
    // never leave its initial `false`.
    showCalendarEvents: { type: Boolean, default: false },
    hasCalendarEvents: { type: Boolean, default: false },
    showEventOptions: { type: Boolean, required: true },
    addingAvailabilityAsGuest: { type: Boolean, required: true },
  },

  emits: [
    "mouseOverRespondent",
    "mouseLeaveRespondent",
    "clickRespondent",
    "editGuestAvailability",
    "addAvailabilityAsGuest",
    "update:showBestTimes",
    "toggleShowEventOptions",
    "update:hideIfNeeded",
    "update:showResponseCounts",
    "update:startCalendarOnMonday",
    "update:showCalendarEvents",
    "refreshEvent",
  ],

  data() {
    return {
      deleteAvailabilityDialog: false,
      userToDelete: null,
      desktopMaxHeight: 0,
      respondentsListMinHeight: 400,

      oldCurRespondents: [],
      curRespondentsAddedTime: {}, // Map of respondent id to time they were added

      hasMounted: false,
    }
  },

  computed: {
    ...mapState(["authUser"]),
    allowExportCsv() {
      if (this.isPhone) return false

      return this.event.blindAvailabilityEnabled
        ? this.isOwner && this.respondents.length > 0
        : this.respondents.length > 0
    },
    curRespondentsSet() {
      return new Set(this.curRespondents)
    },
    isCurTimeslotSelected() {
      return (
        this.curTimeslot.dayIndex !== -1 && this.curTimeslot.timeIndex !== -1
      )
    },
    numUsersAvailable() {
      this.curTimeslot
      let numUsers = 0
      for (const key in this.curTimeslotAvailability) {
        if (this.curTimeslotAvailability[key]) numUsers++
      }
      return numUsers
    },
    numCurRespondentsAvailable() {
      this.curTimeslot
      let numUsers = 0
      for (const key in this.curTimeslotAvailability) {
        if (
          this.curTimeslotAvailability[key] &&
          this.curRespondentsSet.has(key)
        )
          numUsers++
      }
      return numUsers
    },
    showIfNeededStar() {
      if (this.hideIfNeeded) {
        return false
      }

      for (const user of this.respondents) {
        if (this.respondentIfNeeded(user._id)) {
          return true
        }
      }
      return false
    },
    isPhone() {
      return isPhone(this.$vuetify)
    },
    orderedRespondents() {
      const orderedRespondents = [...this.respondents]
      orderedRespondents.sort((a, b) => {
        // Sort by added time if both are in curRespondents
        // Sort curRespondents before others
        if (
          this.curRespondentsSet.has(a._id) &&
          this.curRespondentsSet.has(b._id)
        ) {
          return (
            this.curRespondentsAddedTime[a._id] -
            this.curRespondentsAddedTime[b._id]
          )
        } else if (
          this.curRespondentsSet.has(a._id) &&
          !this.curRespondentsSet.has(b._id)
        ) {
          return -1
        } else if (
          !this.curRespondentsSet.has(a._id) &&
          this.curRespondentsSet.has(b._id)
        ) {
          return 1
        }

        // Otherwise, sort by the name as shown — sorting on firstName while
        // rendering a nickname puts nicknamed people in visibly wrong places.
        return displayName(a).localeCompare(displayName(b))
      })
      return orderedRespondents
    },
    respondentsListMaxHeight() {
      return Math.max(this.desktopMaxHeight, this.respondentsListMinHeight)
    },
  },

  methods: {
    ...mapActions(["showError", "showInfo"]),
    displayName,
    /** Emit clickRespondent event */
    clickRespondent(e, userId) {
      e.stopImmediatePropagation()
      this.$emit("clickRespondent", e, userId)
    },
    /** Returns the class of the given respondent */
    respondentClass(id) {
      const c = []
      if (/*this.curRespondent == id ||*/ this.curRespondentsSet.has(id)) {
        // c.push("tw-font-bold")
      } else if (this.curRespondents.length > 0) {
        c.push("tw-text-parchment-dim")
      }

      if (
        (this.curRespondentsSet.has(id) || this.curRespondents.length === 0) &&
        this.respondentIfNeeded(id)
      ) {
        c.push("tw-bg-yellow")
      }

      if (!this.curTimeslotAvailability[id]) {
        c.push("tw-line-through")
        c.push("tw-text-parchment-dim")
      }
      return c
    },
    /** Returns whether the respondent has "ifNeeded" availability for the current timeslot */
    respondentIfNeeded(id) {
      if (!this.curDate || this.hideIfNeeded) return false

      return Boolean(
        this.parsedResponses[id]?.ifNeeded?.has(this.curDate.getTime())
      )
    },
    /** Returns whether the current respondent is selected (for subset avail) */
    respondentSelected(id) {
      return this.curRespondentsSet.has(id)
    },
    /** Returns whether the user is a guest */
    isGuest(user) {
      return user._id == user.firstName
    },
    /** Shows the delete availability dialog */
    showDeleteAvailabilityDialog(user) {
      this.deleteAvailabilityDialog = true
      this.userToDelete = user
    },
    /** Deletes the user's availability on the server */
    async deleteAvailability(user) {
      try {
        await _delete(`/events/${this.eventId}/response`, {
          guest: this.isGuest(user),
          userId: user._id,
          name: user._id,
        })
        this.$emit("refreshEvent")
        this.showInfo("Availability successfully deleted!")
      } catch (e) {
        console.error(e)
        this.showError(
          "There was an error deleting that person's availability!"
        )
      }
    },
    setDesktopMaxHeight() {
      const el = this.$refs.scrollableSection
      if (el) {
        const { top } = el.getBoundingClientRect()
        this.desktopMaxHeight = window.innerHeight - top - 32
      } else {
        this.desktopMaxHeight = 0
      }
    },
    /** Copies the given email to the clipboard */
    async copyEmailToClipboard(email) {
      try {
        await navigator.clipboard.writeText(email)
        this.showInfo("Email copied to clipboard!")
      } catch (err) {
        console.error("Failed to copy email: ", err)
        this.showError("Failed to copy email.")
      }
    },
  },

  mounted() {
    this.setDesktopMaxHeight()

    addEventListener("resize", this.setDesktopMaxHeight)
    // addEventListener("scroll", this.setDesktopMaxHeight)

    this.$nextTick(() => {
      this.hasMounted = true
    })
  },

  beforeUnmount() {
    removeEventListener("resize", this.setDesktopMaxHeight)
    // removeEventListener("scroll", this.setDesktopMaxHeight)
  },

  watch: {
    curRespondents: {
      deep: true,
      handler() {
        const oldSet = new Set(this.oldCurRespondents)
        const newSet = new Set(this.curRespondents)

        // Get added respondents (in newSet but not in oldSet)
        const addedRespondents = this.curRespondents.filter(
          (id) => !oldSet.has(id)
        )

        // Get removed respondents (in oldSet but not in newSet)
        const removedRespondents = this.oldCurRespondents.filter(
          (id) => !newSet.has(id)
        )

        // Update curRespondentsAddedTime. Rebuilt and reassigned once rather
        // than mutated per id — replacing the object is what publishes the
        // added keys and the removed ones in a single update.
        if (addedRespondents.length > 0 || removedRespondents.length > 0) {
          const next = { ...this.curRespondentsAddedTime }
          for (const id of addedRespondents) {
            next[id] = new Date().getTime()
          }
          for (const id of removedRespondents) {
            delete next[id]
          }
          this.curRespondentsAddedTime = next
        }

        this.oldCurRespondents = [...this.curRespondents]
      },
    },
  },
}
</script>
