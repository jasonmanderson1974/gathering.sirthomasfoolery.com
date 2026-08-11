<template>
  <div
    class="tw-flex tw-h-full tw-flex-col tw-items-center tw-justify-center tw-p-2"
  >
    <SirThomasFoolery :size="160" class="tw-mb-6" />
    <div class="tw-mb-8 tw-max-w-[19rem]">
      <h1 class="tw-mb-2 tw-text-center tw-text-2xl tw-font-medium">
        404 - Page not found
      </h1>
      <div class="tw-text-center tw-text-parchment-dim">
        The page you’re looking for doesn’t exist or an error occurred.
      </div>
    </div>
    <v-btn href="/home" color="primary">Back to home</v-btn>
  </div>
</template>

<script>
import { isPhone } from "@/utils"
import SirThomasFoolery from "@/components/general/SirThomasFoolery.vue"

export default {
  name: "PageNotFound",

  head: {
    title: "Page not found · The Fellowship",
  },

  components: { SirThomasFoolery },

  computed: {
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },
}
</script>
