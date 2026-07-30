import { mapActions, mapState } from "vuex"
import {
  dateToTimeNum,
  getDateWithTimezone,
  getISODateString,
  getTimeOptions,
  signInGoogle,
} from "@/utils"
import { authTypes, eventTypes } from "@/constants"
import {
  availabilityModes,
  getAvailabilityMode,
} from "@/components/availabilityModes"
import { buildEventDates, dateOptions } from "@/components/newEventDates"

/**
 * The form fields `reset()` restores, and the values it restores them to.
 *
 * Deliberately not everything in `data()`: `timezone`, `timeIncrement` and
 * `showEmailReminders` outlive a reset, as do the dialog and loading flags.
 */
const sharedFormDefaults = () => ({
  name: "",
  startTime: 9,
  endTime: 17,
  selectedDays: [],
  selectedDaysOfWeek: [],
  startOnMonday: false,
  notificationsEnabled: false,
  availabilityMode: availabilityModes.DATES_AND_TIMES,
  selectedDateOption: dateOptions.SPECIFIC,
  emails: [],
  showAdvancedOptions: false,
  collectEmails: false,
  blindAvailabilityEnabled: false,
  sendEmailAfterXResponsesEnabled: false,
  sendEmailAfterXResponses: 3,
})

/**
 * The fields carried through the Google-contacts OAuth round-trip: written into
 * the state handed to `signInGoogle`, read back out in `mounted`.
 */
export const sharedContactsFields = Object.freeze([
  "name",
  "startTime",
  "endTime",
  "availabilityMode",
  "selectedDateOption",
  "selectedDaysOfWeek",
  "selectedDays",
  "notificationsEnabled",
  "timezone",
])

/**
 * The fields snapshotted on open and compared on close to decide whether the
 * edit dialog should warn about unsaved changes.
 */
export const sharedTrackedFields = Object.freeze([
  "name",
  "startTime",
  "endTime",
  "availabilityMode",
  "selectedDays",
  "selectedDaysOfWeek",
  "selectedDateOption",
  "notificationsEnabled",
  "emails",
  "blindAvailabilityEnabled",
  "sendEmailAfterXResponsesEnabled",
  "sendEmailAfterXResponses",
])

const snapshot = (value) => (Array.isArray(value) ? [...value] : value)

/**
 * Everything NewEvent and NewSignUp share (G2). The two are the same form over
 * the same event document — one collects availability, the other collects sign
 * ups — and before this they held near-identical copies of the props, the field
 * state, the contacts round-trip, the populate-from-event pass and the
 * unsaved-changes check.
 *
 * A factory rather than a plain mixin because the two disagree on three form
 * defaults, and the defaults are needed twice: to seed `data()` and to restore
 * in `reset()`. It takes a function, not an object, so `startOnMonday` can read
 * `localStorage` per instance rather than once at import.
 *
 * What stays in the components is what genuinely differs: the templates, the
 * availability-mode derivation (only NewEvent offers custom times or time
 * blocks), and `submit()` — same date arithmetic, which is why that part lives
 * in `newEventDates.js`, but different payload, route and follow-up.
 *
 * Two hooks a component may define:
 *  - `updateExtraFieldsFromEvent()` — populate its own fields from `this.event`
 *  - `contactsFields` / `trackedFields` — override to extend the shared lists
 *
 * @param {() => object} defaultOverrides - form defaults this component differs on
 */
export const newEventFormMixin = (defaultOverrides = () => ({})) => {
  const formDefaults = () => ({
    ...sharedFormDefaults(),
    ...defaultOverrides(),
  })

  return {
    props: {
      event: { type: Object },
      edit: { type: Boolean, default: false },
      dialog: { type: Boolean, default: true },
      contactsPayload: { type: Object, default: () => ({}) },
      /** The folder the dialog was opened from; null when created at top level */
      folderId: { type: String, default: null },
    },

    data: () => ({
      ...formDefaults(),

      formValid: true,
      loading: false,

      availabilityModes,
      dateOptions,

      // Email reminders
      showEmailReminders: false,

      timezone: {},

      // Unsaved changes
      initialEventData: {},

      hasMounted: false,
    }),

    mounted() {
      if (Object.keys(this.contactsPayload).length > 0) {
        this.toggleEmailReminders(true)

        /** Get previously filled out data after enabling contacts */
        const defaults = formDefaults()
        for (const field of this.contactsFields) {
          this[field] = this.contactsPayload[field] ?? defaults[field]
        }

        this.$refs.form.resetValidation()
      }

      this.$nextTick(() => {
        this.hasMounted = true
      })
    },

    computed: {
      ...mapState(["authUser", "daysOnlyEnabled"]),

      contactsFields: () => sharedContactsFields,
      trackedFields: () => sharedTrackedFields,

      nameRules() {
        return [(v) => !!v || "Event name is required"]
      },
      selectedDaysRules() {
        return [
          (selectedDays) =>
            selectedDays.length > 0 || "Please select at least one day",
        ]
      },
      addedEmails() {
        if (Object.keys(this.contactsPayload).length > 0)
          return this.contactsPayload.emails
        return this.event && this.event.remindees
          ? this.event.remindees.map((r) => r.email)
          : []
      },
      times() {
        return getTimeOptions()
      },
      minCalendarDate() {
        if (this.edit) {
          return ""
        }

        let today = new Date()
        let dd = String(today.getDate()).padStart(2, "0")
        let mm = String(today.getMonth() + 1).padStart(2, "0")
        let yyyy = today.getFullYear()

        return yyyy + "-" + mm + "-" + dd
      },
    },

    methods: {
      ...mapActions(["showError"]),

      blurNameField() {
        this.$refs["name-field"].blur()
      },

      reset() {
        Object.assign(this, formDefaults())
        this.$refs.form.resetValidation()
      },

      /**
       * Turns the current selections into the `dates`, `duration` and `type` the
       * API takes, writing back the day-of-week list the toggle group is bound
       * to (`buildEventDates` drops whichever wrap-around Sunday is hidden).
       */
      buildDatesPayload() {
        this.selectedDays.sort()

        const { dates, duration, type, selectedDaysOfWeek } = buildEventDates({
          daysOnly: this.daysOnly,
          selectedDays: this.selectedDays,
          selectedDaysOfWeek: this.selectedDaysOfWeek,
          selectedDateOption: this.selectedDateOption,
          startTime: this.startTime,
          endTime: this.endTime,
          startOnMonday: this.startOnMonday,
          timezone: this.timezone,
        })

        this.selectedDaysOfWeek = selectedDaysOfWeek

        return { dates, duration, type }
      },

      toggleEmailReminders(delayed = false) {
        if (delayed) {
          setTimeout(
            () => (this.showEmailReminders = !this.showEmailReminders),
            300
          )
        } else {
          this.showEmailReminders = !this.showEmailReminders
        }
      },

      /** Redirects user to oauth page requesting access to the user's contacts */
      requestContactsAccess({ emails }) {
        const payload = { emails }
        for (const field of this.contactsFields) {
          payload[field] = this[field]
        }

        signInGoogle({
          state: {
            type: authTypes.EVENT_CONTACTS,
            eventId: this.event ? this.event.shortId ?? this.event._id : "",
            payload,
          },
          requestContactsPermission: true,
        })
      },

      /** Populates the form fields based on this.event */
      updateFieldsFromEvent() {
        if (!this.event) return

        this.name = this.event.name

        // Set start time, accounting for the timezone
        this.startTime = Math.floor(
          dateToTimeNum(getDateWithTimezone(this.event.dates[0]), true)
        )
        this.startTime %= 24

        this.endTime = (this.startTime + this.event.duration) % 24
        this.notificationsEnabled = this.event.notificationsEnabled
        this.blindAvailabilityEnabled = this.event.blindAvailabilityEnabled
        this.availabilityMode = getAvailabilityMode(this.event).mode

        if (
          this.event.sendEmailAfterXResponses !== null &&
          this.event.sendEmailAfterXResponses > 0
        ) {
          this.sendEmailAfterXResponsesEnabled = true
          this.sendEmailAfterXResponses = this.event.sendEmailAfterXResponses
        }

        if (this.event.daysOnly) {
          this.selectedDateOption = this.dateOptions.SPECIFIC
          this.selectedDays = this.event.dates.map((date) =>
            getISODateString(date, true)
          )
        } else if (this.event.type === eventTypes.SPECIFIC_DATES) {
          this.selectedDateOption = this.dateOptions.SPECIFIC
          this.selectedDays = this.event.dates.map((date) =>
            getISODateString(getDateWithTimezone(date), true)
          )
        } else if (this.event.type === eventTypes.DOW) {
          this.selectedDateOption = this.dateOptions.DOW
          this.selectedDaysOfWeek = this.event.dates.map((date) => {
            const day = getDateWithTimezone(date).getUTCDay()
            // The toggle group shows a trailing Sunday instead of a leading one
            return this.event.startOnMonday && day === 0 ? 7 : day
          })
          if (this.event.startOnMonday) {
            this.startOnMonday = true
          }
        }

        this.updateExtraFieldsFromEvent?.()
      },

      resetToEventData() {
        this.updateFieldsFromEvent()
        // Absent when the reminders section is hidden — an ownerless event, or
        // a form that has no email input at all
        this.$refs.emailInput?.reset()
      },

      setInitialEventData() {
        this.initialEventData = Object.fromEntries(
          this.trackedFields.map((field) => [field, snapshot(this[field])])
        )
      },

      hasEventBeenEdited() {
        return this.trackedFields.some(
          (field) =>
            JSON.stringify(this[field]) !==
            JSON.stringify(this.initialEventData[field])
        )
      },
    },

    watch: {
      event: {
        immediate: true,
        handler() {
          this.updateFieldsFromEvent()
          this.setInitialEventData()
        },
      },
      selectedDateOption() {
        // Reset the other date / day selection when date option is changed
        if (this.selectedDateOption === this.dateOptions.SPECIFIC) {
          this.selectedDaysOfWeek = []
        } else if (this.selectedDateOption === this.dateOptions.DOW) {
          this.selectedDays = []
        }
      },
    },
  }
}
