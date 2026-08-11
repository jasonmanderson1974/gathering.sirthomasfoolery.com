<template>
  <div>
    <div class="tw-mb-1 tw-text-sm tw-text-parchment">Buffer time</div>
    <div class="tw-mb-2 tw-text-xs tw-text-parchment-dim">
      Add time around calendar events
    </div>
    <v-switch
      id="buffer-time-switch"
      :input-value="bufferTime.enabled"
      @change="(val) => updateCalendarOption('enabled', val)"
      inset
      class="tw-flex tw-items-center"
      hide-details
    >
      <template v-slot:label>
        <div
          class="tw-flex tw-items-center tw-justify-center tw-gap-2 tw-text-sm tw-text-parchment"
        >
          <v-select
            :menu-props="{ auto: true }"
            dense
            hide-details
            :items="bufferTimes"
            class="-tw-mt-0.5 tw-w-20 tw-text-xs"
            :model-value="bufferTime.time"
            @input="(val) => updateCalendarOption('time', val)"
            @click="
              (e) => {
                e.preventDefault()
                e.stopPropagation()
              }
            "
          ></v-select>
        </div>
      </template>
    </v-switch>
  </div>
</template>

<script>
import { calendarOptionSync } from "@/mixins/calendarOptionSync"

export default {
  name: "BufferTimeToggle",

  // Declares the `bufferTime` + `syncWithBackend` props and
  // updateCalendarOption — see the mixin.
  mixins: [calendarOptionSync("bufferTime", "buffer time")],

  data() {
    return {
      bufferTimes: [
        { text: "15 min", value: 15 },
        { text: "30 min", value: 30 },
        { text: "45 min", value: 45 },
        { text: "1 hour", value: 60 },
      ],
    }
  },
}
</script>
