<template>
  <div class="tw-mx-auto tw-mb-12 tw-mt-5 tw-max-w-6xl">
    <div class="tw-flex tw-flex-col tw-gap-16 tw-p-4">
      <!-- Name change section -->
      <div class="tw-flex tw-flex-col tw-gap-5">
        <div
          class="tw-text-xl tw-font-medium tw-text-parchment sm:tw-text-2xl"
        >
          Profile
        </div>
        <!-- Photo -->
        <div>
          <div class="tw-mb-1 tw-font-medium">Photo</div>
          <div class="tw-mb-3 tw-max-w-lg tw-text-sm tw-text-parchment-dim">
            Shown beside your name throughout the Fellowship. Without one, your
            initials stand in.
          </div>
          <div class="tw-flex tw-items-center tw-gap-4">
            <UserAvatarContent :user="authUser" :size="96" />
            <div class="tw-flex tw-flex-col tw-items-start tw-gap-1">
              <v-btn
                text
                class="tw-text-brass"
                :loading="savingAvatar"
                @click="$refs.avatarEditor.pickFile()"
              >
                {{ hasAvatar ? "Change photo" : "Add photo" }}
              </v-btn>
              <v-btn
                v-if="hasAvatar"
                text
                :loading="removingAvatar"
                @click="removeAvatar"
              >
                Remove photo
              </v-btn>
            </div>
          </div>
          <AvatarEditorDialog
            ref="avatarEditor"
            v-model="avatarDialog"
            :saving="savingAvatar"
            @save="saveAvatar"
          />
        </div>

        <div>
          <div class="tw-mb-1 tw-font-medium">Name</div>
          <div class="tw-flex tw-max-w-lg tw-items-center tw-gap-2">
            <v-text-field
              v-model="firstName"
              hide-details
              outlined
              placeholder="First name"
              :dense="isPhone"
            />
            <v-text-field
              v-model="lastName"
              hide-details
              outlined
              placeholder="Last name"
              :dense="isPhone"
            />
          </div>
          <v-expand-transition>
            <div v-if="profileUnsavedChanges">
              <div class="tw-mt-4">
                <v-btn
                  @click="resetProfileChanges"
                  color="primary"
                  outlined
                  class="tw-mr-2"
                  >Cancel</v-btn
                >
                <v-btn @click="saveName" color="primary">Save changes</v-btn>
              </div>
            </div>
          </v-expand-transition>
        </div>

        <!-- Nickname -->
        <div>
          <div class="tw-mb-1 tw-font-medium">Nickname</div>
          <div class="tw-mb-2 tw-max-w-lg tw-text-sm tw-text-parchment-dim">
            What the Fellowship calls you. Set one and it stands in for your
            name throughout — leave it empty to go by your name.
          </div>
          <div class="tw-flex tw-max-w-lg tw-items-center tw-gap-2">
            <v-text-field
              v-model="nickname"
              outlined
              hide-details
              placeholder="Nickname"
              :maxlength="nicknameMaxLength"
              :dense="isPhone"
            />
          </div>
          <v-expand-transition>
            <div v-if="nicknameUnsavedChanges" class="tw-mt-4">
              <v-btn
                @click="resetNickname"
                color="primary"
                outlined
                class="tw-mr-2"
                >Cancel</v-btn
              >
              <v-btn
                @click="saveNickname"
                color="primary"
                :loading="savingNickname"
                >Save changes</v-btn
              >
            </div>
          </v-expand-transition>
        </div>

        <!-- Email -->
        <div>
          <div class="tw-mb-1 tw-font-medium">Email</div>
          <div class="tw-mb-2 tw-max-w-lg tw-text-sm tw-text-parchment-dim">
            Your email is how you sign in. Changing it requires confirming a code
            sent to the new address.
          </div>
          <div v-if="emailStep === 'idle'" class="tw-max-w-lg">
            <v-text-field
              :value="authUser.email"
              outlined
              hide-details
              disabled
              class="tw-mb-2"
              :dense="isPhone"
            />
            <div class="tw-flex tw-items-start tw-gap-2">
              <v-text-field
                v-model="newEmail"
                outlined
                hide-details="auto"
                placeholder="New email address"
                type="email"
                :error-messages="newEmailError"
                :dense="isPhone"
                @keydown.enter="sendEmailCode"
              />
              <v-btn
                color="primary"
                :loading="sendingEmailCode"
                :disabled="!newEmail.trim() || sendingEmailCode"
                @click="sendEmailCode"
                >Send code</v-btn
              >
            </div>
          </div>
          <div v-else class="tw-max-w-lg">
            <div class="tw-mb-2 tw-text-sm tw-text-parchment-dim">
              Enter the code sent to
              <strong class="tw-text-parchment">{{ newEmail }}</strong>.
            </div>
            <div class="tw-flex tw-items-start tw-gap-2">
              <v-text-field
                v-model="emailCode"
                outlined
                hide-details="auto"
                placeholder="6-digit code"
                maxlength="6"
                autocomplete="one-time-code"
                :error-messages="emailCodeError"
                :dense="isPhone"
                @keydown.enter="verifyEmailCode"
              />
              <v-btn
                color="primary"
                :loading="verifyingEmail"
                :disabled="emailCode.length !== 6 || verifyingEmail"
                @click="verifyEmailCode"
                >Verify &amp; update</v-btn
              >
              <v-btn text @click="cancelEmailChange">Cancel</v-btn>
            </div>
          </div>
        </div>

        <!-- Phone -->
        <div>
          <div class="tw-mb-1 tw-font-medium">Phone</div>
          <div class="tw-flex tw-max-w-lg tw-items-center tw-gap-2">
            <v-text-field
              v-model="phone"
              outlined
              hide-details
              placeholder="Phone number"
              type="tel"
              :dense="isPhone"
              @blur="beautifyPhone"
            />
          </div>
          <v-expand-transition>
            <div v-if="phoneUnsavedChanges" class="tw-mt-4">
              <v-btn @click="resetPhone" color="primary" outlined class="tw-mr-2"
                >Cancel</v-btn
              >
              <v-btn @click="savePhone" color="primary" :loading="savingPhone"
                >Save changes</v-btn
              >
            </div>
          </v-expand-transition>
        </div>
      </div>

      <!-- Calendar Access Section -->
      <div class="tw-flex tw-flex-col tw-gap-5">
        <div
          class="tw-text-xl tw-font-medium tw-text-parchment sm:tw-text-2xl"
        >
          Calendar access
        </div>
        <div class="tw-flex tw-flex-col tw-gap-5 sm:tw-flex-row sm:tw-gap-28">
          <div class="tw-text-parchment">
            We do not store your calendar data anywhere on our servers, and we
            only fetch your calendar events for the time frame you specify in
            order to display your calendar events while you fill out your
            availability.
          </div>
          <v-btn
            outlined
            class="tw-text-red"
            href="https://myaccount.google.com/connections?filters=3,4&hl=en"
            target="_blank"
            >Revoke calendar access</v-btn
          >
        </div>
        <CalendarAccounts :skip-calendar-fetch="true"></CalendarAccounts>
      </div>

      <!-- Permissions Section -->
      <div class="tw-flex tw-flex-col tw-gap-5">
        <div
          class="tw-text-xl tw-font-medium tw-text-parchment sm:tw-text-2xl"
        >
          Permissions
        </div>
        <div
          class="tw-flex tw-flex-col tw-rounded-md tw-border-[1px] tw-border-brass-dim"
        >
          <div
            class="tw-flex tw-w-full tw-flex-row tw-border-b-[1px] tw-border-brass-dim"
          >
            <div
              v-for="(h, i) in heading"
              :key="h"
              :class="`tw-border-r-[${i == heading.length - 1 ? '0' : '1'}px]`"
              class="tw-w-1/3 tw-border-brass-dim tw-p-4 tw-font-bold"
            >
              {{ h }}
            </div>
          </div>

          <div
            v-for="(c, j) in content"
            :key="j"
            :class="`tw-border-b-[${j == content.length - 1 ? '0' : '1'}px]`"
            class="tw-flex tw-w-full tw-flex-row tw-border-brass-dim"
          >
            <div
              v-for="(text, k) in c"
              :key="k"
              :class="`tw-border-r-[${k == c.length - 1 ? '0' : '1'}px]`"
              class="tw-w-1/3 tw-border-brass-dim tw-p-4"
            >
              {{ text }}
            </div>
          </div>
        </div>
      </div>

      <!-- Question Section -->
      <div class="tw-flex tw-flex-col tw-gap-5">
        <div
          class="tw-text-xl tw-font-medium tw-text-parchment sm:tw-text-2xl"
        >
          Have a question?
        </div>
        <div class="tw-flex tw-flex-col tw-gap-5 sm:tw-flex-row sm:tw-gap-28">
          <div class="tw-text-parchment">
            Email us at
            <a
              href="mailto:sirthomasfoolery24@gmail.com"
              class="tw-text-parchment tw-underline"
              >sirthomasfoolery24@gmail.com</a
            >
            with any questions!
          </div>
        </div>
      </div>

      <!-- Delete Account Section -->
      <div class="tw-mt-28 tw-flex tw-flex-row tw-justify-center">
        <div class="tw-w-64">
          <v-dialog v-model="deleteDialog" width="400" persistent>
            <template v-slot:activator="{ on, attrs }">
              <v-btn outlined class="tw-text-red" block v-bind="attrs" v-on="on"
                >Delete account</v-btn
              >
            </template>
            <v-card>
              <v-card-title>Are you sure?</v-card-title>
              <v-card-text class="tw-text-sm tw-text-parchment-dim"
                >Are you sure you want to delete your account? All your account
                data will be lost.</v-card-text
              >
              <div class="tw-mx-6">
                <div class="tw-text-sm tw-text-parchment-dim">
                  Type your email in the box below to confirm:
                </div>
                <v-text-field
                  v-model="deleteValidateEmail"
                  autofocus
                  class="tw-flex-initial tw-text-white"
                  :placeholder="authUser.email"
                />
              </div>
              <v-card-actions>
                <v-spacer />
                <v-btn text @click="deleteDialog = false">Cancel</v-btn>
                <v-btn
                  text
                  color="error"
                  @click="deleteAccount()"
                  :disabled="authUser.email != deleteValidateEmail"
                  >Delete</v-btn
                >
              </v-card-actions>
            </v-card>
          </v-dialog>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { mapState, mapActions, mapMutations } from "vuex"
import { _delete, patch, post, put, isPhone, get, formatPhone } from "@/utils"
import CalendarAccounts from "@/components/settings/CalendarAccounts.vue"
import AvatarEditorDialog from "@/components/settings/AvatarEditorDialog.vue"
import UserAvatarContent from "@/components/UserAvatarContent.vue"

export default {
  name: "Settings",

  metaInfo: {
    title: "Settings · The Fellowship",
  },

  components: { CalendarAccounts, AvatarEditorDialog, UserAvatarContent },

  data: () => ({
    dialog: false,
    deleteDialog: false,
    deleteValidateEmail: "",
    heading: ["Permission", "Purpose", "Requested When"],
    content: [
      [
        "View all calendar events",
        "Allows us to display the names/times of your calendar events",
        "User tries to input availability automatically with Google Calendar",
      ],
      [
        "View all calendars subscribed to",
        "Allows us to display calendar events on all your calendars instead of just your primary calendar",
        "User tries to input availability automatically with Google Calendar",
      ],
    ],

    // Profile settings
    avatarDialog: false,
    savingAvatar: false,
    removingAvatar: false,
    firstName: "",
    lastName: "",
    nickname: "",
    savingNickname: false,
    // Matches nicknameMaxRunes on the server, which truncates rather than
    // rejecting — so the input stops you before the server silently would.
    nicknameMaxLength: 40,
    phone: "",
    savingPhone: false,

    // Email change flow
    newEmail: "",
    newEmailError: "",
    emailStep: "idle", // 'idle' | 'code'
    emailCode: "",
    emailCodeError: "",
    sendingEmailCode: false,
    verifyingEmail: false,
  }),

  computed: {
    ...mapState(["authUser"]),
    // avatarUpdatedAt is set on upload and unset on removal, so its presence is
    // the whole answer to "does this member have a photo".
    hasAvatar() {
      return !!this.authUser?.avatarUpdatedAt
    },
    nameUnsavedChanges() {
      return (
        this.firstName !== this.authUser.firstName ||
        this.lastName !== this.authUser.lastName
      )
    },
    profileUnsavedChanges() {
      return this.nameUnsavedChanges
    },
    nicknameUnsavedChanges() {
      return this.nickname.trim() !== (this.authUser.nickname || "")
    },
    phoneUnsavedChanges() {
      return this.phone !== (this.authUser.phone || "")
    },
    isPhone() {
      return isPhone(this.$vuetify)
    },
  },

  methods: {
    ...mapActions(["showError", "showInfo", "refreshAuthUser"]),
    ...mapMutations(["setAuthUser"]),
    resetPhone() {
      this.phone = this.authUser.phone || ""
    },
    formatPhone,
    beautifyPhone() {
      this.phone = this.formatPhone(this.phone)
    },
    // Same shape as saveNickname below: PUT → refresh → toast. The refresh is
    // what makes the new photo appear everywhere at once, since every avatar on
    // screen is keyed off authUser.avatarUpdatedAt.
    saveAvatar(image) {
      this.savingAvatar = true
      put(`/user/avatar`, { image })
        .then(async () => {
          await this.refreshAuthUser()
          this.avatarDialog = false
          this.showInfo("Photo updated.")
        })
        .catch(() => {
          this.showError("There was a problem saving your photo.")
        })
        .finally(() => {
          this.savingAvatar = false
        })
    },
    removeAvatar() {
      this.removingAvatar = true
      _delete(`/user/avatar`)
        .then(async () => {
          await this.refreshAuthUser()
          this.showInfo("Photo removed.")
        })
        .catch(() => {
          this.showError("There was a problem removing your photo.")
        })
        .finally(() => {
          this.removingAvatar = false
        })
    },
    resetNickname() {
      this.nickname = this.authUser.nickname || ""
    },
    // Saved the phone way (patch → refresh → toast), not the name way: saveName
    // reloads the whole page, which is a heavy answer to a one-field edit.
    saveNickname() {
      this.savingNickname = true
      patch(`/user/nickname`, { nickname: this.nickname.trim() })
        .then(async () => {
          await this.refreshAuthUser()
          this.nickname = this.authUser.nickname || ""
          this.showInfo("Nickname updated.")
        })
        .catch(() => {
          this.showError("There was a problem updating your nickname.")
        })
        .finally(() => {
          this.savingNickname = false
        })
    },
    savePhone() {
      this.phone = this.formatPhone(this.phone)
      this.savingPhone = true
      patch(`/user/phone`, { phone: this.phone.trim() })
        .then(async () => {
          await this.refreshAuthUser()
          this.showInfo("Phone number updated.")
        })
        .catch(() => {
          this.showError("There was a problem updating your phone number.")
        })
        .finally(() => {
          this.savingPhone = false
        })
    },
    validateNewEmail() {
      const email = this.newEmail.trim()
      if (!email) {
        this.newEmailError = "Please enter an email address."
        return false
      }
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
        this.newEmailError = "Please enter a valid email address."
        return false
      }
      if (email.includes("+")) {
        this.newEmailError = "Email aliases with '+' are not allowed."
        return false
      }
      if (email.toLowerCase() === (this.authUser.email || "").toLowerCase()) {
        this.newEmailError = "That is already your email."
        return false
      }
      return true
    },
    sendEmailCode() {
      this.newEmailError = ""
      if (!this.validateNewEmail()) return
      this.sendingEmailCode = true
      post(`/user/email/request-change`, { email: this.newEmail.trim() })
        .then(() => {
          this.emailStep = "code"
          this.emailCode = ""
          this.emailCodeError = ""
        })
        .catch((err) => {
          const code = err && err.parsed && err.parsed.error
          this.newEmailError =
            code === "email-taken"
              ? "That email already belongs to another account."
              : code === "email-unchanged"
              ? "That is already your email."
              : code === "invalid-email"
              ? "Please enter a valid email address."
              : code === "otp-rate-limited"
              ? "Too many attempts. Please wait a few minutes."
              : code === "otp-send-failed"
              ? "Could not send the code. Please try again."
              : "Something went wrong. Please try again."
        })
        .finally(() => {
          this.sendingEmailCode = false
        })
    },
    verifyEmailCode() {
      this.emailCodeError = ""
      this.verifyingEmail = true
      post(`/user/email/verify-change`, {
        email: this.newEmail.trim(),
        code: this.emailCode.trim(),
      })
        .then((user) => {
          this.setAuthUser(user)
          this.emailStep = "idle"
          this.newEmail = ""
          this.emailCode = ""
          this.showInfo("Email address updated.")
        })
        .catch((err) => {
          const code = err && err.parsed && err.parsed.error
          this.emailCodeError =
            code === "otp-expired"
              ? "Code expired. Please request a new one."
              : code === "otp-too-many-attempts"
              ? "Too many attempts. Please request a new code."
              : code === "email-taken"
              ? "That email now belongs to another account."
              : "Invalid code. Please try again."
        })
        .finally(() => {
          this.verifyingEmail = false
        })
    },
    cancelEmailChange() {
      this.emailStep = "idle"
      this.newEmail = ""
      this.emailCode = ""
      this.newEmailError = ""
      this.emailCodeError = ""
    },
    deleteAccount() {
      _delete(`/user`)
        .then(() => {
          window.location.reload()
        })
        .catch((err) => {
          this.showError(
            "There was a problem deleting your account! Please try again later."
          )
        })
    },
    resetProfileChanges() {
      this.firstName = this.authUser.firstName
      this.lastName = this.authUser.lastName
    },
    saveName() {
      patch(`/user/name`, {
        firstName: this.firstName,
        lastName: this.lastName,
      })
        .then(() => {
          window.location.reload()
        })
        .catch((err) => {
          this.showError(
            "There was a problem updating your name! Please try again later."
          )
        })
    },
  },

  created() {
    this.firstName = this.authUser.firstName
    this.lastName = this.authUser.lastName
    this.nickname = this.authUser.nickname || ""
    this.phone = this.authUser.phone || ""
  },
}
</script>
