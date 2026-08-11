<template>
  <router-link
    :to="{
      name: 'event',
      params: { eventId: event.shortId ?? event._id },
    }"
  >
    <v-container
      v-ripple
      class="tw-flex tw-min-h-16 tw-items-center tw-justify-between tw-rounded-lg tw-bg-leather tw-px-4 tw-py-2.5 tw-text-parchment tw-drop-shadow tw-transition-all hover:tw-drop-shadow-md sm:tw-py-3"
      :data-ph-capture-attribute-event-id="event._id"
      :data-ph-capture-attribute-event-name="event.name"
    >
      <div class="tw-flex tw-items-center">
        <div
          class="tw-flex tw-size-10 tw-shrink-0 tw-items-center tw-justify-center tw-rounded"
          :class="{
            'tw-bg-green-felt': isOwner,
            'tw-bg-leather': !isOwner,
          }"
        >
          <v-icon :color="isOwner ? 'green' : 'grey'">{{
            isDow
              ? "mdi-calendar-range"
              : event.daysOnly
              ? "mdi-calendar-month"
              : "mdi-calendar"
          }}</v-icon>
        </div>
        <div class="tw-ml-3">
          <div>{{ event.name }}</div>
          <div class="tw-text-sm tw-font-light tw-text-parchment-dim">
            {{ dateString }}
          </div>
        </div>
      </div>
      <div class="tw-min-w-max">
        <v-chip
          size="small"
          class="tw-m-0.5 tw-bg-leather tw-text-parchment-dim"
        >
          <v-icon left size="small"> mdi-account-multiple </v-icon>
          {{ event.numResponses }}
        </v-chip>
        <v-menu
          v-if="isOwner"
          v-model="showMenu"
          ref="menu"
          :close-on-content-click="false"
          transition="slide-x-transition"
          right
        >
          <template v-slot:activator="{ props }">
            <v-btn variant="plain" icon v-bind="props" @click.prevent>
              <v-icon>mdi-dots-vertical</v-icon>
            </v-btn>
          </template>

          <v-list class="tw-py-1" density="compact">
            <v-list-item @click="copyLink">
              <v-list-item-title>Copy link</v-list-item-title>
            </v-list-item>
            <v-divider />
            <v-dialog v-model="duplicateDialog" width="400" persistent>
              <template v-slot:activator="{ props }">
                <v-list-item id="duplicate-event-btn" v-bind="props">
                  <v-list-item-title>Duplicate</v-list-item-title>
                </v-list-item>
              </template>
              <v-card>
                <v-card-title>Duplicate {{ typeText }}</v-card-title>
                <v-card-text>
                  <v-text-field
                    v-model="duplicateDialogOptions.name"
                    required
                    placeholder="Name your event..."
                    :disabled="duplicateDialogOptions.loading"
                    hide-details
                    variant="solo"
                  />
                  <v-checkbox
                    v-model="duplicateDialogOptions.copyAvailability"
                    label="Copy responses"
                    :disabled="duplicateDialogOptions.loading"
                    hide-details
                    class="tw-mt-2"
                  />
                </v-card-text>
                <v-card-actions>
                  <v-spacer />
                  <v-btn
                    variant="text"
                    @click="duplicateDialog = false"
                    :disabled="duplicateDialogOptions.loading"
                    >Cancel</v-btn
                  >
                  <v-btn
                    variant="text"
                    color="primary"
                    @click="duplicateEvent"
                    :loading="duplicateDialogOptions.loading"
                    >Confirm</v-btn
                  >
                </v-card-actions>
              </v-card>
            </v-dialog>
            <v-menu
              v-if="isOwner"
              right
              :close-on-content-click="false"
              open-on-hover
            >
              <template v-slot:activator="{ props: menuProps }">
                <v-list-item
                  v-bind="menuProps"
                  class="tw-cursor-pointer tw-pr-1 hover:tw-bg-leather"
                >
                  <v-list-item-title>Move to</v-list-item-title>
                  <!-- v-list-item-icon / -action / -content are all gone in
                       Vuetify 3; trailing adornments go in the `append` slot. -->
                  <template #append>
                    <v-icon size="small">mdi-chevron-right</v-icon>
                  </template>
                </v-list-item>
              </template>
              <v-list density="compact" class="tw-py-1">
                <v-list-item @click="moveEventToFolder(null)" class="tw-pr-1">
                  <v-list-item-title>No folder</v-list-item-title>
                  <template v-if="folderId === null" #append>
                    <v-icon size="small">mdi-check</v-icon>
                  </template>
                </v-list-item>
                <v-list-item
                  v-for="folder in folders"
                  :key="folder._id"
                  @click="moveEventToFolder(folder._id)"
                  class="tw-pr-1"
                >
                  <v-list-item-title>{{ folder.name }}</v-list-item-title>
                  <template v-if="folder._id === folderId" #append>
                    <v-icon size="small">mdi-check</v-icon>
                  </template>
                </v-list-item>
              </v-list>
            </v-menu>
            <v-divider />
            <v-list-item @click="_archiveEvent">
              <v-list-item-title>{{
                event.isArchived ? "Unarchive" : "Archive"
              }}</v-list-item-title>
            </v-list-item>
            <v-dialog v-model="removeDialog" width="400" persistent>
              <template v-slot:activator="{ props }">
                <v-list-item
                  id="delete-event-btn"
                  class="tw-text-red"
                  v-bind="props"
                >
                  <v-list-item-title>Delete {{ typeText }}</v-list-item-title>
                </v-list-item>
              </template>
              <v-card>
                <v-card-title>Are you sure?</v-card-title>
                <v-card-text
                  >Are you sure you want to delete this
                  {{ typeText }}?</v-card-text
                >
                <v-card-actions>
                  <v-spacer />
                  <v-btn variant="text" @click="removeDialog = false"
                    >Cancel</v-btn
                  >
                  <v-btn variant="text" color="error" @click="removeEvent"
                    >I'm sure</v-btn
                  >
                </v-card-actions>
              </v-card>
            </v-dialog>
          </v-list>
        </v-menu>
        <v-icon v-else class="tw-ml-2 tw-mr-1 tw-opacity-75"
          >mdi-chevron-right</v-icon
        >
      </div>
    </v-container>
  </router-link>
</template>

<script>
import { getDateRangeStringForEvent, _delete, isPhone, post } from "@/utils"
import { mapActions, mapState } from "vuex"
import { eventTypes } from "@/constants"

export default {
  name: "EventItem",

  props: {
    event: { type: Object, required: true },
    folderId: { type: String, default: null },
  },

  data: () => ({
    showMenu: false,
    duplicateDialog: false,
    duplicateDialogOptions: {
      name: "",
      copyAvailability: false,
      loading: false,
    },
    removeDialog: false,
  }),

  computed: {
    ...mapState(["authUser", "folders"]),
    dateString() {
      return getDateRangeStringForEvent(this.event)
    },
    isOwner() {
      return this.event.ownerId === this.authUser._id
    },
    isDow() {
      return this.event.type === eventTypes.DOW
    },
    typeText() {
      return "event"
    },
  },

  methods: {
    ...mapActions([
      "showError",
      "showInfo",
      "getEvents",
      "setEventFolder",
      "archiveEvent",
      "refreshAuthUser",
    ]),
    _archiveEvent() {
      this.archiveEvent({
        eventId: this.event._id,
        archive: !this.event.isArchived,
      })
    },
    moveEventToFolder(folderId) {
      this.setEventFolder({
        eventId: this.event._id,
        folderId: folderId,
      })
      this.showMenu = false
    },
    copyLink() {
      /* Copies event link to clipboard */
      navigator.clipboard.writeText(
        `${window.location.origin}/e/${this.event.shortId ?? this.event._id}`
      )
      this.showInfo("Link copied to clipboard!")
      this.showMenu = false
    },
    isPhone() {
      return isPhone(this.$vuetify)
    },
    removeEvent() {
      _delete(`/events/${this.event._id}`)
        .then(() => {
          this.refreshAuthUser()
          this.getEvents()
          this.$refs.menu.save() // NOTE: Not sure why but without this line, the menu persists to the next event.
        })
        .catch(() => {
          this.showError(
            "There was a problem removing that event! Please try again later."
          )
        })
    },
    duplicateEvent() {
      this.duplicateDialogOptions.loading = true
      post(`/events/${this.event._id}/duplicate`, {
        eventName: this.duplicateDialogOptions.name,
        copyAvailability: this.duplicateDialogOptions.copyAvailability,
      })
        .then(() => {
          this.getEvents()
          this.$refs.menu.save() // NOTE: Not sure why but without this line, the menu persists to the next event.
        })
        .catch(() => {
          this.showError(
            "There was a problem duplicating that event! Please try again later."
          )
        })
        .finally(() => {
          this.duplicateDialogOptions.loading = false
        })
    },
  },

  watch: {
    duplicateDialog: {
      immediate: true,
      handler(val) {
        if (val) {
          this.duplicateDialogOptions.name = `Copy of ${this.event.name}`
        }
      },
    },
  },
}
</script>
