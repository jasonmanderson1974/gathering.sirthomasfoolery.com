<template>
  <div
    class="flw-panel tw-flex tw-w-full tw-cursor-pointer tw-flex-col tw-p-4 tw-text-left tw-transition-all sm:tw-p-6"
    :class="{ 'tw-border-brass-bright': toggled }"
    @click="() => (toggled = !toggled)"
  >
    <div
      class="tw-flex tw-flex-row tw-content-center tw-justify-between tw-text-base"
    >
      <div
        class="tw-mr-4 tw-font-head tw-text-lg tw-text-parchment"
        v-html="question"
      ></div>
      <v-icon
        size="x-large"
        :class="`${
          toggled
            ? 'tw-rotate-45 tw-text-brass-bright'
            : 'tw-rotate-0 tw-text-brass-dim'
        }`"
        >mdi-plus</v-icon
      >
    </div>

    <v-expand-transition>
      <div v-if="toggled">
        <div class="tw-pt-4 tw-font-body tw-text-base tw-text-parchment-dim sm:tw-pt-6">
          <div v-html="answer"></div>
          <div
            v-if="authRequired"
            class="tw-mt-6 tw-text-sm tw-font-medium tw-text-parchment-dim"
          >
            *
            <a @click.stop="$emit('signIn')" class="tw-text-brass tw-underline"
              >Enter</a
            >
            to use this feature
          </div>
        </div>
      </div>
    </v-expand-transition>
  </div>
</template>

<script>
export default {
  name: "FAQ",

  props: {
    question: { type: String, required: true },
    answer: { type: String },
    authRequired: { type: Boolean, default: false },
  },

  data: () => ({
    toggled: false,
  }),

  computed: {},

  methods: {},
}
</script>
