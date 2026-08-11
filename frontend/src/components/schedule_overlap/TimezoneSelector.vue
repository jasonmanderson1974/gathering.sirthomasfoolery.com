<!-- Allows user to change timezone -->
<template>
  <div
    class="tw-flex tw-items-center tw-justify-center"
    id="timezone-select-container"
  >
    <div :class="`tw-mr-2 tw-mt-px ${labelColor}`">{{ label }}</div>
    <v-select
      id="timezone-select"
      :model-value="modelValue"
      @update:model-value="onChange"
      :items="timezones"
      :menu-props="{ auto: true }"
      class="tw-z-20 -tw-mt-px tw-w-52 tw-text-sm"
      density="compact"
      color="#219653"
      item-color="green"
      hide-details
      item-title="label"
      item-value="value"
      return-object
    >
      <!-- Vuetify 3's item slot hands over a single `props` bundle instead of
           `on`/`attrs`, and wraps the underlying object: the timezone itself is
           `item.raw`, not `item`. -->
      <template v-slot:item="{ props, item }">
        <v-list-item v-bind="props" :title="null">
          <v-list-item-title>
            {{ item.raw.gmtString }} {{ item.raw.label }}
          </v-list-item-title>
        </v-list-item>
      </template>
      <!-- Rendered from `modelValue`, not from the slot's `item`. The
           `timezones` computed rebuilds its objects on every evaluation, so the
           selected object is never reference-equal to the one in `items`;
           `item-value="value"` above is what lets Vuetify match them, and this
           reads the source of truth directly rather than depending on that
           match. Vuetify 3 already wraps the slot in `.v-select__selection`, so
           there is no wrapper div here — the old one nested a second copy. -->
      <template v-slot:selection>
        {{ modelValue.gmtString }} {{ modelValue.label }}
      </template>
    </v-select>
    <v-btn v-if="timezoneModified" @click="resetTimezone" icon color="primary"
      ><v-icon>mdi-refresh</v-icon></v-btn
    >
  </div>
</template>

<script>
import { allTimezones } from "@/constants"
import dayjs from "dayjs"
import utcPlugin from "dayjs/plugin/utc"
import timezonePlugin from "dayjs/plugin/timezone"
dayjs.extend(utcPlugin)
dayjs.extend(timezonePlugin)

export default {
  name: "TimezoneSelector",

  props: {
    modelValue: { type: Object, required: true },
    label: { type: String, default: "Shown in" },
    labelColor: { type: String, default: "" },
    referenceDate: { type: Date, default: null },
  },

  emits: ["update:modelValue"],

  created() {
    if (localStorage["timezone"]) {
      this.timezoneModified = true
    }

    if (this.modelValue.value) return // Timezone has already been set

    // Set timezone to localstorage timezone if localstorage is set
    if (localStorage["timezone"]) {
      this.$emit("update:modelValue", JSON.parse(localStorage["timezone"]))
      return
    }

    // Otherwise, set timezone to local timezone
    this.$emit("update:modelValue", this.getLocalTimezone())
  },

  data() {
    return {
      timezoneModified: false, // Whether the timezone has been modified from the local timezone
    }
  },

  computed: {
    effectiveReferenceDate() {
      return this.referenceDate ?? new Date()
    },
    /** Returns an array of all supported timezones */
    timezones() {
      // ===============================================================================
      // Source: https://github.com/ndom91/react-timezone-select/blob/main/src/index.tsx
      // ===============================================================================

      const t = Object.entries(allTimezones)
        .map((zone) => {
          try {
            const min = dayjs(this.effectiveReferenceDate)
              .tz(zone[0])
              .utcOffset()
            const hr = `${(min / 60) ^ 0}:${
              min % 60 === 0 ? "00" : Math.abs(min % 60)
            }`
            const gmtString = `(GMT${hr.includes("-") ? hr : `+${hr}`})`
            const label = `${zone[1]}`

            return {
              value: zone[0],
              label: label,
              gmtString: gmtString,
              offset: min,
            }
          } catch (e) {
            console.error(e)
            return null
          }
        })
        .filter(Boolean)
        .sort((a, b) => a.offset - b.offset)
      return t
    },
  },

  methods: {
    /** Updates local storage and emits the new timezone */
    onChange(val) {
      localStorage["timezone"] = JSON.stringify(val)
      this.$emit("update:modelValue", val)
      this.timezoneModified = true
    },
    /** Returns a timezone object for the local timezone */
    getLocalTimezone() {
      const localTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone

      // Step 1: Exact match on IANA timezone name
      let timezoneObject = this.timezones.find((t) => t.value === localTimezone)

      if (!timezoneObject) {
        // Step 2: Match by offsets at two reference dates (Jan + Jul)
        // Distinguishes DST-observing zones from non-DST zones that share
        // the same current offset (e.g. Europe/Belgrade vs Africa/Casablanca)
        const janOffset = dayjs
          .tz("2024-01-15 12:00", localTimezone)
          .utcOffset()
        const julOffset = dayjs
          .tz("2024-07-15 12:00", localTimezone)
          .utcOffset()

        timezoneObject = this.timezones.find((t) => {
          const tJan = dayjs.tz("2024-01-15 12:00", t.value).utcOffset()
          const tJul = dayjs.tz("2024-07-15 12:00", t.value).utcOffset()
          return tJan === janOffset && tJul === julOffset
        })
      }

      if (!timezoneObject) {
        // Step 3: Final fallback — current offset only
        const offset = dayjs(this.effectiveReferenceDate)
          .tz(localTimezone)
          .utcOffset()
        timezoneObject = this.timezones.find((t) => t.offset === offset)
      }

      return timezoneObject
    },
    /** Resets timezone to the local timezone and clears localstorage as well */
    resetTimezone() {
      this.$emit("update:modelValue", this.getLocalTimezone())
      localStorage.removeItem("timezone")
      this.timezoneModified = false
    },
  },

  watch: {
    referenceDate() {
      if (!this.modelValue?.value) {
        return
      }

      const refreshedTimezone = this.timezones.find(
        (timezone) => timezone.value === this.modelValue.value
      )

      if (
        !refreshedTimezone ||
        refreshedTimezone.offset === this.modelValue.offset
      ) {
        return
      }

      if (localStorage["timezone"]) {
        localStorage["timezone"] = JSON.stringify(refreshedTimezone)
      }

      this.$emit("update:modelValue", refreshedTimezone)
    },
  },
}
</script>
