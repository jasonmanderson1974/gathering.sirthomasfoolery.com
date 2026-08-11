<template>
  <div
    id="gathering-summary"
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-flex tw-items-start tw-gap-2">
      <v-icon size="small" class="tw-mt-0.5 tw-text-brass"
        >mdi-calendar-check</v-icon
      >
      <div class="tw-min-w-0 tw-flex-1">
        <div class="tw-text-xs tw-uppercase tw-tracking-wide tw-text-brass">
          Gathering set
        </div>
        <div class="tw-mt-0.5 tw-text-base tw-font-medium">
          {{ scheduledText }}
        </div>
        <div class="tw-mt-1 tw-text-xs tw-text-parchment-dim">
          {{ reminderText }}
        </div>
      </div>

      <!-- Changing or calling off the gathering is the organiser's business,
           so the menu only exists for them. Everyone else still sees the card. -->
      <v-menu v-if="canEdit" location="bottom end">
        <template v-slot:activator="{ props }">
          <v-btn
            icon
            size="small"
            v-bind="props"
            aria-label="Gathering options"
          >
            <v-icon size="small">mdi-dots-vertical</v-icon>
          </v-btn>
        </template>
        <v-list density="compact">
          <v-list-item @click="$emit('reschedule')">
            <v-list-item-title>Reschedule</v-list-item-title>
          </v-list-item>
          <v-list-item @click="$emit('cancel-gathering')">
            <v-list-item-title class="tw-text-red">
              Cancel gathering
            </v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>
    </div>

    <v-btn
      :href="icsHref"
      size="small"
      variant="outlined"
      class="tw-mt-3 tw-w-full tw-text-brass"
    >
      <v-icon size="small" start>mdi-calendar-plus</v-icon>
      Add to calendar
    </v-btn>
  </div>
</template>

<script>
import { getStartEndDateString } from "@/utils/date_utils"
import { reminderSummaryText, icsUrl } from "./gatheringSummary"

/**
 * The confirmed gathering at a glance: when it is, whether it repeats or sends
 * a reminder, an .ics download, and (for whoever can manage the event) the way
 * to reschedule or call it off.
 *
 * Sits at the top of the event page's right-hand sidebar, which only exists
 * once a time is locked in and the availability grid has collapsed. Before
 * this card the confirmed time was only ever visible inside ToolRow's
 * "Gathering set" dropdown, which ordinary members never see.
 *
 * Purely presentational — it emits, Event.vue delegates to ScheduleOverlap.
 */
export default {
  name: "GatheringSummary",

  props: {
    event: { type: Object, required: true },
    canEdit: { type: Boolean, default: false },
  },

  emits: ["reschedule", "cancel-gathering"],

  computed: {
    /**
     * "Sat, Mar 14, 7:00 PM - 10:00 PM PST". Deliberately the full range and
     * not ToolRow's start-only text — this is the one place a member reads to
     * find out how long they're committing to.
     */
    scheduledText() {
      const scheduled = this.event.scheduledEvent
      if (!scheduled?.startDate || !scheduled?.endDate) return ""
      return getStartEndDateString(
        new Date(scheduled.startDate),
        new Date(scheduled.endDate)
      )
    },
    reminderText() {
      return reminderSummaryText(this.event)
    },
    icsHref() {
      return icsUrl(this.event)
    },
  },
}
</script>
