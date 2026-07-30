import { mapActions, mapState } from "vuex"
import {
  dateToTimeNum,
  getDateWithTimezone,
  getISODateString,
  getTimeOptions,
  signInGoogle,
} from "@/utils"
import { authTypes, eventTypes } from "@/constants"
import { prefersStartOnMonday } from "@/utils"
import {
  availabilityModes,
  getAvailabilityMode,
} from "@/components/availabilityModes"
import { buildEventDates, dateOptions } from "@/components/newEventDates"

/**
 * The form fields `reset()` restores, and the values it restores them to. A
 * function, not a constant, because `startOnMonday` reads `localStorage` and
 * the arrays must be fresh per instance.
 *
 * Deliberately not everything in `data()`: `timezone`, `timeIncrement` and
 * `showEmailReminders` outlive a reset, as do the dialog and loading flags.
 */
const formDefaults = () => ({
  name: "",
  startTime: 9,
  endTime: 17,
  selectedDays: [],
  selectedDaysOfWeek: [],
  startOnMonday: prefersStartOnMonday(),
  notificationsEnabled: true,
  availabilityMode: availabilityModes.DATES_AND_TIMES,
  // Only meaningful in DATES_AND_TIMES mode:
  // false = "Same Times Every Day", true = "Custom Times Every Day"
  customTimes: false,
  selectedDateOption: dateOptions.SPECIFIC,
  emails: [],
  showAdvancedOptions: false,
  collectEmails: false,
  blindAvailabilityEnabled: false,
  sendEmailAfterXResponsesEnabled: false,
  sendEmailAfterXResponses: 3,
  location: "",
})

/**
 * The fields carried through the Google-contacts OAuth round-trip: written into
 * the state handed to `signInGoogle`, read back out in `mounted`.
 */
const contactsFields = Object.freeze([
  "name",
  "startTime",
  "endTime",
  "availabilityMode",
  "selectedDateOption",
  "selectedDaysOfWeek",
  "selectedDays",
  "notificationsEnabled",
  "timezone",
  "customTimes",
  "location",
])

/**
 * The fields snapshotted on open and compared on close to decide whether the
 * edit dialog should warn about unsaved changes.
 */
const trackedFields = Object.freeze([
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
  "customTimes",
  "timeIncrement",
  "location",
])

const snapshot = (value) => (Array.isArray(value) ? [...value] : value)

/**
 * The non-template half of the "Call a Gathering" form (G2): the field state,
 * the Google-contacts OAuth round-trip, the populate-from-event pass, the
 * unsaved-changes check and `reset()`.
 *
 * It was extracted when NewEvent and NewSignUp held near-identical copies of
 * all of it, and was a factory over per-component defaults for that reason.
 * NewSignUp is gone, so the generality went with it — but the split stays,
 * because it is what keeps NewEvent.vue near 500 lines instead of 900.
 *
 * What NewEvent keeps is the template, the availability-mode derivation (it
 * owns `daysOnly`, which `buildDatesPayload` below reads), and `submit()` — the
 * date arithmetic that used to be duplicated lives in `newEventDates.js`.
 */
export const newEventFormMixin = {
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

    // Outside formDefaults: `reset()` deliberately leaves it alone
    timeIncrement: 15,

    // Unsaved changes
    initialEventData: {},

    hasMounted: false,
  }),

  mounted() {
    if (Object.keys(this.contactsPayload).length > 0) {
      this.toggleEmailReminders(true)

      /** Get previously filled out data after enabling contacts */
      const defaults = formDefaults()
      for (const field of contactsFields) {
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
      for (const field of contactsFields) {
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
      const { mode, customTimes } = getAvailabilityMode(this.event)
      this.availabilityMode = mode
      this.customTimes = customTimes
      this.startOnMonday = this.event.startOnMonday
      this.collectEmails = this.event.collectEmails
      this.timeIncrement = this.event.timeIncrement ?? 15
      this.location = this.event.location ?? ""

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
    },

    resetToEventData() {
      this.updateFieldsFromEvent()
      // Absent when the reminders section is hidden — an ownerless event, or
      // a form that has no email input at all
      this.$refs.emailInput?.reset()
    },

    setInitialEventData() {
      this.initialEventData = Object.fromEntries(
        trackedFields.map((field) => [field, snapshot(this[field])])
      )
    },

    hasEventBeenEdited() {
      return trackedFields.some(
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
