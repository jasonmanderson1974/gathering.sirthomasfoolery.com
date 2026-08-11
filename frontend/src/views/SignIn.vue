<template>
  <div
    class="tw-flex tw-min-h-screen tw-items-center tw-justify-center tw-bg-transparent tw-px-4"
  >
    <div class="tw-w-full tw-max-w-[420px]">
      <!-- Crest -->
      <div class="tw-mb-8 tw-flex tw-justify-center">
        <router-link
          :to="{ name: 'landing' }"
          class="tw-flex tw-flex-col tw-items-center tw-gap-2 tw-no-underline"
        >
          <SirThomasFoolery :size="72" />
          <span
            class="tw-font-display tw-text-lg tw-font-bold tw-tracking-[0.16em] tw-text-parchment"
            >THE FELLOWSHIP</span
          >
        </router-link>
      </div>

      <v-card class="tw-rounded-xl tw-px-2 tw-py-4">
        <!-- Main sign-in screen -->
        <template v-if="step === 'select'">
          <v-card-title class="tw-flex tw-flex-col tw-items-center tw-pb-0">
            <div class="tw-font-head tw-text-3xl tw-text-parchment">
              {{ isSignUp ? "Join the Fellowship" : "Welcome Back, Good Sir" }}
            </div>
            <div class="tw-mt-1 tw-text-sm tw-font-normal tw-text-parchment-dim">
              {{
                isSignUp ? "Establish your standing" : "Resume your standing"
              }}
            </div>
          </v-card-title>
          <v-card-text class="tw-flex tw-flex-col tw-items-center tw-pt-6">
            <div class="tw-mb-4 tw-flex tw-w-full tw-flex-col tw-gap-y-2">
              <div>
                <div class="tw-mb-1 tw-text-sm tw-font-medium">
                  Email address
                </div>
                <v-text-field
                  v-model="email"
                  class="tw-mb-2"
                  placeholder="Enter your email..."
                  type="email"
                  solo
                  hide-details="auto"
                  :error-messages="emailError"
                  @keydown.enter="submitEmail"
                />
                <v-btn
                  block
                  color="primary"
                  :loading="sending"
                  :disabled="sending"
                  @click="submitEmail"
                >
                  {{ isSignUp ? "Join the Fellowship" : "Continue" }}
                </v-btn>
              </div>
            </div>
            <div class="tw-text-center tw-text-xs">
              By continuing, you agree to our
              <router-link
                class="tw-text-brass"
                :to="{ name: 'privacy-policy' }"
              >
                privacy policy
              </router-link>
            </div>
          </v-card-text>
        </template>

        <!-- Onboarding: name entry for new users -->
        <template v-else-if="step === 'onboarding'">
          <v-card-title class="tw-flex tw-items-center">
            <v-btn icon small @click="step = 'select'" class="tw-mr-1">
              <v-icon>mdi-arrow-left</v-icon>
            </v-btn>
            What's your name?
          </v-card-title>
          <v-card-text>
            <p class="tw-mb-4 tw-text-sm tw-text-parchment-dim">
              We just need a couple details to set up your account.
            </p>
            <div class="tw-mb-1 tw-text-sm tw-font-medium">First name</div>
            <v-text-field
              v-model="firstName"
              placeholder="First name"
              solo
              hide-details="auto"
              autofocus
              @keydown.enter="
                $refs.lastNameField && $refs.lastNameField.focus()
              "
              class="tw-mb-3"
            />
            <div class="tw-mb-1 tw-text-sm tw-font-medium">Last name</div>
            <v-text-field
              ref="lastNameField"
              v-model="lastName"
              placeholder="Last name (optional)"
              solo
              hide-details="auto"
              @keydown.enter="submitOnboarding"
              class="tw-mb-3"
            />
            <div class="tw-mb-1 tw-text-sm tw-font-medium">Email</div>
            <v-text-field
              :value="email"
              placeholder="Email..."
              solo
              hide-details="auto"
              disabled
              background-color="#2e2117"
              class="tw-mb-3"
            />
            <v-btn
              block
              color="primary"
              :loading="sending"
              :disabled="!firstName.trim() || sending"
              @click="submitOnboarding"
            >
              Continue
            </v-btn>
            <p
              v-if="otpError"
              class="tw-mt-3 tw-text-sm tw-text-oxblood"
            >
              {{ otpError }}
            </p>
          </v-card-text>
        </template>

        <!-- OTP code input -->
        <template v-else-if="step === 'otp'">
          <v-card-title class="tw-flex tw-items-center">
            <v-btn
              icon
              small
              @click="step = isNewUser ? 'onboarding' : 'select'"
              class="tw-mr-1"
            >
              <v-icon>mdi-arrow-left</v-icon>
            </v-btn>
            Enter verification code
          </v-card-title>
          <v-card-text>
            <p class="tw-mb-4 tw-text-sm tw-text-parchment-dim">
              Enter the 6-digit code sent to
              <strong>{{ email }}</strong>
            </p>
            <div class="tw-mb-1 tw-text-sm tw-font-medium">
              Verification code
            </div>
            <v-text-field
              v-model="otpCode"
              placeholder="Enter 6-digit code..."
              solo
              hide-details="auto"
              maxlength="6"
              autocomplete="one-time-code"
              :error-messages="otpError"
              @keydown.enter="verifyOtp"
              autofocus
              class="tw-mb-2"
            />
            <v-btn
              block
              color="primary"
              :loading="verifying"
              :disabled="otpCode.length !== 6 || verifying"
              @click="verifyOtp"
            >
              Verify
            </v-btn>
            <div class="tw-mt-3 tw-text-center">
              <v-btn
                text
                x-small
                :disabled="resendCooldown > 0"
                @click="resendOtp"
              >
                {{
                  resendCooldown > 0
                    ? `Resend code (${resendCooldown}s)`
                    : "Resend code"
                }}
              </v-btn>
            </div>
          </v-card-text>
        </template>
      </v-card>

      <div
        class="tw-mt-4 tw-rounded-xl tw-border tw-border-brass-dim tw-bg-leather/60 tw-py-4 tw-text-center tw-text-sm tw-text-parchment-dim"
      >
        <template v-if="isSignUp">
          Already a member?
          <router-link
            class="tw-font-medium tw-text-brass"
            :to="{ name: 'sign-in', query: $route.query }"
            >Enter</router-link
          >
        </template>
        <template v-else>
          Not yet a member?
          <router-link
            class="tw-font-medium tw-text-brass"
            :to="{ name: 'sign-up', query: $route.query }"
            >Seek admittance</router-link
          >
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import { post } from "@/utils"
import { isSafeRedirect } from "@/router"
import { mapMutations } from "vuex"
import SirThomasFoolery from "@/components/general/SirThomasFoolery.vue"

export default {
  name: "SignIn",

  props: {
    initialIsSignUp: { type: Boolean, default: false },
  },

  metaInfo() {
    return {
      title: this.isSignUp
        ? "Join · The Fellowship"
        : "Enter · The Fellowship",
    }
  },

  components: {
    SirThomasFoolery,
  },

  data() {
    return {
      isSignUp: this.initialIsSignUp,
      step: "select",
      email: "",
      firstName: "",
      lastName: "",
      otpCode: "",
      emailError: "",
      otpError: "",
      sending: false,
      verifying: false,
      isNewUser: false,
      resendCooldown: 0,
      resendTimer: null,
    }
  },

  methods: {
    ...mapMutations(["setAuthUser"]),
    validateEmail() {
      const email = this.email.trim()
      if (!email) {
        this.emailError = "Please enter an email address."
        return false
      }
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
        this.emailError = "Please enter a valid email address."
        return false
      }
      if (email.includes("+")) {
        this.emailError = "Email aliases with '+' are not allowed."
        return false
      }
      return true
    },
    async submitEmail() {
      if (this.sending) return
      this.emailError = ""
      if (!this.validateEmail()) return
      this.sending = true
      try {
        const res = await post("/auth/otp/check-email", { email: this.email })
        if (res.invited === false) {
          this.emailError =
            "The Fellowship is by invitation only — this address isn't on the roll. Speak to a member to be added."
          return
        }
        this.isNewUser = res.isNewUser
        if (this.isNewUser) {
          this.step = "onboarding"
        } else {
          await this.sendOtpEmail()
          this.step = "otp"
          this.otpCode = ""
          this.otpError = ""
        }
      } catch (err) {
        const code = err && err.parsed && err.parsed.error
        this.emailError =
          code === "otp-send-failed" || code === "otp-rate-limited"
            ? this.otpSendErrorMessage(err)
            : "Something went wrong. Please try again."
      } finally {
        this.sending = false
      }
    },
    async submitOnboarding() {
      if (!this.firstName.trim() || this.sending) return
      this.sending = true
      try {
        await this.sendOtpEmail()
        this.step = "otp"
        this.otpCode = ""
        this.otpError = ""
      } catch (err) {
        this.otpError = this.otpSendErrorMessage(err)
      } finally {
        this.sending = false
      }
    },
    async sendOtpEmail() {
      await post("/auth/otp/send", { email: this.email })
      this.startResendCooldown()
    },
    // Themed message for a failed code send. Distinguishes a delivery failure
    // (mail relay) and a rate limit from a generic error.
    otpSendErrorMessage(err) {
      const code = err && err.parsed && err.parsed.error
      if (code === "otp-rate-limited") {
        return "Too many requests. Kindly wait a few minutes before trying again."
      }
      if (code === "otp-send-failed") {
        return "The code could not be dispatched — the post has failed us. Try again shortly, or send word to a member if it persists."
      }
      return "Failed to send code. Please try again."
    },
    async resendOtp() {
      if (this.sending || this.resendCooldown > 0) return
      this.sending = true
      try {
        await this.sendOtpEmail()
        this.otpCode = ""
        this.otpError = ""
      } catch (err) {
        this.otpError = this.otpSendErrorMessage(err)
      } finally {
        this.sending = false
      }
    },
    async verifyOtp() {
      if (this.otpCode.length !== 6 || this.verifying) return
      this.otpError = ""
      this.verifying = true
      try {
        const body = {
          email: this.email,
          code: this.otpCode,
          timezoneOffset: new Date().getTimezoneOffset(),
        }
        if (this.isNewUser) {
          body.firstName = this.firstName.trim()
          body.lastName = this.lastName.trim()
        }
        const user = await post("/auth/otp/verify", body)
        this.setAuthUser(user)
        await this.handlePostAuthRedirect()
      } catch (err) {
        const errorCode = err?.parsed?.error
        if (errorCode === "otp-expired") {
          this.otpError = "Code has expired. Please request a new one."
        } else if (errorCode === "otp-too-many-attempts") {
          this.otpError = "Too many attempts. Please request a new code."
        } else {
          this.otpError = "Invalid code. Please try again."
        }
      } finally {
        this.verifying = false
      }
    },
    async handlePostAuthRedirect() {
      // The router guard parks the intended destination here when it bounces an
      // unauthenticated visitor, so a shared /e/:id link round-trips through
      // login back to the event instead of dumping them on the dashboard.
      const redirect = this.$route.query.redirect
      if (isSafeRedirect(redirect)) {
        this.$router.replace(redirect)
        return
      }
      this.$router.replace({ name: "home" })
    },
    startResendCooldown() {
      this.resendCooldown = 30
      if (this.resendTimer) clearInterval(this.resendTimer)
      this.resendTimer = setInterval(() => {
        this.resendCooldown--
        if (this.resendCooldown <= 0) {
          clearInterval(this.resendTimer)
          this.resendTimer = null
        }
      }, 1000)
    },
  },

  created() {
    // Bounced here from a Google sign-in that wasn't on the invite roll
    if (this.$route.query.notInvited) {
      this.emailError =
        "The Fellowship is by invitation only — that account isn't on the roll. Speak to a member to be added."
    }
  },

  beforeUnmount() {
    if (this.resendTimer) clearInterval(this.resendTimer)
  },
}
</script>
