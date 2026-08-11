<template>
  <div
    v-if="showGradient"
    class="tw-pointer-events-none tw-absolute tw-bottom-0 tw-left-0 tw-right-0 tw-z-20 tw-flex tw-h-16 tw-items-end tw-justify-center"
    :style="{ background: gradient }"
  >
    <v-btn
      v-if="showArrow"
      icon
      size="x-small"
      class="tw-pointer-events-auto tw-transform"
      @click="scrollToBottom"
    >
      <v-icon>mdi-chevron-down</v-icon>
    </v-btn>
  </div>
</template>

<script>
export default {
  name: "OverflowGradient",
  props: {
    /**
     * The scrolling element — or a component wrapping one.
     *
     * Accepts both deliberately. A `ref` on a plain element gives the element,
     * but a `ref` on a *component* gives that component's instance, and which
     * of the two you get changed under us: Vuetify 2's `VCardText` was a
     * functional component (no instance, so the ref was the DOM node) and
     * Vuetify 3's is a real one. `NewEvent` puts its ref on a `<v-card-text>`,
     * so it silently started handing over a component proxy — on which
     * `addEventListener` is undefined, and the whole panel threw on mount.
     * `el()` normalises the two, so neither call site has to know.
     */
    scrollContainer: {
      type: [HTMLElement, Object],
      required: true,
    },
    showArrow: {
      type: Boolean,
      default: true,
    },
    /**
     * The colour the fade resolves to — i.e. whatever this sits on top of.
     *
     * Was hardcoded to white, which is a pre-Fellowship light-theme leftover:
     * on the dark theme it drew a white veil over the bottom of whatever it was
     * covering, which is what made the responses list look washed out. Defaults
     * to the page's wood; pass the surface colour when it sits on something
     * else.
     */
    fadeTo: {
      type: String,
      default: "#1c1410", // wood-deep, the page background
    },
  },

  computed: {
    gradient() {
      // The transparent stop must be the SAME colour, not `transparent`:
      // `transparent` is rgba(0,0,0,0), so interpolating from it darkens the
      // midpoint into a grey smear on any non-black background.
      return `linear-gradient(to bottom, ${this.fadeTo}00 0%, ${this.fadeTo} 100%)`
    },
  },
  data() {
    return {
      showGradient: false,
    }
  },
  mounted() {
    this.checkScroll()
    this.el()?.addEventListener("scroll", this.checkScroll)
  },
  beforeUnmount() {
    this.el()?.removeEventListener("scroll", this.checkScroll)
  },
  methods: {
    /** The scrolling DOM element, whichever shape the prop arrived in. */
    el() {
      const c = this.scrollContainer
      if (!c) return null
      const resolved = c instanceof HTMLElement ? c : c.$el
      return resolved instanceof HTMLElement ? resolved : null
    },
    /**
     * Checks if the scroll bar is scrolled to the bottom of the client
     */
    checkScroll() {
      const el = this.el()
      if (!el) {
        this.showGradient = false
        return
      }
      const { scrollHeight, clientHeight, scrollTop } = el
      this.showGradient =
        scrollHeight > clientHeight &&
        scrollTop < scrollHeight - clientHeight - 1 // 1px tolerance
    },
    scrollToBottom() {
      const el = this.el()
      if (!el) return
      // A raw HTMLElement, not Vue state — setting scrollTop is imperative DOM
      // API, not a mutation of parent data.
      el.scrollTop = el.scrollHeight
    },
  },
}
</script>
