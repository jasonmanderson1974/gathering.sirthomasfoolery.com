<template>
  <v-snackbar v-model="show" location="top" :color="color">
    <span class="tw-mr-2 tw-text-sm">{{ text }}</span>

    <!-- Vuetify 3 renamed this slot from `action` to `actions` and no longer
         passes `attrs` down to it. -->
    <template v-slot:actions>
      <v-btn icon @click="show = false">
        <v-icon>mdi-close</v-icon>
      </v-btn>
    </template>
  </v-snackbar>
</template>

<script>
export default {
  name: "AutoSnackbar",

  props: {
    text: { type: String, default: "" },
    color: { type: String, default: "" },
  },

  data() {
    return {
      show: false,
    }
  },

  created() {},

  watch: {
    text: {
      immediate: true,
      handler(text) {
        if (text) {
          this.show = true
        } else this.show = false
      },
    },
  },
}
</script>
