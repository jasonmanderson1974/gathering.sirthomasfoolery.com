<template>
  <v-dialog :value="value" max-width="400" @input="$emit('input', $event)">
    <v-card>
      <v-card-title class="tw-break-words">{{ title }}</v-card-title>
      <v-card-text v-if="body">{{ body }}</v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn text @click="$emit('input', false)">Cancel</v-btn>
        <v-btn color="red darken-1" text @click="$emit('confirm')">
          {{ confirmLabel }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script>
/**
 * "Delete X?" — one dialog for every deletion on the event page (F18).
 *
 * Deleting a list entry, a whole list, a comment or a poll all used to happen
 * on the first click, with no way back: the entry was gone, and on a list it
 * took its sub-entries with it. This is the pause, and it NAMES what is about
 * to go rather than asking "are you sure?" about something unspecified.
 *
 * Deliberately dumb. It owns no state and knows nothing about what it is
 * deleting — the caller holds the pending target and does the work on
 * `confirm`, which is the idiom Dashboard already uses for folders. That keeps
 * the wording next to the data that produces it (see describeListDeletion and
 * describeItemDeletion in components/event/eventLists.js).
 *
 * The ad-hoc dialogs elsewhere in the app are left alone; this is not a
 * migration.
 */
export default {
  name: "ConfirmDeleteDialog",

  props: {
    value: { type: Boolean, default: false },
    // Names the target: `Delete "Menu"?`
    title: { type: String, required: true },
    // What else goes with it, when that isn't obvious. Omitted for a plain
    // entry with nothing nested under it.
    body: { type: String, default: "" },
    confirmLabel: { type: String, default: "Delete" },
  },

  emits: ["input", "confirm"],
}
</script>
