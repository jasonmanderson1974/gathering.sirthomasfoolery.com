<template>
  <span>
    <div
      class="tw-mx-auto tw-mb-24 tw-mt-4 tw-max-w-6xl tw-space-y-4 sm:tw-mb-12 sm:tw-mt-7"
    >
      <div
        v-if="loading && !eventsNotEmpty"
        class="tw-flex tw-h-[calc(100vh-10rem)] tw-w-full tw-items-center tw-justify-center"
      >
        <v-progress-circular
          indeterminate
          color="primary"
          :size="20"
          :width="2"
        ></v-progress-circular>
      </div>

      <v-fade-transition>
        <Dashboard v-if="!loading || eventsNotEmpty" />
      </v-fade-transition>

      <!-- FAB -->
      <BottomFab
        v-if="isPhone && canCreateEvents"
        id="create-event-btn"
        @click="() => _createNew()"
      >
        <v-icon>mdi-plus</v-icon>
      </BottomFab>
    </div>
  </span>
</template>

<script>
import EventType from "@/components/EventType.vue"
import BottomFab from "@/components/BottomFab.vue"
import Dashboard from "@/components/home/Dashboard.vue"
import { mapState, mapActions, mapMutations, mapGetters } from "vuex"
import { eventTypes } from "@/constants"
import { isPhone, get } from "@/utils"

export default {
  name: "Home",

  metaInfo: {
    title: "Home · The Fellowship",
  },

  components: {
    EventType,
    BottomFab,
    Dashboard,
  },

  props: {
    contactsPayload: {
      type: Object,
      default: () => ({}),
    },
  },

  data: () => ({
    loading: true,
  }),

  mounted() {
    // If coming from enabling contacts, show the dialog. Checks if contactsPayload is not an Observer.
    this.setNewDialogOptions({
      show: Object.keys(this.contactsPayload).length > 0,
      contactsPayload: this.contactsPayload,
      eventOnly: false,
    })
  },

  computed: {
    ...mapState(["events", "authUser"]),
    ...mapGetters(["canCreateEvents"]),
    eventsNotEmpty() {
      return this.events.length > 0
    },
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },

  methods: {
    ...mapMutations(["setAuthUser", "setNewDialogOptions"]),
    ...mapActions(["getEvents", "createNew"]),
    userRespondedToEvent(event) {
      return event.hasResponded ?? false
    },
    _createNew() {
      this.createNew({ eventOnly: false })
    },
    createFolder() {},
  },

  created() {
    this.getEvents().then(() => {
      this.loading = false
    })
    get("/user/profile")
      .then((authUser) => {
        this.setAuthUser(authUser)
      })
      .catch(() => {
        this.setAuthUser(null)
      })
  },
}
</script>
