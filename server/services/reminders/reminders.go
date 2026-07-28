// Package reminders runs an in-process scheduler for every email the app sends
// on a schedule rather than in response to a request:
//
//   - the one-time reminder to each availability respondent, a configurable
//     number of hours before a confirmed gathering starts
//   - the three "please fill this in" nudges to an event's remindees, at 0h,
//     24h and 72h after they were added (see remindee_nudges.go)
//
// Everything goes via Gmail SMTP (utils.SendEmail), the same transport used for
// OTP and invite emails. This replaced a Cloud Tasks + Listmonk arrangement that
// was never configured here, so none of it ever actually sent.
//
// Single-VM assumption: no distributed locking.
package reminders

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/utils"
)

// SendFunc sends a single email. Matches utils.SendEmail's signature so it can
// be injected in tests (and defaulted to utils.SendEmail in production).
type SendFunc func(toEmail, subject, body, contentType string) error

const defaultInterval = 5 * time.Minute

// StartReminderScheduler launches the background ticker and returns a stop
// function. Each tick rolls recurring gatherings forward (C5) and, when Gmail
// SMTP creds are present, sends any due pre-gathering reminders and remindee
// nudges. Sending no-ops without creds, but recurrence advancement runs
// regardless — it only touches the DB, so the event page always shows the next
// occurrence even on an email-less instance.
func StartReminderScheduler() func() {
	emailConfigured := os.Getenv("GMAIL_APP_PASSWORD") != "" && os.Getenv("SCHEJ_EMAIL_ADDRESS") != ""
	if !emailConfigured {
		logger.StdOut.Println("GMAIL creds not set — reminder emails disabled (recurring-gathering advance still runs)")
	}

	interval := defaultInterval
	if v := os.Getenv("REMINDER_SCHEDULER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		} else {
			logger.StdErr.Println("invalid REMINDER_SCHEDULER_INTERVAL, using default:", err)
		}
	}

	tick := func() {
		now := time.Now()
		// Advance recurring gatherings first (this also captures each completed
		// occurrence into the Chronicle before its RSVPs are cleared), then
		// archive one-off gatherings that have ended, then send due reminders.
		advanceRecurringGatherings(now)
		archivePastGatherings(now)
		if emailConfigured {
			processDueReminders(now, utils.SendEmail)
			processRemindeeNudges(now, utils.SendEmail)
		}
	}

	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		// Run once at startup so a reminder/advance that came due while the server
		// was down doesn't wait a full interval.
		tick()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				tick()
			}
		}
	}()

	logger.StdOut.Println("Gathering scheduler started, interval:", interval)
	return func() {
		ticker.Stop()
		close(done)
	}
}

// advanceRecurringGatherings rolls every recurring gathering whose current
// occurrence has ended forward to its next future occurrence, clearing that
// cycle's RSVPs and re-arming the reminder. It skips past a long outage (jumps
// straight to the next occurrence after `now`, not replaying missed ones) and
// stops a series once its next occurrence would fall after RecurrenceUntil.
func advanceRecurringGatherings(now time.Time) {
	events, err := db.GetRecurringGatheringsToAdvance(primitive.NewDateTimeFromTime(now))
	if err != nil {
		return // already logged
	}

	for i := range events {
		event := events[i]
		rec := event.GatheringRecurrence
		if event.ScheduledEvent == nil || !rec.IsRecurring() {
			continue
		}

		start := event.ScheduledEvent.StartDate.Time()
		duration := event.ScheduledEvent.EndDate.Time().Sub(start)

		next := rec.NextOccurrenceAfter(start, now)
		if next.IsZero() {
			continue
		}
		// Series exhausted: the next occurrence is past its end date. Leave the
		// gathering on its last occurrence rather than rolling into the void.
		if rec.Until != nil && next.After(rec.Until.Time()) {
			continue
		}

		// Capture the just-completed occurrence into the Chronicle (C10) BEFORE
		// advancing, since AdvanceGathering clears this cycle's RSVPs. Dup-safe
		// (unique index on eventId+startDate), so a racing tick can't double-record.
		db.InsertChronicleEntry(buildChronicleEntry(&event, now))

		newStart := primitive.NewDateTimeFromTime(next)
		newEnd := primitive.NewDateTimeFromTime(next.Add(duration))
		if _, err := db.AdvanceGathering(event.Id, event.ScheduledEvent.StartDate, newStart, newEnd); err != nil {
			// already logged; try again next tick
			continue
		}
	}
}

// archivePastGatherings captures one-off (non-recurring) gatherings whose
// confirmed time has passed into the Chronicle, exactly once each (C10). On
// first run after deploy this also backfills any already-past gatherings.
func archivePastGatherings(now time.Time) {
	events, err := db.GetPastNonRecurringGatheringsToArchive(primitive.NewDateTimeFromTime(now))
	if err != nil {
		return // already logged
	}

	for i := range events {
		event := events[i]
		if event.ScheduledEvent == nil {
			continue
		}
		if err := db.InsertChronicleEntry(buildChronicleEntry(&event, now)); err != nil {
			continue // transient — retry next tick (not yet marked chronicled)
		}
		db.MarkEventChronicled(event.Id)
	}
}

// buildChronicleEntry snapshots a completed gathering (its confirmed time,
// venue, description, and the roster of who was going/maybe) into a
// ChronicleEntry. Pure apart from reading the event.
func buildChronicleEntry(event *models.Event, capturedAt time.Time) models.ChronicleEntry {
	attendees, headCount := chronicleAttendees(event.Rsvps)

	entry := models.ChronicleEntry{
		Id:          primitive.NewObjectID(),
		EventId:     event.Id,
		ShortId:     event.ShortId,
		Name:        event.Name,
		Description: event.Description,
		Location:    event.Location,
		Attendees:   attendees,
		HeadCount:   headCount,
		CapturedAt:  primitive.NewDateTimeFromTime(capturedAt),
	}
	if event.ScheduledEvent != nil {
		entry.StartDate = event.ScheduledEvent.StartDate
		entry.EndDate = event.ScheduledEvent.EndDate
	}
	return entry
}

// chronicleAttendees turns a gathering's RSVPs into the going/maybe roster plus
// a total headcount (sum of 1 + guestCount over those attendees). Decliners are
// excluded. Deterministic-ish; order isn't guaranteed (map iteration), so
// callers that need stable order should sort.
func chronicleAttendees(rsvps map[string]*models.Rsvp) ([]models.ChronicleAttendee, int) {
	attendees := make([]models.ChronicleAttendee, 0)
	headCount := 0
	for key, rsvp := range rsvps {
		if rsvp == nil {
			continue
		}
		if rsvp.Status != models.RsvpGoing && rsvp.Status != models.RsvpMaybe {
			continue
		}
		name := rsvp.Name
		if name == "" {
			name = key
		}
		attendees = append(attendees, models.ChronicleAttendee{
			Name:       name,
			Status:     rsvp.Status,
			GuestCount: rsvp.GuestCount,
		})
		headCount += 1 + rsvp.GuestCount
	}
	return attendees, headCount
}

// processDueReminders sends reminders for every gathering that has entered its
// lead-time window and hasn't been reminded yet, then marks each as sent so it
// is never re-sent (even if some individual sends failed).
func processDueReminders(now time.Time, send SendFunc) {
	events, err := db.GetEventsWithPendingReminders()
	if err != nil {
		return // already logged
	}

	for i := range events {
		event := events[i]
		if event.ScheduledEvent == nil || event.GatheringReminder == nil {
			continue
		}
		start := event.ScheduledEvent.StartDate.Time()
		if !isReminderDue(now, start, event.GatheringReminder.LeadTimeHours) {
			continue
		}

		lookupEmail := func(userId string) string {
			user, err := db.GetUserById(userId)
			if err != nil || user == nil {
				return ""
			}
			return user.Email
		}

		// Prefer RSVPs once anyone has responded (remind going + maybe, skip
		// decliners). Before any RSVP exists, fall back to all availability
		// respondents so reminders keep working.
		var emails []string
		if len(event.Rsvps) > 0 {
			emails = collectRsvpRecipientEmails(&event, lookupEmail)
		} else {
			responses, respErr := db.GetEventResponses(event.Id.Hex())
			if respErr != nil {
				// Transient DB error — skip (don't mark sent) so we retry next tick.
				continue
			}
			emails = collectRecipientEmails(responses, lookupEmail)
		}

		subject, body := buildReminderEmail(&event, start)
		for _, email := range emails {
			if err := send(email, subject, body, "text/html"); err != nil {
				logger.StdErr.Println("reminder email failed for", email, ":", err)
			}
		}

		// Mark sent regardless of per-recipient failures to avoid resend loops.
		db.MarkGatheringReminderSent(event.Id, primitive.NewDateTimeFromTime(now))
	}
}

// isReminderDue reports whether a reminder should fire now: the gathering is
// still in the future and we've reached (start - leadTimeHours).
func isReminderDue(now, start time.Time, leadTimeHours int) bool {
	if !start.After(now) {
		return false // gathering already started/past — don't send a late reminder
	}
	dueAt := start.Add(-time.Duration(leadTimeHours) * time.Hour)
	return !now.Before(dueAt) // now >= dueAt
}

// collectRecipientEmails returns the deduped set of email addresses to remind:
// a guest's supplied email, or a signed-in responder's account email resolved
// via lookupEmail (userId hex -> email, "" if none). Order is stable by first
// appearance so output is deterministic (helps tests).
func collectRecipientEmails(responses []models.EventResponse, lookupEmail func(userId string) string) []string {
	seen := make(map[string]bool)
	emails := make([]string, 0)

	add := func(email string) {
		if email == "" || seen[email] {
			return
		}
		seen[email] = true
		emails = append(emails, email)
	}

	for _, resp := range responses {
		if resp.Response != nil && resp.Response.Email != "" {
			add(resp.Response.Email) // guest-supplied email
			continue
		}
		if _, err := primitive.ObjectIDFromHex(resp.UserId); err == nil {
			add(lookupEmail(resp.UserId)) // signed-in responder
		}
	}

	return emails
}

// collectRsvpRecipientEmails returns the deduped emails of everyone who RSVP'd
// going or maybe (decliners excluded). Uses the RSVP's stored email, else
// resolves a signed-in RSVP's account email via lookupEmail. Keyed like
// collectRecipientEmails so both share the reminder pipeline. `key` is the map
// key (guest name or user-id hex); it's the userId lookup source when a
// signed-in RSVP has no stored email.
func collectRsvpRecipientEmails(event *models.Event, lookupEmail func(userId string) string) []string {
	seen := make(map[string]bool)
	emails := make([]string, 0)

	add := func(email string) {
		if email == "" || seen[email] {
			return
		}
		seen[email] = true
		emails = append(emails, email)
	}

	for key, rsvp := range event.Rsvps {
		if rsvp == nil {
			continue
		}
		if rsvp.Status != models.RsvpGoing && rsvp.Status != models.RsvpMaybe {
			continue // skip decliners
		}
		if rsvp.Email != "" {
			add(rsvp.Email)
			continue
		}
		// Signed-in RSVP with no stored email: the map key is the user-id hex.
		if _, err := primitive.ObjectIDFromHex(key); err == nil {
			add(lookupEmail(key))
		}
	}

	return emails
}

// buildReminderEmail renders the subject + inline-HTML body for a gathering
// reminder, formatting the start time in the event's saved timezone (falling
// back to UTC). Styled to match the Fellowship OTP/invite emails.
func buildReminderEmail(event *models.Event, start time.Time) (subject, body string) {
	loc := time.UTC
	if event.GatheringReminder != nil && event.GatheringReminder.Timezone != "" {
		if l, err := time.LoadLocation(event.GatheringReminder.Timezone); err == nil {
			loc = l
		}
	}
	when := start.In(loc).Format("Monday, January 2 at 3:04 PM MST")

	subject = fmt.Sprintf("Reminder: %s", event.Name)
	eventUrl := fmt.Sprintf("%s/e/%s", utils.GetBaseUrl(), event.GetId())
	// Universal "add to calendar" (.ics) — works for members without a Google
	// account. Same-origin /api path (prod), served by getEventIcs.
	icsUrl := fmt.Sprintf("%s/api/events/%s/ics", utils.GetBaseUrl(), event.GetId())

	// event.Name, Location and Description are all user-supplied, so they go
	// through the escaping helpers rather than straight into the markup.
	locationRow := ""
	if event.Location != nil && *event.Location != "" {
		mapsUrl := "https://www.google.com/maps/search/?api=1&query=" + url.QueryEscape(*event.Location)
		locationRow = utils.EmailRow("📍 " + utils.EmailLink(mapsUrl, *event.Location))
	}

	descriptionRow := ""
	if event.Description != nil && *event.Description != "" {
		descriptionRow = utils.EmailParagraph(*event.Description)
	}

	body = utils.RenderEmailWithFooter(
		"A gathering approaches",
		utils.EmailStrongLine(event.Name)+
			utils.EmailAccentLine(when)+
			locationRow+
			descriptionRow,
		utils.EmailFooterURL(eventUrl),
		utils.EmailAction{Label: "View the Gathering", URL: eventUrl},
		utils.EmailAction{Label: "Add to calendar", URL: icsUrl, Secondary: true},
	)

	return subject, body
}
