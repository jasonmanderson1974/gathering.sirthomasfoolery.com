<template>
  <div
    v-if="polls.length || isEventOwner"
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-mb-2 tw-text-base tw-font-medium">Polls</div>

    <!-- Existing polls -->
    <div v-if="polls.length" class="tw-space-y-4">
      <div
        v-for="poll in polls"
        :key="poll._id"
        class="tw-rounded tw-border tw-border-brass-dim/60 tw-p-2 sm:tw-p-3"
      >
        <div class="tw-flex tw-items-start tw-justify-between tw-gap-2">
          <div class="tw-min-w-0">
            <div class="tw-break-words tw-font-medium">{{ poll.title }}</div>
            <div class="tw-text-xs tw-text-parchment-dim">
              {{ poll.allowMultiple ? "Choose one or more" : "Choose one" }}
            </div>
          </div>
          <v-btn
            v-if="isEventOwner"
            icon
            x-small
            class="tw-flex-none tw-text-red"
            title="Delete poll"
            @click="askDeletePoll(poll)"
          >
            <v-icon small>mdi-delete</v-icon>
          </v-btn>
        </div>

        <!-- Options -->
        <div class="tw-mt-2 tw-space-y-1">
          <div
            v-for="option in poll.options"
            :key="option._id"
            class="tw-flex tw-cursor-pointer tw-items-center tw-gap-2 tw-rounded tw-px-2 tw-py-1"
            :class="[
              isChosen(poll, option._id)
                ? 'tw-bg-brass/20'
                : 'hover:tw-bg-brass/10',
              !canVote && 'tw-cursor-default tw-opacity-90',
            ]"
            @click="toggle(poll, option._id)"
          >
            <v-icon
              small
              :class="
                isChosen(poll, option._id)
                  ? 'tw-text-brass'
                  : 'tw-text-parchment-dim'
              "
            >
              {{ chooseIcon(poll, option._id) }}
            </v-icon>
            <span class="tw-min-w-0 tw-flex-grow tw-break-words tw-text-sm">
              {{ option.label }}
            </span>
            <span class="tw-flex-none tw-text-xs tw-text-parchment-dim">
              {{ voteCount(option) }}
            </span>
          </div>
        </div>

        <!-- Voter roster (transparent, like the RSVP roster) -->
        <div
          v-if="pollHasVotes(poll)"
          class="tw-mt-2 tw-space-y-0.5 tw-text-xs"
        >
          <div v-for="option in poll.options" :key="`voters-${option._id}`">
            <template v-if="voters(option).length">
              <span class="tw-font-medium">{{ option.label }}:</span>
              <span class="tw-text-parchment-dim">
                {{ voters(option).join(", ") }}
              </span>
            </template>
          </div>
        </div>
      </div>
    </div>
    <div v-else-if="!showNewPoll" class="tw-text-sm tw-text-parchment-dim">
      No polls yet.
    </div>

    <!-- Owner: create a new poll -->
    <template v-if="isEventOwner">
      <div
        v-if="showNewPoll"
        class="tw-mt-3 tw-border-t tw-border-brass-dim tw-pt-3"
      >
        <v-text-field
          v-model="newTitle"
          label="Poll question (e.g. Where should we meet?)"
          dense
          hide-details
          class="tw-mb-2"
        />
        <div
          v-for="(opt, i) in newOptions"
          :key="i"
          class="tw-mb-1 tw-flex tw-items-center tw-gap-1"
        >
          <v-text-field
            v-model="newOptions[i]"
            :label="`Option ${i + 1}`"
            dense
            hide-details
          />
          <v-btn
            icon
            x-small
            class="tw-text-parchment-dim"
            :disabled="newOptions.length <= 2"
            @click="removeOption(i)"
          >
            <v-icon small>mdi-close</v-icon>
          </v-btn>
        </div>
        <div class="tw-mt-1 tw-flex tw-flex-wrap tw-items-center tw-gap-3">
          <a class="tw-text-xs tw-text-brass" @click="addOption"
            >+ Add option</a
          >
          <v-checkbox
            v-model="newAllowMultiple"
            label="Allow multiple choices"
            dense
            hide-details
            class="tw-mt-0 tw-pt-0"
          />
        </div>
        <div class="tw-mt-3 tw-flex tw-gap-2">
          <v-btn small text @click="cancelNewPoll">Cancel</v-btn>
          <v-btn
            small
            class="tw-bg-brass tw-text-wood-deep"
            :disabled="!canCreate"
            @click="createPoll"
            >Create poll</v-btn
          >
        </div>
      </div>
      <v-btn
        v-else
        small
        outlined
        class="tw-mt-3 tw-text-brass"
        @click="showNewPoll = true"
      >
        <v-icon small left>mdi-plus</v-icon>
        Add poll
      </v-btn>
    </template>

    <ConfirmDeleteDialog
      :model-value="!!pendingDelete"
      :title="pendingDelete ? pendingDelete.title : ''"
      :body="pendingDelete ? pendingDelete.body : ''"
      @input="onConfirmDialogInput"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script>
import { mapState } from "vuex"
import ConfirmDeleteDialog from "@/components/general/ConfirmDeleteDialog.vue"

/**
 * Venue / activity polls on an event (C6). Presentational: reads event.polls,
 * emits create-poll / delete-poll / vote-poll for Event.vue to persist +
 * refresh. The owner creates/deletes polls; members and guests vote (guests by
 * name, same trust model as RSVP/comments). Votes live on each option, so counts
 * and the voter roster render straight from the event.
 */
export default {
  name: "EventPolls",

  components: { ConfirmDeleteDialog },

  props: {
    event: { type: Object, required: true },
  },

  data: () => ({
    showNewPoll: false,
    newTitle: "",
    newAllowMultiple: false,
    newOptions: ["", ""],
    // The poll awaiting confirmation, with the wording for it. Null when the
    // dialog is shut.
    pendingDelete: null,
  }),

  emits: ["create-poll", "delete-poll", "vote-poll"],

  computed: {
    ...mapState(["authUser"]),
    polls() {
      return this.event.polls ?? []
    },
    isEventOwner() {
      return (
        !!this.authUser &&
        !!this.event.ownerId &&
        this.event.ownerId !== 0 &&
        this.authUser._id === this.event.ownerId
      )
    },
    // The map key identifying the current viewer. Always their user id — the
    // server keys votes by session, so a name can no longer stand in for one.
    // Legacy name-keyed votes still render in the roster.
    myKey() {
      return this.authUser?._id ?? null
    },
    canVote() {
      return !!this.myKey
    },
    canCreate() {
      return (
        this.newTitle.trim().length > 0 &&
        this.newOptions.filter((o) => o.trim().length > 0).length >= 2
      )
    },
  },

  methods: {
    // Deleting a poll takes everyone's votes with it, which is the part that
    // isn't obvious from a bin icon.
    askDeletePoll(poll) {
      const title = poll.title ?? ""
      const voters = new Set()
      for (const option of poll.options ?? []) {
        for (const id of Object.keys(option.votes ?? {})) voters.add(id)
      }
      this.pendingDelete = {
        title: `Delete "${
          title.length > 80 ? `${title.slice(0, 79)}…` : title
        }"?`,
        body: voters.size
          ? `This removes the poll and the ${voters.size} ${
              voters.size === 1 ? "vote" : "votes"
            } cast on it.`
          : "No one has voted on this poll yet.",
        pollId: poll._id,
      }
    },
    onConfirmDialogInput(open) {
      if (!open) this.pendingDelete = null
    },
    confirmDelete() {
      const pending = this.pendingDelete
      this.pendingDelete = null
      if (!pending) return
      this.$emit("delete-poll", pending.pollId)
    },

    voteCount(option) {
      return Object.keys(option.votes ?? {}).length
    },
    voters(option) {
      return Object.values(option.votes ?? {})
    },
    pollHasVotes(poll) {
      return poll.options.some((o) => this.voteCount(o) > 0)
    },
    isChosen(poll, optionId) {
      if (!this.myKey) return false
      const option = poll.options.find((o) => o._id === optionId)
      return !!option?.votes?.[this.myKey]
    },
    chooseIcon(poll, optionId) {
      const chosen = this.isChosen(poll, optionId)
      if (poll.allowMultiple) {
        return chosen ? "mdi-checkbox-marked" : "mdi-checkbox-blank-outline"
      }
      return chosen ? "mdi-radiobox-marked" : "mdi-radiobox-blank"
    },
    // Current selection set for a poll, derived from the persisted votes.
    mySelections(poll) {
      if (!this.myKey) return []
      return poll.options.filter((o) => o.votes?.[this.myKey]).map((o) => o._id)
    },
    toggle(poll, optionId) {
      if (!this.canVote) return
      const current = new Set(this.mySelections(poll))
      if (poll.allowMultiple) {
        current.has(optionId) ? current.delete(optionId) : current.add(optionId)
      } else {
        // Single choice: clicking the active option clears it; else select it.
        if (current.has(optionId)) current.clear()
        else {
          current.clear()
          current.add(optionId)
        }
      }
      // Identity is never sent — the server takes it from the session.
      this.$emit("vote-poll", {
        pollId: poll._id,
        payload: { optionIds: [...current] },
      })
    },
    addOption() {
      this.newOptions.push("")
    },
    removeOption(i) {
      if (this.newOptions.length > 2) this.newOptions.splice(i, 1)
    },
    createPoll() {
      if (!this.canCreate) return
      this.$emit("create-poll", {
        title: this.newTitle.trim(),
        allowMultiple: this.newAllowMultiple,
        options: this.newOptions.map((o) => o.trim()).filter(Boolean),
      })
      this.cancelNewPoll()
    },
    cancelNewPoll() {
      this.showNewPoll = false
      this.newTitle = ""
      this.newAllowMultiple = false
      this.newOptions = ["", ""]
    },
  },
}
</script>
