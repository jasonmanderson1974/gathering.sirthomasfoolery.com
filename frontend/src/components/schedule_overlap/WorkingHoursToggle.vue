<template>
  <div>
    <div class="tw-mb-1 tw-text-sm tw-text-parchment">Working hours</div>
    <div class="tw-mb-2 tw-text-xs tw-text-parchment-dim">
      Only autofill availability between working hours
    </div>
    <v-switch
      id="working-hours-toggle"
      inset
      :input-value="workingHours.enabled"
      @change="(val) => updateCalendarOption('enabled', val)"
      hide-details
    >
      <template v-slot:label>
        <div class="tw-text-sm tw-text-parchment">
          <div class="tw-flex tw-items-center tw-gap-2">
            <v-select
              :menu-props="{ auto: true }"
              density="compact"
              hide-details
              return-object
              class="-tw-mt-0.5 tw-w-20 tw-text-xs"
              :items="times"
              item-title="text"
              :model-value="workingHours.startTime"
              @update:model-value="
                (val) => updateCalendarOption('startTime', val.time)
              "
              @click="
                (e) => {
                  e.preventDefault()
                  e.stopPropagation()
                }
              "
            />
            <div>to</div>
            <v-select
              :menu-props="{ auto: true }"
              density="compact"
              hide-details
              return-object
              class="-tw-mt-0.5 tw-w-20 tw-text-xs"
              :items="times"
              item-title="text"
              :model-value="workingHours.endTime"
              @update:model-value="
                (val) => updateCalendarOption('endTime', val.time)
              "
              @click="
                (e) => {
                  e.preventDefault()
                  e.stopPropagation()
                }
              "
            />
          </div>
        </div>
      </template>
    </v-switch>
  </div>
</template>

<script>
import { getTimeOptions } from "@/utils"
import { calendarOptionSync } from "@/mixins/calendarOptionSync"

export default {
  name: "WorkingHoursToggle",

  // Declares the `workingHours` + `syncWithBackend` props and
  // updateCalendarOption — see the mixin.
  mixins: [calendarOptionSync("workingHours", "working hours")],

  computed: {
    times() {
      return getTimeOptions()
    },
  },
}
</script>
