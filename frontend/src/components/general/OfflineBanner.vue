<template>
  <!-- v-if, not v-show: `tailwind.config.js` sets `important: true`, so the
       `tw-flex` below compiles to `display: flex !important` and would beat the
       inline `display: none` v-show sets — the banner would simply never go
       away, with no error anywhere. -->
  <div
    v-if="offline"
    class="tw-pointer-events-none tw-fixed tw-left-1/2 tw-z-50 tw--translate-x-1/2"
    :class="headerOffset"
    role="status"
    aria-live="polite"
  >
    <div
      class="tw-flex tw-items-center tw-gap-2 tw-rounded-full tw-border tw-border-brass-dim tw-bg-wood-deep tw-px-4 tw-py-1.5 tw-shadow-lg"
    >
      <v-icon size="16" class="tw-text-brass">mdi-cloud-off-outline</v-icon>
      <span class="tw-text-xs tw-text-parchment">
        Offline — showing what was saved on this device
      </span>
    </div>
  </div>
</template>

<script>
/**
 * Says why the page might be out of date.
 *
 * Deliberately non-interactive and out of the layout flow: it appears and
 * disappears with the signal, and a bar that reflowed the page each time would
 * be worse than the problem. `pointer-events-none` so it can never swallow a
 * tap meant for what is underneath it.
 */
export default {
  name: "OfflineBanner",

  props: {
    offline: { type: Boolean, default: false },
    // Whether the app header is showing, so the pill clears it. Two literal
    // class strings rather than a computed one: Tailwind purges on literal
    // source text, so `tw-top-${n}` would emit no CSS at all.
    belowHeader: { type: Boolean, default: true },
  },

  computed: {
    headerOffset() {
      return this.belowHeader ? "tw-top-16 sm:tw-top-20" : "tw-top-3"
    },
  },
}
</script>
