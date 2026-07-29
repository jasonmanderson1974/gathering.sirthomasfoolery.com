<template>
  <div>
    <!--
      The picker lives here rather than in the caller so the whole
      choose → crop → export flow stays in one component. Callers open it with
      $refs.<ref>.pickFile().
    -->
    <input
      ref="file"
      type="file"
      accept="image/*"
      class="tw-hidden"
      @change="onFileChosen"
    />

    <v-dialog
      :value="value"
      @input="onDialogInput"
      width="480"
      content-class="tw-m-0"
      persistent
    >
      <v-card>
        <v-card-title>Choose your photo</v-card-title>
        <v-card-text>
          <div class="tw-mb-3 tw-text-sm tw-text-parchment-dim">
            Drag to reposition, pinch or scroll to zoom. Your photo is saved as
            a square.
          </div>

          <div v-if="loading" class="tw-py-8 tw-text-center tw-text-sm">
            Preparing the editor…
          </div>

          <!--
            Cropper replaces this element with its own container, so it must be
            in the DOM before the library is initialised — hence the nextTick in
            openWith().
          -->
          <div v-show="!loading" class="tw-max-h-[60vh]">
            <img
              ref="image"
              :src="imageSrc"
              alt=""
              class="tw-block tw-max-w-full"
            />
          </div>
        </v-card-text>

        <v-card-actions>
          <v-btn
            text
            :disabled="!ready"
            @click="rotate(-90)"
            title="Rotate left"
          >
            <v-icon>mdi-rotate-left</v-icon>
          </v-btn>
          <v-btn
            text
            :disabled="!ready"
            @click="rotate(90)"
            title="Rotate right"
          >
            <v-icon>mdi-rotate-right</v-icon>
          </v-btn>
          <v-spacer />
          <v-btn text class="tw-text-brass" @click="close">Cancel</v-btn>
          <v-btn text :disabled="!ready" :loading="saving" @click="save">
            Save photo
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script>
import { mapActions } from "vuex"

/**
 * Crop-and-rotate dialog for a profile photo.
 *
 * Emits the finished image as a 256x256 JPEG data URL and does NOT know where
 * it goes — Settings PUTs it to /user/avatar, and the admin edit (F6) will PUT
 * the same thing to a different route on someone else's behalf.
 *
 * The server re-crops and re-encodes whatever it receives, so this is about
 * letting a member choose which part of their photo is shown, not about
 * producing bytes the backend trusts.
 */

// Comfortably larger than any phone photo; the point is to fail fast on a
// video or a RAW file rather than hang the browser reading it.
const MAX_FILE_BYTES = 10 * 1024 * 1024

export default {
  name: "AvatarEditorDialog",

  props: {
    value: { type: Boolean, default: false },
    /** Set by the caller while its upload request is in flight. */
    saving: { type: Boolean, default: false },
  },

  emits: ["input", "save"],

  data: () => ({
    imageSrc: "",
    cropper: null,
    loading: false,
  }),

  computed: {
    ready() {
      return !!this.cropper && !this.loading
    },
  },

  watch: {
    // Teardown hangs off the prop, not off the Cancel button, so it happens
    // however the dialog closes — including when the caller closes it after a
    // successful upload. A surviving Cropper instance keeps its listeners and
    // its copy of the image.
    value(open) {
      if (!open) this.teardown()
    },
  },

  beforeDestroy() {
    this.teardown()
  },

  methods: {
    ...mapActions(["showError"]),

    /** Public entry point: opens the file picker. */
    pickFile() {
      this.$refs.file?.click()
    },

    onFileChosen(event) {
      const file = event.target.files?.[0]
      // Clearing the input means picking the same file twice still fires
      // `change` — otherwise a member who cancels and re-picks gets nothing.
      event.target.value = ""
      if (!file) return

      if (!file.type.startsWith("image/")) {
        this.showError("That file isn't an image.")
        return
      }
      if (file.size > MAX_FILE_BYTES) {
        this.showError("That image is too large. Please choose one under 10MB.")
        return
      }

      const reader = new FileReader()
      reader.onload = () => this.openWith(reader.result)
      reader.onerror = () => this.showError("That image could not be read.")
      reader.readAsDataURL(file)
    },

    async openWith(dataUrl) {
      this.teardown()
      this.imageSrc = dataUrl
      this.loading = true
      this.$emit("input", true)

      // The <img> has to exist before Cropper can take it over.
      await this.$nextTick()

      try {
        // Loaded on demand: cropperjs is only ever needed on this one dialog,
        // so it gets its own chunk instead of weight on every page load.
        const [{ default: Cropper }] = await Promise.all([
          import("cropperjs"),
          import("cropperjs/dist/cropper.css"),
        ])
        this.cropper = new Cropper(this.$refs.image, {
          aspectRatio: 1,
          viewMode: 1,
          // Start with the largest square the photo allows, so the common case
          // (a photo already roughly square) needs no adjustment at all.
          autoCropArea: 1,
        })
      } catch {
        this.showError("The photo editor could not be loaded.")
        this.close()
      } finally {
        this.loading = false
      }
    },

    rotate(degrees) {
      this.cropper?.rotate(degrees)
    },

    save() {
      const canvas = this.cropper?.getCroppedCanvas({
        width: 256,
        height: 256,
        imageSmoothingQuality: "high",
      })
      if (!canvas) {
        this.showError("That crop could not be saved. Try adjusting it.")
        return
      }
      // Quality 0.85 matches what the server re-encodes at, so this is not a
      // second lossy step of a different strength.
      this.$emit("save", canvas.toDataURL("image/jpeg", 0.85))
    },

    close() {
      this.$emit("input", false)
    },

    onDialogInput(open) {
      if (!open) this.close()
    },

    teardown() {
      this.cropper?.destroy()
      this.cropper = null
      this.imageSrc = ""
    },
  },
}
</script>
