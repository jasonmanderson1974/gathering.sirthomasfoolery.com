<template>
  <div>
    <div
      class="tw-flex tw-min-h-[5rem] tw-flex-1 tw-items-center tw-justify-center tw-text-sm sm:tw-mt-0 sm:tw-justify-between"
    >
      <div
        :class="
          state === states.EDIT_AVAILABILITY
            ? 'tw-justify-center'
            : 'tw-justify-between'
        "
        class="tw-flex tw-flex-1 tw-flex-wrap tw-gap-x-4 tw-gap-y-2 tw-py-4 sm:tw-justify-start sm:tw-gap-x-4"
      >
        <!-- Select timezone -->
        <div v-if="!event.daysOnly" class="tw-flex tw-items-center tw-gap-2">
          <TimezoneSelector
            class="tw-w-full sm:tw-w-[unset]"
            :value="curTimezone"
            :reference-date="timezoneReferenceDate"
            @input="(val) => $emit('update:curTimezone', val)"
          />
          <v-select
            :value="timeType"
            @input="$emit('update:timeType', $event)"
            :items="timeTypeOptions"
            :menu-props="{ auto: true }"
            item-text="label"
            item-value="value"
            class="tw-z-20 -tw-mt-px tw-w-16 tw-text-sm"
            dense
            hide-details
          />
        </div>
        <div
          v-if="isPhone && !event.daysOnly"
          class="tw-flex tw-basis-full tw-items-center tw-gap-x-2 tw-py-4"
        >
          Show
          <v-select
            :value="mobileNumDays"
            @input="$emit('update:mobileNumDays', $event)"
            :items="mobileNumDaysOptions"
            :menu-props="{ auto: true }"
            item-text="label"
            item-value="value"
            class="-tw-mt-px tw-flex-none tw-shrink tw-basis-24 tw-text-sm"
            dense
            hide-details
          />
          at a time
        </div>

        <template v-if="state !== states.EDIT_AVAILABILITY && isPhone">
          <EventOptions
            class="tw-mt-2 tw-w-full"
            :event="event"
            :showBestTimes="showBestTimes"
            @update:showBestTimes="(val) => $emit('update:showBestTimes', val)"
            :hideIfNeeded="hideIfNeeded"
            @update:hideIfNeeded="(val) => $emit('update:hideIfNeeded', val)"
            :showEventOptions="showEventOptions"
            @toggleShowEventOptions="$emit('toggleShowEventOptions')"
            :startCalendarOnMonday="startCalendarOnMonday"
            @update:startCalendarOnMonday="
              (val) => $emit('update:startCalendarOnMonday', val)
            "
            :numResponses="numResponses"
          />
        </template>
        <template
          v-if="state === states.EDIT_AVAILABILITY && isWeekly && !isPhone"
        >
          <v-spacer />
          <div class="tw-min-w-fit">
            <GCalWeekSelector
              v-if="calendarPermissionGranted"
              :week-offset="weekOffset"
              :event="event"
              @update:weekOffset="(val) => $emit('update:weekOffset', val)"
              :start-on-monday="event.startOnMonday"
            />
          </div>
        </template>
      </div>

      <div
        v-if="showScheduleEventButton"
        style="width: 181.5px"
        class="tw-hidden sm:tw-flex"
      >
        <template v-if="state !== states.SCHEDULE_EVENT">
          <!-- A gathering time is already confirmed: show it + a menu to change/cancel -->
          <v-menu v-if="event.scheduledEvent" offset-y class="tw-z-20">
            <template v-slot:activator="{ on, attrs }">
              <v-btn
                outlined
                class="tw-w-full tw-text-brass"
                v-bind="attrs"
                v-on="on"
              >
                <v-icon small>mdi-calendar-check</v-icon>
                <span class="tw-ml-2 tw-truncate">Gathering set</span>
                <v-icon small right>mdi-chevron-down</v-icon>
              </v-btn>
            </template>
            <v-list dense>
              <v-list-item two-line class="tw-pointer-events-none">
                <v-list-item-content>
                  <v-list-item-title>{{
                    scheduledGatheringText
                  }}</v-list-item-title>
                  <v-list-item-subtitle v-if="reminderSummaryText">
                    {{ reminderSummaryText }}
                  </v-list-item-subtitle>
                </v-list-item-content>
              </v-list-item>
              <v-divider />
              <v-list-item :href="icsUrl">
                <v-icon small class="tw-mr-2">mdi-calendar-plus</v-icon>
                <v-list-item-title>Add to calendar</v-list-item-title>
              </v-list-item>
              <v-list-item @click="(e) => $emit('scheduleEvent', e)">
                <v-list-item-title>Reschedule</v-list-item-title>
              </v-list-item>
              <v-list-item @click="() => $emit('cancelGathering')">
                <v-list-item-title class="tw-text-red">
                  Cancel gathering
                </v-list-item-title>
              </v-list-item>
            </v-list>
          </v-menu>
          <v-btn
            v-else
            outlined
            class="tw-w-full tw-text-brass"
            @click="(e) => $emit('scheduleEvent', e)"
          >
            <v-icon small>mdi-calendar-check</v-icon>
            <span class="tw-ml-2">Schedule event</span>
          </v-btn>
        </template>
        <template v-else>
          <v-btn
            outlined
            class="tw-mr-1 tw-text-red"
            @click="(e) => $emit('cancelScheduleEvent', e)"
          >
            Cancel
          </v-btn>
          <v-menu offset-y class="tw-z-20">
            <template v-slot:activator="{ on, attrs }">
              <v-btn
                :disabled="!allowScheduleEvent"
                class="tw-bg-brass tw-text-wood-deep"
                v-bind="attrs"
                v-on="on"
              >
                Schedule
              </v-btn>
            </template>
            <v-list dense>
              <!-- Pre-gathering reminder options (persisted on confirm) -->
              <v-list-item @click.stop>
                <v-checkbox
                  v-model="reminderEnabledLocal"
                  label="Email reminder to respondents"
                  dense
                  hide-details
                  class="tw-mt-0 tw-pt-0"
                />
              </v-list-item>
              <v-list-item v-if="reminderEnabledLocal" @click.stop>
                <v-select
                  :value="reminderLeadTimeHours"
                  :items="reminderLeadTimeOptions"
                  dense
                  hide-details
                  label="Send reminder"
                  @change="(v) => $emit('update:reminderLeadTimeHours', v)"
                />
              </v-list-item>
              <!-- Recurrence (C5): repeat this gathering on a cadence -->
              <v-list-item @click.stop>
                <v-select
                  :value="recurrenceFrequency"
                  :items="recurrenceOptions"
                  dense
                  hide-details
                  label="Repeat"
                  @change="(v) => $emit('update:recurrenceFrequency', v)"
                />
              </v-list-item>
              <!-- Venue for the confirmed gathering (optional) -->
              <v-list-item @click.stop>
                <LocationInput
                  :value="location"
                  label="Location"
                  placeholder="Where? (optional)"
                  dense
                  hide-icon
                  @input="(v) => $emit('update:location', v)"
                />
              </v-list-item>
              <v-divider />
              <!-- Save the confirmed gathering to Gathering (no external calendar).
                   Members can add it to their own calendar afterwards via the
                   "Add to calendar" (.ics) item on the confirmed gathering, which
                   carries the recurrence rule. -->
              <v-list-item @click="() => $emit('saveScheduleEvent')">
                <v-icon small class="tw-mr-2 tw-text-brass"
                  >mdi-calendar-check</v-icon
                >
                <v-list-item-content>
                  <v-list-item-title class="tw-text-brass"
                    >Save gathering</v-list-item-title
                  >
                </v-list-item-content>
              </v-list-item>
            </v-list>
          </v-menu>
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import TimezoneSelector from "./TimezoneSelector.vue"
import GCalWeekSelector from "./GCalWeekSelector.vue"
import { isPhone } from "@/utils"
import EventOptions from "./EventOptions.vue"
import LocationInput from "@/components/LocationInput.vue"
import { timeTypes, isOwnerlessEvent } from "@/constants"
import { mapState } from "vuex"
import {
  recurrenceLabel,
  reminderSummaryText,
  icsUrl,
} from "@/components/event/gatheringSummary"

export default {
  name: "ToolRow",

  props: {
    event: { type: Object, required: true },
    state: { type: String, required: true },
    states: { type: Object, required: true },
    curTimezone: { type: Object, required: true },
    startCalendarOnMonday: { type: Boolean, default: false },
    showBestTimes: { type: Boolean, required: true },
    hideIfNeeded: { type: Boolean, required: true },
    isWeekly: { type: Boolean, required: true },
    calendarPermissionGranted: { type: Boolean, required: true },
    weekOffset: { type: Number, required: true },
    timezoneReferenceDate: { type: Date, required: false, default: null },
    numResponses: { type: Number, required: true },
    mobileNumDays: { type: Number, default: 3 }, // The number of days to show at a time on mobile
    allowScheduleEvent: { type: Boolean, required: true },
    showEventOptions: { type: Boolean, required: true },
    timeType: { type: String, required: true },
    reminderEnabled: { type: Boolean, default: true },
    reminderLeadTimeHours: { type: Number, default: 24 },
    recurrenceFrequency: { type: String, default: "none" },
    location: { type: String, default: "" },
  },

  components: {
    TimezoneSelector,
    LocationInput,
    GCalWeekSelector,
    EventOptions,
  },

  data: () => ({
    mobileNumDaysOptions: [
      { label: "3 days", value: 3 },
      { label: "7 days", value: 7 },
    ],
    timeTypeOptions: [
      { label: "12h", value: timeTypes.HOUR12 },
      { label: "24h", value: timeTypes.HOUR24 },
    ],
  }),

  computed: {
    ...mapState(["authUser"]),
    isPhone() {
      return isPhone(this.$vuetify)
    },
    guestEvent() {
      return isOwnerlessEvent(this.event)
    },
    isOwner() {
      return this.event.ownerId == this.authUser?._id
    },
    showScheduleEventButton() {
      return (
        !this.event.daysOnly &&
        this.numResponses > 0 &&
        this.state !== this.states.EDIT_AVAILABILITY &&
        (this.guestEvent || this.isOwner)
      )
    },
    // Two-way proxy so the menu checkbox writes back to ScheduleOverlap state
    reminderEnabledLocal: {
      get() {
        return this.reminderEnabled
      },
      set(v) {
        this.$emit("update:reminderEnabled", v)
      },
    },
    reminderLeadTimeOptions() {
      return [
        { text: "12 hours before", value: 12 },
        { text: "24 hours before", value: 24 },
        { text: "48 hours before", value: 48 },
      ]
    },
    recurrenceOptions() {
      return [
        { text: "Does not repeat", value: "none" },
        { text: "Weekly", value: "weekly" },
        { text: "Every 2 weeks", value: "biweekly" },
        { text: "Monthly", value: "monthly" },
      ]
    },
    // Human label for the event's stored recurrence (shown in the summary).
    recurrenceLabel() {
      return recurrenceLabel(this.event)
    },
    scheduledGatheringText() {
      const s = this.event.scheduledEvent
      if (!s || !s.startDate) return ""
      const opts = {
        weekday: "short",
        month: "short",
        day: "numeric",
        hour: "numeric",
        minute: "2-digit",
      }
      try {
        return new Date(s.startDate).toLocaleString([], {
          ...opts,
          timeZone: this.curTimezone?.value,
        })
      } catch (e) {
        return new Date(s.startDate).toLocaleString([], opts)
      }
    },
    reminderSummaryText() {
      return reminderSummaryText(this.event)
    },
    // Universal .ics download for the confirmed gathering (served by getEventIcs)
    icsUrl() {
      return icsUrl(this.event)
    },
  },
}
</script>
