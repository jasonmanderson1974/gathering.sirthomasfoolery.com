<template>
  <!-- Google sign in not supported dialog -->
  <v-dialog :model-value="modelValue" width="400" content-class="tw-m-0">
    <v-card>
      <v-card-title>Google sign in not supported</v-card-title>
      <v-card-text>
        <p>
          This usually happens when you open a Fellowship link from Instagram or
          Messenger.
        </p>
        <p>
          To get around this issue, click the three dots in the top right and
          then "Open in system browser" or "Open in Chrome"
        </p>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn
          text
          class="tw-text-brass"
          @click="$emit('update:modelValue', false)"
        >
          Ok
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script>
export default {
  name: "SignInNotSupportedDialog",

  props: {
    modelValue: { type: Boolean, required: true },
  },
}
</script>
