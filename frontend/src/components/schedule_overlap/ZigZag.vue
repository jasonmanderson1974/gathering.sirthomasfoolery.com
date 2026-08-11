<!-- The torn-paper edge marking a break between non-consecutive days.
     The gradients mask the grid with the PAGE colour and draw a hairline along
     the tear, so both are theme colours: #1c1410 is wood-deep (what the grid
     sits on) and #8a7333 is brass-dim. They were `#1c1410` and `#8a7333`, a
     pre-Fellowship light-theme assumption that drew a bright #1c1410 sawtooth
     down the edge of the availability grid. -->
<template>
  <div ref="container" class="tw-overflow-hidden">
    <div :class="left ? 'line1-left' : 'line1-right'" :style="lineStyle"></div>
    <div :class="left ? 'line2-left' : 'line2-right'" :style="lineStyle"></div>
  </div>
</template>

<style scoped>
.line1-left {
  background: linear-gradient(
    45deg,
    #1c1410,
    #1c1410 49%,
    #8a7333 49%,
    transparent 51%
  );
}
.line2-left {
  background: linear-gradient(
    -45deg,
    transparent,
    transparent 49%,
    #8a7333 49%,
    #1c1410 51%
  );
}

.line1-right {
  background: linear-gradient(
    45deg,
    transparent,
    transparent 49%,
    #8a7333 51%,
    #1c1410 51%
  );
}
.line2-right {
  background: linear-gradient(
    -45deg,
    #1c1410,
    #1c1410 49%,
    #8a7333 51%,
    transparent 51%
  );
}
</style>

<script>
export default {
  name: "ZigZag",

  props: {
    left: { type: Boolean, default: false },
    right: { type: Boolean, default: false },
  },

  mounted() {
    // Background size is 2 * width of the element
    this.backgroundSize = this.$refs.container.offsetWidth * 2
  },

  data() {
    return {
      backgroundSize: 0,
    }
  },

  computed: {
    lineStyle() {
      return {
        position: "absolute",
        width: "200%",
        height: "100%",
        backgroundSize: `${this.backgroundSize}px ${this.backgroundSize}px`,
        transform: this.left
          ? `translate(${-this.backgroundSize / 2}px, 0)`
          : "",
      }
    },
  },
}
</script>
