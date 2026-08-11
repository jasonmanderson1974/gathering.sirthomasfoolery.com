<template>
  <div class="tw-mx-auto tw-mb-12 tw-mt-5 tw-max-w-3xl">
    <div class="tw-flex tw-flex-col tw-gap-6 tw-p-4">
      <!-- Header -->
      <div class="tw-flex tw-flex-col tw-gap-1">
        <div class="tw-font-head tw-text-2xl tw-text-parchment sm:tw-text-3xl">
          The Chronicle
        </div>
        <div class="tw-text-sm tw-text-parchment-dim">
          A record of gatherings past — where the Fellowship has met, and who
          was there.
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="tw-flex tw-justify-center tw-py-12">
        <v-progress-circular indeterminate color="brass" />
      </div>

      <!-- Empty -->
      <div
        v-else-if="!entries.length"
        class="tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-6 tw-text-center tw-text-parchment-dim"
      >
        No gatherings have been recorded yet. Once a scheduled gathering's time
        passes, it is inscribed here.
      </div>

      <!-- Entries -->
      <div v-else class="tw-flex tw-flex-col tw-gap-4">
        <div
          v-for="entry in entries"
          :key="entry._id"
          class="tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-4 tw-text-parchment"
        >
          <div
            class="tw-flex tw-flex-wrap tw-items-baseline tw-justify-between tw-gap-x-3 tw-gap-y-1"
          >
            <router-link
              :to="`/e/${entry.shortId || entry.eventId}`"
              class="tw-font-head tw-text-lg tw-text-brass hover:tw-underline"
            >
              {{ entry.name || "A gathering" }}
            </router-link>
            <div class="tw-text-sm tw-text-parchment-dim">
              {{ formatDate(entry.startDate) }}
            </div>
          </div>

          <div
            v-if="entry.location"
            class="tw-mt-1 tw-text-sm tw-text-parchment-dim"
          >
            <v-icon x-small class="tw-mr-1 tw-text-parchment-dim"
              >mdi-map-marker</v-icon
            >
            {{ entry.location }}
          </div>

          <div
            v-if="entry.description"
            class="tw-mt-2 tw-whitespace-pre-wrap tw-break-words tw-text-sm tw-text-parchment-dim"
          >
            {{ entry.description }}
          </div>

          <!-- Attendees -->
          <div
            class="tw-mt-3 tw-border-t tw-border-brass-dim/60 tw-pt-2 tw-text-sm"
          >
            <template v-if="entry.attendees && entry.attendees.length">
              <span class="tw-font-medium"
                >{{ entry.headCount }} attended:</span
              >
              <span class="tw-text-parchment-dim">
                {{ attendeeList(entry.attendees) }}
              </span>
            </template>
            <span v-else class="tw-text-parchment-dim">
              No attendance was recorded.
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { mapGetters } from "vuex"
import dayjs from "dayjs"
import { get } from "@/utils"

/**
 * The Chronicle (C10): a members-only, read-only archive of past gatherings,
 * auto-captured by the server when a confirmed gathering's time passes. Members
 * only — guests are redirected home (the /chronicle API is member-gated too).
 */
export default {
  name: "Chronicle",

  metaInfo() {
    return { title: "The Chronicle" }
  },

  data() {
    return {
      entries: [],
      loading: true,
    }
  },

  computed: {
    ...mapGetters(["canInvite"]),
  },

  async created() {
    // Members only (mirrors the Fellowship directory + the member-gated API).
    if (!this.canInvite) {
      this.$router.replace({ name: "home" })
      return
    }
    try {
      this.entries = await get("/chronicle")
    } catch (err) {
      this.entries = []
    } finally {
      this.loading = false
    }
  },

  methods: {
    formatDate(dt) {
      return dayjs(dt).format("dddd, MMMM D, YYYY · h:mm A")
    },
    attendeeList(attendees) {
      return attendees
        .map((a) =>
          a.guestCount > 0 ? `${a.name} (+${a.guestCount})` : a.name
        )
        .join(", ")
    },
  },
}
</script>
