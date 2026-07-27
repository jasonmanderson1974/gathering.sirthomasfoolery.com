package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EventType string

const (
	SPECIFIC_DATES EventType = "specific_dates"
	DOW            EventType = "dow"
)

// Object containing information associated with the remindee
type Remindee struct {
	Email     string   `json:"email" bson:"email,omitempty"`
	TaskIds   []string `json:"-" bson:"taskIds,omitempty"` // Task IDs of the scheduled emails
	Responded *bool    `json:"responded" bson:"responded,omitempty"`
}

// Configuration + bookkeeping for the pre-gathering reminder email that fires
// once, LeadTimeHours before a confirmed gathering's start (see ScheduledEvent).
// The in-process reminder scheduler (services/reminders) reads these fields.
type GatheringReminder struct {
	Enabled       bool                `json:"enabled" bson:"enabled"`
	LeadTimeHours int                 `json:"leadTimeHours" bson:"leadTimeHours,omitempty"`
	Timezone      string              `json:"timezone" bson:"timezone,omitempty"` // IANA tz for formatting the email time (e.g. "America/Los_Angeles")
	SentAt        *primitive.DateTime `json:"sentAt" bson:"sentAt,omitempty"`     // nil = not yet sent
}

// RecurrenceFrequency is how often a confirmed gathering repeats (C5).
type RecurrenceFrequency string

const (
	RecurrenceNone     RecurrenceFrequency = ""
	RecurrenceWeekly   RecurrenceFrequency = "weekly"
	RecurrenceBiweekly RecurrenceFrequency = "biweekly"
	RecurrenceMonthly  RecurrenceFrequency = "monthly"
)

// GatheringRecurrence makes a confirmed gathering repeat (C5). It drives two
// things: the .ics RRULE (so a single "add to calendar" covers the whole
// series in members' calendars) and the in-process scheduler that rolls
// ScheduledEvent forward to the next occurrence once the current one ends
// (services/reminders.advanceRecurringGatherings). Paired with ScheduledEvent.
type GatheringRecurrence struct {
	Frequency RecurrenceFrequency `json:"frequency" bson:"frequency"`
	// Until, when set, is the latest date an occurrence may START on; the series
	// stops advancing once the next occurrence would fall after it. nil = no end.
	Until *primitive.DateTime `json:"until" bson:"until,omitempty"`
}

// IsRecurring reports whether this is a real, advanceable recurrence.
func (r *GatheringRecurrence) IsRecurring() bool {
	if r == nil {
		return false
	}
	switch r.Frequency {
	case RecurrenceWeekly, RecurrenceBiweekly, RecurrenceMonthly:
		return true
	default:
		return false
	}
}

// Step advances t by one interval of the recurrence, preserving time-of-day.
// Monthly keeps the same day-of-month, clamping to the last valid day for
// short months (see addMonthsClamped). Returns t unchanged for a non-recurring
// frequency — callers must gate on IsRecurring.
func (r *GatheringRecurrence) Step(t time.Time) time.Time {
	switch r.Frequency {
	case RecurrenceWeekly:
		return t.AddDate(0, 0, 7)
	case RecurrenceBiweekly:
		return t.AddDate(0, 0, 14)
	case RecurrenceMonthly:
		return addMonthsClamped(t, 1)
	default:
		return t
	}
}

// NextOccurrenceAfter returns the first occurrence start strictly after `after`,
// stepping from `start` by the frequency. Returns the zero time if this is not
// a recurring gathering. Bounded so pathological input can't loop forever.
func (r *GatheringRecurrence) NextOccurrenceAfter(start, after time.Time) time.Time {
	if !r.IsRecurring() {
		return time.Time{}
	}
	next := start
	for i := 0; i < 10000 && !next.After(after); i++ {
		next = r.Step(next)
	}
	if !next.After(after) {
		return time.Time{}
	}
	return next
}

// RRULE renders the iCalendar RRULE string for this recurrence (RFC 5545),
// e.g. "FREQ=WEEKLY", "FREQ=WEEKLY;INTERVAL=2", "FREQ=MONTHLY", optionally with
// ";UNTIL=<UTC>". Returns "" when not recurring. Note: for monthly gatherings on
// day 29–31 the server's advance clamps to the month's last day, which can
// diverge from a strict RRULE reader — fine for this club (meetings fall on
// normal days); see addMonthsClamped.
func (r *GatheringRecurrence) RRULE() string {
	if r == nil {
		return ""
	}
	var base string
	switch r.Frequency {
	case RecurrenceWeekly:
		base = "FREQ=WEEKLY"
	case RecurrenceBiweekly:
		base = "FREQ=WEEKLY;INTERVAL=2"
	case RecurrenceMonthly:
		base = "FREQ=MONTHLY"
	default:
		return ""
	}
	if r.Until != nil {
		base += ";UNTIL=" + r.Until.Time().UTC().Format("20060102T150405Z")
	}
	return base
}

// addMonthsClamped adds n calendar months to t, preserving time-of-day and
// clamping the day to the target month's last day (so Jan 31 + 1 month lands on
// Feb 28/29, not Mar 3 as time.AddDate would normalize it).
func addMonthsClamped(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	// time.Date normalizes an out-of-range month (e.g. 13 -> next Jan), so this
	// is safe across year boundaries.
	first := time.Date(y, m+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	daysInMonth := first.AddDate(0, 1, -1).Day()
	if d > daysInMonth {
		d = daysInMonth
	}
	return time.Date(first.Year(), first.Month(), d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// RecurrenceLabel is a short human label for a frequency (used in emails / logs).
func RecurrenceLabel(f RecurrenceFrequency) string {
	switch f {
	case RecurrenceWeekly:
		return "weekly"
	case RecurrenceBiweekly:
		return "every 2 weeks"
	case RecurrenceMonthly:
		return "monthly"
	default:
		return ""
	}
}

// RSVP to a confirmed gathering (paired with ScheduledEvent). Stored on the
// Event as a map keyed by guest name or signed-in user id, mirroring
// SignUpResponses.
type RsvpStatus string

const (
	RsvpGoing RsvpStatus = "going"
	RsvpMaybe RsvpStatus = "maybe"
	RsvpNo    RsvpStatus = "no"
)

type Rsvp struct {
	Status RsvpStatus `json:"status" bson:"status"`
	// GuestCount is the number of ADDITIONAL people this responder is bringing
	// (a spouse/plus-one), i.e. the headcount for this RSVP is 1 + GuestCount.
	// Only meaningful for going/maybe.
	GuestCount  int                `json:"guestCount" bson:"guestCount,omitempty"`
	Name        string             `json:"name" bson:"name,omitempty"`
	Email       string             `json:"email" bson:"email,omitempty"`
	UserId      primitive.ObjectID `json:"userId" bson:"userId,omitempty"`
	RespondedAt primitive.DateTime `json:"respondedAt" bson:"respondedAt,omitempty"`
}

// Poll is a lightweight multiple-choice poll on an event (C6) — e.g. "Where
// should we meet?" or "What should we do?". The owner creates it; members and
// guests vote. Votes live on each option (keyed by responder) so counts + the
// voter roster render straight from the event with no extra fetch. Stored as an
// array on the Event, mirroring SignUpBlocks.
type Poll struct {
	Id    primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Title string             `json:"title" bson:"title,omitempty"`
	// AllowMultiple lets a voter pick more than one option (else single-choice).
	AllowMultiple bool         `json:"allowMultiple" bson:"allowMultiple,omitempty"`
	Options       []PollOption `json:"options" bson:"options,omitempty"`
}

type PollOption struct {
	Id    primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Label string             `json:"label" bson:"label,omitempty"`
	// Votes maps a responder key (guest name / signed-in user-id hex) to that
	// voter's display name — so a count is len(Votes) and the roster is its values.
	Votes map[string]string `json:"votes" bson:"votes,omitempty"`
}

type SignUpBlock struct {
	Id        primitive.ObjectID  `json:"_id" bson:"_id,omitempty"`
	Name      string              `json:"name" bson:"name,omitempty"`
	Capacity  *int                `json:"capacity" bson:"capacity,omitempty"`
	StartDate *primitive.DateTime `json:"startDate" bson:"startDate,omitempty"`
	EndDate   *primitive.DateTime `json:"endDate" bson:"endDate,omitempty"`
}

type SignUpResponse struct {
	// The IDs of the sign up blocks the user is CONFIRMED for (within capacity)
	SignUpBlockIds []primitive.ObjectID `json:"signUpBlockIds" bson:"signUpBlockIds,omitempty"`

	// The IDs of the sign up blocks the user is WAITLISTED for (block was full).
	// Assigned server-side by capacity; see assignSignUpBlocks (C9).
	WaitlistBlockIds []primitive.ObjectID `json:"waitlistBlockIds" bson:"waitlistBlockIds,omitempty"`

	// Guest information
	Name  string `json:"name" bson:"name,omitempty"`
	Email string `json:"email" bson:"email,omitempty"`

	// User information
	UserId primitive.ObjectID `json:"userId" bson:"userId,omitempty"`
	User   *User              `json:"user" bson:",omitempty"`
}

// Representation of an Event in the mongoDB database
type Event struct {
	Id          primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	ShortId     *string            `json:"shortId" bson:"shortId,omitempty"`
	OwnerId     primitive.ObjectID `json:"ownerId" bson:"ownerId,omitempty"`
	Name        string             `json:"name" bson:"name,omitempty"`
	Description *string            `json:"description" bson:"description,omitempty"`
	// Free-text venue/address for the gathering (C12). Surfaced on the event
	// page, in the .ics LOCATION, and in the reminder email.
	Location   *string `json:"location" bson:"location,omitempty"`
	IsArchived *bool   `json:"isArchived" bson:"isArchived,omitempty"`
	IsDeleted  *bool   `json:"isDeleted" bson:"isDeleted,omitempty"`

	Duration                 *float32             `json:"duration" bson:"duration,omitempty"`
	Dates                    []primitive.DateTime `json:"dates" bson:"dates,omitempty"`
	NotificationsEnabled     *bool                `json:"notificationsEnabled" bson:"notificationsEnabled,omitempty"`
	SendEmailAfterXResponses *int                 `json:"sendEmailAfterXResponses" bson:"sendEmailAfterXResponses,omitempty"`
	When2meetHref            *string              `json:"when2meetHref" bson:"when2meetHref,omitempty"`
	CollectEmails            *bool                `json:"collectEmails" bson:"collectEmails,omitempty"`
	TimeIncrement            *int                 `json:"timeIncrement" bson:"timeIncrement,omitempty"`

	// Used for specific times for specific dates feature
	HasSpecificTimes *bool                `json:"hasSpecificTimes" bson:"hasSpecificTimes,omitempty"`
	Times            []primitive.DateTime `json:"times" bson:"times,omitempty"`

	// When true (only meaningful with HasSpecificTimes), recipients must select
	// an entire contiguous block of specific times at once, not individual slots.
	WholeBlockSelection *bool `json:"wholeBlockSelection" bson:"wholeBlockSelection,omitempty"`

	Type EventType `json:"type" bson:"type,omitempty"`

	// Sign up form details
	IsSignUpForm    *bool                      `json:"isSignUpForm" bson:"isSignUpForm,omitempty"`
	SignUpBlocks    *[]SignUpBlock             `json:"signUpBlocks" bson:"signUpBlocks,omitempty"`
	SignUpResponses map[string]*SignUpResponse `json:"signUpResponses" bson:"signUpResponses"`

	// Whether to start the event on Monday (as opposed to Sunday, used for DOW events)
	StartOnMonday *bool `json:"startOnMonday" bson:"startOnMonday,omitempty"`

	// Whether to enable blind availability
	BlindAvailabilityEnabled *bool `json:"blindAvailabilityEnabled" bson:"blindAvailabilityEnabled,omitempty"`

	// Whether to only poll for days, not times
	DaysOnly *bool `json:"daysOnly" bson:"daysOnly,omitempty"`

	// Availability responses - old format for backward compatibility (fetched from eventResponses collection)
	ResponsesMap map[string]*Response `json:"responses" bson:"-"`

	// Used to store the number of responses for the event
	NumResponses *int `json:"numResponses" bson:"numResponses,omitempty"`

	// Scheduled event (the confirmed gathering time, once the owner locks it in)
	ScheduledEvent  *CalendarEvent `json:"scheduledEvent" bson:"scheduledEvent,omitempty"`
	CalendarEventId string         `json:"calendarEventId" bson:"calendarEventId,omitempty"`

	// Pre-gathering reminder email config/state (paired with ScheduledEvent)
	GatheringReminder *GatheringReminder `json:"gatheringReminder" bson:"gatheringReminder,omitempty"`

	// Recurrence config for a repeating gathering (C5, paired with ScheduledEvent).
	// nil = a one-off gathering.
	GatheringRecurrence *GatheringRecurrence `json:"gatheringRecurrence" bson:"gatheringRecurrence,omitempty"`

	// RSVPs to the confirmed gathering, keyed by guest name / signed-in user id
	Rsvps map[string]*Rsvp `json:"rsvps" bson:"rsvps,omitempty"`

	// Venue / activity polls (C6). Owner-created multiple-choice polls; votes
	// live on each option.
	Polls []Poll `json:"polls" bson:"polls,omitempty"`

	// Whether this (non-recurring) gathering has been captured into the
	// Chronicle (C10). Set once by the scheduler after the gathering ends so it
	// isn't re-snapshotted. Recurring gatherings are captured per-occurrence at
	// advance time instead and don't use this flag.
	Chronicled bool `json:"chronicled" bson:"chronicled,omitempty"`

	// Discussion thread (fetched from the comments collection; not stored here)
	Comments []Comment `json:"comments" bson:"-"`

	// Remindees
	Remindees *[]Remindee `json:"remindees" bson:"remindees,omitempty"`
}

func (e *Event) GetId() string {
	if e.ShortId != nil {
		return *e.ShortId
	}

	return e.Id.Hex()
}
