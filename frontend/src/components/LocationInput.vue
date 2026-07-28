<template>
  <v-combobox
    :value="value"
    :items="items"
    :loading="loading"
    :label="label"
    :placeholder="placeholder"
    :solo="solo"
    :dense="dense"
    :hide-details="hideDetails"
    :autofocus="autofocus"
    :prepend-inner-icon="hideIcon ? undefined : 'mdi-map-marker'"
    :search-input.sync="search"
    :menu-props="{ contentClass: 'location-input-menu' }"
    no-filter
    hide-no-data
    append-icon=""
    @input="onInput"
    @keyup.enter="$emit('enter')"
  />
</template>

<script>
import {
  fetchPlaceSuggestions,
  newSessionToken,
  isPlacesEnabled,
} from "@/utils"

/**
 * Venue / address input, shared by the three places a gathering's location can
 * be set: the Call a Gathering modal, the Schedule menu, and the inline editor
 * on the event page.
 *
 * A combobox rather than an autocomplete because free-form text is a
 * first-class answer here — "Greg's back garden" is a perfectly good venue.
 * Google suggestions are an optional convenience layered on top; with no Maps
 * key configured this is simply a text field, which is what it was before.
 */
export default {
  name: "LocationInput",

  props: {
    value: { type: String, default: "" },
    label: { type: String, default: undefined },
    placeholder: { type: String, default: "Venue or address…" },
    solo: { type: Boolean, default: false },
    dense: { type: Boolean, default: false },
    hideDetails: { type: [Boolean, String], default: "auto" },
    autofocus: { type: Boolean, default: false },
    hideIcon: { type: Boolean, default: false },
  },

  data() {
    return {
      items: [],
      loading: false,
      search: this.value ?? "",
      sessionToken: null,
      debounceTimer: null,
      // Guards against an in-flight lookup resolving after a newer one
      requestSeq: 0,
    }
  },

  watch: {
    value(v) {
      // Keep the typed text in step when the parent resets or reassigns
      if ((v ?? "") !== (this.search ?? "")) this.search = v ?? ""
    },
    search(query) {
      // v-combobox reports free typing here; mirror it out so the parent sees
      // text that was never "selected" from the menu
      if ((query ?? "") !== (this.value ?? "")) this.$emit("input", query ?? "")
      this.queueLookup(query)
    },
  },

  methods: {
    queueLookup(query) {
      if (!isPlacesEnabled()) return
      clearTimeout(this.debounceTimer)

      if (!query || query.trim().length < 3) {
        this.items = []
        this.loading = false
        return
      }

      this.debounceTimer = setTimeout(() => this.lookup(query), 250)
    },

    async lookup(query) {
      const seq = ++this.requestSeq
      this.loading = true
      try {
        if (!this.sessionToken) this.sessionToken = await newSessionToken()
        const suggestions = await fetchPlaceSuggestions(
          query,
          this.sessionToken
        )
        if (seq !== this.requestSeq) return // a newer keystroke won
        this.items = suggestions.map((s) => s.text)
      } finally {
        if (seq === this.requestSeq) this.loading = false
      }
    },

    onInput(v) {
      // Picking a suggestion ends the billing session
      this.sessionToken = null
      this.items = []
      this.$emit("input", typeof v === "string" ? v : v?.text ?? "")
    },
  },

  beforeDestroy() {
    clearTimeout(this.debounceTimer)
  },
}
</script>

<style>
/* Match the dark parchment/brass surfaces rather than Vuetify's light menu */
.location-input-menu {
  background-color: #2e2117; /* leather */
}
.location-input-menu .v-list-item__title {
  color: #ede4d3; /* parchment */
  font-size: 0.875rem;
}
</style>
