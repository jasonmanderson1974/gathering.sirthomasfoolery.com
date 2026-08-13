<template>
  <div class="tw-flex tw-gap-2">
    <div class="tw-mt-0.5 tw-flex-none">
      <MemberHoverCard :user-id="authorId" :fallback="author">
        <UserAvatarContent :user="author" :size="22" />
      </MemberHoverCard>
    </div>
    <div class="tw-min-w-0 tw-flex-grow">
      <div class="tw-flex tw-items-baseline tw-gap-2">
        <MemberHoverCard :user-id="authorId" :fallback="author">
          <span class="tw-text-sm tw-font-medium">
            {{ comment.authorName }}
          </span>
        </MemberHoverCard>
        <span class="tw-text-xs tw-text-parchment-dim">
          {{ formatTime(comment.createdAt) }}
          <span v-if="comment.updatedAt">· edited</span>
        </span>
      </div>

      <!-- Inline edit -->
      <div v-if="editing" class="tw-mt-1 tw-flex tw-items-center tw-gap-2">
        <v-textarea
          v-model="text"
          density="compact"
          hide-details
          auto-grow
          :rows="1"
          class="tw-flex-grow tw-text-sm"
          autofocus
        ></v-textarea>
        <v-btn icon size="x-small" @click="$emit('cancel-edit')">
          <v-icon size="small">mdi-close</v-icon>
        </v-btn>
        <v-btn
          icon
          size="x-small"
          color="primary"
          @click="$emit('save-edit', comment)"
        >
          <v-icon size="small">mdi-check</v-icon>
        </v-btn>
      </div>

      <!-- One span per part, no whitespace between them: the parts already
           carry the comment's own spacing and this renders pre-wrap, so any
           the template added would show up in the message. -->
      <div
        v-else
        class="tw-whitespace-pre-wrap tw-break-words tw-text-sm tw-text-parchment-dim"
      >
        <span v-for="(part, i) in parts" :key="i" :class="mentionClass(part)">{{
          part.type === "mention" ? `@${part.text}` : part.text
        }}</span>
      </div>

      <!-- Controls -->
      <div v-if="!editing" class="tw-mt-0.5 tw-flex tw-gap-3">
        <a
          v-if="canEdit"
          class="tw-text-xs tw-text-brass"
          @click="$emit('start-edit', comment)"
          >Edit</a
        >
        <a
          v-if="canDelete"
          class="tw-text-xs tw-text-red"
          @click="$emit('remove', comment)"
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
import UserAvatarContent from "@/components/UserAvatarContent.vue"
import MemberHoverCard from "@/components/general/MemberHoverCard.vue"
import { splitMentions } from "@/components/event/mentionText"
import { userFromDisplayName } from "@/utils"

/**
 * A single comment in the event discussion — used both for top-level messages
 * and for replies inside a thread (C13). Purely presentational; every action is
 * emitted up to EventComments.vue, which owns the editing state.
 */
export default {
  name: "CommentRow",

  components: { UserAvatarContent, MemberHoverCard },

  props: {
    comment: { type: Object, required: true },
    canEdit: { type: Boolean, default: false },
    canDelete: { type: Boolean, default: false },
    /** Whether to offer "Make a thread" (member+, top-level comments only). */
    canTagThread: { type: Boolean, default: false },
    editing: { type: Boolean, default: false },
    editText: { type: String, default: "" },
    /** The reader's own account id, so a mention of them stands out (F9). */
    viewerId: { type: String, default: "" },
  },

  emits: [
    "start-edit",
    "cancel-edit",
    "save-edit",
    "remove",
    "tag-thread",
    "update:editText",
  ],

  computed: {
    /**
     * The comment split into text and mention runs (F9). Mentions are stored
     * inside the text as `@[Name](id)`, so a comment written before F7, or one
     * that simply names nobody, comes back as a single text part.
     */
    parts() {
      return splitMentions(this.comment.text)
    },
    /**
     * The account behind this comment, for the avatar. The server attaches
     * `author` for comments whose account still resolves; guests and deleted
     * accounts have none, so the stored name snapshot stands in and yields a
     * monogram.
     */
    author() {
      return this.comment.author ?? userFromDisplayName(this.comment.authorName)
    },
    /**
     * The account id for the hover card, and ONLY when the server resolved the
     * author. `comment.userId` is not usable here: legacy rows hold a guest's
     * NAME in that field, which would key the lookup on something that is not
     * an id and offer a card over someone who has no account at all.
     */
    authorId() {
      return this.comment.author?._id ?? ""
    },
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
    /**
     * Mentions read as brass, like every other name-shaped thing in the app;
     * being named yourself gets a background as well, so you can find it while
     * scanning a long thread rather than reading for your own name.
     */
    mentionClass(part) {
      if (part.type !== "mention") return null
      const mine = !!this.viewerId && part.userId === this.viewerId
      return mine
        ? "tw-rounded tw-bg-brass/20 tw-px-1 tw-font-medium tw-text-brass"
        : "tw-font-medium tw-text-brass"
    },
  },
}
</script>
