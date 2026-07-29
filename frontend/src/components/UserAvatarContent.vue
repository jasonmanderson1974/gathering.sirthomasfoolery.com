<template>
  <v-avatar v-if="user" :size="size">
    <img v-if="user.picture" :src="user.picture" referrerpolicy="no-referrer" />
    <v-icon
      class="-tw-mt-1"
      :size="size"
      v-else-if="user.calendarType === calendarTypes.APPLE"
    >
      mdi-apple
    </v-icon>
    <v-icon
      :size="size"
      v-else-if="user.calendarType === calendarTypes.OUTLOOK"
    >
      mdi-microsoft-outlook
    </v-icon>
    <div
      v-else
      :class="`tw-flex tw-size-full tw-items-center tw-justify-center tw-bg-[linear-gradient(-25deg,#2b6cb0,#63b3ed,#2b6cb0)] tw-text-${textSize} tw-text-white`"
    >
      {{ monogram }}
    </div>
  </v-avatar>
</template>

<script>
import { calendarTypes } from "@/constants"
import { displayName } from "@/utils"

export default {
  name: "UserAvatarContent",
  props: {
    user: Object,
    size: { type: Number, default: 48 },
  },

  computed: {
    calendarTypes() {
      return calendarTypes
    },
    textSize() {
      return this.size <= 24 ? "xs" : "lg"
    },
    /**
     * First character of the name as shown, so a nicknamed user's monogram
     * matches the name beside it. Falls back to the email for accounts with no
     * name at all.
     */
    monogram() {
      return (
        displayName(this.user).charAt(0) || this.user.email?.charAt(0) || ""
      )
    },
  },
}
</script>
