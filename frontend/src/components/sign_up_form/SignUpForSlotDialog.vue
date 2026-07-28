<template>
  <v-dialog
    :value="value"
    @input="(e) => $emit('input', e)"
    width="400"
    content-class="tw-m-0"
  >
    <v-card>
      <v-card-title class="tw-flex">
        <div>Join slot</div>
        <v-spacer />
        <v-btn icon @click="$emit('input', false)">
          <v-icon>mdi-close</v-icon>
        </v-btn>
      </v-card-title>
      <v-card-text>
        <div class="mb-2">
          <SignUpBlock :signUpBlock="signUpBlock" infoOnly></SignUpBlock>
        </div>

        <div class="tw-flex tw-flex-col tw-gap-y-4">
          <div>
            NOTE: After joining a slot,
            <span class="tw-font-bold"
              >you will need to contact the sign up creator in order to edit
              your slot.</span
            >
          </div>

          <div v-if="event.blindAvailabilityEnabled">
            The sign up creator has hidden attendees from each other.
            <span class="tw-font-bold"
              >Your name will only be visible to you.</span
            >
          </div>

          <div class="tw-flex">
            <v-spacer />
            <v-btn @click="submit" class="tw-bg-brass" dark> Join slot </v-btn>
          </div>
        </div>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script>
import { isPhone } from "@/utils"
import { mapState } from "vuex"

import SignUpBlock from "./SignUpBlock.vue"

export default {
  name: "SignUpForSlotDialog",

  emits: ["input", "submit"],

  components: { SignUpBlock },

  props: {
    value: { type: Boolean, required: true },
    event: { type: Object, required: true },
    signUpBlock: { type: Object, required: true },
  },

  data() {
    return {}
  },

  computed: {
    ...mapState(["authUser"]),
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },

  methods: {
    submit() {
      // The signer-up is identified by their session; the name/email fields
      // this dialog used to collect were for anonymous visitors, who no longer
      // exist. Nothing left to validate.
      this.$emit("submit")
    },
  },
}
</script>
