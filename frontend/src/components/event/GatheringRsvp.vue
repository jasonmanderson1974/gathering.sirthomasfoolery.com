<template>
  <div
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-flex tw-flex-wrap tw-items-center tw-gap-x-3 tw-gap-y-1">
      <div class="tw-text-base tw-font-medium">Who's coming?</div>
      <div class="tw-text-sm tw-text-parchment-dim">
        {{ counts.going }} going{{ guestSuffix(counts.goingGuests) }} ·
        {{ counts.maybe }} maybe{{ guestSuffix(counts.maybeGuests) }} ·
        {{ counts.no }} can't
      </div>
    </div>

    <!-- RSVP buttons -->
    <div class="tw-mt-3 tw-flex tw-flex-wrap tw-gap-2">
      <v-btn
        v-for="opt in options"
        :key="opt.value"
        size="small"
        :variant="myStatus === opt.value ? 'flat' : 'outlined'"
        :class="
          myStatus === opt.value
            ? 'tw-bg-brass tw-text-wood-deep'
            : 'tw-text-brass'
        "
        @click="choose(opt.value)"
      >
        <v-icon size="small" start>{{ opt.icon }}</v-icon>
        {{ opt.label }}
      </v-btn>
    </div>

    <!-- Plus-one stepper (only once you're going/maybe) -->
    <div
      v-if="canBringGuests"
      class="tw-mt-3 tw-flex tw-items-center tw-gap-2 tw-text-sm"
    >
      <span class="tw-text-parchment-dim">Bringing guests:</span>
      <v-btn
        icon
        size="x-small"
        variant="outlined"
        class="tw-text-brass"
        :disabled="localGuestCount <= 0"
        @click="changeGuests(-1)"
      >
        <v-icon size="small">mdi-minus</v-icon>
      </v-btn>
      <span class="tw-w-4 tw-text-center tw-font-medium">{{
        localGuestCount
      }}</span>
      <v-btn
        icon
        size="x-small"
        variant="outlined"
        class="tw-text-brass"
        :disabled="localGuestCount >= 20"
        @click="changeGuests(1)"
      >
        <v-icon size="small">mdi-plus</v-icon>
      </v-btn>
    </div>

    <!-- Roster grouped by status -->
    <div v-if="hasAnyRsvp" class="tw-mt-3 tw-space-y-2 tw-text-sm">
      <div v-for="opt in options" :key="`roster-${opt.value}`">
        <template v-if="rosters[opt.value].length">
          <div class="tw-font-medium">{{ opt.label }}</div>
          <div class="tw-mt-1 tw-flex tw-flex-wrap tw-gap-x-4 tw-gap-y-1.5">
            <div
              v-for="entry in rosters[opt.value]"
              :key="`${opt.value}-${entry.key}`"
              class="tw-flex tw-min-w-0 tw-items-center tw-gap-1.5"
            >
              <span class="tw-flex-none">
                <UserAvatarContent :user="entry.user" :size="22" />
              </span>
              <span class="tw-truncate tw-text-parchment-dim">
                {{ entry.label }}
              </span>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import { mapState } from "vuex"
import UserAvatarContent from "@/components/UserAvatarContent.vue"
import { userFromDisplayName } from "@/utils"

/**
 * RSVP widget for a confirmed gathering (shown when event.scheduledEvent
 * exists). Presentational: reads event.rsvps, emits set-rsvp / clear-rsvp for
 * Event.vue to persist. Signed-in users RSVP directly; guests enter a name
 * first (same trust model as guest availability).
 */
export default {
  name: "GatheringRsvp",

  components: { UserAvatarContent },

  props: {
    event: { type: Object, required: true },
  },

  data: () => ({
    localGuestCount: 0,
    options: [
      { value: "going", label: "Going", icon: "mdi-check" },
      { value: "maybe", label: "Maybe", icon: "mdi-help" },
      { value: "no", label: "Can't make it", icon: "mdi-close" },
    ],
  }),

  emits: ["set-rsvp", "clear-rsvp"],

  computed: {
    ...mapState(["authUser"]),
    rsvps() {
      return this.event.rsvps ?? {}
    },
    hasAnyRsvp() {
      return Object.keys(this.rsvps).length > 0
    },
    counts() {
      const c = { going: 0, maybe: 0, no: 0, goingGuests: 0, maybeGuests: 0 }
      for (const r of Object.values(this.rsvps)) {
        if (!r || c[r.status] === undefined) continue
        c[r.status]++
        if (r.status === "going") c.goingGuests += r.guestCount || 0
        else if (r.status === "maybe") c.maybeGuests += r.guestCount || 0
      }
      return c
    },
    /**
     * One entry per RSVP, grouped by status, carrying what a row renders: the
     * label (name + any plus-ones) and the account behind it.
     *
     * The server attaches `user` to every account-keyed RSVP (F11). A legacy
     * name-keyed row has none, so the stored name stands in and yields a
     * monogram — the same fallback comments use.
     */
    rosters() {
      const r = { going: [], maybe: [], no: [] }
      for (const [key, rsvp] of Object.entries(this.rsvps)) {
        if (!rsvp || !r[rsvp.status]) continue
        const name = rsvp.name || key
        const extra = rsvp.guestCount > 0 ? ` (+${rsvp.guestCount})` : ""
        r[rsvp.status].push({
          key,
          label: `${name}${extra}`,
          user: rsvp.user ?? userFromDisplayName(name),
        })
      }
      return r
    },
    // The map key identifying the current viewer. Always their user id — the
    // server keys RSVPs by session, so a name can no longer stand in for one.
    // Legacy name-keyed RSVPs still render in the roster; they just aren't
    // matched to a viewer.
    myKey() {
      return this.authUser?._id ?? null
    },
    myRsvp() {
      return this.myKey ? this.rsvps[this.myKey] : null
    },
    myStatus() {
      return this.myRsvp?.status ?? null
    },
    // Plus-ones only make sense once you're (tentatively) attending.
    canBringGuests() {
      return this.myStatus === "going" || this.myStatus === "maybe"
    },
  },

  watch: {
    // Keep the local stepper in sync with the persisted RSVP.
    myRsvp: {
      immediate: true,
      handler(rsvp) {
        this.localGuestCount = rsvp?.guestCount ?? 0
      },
    },
  },

  methods: {
    guestSuffix(n) {
      return n > 0 ? ` (+${n})` : ""
    },
    choose(status) {
      // Clicking the active choice again clears the RSVP.
      if (this.myStatus === status) {
        this.clear()
        return
      }
      this.submit(status, status === "no" ? 0 : this.localGuestCount)
    },
    changeGuests(delta) {
      const next = Math.min(20, Math.max(0, this.localGuestCount + delta))
      if (next === this.localGuestCount) return
      this.localGuestCount = next
      // Re-submit the existing RSVP with the new guest count.
      if (this.canBringGuests) this.submit(this.myStatus, next)
    },
    submit(status, guestCount) {
      this.$emit("set-rsvp", { status, guestCount })
    },
    clear() {
      this.$emit("clear-rsvp")
    },
  },
}
</script>
