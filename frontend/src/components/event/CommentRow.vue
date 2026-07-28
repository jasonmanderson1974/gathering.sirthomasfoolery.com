<template>
  <div class="tw-flex tw-gap-2">
    <v-avatar :size="22" class="tw-mt-0.5 tw-flex-none">
      <v-icon small>mdi-account</v-icon>
    </v-avatar>
    <div class="tw-min-w-0 tw-flex-grow">
      <div class="tw-flex tw-items-baseline tw-gap-2">
        <span class="tw-text-sm tw-font-medium">{{ comment.authorName }}</span>
        <span class="tw-text-xs tw-text-parchment-dim">
          {{ formatTime(comment.createdAt) }}
          <span v-if="comment.updatedAt">· edited</span>
        </span>
      </div>

      <!-- Inline edit -->
      <div v-if="editing" class="tw-mt-1 tw-flex tw-items-center tw-gap-2">
        <v-textarea
          v-model="text"
          dense
          hide-details
          auto-grow
          :rows="1"
          class="tw-flex-grow tw-text-sm"
          autofocus
        ></v-textarea>
        <v-btn icon x-small @click="$emit('cancel-edit')">
          <v-icon small>mdi-close</v-icon>
        </v-btn>
        <v-btn icon x-small color="primary" @click="$emit('save-edit', comment)">
          <v-icon small>mdi-check</v-icon>
        </v-btn>
      </div>

      <div v-else class="tw-whitespace-pre-wrap tw-break-words tw-text-sm tw-text-parchment-dim">
        {{ comment.text }}
      </div>

      <!-- Controls -->
      <div v-if="!editing" class="tw-mt-0.5 tw-flex tw-gap-3">
        <a v-if="canEdit" class="tw-text-xs tw-text-brass" @click="$emit('start-edit', comment)"
          >Edit</a
        >
        <a v-if="canDelete" class="tw-text-xs tw-text-red" @click="$emit('remove', comment)"
          >Delete</a
        >
        <a
          v-if="canTagThread"
          class="tw-text-xs tw-text-brass"
          @click="$emit('tag-thread', comment)"
          >Make a thread</a
        >
      </div>
    </div>
  </div>
</template>

<script>
import dayjs from "dayjs"

/**
 * A single comment in the event discussion — used both for top-level messages
 * and for replies inside a thread (C13). Purely presentational; every action is
 * emitted up to EventComments.vue, which owns the editing state.
 */
export default {
  name: "CommentRow",

  props: {
    comment: { type: Object, required: true },
    canEdit: { type: Boolean, default: false },
    canDelete: { type: Boolean, default: false },
    /** Whether to offer "Make a thread" (member+, top-level comments only). */
    canTagThread: { type: Boolean, default: false },
    editing: { type: Boolean, default: false },
    editText: { type: String, default: "" },
  },

  emits: ["start-edit", "cancel-edit", "save-edit", "remove", "tag-thread", "update:editText"],

  computed: {
    // Proxies the parent's shared edit buffer via .sync, so only one row is ever
    // in edit mode and the parent keeps ownership of the draft text.
    text: {
      get() {
        return this.editText
      },
      set(value) {
        this.$emit("update:editText", value)
      },
    },
  },

  methods: {
    formatTime(dt) {
      return dayjs(dt).format("MMM D, h:mm A")
    },
  },
}
</script>
