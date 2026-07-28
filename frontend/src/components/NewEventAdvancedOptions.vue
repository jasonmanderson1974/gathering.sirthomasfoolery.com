<template>
<div class="tw-flex tw-flex-col tw-gap-5 tw-pt-2">
  <div v-if="!edit" class="tw-flex tw-items-center tw-gap-x-2">
    <div class="tw-text-sm tw-text-parchment">Time increment:</div>
    <v-select
      v-model="timeIncrementProxy"
      dense
      class="-tw-mt-[2px] tw-w-24 tw-grow-0 tw-text-sm"
      menu-props="auto"
      hide-details
      :items="timeIncrementItems"
    ></v-select>
  </div>
  <v-checkbox
    v-if="authUser && !guestEvent"
    v-model="collectEmailsProxy"
    hide-details
  >
    <template v-slot:label>
      <span class="tw-text-sm tw-text-parchment">
        Collect respondents' email addresses
      </span>
    </template>
    <template v-slot:message="{ message }">
      <div
        class="-tw-mt-1 tw-ml-[32px] tw-text-xs tw-text-parchment-dim"
      >
        {{ message }}
      </div>
    </template>
  </v-checkbox>
  <v-checkbox
    v-else-if="!guestEvent"
    disabled
    messages="test"
    off-icon="mdi-checkbox-blank-off-outline"
  >
    <template v-slot:label>
      <span class="tw-text-sm"
        >Collect respondents' email addresses</span
      >
    </template>
    <template v-slot:message>
      <div
        class="tw-pointer-events-auto -tw-mt-1 tw-ml-[32px] tw-text-xs tw-text-parchment-dim"
      >
        <span class="tw-font-medium tw-text-parchment-dim"
          ><a @click="$emit('signIn')">Sign in</a>
          to use this feature
        </span>
      </div>
    </template>
  </v-checkbox>
  <v-checkbox
    v-if="authUser && !guestEvent"
    v-model="blindAvailabilityEnabledProxy"
    messages="Only show responses to event creator"
  >
    <template v-slot:label>
      <span class="tw-text-sm tw-text-parchment">
        Hide responses from respondents
      </span>
    </template>
    <template v-slot:message="{ message }">
      <div
        class="-tw-mt-1 tw-ml-[32px] tw-text-xs tw-text-parchment-dim"
      >
        {{ message }}
      </div>
    </template>
  </v-checkbox>
  <v-checkbox
    v-else-if="!guestEvent"
    disabled
    messages="Only show responses to event creator. "
    off-icon="mdi-checkbox-blank-off-outline"
  >
    <template v-slot:label>
      <span class="tw-text-sm"
        >Hide responses from respondents</span
      >
    </template>
    <template v-slot:message="{ message }">
      <div
        class="tw-pointer-events-auto -tw-mt-1 tw-ml-[32px] tw-text-xs tw-text-parchment-dim"
      >
        {{ message }}
        <span class="tw-font-medium tw-text-parchment-dim"
          ><a @click="$emit('signIn')">Sign in</a>
          to use this feature
        </span>
      </div>
    </template>
  </v-checkbox>
  <v-checkbox
    v-if="authUser && !guestEvent"
    v-model="sendEmailAfterXResponsesEnabledProxy"
    hide-details
  >
    <template v-slot:label>
      <div
        :class="!sendEmailAfterXResponsesEnabled && 'tw-opacity-50'"
        class="tw-flex tw-items-center tw-gap-x-2 tw-text-sm tw-text-parchment-dim"
      >
        <div>Email me after</div>
        <v-text-field
          v-model="sendEmailAfterXResponsesProxy"
          @click="
            (e) => {
              e.preventDefault()
              e.stopPropagation()
            }
          "
          :disabled="!sendEmailAfterXResponsesEnabled"
          dense
          class="email-me-after-text-field -tw-mt-[2px] tw-w-10"
          menu-props="auto"
          hide-details
          type="number"
          min="1"
        ></v-text-field>
        <div>responses</div>
      </div>
    </template>
  </v-checkbox>
  <TimezoneSelector
    v-model="timezoneProxy"
    label="Timezone"
    @input="$emit('timezone-input', $event)"
  />
</div>
</template>

<script>
import { mapState } from "vuex"
import TimezoneSelector from "@/components/schedule_overlap/TimezoneSelector.vue"

/**
 * "Advanced options" panel content for the new/edit event form: time
 * increment, collect-emails, hide-responses, email-after-X-responses, and
 * timezone. Extracted from NewEvent.vue (TODO A11, Tier 2). All field state
 * stays in NewEvent (submit/reset/hasEventBeenEdited read it); each field is
 * two-way bound via `.sync` (update:* emits through the *Proxy computeds).
 */
export default {
  name: "NewEventAdvancedOptions",

  props: {
    edit: { type: Boolean, default: false },
    guestEvent: { type: Boolean, default: false },
    timeIncrement: { type: Number, default: 15 },
    collectEmails: { type: Boolean, default: false },
    blindAvailabilityEnabled: { type: Boolean, default: false },
    sendEmailAfterXResponsesEnabled: { type: Boolean, default: false },
    sendEmailAfterXResponses: { default: 3 },
    timezone: { type: Object, default: () => ({}) },
  },

  emits: [
    "update:timeIncrement",
    "update:collectEmails",
    "update:blindAvailabilityEnabled",
    "update:sendEmailAfterXResponsesEnabled",
    "update:sendEmailAfterXResponses",
    "update:timezone",
    "signIn",
    "timezone-input",
  ],

  components: {
    TimezoneSelector,
  },

  computed: {
    ...mapState(["authUser"]),
    timeIncrementItems() {
      return [
        { text: "15 min", value: 15 },
        { text: "30 min", value: 30 },
        { text: "60 min", value: 60 },
      ]
    },
    timeIncrementProxy: {
      get() {
        return this.timeIncrement
      },
      set(v) {
        this.$emit("update:timeIncrement", v)
      },
    },
    collectEmailsProxy: {
      get() {
        return this.collectEmails
      },
      set(v) {
        this.$emit("update:collectEmails", v)
      },
    },
    blindAvailabilityEnabledProxy: {
      get() {
        return this.blindAvailabilityEnabled
      },
      set(v) {
        this.$emit("update:blindAvailabilityEnabled", v)
      },
    },
    sendEmailAfterXResponsesEnabledProxy: {
      get() {
        return this.sendEmailAfterXResponsesEnabled
      },
      set(v) {
        this.$emit("update:sendEmailAfterXResponsesEnabled", v)
      },
    },
    sendEmailAfterXResponsesProxy: {
      get() {
        return this.sendEmailAfterXResponses
      },
      set(v) {
        this.$emit("update:sendEmailAfterXResponses", v)
      },
    },
    timezoneProxy: {
      get() {
        return this.timezone
      },
      set(v) {
        this.$emit("update:timezone", v)
      },
    },
  },
}
</script>
