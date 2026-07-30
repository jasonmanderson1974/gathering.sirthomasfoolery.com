<template>
  <v-avatar v-if="user" :size="size">
    <!--
      crossorigin is set only for our own avatar route, and only matters under
      `npm run serve`, where the API is a different origin and a plain <img>
      would send no session cookie to a route that now requires one. Binding
      null omits the attribute, which is what the Google `picture` fallback
      needs — see isOwnAvatarUrl.
    -->
    <img
      v-if="src"
      :src="src"
      :crossorigin="needsCredentials ? 'use-credentials' : null"
      referrerpolicy="no-referrer"
    />
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
    <!--
      `tw-rounded-full` is load-bearing, not decoration: v-avatar clips to a
      circle, so a square border on this div survives only where the square
      touches the circle — four tick marks at the compass points instead of a
      ring. Rounding the div makes its border follow the same curve.
    -->
    <div
      v-else
      :class="`tw-flex tw-size-full tw-items-center tw-justify-center tw-rounded-full tw-border tw-border-brass-dim tw-bg-wood tw-font-medium tw-text-${textSize} tw-text-brass`"
    >
      {{ initials }}
    </div>
  </v-avatar>
</template>

<script>
import { calendarTypes } from "@/constants"
import { avatarUrl, isOwnAvatarUrl, monogram } from "@/utils"

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
    needsCredentials() {
      return isOwnAvatarUrl(this.src)
    },
    textSize() {
      return this.size <= 24 ? "xs" : "lg"
    },
    /**
     * Two initials at a readable size, one when the avatar is too small to fit
     * them — CalendarAccount renders at 24, and below about 16 "AL" is a
     * smudge.
     */
    initials() {
      const full = monogram(this.user)
      return this.size < 32 ? full.charAt(0) : full
    },
  },
}
</script>
