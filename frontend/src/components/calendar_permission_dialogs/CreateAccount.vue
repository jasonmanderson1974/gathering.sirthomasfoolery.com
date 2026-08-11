<template>
  <div class="tw-flex tw-flex-col tw-gap-6">
    <div
      class="tw-text-md tw-flex tw-flex-col tw-items-center tw-gap-2 tw-font-normal"
    >
      Create a Fellowship account to link your
      <div class="tw-flex tw-items-center tw-gap-1">
        <v-img
          class="-tw-mt-1 tw-flex-initial"
          width="15"
          height="15"
          src="@/assets/apple_logo.svg"
        />
        <span class="tw-font-medium">Apple Calendar</span>
      </div>
    </div>
    <div v-if="!authUser">
      <SignInGoogleBtn
        @click="$emit('signInLinkApple')"
        class="tw-mb-3 tw-w-full"
      />
      <div class="tw-text-center tw-text-xs tw-text-parchment-dim">
        By continuing, you agree to our
        <router-link
          class="tw-text-brass"
          target="_blank"
          :to="{ name: 'privacy-policy' }"
          >privacy policy</router-link
        >
      </div>
    </div>
    <div v-else>
      <div
        class="tw-flex tw-items-center tw-justify-center tw-text-sm tw-text-parchment-dim"
      >
        <v-icon class="tw-mx-2 tw-text-brass" size="16"
          >mdi-check-circle</v-icon
        >
        Fellowship account created
      </div>
    </div>
    <div class="tw-flex tw-items-center tw-gap-2">
      <v-btn variant="text" class="tw-grow" @click="$emit('back')">Back</v-btn>
      <v-btn
        color="primary"
        class="tw-grow"
        :disabled="!authUser"
        @click="$emit('continue')"
        >Continue</v-btn
      >
    </div>
  </div>
</template>

<script>
import SignInGoogleBtn from "../SignInGoogleBtn.vue"
import { mapState } from "vuex"

export default {
  name: "CreateAccount",

  components: {
    SignInGoogleBtn,
  },

  emits: ["signInLinkApple", "back", "continue"],

  computed: {
    ...mapState(["authUser"]),
  },
}
</script>
