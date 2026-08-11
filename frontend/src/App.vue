<template>
  <v-app>
    <AutoSnackbar color="error" :text="error" />
    <AutoSnackbar color="tw-bg-blue" :text="info" />
    <SignInNotSupportedDialog v-model="webviewDialog" />
    <NewDialog
      v-model="newDialogOptions.show"
      :contactsPayload="newDialogOptions.contactsPayload"
      :folder-id="newDialogOptions.folderId"
    />
    <div
      v-if="showHeader"
      class="tw-fixed tw-z-40 tw-h-14 tw-w-screen tw-border-b tw-border-brass-dim tw-bg-wood-deep sm:tw-h-16"
      dark
    >
      <div
        class="tw-relative tw-m-auto tw-flex tw-h-full tw-max-w-6xl tw-items-center tw-justify-center tw-px-4"
      >
        <router-link
          :to="{ name: 'home' }"
          class="tw-flex tw-items-center tw-gap-2 tw-no-underline"
        >
          <SirThomasFoolery :size="36" />
          <span
            class="tw-font-display tw-text-base tw-font-bold tw-tracking-[0.16em] tw-text-parchment sm:tw-text-lg"
            >THE FELLOWSHIP</span
          >
        </router-link>

        <v-spacer />

        <v-btn
          v-if="$route.name === 'event' && canCreateEvents"
          id="top-right-create-btn"
          variant="text"
          class="tw-font-display tw-tracking-widest tw-text-brass"
          @click="_createNew"
        >
          Call a Gathering
        </v-btn>
        <v-btn
          v-if="$route.name === 'home' && !isPhone && canCreateEvents"
          color="primary"
          class="tw-mx-2 tw-rounded-md tw-font-display tw-tracking-widest tw-text-wood-deep"
          @click="_createNew"
        >
          + Call a Gathering
        </v-btn>
        <div v-if="authUser" class="sm:tw-ml-4">
          <AuthUserMenu />
        </div>
        <v-btn
          v-else
          id="top-right-sign-in-btn"
          variant="text"
          class="tw-font-display tw-tracking-widest tw-text-brass"
          @click="signIn"
        >
          Enter
        </v-btn>
      </div>
    </div>

    <v-main>
      <div class="tw-flex tw-h-screen tw-flex-col">
        <div
          class="tw-relative tw-flex-1 tw-overscroll-auto"
          :class="routerViewClass"
        >
          <router-view v-if="loaded" :key="$route.fullPath" />
        </div>
      </div>
    </v-main>
  </v-app>
</template>

<style>
@import url("https://fonts.googleapis.com/css2?family=DM+Sans&display=swap");

html {
  overflow-y: auto !important;
  /* overscroll-behavior: none; */
  scroll-behavior: smooth;
}

/*
 * Body typeface. This deliberately beats the EB Garamond that index.css sets on
 * `.v-application`: a rule landing directly on every element wins over an
 * inherited one regardless of specificity, so DM Sans is the body face and the
 * serif is reached for explicitly, via Tailwind's `tw-font-display`. It reads
 * like a conflict and isn't — leaving it undocumented is what made it look like
 * one.
 */
* {
  font-family: "DM Sans", sans-serif;
}

/*
 * Vuetify overrides.
 *
 * This block used to be roughly three times this size. Vuetify 3 renders a
 * completely different internal DOM, and most of what was here named classes it
 * never emits — `.v-input__slot`, `.v-input__control`, `.v-menu__content`,
 * `.v-text-field--solo`, `.v-btn--is-elevated`, `.v-size--default`,
 * `.v-input--switch__track`, `.v-input--selection-controls`,
 * `.v-text-field__details`, `.error--text`. Those rules were deleted rather
 * than translated: re-deriving a pile of `!important` declarations against an
 * internal DOM nobody has looked at yet is how you end up fighting the
 * framework for a result no one asked for. What survives is what the theme
 * actually needs, and it is re-expressed in Vuetify 3's own class names.
 *
 * If something here looks wrong, prefer a Vuetify 3 prop or a theme colour over
 * adding another `!important` — that is the trap this block is climbing out of.
 */

/* Vuetify still tracks and letter-spaces button labels; this app doesn't. */
.v-btn {
  letter-spacing: unset !important;
  text-transform: unset !important;
}

/* The squarer, slightly shorter button the rest of the layout is built around.
   v3 renames the size class: `.v-size--default` -> `.v-btn--size-default`. */
.v-btn.v-btn--size-default:not(.v-btn--icon) {
  height: 38px;
  border-radius: theme("borderRadius.md");
}

/* The brass glow on brand buttons. v3 renames `.v-btn--is-elevated` to
   `.v-btn--elevated`; the Tailwind classes it pairs with are ours and unchanged. */
.v-btn.v-btn--elevated.tw-bg-brass,
.v-btn.v-btn--elevated.tw-bg-white.tw-text-brass {
  box-shadow: 0px 2px 8px 0px #c9a44c66 !important;
  border: 1px solid theme("colors.brass-bright") !important;
}

/* Validation messages: v3 keeps `.v-messages__message` but wraps it in
   `.v-input__details` rather than `.v-text-field__details`. */
.v-messages__message {
  font-size: theme("fontSize.xs");
  line-height: 1.25;
}
.v-input__details {
  padding-inline: 0;
}

/* Ours, not Vuetify's — the availability overlay shadows. */
.overlay-avail-shadow-green {
  box-shadow: 0px 3px 6px 0px #1c7d454d !important;
}
.overlay-avail-shadow-yellow {
  box-shadow: 0px 2px 8px 0px #e5a8004d !important;
}
</style>

<script>
import { mapMutations, mapState, mapActions, mapGetters } from "vuex"
import { get, isPhone } from "@/utils"
import AutoSnackbar from "@/components/AutoSnackbar"
import AuthUserMenu from "@/components/AuthUserMenu.vue"
import SignInNotSupportedDialog from "@/components/SignInNotSupportedDialog.vue"
import isWebview from "is-ua-webview"
import NewDialog from "./components/NewDialog.vue"
import SirThomasFoolery from "@/components/general/SirThomasFoolery.vue"

export default {
  name: "App",

  head: {
    htmlAttrs: {
      lang: "en-US",
    },
  },

  components: {
    SirThomasFoolery,
    AutoSnackbar,
    AuthUserMenu,
    SignInNotSupportedDialog,
    NewDialog,
  },

  data: () => ({
    mounted: false,
    loaded: false,
    webviewDialog: false,
  }),

  computed: {
    ...mapState(["authUser", "error", "info", "newDialogOptions"]),
    ...mapGetters(["canCreateEvents"]),
    isPhone() {
      return isPhone(this.$vuetify)
    },
    showHeader() {
      return (
        this.$route.name !== "landing" &&
        this.$route.name !== "auth" &&
        this.$route.name !== "sign-in" &&
        this.$route.name !== "sign-up" &&
        this.$route.name !== "privacy-policy"
      )
    },
    routerViewClass() {
      let c = ""
      if (this.showHeader) {
        if (this.isPhone) {
          c += "tw-pt-12 "
        } else {
          c += "tw-pt-14 "
        }
      }
      return c
    },
  },

  methods: {
    ...mapMutations(["setAuthUser"]),
    ...mapActions(["getEvents", "createNew"]),
    _createNew() {
      this.createNew()
    },
    signIn() {
      // In-app webview browsers can't complete OAuth, so warn instead of
      // routing. Only reachable from the route a shared link lands on.
      if (this.$route.name === "event" && isWebview(navigator.userAgent)) {
        this.webviewDialog = true
        return
      }
      this.$router.push({ name: "sign-in" })
    },
  },

  async created() {
    await get("/user/profile")
      .then((authUser) => {
        this.setAuthUser(authUser)
      })
      .catch(() => {
        this.setAuthUser(null)
      })
      .finally(() => {
        this.loaded = true
      })

    // Event listeners

    this.getEvents()
  },

  mounted() {
    this.mounted = true
  },

  beforeUnmount() {},
}
</script>
