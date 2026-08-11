<template>
  <div class="tw-mt-1 tw-max-w-full sm:tw-mt-2 sm:tw-max-w-[calc(100%-236px)]">
    <!-- Display -->
    <div
      v-if="showLocation"
      class="tw-flex tw-w-full tw-items-center tw-gap-2 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-2 tw-text-xs tw-font-normal tw-text-parchment-dim sm:tw-text-sm"
    >
      <v-icon size="small" class="tw-text-brass">mdi-map-marker</v-icon>
      <a
        :href="mapsUrl"
        target="_blank"
        rel="noopener"
        class="tw-grow tw-text-parchment-dim hover:tw-text-brass"
        >{{ event.location }}</a
      >
      <v-btn
        v-if="canEdit"
        key="edit-location-btn"
        class="-tw-my-1"
        icon
        size="small"
        @click="startEditing"
      >
        <v-icon size="small">mdi-pencil</v-icon>
      </v-btn>
    </div>

    <!-- Add (owner, empty) -->
    <v-btn
      v-else-if="canEdit && !isEditing"
      variant="text"
      class="-tw-ml-2 tw-mt-0 tw-w-min tw-whitespace-nowrap tw-px-2 tw-text-parchment-dim"
      @click="startEditing"
    >
      + Add location
    </v-btn>

    <!-- Edit -->
    <div v-if="isEditing" class="tw-mt-1">
      <div
        class="-tw-mt-[6px] tw-flex tw-w-full tw-flex-grow tw-items-center tw-gap-2"
      >
        <LocationInput
          v-model="newLocation"
          class="tw-flex-grow tw-p-2 tw-text-xs sm:tw-text-sm"
          autofocus
          hide-icon
          hide-details
          @enter="saveLocation"
        />
        <v-btn icon :size="isPhone ? 'small' : 'default'" @click="cancelEditing">
          <v-icon>mdi-close</v-icon>
        </v-btn>
        <v-btn
          icon
          :size="isPhone ? 'small' : 'default'"
          color="primary"
          @click="saveLocation"
        >
          <v-icon>mdi-check</v-icon>
        </v-btn>
      </div>
    </div>
  </div>
</template>

<script>
import { mapActions } from "vuex"
import { isPhone, mapsSearchUrl, put } from "@/utils"
import LocationInput from "@/components/LocationInput.vue"

/**
 * Inline-editable venue/address on the event page (C12). Mirrors
 * EventDescription: everyone sees the location + an "open in maps" link; the
 * owner can edit it. Persists via PUT /events/:id (same minimal payload shape).
 */
export default {
  name: "EventLocation",

  components: {
    LocationInput,
  },

  props: {
    event: { type: Object, required: true },
    canEdit: { type: Boolean, required: true },
  },

  data() {
    return {
      isEditing: false,
      newLocation: this.event.location ?? "",
    }
  },

  computed: {
    isPhone() {
      return isPhone(this.$vuetify)
    },
    showLocation() {
      return this.event.location && !this.isEditing
    },
    mapsUrl() {
      return mapsSearchUrl(this.event.location)
    },
  },

  methods: {
    ...mapActions(["showError"]),
    startEditing() {
      this.newLocation = this.event.location ?? ""
      this.isEditing = true
    },
    cancelEditing() {
      this.newLocation = this.event.location ?? ""
      this.isEditing = false
    },
    saveLocation() {
      const oldEvent = { ...this.event }
      const trimmed = this.newLocation.trim()

      const newEvent = { ...this.event, location: trimmed }
      const eventPayload = {
        name: this.event.name,
        duration: this.event.duration,
        dates: this.event.dates,
        type: this.event.type,
        location: trimmed,
      }

      this.$emit("update:event", newEvent)
      this.isEditing = false
      put(`/events/${this.event._id}`, eventPayload).catch((err) => {
        console.error(err)
        this.showError("Failed to save location! Please try again later.")
        this.$emit("update:event", { ...oldEvent })
      })
    },
  },
}
</script>
