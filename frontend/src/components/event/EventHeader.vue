<template>
  <div class="tw-flex tw-items-center tw-text-parchment">
    <div>
      <div
        class="sm:mb-2 tw-flex tw-flex-wrap tw-items-center tw-gap-x-4 tw-gap-y-2"
      >
        <div
          class="tw-text-xl sm:tw-text-3xl"
          :class="
            canEdit &&
            '-tw-mx-2 -tw-my-1 tw-cursor-pointer tw-rounded tw-px-2 tw-py-1 tw-transition-all hover:tw-bg-leather'
          "
          @click="canEdit && $emit('edit-event')"
        >
          {{ event.name }}
        </div>
        <v-chip
          v-if="event.when2meetHref?.length > 0"
          :href="`https://when2meet.com${event.when2meetHref}`"
          :small="isPhone"
          class="tw-cursor-pointer tw-select-none tw-rounded tw-bg-leather tw-px-2 tw-font-medium sm:tw-px-3"
          >Imported from when2meet</v-chip
        >
        <!-- "Add to calendar" and the recurrence label used to live here as
             chips. Once a gathering is confirmed they are both on the
             GatheringSummary card in the sidebar, a few centimetres away, so
             repeating them in the header was pure noise. -->
      </div>
      <div class="tw-flex tw-items-baseline tw-gap-1">
        <div
          class="tw-text-sm tw-font-normal tw-text-parchment-dim sm:tw-text-base"
        >
          {{ dateString }}
        </div>
        <template v-if="canEdit">
          <v-btn
            id="edit-event-btn"
            @click="$emit('edit-event')"
            class="tw-px-2 tw-text-sm tw-text-brass"
            text
          >
            Edit event
          </v-btn>
        </template>
      </div>
    </div>
    <v-spacer />
    <div class="tw-flex tw-flex-row tw-items-center tw-gap-2.5">
      <div>
        <v-btn
          :icon="isPhone"
          :outlined="!isPhone"
          class="tw-text-brass"
          @click="$emit('copy-link')"
        >
          <span v-if="!isPhone" class="tw-mr-2 tw-text-brass">Copy link</span>
          <v-icon class="tw-text-brass" v-if="!isPhone"
            >mdi-content-copy</v-icon
          >
          <v-icon class="tw-text-brass" v-else>mdi-share</v-icon>
        </v-btn>
      </div>
      <div v-if="!isPhone" class="tw-flex tw-w-40">
        <template v-if="!isEditing">
          <v-btn
            width="10.25rem"
            class="tw-text-white tw-transition-opacity"
            :class="'tw-bg-brass'"
            :disabled="loading && !userHasResponded"
            :style="{ opacity: availabilityBtnOpacity }"
            @click="$emit('add-availability')"
          >
            {{ actionButtonText }}
          </v-btn>
        </template>
        <template v-else>
          <v-btn
            class="tw-mr-1 tw-w-20 tw-text-red"
            @click="$emit('cancel-editing')"
            outlined
          >
            Cancel
          </v-btn>
          <v-btn
            class="tw-w-20 tw-text-white"
            :class="'tw-bg-brass'"
            @click="$emit('save-changes')"
          >
            Save
          </v-btn></template
        >
      </div>
    </div>
  </div>
</template>

<script>
import { isPhone } from "@/utils"
import { mapState } from "vuex"

/**
 * Event page header: title (+ when2meet chip), date
 * string with Edit button, and the action buttons (copy link / refresh /
 * today, mark availability, save/cancel while editing). Extracted from
 * Event.vue (TODO A11, Tier 2) — purely presentational; all state stays in
 * Event.vue, which handles the emitted events.
 */
export default {
  name: "EventHeader",

  props: {
    event: { type: Object, required: true },
    canEdit: { type: Boolean, default: false },
    isEditing: { type: Boolean, default: false },
    dateString: { type: String, default: "" },
    actionButtonText: { type: String, default: "" },
    loading: { type: Boolean, default: false },
    userHasResponded: { type: Boolean, default: false },
    selectedGuestRespondent: { default: null },
    availabilityBtnOpacity: { type: Number, default: 1 },
  },

  emits: [
    "edit-event",
    "copy-link",
    "edit-guest-availability",
    "add-availability",
    "cancel-editing",
    "save-changes",
  ],

  components: {},

  data: () => ({}),

  computed: {
    ...mapState(["authUser"]),
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },
}
</script>
