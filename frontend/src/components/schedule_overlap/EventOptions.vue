<template>
  <ExpandableSection
    v-if="event.daysOnly || numResponses >= 1"
    label="Options"
    :model-value="showEventOptions"
    @input="$emit('toggleShowEventOptions')"
  >
    <div class="tw-flex tw-flex-col tw-gap-4 tw-pt-2">
      <v-switch
        v-if="numResponses > 1 && isPhone"
        inset
        id="show-best-times-toggle"
        :input-value="showBestTimes"
        @change="(val) => $emit('update:showBestTimes', !!val)"
        hide-details
      >
        <template v-slot:label>
          <div class="tw-text-sm tw-text-parchment">
            Show best {{ event.daysOnly ? "days" : "times" }}
          </div>
        </template>
      </v-switch>
      <v-switch
        v-if="numResponses >= 1"
        inset
        id="hide-if-needed-toggle"
        :input-value="hideIfNeeded"
        @change="(val) => $emit('update:hideIfNeeded', !!val)"
        hide-details
      >
        <template v-slot:label>
          <div class="tw-text-sm tw-text-parchment">
            Hide if needed {{ event.daysOnly ? "days" : "times" }}
          </div>
        </template>
      </v-switch>
      <v-switch
        v-if="numResponses >= 1"
        inset
        id="show-response-counts-toggle"
        :input-value="showResponseCounts"
        @change="(val) => $emit('update:showResponseCounts', !!val)"
        hide-details
      >
        <template v-slot:label>
          <div class="tw-text-sm tw-text-parchment">Show response counts</div>
        </template>
      </v-switch>

      <!-- Start on monday -->
      <v-switch
        v-if="event.daysOnly"
        inset
        id="start-calendar-on-monday-toggle"
        :input-value="startCalendarOnMonday"
        @change="(val) => $emit('update:startCalendarOnMonday', !!val)"
        hide-details
      >
        <template v-slot:label>
          <div class="tw-text-sm tw-text-parchment">Start on Monday</div>
        </template>
      </v-switch>
    </div>
  </ExpandableSection>
</template>

<script>
import { isPhone } from "@/utils"
import ExpandableSection from "@/components/ExpandableSection.vue"

export default {
  name: "EventOptions",

  components: {
    ExpandableSection,
  },

  props: {
    event: { type: Object, required: true },
    showBestTimes: { type: Boolean, required: true },
    hideIfNeeded: { type: Boolean, required: true },
    showResponseCounts: { type: Boolean, default: true },
    numResponses: { type: Number, required: true },
    showEventOptions: { type: Boolean, required: true },
    startCalendarOnMonday: { type: Boolean, default: false },
  },

  computed: {
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },
}
</script>
