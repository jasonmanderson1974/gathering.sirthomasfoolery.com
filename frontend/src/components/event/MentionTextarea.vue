<template>
  <!-- Positioning context for the picker, which is absolute so it can overlap
       whatever sits under the composer instead of pushing it down the page. -->
  <div class="tw-relative tw-flex-grow">
    <v-textarea
      ref="input"
      :model-value="modelValue"
      :placeholder="placeholder"
      density="compact"
      hide-details
      auto-grow
      :rows="1"
      class="tw-text-sm"
      @update:model-value="onInput"
      @click="syncTrigger"
      @keyup="syncTrigger"
      @keydown="onKeydown"
      @blur="close"
    ></v-textarea>

    <div
      v-if="open"
      class="tw-absolute tw-left-0 tw-right-0 tw-top-full tw-z-20 tw-mt-1 tw-max-h-56 tw-overflow-y-auto tw-rounded tw-border tw-border-brass-dim tw-bg-wood-deep tw-shadow-lg"
    >
      <!-- mousedown, not click: the textarea would blur first and close the
           picker out from under the pointer, so the click would land on
           nothing. Preventing it also keeps the caret where it was. -->
      <div
        v-for="(candidate, i) in filtered"
        :key="candidate._id"
        class="tw-flex tw-cursor-pointer tw-items-center tw-gap-2 tw-px-2 tw-py-1.5 tw-text-sm"
        :class="
          i === highlight ? 'tw-bg-brass/20 tw-text-brass' : 'tw-text-parchment'
        "
        @mousedown.prevent="select(candidate)"
        @mouseenter="highlight = i"
      >
        <UserAvatarContent :user="candidate" :size="20" />
        <span class="tw-truncate">{{ nameOf(candidate) }}</span>
      </div>
    </div>
  </div>
</template>

<script>
import UserAvatarContent from "@/components/UserAvatarContent.vue"
import { displayName } from "@/utils"
import {
  applyMention,
  filterMentionables,
  mentionTrigger,
} from "@/components/event/mentionText"

/**
 * A comment composer that can @mention people (F9) — used by both the main
 * composer and the per-thread reply composer.
 *
 * All the text handling lives in `mentionText.js`; this owns only what needs a
 * DOM: where the caret is, which candidate is highlighted, and restoring focus
 * after a pick.
 *
 * The picker is a plain absolutely-positioned list rather than the `v-menu` the
 * plan called for. A v-menu manages focus and an overlay, both of which fight a
 * textarea the user is mid-sentence in; a list positioned under the field needs
 * neither and keeps every keystroke in the textarea, where it belongs. It is
 * anchored to the field, not the caret — fine for v1.
 *
 * Candidates come from the event, not from here: whether a caller may mention
 * the whole roll or only the people on this event is the server's rule
 * (GET /events/:id/mentionables), and it is enforced again on the write path,
 * so nothing in this component is load-bearing for privacy.
 */
export default {
  name: "MentionTextarea",

  components: { UserAvatarContent },

  props: {
    modelValue: { type: String, default: "" },
    placeholder: { type: String, default: "" },
    /** Slim users from GET /events/:eventId/mentionables. */
    candidates: { type: Array, default: () => [] },
  },

  emits: ["update:modelValue"],

  data: () => ({
    /** The partial mention at the caret, from mentionTrigger — null when closed. */
    trigger: null,
    highlight: 0,
    /**
     * Where an Escape-dismissed mention started, so it stays dismissed.
     *
     * Without this, Escape does nothing you can see: the keyup that ends the
     * same keypress re-reads the caret, finds the very same partial mention and
     * reopens the picker. Keyed by position rather than a bare flag so a
     * different `@` later in the comment still opens normally.
     */
    dismissedStart: null,
  }),

  computed: {
    filtered() {
      if (!this.trigger) return []
      return filterMentionables(this.candidates, this.trigger.query)
    },
    open() {
      return this.filtered.length > 0
    },
  },

  watch: {
    // A narrowing query can leave the highlight past the end of the list.
    filtered() {
      if (this.highlight >= this.filtered.length) this.highlight = 0
    },
  },

  methods: {
    nameOf: displayName,

    /** The underlying <textarea>, which Vuetify exposes as its own `input` ref. */
    field() {
      return this.$refs.input?.$refs?.input ?? null
    },

    onInput(value) {
      this.$emit("update:modelValue", value)
      // After the parent has taken the value: selectionStart is read off the
      // DOM, which is already current, but the trigger is only meaningful once
      // the keystroke that caused it is in the field.
      this.$nextTick(this.syncTrigger)
    },

    /**
     * Re-read what is being typed at the caret. Reads the field rather than the
     * `value` prop because the prop lags by a tick on input, and a selection
     * change (click, arrow key) doesn't touch it at all.
     */
    syncTrigger() {
      const field = this.field()
      if (!field) return

      const trigger = mentionTrigger(
        field.value.slice(0, field.selectionStart ?? 0)
      )
      if (!trigger) {
        // The caret has left the mention entirely, so there is nothing left to
        // keep dismissed.
        this.trigger = null
        this.dismissedStart = null
        return
      }
      this.trigger = trigger.start === this.dismissedStart ? null : trigger
    },

    close() {
      this.trigger = null
      this.dismissedStart = null
    },

    /** Escape: close the picker and keep it closed for this mention. */
    dismiss() {
      this.dismissedStart = this.trigger?.start ?? null
      this.trigger = null
    },

    /**
     * While the picker is open the arrow keys, Enter and Tab belong to it —
     * Enter especially, which would otherwise break the line mid-name. Every
     * other key falls through to the textarea untouched.
     *
     * Bound to VTextarea's own `keydown` event, never to a listener on this
     * component's root: Vuetify calls `e.stopPropagation()` on Enter while the
     * field is focused (so Enter can't close a surrounding dialog), so anything
     * listening further up sees every key except the one that matters most. It
     * re-emits the same event afterwards, which is why preventDefault here
     * still suppresses the newline.
     *
     * The other three listeners on the textarea are plain (`click`, `keyup`,
     * `blur`) rather than `.native`, which is removed in Vue 3. Nothing is lost:
     * VTextField spreads `$listeners` straight onto the inner element, so these
     * now bind to the `<textarea>` itself rather than to the wrapper — closer to
     * the event, not further from it. `blur` is the one exception, arriving via
     * Vuetify's own `onBlur` re-emit on the next tick.
     */
    onKeydown(e) {
      if (!this.open) return

      switch (e.key) {
        case "ArrowDown":
          e.preventDefault()
          this.highlight = (this.highlight + 1) % this.filtered.length
          break
        case "ArrowUp":
          e.preventDefault()
          this.highlight =
            (this.highlight - 1 + this.filtered.length) % this.filtered.length
          break
        case "Enter":
        case "Tab":
          e.preventDefault()
          this.select(this.filtered[this.highlight])
          break
        case "Escape":
          e.preventDefault()
          // Stop here: an Escape meant for the picker should not also close
          // whatever the composer happens to sit inside.
          e.stopPropagation()
          this.dismiss()
          break
      }
    },

    select(candidate) {
      const field = this.field()
      const text = field ? field.value : this.modelValue ?? ""
      const caret = field ? field.selectionStart ?? text.length : text.length

      const applied = applyMention(text, caret, candidate)
      this.close()
      if (!applied) return

      this.$emit("update:modelValue", applied.text)
      this.highlight = 0
      // Once the parent's re-render has put the token in the field: picking
      // with the mouse leaves focus in the composer, and the caret goes after
      // the token rather than back to the end of the whole comment.
      this.$nextTick(() => {
        const settled = this.field()
        if (!settled) return
        settled.focus()
        settled.setSelectionRange(applied.caret, applied.caret)
      })
    },
  },
}
</script>
