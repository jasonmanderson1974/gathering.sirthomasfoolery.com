<template>
  <v-chip
    :key="user.email"
    size="x-small"
    class="tw-flex tw-w-fit tw-items-center tw-bg-leather tw-text-parchment-dim"
  >
    <span class="tw-mr-2 tw-flex-none">
      <UserAvatarContent :user="user" :size="20" />
    </span>
    {{ user.email }}

    <v-icon
      v-if="removable"
      small
      @click="() => removeEmail(user.email)"
      class="tw-ml-1"
      >mdi-close</v-icon
    >
  </v-chip>
</template>

<script>
import UserAvatarContent from "@/components/UserAvatarContent.vue"

export default {
  name: "UserChip",

  props: {
    user: { type: Object, required: true },
    removable: { type: Boolean, default: false },
    removeEmail: { type: Function, default: () => {} },
  },

  components: { UserAvatarContent },

  data: () => ({}),

  created() {},

  computed: {},

  methods: {},
}
</script>
