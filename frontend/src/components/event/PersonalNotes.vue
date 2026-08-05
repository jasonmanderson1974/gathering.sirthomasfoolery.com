<template>
  <div
    class="tw-mt-3 tw-rounded-md tw-border tw-border-brass-dim tw-bg-leather tw-p-3 tw-text-parchment sm:tw-p-4"
  >
    <div class="tw-mb-2 tw-flex tw-items-center tw-justify-between tw-gap-2">
      <div class="tw-text-base tw-font-medium">My Notes</div>
      <!-- Write / Preview, in the band tabs' own idiom rather than a
           v-btn-toggle: this switches which panel you are looking at, which is
           what the tabs above already mean here. -->
      <div class="tw-flex tw-flex-none tw-gap-1">
        <v-btn
          v-for="mode in modes"
          :key="mode.value"
          text
          small
          :class="`tw-text-xs ${
            view === mode.value
              ? 'tw-bg-brass/10 tw-text-brass'
              : 'tw-text-parchment-dim'
          }`"
          @click="setView(mode.value)"
        >
          {{ mode.title }}
        </v-btn>
      </div>
    </div>

    <div v-if="loading" class="tw-py-6 tw-text-center">
      <v-progress-circular indeterminate color="brass" size="24" />
    </div>

    <template v-else>
      <!-- The toolbar: markdown without having to remember markdown.
           v-if, NOT v-show, and that is forced rather than chosen: this
           project sets Tailwind's `important: true`, so `tw-flex` compiles to
           `display: flex !important` and beats the inline `display: none`
           v-show sets. A v-show here renders a toolbar over the preview. The
           buttons are stateless, so there is nothing to preserve by keeping
           them mounted. -->
      <div
        v-if="view === 'write'"
        class="tw-mb-2 tw-flex tw-flex-wrap tw-items-center"
        :class="phone ? 'tw-gap-1' : ''"
      >
        <v-btn
          v-for="action in actions"
          :key="action.id"
          icon
          :x-small="!phone"
          :small="phone"
          class="tw-text-parchment-dim"
          :title="action.title"
          @click="format(action.id)"
        >
          <v-icon small>{{ action.icon }}</v-icon>
        </v-btn>
      </div>

      <!-- v-show, not v-if: the textarea keeps its caret and its scroll
           position across a trip to the preview and back. -->
      <v-textarea
        v-show="view === 'write'"
        ref="input"
        :value="draft"
        placeholder="Anything you want to remember about this gathering — only you can see it."
        dense
        hide-details
        auto-grow
        :rows="10"
        :maxlength="maxLength"
        class="tw-text-sm"
        @input="onInput"
        @blur.native="flush"
      />

      <div
        v-show="view === 'preview'"
        class="flw-prose tw-min-h-[6rem] tw-text-sm"
        v-html="rendered"
      ></div>

      <div class="tw-mt-2 tw-flex tw-items-center tw-justify-between tw-gap-2">
        <div :class="`tw-text-xs ${toneClass}`">{{ saveState.text }}</div>
        <div class="tw-flex tw-flex-none tw-items-center tw-gap-3">
          <div v-if="nearLimit" class="tw-text-xs tw-text-parchment-dim">
            {{ draft.length }} / {{ maxLength }}
          </div>
          <v-btn
            small
            text
            class="tw-text-brass"
            :disabled="!dirty || saving"
            @click="save"
          >
            Save
          </v-btn>
        </div>
      </div>
    </template>
  </div>
</template>

<script>
import { mapActions } from "vuex"
import dayjs from "dayjs"
import { isPhone } from "@/utils"
import { renderMarkdown } from "@/utils/markdown"
import {
  TOOLBAR_ACTIONS,
  NOTE_MAX_LENGTH,
  applyToolbarAction,
  isNoteDirty,
  describeSaveState,
} from "@/components/event/markdownEditor"
import { getPersonalNote, savePersonalNote } from "@/utils/services/PersonalService"

const AUTOSAVE_DELAY_MS = 1500

/**
 * The "My Notes" tab (F20): one private markdown document per person per
 * gathering.
 *
 * All the formatting logic lives in markdownEditor.js; this owns only what
 * needs a DOM — where the caret is, and putting it back after a toolbar press —
 * and when to save. Same division as MentionTextarea.vue, and for the same
 * reason: the vitest environment has no jsdom, so anything left in here is
 * untestable.
 */
export default {
  name: "PersonalNotes",

  props: {
    /** Short id or _id — whichever the route gave; the server resolves both. */
    eventId: { type: String, required: true },
    /**
     * Whether this tab is the one on screen. Going true loads the note; going
     * FALSE flushes a pending save, which is what makes switching tabs
     * mid-sentence safe.
     */
    active: { type: Boolean, default: false },
  },

  data: () => ({
    view: "write",
    draft: "",
    // What the server last confirmed. `dirty` is the comparison of the two, not
    // a flag — typing a character and deleting it must leave the note clean.
    saved: "",
    savedAt: null,
    loading: true,
    loaded: false,
    saving: false,
    failed: false,
    // The text of the save currently in flight, so its result can be applied
    // without clobbering anything typed while it was away.
    inFlightText: null,
    timer: null,
    maxLength: NOTE_MAX_LENGTH,
    actions: TOOLBAR_ACTIONS,
    modes: [
      { value: "write", title: "Write" },
      { value: "preview", title: "Preview" },
    ],
  }),

  computed: {
    phone() {
      return isPhone(this.$vuetify)
    },
    dirty() {
      return isNoteDirty(this.draft, this.saved)
    },
    saveState() {
      return describeSaveState({
        dirty: this.dirty,
        saving: this.saving,
        failed: this.failed,
        savedAtLabel: this.savedAt ? dayjs(this.savedAt).format("HH:mm") : null,
      })
    },
    toneClass() {
      if (this.saveState.tone === "error") return "tw-text-red"
      if (this.saveState.tone === "brass") return "tw-text-brass"
      return "tw-text-parchment-dim"
    },
    // The counter only appears when it is about to matter. A running character
    // count on a blank page is discouraging.
    nearLimit() {
      return this.draft.length > this.maxLength * 0.9
    },
    rendered() {
      // Safe for v-html: renderMarkdown runs the output through DOMPurify with
      // an explicit tag allowlist (utils/markdown.js).
      return renderMarkdown(this.draft)
    },
  },

  created() {
    if (this.active) this.load()
  },

  mounted() {
    window.addEventListener("beforeunload", this.warnIfUnsaved)
  },

  beforeDestroy() {
    window.removeEventListener("beforeunload", this.warnIfUnsaved)
    this.flush()
  },

  watch: {
    active(now, before) {
      if (now && !before) {
        this.load()
        return
      }
      // Leaving the tab: whatever is typed goes now, rather than waiting out a
      // debounce the user can no longer see.
      if (!now && before) this.flush()
    },
  },

  methods: {
    ...mapActions(["showError"]),

    /** The underlying <textarea>, which Vuetify exposes as its own `input` ref. */
    field() {
      return this.$refs.input?.$refs?.input ?? null
    },

    /** Fetched once. Re-reading would risk overwriting an unsaved draft. */
    async load() {
      if (this.loaded) return
      this.loaded = true
      try {
        const note = (await getPersonalNote(this.eventId)) ?? {}
        this.draft = note.text ?? ""
        this.saved = this.draft
        this.savedAt = note.updatedAt ?? null
      } catch (err) {
        this.loaded = false // let opening the tab again retry
        this.showError("Could not load your notes. Please try again.")
      } finally {
        this.loading = false
      }
    },

    setView(value) {
      // Going to the preview is a natural pause — save on the way.
      if (value === "preview") this.flush()
      this.view = value
    },

    onInput(value) {
      this.draft = value ?? ""
      this.failed = false
      this.scheduleSave()
    },

    scheduleSave() {
      clearTimeout(this.timer)
      if (!this.dirty) return
      this.timer = setTimeout(this.save, AUTOSAVE_DELAY_MS)
    },

    /** Save now, cancelling any pending autosave. */
    flush() {
      clearTimeout(this.timer)
      this.timer = null
      if (this.dirty && !this.saving) this.save()
    },

    /**
     * One request at a time.
     *
     * Two overlapping PUTs of a whole document is the one way this feature
     * could lose text: the second could land first, and the first would then
     * resurrect the older version on top of it. Anything typed while a save is
     * in flight is picked up by the follow-up scheduled when it returns.
     */
    async save() {
      if (this.saving || !this.dirty) return
      this.saving = true
      this.failed = false
      const sending = this.draft
      this.inFlightText = sending

      try {
        const note = await savePersonalNote(this.eventId, sending)
        // Trust the server's copy: it trimmed the text and stamped the time.
        this.saved = note?.text ?? sending
        this.savedAt = note?.updatedAt ?? Date.now()
      } catch (err) {
        this.failed = true
        this.showError("Could not save your notes. Please try again.")
      } finally {
        this.saving = false
        this.inFlightText = null
      }

      // Typed while that was away — go again.
      if (!this.failed && this.dirty) this.scheduleSave()
    },

    /**
     * A native "leave without saving?" prompt while there is unsaved text —
     * the same guard Event.vue already puts on unsaved availability. A tab
     * being closed is the one exit no flush can outrun.
     */
    warnIfUnsaved(e) {
      if (!this.dirty) return
      e.preventDefault()
      e.returnValue = ""
    },

    /**
     * Apply a toolbar action to whatever is selected, then put the caret back.
     *
     * The selection is read off the FIELD rather than from component state: a
     * click or an arrow key moves it without touching `draft`, so state would
     * lag by exactly the amount that matters here.
     */
    format(action) {
      const field = this.field()
      const next = applyToolbarAction(action, {
        text: field ? field.value : this.draft,
        selectionStart: field ? field.selectionStart : this.draft.length,
        selectionEnd: field ? field.selectionEnd : this.draft.length,
      })

      this.draft = next.text
      this.failed = false
      this.scheduleSave()

      // $nextTick is mandatory in Vue 2: the value round-trips through the
      // binding, and setting a selection before the DOM holds the new text is
      // a silent no-op.
      this.$nextTick(() => {
        const settled = this.field()
        if (!settled) return
        settled.focus()
        settled.setSelectionRange(next.selectionStart, next.selectionEnd)
      })
    },
  },
}
</script>
