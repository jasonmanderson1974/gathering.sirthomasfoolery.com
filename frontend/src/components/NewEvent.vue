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
      </div>
      <v-spacer />
      <v-btn v-if="dialog" @click="$emit('input', false)" icon>
        <v-icon>mdi-close</v-icon>
      </v-btn>
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
          variant="solo"
          @keyup.enter="blurNameField"
          :rules="nameRules"
          autofocus
          required
        />

        <LocationInput
          v-model="location"
          solo
          placeholder="Where? (optional)"
        />

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
                <v-radio :model-value="false" color="primary">
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
                        :model-value="startTime"
                        @update:model-value="(t) => (startTime = t.time)"
                        :menu-props="{ auto: true }"
                        :items="times"
                        item-title="text"
                        return-object
                        hide-details
                        variant="solo"
                      ></v-select>
                      <div>to</div>
                      <v-select
                        :model-value="endTime"
                        @update:model-value="(t) => (endTime = t.time)"
                        :menu-props="{ auto: true }"
                        :items="times"
                        item-title="text"
                        return-object
                        hide-details
                        variant="solo"
                      ></v-select>
                    </div>
                  </div>
                </v-expand-transition>
                <v-radio :model-value="true" color="primary" class="tw-mt-3">
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
            variant="solo"
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
                  <v-btn variant="flat" v-show="!startOnMonday"> Sun </v-btn>
                  <v-btn variant="flat"> Mon </v-btn>
                  <v-btn variant="flat"> Tue </v-btn>
                  <v-btn variant="flat"> Wed </v-btn>
                  <v-btn variant="flat"> Thu </v-btn>
                  <v-btn variant="flat"> Fri </v-btn>
                  <v-btn variant="flat"> Sat </v-btn>
                  <v-btn variant="flat" v-show="startOnMonday"> Sun </v-btn>
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
                      <template v-slot:activator="{ props }">
                        <v-icon size="small" v-bind="props"
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
              v-model:timeIncrement="timeIncrement"
              v-model:collectEmails="collectEmails"
              v-model:blindAvailabilityEnabled="blindAvailabilityEnabled"
              v-model:sendEmailAfterXResponsesEnabled="
                sendEmailAfterXResponsesEnabled
              "
              v-model:sendEmailAfterXResponses="sendEmailAfterXResponses"
              v-model:timezone="timezone"
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

    <!-- Fades to the dialog's own surface, not the page: index.css tints
         overlay cards to #241a13. -->
    <OverflowGradient
      v-if="hasMounted"
      :scrollContainer="$refs.cardText"
      fade-to="#241a13"
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
import { post, put } from "@/utils"
import { mapActions } from "vuex"
import NewEventAdvancedOptions from "./NewEventAdvancedOptions.vue"
import EmailInput from "./event/EmailInput.vue"
import DatePicker from "@/components/DatePicker.vue"
import SlideToggle from "./SlideToggle.vue"
import LocationInput from "@/components/LocationInput.vue"
import { getAvailabilityFields } from "./availabilityModes"
import AlertText from "@/components/AlertText.vue"
import OverflowGradient from "@/components/OverflowGradient.vue"
import { isOwnerlessEvent } from "@/constants"
import ExpandableSection from "./ExpandableSection.vue"
import { newEventFormMixin } from "@/mixins/newEventForm"

export default {
  name: "NewEvent",

  emits: ["input"],

  mixins: [newEventFormMixin],

  props: {
    isDialogOpen: { type: Boolean, default: false },
  },

  components: {
    NewEventAdvancedOptions,
    EmailInput,
    DatePicker,
    SlideToggle,
    LocationInput,
    ExpandableSection,
    AlertText,
    OverflowGradient,
  },

  computed: {
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
    guestEvent() {
      return isOwnerlessEvent(this.event)
    },
  },

  methods: {
    ...mapActions(["setEventFolder"]),
    submit() {
      if (!this.$refs.form.validate()) return

      const { dates, duration, type } = this.buildDatesPayload()

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
  },

  watch: {
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
