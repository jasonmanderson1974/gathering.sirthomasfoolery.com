package reminders

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
	"sirtom/server/utils"
)

// insertNudgeEvent creates an event with the given remindees and registers
// cleanup. addedAt is applied to every remindee that doesn't carry its own.
func insertNudgeEvent(t *testing.T, addedAt time.Time, remindees []models.Remindee) primitive.ObjectID {
	t.Helper()
	ctx := context.Background()

	added := primitive.NewDateTimeFromTime(addedAt)
	for i := range remindees {
		if remindees[i].AddedAt == nil {
			remindees[i].AddedAt = &added
		}
	}

	eventId := primitive.NewObjectID()
	_, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:        eventId,
		Name:      "Nudge Gathering",
		Type:      models.SPECIFIC_DATES,
		Remindees: &remindees,
	})
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { db.EventsCollection.DeleteOne(ctx, bson.M{"_id": eventId}) })
	return eventId
}

func remindeesOf(t *testing.T, eventId primitive.ObjectID) []models.Remindee {
	t.Helper()
	var event models.Event
	if err := db.EventsCollection.FindOne(context.Background(), bson.M{"_id": eventId}).Decode(&event); err != nil {
		t.Fatalf("reload event: %v", err)
	}
	if event.Remindees == nil {
		return nil
	}
	return *event.Remindees
}

func TestProcessRemindeeNudges_SendsStageOneAndMarksIt(t *testing.T) {
	requireDB(t)

	now := time.Now()
	eventId := insertNudgeEvent(t, now, []models.Remindee{
		{Email: "nudge-a@example.test", Responded: utils.FalsePtr()},
	})

	var sentTo []string
	processRemindeeNudges(now, func(to, subject, body, contentType string) error {
		sentTo = append(sentTo, to)
		return nil
	})

	if len(sentTo) != 1 || sentTo[0] != "nudge-a@example.test" {
		t.Fatalf("expected one nudge to nudge-a@example.test, got %v", sentTo)
	}

	got := remindeesOf(t, eventId)
	if len(got) != 1 || got[0].NudgeStage != 1 {
		t.Fatalf("expected nudgeStage 1, got %+v", got)
	}
	if got[0].LastNudgedAt == nil {
		t.Error("expected lastNudgedAt to be recorded")
	}
}

// A second tick before the next offset must not re-send.
func TestProcessRemindeeNudges_DoesNotResendWithinTheSameStage(t *testing.T) {
	requireDB(t)

	now := time.Now()
	insertNudgeEvent(t, now, []models.Remindee{
		{Email: "nudge-b@example.test", Responded: utils.FalsePtr()},
	})

	count := 0
	send := func(to, subject, body, contentType string) error { count++; return nil }

	processRemindeeNudges(now, send)
	processRemindeeNudges(now.Add(time.Minute), send)

	if count != 1 {
		t.Fatalf("expected exactly one send across two ticks, got %d", count)
	}
}

func TestProcessRemindeeNudges_SkipsRespondedRemindees(t *testing.T) {
	requireDB(t)

	now := time.Now()
	insertNudgeEvent(t, now, []models.Remindee{
		{Email: "nudge-done@example.test", Responded: utils.TruePtr()},
	})

	count := 0
	processRemindeeNudges(now, func(to, subject, body, contentType string) error { count++; return nil })

	if count != 0 {
		t.Fatalf("a remindee who has responded should not be nudged (got %d sends)", count)
	}
}

// Anything older than the cutoff is retired without a send — this is what stops
// the first tick after deploy from nudging every historic remindee.
func TestProcessRemindeeNudges_RetiresStaleRemindeesWithoutSending(t *testing.T) {
	requireDB(t)

	now := time.Now()
	stale := now.Add(-30 * 24 * time.Hour)
	eventId := insertNudgeEvent(t, stale, []models.Remindee{
		{Email: "nudge-stale@example.test", Responded: utils.FalsePtr()},
	})

	count := 0
	processRemindeeNudges(now, func(to, subject, body, contentType string) error { count++; return nil })

	if count != 0 {
		t.Fatalf("stale remindee should not be nudged (got %d sends)", count)
	}
	got := remindeesOf(t, eventId)
	if len(got) != 1 || got[0].NudgeStage != maxNudgeStage {
		t.Fatalf("stale remindee should be retired at stage %d, got %+v", maxNudgeStage, got)
	}
}

// Once a time is confirmed there is nothing left to ask for.
func TestGetEventsWithPendingRemindeeNudges_ExcludesScheduledEvents(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	now := time.Now()
	added := primitive.NewDateTimeFromTime(now)
	eventId := primitive.NewObjectID()
	start := primitive.NewDateTimeFromTime(now.Add(48 * time.Hour))
	remindees := []models.Remindee{{Email: "nudge-sched@example.test", Responded: utils.FalsePtr(), AddedAt: &added}}

	_, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:             eventId,
		Name:           "Already Settled",
		Type:           models.SPECIFIC_DATES,
		Remindees:      &remindees,
		ScheduledEvent: &models.CalendarEvent{StartDate: start, EndDate: start},
	})
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	defer db.EventsCollection.DeleteOne(ctx, bson.M{"_id": eventId})

	events, err := db.GetEventsWithPendingRemindeeNudges()
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	for _, e := range events {
		if e.Id == eventId {
			t.Error("a scheduled event should not be returned for nudging")
		}
	}
}

func TestMarkRemindeeNudged_CompareAndSet(t *testing.T) {
	requireDB(t)

	now := time.Now()
	sentAt := primitive.NewDateTimeFromTime(now)
	eventId := insertNudgeEvent(t, now, []models.Remindee{
		{Email: "nudge-cas@example.test", Responded: utils.FalsePtr()},
	})

	// Stage 0 is omitted from BSON, so "still at 0" has to match a missing field.
	ok, err := db.MarkRemindeeNudged(eventId, "nudge-cas@example.test", 0, 1, sentAt)
	if err != nil || !ok {
		t.Fatalf("first claim should succeed (ok=%v, err=%v)", ok, err)
	}

	// A second tick still believing the stage is 0 must lose.
	ok, err = db.MarkRemindeeNudged(eventId, "nudge-cas@example.test", 0, 1, sentAt)
	if err != nil {
		t.Fatalf("second claim errored: %v", err)
	}
	if ok {
		t.Error("a stale expectedStage must not win the compare-and-set")
	}

	// The correct current stage still advances.
	ok, err = db.MarkRemindeeNudged(eventId, "nudge-cas@example.test", 1, 2, sentAt)
	if err != nil || !ok {
		t.Fatalf("claim from the current stage should succeed (ok=%v, err=%v)", ok, err)
	}
}
