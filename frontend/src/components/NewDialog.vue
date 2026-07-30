<template>
  <v-dialog
    :value="value"
    @click:outside="handleDialogInput"
    no-click-animation
    persistent
    content-class="tw-max-w-[28rem]"
    :fullscreen="isPhone"
    scrollable
    :transition="isPhone ? `dialog-bottom-transition` : `dialog-transition`"
  >
    <UnsavedChangesDialog v-model="unsavedChangesDialog" @leave="exitDialog">
    </UnsavedChangesDialog>
    <v-card class="tw-pt-4">
      <NewEvent
        ref="form"
        :key="`event-${value}`"
        :event="event"
        :edit="edit"
        @input="handleDialogInput"
        :is-dialog-open="value"
        :contactsPayload="contactsPayload"
        :folder-id="folderId"
        @signIn="$emit('signIn')"
      />
    </v-card>
  </v-dialog>
</template>

<script>
import { isPhone } from "@/utils"
import NewEvent from "@/components/NewEvent.vue"
import UnsavedChangesDialog from "@/components/general/UnsavedChangesDialog.vue"

export default {
  name: "NewDialog",

  emits: ["input"],

  props: {
    value: { type: Boolean, required: true },
    event: { type: Object },
    edit: { type: Boolean, default: false },
    contactsPayload: { type: Object, default: () => ({}) },
    folderId: { type: String, default: null },
  },

  components: {
    NewEvent,
    UnsavedChangesDialog,
  },

  data() {
    return {
      unsavedChangesDialog: false,
    }
  },

  computed: {
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },

  methods: {
    handleDialogInput() {
      if (!this.edit || !this.$refs.form.hasEventBeenEdited()) {
        this.exitDialog()
      } else {
        this.unsavedChangesDialog = true
      }
    },
    exitDialog() {
      this.$emit("input", false)
      if (this.edit) this.$refs.form.resetToEventData()
      else this.$refs.form.reset()
    },
  },
}
</script>
