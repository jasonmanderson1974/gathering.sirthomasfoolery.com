<!-- Displays auth user's avatar, which displays a menu when clicked -->
<template>
  <span>
    <v-menu v-if="authUser" offset-y left>
      <template v-slot:activator="{ on }">
        <v-btn id="user-menu-btn" icon :width="size" :height="size" v-on="on">
          <v-avatar :size="size">
            <UserAvatarContent :user="authUser" :size="size" />
          </v-avatar>
        </v-btn>
      </template>
      <v-list class="py-0" :dense="isPhone">
        <v-list-item>
          <v-list-item-title>
            <strong>{{ displayName(authUser) }}</strong>
          </v-list-item-title>
        </v-list-item>
        <!-- <v-list-item id="add-team-member-btn" @click="addTeamMember">
          <v-list-item-title class="tw-flex tw-items-center tw-gap-1">
            <v-icon class="tw-mr-1" small color="black"
              >mdi-account-plus</v-icon
            >
            Add team member
          </v-list-item-title>
        </v-list-item> -->
        <v-list-item
          v-if="canInvite"
          id="fellowship-btn"
          @click="goToFellowship"
        >
          <v-list-item-title class="tw-flex tw-items-center tw-gap-1">
            <v-icon class="tw-mr-1" small color="black"
              >mdi-book-account-outline</v-icon
            >
            The Fellowship
          </v-list-item-title>
        </v-list-item>
        <v-list-item
          v-if="canInvite"
          id="chronicle-btn"
          @click="goToChronicle"
        >
          <v-list-item-title class="tw-flex tw-items-center tw-gap-1">
            <v-icon class="tw-mr-1" small color="black">mdi-book-open-page-variant-outline</v-icon>
            The Chronicle
          </v-list-item-title>
        </v-list-item>
        <v-list-item
          v-if="canInvite"
          id="members-btn"
          @click="goToAdmin"
        >
          <v-list-item-title class="tw-flex tw-items-center tw-gap-1">
            <v-icon class="tw-mr-1" small color="black">mdi-account-group</v-icon>
            The Roll
          </v-list-item-title>
        </v-list-item>
        <v-list-item id="settings-btn" @click="goToSettings">
          <v-list-item-title class="tw-flex tw-items-center tw-gap-1">
            <v-icon class="tw-mr-1" small color="black">mdi-cog</v-icon>
            Settings
          </v-list-item-title>
        </v-list-item>
        <v-divider></v-divider>
        <v-list-item id="sign-out-btn" @click="signOut">
          <v-list-item-title class="red--text tw-flex tw-items-center tw-gap-1">
            <v-icon class="tw-mr-1" small color="red">mdi-logout</v-icon>
            Sign Out
          </v-list-item-title>
        </v-list-item>
      </v-list>
    </v-menu>
    <TeamsNotReadyDialog v-model="showTeamsNotReadyDialog" />
  </span>
</template>

<script>
import UserAvatarContent from "@/components/UserAvatarContent"
import { mapState, mapMutations, mapGetters } from "vuex"
import { post, isPhone, displayName } from "@/utils"
import TeamsNotReadyDialog from "./TeamsNotReadyDialog.vue"

export default {
  name: "AuthUserMenu",

  components: {
    UserAvatarContent,
    TeamsNotReadyDialog,
  },

  data() {
    return {
      showTeamsNotReadyDialog: false,
    }
  },

  computed: {
    ...mapState(["authUser"]),
    ...mapGetters(["canInvite"]),
    isPhone() {
      return isPhone(this.$vuetify)
    },
    size() {
      return this.isPhone ? 32 : 42
    },
  },

  methods: {
    ...mapMutations(["setAuthUser"]),
    displayName,
    async signOut() {
      await post("/auth/sign-out")
      this.setAuthUser(null)
      location.reload()
    },
    goToSettings() {
      this.$router.replace({ name: "settings" })
    },
    goToFellowship() {
      this.$router.replace({ name: "fellowship" })
    },
    goToChronicle() {
      this.$router.replace({ name: "chronicle" })
    },
    goToAdmin() {
      this.$router.replace({ name: "admin" })
    },
    addTeamMember() {
      this.showTeamsNotReadyDialog = true
    },
  },
}
</script>
