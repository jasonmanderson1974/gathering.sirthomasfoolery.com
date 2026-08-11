<template>
  <ExpandableSection
    v-if="event.daysOnly || numResponses >= 1"
    label="Options"
    :model-value="showEventOptions"
    @update:model-value="$emit('toggleShowEventOptions')"
  >
    <div class="tw-flex tw-flex-col tw-gap-4 tw-pt-2">
      <v-switch
        v-if="numResponses > 1 && isPhone"
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
      <v-switch
        v-if="numResponses >= 1"
        inset
        id="hide-if-needed-toggle"
        :model-value="hideIfNeeded"
        @update:model-value="(val) => $emit('update:hideIfNeeded', !!val)"
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
        :model-value="showResponseCounts"
        @update:model-value="(val) => $emit('update:showResponseCounts', !!val)"
        hide-details
      >
        <template v-slot:label>
          <div class="tw-text-sm tw-text-parchment">Show response counts</div>
        </template>
      </v-switch>

      <!-- Your own calendar, drawn behind the grid.
           Only offered when there is actually something to draw: a member with
           no linked calendar would otherwise get a switch that does nothing.
           Until now this was worse than absent — the state existed but nothing
           was bound to it, so the events could only ever be seen while editing
           your availability. -->
      <v-switch
        v-if="hasCalendarEvents"
        inset
        id="show-calendar-events-toggle"
        :model-value="showCalendarEvents"
        @update:model-value="(val) => $emit('update:showCalendarEvents', !!val)"
        hide-details
      >
        <template v-slot:label>
          <div class="tw-text-sm tw-text-parchment">
            Show my calendar events
          </div>
        </template>
      </v-switch>

      <!-- Start on monday -->
      <v-switch
        v-if="event.daysOnly"
        inset
        id="start-calendar-on-monday-toggle"
        :model-value="startCalendarOnMonday"
        @update:model-value="
          (val) => $emit('update:startCalendarOnMonday', !!val)
        "
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
    showCalendarEvents: { type: Boolean, default: false },
    /** Whether this member has any calendar events in the visible range. */
    hasCalendarEvents: { type: Boolean, default: false },
  },

  computed: {
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },
}
</script>
