<template>
  <span>
    <Tooltip :content="tooltipContent">
      <!-- Collapsed once a gathering is confirmed: the grid, the tool row and
           the phone bottom stack all go, leaving the page to the discussion and
           the lists. v-if rather than v-show — tailwind.config.js sets
           `important: true`, so tw-flex on the row below would beat the inline
           display:none that v-show sets. The component itself stays mounted:
           Event.vue reads ~8 computeds through this instance. -->
      <div
        v-if="!collapsed"
        class="tw-select-none tw-py-4"
        style="-webkit-touch-callout: none"
      >
        <div class="tw-flex tw-flex-col sm:tw-flex-row">
          <div class="tw-flex tw-grow tw-pl-4 tw-pr-4">
            <template v-if="event.daysOnly">
              <div class="tw-grow">
                <div class="tw-flex tw-items-center tw-justify-between">
                  <v-btn
                    :class="hasPrevPage ? 'tw-visible' : 'tw-invisible'"
                    class="tw-border-brass-dim"
                    variant="outlined"
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
                    class="tw-border-brass-dim"
                    variant="outlined"
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
                        <v-icon size="small">mdi-information-outline</v-icon>
                        {{ hintText }}
                      </div>
                      <v-icon size="small" @click="closeHint">mdi-close</v-icon>
                    </div>
                  </div>
                </v-expand-transition>

                <ToolRow
                  v-if="!isPhone && !calendarOnly"
                  :event="event"
                  :state="state"
                  :states="states"
                  v-model:cur-timezone="curTimezone"
                  :timezone-reference-date="timezoneReferenceDate"
                  v-model:show-best-times="showBestTimes"
                  v-model:hide-if-needed="hideIfNeeded"
                  :is-weekly="isWeekly"
                  :calendar-permission-granted="calendarPermissionGranted"
                  :week-offset="weekOffset"
                  :num-responses="respondents.length"
                  v-model:mobile-num-days="mobileNumDays"
                  :allow-schedule-event="allowScheduleEvent"
                  :show-event-options="showEventOptions"
                  v-model:time-type="timeType"
                  @toggleShowEventOptions="toggleShowEventOptions"
                  @update:weekOffset="(val) => $emit('update:weekOffset', val)"
                  @scheduleEvent="scheduleEvent"
                  @cancelScheduleEvent="cancelScheduleEvent"
                  @saveScheduleEvent="saveScheduleEvent"
                  @cancelGathering="cancelGathering"
                  v-model:reminder-enabled="reminderEnabled"
                  v-model:reminder-lead-time-hours="reminderLeadTimeHours"
                  v-model:recurrence-frequency="recurrenceFrequency"
                  v-model:location="scheduleLocation"
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
                      class="tw-border-brass-dim"
                      variant="outlined"
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
                    <!-- One key on the <template>, not one per child: Vue 3
                         keys the iterated template itself, so the gap spacer
                         and the day column are a single keyed unit. -->
                    <template v-for="(day, i) in days" :key="i">
                      <div
                        v-if="!day.isConsecutive"
                        :style="{ width: `${SPLIT_GAP_WIDTH}px` }"
                      ></div>
                      <div class="tw-flex-1 tw-bg-wood-deep">
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

                        <template v-for="(day, d) in days" :key="d">
                          <div
                            v-if="!day.isConsecutive"
                            :style="{ width: `${SPLIT_GAP_WIDTH}px` }"
                          ></div>
                          <div
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
                                :key="calendarEvent.id"
                              >
                                <CalendarEventBlock
                                  :blockStyle="getTimeBlockStyle(calendarEvent)"
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
                        <v-icon size="small">mdi-information-outline</v-icon>
                        {{ hintText }}
                      </div>
                      <v-icon size="small" @click="closeHint">mdi-close</v-icon>
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
                  v-model:cur-timezone="curTimezone"
                  :timezone-reference-date="timezoneReferenceDate"
                  v-model:show-best-times="showBestTimes"
                  v-model:hide-if-needed="hideIfNeeded"
                  :is-weekly="isWeekly"
                  :calendar-permission-granted="calendarPermissionGranted"
                  :week-offset="weekOffset"
                  :num-responses="respondents.length"
                  v-model:mobile-num-days="mobileNumDays"
                  :allow-schedule-event="allowScheduleEvent"
                  :show-event-options="showEventOptions"
                  v-model:time-type="timeType"
                  @toggleShowEventOptions="toggleShowEventOptions"
                  @update:weekOffset="(val) => $emit('update:weekOffset', val)"
                  @scheduleEvent="scheduleEvent"
                  @cancelScheduleEvent="cancelScheduleEvent"
                  @saveScheduleEvent="saveScheduleEvent"
                  @cancelGathering="cancelGathering"
                  v-model:reminder-enabled="reminderEnabled"
                  v-model:reminder-lead-time-hours="reminderLeadTimeHours"
                  v-model:recurrence-frequency="recurrenceFrequency"
                  v-model:location="scheduleLocation"
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
                  <v-btn
                    class="tw-border-brass-dim"
                    variant="outlined"
                    icon
                    @click="nextPage"
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
            <template v-if="state === states.SET_SPECIFIC_TIMES">
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
                    <v-icon size="small">mdi-pencil</v-icon>
                  </div>
                  <span v-else>
                    {{
                      authUser && !addingAvailabilityAsGuest
                        ? displayName(authUser)
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
                        <v-btn
                          variant="text"
                          @click="editGuestNameDialog = false"
                          >Cancel</v-btn
                        >
                        <v-btn
                          variant="text"
                          color="primary"
                          @click="saveGuestName"
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
                  @calendarsChanged="$emit('calendarsChanged')"
                ></CalendarAccounts>

                <div v-if="showOverlayAvailabilityToggle">
                  <v-switch
                    id="overlay-availabilities-toggle"
                    inset
                    :model-value="overlayAvailability"
                    @update:model-value="updateOverlayAvailability"
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
                    :model-value="showEditOptions"
                    @update:model-value="toggleShowEditOptions"
                  >
                    <div class="tw-flex tw-flex-col tw-gap-5 tw-pt-2.5">
                      <v-dialog
                        v-if="showCalendarOptions"
                        v-model="calendarOptionsDialog"
                        width="500"
                      >
                        <template v-slot:activator="{ props }">
                          <v-btn
                            variant="outlined"
                            class="tw-border-brass-dim tw-text-sm"
                            v-bind="props"
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
                            <BufferTimeSwitch v-model:bufferTime="bufferTime" />

                            <WorkingHoursToggle
                              v-model:workingHours="workingHours"
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
                    <template v-slot:activator="{ props }">
                      <span
                        v-bind="props"
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
                        <v-btn
                          variant="text"
                          @click="deleteAvailabilityDialog = false"
                          >Cancel</v-btn
                        >
                        <v-btn
                          variant="text"
                          color="error"
                          @click="confirmDeleteAvailability"
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
                  v-model:showCalendarEvents="showCalendarEvents"
                  :hasCalendarEvents="hasCalendarEvents"
                  :responsesFormatted="responsesFormatted"
                  :timezone="curTimezone"
                  v-model:show-best-times="showBestTimes"
                  v-model:hide-if-needed="hideIfNeeded"
                  v-model:show-response-counts="showResponseCounts"
                  v-model:start-calendar-on-monday="startCalendarOnMonday"
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
          v-model:cur-timezone="curTimezone"
          :timezone-reference-date="timezoneReferenceDate"
          v-model:show-best-times="showBestTimes"
          v-model:hide-if-needed="hideIfNeeded"
          v-model:start-calendar-on-monday="startCalendarOnMonday"
          :is-weekly="isWeekly"
          :calendar-permission-granted="calendarPermissionGranted"
          :week-offset="weekOffset"
          :num-responses="respondents.length"
          v-model:mobile-num-days="mobileNumDays"
          :allow-schedule-event="allowScheduleEvent"
          :show-event-options="showEventOptions"
          v-model:time-type="timeType"
          @toggleShowEventOptions="toggleShowEventOptions"
          @update:weekOffset="(val) => $emit('update:weekOffset', val)"
          @scheduleEvent="scheduleEvent"
          @cancelScheduleEvent="cancelScheduleEvent"
          @saveScheduleEvent="saveScheduleEvent"
          @cancelGathering="cancelGathering"
          v-model:reminder-enabled="reminderEnabled"
          v-model:reminder-lead-time-hours="reminderLeadTimeHours"
          v-model:recurrence-frequency="recurrenceFrequency"
          v-model:location="scheduleLocation"
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
                    <v-icon size="small">mdi-information-outline</v-icon>
                    <div>
                      {{ hintText }}
                    </div>
                  </div>
                  <v-icon size="small" @click="closeHint">mdi-close</v-icon>
                </div>
              </div>
            </template>
          </v-expand-transition>

          <!-- Fixed pos availability toggle (mobile) -->
          <v-expand-transition>
            <div v-if="editing">
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
                  v-model:showCalendarEvents="showCalendarEvents"
                  :hasCalendarEvents="hasCalendarEvents"
                  :responsesFormatted="responsesFormatted"
                  :timezone="curTimezone"
                  v-model:show-best-times="showBestTimes"
                  v-model:hide-if-needed="hideIfNeeded"
                  v-model:show-response-counts="showResponseCounts"
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
  getDateHoursOffset,
  post,
  put,
  isPhone,
  dateToDowDate,
  isTouchEnabled,
  isElementInViewport,
  userPrefers12h,
  prefersStartOnMonday,
  displayName,
} from "@/utils"
import {
  availabilityTypes,
  calendarOptionsDefaults,
  eventTypes,
  isOwnerlessEvent,
  timeTypes,
} from "@/constants"
import { setScheduledEvent } from "@/utils/services/EventService"
import { nextScheduleLocation } from "./scheduleLocation"
import { mapMutations, mapActions, mapState, mapGetters } from "vuex"
import CalendarAccounts from "@/components/settings/CalendarAccounts.vue"
import ZigZag from "./ZigZag.vue"
import ToolRow from "./ToolRow.vue"
import RespondentsList from "./RespondentsList.vue"
import GCalWeekSelector from "./GCalWeekSelector.vue"
import ExpandableSection from "../ExpandableSection.vue"
import WorkingHoursToggle from "./WorkingHoursToggle.vue"
import Tooltip from "../Tooltip.vue"
import ColorLegend from "./ColorLegend.vue"

import dayjs from "dayjs"
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
import calendarDaysMixin from "./calendarDaysMixin"
import timeGridMixin from "./timeGridMixin"
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
    calendarDaysMixin,
    timeGridMixin,
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
    collapsed: { type: Boolean, default: false }, // Whether to render nothing at all (gathering already scheduled) while staying mounted
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
      return "13rem"
    },
    /** Only allow scheduling when a curScheduledEvent exists */
    allowScheduleEvent() {
      return !!this.curScheduledEvent
    },
    allowDrag() {
      return (
        this.state === this.states.EDIT_AVAILABILITY ||
        this.state === this.states.SCHEDULE_EVENT ||
        this.state === this.states.SET_SPECIFIC_TIMES
      )
    },
    defaultState() {
      // Either the heatmap or the best_times state, depending on the toggle
      return this.showBestTimes ? this.states.BEST_TIMES : this.states.HEATMAP
    },
    editing() {
      // Returns whether currently in the editing state
      return this.state === this.states.EDIT_AVAILABILITY
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
    isSpecificTimes() {
      return this.event.hasSpecificTimes
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
    showLeftZigZag() {
      return this.calendarScrollLeft > 0
    },
    showRightZigZag() {
      return Math.ceil(this.calendarScrollLeft) < this.calendarMaxScroll
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

    /**
     * Whether this member has any calendar events in the range on screen.
     *
     * Gates the "Show my calendar events" switch: without a linked calendar
     * there is nothing to draw, and a switch that visibly does nothing is worse
     * than no switch at all.
     */
    hasCalendarEvents() {
      return (this.calendarEventsByDay ?? []).some((day) => day?.length > 0)
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
  },
  methods: {
    ...mapMutations(["setAuthUser"]),
    ...mapActions(["showInfo", "showError"]),
    displayName,

    /**
     * Confirm the delete-availability dialog: fire the event, then close.
     *
     * Extracted from an inline two-statement `@click`: Vue 3's template
     * compiler parses a handler as a single JavaScript *expression*, so two
     * statements separated by a newline no longer compile.
     */
    confirmDeleteAvailability() {
      this.$emit("deleteAvailability")
      this.deleteAvailabilityDialog = false
    },

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
        // Half-hour timezones used to fail every one of these lookups: the
        // grid was shifted 30 minutes off the stored instants, so `date` was
        // never in the set. splitTimes no longer shifts this view — see
        // gridTimeOffset.
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
      this.state = this.states.EDIT_AVAILABILITY
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
    //#region Grid interactions
    // -----------------------------------
    /**
     * Attach everything that hangs off the #drag-section element: the
     * ResizeObserver that keeps timeslot geometry honest through layout changes
     * no window resize fires for, and the pointer listeners that drive
     * drag-to-select.
     *
     * Called from mounted() and again whenever `collapsed` goes false, because
     * a page that first rendered collapsed had no #drag-section to bind to —
     * without the re-bind, dragging on the re-expanded grid silently does
     * nothing.
     */
    bindGridInteractions() {
      const dragSection = document.getElementById("drag-section")
      if (!dragSection) return
      this._gridEl = dragSection

      this._resizeObserver = new ResizeObserver(() => {
        this.setTimeslotSize()
      })
      this._resizeObserver.observe(dragSection)

      if (this.calendarOnly) return
      if (isTouchEnabled()) {
        dragSection.addEventListener("touchstart", this.startDrag)
        dragSection.addEventListener("touchmove", this.moveDrag)
        dragSection.addEventListener("touchend", this.endDrag)
        dragSection.addEventListener("touchcancel", this.endDrag)
      }
      dragSection.addEventListener("mousedown", this.startDrag)
      dragSection.addEventListener("mousemove", this.moveDrag)
      dragSection.addEventListener("mouseup", this.endDrag)
    },

    /** Undo bindGridInteractions, against the element it actually bound to. */
    unbindGridInteractions() {
      if (this._resizeObserver) {
        this._resizeObserver.disconnect()
        this._resizeObserver = null
      }

      const dragSection = this._gridEl
      if (!dragSection) return
      this._gridEl = null

      dragSection.removeEventListener("touchstart", this.startDrag)
      dragSection.removeEventListener("touchmove", this.moveDrag)
      dragSection.removeEventListener("touchend", this.endDrag)
      dragSection.removeEventListener("touchcancel", this.endDrag)
      dragSection.removeEventListener("mousedown", this.startDrag)
      dragSection.removeEventListener("mousemove", this.moveDrag)
      dragSection.removeEventListener("mouseup", this.endDrag)
    },
    //#endregion

    // -----------------------------------
    //#region Schedule event
    // -----------------------------------
    scheduleEvent() {
      this.state = this.states.SCHEDULE_EVENT
    },
    cancelScheduleEvent() {
      this.state = this.defaultState
    },

    /** Redirect user to Google Calendar to finish the creation of the event */
    saveScheduleEvent() {
      if (!this.curScheduledEvent) return

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
    onScroll() {
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
    //#region Specific times for specific days
    // -----------------------------------

    /**
     * Saves the temporary times to the event.
     *
     * Writes straight through the `event` prop: the parent shares the same
     * object, so this is how the edited times reach it — there is no
     * update event to emit. Untangling it belongs with the split of this
     * component (TODO2 G2), not with a lint pass.
     */
    /* eslint-disable vue/no-mutating-props */
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
    /* eslint-enable vue/no-mutating-props */

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
    /**
     * The grid is torn out of the DOM while collapsed, taking #drag-section
     * with it, so its listeners have to be re-attached to the new element on
     * the way back. setTimeslotSize() too: geometry last measured against a
     * grid that wasn't rendered is meaningless.
     */
    collapsed(isCollapsed) {
      this.unbindGridInteractions()
      if (isCollapsed) return

      this.$nextTick(() => {
        this.bindGridInteractions()
        this.setTimeslotSize()
      })
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

    this.bindGridInteractions()

    // Parse sign up blocks and responses
  },
  beforeUnmount() {
    this.unbindGridInteractions()
    removeEventListener("click", this.deselectRespondents)
    removeEventListener("resize", this.onResize)
    removeEventListener("scroll", this.onScroll)
  },
  components: {
    AvailabilityTypeToggle,
    ColorLegend,
    ExpandableSection,
    BufferTimeSwitch,
    ZigZag,
    ToolRow,
    CalendarAccounts,
    RespondentsList,
    GCalWeekSelector,
    WorkingHoursToggle,
    CalendarEventBlock, // Added component registration
    SpecificTimesInstructions, // Added component registration
    Tooltip,
  },
}
</script>
