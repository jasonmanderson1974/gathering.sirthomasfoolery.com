<template>
  <v-card
    :flat="dialog"
    :class="{ 'tw-py-4': !dialog, 'tw-flex-1': dialog }"
    class="tw-relative tw-flex tw-max-w-[28rem] tw-flex-col tw-overflow-hidden tw-rounded-lg tw-transition-all"
  >
    <v-card-title class="tw-mb-2 tw-flex tw-gap-2 tw-px-4 sm:tw-px-8">
      <div>
        <div class="tw-mb-1 tw-font-head tw-text-2xl tw-text-parchment">
          {{ edit ? "Amend the Gathering" : "Call a Gathering" }}
        </div>
        <div
          v-if="dialog && showHelp"
          class="tw-text-xs tw-font-normal tw-italic tw-text-parchment-dim"
        >
          Ideal for one-time / recurring meetings
        </div>
      </div>
      <v-spacer />
      <template v-if="dialog">
        <v-btn v-if="showHelp" icon @click="helpDialog = true">
          <v-icon>mdi-information-outline</v-icon>
        </v-btn>
        <v-btn v-else @click="$emit('input', false)" icon>
          <v-icon>mdi-close</v-icon>
        </v-btn>
        <HelpDialog v-model="helpDialog">
          <template v-slot:header>Events</template>
          <div class="tw-mb-4">
            Use events to collect people's availabilities and compare them
            across certain days.
          </div>
        </HelpDialog>
      </template>
    </v-card-title>
    <v-card-text
      ref="cardText"
      class="tw-relative tw-flex-1 tw-overflow-auto tw-px-4 tw-py-1 sm:tw-px-8"
    >
      <AlertText v-if="edit && guestEvent" class="tw-mb-4">
        This gathering was created before sign-in was required, so it has no
        owner — any member of the Fellowship can edit it.
      </AlertText>
      <v-form
        ref="form"
        v-model="formValid"
        lazy-validation
        class="tw-flex tw-flex-col tw-gap-y-6"
        :disabled="loading"
      >
        <v-text-field
          ref="name-field"
          v-model="name"
          placeholder="Name your event..."
          hide-details="auto"
          solo
          @keyup.enter="blurNameField"
          :rules="nameRules"
          autofocus
          required
        />

        <LocationInput v-model="location" solo placeholder="Where? (optional)" />

        <SlideToggle
          v-if="showAvailabilityToggle"
          class="tw-w-full"
          v-model="availabilityMode"
          :options="availabilityModeOptions"
          wrap
        />

        <div>
          <v-expand-transition>
            <div v-if="!daysOnly">
              <div class="tw-mb-2 tw-text-lg tw-text-parchment">
                What times might work?
              </div>
              <v-radio-group
                v-if="availabilityMode === availabilityModes.DATES_AND_TIMES"
                v-model="customTimes"
                class="tw-mb-2 tw-mt-0 tw-pt-0"
                hide-details
              >
                <v-radio :value="false" color="primary">
                  <template v-slot:label>
                    <span
                      class="tw-text-sm"
                      :class="
                        !customTimes
                          ? 'tw-text-parchment'
                          : 'tw-text-parchment-dim'
                      "
                    >
                      Same Times Every Day
                    </span>
                  </template>
                </v-radio>
                <v-expand-transition>
                  <div v-if="!customTimes" class="tw-ml-[32px] tw-mt-2">
                    <div
                      class="tw-flex tw-items-baseline tw-justify-center tw-space-x-2"
                    >
                      <v-select
                        :value="startTime"
                        @input="(t) => (startTime = t.time)"
                        menu-props="auto"
                        :items="times"
                        return-object
                        hide-details
                        solo
                      ></v-select>
                      <div>to</div>
                      <v-select
                        :value="endTime"
                        @input="(t) => (endTime = t.time)"
                        menu-props="auto"
                        :items="times"
                        return-object
                        hide-details
                        solo
                      ></v-select>
                    </div>
                  </div>
                </v-expand-transition>
                <v-radio :value="true" color="primary" class="tw-mt-3">
                  <template v-slot:label>
                    <span
                      class="tw-text-sm"
                      :class="
                        customTimes
                          ? 'tw-text-parchment'
                          : 'tw-text-parchment-dim'
                      "
                    >
                      Custom Times Every Day
                    </span>
                  </template>
                </v-radio>
                <v-expand-transition>
                  <div
                    v-if="customTimes"
                    class="tw-ml-[32px] tw-text-xs tw-text-parchment-dim"
                  >
                    Specify the times in the next step
                  </div>
                </v-expand-transition>
              </v-radio-group>

              <div
                v-else-if="availabilityMode === availabilityModes.TIME_BLOCKS"
                class="tw-mb-2 tw-text-xs tw-text-parchment-dim"
              >
                Recipients can only select an entire block, not partial times.
                Specify the blocks in the next step.
              </div>
            </div>
          </v-expand-transition>

          <div class="tw-mb-2 tw-text-lg tw-text-parchment">
            What
            {{ selectedDateOption === dateOptions.SPECIFIC ? "dates" : "days" }}
            might work?
          </div>
          <v-select
            v-if="!edit && !daysOnly"
            v-model="selectedDateOption"
            :items="Object.values(dateOptions)"
            solo
            hide-details
            class="tw-mb-4"
          />

          <v-expand-transition>
            <div v-if="selectedDateOption === dateOptions.SPECIFIC || daysOnly">
              <div class="tw-mb-2 tw-text-xs tw-text-parchment-dim">
                Drag to select multiple dates
              </div>
              <v-input
                v-model="selectedDays"
                hide-details="auto"
                :rules="selectedDaysRules"
                key="date-picker"
              >
                <DatePicker
                  v-model="selectedDays"
                  :minCalendarDate="minCalendarDate"
                  :startCalendarOnMonday="startOnMonday"
                />
              </v-input>
            </div>
            <div v-else-if="selectedDateOption === dateOptions.DOW">
              <v-input
                v-model="selectedDaysOfWeek"
                hide-details="auto"
                :rules="selectedDaysRules"
                key="days-of-week"
                class="tw-w-fit"
              >
                <v-btn-toggle
                  v-model="selectedDaysOfWeek"
                  multiple
                  solo
                  color="primary"
                >
                  <v-btn depressed v-show="!startOnMonday"> Sun </v-btn>
                  <v-btn depressed> Mon </v-btn>
                  <v-btn depressed> Tue </v-btn>
                  <v-btn depressed> Wed </v-btn>
                  <v-btn depressed> Thu </v-btn>
                  <v-btn depressed> Fri </v-btn>
                  <v-btn depressed> Sat </v-btn>
                  <v-btn depressed v-show="startOnMonday"> Sun </v-btn>
                </v-btn-toggle>
              </v-input>
              <v-checkbox class="tw-mt-2" v-model="startOnMonday" hide-details>
                <template v-slot:label>
                  <span class="tw-text-sm tw-text-parchment-dim">
                    Start on Monday
                  </span>
                </template>
              </v-checkbox>
            </div>
          </v-expand-transition>
        </div>

        <v-checkbox
          v-if="!guestEvent && authUser"
          v-model="notificationsEnabled"
          hide-details
          class="tw-mt-2"
        >
          <template v-slot:label>
            <span class="tw-text-sm tw-text-parchment-dim"
              >Email me each time someone joins my event</span
            >
          </template>
        </v-checkbox>
        <v-checkbox
          v-else-if="!guestEvent"
          disabled
          messages="test"
          off-icon="mdi-checkbox-blank-off-outline"
          class="tw-mt-2"
        >
          <template v-slot:label>
            <span class="tw-text-sm"
              >Email me each time someone joins my event</span
            >
          </template>
          <template v-slot:message>
            <div
              class="tw-pointer-events-auto -tw-mt-1 tw-ml-[32px] tw-text-xs tw-text-parchment-dim"
            >
              <span class="tw-font-medium tw-text-parchment-dim"
                ><a @click="$emit('signIn')">Sign in</a>
                to use this feature
              </span>
            </div>
          </template>
        </v-checkbox>

        <div class="tw-flex tw-flex-col tw-gap-2">
          <ExpandableSection
            v-if="authUser && !guestEvent"
            label="Email reminders"
            v-model="showEmailReminders"
            :auto-scroll="dialog"
          >
            <div class="tw-flex tw-flex-col tw-gap-5 tw-pt-2">
              <EmailInput
                v-show="authUser"
                ref="emailInput"
                @requestContactsAccess="requestContactsAccess"
                labelColor="tw-text-parchment-dim"
                :addedEmails="addedEmails"
                @update:emails="(newEmails) => (emails = newEmails)"
              >
                <template v-slot:header>
                  <div class="tw-flex tw-gap-1">
                    <div class="tw-text-parchment-dim">
                      Remind people to fill out the event
                    </div>

                    <v-tooltip
                      top
                      content-class="tw-bg-very-dark-gray tw-shadow-lg tw-opacity-100 tw-py-4"
                    >
                      <template v-slot:activator="{ on, attrs }">
                        <v-icon small v-bind="attrs" v-on="on"
                          >mdi-information-outline
                        </v-icon>
                      </template>
                      <div>
                        Reminder emails will be sent the day of event
                        creation,<br />one day after, and three days after. You
                        will also receive <br />an email when everybody has
                        filled out the event.
                      </div>
                    </v-tooltip>
                  </div>
                </template>
              </EmailInput>
            </div>
          </ExpandableSection>

          <ExpandableSection
            v-model="showAdvancedOptions"
            label="Advanced options"
            :auto-scroll="dialog"
          >
            <NewEventAdvancedOptions
              :edit="edit"
              :guestEvent="guestEvent"
              :timeIncrement.sync="timeIncrement"
              :collectEmails.sync="collectEmails"
              :blindAvailabilityEnabled.sync="blindAvailabilityEnabled"
              :sendEmailAfterXResponsesEnabled.sync="
                sendEmailAfterXResponsesEnabled
              "
              :sendEmailAfterXResponses.sync="sendEmailAfterXResponses"
              :timezone.sync="timezone"
              @signIn="$emit('signIn')"
            />
          </ExpandableSection>
        </div>
      </v-form>
    </v-card-text>
    <v-card-actions class="tw-relative tw-px-4 sm:tw-px-8">
      <div class="tw-relative tw-w-full">
        <v-btn
          :disabled="!formValid"
          block
          :loading="loading"
          color="primary"
          class="tw-mt-4 tw-text-wood-deep"
          @click="submit"
        >
          {{
            specificTimesEnabled
              ? "Next"
              : edit
              ? "Save edits"
              : "Call a Gathering"
          }}
        </v-btn>
        <div
          :class="formValid ? 'tw-invisible' : 'tw-visible'"
          class="tw-mt-1 tw-text-xs tw-text-red"
        >
          Please fix form errors before continuing
        </div>
      </div>
    </v-card-actions>

    <OverflowGradient
      v-if="hasMounted"
      :scrollContainer="$refs.cardText"
      class="tw-bottom-[90px]"
    />
  </v-card>
</template>

<style>
.email-me-after-text-field input {
  padding: 0px !important;
}
</style>

<script>
import { eventTypes, dayIndexToDayString, authTypes } from "@/constants"
import {
  post,
  put,
  timeNumToTimeString,
  dateToTimeNum,
  getISODateString,
  isPhone,
  signInGoogle,
  getDateWithTimezone,
  getTimeOptions,
  prefersStartOnMonday,
} from "@/utils"
import { mapActions, mapState } from "vuex"
import NewEventAdvancedOptions from "./NewEventAdvancedOptions.vue"
import HelpDialog from "./HelpDialog.vue"
import EmailInput from "./event/EmailInput.vue"
import DatePicker from "@/components/DatePicker.vue"
import SlideToggle from "./SlideToggle.vue"
import LocationInput from "@/components/LocationInput.vue"
import {
  availabilityModes,
  getAvailabilityFields,
  getAvailabilityMode,
} from "./availabilityModes"
import AlertText from "@/components/AlertText.vue"
import OverflowGradient from "@/components/OverflowGradient.vue"
import { isOwnerlessEvent } from "@/constants"
import dayjs from "dayjs"
import utcPlugin from "dayjs/plugin/utc"
import timezonePlugin from "dayjs/plugin/timezone"
import ExpandableSection from "./ExpandableSection.vue"
dayjs.extend(utcPlugin)
dayjs.extend(timezonePlugin)

export default {
  name: "NewEvent",

  emits: ["input"],

  props: {
    event: { type: Object },
    edit: { type: Boolean, default: false },
    dialog: { type: Boolean, default: true },
    contactsPayload: { type: Object, default: () => ({}) },
    showHelp: { type: Boolean, default: false },
    folderId: { type: String, default: null },
    isDialogOpen: { type: Boolean, default: false },
  },

  components: {
    NewEventAdvancedOptions,
    HelpDialog,
    EmailInput,
    DatePicker,
    SlideToggle,
    LocationInput,
    ExpandableSection,
    AlertText,
    OverflowGradient,
  },

  data: () => ({
    formValid: true,
    name: "",
    startTime: 9,
    endTime: 17,
    loading: false,
    selectedDays: [],
    selectedDaysOfWeek: [],
    startOnMonday: prefersStartOnMonday(),
    notificationsEnabled: true,

    // Primary availability mode. `daysOnly`, `specificTimesEnabled` and
    // `wholeBlockSelection` (the fields the API actually takes) are derived
    // from this and `customTimes` — see the computed properties below.
    availabilityModes,
    availabilityMode: availabilityModes.DATES_AND_TIMES,

    // Only meaningful in DATES_AND_TIMES mode:
    // false = "Same Times Every Day", true = "Custom Times Every Day"
    customTimes: false,

    // Date options
    dateOptions: Object.freeze({
      SPECIFIC: "Specific dates",
      DOW: "Days of the week",
    }),
    selectedDateOption: "Specific dates",

    // Email reminders
    showEmailReminders: false,
    emails: [], // For email reminders

    // Advanced options
    showAdvancedOptions: false,
    timeIncrement: 15,
    collectEmails: false,
    blindAvailabilityEnabled: false,
    timezone: {},
    location: "",
    sendEmailAfterXResponsesEnabled: false,
    sendEmailAfterXResponses: 3,

    helpDialog: false,

    // Unsaved changes
    initialEventData: {},

    hasMounted: false,
  }),

  mounted() {
    if (Object.keys(this.contactsPayload).length > 0) {
      this.toggleEmailReminders(true)

      /** Get previously filled out data after enabling contacts  */
      this.name = this.contactsPayload.name
      this.startTime = this.contactsPayload.startTime
      this.endTime = this.contactsPayload.endTime
      this.availabilityMode =
        this.contactsPayload.availabilityMode ??
        this.availabilityModes.DATES_AND_TIMES
      this.customTimes = this.contactsPayload.customTimes ?? false
      this.selectedDateOption = this.contactsPayload.selectedDateOption
      this.selectedDaysOfWeek = this.contactsPayload.selectedDaysOfWeek
      this.selectedDays = this.contactsPayload.selectedDays
      this.notificationsEnabled = this.contactsPayload.notificationsEnabled
      this.timezone = this.contactsPayload.timezone
      this.location = this.contactsPayload.location ?? ""

      this.$refs.form.resetValidation()
    }

    this.$nextTick(() => {
      this.hasMounted = true
    })
  },

  computed: {
    ...mapState(["authUser", "daysOnlyEnabled"]),
    /** The options shown in the primary availability toggle */
    availabilityModeOptions() {
      const options = [
        {
          text: "Dates and times",
          value: this.availabilityModes.DATES_AND_TIMES,
        },
        {
          text: "Dates w/ time blocks",
          value: this.availabilityModes.TIME_BLOCKS,
        },
      ]

      // Switching an existing event into or out of dates-only would change what
      // its dates mean, so that option is only offered when creating
      if (this.daysOnlyEnabled && !this.edit) {
        options.push({
          text: "Dates only",
          value: this.availabilityModes.DATES_ONLY,
        })
      }

      return options
    },
    showAvailabilityToggle() {
      // A dates-only event being edited has no mode left to switch to
      return !(this.edit && this.event?.daysOnly)
    },
    availabilityFields() {
      return getAvailabilityFields(this.availabilityMode, this.customTimes)
    },
    daysOnly() {
      return this.availabilityFields.daysOnly
    },
    wholeBlockSelection() {
      return this.availabilityFields.wholeBlockSelection
    },
    specificTimesEnabled() {
      return this.availabilityFields.hasSpecificTimes
    },
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
    isPhone() {
      return isPhone(this.$vuetify)
    },
    guestEvent() {
      return isOwnerlessEvent(this.event)
    },
  },

  methods: {
    ...mapActions(["showError", "setEventFolder"]),
    blurNameField() {
      this.$refs["name-field"].blur()
    },
    reset() {
      this.name = ""
      this.startTime = 9
      this.endTime = 17
      this.availabilityMode = this.availabilityModes.DATES_AND_TIMES
      this.customTimes = false
      this.selectedDays = []
      this.selectedDaysOfWeek = []
      this.notificationsEnabled = true
      this.selectedDateOption = "Specific dates"
      this.emails = []
      this.showAdvancedOptions = false
      this.blindAvailabilityEnabled = false
      this.sendEmailAfterXResponsesEnabled = false
      this.sendEmailAfterXResponses = 3
      this.collectEmails = false
      this.location = ""
      this.startOnMonday = prefersStartOnMonday()

      this.$refs.form.resetValidation()
    },
    submit() {
      if (!this.$refs.form.validate()) return

      this.selectedDays.sort()

      // Get duration of event
      let duration = this.endTime - this.startTime
      if (duration <= 0) duration += 24

      // Get date objects for each selected day
      let dates = []
      let type = ""
      if (this.daysOnly) {
        duration = 0
        type = eventTypes.SPECIFIC_DATES

        for (const day of this.selectedDays) {
          const date = new Date(`${day} 00:00:00Z`)
          dates.push(date)
        }
      } else {
        const startTimeString = timeNumToTimeString(this.startTime)
        if (this.selectedDateOption === this.dateOptions.SPECIFIC) {
          type = eventTypes.SPECIFIC_DATES

          for (const day of this.selectedDays) {
            const date = dayjs.tz(
              `${day} ${startTimeString}`,
              this.timezone.value
            )
            dates.push(date.toDate())
          }
        } else if (this.selectedDateOption === this.dateOptions.DOW) {
          type = eventTypes.DOW

          this.selectedDaysOfWeek.sort((a, b) => a - b)
          this.selectedDaysOfWeek = this.selectedDaysOfWeek.filter(
            (dayIndex) => {
              return this.startOnMonday ? dayIndex !== 0 : dayIndex !== 7
            }
          )
          for (const dayIndex of this.selectedDaysOfWeek) {
            const day = dayIndexToDayString[dayIndex]
            const date = dayjs.tz(
              `${day} ${startTimeString}`,
              this.timezone.value
            )

            // The reference dates (dayIndexToDayString) are from June 2018, which may have
            // a different DST offset than the current date. Adjust so the stored UTC time
            // corresponds to the user's current timezone offset.
            const refOffset = date.utcOffset()
            const currentOffset = dayjs().tz(this.timezone.value).utcOffset()
            dates.push(
              date.subtract(currentOffset - refOffset, "minutes").toDate()
            )
          }
        }
      }

      this.loading = true

      const payload = {
        name: this.name,
        duration: duration,
        dates: dates,
        hasSpecificTimes: this.specificTimesEnabled,
        wholeBlockSelection: this.wholeBlockSelection,
        notificationsEnabled: !this.authUser
          ? false
          : this.notificationsEnabled,
        blindAvailabilityEnabled: this.blindAvailabilityEnabled,
        daysOnly: this.daysOnly,
        remindees: this.emails,
        type: type,
        sendEmailAfterXResponses: this.sendEmailAfterXResponsesEnabled
          ? parseInt(this.sendEmailAfterXResponses)
          : -1,
        collectEmails: this.collectEmails,
        startOnMonday: this.startOnMonday,
        timeIncrement: this.timeIncrement,
        location: this.location.trim(),
      }


      if (!this.edit) {
        // Create new event on backend
        post("/events", payload)
          .then(async ({ eventId, shortId }) => {
            if (this.authUser) {
              await this.setEventFolder({ eventId, folderId: this.folderId })
            }
            this.$router.push({
              name: "event",
              params: {
                eventId: shortId ?? eventId,
                initialTimezone: this.timezone,
              },
            })

            this.$emit("input", false)
            this.reset()
          })
          .catch((err) => {
            this.showError(
              "There was a problem creating that event! Please try again later."
            )
            console.error(err)
          })
          .finally(() => {
            this.loading = false
          })
      } else {
        // Edit event on backend
        if (this.event) {
          put(`/events/${this.event._id}`, payload)
            .then(() => {
              // this.$emit("input", false)
              // this.reset()
              localStorage.setItem(`from-edit-event-${this.event._id}`, "true")
              window.location.reload()
            })
            .catch(() => {
              this.showError(
                "There was a problem editing this event! Please try again later."
              )
            })
            .finally(() => {
              this.loading = false
            })
        }
      }
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
      const payload = {
        emails,
        name: this.name,
        startTime: this.startTime,
        endTime: this.endTime,
        availabilityMode: this.availabilityMode,
        customTimes: this.customTimes,
        selectedDays: this.selectedDays,
        selectedDaysOfWeek: this.selectedDaysOfWeek,
        selectedDateOption: this.selectedDateOption,
        notificationsEnabled: this.notificationsEnabled,
        timezone: this.timezone,
        location: this.location,
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
    /** Update state based on the contactsPayload after granting contacts access */
    contactsAccessGranted({ curScheduledEvent, ...data }) {
      this.curScheduledEvent = curScheduledEvent
      this.$refs.confirmDetailsDialog?.setData(data)
      this.confirmDetailsDialog = true
    },

    /** Populates the form fields based on this.event */
    updateFieldsFromEvent() {
      if (this.event) {
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
          const selectedDays = []
          for (let date of this.event.dates) {
            selectedDays.push(getISODateString(date, true))
          }
          this.selectedDays = selectedDays
        } else {
          if (this.event.type === eventTypes.SPECIFIC_DATES) {
            this.selectedDateOption = this.dateOptions.SPECIFIC
            const selectedDays = []
            for (let date of this.event.dates) {
              date = getDateWithTimezone(date)

              selectedDays.push(getISODateString(date, true))
            }
            this.selectedDays = selectedDays
          } else if (this.event.type === eventTypes.DOW) {
            this.selectedDateOption = this.dateOptions.DOW
            const selectedDaysOfWeek = []
            for (let date of this.event.dates) {
              date = getDateWithTimezone(date)

              if (this.event.startOnMonday && date.getUTCDay() === 0) {
                selectedDaysOfWeek.push(7)
              } else {
                selectedDaysOfWeek.push(date.getUTCDay())
              }
            }
            this.selectedDaysOfWeek = selectedDaysOfWeek
            if (this.event.startOnMonday) {
              this.startOnMonday = true
            }
          }
        }
      }
    },
    resetToEventData() {
      this.updateFieldsFromEvent()
      this.$refs.emailInput.reset()
    },
    setInitialEventData() {
      this.initialEventData = {
        name: this.name,
        startTime: this.startTime,
        endTime: this.endTime,
        availabilityMode: this.availabilityMode,
        customTimes: this.customTimes,
        selectedDays: this.selectedDays,
        selectedDaysOfWeek: this.selectedDaysOfWeek,
        selectedDateOption: this.selectedDateOption,
        notificationsEnabled: this.notificationsEnabled,
        emails: [...this.emails],
        blindAvailabilityEnabled: this.blindAvailabilityEnabled,
        sendEmailAfterXResponsesEnabled: this.sendEmailAfterXResponsesEnabled,
        sendEmailAfterXResponses: this.sendEmailAfterXResponses,
        timeIncrement: this.timeIncrement,
        location: this.location,
      }
    },
    hasEventBeenEdited() {
      return (
        this.name !== this.initialEventData.name ||
        this.startTime !== this.initialEventData.startTime ||
        this.endTime !== this.initialEventData.endTime ||
        this.availabilityMode !== this.initialEventData.availabilityMode ||
        this.customTimes !== this.initialEventData.customTimes ||
        this.selectedDateOption !== this.initialEventData.selectedDateOption ||
        JSON.stringify(this.selectedDays) !==
          JSON.stringify(this.initialEventData.selectedDays) ||
        JSON.stringify(this.selectedDaysOfWeek) !==
          JSON.stringify(this.initialEventData.selectedDaysOfWeek) ||
        this.notificationsEnabled !==
          this.initialEventData.notificationsEnabled ||
        JSON.stringify(this.emails) !==
          JSON.stringify(this.initialEventData.emails) ||
        this.blindAvailabilityEnabled !==
          this.initialEventData.blindAvailabilityEnabled ||
        this.sendEmailAfterXResponsesEnabled !==
          this.initialEventData.sendEmailAfterXResponsesEnabled ||
        this.sendEmailAfterXResponses !==
          this.initialEventData.sendEmailAfterXResponses ||
        this.location !== this.initialEventData.location
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
    startOnMonday() {
      localStorage.setItem("startCalendarOnMonday", this.startOnMonday)
    },
    isDialogOpen(newVal) {
      if (newVal) {
        this.reset()
      }
    },
  },
}
</script>
