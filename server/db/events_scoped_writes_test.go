package db_test

import (
	"context"
	"sync"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// A16: these cover the guarantees the scoped writes exist to provide — that
// concurrent callers don't undo each other, and that the two "send this email
// once" guards really are once. Each one fails against the old
// whole-document `$set: event`.
//
// Remindee fixtures below carry nudgeStage = alreadyNudged so the reminder
// scheduler's sweep skips them: `go test` runs packages in parallel against one
// Mongo, and services/reminders sweeps every eligible event in the database.

// alreadyNudged marks a fixture remindee as fully nudged (reminders.maxNudgeStage,
// not importable from here — that package imports db).
const alreadyNudged = 3

func insertScopedTestEvent(t *testing.T, event models.Event) primitive.ObjectID {
	t.Helper()
	if event.Id.IsZero() {
		event.Id = primitive.NewObjectID()
	}
	if _, err := db.EventsCollection.InsertOne(context.Background(), event); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() {
		db.EventsCollection.DeleteOne(context.Background(), bson.M{"_id": event.Id})
	})
	return event.Id
}

func reloadEvent(t *testing.T, id primitive.ObjectID) models.Event {
	t.Helper()
	var ev models.Event
	if err := db.EventsCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&ev); err != nil {
		t.Fatalf("reload event: %v", err)
	}
	return ev
}

// The headline case: concurrent responders must all be counted. A
// read-modify-write of the whole document loses all but one.
func TestIncrementNumResponsesIsAtomicUnderConcurrency(t *testing.T) {
	zero := 0
	id := insertScopedTestEvent(t, models.Event{Name: "A16 count", NumResponses: &zero})

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			db.IncrementNumResponses(id, 1)
		}()
	}
	wg.Wait()

	got := reloadEvent(t, id)
	if got.NumResponses == nil || *got.NumResponses != n {
		t.Fatalf("numResponses = %v, want %d — increments were lost", got.NumResponses, n)
	}
}

func TestIncrementNumResponsesDecrements(t *testing.T) {
	three := 3
	id := insertScopedTestEvent(t, models.Event{Name: "A16 count down", NumResponses: &three})

	if err := db.IncrementNumResponses(id, -1); err != nil {
		t.Fatalf("decrement: %v", err)
	}
	if got := reloadEvent(t, id); got.NumResponses == nil || *got.NumResponses != 2 {
		t.Fatalf("numResponses = %v, want 2", got.NumResponses)
	}
}

// Only one of N simultaneous responders may be told they were the Nth.
func TestDisarmSendEmailAfterXResponsesWinsOnce(t *testing.T) {
	threshold := 3
	id := insertScopedTestEvent(t, models.Event{
		Name: "A16 disarm", SendEmailAfterXResponses: &threshold,
	})

	const n = 10
	var wins int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if ok, _ := db.DisarmSendEmailAfterXResponses(id, 3); ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if wins != 1 {
		t.Fatalf("%d callers were told to send the email, want exactly 1", wins)
	}
	if got := reloadEvent(t, id); got.SendEmailAfterXResponses == nil || *got.SendEmailAfterXResponses != -1 {
		t.Fatalf("threshold = %v, want -1 (disarmed)", got.SendEmailAfterXResponses)
	}
}

func TestDisarmSendEmailAfterXResponsesIgnoresStaleExpectation(t *testing.T) {
	threshold := 5
	id := insertScopedTestEvent(t, models.Event{
		Name: "A16 disarm stale", SendEmailAfterXResponses: &threshold,
	})

	ok, err := db.DisarmSendEmailAfterXResponses(id, 3) // caller saw a different value
	if err != nil {
		t.Fatalf("disarm: %v", err)
	}
	if ok {
		t.Error("a stale threshold must not win the compare-and-set")
	}
	if got := reloadEvent(t, id); *got.SendEmailAfterXResponses != 5 {
		t.Errorf("threshold was modified to %d despite the mismatch", *got.SendEmailAfterXResponses)
	}
}

// The remindee flag is also a send-once guard: whoever flips it sends the
// "everyone has responded" mail.
func TestMarkRemindeeRespondedFlipsOnce(t *testing.T) {
	no := false
	remindees := []models.Remindee{
		{Email: "a@example.test", Responded: &no, NudgeStage: alreadyNudged},
		{Email: "b@example.test", Responded: &no, NudgeStage: alreadyNudged},
	}
	id := insertScopedTestEvent(t, models.Event{Name: "A16 remindees", Remindees: &remindees})

	first, err := db.MarkRemindeeResponded(id, "a@example.test")
	if err != nil || !first {
		t.Fatalf("first flip should win (ok=%v err=%v)", first, err)
	}
	second, err := db.MarkRemindeeResponded(id, "a@example.test")
	if err != nil {
		t.Fatalf("second flip errored: %v", err)
	}
	if second {
		t.Error("an already-responded remindee must not flip again")
	}

	got := reloadEvent(t, id)
	for _, r := range *got.Remindees {
		switch r.Email {
		case "a@example.test":
			if r.Responded == nil || !*r.Responded {
				t.Error("a@ should be marked responded")
			}
		case "b@example.test":
			if r.Responded != nil && *r.Responded {
				t.Error("b@ was flipped by a write aimed at a@")
			}
		}
	}
}

func TestMarkRemindeeRespondedUnknownEmail(t *testing.T) {
	no := false
	remindees := []models.Remindee{{Email: "a@example.test", Responded: &no, NudgeStage: alreadyNudged}}
	id := insertScopedTestEvent(t, models.Event{Name: "A16 unknown remindee", Remindees: &remindees})

	ok, err := db.MarkRemindeeResponded(id, "nobody@example.test")
	if err != nil {
		t.Fatalf("errored: %v", err)
	}
	if ok {
		t.Error("an unknown email must not report a flip")
	}
}

// An edit must leave everything it doesn't own alone. Under the old whole-
// document write, a response or comment made while the edit dialog was open was
// reverted by whoever saved second.
func TestUpdateEditableEventFieldsLeavesOtherFieldsAlone(t *testing.T) {
	five := 5
	id := insertScopedTestEvent(t, models.Event{
		Name:         "A16 original",
		NumResponses: &five,
		Rsvps: map[string]*models.Rsvp{
			"someone": {Status: models.RsvpGoing},
		},
	})

	// A stale snapshot: an editor who loaded the event before those arrived.
	zero := 0
	stale := models.Event{
		Id:           id,
		Name:         "A16 renamed",
		NumResponses: &zero,
		Rsvps:        map[string]*models.Rsvp{},
	}
	if err := db.UpdateEditableEventFields(&stale); err != nil {
		t.Fatalf("update: %v", err)
	}

	got := reloadEvent(t, id)
	if got.Name != "A16 renamed" {
		t.Errorf("the edit didn't apply: name = %q", got.Name)
	}
	if got.NumResponses == nil || *got.NumResponses != 5 {
		t.Errorf("numResponses was clobbered: %v, want 5", got.NumResponses)
	}
	if _, ok := got.Rsvps["someone"]; !ok {
		t.Error("a concurrent RSVP was clobbered by the edit")
	}
}

// Every edit-owned field is `omitempty`, so a nil pointer was previously
// omitted from the write and kept its stored value. Naming the fields
// explicitly would have written nulls instead — this pins that behaviour.
func TestUpdateEditableEventFieldsOmitsUnsetFields(t *testing.T) {
	desc := "the original description"
	id := insertScopedTestEvent(t, models.Event{
		Name: "A16 omitempty", Description: &desc,
	})

	if err := db.UpdateEditableEventFields(&models.Event{Id: id, Name: "A16 renamed"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	got := reloadEvent(t, id)
	if got.Name != "A16 renamed" {
		t.Errorf("name = %q, want the edit applied", got.Name)
	}
	if got.Description == nil || *got.Description != desc {
		t.Errorf("description = %v, want it left intact by an unset field", got.Description)
	}
}
