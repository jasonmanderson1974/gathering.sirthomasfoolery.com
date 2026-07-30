<template>
  <div></div>
</template>

<script>
import { get, post } from "@/utils"
import { mapMutations, mapState } from "vuex"
import { authTypes, calendarTypes } from "@/constants"

export default {
  name: "Auth",

  computed: {
    ...mapState(["authUser"]),
  },

  methods: {
    ...mapMutations(["setAuthUser"]),
  },

  async created() {
    let { error, code, scope, state } = this.$route.query
    if (error) this.$router.replace({ name: "home" })

    if (state) state = JSON.parse(decodeURIComponent(state))

    // Sign in and set auth user
    try {
      if (
        state?.type === authTypes.ADD_CALENDAR_ACCOUNT ||
        state?.type === authTypes.ADD_CALENDAR_ACCOUNT_FROM_EDIT
      ) {
        if (state.calendarType === calendarTypes.GOOGLE) {
          await post("/user/add-google-calendar-account", { code, scope })
        } else if (state.calendarType === calendarTypes.OUTLOOK) {
          await post("/user/add-outlook-calendar-account", {
            code,
            scope: state.scope,
          })
        } else {
          throw new Error("Invalid calendar type")
        }
      } else {
        const user = await post("/auth/sign-in", {
          code,
          scope: scope ?? state.scope,
          calendarType: state.calendarType,
          timezoneOffset: new Date().getTimezoneOffset(),
        })

        this.setAuthUser(user)
      }

      // Redirect to the correct place based on "state", otherwise, just redirect to home
      if (state) {
        let authUser
        switch (state.type) {
          case authTypes.EVENT_ADD_AVAILABILITY:
            this.$router.replace({
              name: "event",
              params: { eventId: state.eventId, fromSignIn: true },
            })
            break
          case authTypes.EVENT_SIGN_IN:
            this.$router.replace({
              name: "event",
              params: { eventId: state.eventId },
            })
            break
          case authTypes.EVENT_SIGN_IN_LINK_APPLE:
            this.$router.replace({
              name: "event",
              params: { eventId: state.eventId, linkApple: true },
            })
            break
          case authTypes.ADD_CALENDAR_ACCOUNT:
            this.$router.replace({
              name: "settings",
            })
            authUser = await get("/user/profile")
            this.setAuthUser(authUser)
            break
          case authTypes.ADD_CALENDAR_ACCOUNT_FROM_EDIT:
            this.$router.replace({
              name: "event",
              params: { eventId: state.eventId, fromSignIn: true },
            })
            authUser = await get("/user/profile")
            this.setAuthUser(authUser)
            break
          case authTypes.EVENT_CONTACTS:
            if (state.eventId == "") {
              this.$router.replace({
                name: "home",
                params: {
                  contactsPayload: state.payload,
                },
              })
            } else {
              this.$router.replace({
                name: "event",
                params: {
                  eventId: state.eventId,
                  contactsPayload: state.payload,
                },
              })
            }
            break
          default:
            this.$router.replace({ name: "home" })
        }
      } else {
        this.$router.replace({ name: "home" })
      }
    } catch (err) {
      console.error(err)
      if (err?.parsed?.error === "not-invited") {
        this.$router.replace({ name: "sign-in", query: { notInvited: "1" } })
      }
    }
  },
}
</script>
