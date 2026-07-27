<template>
  <div class="tw-fixed tw-bottom-0 tw-z-20 tw-flex tw-w-full tw-flex-col">
    <div
      class="tw-flex tw-h-[4rem] tw-w-full tw-items-center tw-px-4"
      :class="`${isIOS ? 'tw-pb-2' : ''} ${
        isScheduling ? 'tw-bg-blue' : 'tw-bg-brass'
      }`"
    >
      <template v-if="!isEditing && !isScheduling">
        <v-btn
          v-if="!event.daysOnly && numResponses > 0"
          text
          class="tw-text-white"
          @click="$emit('schedule-event')"
          >Schedule</v-btn
        >
        <v-spacer />
        <v-btn
          v-if="!authUser && selectedGuestRespondent"
          class="tw-bg-leather tw-text-brass tw-transition-opacity"
          :style="{ opacity: availabilityBtnOpacity }"
          @click="$emit('edit-guest-availability')"
        >
          {{ mobileGuestActionButtonText }}
        </v-btn>
        <v-btn
          v-else
          class="tw-bg-leather tw-text-brass tw-transition-opacity"
          :disabled="loading && !userHasResponded"
          :style="{ opacity: availabilityBtnOpacity }"
          @click="$emit('add-availability')"
        >
          {{ mobileActionButtonText }}
        </v-btn>
      </template>
      <template v-else-if="isEditing">
        <v-btn text class="tw-text-white" @click="$emit('cancel-editing')">
          Cancel
        </v-btn>
        <v-spacer />
        <v-btn
          class="tw-bg-leather tw-text-brass"
          @click="$emit('save-changes')"
        >
          Save
        </v-btn>
      </template>
      <template v-else-if="isScheduling">
        <v-btn
          text
          class="tw-text-white"
          @click="$emit('cancel-schedule-event')"
        >
          Cancel
        </v-btn>
        <v-spacer />
        <v-btn
          :disabled="!allowScheduleEvent"
          class="tw-bg-leather tw-text-brass"
          @click="$emit('save-schedule-event')"
        >
          Save
        </v-btn>
      </template>
    </div>
  </div>
</template>

<script>
import { isIOS } from "@/utils"
import { mapState } from "vuex"

/**
 * Fixed bottom action bar shown on phones: schedule / mark availability in
 * the default state, Save/Cancel while editing, Schedule/Cancel while
 * scheduling. Extracted from Event.vue (TODO A11, Tier 2) — purely
 * presentational; all state stays in Event.vue, which handles the emitted
 * events. The parent gates rendering (phone + not setting specific times).
 */
export default {
  name: "EventBottomBar",

  props: {
    event: { type: Object, required: true },
    isSignUp: { type: Boolean, default: false },
    isEditing: { type: Boolean, default: false },
    isScheduling: { type: Boolean, default: false },
    numResponses: { type: Number, default: 0 },
    selectedGuestRespondent: { default: null },
    availabilityBtnOpacity: { type: Number, default: 1 },
    loading: { type: Boolean, default: false },
    userHasResponded: { type: Boolean, default: false },
    allowScheduleEvent: { type: Boolean, default: false },
  },

  emits: [
    "schedule-event",
    "edit-guest-availability",
    "add-availability",
    "cancel-editing",
    "save-changes",
    "cancel-schedule-event",
    "save-schedule-event",
  ],

  computed: {
    ...mapState(["authUser"]),
    isIOS() {
      return isIOS()
    },
    mobileGuestActionButtonText() {
      return this.event.blindAvailabilityEnabled
        ? "Edit availability"
        : `Edit ${this.selectedGuestRespondent}'s availability`
    },
    mobileActionButtonText() {
      if (this.isSignUp) return "Edit slots"
      return this.userHasResponded ? "Edit availability" : "Mark availability"
    },
  },
}
</script>
