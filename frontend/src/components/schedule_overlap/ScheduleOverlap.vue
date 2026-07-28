<template>
  <span>
    <Tooltip :content="tooltipContent">
      <div class="tw-select-none tw-py-4" style="-webkit-touch-callout: none">
        <div class="tw-flex tw-flex-col sm:tw-flex-row">
          <div
            class="tw-flex tw-grow tw-pl-4"
            :class="isSignUp ? '' : 'tw-pr-4'"
          >
            <template v-if="event.daysOnly">
              <div class="tw-grow">
                <div class="tw-flex tw-items-center tw-justify-between">
                  <v-btn
                    :class="hasPrevPage ? 'tw-visible' : 'tw-invisible'"
                    class="tw-border-gray"
                    outlined
                    icon
                    @click="prevPage"
                    ><v-icon>mdi-chevron-left</v-icon></v-btn
                  >
                  <div
                    class="tw-text-lg tw-font-medium tw-capitalize sm:tw-text-xl"
                  >
                    {{ curMonthText }}
                  </div>
                  <v-btn
                    :class="hasNextPage ? 'tw-visible' : 'tw-invisible'"
                    class="tw-border-gray"
                    outlined
                    icon
                    @click="nextPage"
                    ><v-icon>mdi-chevron-right</v-icon></v-btn
                  >
                </div>
                <!-- Header -->
                <div class="tw-flex tw-w-full">
                  <div
                    v-for="day in daysOfWeek"
                    :key="day"
                    class="tw-flex-1 tw-p-2 tw-text-center tw-text-base tw-capitalize tw-text-parchment-dim"
                  >
                    {{ day }}
                  </div>
                </div>
                <!-- Days grid -->
                <div class="tw-relative">
                  <div
                    id="drag-section"
                    class="tw-grid tw-grid-cols-7"
                    @mouseleave="resetCurTimeslot"
                  >
                    <div
                      v-for="(day, i) in monthDays"
                      :key="day.time"
                      class="timeslot tw-flex tw-aspect-square tw-flex-col tw-items-center tw-justify-center tw-text-sm sm:tw-text-base"
                      :class="dayTimeslotClassStyle[i].class"
                      :style="dayTimeslotClassStyle[i].style"
                      v-on="dayTimeslotVon[i]"
                    >
                      <span>{{ day.date }}</span>
                      <span
                        v-if="dayTimeslotCounts[i]"
                        class="timeslot-count timeslot-count--day tw-leading-none"
                        :class="dayTimeslotCounts[i].class"
                        >{{ dayTimeslotCounts[i].text }}</span
                      >
                    </div>
                  </div>
                  <ZigZag
                    v-if="hasPrevPage"
                    left
                    class="tw-absolute tw-left-0 tw-top-0 tw-h-full tw-w-3"
                  />
                  <ZigZag
                    v-if="hasNextPage"
                    right
                    class="tw-absolute tw-right-0 tw-top-0 tw-h-full tw-w-3"
                  />
                </div>

                <v-expand-transition>
                  <div
                    :key="hintText"
                    v-if="!isPhone && hintTextShown"
                    class="tw-sticky tw-bottom-4 tw-z-10 tw-flex"
                  >
                    <div
                      class="tw-mt-2 tw-flex tw-w-full tw-items-center tw-justify-between tw-gap-1 tw-rounded-md tw-bg-leather tw-p-2 tw-px-[7px] tw-text-sm tw-text-parchment-dim"
                    >
                      <div class="tw-flex tw-items-center tw-gap-1">
                        <v-icon small>mdi-information-outline</v-icon>
                        {{ hintText }}
                      </div>
                      <v-icon small @click="closeHint">mdi-close</v-icon>
                    </div>
                  </div>
                </v-expand-transition>

                <ToolRow
                  v-if="!isPhone && !calendarOnly"
                  :event="event"
                  :state="state"
                  :states="states"
                  :cur-timezone.sync="curTimezone"
                  :timezone-reference-date="timezoneReferenceDate"
                  :show-best-times.sync="showBestTimes"
                  :hide-if-needed.sync="hideIfNeeded"
                  :is-weekly="isWeekly"
                  :calendar-permission-granted="calendarPermissionGranted"
                  :week-offset="weekOffset"
                  :num-responses="respondents.length"
                  :mobile-num-days.sync="mobileNumDays"
                  :allow-schedule-event="allowScheduleEvent"
                  :show-event-options="showEventOptions"
                  :time-type.sync="timeType"
                  @toggleShowEventOptions="toggleShowEventOptions"
                  @update:weekOffset="(val) => $emit('update:weekOffset', val)"
                  @scheduleEvent="scheduleEvent"
                  @cancelScheduleEvent="cancelScheduleEvent"
                  @saveScheduleEvent="saveScheduleEvent"
                  @cancelGathering="cancelGathering"
                  :reminder-enabled.sync="reminderEnabled"
                  :reminder-lead-time-hours.sync="reminderLeadTimeHours"
                  :recurrence-frequency.sync="recurrenceFrequency"
                  :location.sync="scheduleLocation"
                />
              </div>
            </template>
            <template v-else>
              <!-- Times -->
              <div
                :class="calendarOnly ? 'tw-w-12' : ''"
                class="tw-w-8 tw-flex-none sm:tw-w-12"
              >
                <div
                  :class="calendarOnly ? 'tw-invisible' : 'tw-visible'"
                  class="tw-sticky tw-top-14 tw-z-10 -tw-ml-3 tw-mb-3 tw-h-11 tw-bg-wood-deep sm:tw-top-16 sm:tw-ml-0"
                >
                  <div
                    :class="hasPrevPage ? 'tw-visible' : 'tw-invisible'"
                    class="tw-sticky tw-top-14 tw-ml-0.5 tw-self-start tw-pt-1.5 sm:tw-top-16 sm:-tw-ml-2"
                  >
                    <v-btn
                      class="tw-border-gray"
                      outlined
                      icon
                      @click="prevPage"
                      ><v-icon>mdi-chevron-left</v-icon></v-btn
                    >
                  </div>
                </div>

                <div
                  :class="calendarOnly ? '' : '-tw-ml-3'"
                  class="-tw-mt-[8px] sm:tw-ml-0"
                >
                  <div
                    v-for="(time, i) in splitTimes[0]"
                    :key="i"
                    :id="time.id"
                    class="tw-pr-1 tw-text-right tw-text-xs tw-font-light tw-uppercase sm:tw-pr-2"
                    :style="{ height: `${timeslotHeight}px` }"
                  >
                    {{ time.text }}
                  </div>
                </div>

                <template v-if="splitTimes[1].length > 0">
                  <div
                    :style="{
                      height: `${SPLIT_GAP_HEIGHT}px`,
                    }"
                  ></div>
                  <div
                    v-if="splitTimes[1].length > 0"
                    :class="calendarOnly ? '' : '-tw-ml-3'"
                    class="sm:tw-ml-0"
                  >
                    <div
                      v-for="(time, i) in splitTimes[1]"
                      :key="i"
                      :id="time.id"
                      class="tw-pr-1 tw-text-right tw-text-xs tw-font-light tw-uppercase sm:tw-pr-2"
                      :style="{ height: `${timeslotHeight}px` }"
                    >
                      {{ time.text }}
                    </div>
                  </div>
                </template>
              </div>

              <!-- Middle section -->
              <div class="tw-grow">
                <div
                  ref="calendar"
                  @scroll="onCalendarScroll"
                  class="tw-relative tw-flex tw-flex-col"
                >
                  <!-- Days -->
                  <div
                    :class="
                      sampleCalendarEventsByDay
                        ? undefined
                        : 'tw-sticky tw-top-14'
                    "
                    class="tw-z-10 tw-flex tw-h-14 tw-items-center tw-bg-wood-deep sm:tw-top-16"
                  >
                    <template v-for="(day, i) in days">
                      <div
                        v-if="!day.isConsecutive"
                        :style="{ width: `${SPLIT_GAP_WIDTH}px` }"
                        :key="`${i}-gap`"
                      ></div>
                      <div :key="i" class="tw-flex-1 tw-bg-wood-deep">
                        <div class="tw-text-center">
                          <div
                            v-if="isSpecificDates"
                            class="tw-text-[12px] tw-font-light tw-capitalize tw-text-parchment-dim sm:tw-text-xs"
                          >
                            {{ day.dateString }}
                          </div>
                          <div class="tw-text-base tw-capitalize sm:tw-text-lg">
                            {{ day.dayText }}
                          </div>
                        </div>
                      </div>
                    </template>
                  </div>

                  <!-- Calendar -->
                  <div class="tw-flex tw-flex-col">
                    <div class="tw-flex-1">
                      <div
                        id="drag-section"
                        data-long-press-delay="500"
                        class="tw-relative tw-flex"
                        @mouseleave="resetCurTimeslot"
                      >
                        <!-- Loader -->
                        <div
                          v-if="showLoader"
                          class="tw-absolute tw-z-10 tw-grid tw-h-full tw-w-full tw-place-content-center"
                        >
                          <v-progress-circular
                            class="tw-text-brass"
                            indeterminate
                          />
                        </div>

                        <template v-for="(day, d) in days">
                          <div
                            v-if="!day.isConsecutive"
                            :style="{ width: `${SPLIT_GAP_WIDTH}px` }"
                            :key="`${d}-gap`"
                          ></div>
                          <div
                            :key="d"
                            class="tw-relative tw-flex-1"
                            :class="loadingResponses.loading && 'tw-opacity-50'"
                          >
                            <!-- Timeslots -->
                            <div
                              v-for="(_, t) in splitTimes[0]"
                              :key="`${d}-${t}-0`"
                              class="tw-w-full"
                            >
                              <div
                                class="timeslot tw-relative tw-flex tw-items-center tw-justify-center"
                                :class="
                                  timeslotClassStyle[d * times.length + t]
                                    ?.class
                                "
                                :style="
                                  timeslotClassStyle[d * times.length + t]
                                    ?.style
                                "
                                v-on="timeslotVon[d * times.length + t]"
                              >
                                <span
                                  v-if="timeslotCounts[d * times.length + t]"
                                  class="timeslot-count"
                                  :class="
                                    timeslotCounts[d * times.length + t].class
                                  "
                                  >{{
                                    timeslotCounts[d * times.length + t].text
                                  }}</span
                                >
                              </div>
                            </div>

                            <template v-if="splitTimes[1].length > 0">
                              <div
                                :style="{
                                  height: `${SPLIT_GAP_HEIGHT}px`,
                                }"
                              ></div>
                              <div
                                v-for="(_, t) in splitTimes[1]"
                                :key="`${d}-${t}-1`"
                                class="tw-w-full"
                              >
                                <div
                                  class="timeslot tw-relative tw-flex tw-items-center tw-justify-center"
                                  :class="
                                    timeslotClassStyle[
                                      d * times.length +
                                        t +
                                        splitTimes[0].length
                                    ]?.class
                                  "
                                  :style="
                                    timeslotClassStyle[
                                      d * times.length +
                                        t +
                                        splitTimes[0].length
                                    ]?.style
                                  "
                                  v-on="
                                    timeslotVon[
                                      d * times.length +
                                        t +
                                        splitTimes[0].length
                                    ]
                                  "
                                >
                                  <span
                                    v-if="
                                      timeslotCounts[
                                        d * times.length +
                                          t +
                                          splitTimes[0].length
                                      ]
                                    "
                                    class="timeslot-count"
                                    :class="
                                      timeslotCounts[
                                        d * times.length +
                                          t +
                                          splitTimes[0].length
                                      ].class
                                    "
                                    >{{
                                      timeslotCounts[
                                        d * times.length +
                                          t +
                                          splitTimes[0].length
                                      ].text
                                    }}</span
                                  >
                                </div>
                              </div>
                            </template>

                            <!-- Calendar events -->
                            <template
                              v-if="
                                !loadingCalendarEvents &&
                                (editing ||
                                  alwaysShowCalendarEvents ||
                                  showCalendarEvents)
                              "
                            >
                              <template
                                v-for="calendarEvent in calendarEventsByDay[
                                  d + page * maxDaysPerPage
                                ]"
                              >
                                <CalendarEventBlock
                                  :blockStyle="getTimeBlockStyle(calendarEvent)"
                                  :key="calendarEvent.id"
                                  :calendarEvent="calendarEvent"
                                  :noEventNames="noEventNames"
                                  transitionName="fade-transition"
                                />
                              </template>
                            </template>

                            <!-- Scheduled event -->
                            <div v-if="state === states.SCHEDULE_EVENT">
                              <div
                                v-if="
                                  (dragStart && dragStart.col === d) ||
                                  (!dragStart &&
                                    curScheduledEvent &&
                                    curScheduledEvent.col === d)
                                "
                                class="tw-absolute tw-w-full tw-select-none tw-p-px"
                                :style="scheduledEventStyle"
                                style="pointer-events: none"
                              >
                                <div
                                  class="tw-h-full tw-w-full tw-overflow-hidden tw-text-ellipsis tw-rounded tw-border tw-border-solid tw-border-brass tw-bg-brass tw-p-px tw-text-xs"
                                >
                                  <div class="tw-font-medium tw-text-wood-deep">
                                    {{ event.name }}
                                  </div>
                                </div>
                              </div>
                            </div>

                            <!-- Sign up blocks (dragged / saved / to-add) -->
                            <SignUpBlocksOverlay
                              :dragging="
                                state === states.EDIT_SIGN_UP_BLOCKS &&
                                !!dragStart &&
                                dragStart.col === d
                              "
                              :draggedBlockStyle="signUpBlockBeingDraggedStyle"
                              :draggedBlockName="newSignUpBlockName"
                              :isSignUp="isSignUp"
                              :blocks="
                                signUpBlocksByDay[d + page * maxDaysPerPage]
                              "
                              :blocksToAdd="
                                signUpBlocksToAddByDay[
                                  d + page * maxDaysPerPage
                                ]
                              "
                              @block-click="handleSignUpBlockClick"
                            />

                            <!-- Overlaid availabilities -->
                            <div v-if="overlayAvailability">
                              <div
                                v-for="(timeBlock, tb) in overlaidAvailability[
                                  d
                                ]"
                                :key="tb"
                                class="tw-absolute tw-w-full tw-select-none tw-p-px"
                                :style="getTimeBlockStyle(timeBlock)"
                                style="pointer-events: none"
                              >
                                <div
                                  class="tw-h-full tw-w-full tw-border-2"
                                  :class="
                                    timeBlock.type === 'available'
                                      ? 'overlay-avail-shadow-green tw-border-[#00994CB3] tw-bg-[#00994C66]'
                                      : 'overlay-avail-shadow-yellow tw-border-[#997700CC] tw-bg-[#FFE8B8B3]'
                                  "
                                ></div>
                              </div>
                            </div>
                          </div>
                        </template>
                      </div>
                    </div>
                  </div>

                  <ZigZag
                    v-if="hasPrevPage"
                    left
                    class="tw-absolute tw-left-0 tw-top-0 tw-h-full tw-w-3"
                  />
                  <ZigZag
                    v-if="hasNextPage"
                    right
                    class="tw-absolute tw-right-0 tw-top-0 tw-h-full tw-w-3"
                  />
                </div>

                <!-- Hint text (desktop) -->
                <v-expand-transition>
                  <div
                    :key="hintText"
                    v-if="!isPhone && hintTextShown"
                    class="tw-sticky tw-bottom-4 tw-z-10 tw-flex"
                  >
                    <div
                      class="tw-mt-2 tw-flex tw-w-full tw-items-center tw-justify-between tw-gap-1 tw-rounded-md tw-bg-leather tw-p-2 tw-px-[7px] tw-text-sm tw-text-parchment-dim"
                    >
                      <div class="tw-flex tw-items-center tw-gap-1">
                        <v-icon small>mdi-information-outline</v-icon>
                        {{ hintText }}
                      </div>
                      <v-icon small @click="closeHint">mdi-close</v-icon>
                    </div>
                  </div>
                </v-expand-transition>

                <v-expand-transition>
                  <div
                    v-if="
                      state !== states.EDIT_AVAILABILITY &&
                      max !== respondents.length &&
                      Object.keys(fetchedResponses).length !== 0 &&
                      !loadingResponses.loading
                    "
                  >
                    <div class="tw-mt-2 tw-text-sm tw-text-parchment-dim">
                      Note: There's no time when all
                      {{ respondents.length }} respondents are available.
                    </div>
                  </div>
                </v-expand-transition>

                <ToolRow
                  v-if="!isPhone && !calendarOnly"
                  :event="event"
                  :state="state"
                  :states="states"
                  :cur-timezone.sync="curTimezone"
                  :timezone-reference-date="timezoneReferenceDate"
                  :show-best-times.sync="showBestTimes"
                  :hide-if-needed.sync="hideIfNeeded"
                  :is-weekly="isWeekly"
                  :calendar-permission-granted="calendarPermissionGranted"
                  :week-offset="weekOffset"
                  :num-responses="respondents.length"
                  :mobile-num-days.sync="mobileNumDays"
                  :allow-schedule-event="allowScheduleEvent"
                  :show-event-options="showEventOptions"
                  :time-type.sync="timeType"
                  @toggleShowEventOptions="toggleShowEventOptions"
                  @update:weekOffset="(val) => $emit('update:weekOffset', val)"
                  @scheduleEvent="scheduleEvent"
                  @cancelScheduleEvent="cancelScheduleEvent"
                  @saveScheduleEvent="saveScheduleEvent"
                  @cancelGathering="cancelGathering"
                  :reminder-enabled.sync="reminderEnabled"
                  :reminder-lead-time-hours.sync="reminderLeadTimeHours"
                  :recurrence-frequency.sync="recurrenceFrequency"
                  :location.sync="scheduleLocation"
                />
              </div>

              <div
                v-if="!calendarOnly"
                :class="calendarOnly ? 'tw-invisible' : 'tw-visible'"
                class="tw-sticky tw-top-14 tw-z-10 tw-mb-4 tw-h-11 tw-bg-wood-deep sm:tw-top-16"
              >
                <div
                  :class="hasNextPage ? 'tw-visible' : 'tw-invisible'"
                  class="tw-sticky tw-top-14 -tw-mr-2 tw-self-start tw-pt-1.5 sm:tw-top-16"
                >
                  <v-btn class="tw-border-gray" outlined icon @click="nextPage"
                    ><v-icon>mdi-chevron-right</v-icon></v-btn
                  >
                </div>
              </div>
            </template>
          </div>

          <!-- Right hand side content -->

          <div
            v-if="!calendarOnly"
            class="tw-px-4 tw-py-4 sm:tw-sticky sm:tw-top-16 sm:tw-flex-none sm:tw-self-start sm:tw-py-0 sm:tw-pl-0 sm:tw-pr-0 sm:tw-pt-14"
            :style="{ width: rightSideWidth }"
          >
            <!-- Show section on the right depending on some if conditions -->
            <template v-if="isSignUp">
              <div class="tw-mb-2 tw-text-lg tw-text-parchment">Slots</div>
              <div v-if="!isOwner" class="tw-mb-3 tw-flex tw-flex-col">
                <div
                  class="tw-flex tw-flex-col tw-gap-1 tw-rounded-md tw-bg-leather tw-p-3 tw-text-xs tw-italic tw-text-parchment-dim"
                >
                  <div v-if="!authUser || alreadyRespondedToSignUpForm">
                    <a class="tw-underline" :href="`mailto:${event.ownerId}`"
                      >Contact sign up creator</a
                    >
                    to edit your slot
                  </div>
                  <div v-if="event.blindAvailabilityEnabled">
                    Responses are only visible to creator
                  </div>
                </div>
              </div>
              <SignUpBlocksList
                ref="signUpBlocksList"
                :signUpBlocks="signUpBlocksByDay.flat()"
                :signUpBlocksToAdd="signUpBlocksToAddByDay.flat()"
                :isEditing="state == states.EDIT_SIGN_UP_BLOCKS"
                :isOwner="isOwner"
                :alreadyResponded="alreadyRespondedToSignUpForm"
                :anonymous="event.blindAvailabilityEnabled"
                @update:signUpBlock="editSignUpBlock"
                @delete:signUpBlock="deleteSignUpBlock"
                @signUpForBlock="$emit('signUpForBlock', $event)"
              />
            </template>
            <template v-else-if="state === states.SET_SPECIFIC_TIMES">
              <SpecificTimesInstructions
                v-if="!isPhone"
                :numTempTimes="tempTimes.size"
                @saveTempTimes="saveTempTimes"
              />
            </template>
            <template v-else>
              <div
                class="tw-flex tw-flex-col tw-gap-5"
                v-if="state == states.EDIT_AVAILABILITY"
              >
                <div
                  v-if="
                    !(
                      calendarPermissionGranted &&
                      !event.daysOnly &&
                      !addingAvailabilityAsGuest
                    )
                  "
                  class="tw-flex tw-flex-wrap tw-items-baseline tw-gap-1 tw-text-sm tw-italic tw-text-parchment-dim"
                >
                  {{
                    (userHasResponded && !addingAvailabilityAsGuest) ||
                    curGuestId
                      ? "Editing"
                      : "Adding"
                  }}
                  availability as
                  <div
                    v-if="curGuestId && canEditGuestName"
                    class="tw-group tw-mt-0.5 tw-flex tw-w-fit tw-cursor-pointer tw-items-center tw-gap-1"
                    @click="openEditGuestNameDialog"
                  >
                    <span class="tw-font-medium group-hover:tw-underline">{{
                      curGuestId
                    }}</span>
                    <v-icon small>mdi-pencil</v-icon>
                  </div>
                  <span v-else>
                    {{
                      authUser && !addingAvailabilityAsGuest
                        ? `${authUser.firstName} ${authUser.lastName}`
                        : curGuestId?.length > 0
                        ? curGuestId
                        : "a guest"
                    }}
                  </span>
                  <v-dialog
                    v-model="editGuestNameDialog"
                    width="400"
                    content-class="tw-m-0"
                  >
                    <v-card>
                      <v-card-title>Edit guest name</v-card-title>
                      <v-card-text>
                        <v-text-field
                          v-model="newGuestName"
                          label="Guest name"
                          autofocus
                          @keydown.enter="saveGuestName"
                          hide-details
                        ></v-text-field>
                      </v-card-text>
                      <v-card-actions>
                        <v-spacer />
                        <v-btn text @click="editGuestNameDialog = false"
                          >Cancel</v-btn
                        >
                        <v-btn text color="primary" @click="saveGuestName"
                          >Save</v-btn
                        >
                      </v-card-actions>
                    </v-card>
                  </v-dialog>
                </div>
                <div class="tw-flex tw-flex-col tw-gap-3">
                  <AvailabilityTypeToggle
                    v-if="!isPhone"
                    class="tw-w-full"
                    v-model="availabilityType"
                  />
                  <ColorLegend />
                </div>
                <!-- User's calendar accounts -->
                <CalendarAccounts
                  v-if="
                    calendarPermissionGranted &&
                    !event.daysOnly &&
                    !addingAvailabilityAsGuest
                  "
                  :toggleState="true"
                  :eventId="event._id"
                  :calendar-events-map="calendarEventsMap"
                  :initialCalendarAccountsData="authUser.calendarAccounts"
                ></CalendarAccounts>

                <div v-if="showOverlayAvailabilityToggle">
                  <v-switch
                    id="overlay-availabilities-toggle"
                    inset
                    :input-value="overlayAvailability"
                    @change="updateOverlayAvailability"
                    hide-details
                  >
                    <template v-slot:label>
                      <div class="tw-text-sm tw-text-parchment">
                        Overlay availabilities
                      </div>
                    </template>
                  </v-switch>

                  <div class="tw-mt-2 tw-text-xs tw-text-parchment-dim">
                    View everyone's availability while inputting your own
                  </div>
                </div>

                <!-- Options section -->
                <div
                  v-if="!event.daysOnly && showCalendarOptions"
                  ref="optionsSection"
                >
                  <ExpandableSection
                    label="Options"
                    :value="showEditOptions"
                    @input="toggleShowEditOptions"
                  >
                    <div class="tw-flex tw-flex-col tw-gap-5 tw-pt-2.5">
                      <v-dialog
                        v-if="showCalendarOptions"
                        v-model="calendarOptionsDialog"
                        width="500"
                      >
                        <template v-slot:activator="{ on, attrs }">
                          <v-btn
                            outlined
                            class="tw-border-gray tw-text-sm"
                            v-on="on"
                            v-bind="attrs"
                          >
                            Calendar options...
                          </v-btn>
                        </template>

                        <v-card>
                          <v-card-title class="tw-flex">
                            <div>Calendar options</div>
                            <v-spacer />
                            <v-btn icon @click="calendarOptionsDialog = false">
                              <v-icon>mdi-close</v-icon>
                            </v-btn>
                          </v-card-title>
                          <v-card-text
                            class="tw-flex tw-flex-col tw-gap-6 tw-pb-8 tw-pt-2"
                          >
                            <BufferTimeSwitch :bufferTime.sync="bufferTime" />

                            <WorkingHoursToggle
                              :workingHours.sync="workingHours"
                              :timezone="curTimezone"
                            />
                          </v-card-text>
                        </v-card>
                      </v-dialog>
                    </div>
                  </ExpandableSection>
                </div>

                <!-- Delete availability button -->
                <div
                  v-if="
                    (!addingAvailabilityAsGuest && userHasResponded) ||
                    curGuestId
                  "
                >
                  <v-dialog
                    v-model="deleteAvailabilityDialog"
                    width="500"
                    persistent
                  >
                    <template v-slot:activator="{ on, attrs }">
                      <span
                        v-bind="attrs"
                        v-on="on"
                        class="tw-cursor-pointer tw-text-sm tw-text-red"
                      >
                        Delete availability
                      </span>
                    </template>

                    <v-card>
                      <v-card-title>Are you sure?</v-card-title>
                      <v-card-text class="tw-text-sm tw-text-parchment-dim"
                        >Are you sure you want to delete your availability from
                        this event?</v-card-text
                      >
                      <v-card-actions>
                        <v-spacer />
                        <v-btn text @click="deleteAvailabilityDialog = false"
                          >Cancel</v-btn
                        >
                        <v-btn
                          text
                          color="error"
                          @click="
                            $emit('deleteAvailability')
                            deleteAvailabilityDialog = false
                          "
                          >Delete</v-btn
                        >
                      </v-card-actions>
                    </v-card>
                  </v-dialog>
                </div>
              </div>
              <template v-else>
                <RespondentsList
                  ref="respondentsList"
                  :event="event"
                  :eventId="event._id"
                  :days="allDays"
                  :times="times"
                  :curDate="getDateFromRowCol(curTimeslot.row, curTimeslot.col)"
                  :curRespondent="curRespondent"
                  :curRespondents="curRespondents"
                  :curTimeslot="curTimeslot"
                  :curTimeslotAvailability="curTimeslotAvailability"
                  :respondents="respondents"
                  :parsedResponses="parsedResponses"
                  :isOwner="isOwner"
                  :attendees="event.attendees"
                  :showCalendarEvents.sync="showCalendarEvents"
                  :responsesFormatted="responsesFormatted"
                  :timezone="curTimezone"
                  :show-best-times.sync="showBestTimes"
                  :hide-if-needed.sync="hideIfNeeded"
                  :show-response-counts.sync="showResponseCounts"
                  :start-calendar-on-monday.sync="startCalendarOnMonday"
                  :show-event-options="showEventOptions"
                  :addingAvailabilityAsGuest="addingAvailabilityAsGuest"
                  @toggleShowEventOptions="toggleShowEventOptions"
                  @addAvailability="$emit('addAvailability')"
                  @addAvailabilityAsGuest="$emit('addAvailabilityAsGuest')"
                  @mouseOverRespondent="mouseOverRespondent"
                  @mouseLeaveRespondent="mouseLeaveRespondent"
                  @clickRespondent="clickRespondent"
                  @editGuestAvailability="editGuestAvailability"
                  @refreshEvent="refreshEvent"
                />
              </template>
            </template>
          </div>
        </div>

        <ToolRow
          v-if="isPhone && !calendarOnly"
          class="tw-px-4"
          :event="event"
          :state="state"
          :states="states"
          :cur-timezone.sync="curTimezone"
          :timezone-reference-date="timezoneReferenceDate"
          :show-best-times.sync="showBestTimes"
          :hide-if-needed.sync="hideIfNeeded"
          :start-calendar-on-monday.sync="startCalendarOnMonday"
          :is-weekly="isWeekly"
          :calendar-permission-granted="calendarPermissionGranted"
          :week-offset="weekOffset"
          :num-responses="respondents.length"
          :mobile-num-days.sync="mobileNumDays"
          :allow-schedule-event="allowScheduleEvent"
          :show-event-options="showEventOptions"
          :time-type.sync="timeType"
          @toggleShowEventOptions="toggleShowEventOptions"
          @update:weekOffset="(val) => $emit('update:weekOffset', val)"
          @scheduleEvent="scheduleEvent"
          @cancelScheduleEvent="cancelScheduleEvent"
          @saveScheduleEvent="saveScheduleEvent"
          @cancelGathering="cancelGathering"
          :reminder-enabled.sync="reminderEnabled"
          :reminder-lead-time-hours.sync="reminderLeadTimeHours"
          :recurrence-frequency.sync="recurrenceFrequency"
          :location.sync="scheduleLocation"
        />

        <!-- Fixed bottom section for mobile -->
        <div
          v-if="isPhone && !calendarOnly"
          class="tw-fixed tw-z-20 tw-w-full"
          :style="{ bottom: '4rem' }"
        >
          <!-- Hint text (mobile) -->
          <v-expand-transition>
            <template v-if="hintTextShown">
              <div :key="hintText">
                <div
                  :class="`tw-flex tw-w-full tw-items-center tw-justify-between tw-gap-1 tw-bg-leather tw-px-2 tw-py-2 tw-text-sm tw-text-parchment-dim`"
                >
                  <div
                    :class="`tw-flex tw-gap-${hintText.length > 60 ? 2 : 1}`"
                  >
                    <v-icon small>mdi-information-outline</v-icon>
                    <div>
                      {{ hintText }}
                    </div>
                  </div>
                  <v-icon small @click="closeHint">mdi-close</v-icon>
                </div>
              </div>
            </template>
          </v-expand-transition>

          <!-- Fixed pos availability toggle (mobile) -->
          <v-expand-transition>
            <div v-if="editing && !isSignUp">
              <div class="tw-bg-leather tw-p-4">
                <AvailabilityTypeToggle
                  class="tw-w-full"
                  v-model="availabilityType"
                />
              </div>
            </div>
          </v-expand-transition>

          <!-- GCal week selector -->
          <v-expand-transition>
            <div v-if="isWeekly && editing && calendarPermissionGranted">
              <div class="tw-h-16 tw-text-sm">
                <GCalWeekSelector
                  :week-offset="weekOffset"
                  :event="event"
                  @update:weekOffset="(val) => $emit('update:weekOffset', val)"
                  :start-on-monday="event.startOnMonday"
                />
              </div>
            </div>
          </v-expand-transition>

          <!-- Respondents list -->
          <v-expand-transition>
            <div v-if="delayedShowStickyRespondents">
              <div class="tw-bg-leather tw-p-4">
                <RespondentsList
                  :max-height="100"
                  :event="event"
                  :eventId="event._id"
                  :days="allDays"
                  :times="times"
                  :curDate="getDateFromRowCol(curTimeslot.row, curTimeslot.col)"
                  :curRespondent="curRespondent"
                  :curRespondents="curRespondents"
                  :curTimeslot="curTimeslot"
                  :curTimeslotAvailability="curTimeslotAvailability"
                  :respondents="respondents"
                  :parsedResponses="parsedResponses"
                  :isOwner="isOwner"
                  :attendees="event.attendees"
                  :showCalendarEvents.sync="showCalendarEvents"
                  :responsesFormatted="responsesFormatted"
                  :timezone="curTimezone"
                  :show-best-times.sync="showBestTimes"
                  :hide-if-needed.sync="hideIfNeeded"
                  :show-response-counts.sync="showResponseCounts"
                  :show-event-options="showEventOptions"
                  :addingAvailabilityAsGuest="addingAvailabilityAsGuest"
                  @toggleShowEventOptions="toggleShowEventOptions"
                  @addAvailability="$emit('addAvailability')"
                  @addAvailabilityAsGuest="$emit('addAvailabilityAsGuest')"
                  @mouseOverRespondent="mouseOverRespondent"
                  @mouseLeaveRespondent="mouseLeaveRespondent"
                  @clickRespondent="clickRespondent"
                  @editGuestAvailability="editGuestAvailability"
                  @refreshEvent="refreshEvent"
                />
              </div>
            </div>
          </v-expand-transition>

          <!-- Specific times instructions -->
          <v-expand-transition>
            <div
              v-if="state === states.SET_SPECIFIC_TIMES"
              class="-tw-mb-16 tw-bg-leather tw-p-4"
            >
              <SpecificTimesInstructions
                :numTempTimes="tempTimes.size"
                @saveTempTimes="saveTempTimes"
              />
            </div>
          </v-expand-transition>
        </div>
      </div>
    </Tooltip>
  </span>
</template>

<style scoped>
.animate-bg-color {
  transition: background-color 0.25s ease-in-out;
}

.break {
  flex-basis: 100%;
  height: 0;
}

/* Traffic-light response count rendered as a solid pill inside a timeslot.
   The pill supplies its own background so the digit stays legible over any
   heatmap cell color, in both light and dark themes. */
.timeslot-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 15px;
  padding: 0 4px;
  border-radius: 9999px;
  font-size: 10px;
  font-weight: 700;
  line-height: 15px;
  pointer-events: none;
}
.timeslot-count--day {
  font-size: 11px;
  line-height: 17px;
  min-width: 17px;
  margin-top: 1px;
}
</style>

<style>
/* Make timezone select element the same width as content */
#timezone-select {
  width: 5px;
}
</style>

<script>
import {
  timeNumToTimeText,
  dateCompare,
  getDateHoursOffset,
  post,
  put,
  isBetween,
  clamp,
  isPhone,
  utcTimeToLocalTime,
  splitTimeBlocksByDay,
  getTimeBlock,
  dateToDowDate,
  _delete,
  get,
  getDateDayOffset,
  getSpecificTimesDayStarts,
  isDateBetween,
  generateEnabledCalendarsPayload,
  isTouchEnabled,
  isElementInViewport,
  lightOrDark,
  removeTransparencyFromHex,
  userPrefers12h,
  getCalendarAccountKey,
  getISODateString,
  getDateWithTimezone,
  getScheduleTimezoneOffset,
  getTimezoneReferenceDateForEvent,
  timeNumToTimeString,
  prefersStartOnMonday,
} from "@/utils"
import {
  availabilityTypes,
  calendarOptionsDefaults,
  eventTypes,
  isOwnerlessEvent,
  timeTypes,
  timeslotDurations,
} from "@/constants"
import { setScheduledEvent } from "@/utils/services/EventService"
import { nextScheduleLocation } from "./scheduleLocation"
import { mapMutations, mapActions, mapState, mapGetters } from "vuex"
import UserAvatarContent from "@/components/UserAvatarContent.vue"
import CalendarAccounts from "@/components/settings/CalendarAccounts.vue"
import SignUpBlock from "@/components/sign_up_form/SignUpBlock.vue"
import SignUpBlocksOverlay from "./SignUpBlocksOverlay.vue"
import SignUpBlocksList from "@/components/sign_up_form/SignUpBlocksList.vue"
import ZigZag from "./ZigZag.vue"
import ConfirmDetailsDialog from "./ConfirmDetailsDialog.vue"
import ToolRow from "./ToolRow.vue"
import RespondentsList from "./RespondentsList.vue"
import GCalWeekSelector from "./GCalWeekSelector.vue"
import ExpandableSection from "../ExpandableSection.vue"
import WorkingHoursToggle from "./WorkingHoursToggle.vue"
import AlertText from "../AlertText.vue"
import Tooltip from "../Tooltip.vue"
import ColorLegend from "./ColorLegend.vue"

import dayjs from "dayjs"
import ObjectID from "bson-objectid"
import utcPlugin from "dayjs/plugin/utc"
import timezonePlugin from "dayjs/plugin/timezone"
import AvailabilityTypeToggle from "./AvailabilityTypeToggle.vue"
import BufferTimeSwitch from "./BufferTimeSwitch.vue"
import CalendarEventBlock from "./CalendarEventBlock.vue" // Added import
import SpecificTimesInstructions from "./SpecificTimesInstructions.vue"
import dragGridMixin from "./dragGridMixin"
import availabilityMixin from "./availabilityMixin"
import currentAvailabilityMixin from "./currentAvailabilityMixin"
import respondentSelectionMixin from "./respondentSelectionMixin"
import timeslotStylingMixin from "./timeslotStylingMixin"
import optionsMixin from "./optionsMixin"
import {
  getSpecificTimeBlocks,
  buildSlotToBlockMap,
} from "./specificTimeBlocks"
dayjs.extend(utcPlugin)
dayjs.extend(timezonePlugin)

export default {
  name: "ScheduleOverlap",
  mixins: [
    dragGridMixin,
    availabilityMixin,
    currentAvailabilityMixin,
    respondentSelectionMixin,
    timeslotStylingMixin,
    optionsMixin,
  ],
  props: {
    event: { type: Object, required: true },
    fromEditEvent: { type: Boolean, default: false },

    loadingCalendarEvents: { type: Boolean, default: false }, // Whether we are currently loading the calendar events
    calendarEventsMap: { type: Object, default: () => {} }, // Object of different users' calendar events
    sampleCalendarEventsByDay: { type: Array, required: false }, // Sample calendar events to use for example calendars
    calendarPermissionGranted: { type: Boolean, default: false }, // Whether user has granted google calendar permissions

    weekOffset: { type: Number, default: 0 }, // Week offset used for displaying calendar events on weekly gatherings

    alwaysShowCalendarEvents: { type: Boolean, default: false }, // Whether to show calendar events all the time
    noEventNames: { type: Boolean, default: false }, // Whether to show "busy" instead of the event name
    calendarOnly: { type: Boolean, default: false }, // Whether to only show calendar and not respondents or any other controls
    interactable: { type: Boolean, default: true }, // Whether to allow user to interact with component
    showSnackbar: { type: Boolean, default: true }, // Whether to show snackbar when availability is automatically filled in
    animateTimeslotAlways: { type: Boolean, default: false }, // Whether to animate timeslots all the time
    showHintText: { type: Boolean, default: true }, // Whether to show the hint text telling user what to do

    curGuestId: { type: String, default: "" }, // Id of the current guest being edited
    addingAvailabilityAsGuest: { type: Boolean, default: false }, // Whether the signed in user is adding availability as a guest

    initialTimezone: { type: Object, default: () => ({}) },
  },
  data() {
    return {
      states: {
        HEATMAP: "heatmap", // Display heatmap of availabilities
        SINGLE_AVAILABILITY: "single_availability", // Show one person's availability
        SUBSET_AVAILABILITY: "subset_availability", // Show availability for a subset of people
        BEST_TIMES: "best_times", // Show only the times that work for most people
        EDIT_AVAILABILITY: "edit_availability", // Edit current user's availability
        EDIT_SIGN_UP_BLOCKS: "edit_sign_up_blocks", // Edit the slots on a sign up form
        SCHEDULE_EVENT: "schedule_event", // Schedule event on gcal
        SET_SPECIFIC_TIMES: "set_specific_times", // Set specific times for the event
      },
      state: "best_times",

      availability: new Set(), // The current user's availability
      ifNeeded: new Set(), // The current user's "if needed" availability
      tempTimes: new Set(), // The specific times that the user has selected for the event (pending save)
      availabilityAnimTimeouts: [], // Timeouts for availability animation
      availabilityAnimEnabled: false, // Whether to animate timeslots changing colors
      maxAnimTime: 1200, // Max amount of time for availability animation
      unsavedChanges: false, // If there are unsaved availability changes
      curTimeslot: { row: -1, col: -1 }, // The currently highlighted timeslot
      timeslotSelected: false, // Whether a timeslot is selected (used to persist selection on desktop)
      curTimeslotAvailability: {}, // The users available for the current timeslot
      curRespondent: "", // Id of the active respondent (set on hover)
      curRespondents: [], // Id of currently selected respondents (set on click)
      fetchedResponses: {}, // Responses fetched from the server for the dates currently shown
      loadingResponses: { loading: false, lastFetched: new Date().getTime() }, // Whether we're currently fetching the responses
      responsesFormatted: new Map(), // Map where date/time is mapped to the people that are available then
      tooltipContent: "", // The content of the tooltip

      /* Sign up form */
      signUpBlocksByDay: [], // The current event's sign up blocks by day
      signUpBlocksToAddByDay: [], // The sign up blocks to be added after hitting 'save'

      /* Edit options */
      showEditOptions:
        localStorage["showEditOptions"] == undefined
          ? false
          : localStorage["showEditOptions"] == "true",
      availabilityType: availabilityTypes.AVAILABLE, // The current availability type
      overlayAvailability: false, // Whether to overlay everyone's availability when editing
      bufferTime: calendarOptionsDefaults.bufferTime, // Set in mounted()
      workingHours: calendarOptionsDefaults.workingHours, // Set in mounted()

      /* Event Options */
      showEventOptions:
        localStorage["showEventOptions"] == undefined
          ? false
          : localStorage["showEventOptions"] == "true",
      showBestTimes:
        localStorage["showBestTimes"] == undefined
          ? false
          : localStorage["showBestTimes"] == "true",
      showResponseCounts:
        localStorage["showResponseCounts"] == undefined
          ? true
          : localStorage["showResponseCounts"] == "true",
      hideIfNeeded: false,

      /* Variables for drag stuff */
      DRAG_TYPES: {
        ADD: "add",
        REMOVE: "remove",
      },
      SPLIT_GAP_HEIGHT: 40,
      SPLIT_GAP_WIDTH: 20,
      HOUR_HEIGHT: 60,
      timeslot: {
        width: 0,
        height: 0,
      },
      dragging: false,
      dragType: "add",
      dragStart: null,
      dragCur: null,

      /* Variables for options */
      curTimezone: this.initialTimezone,
      curScheduledEvent: null, // The scheduled event represented in the form {hoursOffset, hoursLength, dayIndex}
      // Pre-gathering reminder options (persisted on save; see saveScheduleEvent)
      reminderEnabled: true,
      reminderLeadTimeHours: 24,
      // Recurrence (C5): "none" | "weekly" | "biweekly" | "monthly"
      recurrenceFrequency: "none",
      // Venue, editable while confirming the time (seeded from the event, and
      // re-seeded by the event watcher when it changes elsewhere)
      scheduleLocation: this.event.location ?? "",
      timeType:
        localStorage["timeType"] ??
        (userPrefers12h() ? timeTypes.HOUR12 : timeTypes.HOUR24), // Whether 12-hour or 24-hour
      showCalendarEvents: false,
      startCalendarOnMonday: prefersStartOnMonday(),

      /* Dialogs */
      deleteAvailabilityDialog: false,
      calendarOptionsDialog: false,
      editGuestNameDialog: false,
      newGuestName: "",

      /* Variables for scrolling */
      optionsVisible: false,
      calendarScrollLeft: 0, // The current scroll position of the calendar
      calendarMaxScroll: 0, // The maximum scroll amount of the calendar, scrolling to this point means we have scrolled to the end
      scrolledToRespondents: false, // whether we have scrolled to the respondents section
      delayedShowStickyRespondents: false, // showStickyRespondents variable but changes 100ms after the actual variable changes (to add some delay)
      delayedShowStickyRespondentsTimeout: null, // Timeout that sets delayedShowStickyRespondents

      /* Variables for pagination */
      page: 0,
      mobileNumDays: localStorage["mobileNumDays"]
        ? parseInt(localStorage["mobileNumDays"])
        : 3, // The number of days to show at a time on mobile
      pageHasChanged: false,

      hasRefreshedAuthUser: false,

      /* Variables for hint */
      hintState: true,

      /** Constants */
      months: [
        "jan",
        "feb",
        "mar",
        "apr",
        "may",
        "jun",
        "jul",
        "aug",
        "sep",
        "oct",
        "nov",
        "dec",
      ],
    }
  },
  computed: {
    ...mapState(["authUser", "overlayAvailabilitiesEnabled"]),
    ...mapGetters(["canInvite", "canManageUsers"]),
    /** Returns the width of the right side of the calendar */
    rightSideWidth() {
      if (this.isPhone) return "100%"
      return this.isSignUp ? "18rem" : "13rem"
    },
    /** Returns the days of the week in the correct order */
    daysOfWeek() {
      if (!this.event.daysOnly) {
        return ["sun", "mon", "tue", "wed", "thu", "fri", "sat"]
      }
      return !this.startCalendarOnMonday
        ? ["sun", "mon", "tue", "wed", "thu", "fri", "sat"]
        : ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
    },
    /** Only allow scheduling when a curScheduledEvent exists */
    allowScheduleEvent() {
      return !!this.curScheduledEvent
    },
    /** Returns the availability as an array */
    availabilityArray() {
      return [...this.availability].map((item) => new Date(item))
    },
    /** Returns the if needed availability as an array */
    ifNeededArray() {
      return [...this.ifNeeded].map((item) => new Date(item))
    },
    allowDrag() {
      return (
        this.state === this.states.EDIT_AVAILABILITY ||
        this.state === this.states.EDIT_SIGN_UP_BLOCKS ||
        this.state === this.states.SCHEDULE_EVENT ||
        this.state === this.states.SET_SPECIFIC_TIMES
      )
    },
    /** Returns an array of calendar events for all of the authUser's enabled calendars, separated by the day they occur on */
    calendarEventsByDay() {
      // If this is an example calendar
      if (this.sampleCalendarEventsByDay) return this.sampleCalendarEventsByDay

      // If the user isn't logged in or is adding availability as a guest
      if (!this.authUser || this.addingAvailabilityAsGuest) return []

      let events = []
      let event

      const calendarAccounts = this.authUser.calendarAccounts

      // Adds events from calendar accounts that are enabled
      for (const id in calendarAccounts) {
        if (!calendarAccounts[id].enabled) continue

        if (Object.prototype.hasOwnProperty.call(this.calendarEventsMap, id)) {
          for (const index in this.calendarEventsMap[id].calendarEvents) {
            event = this.calendarEventsMap[id].calendarEvents[index]

            // Check if we need to update authUser (to get latest subcalendars)
            const subCalendars = calendarAccounts[id].subCalendars
            if (!subCalendars || !(event.calendarId in subCalendars)) {
              // authUser doesn't contain the subCalendar, so push event to events without checking if subcalendar is enabled
              // and queue the authUser to be refreshed
              events.push(event)
              if (!this.hasRefreshedAuthUser) {
                this.refreshAuthUser()
              }
              continue
            }

            // Push event to events if subcalendar is enabled
            if (subCalendars[event.calendarId].enabled) {
              events.push(event)
            }
          }
        }
      }

      const eventsCopy = JSON.parse(JSON.stringify(events))

      const calendarEventsByDay = splitTimeBlocksByDay(
        this.event,
        eventsCopy,
        this.weekOffset,
        this.timezoneOffset
      )

      return calendarEventsByDay
    },
    curRespondentsSet() {
      return new Set(this.curRespondents)
    },

    // -----------------------------------
    //#region Sign up form
    // -----------------------------------

    /** Returns the name of the new sign up block being dragged */
    newSignUpBlockName() {
      return `Slot #${
        this.signUpBlocksByDay.flat().length +
        this.signUpBlocksToAddByDay.flat().length +
        1
      }`
    },

    /** Returns the max allowable drag */
    maxSignUpBlockRowSize() {
      if (!this.dragStart || !this.isSignUp) return null

      const selectedDay = this.signUpBlocksByDay[this.dragStart.col]
      const selectedDayToAdd = this.signUpBlocksToAddByDay[this.dragStart.col]

      if (selectedDay.length === 0 && selectedDayToAdd.length === 0) return null

      let maxSize = Infinity
      for (const block of [...selectedDay, ...selectedDayToAdd]) {
        if (block.hoursOffset * 4 > this.dragStart.row) {
          maxSize = Math.min(
            maxSize,
            block.hoursOffset * 4 - this.dragStart.row
          )
        }
      }

      return maxSize
    },

    /** Whether the current user has already responded to the sign up form */
    alreadyRespondedToSignUpForm() {
      if (!this.authUser || !this.signUpBlocksByDay) return false

      return this.signUpBlocksByDay.some((dayBlocks) =>
        dayBlocks.some((block) =>
          block.responses?.some(
            (response) => response.userId === this.authUser._id
          )
        )
      )
    },

    //#endregion

    /** Returns the max number of people in the curRespondents array available at any given time */
    curRespondentsMax() {
      let max = 0
      if (this.event.daysOnly) {
        for (const day of this.allDays) {
          const num = [
            ...(this.responsesFormatted.get(day.dateObject.getTime()) ??
              new Set()),
          ].filter((r) => this.curRespondentsSet.has(r)).length

          if (num > max) max = num
        }
      } else {
        for (let i = 0; i < this.event.dates.length; i++) {
          const date = new Date(this.event.dates[i])
          for (const time of this.times) {
            const num = [
              ...this.getRespondentsForHoursOffset(date, time.hoursOffset),
            ].filter((r) => this.curRespondentsSet.has(r)).length

            if (num > max) max = num
          }
        }
      }
      return max
    },
    /** Returns the day offset caused by the timezone offset. If the timezone offset changes the date, dayOffset != 0 */
    dayOffset() {
      return Math.floor((this.event.startTime - this.timezoneOffset / 60) / 24)
    },
    /** Returns all the days that are encompassed by startDate and endDate */
    allDays() {
      const days = []
      const datesSoFar = new Set()

      const getDateString = (date) => {
        let dateString = ""
        let dayString = ""
        const offsetDate = new Date(date)
        if (this.isSpecificTimes) {
          offsetDate.setTime(
            offsetDate.getTime() - this.timezoneOffset * 60 * 1000
          )
        } else {
          offsetDate.setDate(offsetDate.getDate() + this.dayOffset)
        }
        if (this.isSpecificDates) {
          dateString = `${
            this.months[offsetDate.getUTCMonth()]
          } ${offsetDate.getUTCDate()}`
          dayString = this.daysOfWeek[offsetDate.getUTCDay()]
        } else if (this.isWeekly) {
          const tmpDate = dateToDowDate(
            this.event.dates,
            offsetDate,
            this.weekOffset,
            true
          )

          dateString = `${
            this.months[tmpDate.getUTCMonth()]
          } ${tmpDate.getUTCDate()}`
          dayString = this.daysOfWeek[tmpDate.getUTCDay()]
        }
        return { dateString, dayString }
      }

      if (
        this.isSpecificTimes &&
        (this.state === this.states.SET_SPECIFIC_TIMES ||
          this.event.times?.length === 0)
      ) {
        for (const day of getSpecificTimesDayStarts(
          this.event.dates,
          this.curTimezone
        )) {
          const { dayString, dateString } = getDateString(day.dateObject)
          days.push({
            dayText: dayString,
            dateString,
            dateObject: day.dateObject,
            isConsecutive: day.isConsecutive,
          })
        }
        return days
      }

      for (let i = 0; i < this.event.dates.length; ++i) {
        const date = new Date(this.event.dates[i])
        datesSoFar.add(date.getTime())

        const { dayString, dateString } = getDateString(date)
        days.push({
          dayText: dayString,
          dateString,
          dateObject: date,
        })
      }

      let dayIndex = 0
      for (let i = 0; i < this.event.dates.length; ++i) {
        const date = new Date(this.event.dates[i])
        // See if the date goes into the next day
        const localStart = new Date(
          date.getTime() - this.timezoneOffset * 60 * 1000
        )
        const localEnd = new Date(
          date.getTime() +
            this.event.duration * 60 * 60 * 1000 -
            this.timezoneOffset * 60 * 1000
        )
        const localEndIsMidnight =
          localEnd.getUTCHours() === 0 && localEnd.getUTCMinutes() === 0
        if (
          localStart.getUTCDate() !== localEnd.getUTCDate() &&
          !localEndIsMidnight
        ) {
          // The date goes into the next day. Split the date into two dates
          let nextDate = new Date(date)
          nextDate.setUTCDate(nextDate.getUTCDate() + 1)
          if (!datesSoFar.has(nextDate.getTime())) {
            datesSoFar.add(nextDate.getTime())

            const { dayString, dateString } = getDateString(nextDate)
            days.splice(dayIndex + 1, 0, {
              dayText: dayString,
              dateString,
              dateObject: nextDate,
              excludeTimes: true,
            })
            dayIndex++
          }
        }
        dayIndex++
      }

      let prevDate = null // Stores the prevDate to check if the current date is consecutive to the previous date
      for (let i = 0; i < days.length; ++i) {
        let isConsecutive = true
        if (prevDate) {
          isConsecutive =
            prevDate.getTime() ===
            days[i].dateObject.getTime() - 24 * 60 * 60 * 1000
        }

        days[i].isConsecutive = isConsecutive

        prevDate = new Date(days[i].dateObject)
      }

      return days
    },
    /** Returns a subset of all days based on the page number */
    days() {
      const slice = this.allDays.slice(
        this.page * this.maxDaysPerPage,
        (this.page + 1) * this.maxDaysPerPage
      )
      slice[0] = { ...slice[0], isConsecutive: true }
      return slice
    },
    /** Returns all the days of the month */
    monthDays() {
      const monthDays = []
      const allDaysSet = new Set(
        this.allDays.map((d) => d.dateObject.getTime())
      )

      // Calculate monthIndex and year from event start date and page num
      const date = new Date(this.event.dates[0])
      const monthIndex = date.getUTCMonth() + this.page
      const year = date.getUTCFullYear()

      const lastDayOfPrevMonth = new Date(Date.UTC(year, monthIndex, 0))
      const lastDayOfCurMonth = new Date(Date.UTC(year, monthIndex + 1, 0))

      // Calculate num days from prev month, cur month, and next month to show
      const curDate = new Date(lastDayOfPrevMonth)
      let numDaysFromPrevMonth = 0
      const numDaysInCurMonth = lastDayOfCurMonth.getUTCDate()
      const numDaysFromNextMonth = 6 - lastDayOfCurMonth.getUTCDay()
      const hasDaysFromPrevMonth = !this.startCalendarOnMonday
        ? lastDayOfPrevMonth.getUTCDay() < 6
        : lastDayOfPrevMonth.getUTCDay() != 0
      if (hasDaysFromPrevMonth) {
        curDate.setUTCDate(
          curDate.getUTCDate() -
            (lastDayOfPrevMonth.getUTCDay() -
              (this.startCalendarOnMonday ? 1 : 0))
        )
        numDaysFromPrevMonth = lastDayOfPrevMonth.getUTCDay() + 1
      } else {
        curDate.setUTCDate(curDate.getUTCDate() + 1)
      }
      curDate.setUTCHours(this.event.startTime)

      // Add all days from prev month, cur month, and next month
      const totalDays =
        numDaysFromPrevMonth + numDaysInCurMonth + numDaysFromNextMonth
      for (let i = 0; i < totalDays; ++i) {
        // Only include days from the current month
        if (curDate.getUTCMonth() === lastDayOfCurMonth.getUTCMonth()) {
          monthDays.push({
            date: curDate.getUTCDate(),
            time: curDate.getTime(),
            dateObject: new Date(curDate),
            included: allDaysSet.has(curDate.getTime()),
          })
        } else {
          monthDays.push({
            date: "",
            time: curDate.getTime(),
            dateObject: new Date(curDate),
            included: false,
          })
        }

        curDate.setUTCDate(curDate.getUTCDate() + 1)
      }

      return monthDays
    },
    /** Map from datetime to whether that month day is included  */
    monthDayIncluded() {
      const includedMap = new Map()
      for (const monthDay of this.monthDays) {
        includedMap.set(monthDay.dateObject.getTime(), monthDay.included)
      }
      return includedMap
    },
    /** Returns the text to show for the current month */
    curMonthText() {
      const date = new Date(this.event.dates[0])
      const monthIndex = date.getUTCMonth() + this.page
      const year = date.getUTCFullYear()
      const lastDayOfCurMonth = new Date(Date.UTC(year, monthIndex + 1, 0))

      const monthText = this.months[lastDayOfCurMonth.getUTCMonth()]
      const yearText = lastDayOfCurMonth.getUTCFullYear()
      return `${monthText} ${yearText}`
    },
    defaultState() {
      // Either the heatmap or the best_times state, depending on the toggle
      return this.showBestTimes ? this.states.BEST_TIMES : this.states.HEATMAP
    },
    editing() {
      // Returns whether currently in the editing state
      return (
        this.state === this.states.EDIT_AVAILABILITY ||
        this.state === this.states.EDIT_SIGN_UP_BLOCKS
      )
    },
    scheduling() {
      // Returns whether currently in the scheduling state
      return this.state === this.states.SCHEDULE_EVENT
    },
    isPhone() {
      return isPhone(this.$vuetify)
    },
    isOwner() {
      return this.authUser?._id === this.event.ownerId
    },
    isGuestEvent() {
      return isOwnerlessEvent(this.event)
    },
    isSpecificDates() {
      return this.event.type === eventTypes.SPECIFIC_DATES || !this.event.type
    },
    isWeekly() {
      return this.event.type === eventTypes.DOW
    },
    isSignUp() {
      return this.event.isSignUpForm
    },
    isSpecificTimes() {
      return this.event.hasSpecificTimes
    },
    respondents() {
      return Object.values(this.parsedResponses)
        .map((r) => r.user)
        .filter(Boolean)
    },
    selectedGuestRespondent() {
      if (this.curRespondents.length !== 1) return ""

      const user = this.parsedResponses[this.curRespondents[0]].user
      return this.isGuest(user) ? user._id : ""
    },
    // Mirrors the server's requireResponseManager: admins always, otherwise
    // whoever manages the event — its owner, or member+ for a legacy ownerless
    // one. This returned true unconditionally, so a non-owner saw the pencil
    // and got a 403 on submit.
    canEditGuestName() {
      if (this.canManageUsers || this.isOwner) return true
      return this.isGuestEvent && this.canInvite
    },
    scheduledEventStyle() {
      const style = {}
      let top, height, isSecondSplit
      if (this.dragging) {
        top = this.dragStart.row
        height = this.dragCur.row - this.dragStart.row + 1
        isSecondSplit = this.dragStart.row >= this.splitTimes[0].length
      } else {
        top = this.curScheduledEvent.row
        height = this.curScheduledEvent.numRows
        isSecondSplit = this.curScheduledEvent.row >= this.splitTimes[0].length
      }

      if (isSecondSplit) {
        style.top = `calc(${top} * ${this.timeslotHeight}px + ${this.SPLIT_GAP_HEIGHT}px)`
      } else {
        style.top = `calc(${top} * ${this.timeslotHeight}px)`
      }
      style.height = `calc(${height} * ${this.timeslotHeight}px)`
      return style
    },
    signUpBlockBeingDraggedStyle() {
      const style = {}
      let top = 0,
        height = 0
      if (this.dragging) {
        top = this.dragStart.row
        height = this.dragCur.row - this.dragStart.row + 1
      }
      style.top = `calc(${top} * 1rem)`
      style.height = `calc(${height} * 1rem)`
      return style
    },
    /** Parses the responses to the gathering, makes necessary changes based on the type of event, and returns it */
    parsedResponses() {
      const parsed = {}

      // Return only current user availability if using blind availabilities and user is not owner
      if (this.event.blindAvailabilityEnabled && !this.isOwner) {
        // Keyed off the session only — the server applies the same rule, and
        // the ?guestName= escape hatch that bypassed it is gone.
        const userId = this.authUser?._id
        if (userId in this.event.responses) {
          const user = {
            ...this.event.responses[userId].user,
            _id: userId,
          }
          parsed[userId] = {
            ...this.event.responses[userId],
            availability: new Set(
              this.fetchedResponses[userId]?.availability?.map((a) =>
                new Date(a).getTime()
              )
            ),
            ifNeeded: new Set(
              this.fetchedResponses[userId]?.ifNeeded?.map((a) =>
                new Date(a).getTime()
              )
            ),
            user: user,
          }
        }
        return parsed
      }

      // Otherwise, parse responses so that if _id is null (i.e. guest user), then it is set to the guest user's name
      for (const k of Object.keys(this.event.responses)) {
        const newUser = {
          ...this.event.responses[k].user,
          _id: k,
        }
        parsed[k] = {
          ...this.event.responses[k],
          availability: new Set(
            this.fetchedResponses[k]?.availability?.map((a) =>
              new Date(a).getTime()
            )
          ),
          ifNeeded: new Set(
            this.fetchedResponses[k]?.ifNeeded?.map((a) =>
              new Date(a).getTime()
            )
          ),
          user: newUser,
        }
      }
      return parsed
    },
    max() {
      let max = 0
      for (const [dateTime, availability] of this.responsesFormatted) {
        if (availability.size > max) {
          max = availability.size
        }
      }

      return max
    },
    /** Returns a set containing the times for the event if it has specific times */
    specificTimesSet() {
      return new Set(this.event.times?.map((t) => new Date(t).getTime()) ?? [])
    },
    /** Whether recipients must select whole blocks of specific times at once */
    isWholeBlockSelection() {
      return !!this.event.wholeBlockSelection && this.isSpecificTimes
    },
    /** The specific times grouped into contiguous blocks (whole-block mode) */
    specificTimeBlocks() {
      if (!this.isWholeBlockSelection) return []
      return getSpecificTimeBlocks(
        [...this.specificTimesSet],
        this.timeslotDuration
      )
    },
    /** Map from a slot's ms timestamp to the block containing it */
    slotToBlock() {
      return buildSlotToBlockMap(this.specificTimeBlocks)
    },
    /**
     * Returns a two dimensional array of times
     * IF endTime < startTime:
     * the first element is an array of times between 12am and end time and the second element is an array of times between start time and 12am
     * ELSE:
     * the first element is an array of times between start time and end time. the second element is an empty array
     * */
    splitTimes() {
      const splitTimes = [[], []]

      const utcStartTime = this.event.startTime
      const utcEndTime = this.event.startTime + this.event.duration
      const localStartTime = utcTimeToLocalTime(
        utcStartTime,
        this.timezoneOffset
      )
      const localEndTime = utcTimeToLocalTime(utcEndTime, this.timezoneOffset)

      // Weird timezones are timezones that are not a multiple of 60 minutes (e.g. GMT-2:30)
      const isWeirdTimezone = this.timezoneOffset % 60 !== 0
      const startTimeIsWeird = utcStartTime % 1 !== 0
      let timeOffset = 0
      if (isWeirdTimezone !== startTimeIsWeird) {
        timeOffset = -0.5
      }

      const getExtraTimes = (hoursOffset) => {
        if (this.timeslotDuration === timeslotDurations.FIFTEEN_MINUTES) {
          return [
            {
              hoursOffset: hoursOffset + 0.25,
            },
            {
              hoursOffset: hoursOffset + 0.5,
            },
            {
              hoursOffset: hoursOffset + 0.75,
            },
          ]
        } else if (this.timeslotDuration === timeslotDurations.THIRTY_MINUTES) {
          return [
            {
              hoursOffset: hoursOffset + 0.5,
            },
          ]
        }
        return []
      }

      if (this.state === this.states.SET_SPECIFIC_TIMES) {
        // Hours offset for specific times starts from minHours
        for (let i = 0; i <= 23; ++i) {
          const hoursOffset = i
          if (i === 9) {
            // add an id so we can scroll to it
            splitTimes[0].push({
              id: "time-9",
              hoursOffset,
              text: timeNumToTimeText(i, this.timeType === timeTypes.HOUR12),
            })
          } else {
            splitTimes[0].push({
              hoursOffset,
              text: timeNumToTimeText(i, this.timeType === timeTypes.HOUR12),
            })
          }
          splitTimes[0].push(...getExtraTimes(hoursOffset))
        }
        return splitTimes
      }

      if (localEndTime <= localStartTime && localEndTime !== 0) {
        for (let i = 0; i < localEndTime; ++i) {
          splitTimes[0].push({
            hoursOffset: this.event.duration - (localEndTime - i),
            text: timeNumToTimeText(i, this.timeType === timeTypes.HOUR12),
          })
          splitTimes[0].push(
            ...getExtraTimes(this.event.duration - (localEndTime - i))
          )
        }
        for (let i = 0; i < 24 - localStartTime; ++i) {
          const adjustedI = i + timeOffset
          splitTimes[1].push({
            hoursOffset: adjustedI,
            text: timeNumToTimeText(
              localStartTime + adjustedI,
              this.timeType === timeTypes.HOUR12
            ),
          })
          splitTimes[1].push(...getExtraTimes(adjustedI))
        }
      } else {
        for (let i = 0; i < this.event.duration; ++i) {
          const adjustedI = i + timeOffset
          const utcTimeNum = this.event.startTime + adjustedI
          const localTimeNum = utcTimeToLocalTime(
            utcTimeNum,
            this.timezoneOffset
          )

          splitTimes[0].push({
            hoursOffset: adjustedI,
            text: timeNumToTimeText(
              localTimeNum,
              this.timeType === timeTypes.HOUR12
            ),
          })
          splitTimes[0].push(...getExtraTimes(adjustedI))
        }
        if (timeOffset !== 0) {
          const localTimeNum = utcTimeToLocalTime(
            this.event.startTime + this.event.duration - 0.5,
            this.timezoneOffset
          )
          splitTimes[0].push({
            hoursOffset: this.event.duration - 0.5,
            text: timeNumToTimeText(
              localTimeNum,
              this.timeType === timeTypes.HOUR12
            ),
          })
          splitTimes[0].push(...getExtraTimes(this.event.duration - 0.5))
        }
        splitTimes[1] = []
      }

      return splitTimes
    },
    /** Returns the times that are encompassed by startTime and endTime */
    times() {
      return [...this.splitTimes[1], ...this.splitTimes[0]]
    },
    timeslotDuration() {
      return this.event.timeIncrement ?? timeslotDurations.FIFTEEN_MINUTES
    },
    timeslotHeight() {
      if (this.timeslotDuration === timeslotDurations.FIFTEEN_MINUTES) {
        return Math.floor(this.HOUR_HEIGHT / 4)
      } else if (this.timeslotDuration === timeslotDurations.THIRTY_MINUTES) {
        return Math.floor(this.HOUR_HEIGHT / 2)
      } else if (this.timeslotDuration === timeslotDurations.ONE_HOUR) {
        return this.HOUR_HEIGHT
      }
      return Math.floor(this.HOUR_HEIGHT / 4)
    },
    timezoneOffset() {
      return getScheduleTimezoneOffset(
        this.event,
        this.curTimezone,
        this.weekOffset
      )
    },
    timezoneReferenceDate() {
      return getTimezoneReferenceDateForEvent(this.event, this.weekOffset)
    },
    userHasResponded() {
      return this.authUser && this.authUser._id in this.parsedResponses
    },
    showLeftZigZag() {
      return this.calendarScrollLeft > 0
    },
    showRightZigZag() {
      return Math.ceil(this.calendarScrollLeft) < this.calendarMaxScroll
    },
    maxDaysPerPage() {
      return this.isPhone ? this.mobileNumDays : 7
    },
    hasNextPage() {
      if (this.event.daysOnly) {
        const lastDay = new Date(this.event.dates[this.event.dates.length - 1])
        const curDate = new Date(this.event.dates[0])
        const monthIndex = curDate.getUTCMonth() + this.page
        const year = curDate.getUTCFullYear()

        const lastDayOfCurMonth = new Date(Date.UTC(year, monthIndex + 1, 0))

        return lastDayOfCurMonth.getTime() < lastDay.getTime()
      }

      return this.allDays.length - (this.page + 1) * this.maxDaysPerPage > 0
    },
    hasPrevPage() {
      return this.page > 0
    },
    /** Returns whether the event has more than one page */
    hasPages() {
      return this.hasNextPage || this.hasPrevPage
    },

    showStickyRespondents() {
      return (
        this.isPhone &&
        !this.scrolledToRespondents &&
        (this.curTimeslot.row !== -1 ||
          this.curRespondent.length > 0 ||
          this.curRespondents.length > 0)
      )
    },

    // Hint stuff
    hintText() {
      if (this.isPhone) {
        switch (this.state) {
          case this.states.EDIT_AVAILABILITY: {
            const daysOrTimes = this.event.daysOnly ? "days" : "times"
            if (this.availabilityType === availabilityTypes.IF_NEEDED) {
              return `Tap and drag to add your "if needed" ${daysOrTimes} in yellow.`
            }
            return `Tap and drag to add your "available" ${daysOrTimes} in green.`
          }
          case this.states.SCHEDULE_EVENT:
            return "Tap and drag on the calendar to set the gathering during those times, then Save."
          default:
            return ""
        }
      }

      switch (this.state) {
        case this.states.EDIT_AVAILABILITY: {
          const daysOrTimes = this.event.daysOnly ? "days" : "times"
          if (this.availabilityType === availabilityTypes.IF_NEEDED) {
            return `Click and drag to add your "if needed" ${daysOrTimes} in yellow.`
          }
          return `Click and drag to add your "available" ${daysOrTimes} in green.`
        }
        case this.states.SCHEDULE_EVENT:
          return "Click and drag on the calendar to set the gathering during those times, then Save."
        default:
          return ""
      }
    },
    hintClosed() {
      return !this.hintState || localStorage[this.hintStateLocalStorageKey]
    },
    hintStateLocalStorageKey() {
      return `closedHintText${this.state}`
    },
    hintTextShown() {
      return this.showHintText && this.hintText != "" && !this.hintClosed
    },

    timeslotClassStyle() {
      const classStyles = []
      for (let d = 0; d < this.days.length; ++d) {
        const day = this.days[d]
        for (let t = 0; t < this.splitTimes[0].length; ++t) {
          const time = this.splitTimes[0][t]
          classStyles.push(this.getTimeTimeslotClassStyle(day, time, d, t))
        }
        for (let t = 0; t < this.splitTimes[1].length; ++t) {
          const time = this.splitTimes[1][t]
          classStyles.push(
            this.getTimeTimeslotClassStyle(
              day,
              time,
              d,
              t + this.splitTimes[0].length
            )
          )
        }
      }
      return classStyles
    },
    dayTimeslotClassStyle() {
      const classStyles = []
      for (let i = 0; i < this.monthDays.length; ++i) {
        classStyles.push(
          this.getDayTimeslotClassStyle(this.monthDays[i].dateObject, i)
        )
      }
      return classStyles
    },
    /** Per-cell traffic-light response counts for the time grid (indexed like timeslotClassStyle) */
    timeslotCounts() {
      const counts = []
      for (let d = 0; d < this.days.length; ++d) {
        for (let t = 0; t < this.splitTimes[0].length; ++t) {
          counts.push(this.getTimeslotCount(this.getDateFromRowCol(t, d)))
        }
        for (let t = 0; t < this.splitTimes[1].length; ++t) {
          counts.push(
            this.getTimeslotCount(
              this.getDateFromRowCol(t + this.splitTimes[0].length, d)
            )
          )
        }
      }
      return counts
    },
    /** Per-cell traffic-light response counts for the daysOnly grid */
    dayTimeslotCounts() {
      const counts = []
      for (let i = 0; i < this.monthDays.length; ++i) {
        const date = this.monthDays[i].dateObject
        counts.push(
          this.monthDayIncluded.get(date.getTime())
            ? this.getTimeslotCount(date)
            : null
        )
      }
      return counts
    },
    timeslotVon() {
      const vons = []
      for (let d = 0; d < this.days.length; ++d) {
        for (let t = 0; t < this.times.length; ++t) {
          vons.push(this.getTimeslotVon(t, d))
        }
      }
      return vons
    },
    dayTimeslotVon() {
      const vons = []
      for (let i = 0; i < this.monthDays.length; ++i) {
        const row = Math.floor(i / 7)
        const col = i % 7
        vons.push(this.getTimeslotVon(row, col))
      }
      return vons
    },

    /** Whether to show spinner on top of availability grid */
    showLoader() {
      return (
        // Loading calendar events
        ((this.alwaysShowCalendarEvents || this.editing) &&
          this.loadingCalendarEvents) ||
        // Loading responses
        this.loadingResponses.loading
      )
    },

    /** Returns an array of time blocks representing the current user's availability
     * (used for displaying current user's availability on top of everybody else's availability)
     */
    overlaidAvailability() {
      const overlaidAvailability = []
      this.days.forEach((day, d) => {
        overlaidAvailability.push([])
        let curBlockIndex = 0
        const addOverlaidAvailabilityBlocks = (time, t) => {
          const date = this.getDateFromRowCol(t, d)
          if (!date) return

          const dragAdd =
            this.dragging &&
            this.inDragRange(t, d) &&
            this.dragType === this.DRAG_TYPES.ADD
          const dragRemove =
            this.dragging &&
            this.inDragRange(t, d) &&
            this.dragType === this.DRAG_TYPES.REMOVE

          // Check if timeslot is available or if needed or in the drag region
          if (
            dragAdd ||
            (!dragRemove &&
              (this.availability.has(date.getTime()) ||
                this.ifNeeded.has(date.getTime())))
          ) {
            // Determine whether to render as available or if needed block
            let type = availabilityTypes.AVAILABLE
            if (dragAdd) {
              type = this.availabilityType
            } else {
              type = this.availability.has(date.getTime())
                ? availabilityTypes.AVAILABLE
                : availabilityTypes.IF_NEEDED
            }

            if (curBlockIndex in overlaidAvailability[d]) {
              if (overlaidAvailability[d][curBlockIndex].type === type) {
                // Increase block length if matching type and curBlockIndex exists
                overlaidAvailability[d][curBlockIndex].hoursLength += 0.25
              } else {
                // Add a new block because type is different
                overlaidAvailability[d].push({
                  hoursOffset: time.hoursOffset,
                  hoursLength: 0.25,
                  type,
                })
                curBlockIndex++
              }
            } else {
              // Add a new block because block doesn't exist for current index
              overlaidAvailability[d].push({
                hoursOffset: time.hoursOffset,
                hoursLength: 0.25,
                type,
              })
            }
          } else if (curBlockIndex in overlaidAvailability[d]) {
            // Only increment cur block index if block already exists at the current index
            curBlockIndex++
          }
        }
        for (let t = 0; t < this.splitTimes[0].length; ++t) {
          addOverlaidAvailabilityBlocks(this.splitTimes[0][t], t)
        }
        if (curBlockIndex in overlaidAvailability[d]) {
          curBlockIndex++
        }
        for (let t = 0; t < this.splitTimes[1].length; ++t) {
          addOverlaidAvailabilityBlocks(
            this.splitTimes[1][t],
            t + this.splitTimes[0].length
          )
        }
      })
      return overlaidAvailability
    },

    // Options
    showOverlayAvailabilityToggle() {
      return this.respondents.length > 0 && this.overlayAvailabilitiesEnabled
    },
    showCalendarOptions() {
      return (
        !this.addingAvailabilityAsGuest &&
        this.calendarPermissionGranted &&
        !this.userHasResponded
      )
    },

    /** Returns an array of the x-offsets of the columns, taking into account the split gaps from non-consecutive days */
    columnOffsets() {
      const offsets = []
      let accumulatedOffset = 0
      for (let i = 0; i < this.days.length; ++i) {
        offsets.push(accumulatedOffset)
        if (!this.days[i].isConsecutive) {
          accumulatedOffset += this.SPLIT_GAP_WIDTH
        }
        accumulatedOffset += this.timeslot.width
      }
      return offsets
    },
  },
  methods: {
    ...mapMutations(["setAuthUser"]),
    ...mapActions(["showInfo", "showError"]),

    // -----------------------------------
    //#region Date
    // -----------------------------------

    /** Returns a date object from the dayindex and hoursoffset given */
    getDateFromDayHoursOffset(dayIndex, hoursOffset) {
      return getDateHoursOffset(this.days[dayIndex].dateObject, hoursOffset)
    },
    /** Returns a date object from the row and column given on the current page */
    getDateFromRowCol(row, col) {
      if (this.event.daysOnly) {
        const dateObject = this.monthDays[row * 7 + col]?.dateObject
        if (!dateObject) return null
        return new Date(dateObject)
      } else {
        return this.getDateFromDayTimeIndex(
          this.maxDaysPerPage * this.page + col,
          row
        )
      }
    },
    isColConsecutive(col) {
      return Boolean(this.days[col]?.isConsecutive)
    },
    /** Returns a date object from the day index and time index given */
    getDateFromDayTimeIndex(dayIndex, timeIndex) {
      const hasSecondSplit = this.splitTimes[1].length > 0
      const isFirstSplit = timeIndex < this.splitTimes[0].length
      const time = isFirstSplit
        ? this.splitTimes[0][timeIndex]
        : this.splitTimes[1][timeIndex - this.splitTimes[0].length]
      let adjustedDayIndex = dayIndex
      if (hasSecondSplit) {
        if (isFirstSplit) {
          adjustedDayIndex = dayIndex - 1
        } else if (dayIndex === this.allDays.length - 1) {
          return null
        }
      }
      const day = this.allDays[adjustedDayIndex]
      if (!day || !time) return null
      if (day.excludeTimes) {
        return null
      }

      const date = getDateHoursOffset(day.dateObject, time.hoursOffset)
      if (this.isSpecificTimes) {
        // TODO: see if we need to do anything for 0.5 timezones
        if (
          this.state !== this.states.SET_SPECIFIC_TIMES &&
          this.event.times?.length > 0
        ) {
          if (!this.specificTimesSet.has(date.getTime())) {
            return null
          }
        }
      } else {
        // Return null for times outside of the correct range
        if (time.hoursOffset < 0 || time.hoursOffset >= this.event.duration) {
          return null
        }
      }
      return date
    },
    //#endregion

    // -----------------------------------
    //#region Editing
    // -----------------------------------
    startEditing() {
      this.state = this.isSignUp
        ? this.states.EDIT_SIGN_UP_BLOCKS
        : this.states.EDIT_AVAILABILITY
      this.availabilityType = availabilityTypes.AVAILABLE
      this.availability = new Set()
      this.ifNeeded = new Set()

      if (this.authUser && !this.addingAvailabilityAsGuest) {
        this.resetCurUserAvailability()
      }
      this.$nextTick(() => (this.unsavedChanges = false))
      this.pageHasChanged = false
    },
    stopEditing() {
      this.state = this.defaultState
      this.stopAvailabilityAnim()

      // Reset options
      this.availabilityType = availabilityTypes.AVAILABLE
      this.overlayAvailability = false
    },
    highlightAvailabilityBtn() {
      this.$emit("highlightAvailabilityBtn")
    },
    editGuestAvailability(id) {
      if (this.authUser) {
        this.$emit("addAvailabilityAsGuest")
      } else {
        this.startEditing()
      }

      this.$nextTick(() => {
        this.populateUserAvailability(id)
        this.$emit("setCurGuestId", id)
      })
    },
    openEditGuestNameDialog() {
      this.newGuestName = this.curGuestId
      this.editGuestNameDialog = true
    },
    async saveGuestName() {
      const newName = this.newGuestName.trim()
      if (newName.length === 0) {
        this.showError("Guest name cannot be empty")
        return
      }
      if (newName === this.curGuestId) {
        this.editGuestNameDialog = false
        return
      }
      try {
        await post(`/events/${this.event._id}/rename-user`, {
          oldName: this.curGuestId,
          newName,
        })
        this.showInfo("Guest name updated successfully")
        this.editGuestNameDialog = false
        this.$emit("setCurGuestId", newName)
        this.refreshEvent()
      } catch (err) {
        const errorMessage =
          err.parsed?.error || err.message || "Failed to update guest name"
        this.showError(errorMessage)
      }
    },
    refreshEvent() {
      this.$emit("refreshEvent")
    },
    //#endregion

    // -----------------------------------
    //#region Schedule event
    // -----------------------------------
    scheduleEvent() {
      this.state = this.states.SCHEDULE_EVENT
      this.$posthog.capture("schedule_event_button_clicked")
    },
    cancelScheduleEvent() {
      this.state = this.defaultState
    },

    /** Redirect user to Google Calendar to finish the creation of the event */
    saveScheduleEvent() {
      if (!this.curScheduledEvent) return

      this.$posthog.capture("schedule_event_confirmed")
      // Get start date, and end date from the area that the user has dragged out
      const { col, row, numRows } = this.curScheduledEvent
      let startDate = this.getDateFromRowCol(row, col)
      let endDate = new Date(startDate)
      endDate.setMinutes(
        startDate.getMinutes() + this.timeslotDuration * numRows
      )

      if (this.isWeekly) {
        // Determine offset based on current day of the week.
        // People expect the event to be scheduled in the future, not the past, which is why this check exists
        let offset = 0
        if (new Date().getDay() > startDate.getDay()) {
          offset = 1
        }

        // Transform startDate and endDate to be the current week offset
        startDate = dateToDowDate(this.event.dates, startDate, offset, true)
        endDate = dateToDowDate(this.event.dates, endDate, offset, true)
      }

      const eventId = this.event.shortId ?? this.event._id

      // Persist the confirmed gathering to Gathering (time + recurrence +
      // reminder). No external calendar is opened here — members add it to
      // their own calendar afterwards via the "Add to calendar" (.ics) link on
      // the confirmed gathering, which carries the recurrence rule (RRULE).
      // Best-effort: silently ignores failure (e.g. not the owner).
      setScheduledEvent(eventId, {
        scheduled: true,
        startDate: startDate.toISOString(),
        endDate: endDate.toISOString(),
        summary: this.event.name,
        timezone: this.curTimezone.value,
        reminderEnabled: this.reminderEnabled,
        reminderLeadTimeHours: this.reminderLeadTimeHours,
        recurrenceFrequency: this.recurrenceFrequency,
        location: this.scheduleLocation.trim(),
      })
        .then(() => this.refreshEvent())
        .catch(() => {})

      this.state = this.defaultState
    },

    /** Cancel a previously-confirmed gathering (also drops its reminder) */
    cancelGathering() {
      const eventId = this.event.shortId ?? this.event._id
      setScheduledEvent(eventId, { scheduled: false })
        .then(() => this.refreshEvent())
        .catch(() => {})
    },
    //#endregion

    // -----------------------------------
    //#region Scroll
    // -----------------------------------
    onCalendarScroll(e) {
      this.calendarMaxScroll = e.target.scrollWidth - e.target.offsetWidth
      this.calendarScrollLeft = e.target.scrollLeft
    },
    onScroll(e) {
      this.checkElementsVisible()
    },
    /** Checks whether certain elements are visible and sets variables accoringly */
    checkElementsVisible() {
      const optionsSectionEl = this.$refs.optionsSection
      if (optionsSectionEl) {
        this.optionsVisible = isElementInViewport(optionsSectionEl, {
          bottomOffset: -64,
        })
      }

      const respondentsListEl = this.$refs.respondentsList?.$el
      if (respondentsListEl) {
        this.scrolledToRespondents = isElementInViewport(respondentsListEl, {
          bottomOffset: -64,
        })
      }
    },
    //#endregion

    // -----------------------------------
    //#region Pagination
    // -----------------------------------
    nextPage(e) {
      e.stopImmediatePropagation()
      this.page++
      this.pageHasChanged = true
    },
    prevPage(e) {
      e.stopImmediatePropagation()
      this.page--
      this.pageHasChanged = true
    },
    //#endregion

    // -----------------------------------
    //#region Resize
    // -----------------------------------
    onResize() {
      this.setTimeslotSize()
    },
    //#endregion

    // -----------------------------------
    //#region hint
    // -----------------------------------
    closeHint() {
      this.hintState = false
      localStorage[this.hintStateLocalStorageKey] = true
    },
    //#endregion

    // -----------------------------------
    //#region Sign up form
    // -----------------------------------

    /** Creates a sign up block for the current day and hour offset */
    createSignUpBlock(dayIndex, hoursOffset, hoursLength) {
      const timeBlock = getTimeBlock(
        this.days[dayIndex].dateObject,
        hoursOffset,
        hoursLength
      )

      return {
        _id: ObjectID().toString(),
        capacity: 1,
        name: this.newSignUpBlockName,
        ...timeBlock,
        hoursOffset,
        hoursLength,
      }
    },

    /** Updates the sign up block with the same id */
    editSignUpBlock(signUpBlock) {
      this.signUpBlocksByDay.forEach((blocksInDay, dayIndex) => {
        blocksInDay.forEach((block, blockIndex) => {
          if (signUpBlock._id === block._id) {
            this.signUpBlocksByDay[dayIndex][blockIndex] = signUpBlock
            this.signUpBlocksByDay = [...this.signUpBlocksByDay]
            return
          }
        })
      })

      this.signUpBlocksToAddByDay.forEach((blocksInDay, dayIndex) => {
        blocksInDay.forEach((block, blockIndex) => {
          if (signUpBlock._id === block._id) {
            this.signUpBlocksToAddByDay[dayIndex][blockIndex] = signUpBlock
            this.signUpBlocksToAddByDay = [...this.signUpBlocksToAddByDay]
            return
          }
        })
      })
    },

    /** Deletes the sign up block with the id */
    deleteSignUpBlock(signUpBlockId) {
      this.signUpBlocksByDay.forEach((blocksInDay, dayIndex) => {
        blocksInDay.forEach((block, blockIndex) => {
          if (signUpBlockId === block._id) {
            this.signUpBlocksByDay[dayIndex].splice(blockIndex, 1)
            return
          }
        })
      })

      this.signUpBlocksToAddByDay.forEach((blocksInDay, dayIndex) => {
        blocksInDay.forEach((block, blockIndex) => {
          if (signUpBlockId === block._id) {
            this.signUpBlocksToAddByDay[dayIndex].splice(blockIndex, 1)
            return
          }
        })
      })
    },

    /** Reloads all the data for the sign up form */
    resetSignUpForm() {
      /** Split sign up blocks by day */
      this.signUpBlocksByDay = splitTimeBlocksByDay(
        this.event,
        this.event.signUpBlocks ?? []
      )

      this.resetSignUpBlocksToAddByDay()

      /** Populate sign up block responses (confirmed) + waitlist */
      const allBlocks = this.signUpBlocksByDay.flat()
      for (const userId in this.event.signUpResponses) {
        const signUpResponse = this.event.signUpResponses[userId]
        for (const signUpBlockId of signUpResponse.signUpBlockIds ?? []) {
          const signUpBlock = allBlocks.find((b) => b._id === signUpBlockId)
          if (!signUpBlock) continue
          if (!signUpBlock.responses) signUpBlock.responses = []
          signUpBlock.responses.push(signUpResponse)
        }
        for (const signUpBlockId of signUpResponse.waitlistBlockIds ?? []) {
          const signUpBlock = allBlocks.find((b) => b._id === signUpBlockId)
          if (!signUpBlock) continue
          if (!signUpBlock.waitlist) signUpBlock.waitlist = []
          signUpBlock.waitlist.push(signUpResponse)
        }
      }
    },

    /** Initialize sign up blocks to be added array */
    resetSignUpBlocksToAddByDay() {
      this.signUpBlocksToAddByDay = []
      for (const day of this.signUpBlocksByDay) {
        this.signUpBlocksToAddByDay.push([])
      }
    },

    /** Emits sign up for block to parent element. A full block is still
     * clickable — the server waitlists the signup (C9). */
    handleSignUpBlockClick(block) {
      if (!this.alreadyRespondedToSignUpForm && !this.isOwner)
        this.$emit("signUpForBlock", block)
    },

    //#endregion

    // -----------------------------------
    //#region Specific times for specific days
    // -----------------------------------

    /** Saves the temporary times to the event */
    saveTempTimes() {
      // Set event times
      this.event.times = [...this.tempTimes]
        .map((t) => new Date(t))
        .sort((a, b) => a.getTime() - b.getTime())

      const { minHours, maxHours } = this.getMinMaxHoursFromTimes(
        this.event.times
      )

      // Set event dates to start at the new times
      for (let i = 0; i < this.event.dates.length; ++i) {
        const date = new Date(this.event.dates[i])
        date.setTime(date.getTime() - this.timezoneOffset * 60 * 1000)
        date.setUTCHours(minHours, 0, 0, 0)
        date.setTime(date.getTime() + this.timezoneOffset * 60 * 1000)
        this.event.dates[i] = date.toISOString()
      }

      // Set event duration to the difference between the max and min hours
      this.event.duration = maxHours - minHours + 1

      // Fix other fields
      if (this.event.remindees) {
        this.event.remindees = this.event.remindees.map((r) => r.email)
      }

      // Update event
      put(`/events/${this.event._id}`, this.event)
        .then(() => {
          this.state = this.defaultState
        })
        .catch((err) => {
          this.showError(err)
        })
    },

    /** Returns the min and max hours from the times */
    getMinMaxHoursFromTimes(times) {
      let minHours = 24
      let maxHours = 0
      for (const time of times) {
        const timeDate = new Date(time)
        const date = new Date(
          timeDate.getTime() - this.timezoneOffset * 60 * 1000
        )
        const localHours = date.getUTCHours()
        if (localHours < minHours) {
          minHours = localHours
        } else if (localHours > maxHours) {
          maxHours = localHours
        }
      }
      return { minHours, maxHours }
    },

    //#endregion

    /** Recalculate availability the calendar based on calendar events */
    reanimateAvailability() {
      if (
        this.state === this.states.EDIT_AVAILABILITY &&
        this.authUser &&
        !(this.authUser?._id in this.event.responses) && // User hasn't responded yet
        !this.loadingCalendarEvents &&
        (!this.unsavedChanges || this.availabilityAnimEnabled)
      ) {
        for (const timeout of this.availabilityAnimTimeouts) {
          clearTimeout(timeout)
        }
        this.setAvailabilityAutomatically()
      }
    },
  },
  watch: {
    availability() {
      if (this.state === this.states.EDIT_AVAILABILITY) {
        this.unsavedChanges = true
      }
    },
    event: {
      immediate: true,
      handler(newEvent, oldEvent) {
        this.fetchResponses()

        // The venue can also be set from the event page's inline editor, which
        // patches the event in place. Follow it, or confirming a time would
        // send the value seeded when this component was created and wipe it.
        this.scheduleLocation = nextScheduleLocation(
          newEvent?.location,
          oldEvent?.location,
          this.scheduleLocation
        )
      },
    },
    state(nextState, prevState) {
      this.$nextTick(() => this.checkElementsVisible())

      // Reset scheduled event when exiting schedule event state
      if (prevState === this.states.SCHEDULE_EVENT) {
        this.curScheduledEvent = null
      } else if (prevState === this.states.EDIT_AVAILABILITY) {
        this.unsavedChanges = false
      }

      if (nextState === this.states.SET_SPECIFIC_TIMES) {
        this.$nextTick(() => {
          const time9 = document.getElementById("time-9")
          if (time9) {
            const yOffset = -150
            const y =
              time9.getBoundingClientRect().top + window.scrollY + yOffset
            window.scrollTo({ top: y, behavior: "smooth" })
          }
        })
      }
    },
    respondents: {
      immediate: true,
      handler() {
        this.curTimeslotAvailability = {}
        for (const respondent of this.respondents) {
          this.curTimeslotAvailability[respondent._id] = true
        }
      },
    },
    calendarEventsByDay(val, oldVal) {
      if (JSON.stringify(val) !== JSON.stringify(oldVal)) {
        this.reanimateAvailability()
      }
    },
    page() {
      this.$nextTick(() => {
        this.setTimeslotSize()
      })
    },
    allDays() {
      this.$nextTick(() => {
        this.setTimeslotSize()
      })
    },
    showStickyRespondents: {
      immediate: true,
      handler(cur) {
        clearTimeout(this.delayedShowStickyRespondentsTimeout)
        this.delayedShowStickyRespondentsTimeout = setTimeout(() => {
          this.delayedShowStickyRespondents = cur
        }, 100)
      },
    },
    maxDaysPerPage() {
      // Set page to 0 if user switches from portrait to landscape orientation and we're on an invalid page number,
      // i.e. we're on a page that displays 0 days
      if (this.page * this.maxDaysPerPage >= this.allDays.length) {
        this.page = 0
      }
    },
    mobileNumDays() {
      // Save mobile num days in localstorage
      localStorage["mobileNumDays"] = this.mobileNumDays

      // Set timeslot size because it has changed
      this.$nextTick(() => {
        this.setTimeslotSize()
      })
    },
    hideIfNeeded() {
      this.getResponsesFormatted()
    },
    parsedResponses() {
      this.getResponsesFormatted()
    },
    showBestTimes() {
      this.onShowBestTimesChange()
    },
    startCalendarOnMonday() {
      localStorage["startCalendarOnMonday"] = this.startCalendarOnMonday
    },
    showResponseCounts() {
      localStorage["showResponseCounts"] = this.showResponseCounts
    },
    bufferTime(cur, prev) {
      if (cur.enabled !== prev.enabled || cur.enabled) {
        this.reanimateAvailability()
      }
    },
    workingHours(cur, prev) {
      if (cur.enabled !== prev.enabled || cur.enabled) {
        this.reanimateAvailability()
      }
    },
    timeType() {
      localStorage["timeType"] = this.timeType
    },
    fromEditEvent() {
      if (this.fromEditEvent && this.isSpecificTimes) {
        this.tempTimes = new Set(
          this.event.times.map((t) => new Date(t).getTime())
        )
        this.state = this.states.SET_SPECIFIC_TIMES
      }
    },
  },
  created() {
    this.resetCurUserAvailability()

    addEventListener("click", this.deselectRespondents)
  },
  mounted() {
    // Get query parameters from URL
    const urlParams = new URLSearchParams(window.location.search)

    // Set initial state
    if (
      this.event.hasSpecificTimes &&
      (this.fromEditEvent || !this.event.times || this.event.times.length === 0)
    ) {
      this.state = this.states.SET_SPECIFIC_TIMES
    } else if (urlParams.get("scheduled_event")) {
      const scheduledEvent = JSON.parse(urlParams.get("scheduled_event"))
      this.curScheduledEvent = scheduledEvent
      this.state = this.states.SCHEDULE_EVENT

      // Remove the scheduled_event parameter from URL to avoid reloading the same state
      const newUrl = new URL(window.location.href)
      newUrl.searchParams.delete("scheduled_event")
      window.history.replaceState({}, document.title, newUrl.toString())
    } else if (this.showBestTimes) {
      this.state = "best_times"
    } else {
      this.state = "heatmap"
    }

    // Set calendar options defaults
    if (this.authUser) {
      this.bufferTime =
        this.authUser?.calendarOptions?.bufferTime ??
        calendarOptionsDefaults.bufferTime
      this.workingHours =
        this.authUser?.calendarOptions?.workingHours ??
        calendarOptionsDefaults.workingHours
    }

    // Set initial calendar max scroll
    // this.calendarMaxScroll =
    //   this.$refs.calendar.scrollWidth - this.$refs.calendar.offsetWidth

    // Get timeslot size
    this.setTimeslotSize()
    addEventListener("resize", this.onResize)
    addEventListener("scroll", this.onScroll)

    // Watch for layout changes (e.g. ads loading, flex reflows) that resize
    // the grid without triggering a window resize event
    const dragSection = document.getElementById("drag-section")
    if (dragSection) {
      this._resizeObserver = new ResizeObserver(() => {
        this.setTimeslotSize()
      })
      this._resizeObserver.observe(dragSection)
    }

    if (!this.calendarOnly) {
      const timesEl = dragSection
      if (timesEl && isTouchEnabled()) {
        timesEl.addEventListener("touchstart", this.startDrag)
        timesEl.addEventListener("touchmove", this.moveDrag)
        timesEl.addEventListener("touchend", this.endDrag)
        timesEl.addEventListener("touchcancel", this.endDrag)
      }
      if (timesEl) {
        timesEl.addEventListener("mousedown", this.startDrag)
        timesEl.addEventListener("mousemove", this.moveDrag)
        timesEl.addEventListener("mouseup", this.endDrag)
      }
    }

    // Parse sign up blocks and responses
    this.resetSignUpForm()
  },
  beforeDestroy() {
    removeEventListener("click", this.deselectRespondents)
    removeEventListener("resize", this.onResize)
    removeEventListener("scroll", this.onScroll)
    if (this._resizeObserver) {
      this._resizeObserver.disconnect()
    }
  },
  components: {
    AlertText,
    AvailabilityTypeToggle,
    ColorLegend,
    ExpandableSection,
    BufferTimeSwitch,
    UserAvatarContent,
    ZigZag,
    ConfirmDetailsDialog,
    ToolRow,
    CalendarAccounts,
    RespondentsList,
    GCalWeekSelector,
    WorkingHoursToggle,
    SignUpBlock,
    SignUpBlocksOverlay,
    SignUpBlocksList,
    CalendarEventBlock, // Added component registration
    SpecificTimesInstructions, // Added component registration
    Tooltip,
  },
}
</script>
