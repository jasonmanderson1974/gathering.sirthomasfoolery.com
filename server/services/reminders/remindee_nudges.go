package reminders

import (
	"fmt"
	"net/url"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/utils"
)

// The "please fill this in" nudges sent to an event's remindees.
//
// These used to be three Cloud Tasks scheduled at creation time, each POSTing
// to a Listmonk template. That put the schedule outside the database and made
// the feature depend on two services this instance has never had, so nothing
// was ever sent. They run on the same ticker as the pre-gathering reminder now,
// with the schedule kept on the remindee itself.

const maxNudgeStage = 3

// Offsets from when the remindee was added: on the next tick, a day later,
// three days later.
var nudgeOffsets = [maxNudgeStage]time.Duration{0, 24 * time.Hour, 72 * time.Hour}

// nudgeMaxAge stops a long-dormant remindee from being nudged. It covers two
// cases: the first tick after this feature ships (every old remindee would
// otherwise look overdue), and a server that was down for a week.
const nudgeMaxAge = 7 * 24 * time.Hour

// maxNudgesPerTick bounds a single tick's SMTP work. utils.SendEmail dials once
// per recipient and Gmail caps daily volume, so a burst is spread over ticks
// rather than attempted at once.
const maxNudgesPerTick = 50

// dueNudgeStage returns the stage that should be sent now for a remindee added
// at addedAt who has already had sentStage nudges, or 0 when nothing is due.
//
// Missed stages are coalesced rather than replayed: after an outage a remindee
// gets the latest nudge they're due, not the whole backlog. This matches how
// advanceRecurringGatherings handles a gap.
func dueNudgeStage(now, addedAt time.Time, sentStage int) int {
	if sentStage >= maxNudgeStage {
		return 0
	}
	if sentStage < 0 {
		sentStage = 0
	}

	due := 0
	for stage := sentStage; stage < maxNudgeStage; stage++ {
		if !now.Before(addedAt.Add(nudgeOffsets[stage])) {
			due = stage + 1
		}
	}
	return due
}

// remindeeAddedAt returns when the nudge clock started for a remindee. Documents
// written before AddedAt existed fall back to the event's creation time, which
// an ObjectID carries — those are all far older than nudgeMaxAge anyway, so they
// retire rather than fire.
func remindeeAddedAt(event *models.Event, r models.Remindee) time.Time {
	if r.AddedAt != nil {
		return r.AddedAt.Time()
	}
	return event.Id.Timestamp()
}

// nudgeRespondedURL is the "I've already responded" link. The email is a query
// parameter and must be escaped — the Cloud Tasks version didn't, so any
// address containing a '+' arrived broken.
func nudgeRespondedURL(eventId, email string) string {
	return fmt.Sprintf("%s/e/%s/responded?email=%s", utils.GetBaseUrl(), eventId, url.QueryEscape(email))
}

// processRemindeeNudges sends any nudges that have come due.
func processRemindeeNudges(now time.Time, send SendFunc) {
	events, err := db.GetEventsWithPendingRemindeeNudges()
	if err != nil {
		return // already logged; retry next tick
	}

	sent := 0
	for i := range events {
		event := events[i]
		if event.Remindees == nil {
			continue
		}

		ownerName := "Somebody"
		if owner, ownerErr := db.GetUserById(event.OwnerId.Hex()); ownerErr == nil && owner != nil {
			ownerName = owner.FirstName
		}
		eventURL := fmt.Sprintf("%s/e/%s", utils.GetBaseUrl(), event.GetId())

		for _, remindee := range *event.Remindees {
			if sent >= maxNudgesPerTick {
				return // rest picked up next tick
			}
			if remindee.Responded != nil && *remindee.Responded {
				continue
			}

			addedAt := remindeeAddedAt(&event, remindee)

			// Too old to be worth nudging — retire it so it stops matching the
			// query, without sending anything.
			if now.Sub(addedAt) > nudgeMaxAge {
				db.MarkRemindeeNudged(event.Id, remindee.Email, remindee.NudgeStage, maxNudgeStage, primitive.NewDateTimeFromTime(now))
				continue
			}

			stage := dueNudgeStage(now, addedAt, remindee.NudgeStage)
			if stage == 0 {
				continue
			}

			// Claim the nudge before sending. If another tick got there first
			// the compare-and-set fails and we skip; if the send then fails we
			// lose that one nudge rather than retrying forever — the same
			// trade-off processDueReminders makes.
			claimed, claimErr := db.MarkRemindeeNudged(
				event.Id, remindee.Email, remindee.NudgeStage, stage, primitive.NewDateTimeFromTime(now),
			)
			if claimErr != nil || !claimed {
				continue
			}

			subject, body := buildRemindeeNudgeEmail(
				stage, ownerName, event.Name, eventURL, nudgeRespondedURL(event.GetId(), remindee.Email),
			)
			if err := send(remindee.Email, subject, body, "text/html"); err != nil {
				logger.StdErr.Println("remindee nudge failed for", remindee.Email, ":", err)
			}
			sent++
		}
	}
}

// buildRemindeeNudgeEmail composes the nudge for a given stage (1..3), which
// only changes the tone — the actions are the same throughout.
func buildRemindeeNudgeEmail(stage int, ownerName, eventName, eventURL, respondedURL string) (subject, body string) {
	var heading, lead string
	switch stage {
	case 1:
		subject = fmt.Sprintf("%s needs your availability for %s", ownerName, eventName)
		heading = "You are summoned"
		lead = fmt.Sprintf("%s has called a Gathering and asks when you might attend.", ownerName)
	case 2:
		subject = fmt.Sprintf("Reminder: %s is still waiting on you", eventName)
		heading = "Still awaiting your reply"
		lead = fmt.Sprintf("%s has yet to hear from you about this Gathering.", ownerName)
	default:
		subject = fmt.Sprintf("Final reminder: %s", eventName)
		heading = "A final word"
		lead = fmt.Sprintf("This is the last we shall trouble you about %s's Gathering.", ownerName)
	}

	body = utils.RenderEmailWithFooter(
		heading,
		utils.EmailParagraph(lead)+utils.EmailStrongLine(eventName),
		utils.EmailFooterURL(eventURL),
		utils.EmailAction{Label: "Share your availability", URL: eventURL},
		utils.EmailAction{Label: "I've already responded", URL: respondedURL, Secondary: true},
	)
	return subject, body
}
