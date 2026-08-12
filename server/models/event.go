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

// IsKnownEventType reports whether t is one of the defined event types. The
// `binding:"required"` tag on the create/edit payloads only rejects the empty
// string, so without this an arbitrary value is stored — and the frontend
// computes calendar bounds for these two types only, leaving an unknown-type
// event with no calendar overlay. Mirrors models.IsKnownRole.
func IsKnownEventType(t EventType) bool {
	switch t {
	case SPECIFIC_DATES, DOW:
		return true
	default:
		return false
	}
}

// Object containing information associated with the remindee
type Remindee struct {
	Email     string `json:"email" bson:"email,omitempty"`
	Responded *bool  `json:"responded" bson:"responded,omitempty"`

	// Nudge bookkeeping. The three "fill this in" emails used to be Cloud Tasks
	// scheduled against Listmonk, so all the state lived outside the database
	// (in a `taskIds` array that documents written before this change still
	// carry — harmless, and dropped on their next write). They're driven by the
	// in-process scheduler now, which needs to know when the clock started and
	// how far along it is.

	// AddedAt is when this remindee was attached to the event; the nudge
	// schedule is measured from it, not from the event's creation, because
	// editEvent can add remindees much later. Absent on older documents, where
	// callers fall back to the event ObjectID's timestamp.
	AddedAt *primitive.DateTime `json:"-" bson:"addedAt,omitempty"`
	// NudgeStage is how many nudges have been sent (0..3). Omitted from BSON
	// when 0, so queries must treat a missing field as 0.
	NudgeStage int `json:"-" bson:"nudgeStage,omitempty"`
	// LastNudgedAt is when the most recent nudge went out (diagnostics only).
	LastNudgedAt *primitive.DateTime `json:"-" bson:"lastNudgedAt,omitempty"`
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

// RSVP to a confirmed gathering (paired with ScheduledEvent). Stored on the
// Event as a map keyed by guest name or signed-in user id.
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
	GuestCount int    `json:"guestCount" bson:"guestCount,omitempty"`
	Name       string `json:"name" bson:"name,omitempty"`
	Email      string `json:"email" bson:"email,omitempty"`

	// User is the resolved account, attached per-request (never stored) so the
	// roster can render an avatar without a lookup per RSVP — the same shape
	// Comment.Author uses. Slimmed to identity fields; see resolveRsvpNames.
	// Nil for a legacy name-keyed row or a deleted account.
	//
	// `bson:"-"` is load-bearing: the RSVP write path $sets the whole rsvps map
	// from the in-memory struct, so an untagged field would be persisted.
	User *User `json:"user,omitempty" bson:"-"`

	UserId      primitive.ObjectID `json:"userId" bson:"userId,omitempty"`
	RespondedAt primitive.DateTime `json:"respondedAt" bson:"respondedAt,omitempty"`
}

// Poll is a lightweight multiple-choice poll on an event (C6) — e.g. "Where
// should we meet?" or "What should we do?". The owner creates it; members and
// guests vote. Votes live on each option (keyed by responder) so counts + the
// voter roster render straight from the event with no extra fetch. Stored as an
// array on the Event.
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

// EventList is a shared checklist on an event (F13) — "Menu", "Bars to Visit".
// The planner creates the list and fixes its Kind; anyone signed in adds items
// to it. Stored as an array on the Event like Polls, but every mutation is a
// targeted array update (see db/event_lists.go) rather than a whole-array $set:
// unlike polls, which a handful of people vote on occasionally, a list invites
// everyone to append at once, and rewriting the array from a value read earlier
// in the request loses concurrent additions.
type EventList struct {
	Id   primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Name string             `json:"name" bson:"name,omitempty"`
	// Kind is one of the ListKind constants, fixed when the list is created.
	// A location list feeds its input through the Google address lookup and
	// renders each item as a maps link; it stores the same plain string either
	// way, matching Event.Location. A checklist gives every item a checkbox.
	Kind  string          `json:"kind" bson:"kind,omitempty"`
	Items []EventListItem `json:"items" bson:"items,omitempty"`

	// Virtual marks a list that is DERIVED at read time rather than stored (N1).
	// Today there is exactly one: the "Assigned" list that GET /my-lists
	// synthesizes from the event's own checklist entries assigned to the caller.
	//
	// `bson:"-"`, like Rsvp.User — nothing writes this list, but the tag is what
	// guarantees it, and a virtual list reaching a write path is a bug rather
	// than a document to create. The client uses it to render the list read-only.
	Virtual bool `json:"virtual,omitempty" bson:"-"`
}

// The list kinds. Anything else is rejected at write time rather than
// defaulted, so a typo can't quietly produce a list that renders as plain text.
const (
	ListKindText      = "text"
	ListKindLocation  = "location"
	ListKindChecklist = "checklist"
)

// EventListItem is one entry on a list. AuthorName is a DisplayName() snapshot
// taken at write time and re-resolved on read (see routes/display_names.go), so
// it survives the author's account being deleted but still follows a nickname
// change.
//
// Items nest up to models-agnostic depth (see routes/event_lists.go's
// maxListItemDepth) by pointing at a parent rather than by containing children:
// a tree of arrays would force whole-array rewrites on every mutation, which is
// exactly what db/event_lists.go's targeted updates exist to avoid.
type EventListItem struct {
	Id   primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Text string             `json:"text" bson:"text,omitempty"`
	// ParentId is nil for a top-level item. A POINTER, not a bare ObjectID:
	// omitempty can't omit a [12]byte array, so a zero id would serialize as 24
	// zeros and read back as a real parent. Items written before nesting existed
	// have no field at all, which decodes to nil — hence no migration.
	//
	// Set once, when the item is added. A move never points it at a NEW parent,
	// only clears it (F17), so an item's depth can shrink but never grow — which
	// is what keeps routes/event_lists.go's depth check exact.
	ParentId *primitive.ObjectID `json:"parentId,omitempty" bson:"parentId,omitempty"`
	// Order places the item among its SIBLINGS — entries sharing its parentId
	// compete only with each other. Fractional on purpose (see
	// listItemOrderStep in routes/event_lists.go): a drop writes only the item
	// that moved, which is what keeps db/event_lists.go's single-targeted-update
	// invariant intact where integer reindexing would rewrite the whole group.
	//
	// NO omitempty on either tag, unlike everything around it. Two reasons: a
	// drop at the top of a list legitimately computes 0, and omitempty would
	// strip it back to "absent"; and absence is precisely how the F17 migration
	// recognizes an item written before ordering existed. Items that predate the
	// field decode to 0 and sort as a tie, which the display's stable
	// array-position tie-break renders as today's insertion order.
	Order      float64            `json:"order" bson:"order"`
	UserId     primitive.ObjectID `json:"userId" bson:"userId,omitempty"`
	AuthorName string             `json:"authorName" bson:"authorName,omitempty"`
	CreatedAt  primitive.DateTime `json:"createdAt" bson:"createdAt,omitempty"`

	// Checklist state, meaningful only on a ListKindChecklist list. All four are
	// absent until someone first ticks the box, and from then on they are always
	// written together — so Checked=false alongside a CheckedByName renders
	// "Unchecked by Bart", while an item nobody has touched renders nothing.
	// Only the last change is kept; there is deliberately no history.
	//
	// omitempty on the bool is safe because the write is a literal bson.M $set
	// (struct tags don't apply to it), so an uncheck really does store false.
	Checked       bool                `json:"checked,omitempty" bson:"checked,omitempty"`
	CheckedBy     *primitive.ObjectID `json:"checkedBy,omitempty" bson:"checkedBy,omitempty"`
	CheckedByName string              `json:"checkedByName,omitempty" bson:"checkedByName,omitempty"`
	CheckedAt     primitive.DateTime  `json:"checkedAt,omitempty" bson:"checkedAt,omitempty"`

	// Who the entry is FOR (N1) — distinct from UserId, who wrote it, and from
	// CheckedBy, who last ticked it. Meaningful only on a ListKindChecklist list.
	//
	// A POINTER for the reason ParentId is one: omitempty can't omit a [12]byte,
	// so a zero id would serialize as 24 zeros and read back as a real member.
	// AssigneeName is a DisplayName() snapshot re-resolved on read exactly like
	// AuthorName and CheckedByName (routes/display_names.go).
	//
	// The pair is always written together, in both directions: unassigning $sets
	// both to their zero value rather than $unset-ing them, which keeps the write
	// inside db's existing setListItemFields.
	AssigneeId   *primitive.ObjectID `json:"assigneeId,omitempty" bson:"assigneeId,omitempty"`
	AssigneeName string              `json:"assigneeName,omitempty" bson:"assigneeName,omitempty"`

	// Where this entry really lives, attached per-request (NEVER stored) when it
	// is synthesized into the virtual "Assigned" list, so the client can write a
	// tick back to the shared list it came from rather than to a private one.
	//
	// `bson:"-"` is load-bearing, not decoration: moveListItems $pushes whole
	// EventListItem values, so an untagged field here would be persisted onto the
	// real item the first time anyone dragged one between lists.
	SourceListId   *primitive.ObjectID `json:"sourceListId,omitempty" bson:"-"`
	SourceListName string              `json:"sourceListName,omitempty" bson:"-"`
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

	// Shared lists (F13). Planner-created; anyone signed in adds items.
	Lists []EventList `json:"lists" bson:"lists,omitempty"`

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
