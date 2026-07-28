<template>
  <div
    class="tw-mt-4 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-mb-2 tw-text-base tw-font-medium">Discussion</div>

    <!-- Signed-out: the discussion is members-and-guests-only, no anonymous access -->
    <div v-if="!authUser" class="tw-text-sm tw-text-parchment-dim">
      <a class="tw-text-brass" @click="goToSignIn">Sign in</a> to read and join the
      discussion.
    </div>

    <template v-else>
      <div v-if="topLevel.length" class="tw-space-y-3">
        <div v-for="comment in topLevel" :key="comment._id">
          <!-- ── Thread root: collapsible header ── -->
          <div
            v-if="comment.isThread"
            class="tw-rounded tw-border tw-border-brass-dim/60 tw-bg-wood-deep/30"
          >
            <div
              class="tw-flex tw-cursor-pointer tw-items-start tw-gap-2 tw-p-2"
              @click="toggleThread(comment._id)"
            >
              <v-icon small class="tw-mt-0.5 tw-flex-none">
                {{ isExpanded(comment._id) ? "mdi-chevron-down" : "mdi-chevron-right" }}
              </v-icon>
              <div class="tw-min-w-0 tw-flex-grow">
                <div class="tw-flex tw-items-center tw-gap-2">
                  <span class="tw-truncate tw-text-sm tw-font-medium">
                    {{ threadTitle(comment.text) }}
                  </span>
                  <v-icon
                    v-if="comment.membersOnly"
                    x-small
                    class="tw-flex-none"
                    title="Members only — guests can't see this thread"
                    >mdi-lock-outline</v-icon
                  >
                </div>
                <div class="tw-text-xs tw-text-parchment-dim">
                  {{ comment.authorName }} ·
                  {{ replyCountLabel(replyCountFor(comment._id)) }}
                </div>
              </div>
            </div>

            <!-- Expanded thread body -->
            <div v-if="isExpanded(comment._id)" class="tw-border-t tw-border-brass-dim/60 tw-p-2">
              <!-- The root comment in full -->
              <CommentRow
                :comment="comment"
                :can-edit="canEditComment(comment)"
                :can-delete="canDeleteComment(comment)"
                :editing="editingId === comment._id"
                :edit-text.sync="editText"
                @start-edit="startEdit"
                @cancel-edit="cancelEdit"
                @save-edit="saveEdit"
                @remove="remove"
              />

              <!-- Replies -->
              <div class="tw-mt-2 tw-space-y-2 tw-border-l tw-border-brass-dim/60 tw-pl-3">
                <CommentRow
                  v-for="reply in repliesFor(comment._id)"
                  :key="reply._id"
                  :comment="reply"
                  :can-edit="canEditComment(reply)"
                  :can-delete="canDeleteComment(reply)"
                  :editing="editingId === reply._id"
                  :edit-text.sync="editText"
                  @start-edit="startEdit"
                  @cancel-edit="cancelEdit"
                  @save-edit="saveEdit"
                  @remove="remove"
                />

                <!-- Reply composer -->
                <div class="tw-flex tw-items-end tw-gap-2 tw-pt-1">
                  <v-textarea
                    v-model="replyText[comment._id]"
                    placeholder="Reply…"
                    dense
                    hide-details
                    auto-grow
                    :rows="1"
                    class="tw-flex-grow tw-text-sm"
                  ></v-textarea>
                  <v-btn
                    small
                    class="tw-bg-brass tw-text-wood-deep"
                    :disabled="!canReply(comment._id)"
                    @click="submitReply(comment)"
                    >Reply</v-btn
                  >
                </div>
              </div>

              <!-- Thread management -->
              <div v-if="comment.canManageThread" class="tw-mt-2 tw-flex tw-gap-3">
                <a class="tw-text-xs tw-text-brass" @click="toggleMembersOnly(comment)">
                  {{ comment.membersOnly ? "Make visible to guests" : "Make members only" }}
                </a>
                <a
                  v-if="replyCountFor(comment._id) === 0"
                  class="tw-text-xs tw-text-brass"
                  @click="untag(comment)"
                  >Un-tag thread</a
                >
                <span v-else class="tw-text-xs tw-text-parchment-dim"
                  >Threads can't be un-tagged once replied to</span
                >
              </div>
            </div>
          </div>

          <!-- ── Ordinary top-level comment ── -->
          <CommentRow
            v-else
            :comment="comment"
            :can-edit="canEditComment(comment)"
            :can-delete="canDeleteComment(comment)"
            :can-tag-thread="canSeeMembersOnly"
            :editing="editingId === comment._id"
            :edit-text.sync="editText"
            @start-edit="startEdit"
            @cancel-edit="cancelEdit"
            @save-edit="saveEdit"
            @remove="remove"
            @tag-thread="openTagDialog"
          />
        </div>
      </div>
      <div v-else class="tw-text-sm tw-text-parchment-dim">
        No messages yet — start the conversation.
      </div>

      <!-- Composer -->
      <div class="tw-mt-3 tw-border-t tw-border-brass-dim tw-pt-3">
        <div class="tw-flex tw-items-end tw-gap-2">
          <v-textarea
            v-model="newText"
            placeholder="Add a message…"
            dense
            hide-details
            auto-grow
            :rows="1"
            class="tw-flex-grow tw-text-sm"
          ></v-textarea>
          <v-btn
            small
            class="tw-bg-brass tw-text-wood-deep"
            :disabled="!canPost"
            @click="submit"
            >Post</v-btn
          >
        </div>
      </div>
    </template>

    <!-- Tag-as-thread dialog -->
    <v-dialog v-model="tagDialog" max-width="420">
      <v-card>
        <v-card-title class="tw-text-base">Tag as thread</v-card-title>
        <v-card-text>
          <div class="tw-mb-3 tw-text-sm tw-italic">
            “{{ threadTitle(tagTarget ? tagTarget.text : "") }}”
          </div>
          <v-checkbox
            v-model="tagMembersOnly"
            dense
            hide-details
            label="Members only"
            class="tw-mt-0"
          ></v-checkbox>
          <div class="tw-mt-1 tw-text-xs tw-text-parchment-dim">
            Guests won't see this thread or any replies within it.
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text small @click="tagDialog = false">Cancel</v-btn>
          <v-btn small color="primary" @click="confirmTag">Create thread</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script>
import { mapGetters, mapState } from "vuex"
import dayjs from "dayjs"
import CommentRow from "@/components/event/CommentRow.vue"
import {
  groupComments,
  replyCount,
  replyCountLabel,
  threadTitle,
} from "@/components/event/commentThreads"

/**
 * Event discussion (C7) with threads (C13). Presentational: reads
 * event.comments, emits add-comment / edit-comment / delete-comment /
 * tag-thread / set-thread-members-only / untag-thread for Event.vue to persist
 * and refresh.
 *
 * Sign-in required — the server sends no comments at all to anonymous callers,
 * so the signed-out state here is a prompt, not a filtered list. Any member or
 * admin can tag a top-level comment as a thread; a members-only thread is hidden
 * server-side from guests, so nothing in this component is load-bearing for
 * privacy.
 */
export default {
  name: "EventComments",

  components: { CommentRow },

  props: {
    event: { type: Object, required: true },
  },

  data: () => ({
    newText: "",
    editingId: null,
    editText: "",
    expandedThreads: [],
    replyText: {},
    tagDialog: false,
    tagTarget: null,
    tagMembersOnly: false,
  }),

  emits: [
    "add-comment",
    "edit-comment",
    "delete-comment",
    "tag-thread",
    "set-thread-members-only",
    "untag-thread",
  ],

  computed: {
    ...mapState(["authUser"]),
    ...mapGetters(["canSeeMembersOnly"]),
    grouped() {
      return groupComments(this.event.comments)
    },
    topLevel() {
      return this.grouped.topLevel
    },
    isEventOwner() {
      return (
        !!this.authUser &&
        !!this.event.ownerId &&
        this.event.ownerId !== 0 &&
        this.authUser._id === this.event.ownerId
      )
    },
    canPost() {
      return !!this.authUser && this.newText.trim().length > 0
    },
  },

  methods: {
    threadTitle,
    replyCountLabel,
    formatTime(dt) {
      return dayjs(dt).format("MMM D, h:mm A")
    },
    repliesFor(threadId) {
      return this.grouped.repliesByThreadId[threadId] ?? []
    },
    replyCountFor(threadId) {
      return replyCount(this.grouped.repliesByThreadId, threadId)
    },
    isExpanded(threadId) {
      return this.expandedThreads.includes(threadId)
    },
    toggleThread(threadId) {
      const i = this.expandedThreads.indexOf(threadId)
      if (i === -1) {
        // Seed the draft key before the composer renders. Vue 2 can't observe a
        // property added later by v-model, so without this the Reply button
        // would never notice the textarea filling up.
        if (!(threadId in this.replyText)) this.$set(this.replyText, threadId, "")
        this.expandedThreads.push(threadId)
      } else {
        this.expandedThreads.splice(i, 1)
      }
    },
    // Legacy guest-authored rows have no signed-in author, so nobody can claim
    // them; they stay readable and remain deletable by the event owner.
    isMine(comment) {
      return !comment.isGuest && !!this.authUser && comment.userId === this.authUser._id
    },
    canEditComment(comment) {
      return this.isMine(comment)
    },
    canDeleteComment(comment) {
      return this.isMine(comment) || this.isEventOwner
    },
    canReply(threadId) {
      return (this.replyText[threadId] ?? "").trim().length > 0
    },
    goToSignIn() {
      this.$router.push({ name: "sign-in" })
    },

    submit() {
      const text = this.newText.trim()
      if (!text) return
      this.$emit("add-comment", { text })
      this.newText = ""
    },
    submitReply(root) {
      const text = (this.replyText[root._id] ?? "").trim()
      if (!text) return
      this.$emit("add-comment", { text, threadId: root._id })
      // Vue 2 reactivity: replyText keys are added dynamically.
      this.$set(this.replyText, root._id, "")
    },
    startEdit(comment) {
      this.editingId = comment._id
      this.editText = comment.text
    },
    cancelEdit() {
      this.editingId = null
      this.editText = ""
    },
    saveEdit(comment) {
      const text = this.editText.trim()
      if (!text) return
      this.$emit("edit-comment", { commentId: comment._id, payload: { text } })
      this.cancelEdit()
    },
    remove(comment) {
      this.$emit("delete-comment", { commentId: comment._id })
    },

    openTagDialog(comment) {
      this.tagTarget = comment
      this.tagMembersOnly = false
      this.tagDialog = true
    },
    confirmTag() {
      if (!this.tagTarget) return
      this.$emit("tag-thread", {
        commentId: this.tagTarget._id,
        payload: { membersOnly: this.tagMembersOnly },
      })
      this.tagDialog = false
      this.tagTarget = null
    },
    toggleMembersOnly(comment) {
      this.$emit("set-thread-members-only", {
        commentId: comment._id,
        payload: { membersOnly: !comment.membersOnly },
      })
    },
    untag(comment) {
      this.$emit("untag-thread", { commentId: comment._id })
    },
  },
}
</script>
