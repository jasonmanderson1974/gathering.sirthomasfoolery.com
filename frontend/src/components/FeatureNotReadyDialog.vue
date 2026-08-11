<template>
  <v-dialog v-model="dialog" max-width="400px" content-class="tw-m-0">
    <v-card>
      <v-card-title>
        <span class="tw-text-xl tw-font-medium">Oops! Feature Not Ready</span>
        <v-spacer />
        <v-btn
          @click="dialog = false"
          icon
          class="tw-absolute tw-right-0 tw-mr-2 tw-self-center"
        >
          <v-icon>mdi-close</v-icon>
        </v-btn>
      </v-card-title>
      <v-card-text class="tw-text-parchment-dim">
        You've caught us a bit early! We're considering adding folders to the
        Fellowship and will do so once we get enough demand from users.
        <v-textarea
          v-model="folderUsageFeedback"
          label="What would you like to use folders for?"
          rows="3"
          class="tw-mt-4"
          variant="outlined"
          density="compact"
        ></v-textarea>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn variant="text" @click="dialog = false">Close</v-btn>
        <v-btn color="primary" @click="submitFeedback">Submit</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script>
import { mapActions } from "vuex"

export default {
  name: "FeatureNotReadyDialog",
  props: {
    modelValue: Boolean,
  },

  emits: ["update:modelValue"],
  data() {
    return {
      folderUsageFeedback: "",
    }
  },
  computed: {
    dialog: {
      get() {
        return this.modelValue
      },
      set(val) {
        this.$emit("update:modelValue", val)
      },
    },
  },
  methods: {
    ...mapActions(["showInfo"]),
    submitFeedback() {
      if (this.folderUsageFeedback.trim() !== "") {
        this.folderUsageFeedback = ""
        this.dialog = false
        this.showInfo("Thanks for your input!")
      }
    },
  },
}
</script>

<style scoped></style>
