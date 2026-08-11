<template>
  <v-scale-transition appear origin="center">
    <!-- `tw-fixed` rather than Vuetify's `position="fixed"`: the Tailwind
         utility wins outright (`important: true`), and the bottom/left/right
         offsets next to it are only meaningful once this is out of flow.
         Vuetify 2's `fab` + `fixed` props did both jobs and neither exists in
         Vuetify 3 — they became inert DOM attributes, leaving a square button
         1,300px down the page (TODO3 L2). -->
    <v-btn
      icon
      size="large"
      class="tw-fixed tw-bottom-4 tw-left-0 tw-right-0 tw-z-30 tw-mx-auto tw-bg-brass tw-text-wood-deep sm:tw-bottom-8"
      @click="(e) => $emit('click', e)"
      v-bind="$attrs"
    >
      <slot />
    </v-btn>
  </v-scale-transition>
</template>

<script>
export default {
  name: "BottomFab",

  emits: ["click"],
}
</script>
