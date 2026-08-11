<!--class="tw-flex tw-items-end tw-justify-start tw-p-1"-->
<template>
  <div>
    <v-btn
      class="-tw-ml-2 tw-w-[calc(100%+1rem)] tw-justify-between tw-px-2"
      block
      variant="text"
      @click="toggle"
    >
      <span class="-tw-ml-px tw-mr-1" :class="labelClass">
        {{ label }}
      </span>
      <v-spacer />
      <v-icon
        :class="`tw-rotate-${modelValue ? '180' : '0'} ${iconClass}`"
        :size="30"
        >mdi-chevron-down</v-icon
      ></v-btn
    >
    <v-expand-transition>
      <div v-show="modelValue">
        <slot></slot>
      </div>
    </v-expand-transition>
    <div ref="scrollTo"></div>
  </div>
</template>

<script>
export default {
  name: "ExpandableSection",

  props: {
    modelValue: { type: Boolean, required: true },
    label: { type: String, default: "" },
    labelClass: { type: String, default: "tw-text-base" },
    iconClass: { type: String, default: "" },
    autoScroll: { type: Boolean, default: false },
  },

  emits: ["update:modelValue"],

  methods: {
    toggle() {
      this.$emit("update:modelValue", !this.modelValue)
    },
    scrollToElement(element) {
      if (this.autoScroll && element) {
        setTimeout(() => element.scrollIntoView({ behavior: "smooth" }), 200)
      }
    },
  },

  watch: {
    modelValue() {
      if (this.modelValue) {
        this.scrollToElement(this.$refs.scrollTo)
      }
    },
  },
}
</script>
