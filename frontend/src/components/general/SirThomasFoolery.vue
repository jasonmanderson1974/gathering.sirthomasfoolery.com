<template>
  <!-- Sir Thomas Foolery — the top-hatted, monocled hare crest of The Fellowship -->
  <img
    :src="crest"
    :width="size"
    :height="size"
    alt="Sir Thomas Foolery, the monocled hare"
    class="flw-crest"
  />
</template>

<script>
import crest from "@/assets/sir_thomas_foolery.png"

export default {
  name: "SirThomasFoolery",

  props: {
    size: { type: [Number, String], default: 116 },
  },

  data() {
    return { crest }
  },
}
</script>

<style scoped>
.flw-crest {
  display: block;
  filter: drop-shadow(0 6px 14px rgba(0, 0, 0, 0.6));
}
</style>
