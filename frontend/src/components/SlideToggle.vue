<template>
  <div
    class="tw-relative tw-flex tw-w-fit tw-items-center tw-rounded-md tw-border tw-border-brass-dim"
  >
    <div
      class="tw-absolute tw-h-full tw-rounded-md tw-border tw-transition-all"
      :class="options[index].borderClass ?? defaultBorderClass"
      :style="{
        ...(options[index].borderStyle ?? defaultBorderStyle),
        transform: `translateX(${index * 100}%)`,
        width: `${100 / options.length}%`,
      }"
    ></div>
    <!-- The key belongs on the <template> in Vue 3, not on the element inside
         it: the template tag is what the v-for iterates. -->
    <template v-for="(tab, i) in options" :key="i">
      <div
        class="tw-flex tw-flex-1 tw-cursor-pointer tw-items-center tw-justify-center tw-gap-1.5 tw-self-stretch tw-overflow-hidden tw-px-4 tw-py-2.5 tw-text-center tw-text-sm tw-font-medium tw-transition-all"
        :class="
          i === index ? tab.activeClass ?? defaultActiveClass : inactiveClass
        "
        :style="tab.style || {}"
        @click="$emit('update:modelValue', tab.value)"
      >
        <slot :name="'option-' + tab.value" :option="tab" :active="i === index">
          <span :class="wrap ? 'tw-leading-tight' : 'tw-line-clamp-1'">{{
            tab.text
          }}</span>
        </slot>
      </div>
    </template>
  </div>
</template>

<script>
export default {
  name: "AvailabilityTypeToggle",

  props: {
    modelValue: { required: true },

    // Array of objects of the following structure:
    // {
    //   text: String,
    //   activeClass?: String,
    //   borderClass?: String,
    //   borderStyle?: Object,
    //   value: String,
    // }
    options: { type: Array, required: true },

    // Allow option labels to wrap onto multiple lines instead of being clamped
    // to one. Useful when there are enough options that labels get cramped.
    wrap: { type: Boolean, default: false },
  },

  data() {
    return {
      index: 0,

      defaultActiveClass: "tw-text-brass tw-bg-brass/10",
      defaultBorderClass: "tw-border-brass",
      defaultBorderStyle: { boxShadow: "0px 2px 8px 0px #c9a44c40" },
      inactiveClass: "tw-text-parchment-dim tw-bg-leather",
    }
  },

  watch: {
    modelValue: {
      immediate: true,
      handler() {
        this.index = this.options.findIndex(
          (tab) => tab.value === this.modelValue
        )
        if (this.index === -1) this.index = 0
      },
    },
  },
}
</script>
