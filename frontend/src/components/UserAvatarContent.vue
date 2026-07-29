<template>
  <v-avatar v-if="user" :size="size">
    <img v-if="src" :src="src" referrerpolicy="no-referrer" />
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
      :class="`tw-flex tw-size-full tw-items-center tw-justify-center tw-border tw-border-brass-dim tw-bg-wood tw-font-medium tw-text-${textSize} tw-text-brass`"
    >
      {{ initials }}
    </div>
  </v-avatar>
</template>

<script>
import { calendarTypes } from "@/constants"
import { avatarUrl, monogram } from "@/utils"

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
    /**
     * An uploaded photo when there is one, otherwise the Google picture — see
     * avatarUrl. Empty means neither, and the monogram shows instead.
     */
    src() {
      return avatarUrl(this.user)
    },
    textSize() {
      return this.size <= 24 ? "xs" : "lg"
    },
    /**
     * Two initials at a readable size, one when the avatar is too small to fit
     * them — CalendarAccount renders at 24 and SignUpBlock at 16, where "AL"
     * is a smudge.
     */
    initials() {
      const full = monogram(this.user)
      return this.size < 32 ? full.charAt(0) : full
    },
  },
}
</script>
