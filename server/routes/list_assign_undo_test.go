package routes

import (
	"runtime"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

func newTestUndoStore(t *testing.T) *assignUndoStore {
	t.Helper()
	s := newAssignUndoStore()
	t.Cleanup(s.Stop)
	return s
}

func sampleRecord(eventId primitive.ObjectID) assignUndoRecord {
	return assignUndoRecord{
		EventId: eventId,
		ListId:  primitive.NewObjectID(),
		Prior:   []priorAssignment{{ItemId: primitive.NewObjectID()}},
	}
}

func TestAssignUndoStore_RoundTrip(t *testing.T) {
	s := newTestUndoStore(t)
	userId, eventId := primitive.NewObjectID(), primitive.NewObjectID()

	token := s.Remember(userId, sampleRecord(eventId))
	if token == "" {
		t.Fatal("Remember returned no token")
	}

	got, ok := s.Take(userId, eventId, token)
	if !ok {
		t.Fatal("Take found nothing straight after Remember")
	}
	if got.EventId != eventId {
		t.Errorf("wrong record returned: %+v", got)
	}

	// Taken exactly once: a double-click on Undo gets one restore, not two
	// writes.
	if _, ok := s.Take(userId, eventId, token); ok {
		t.Error("the same record was taken twice")
	}
}

func TestAssignUndoStore_RefusesTheWrongCaller(t *testing.T) {
	s := newTestUndoStore(t)
	mine, theirs := primitive.NewObjectID(), primitive.NewObjectID()
	eventId := primitive.NewObjectID()

	token := s.Remember(mine, sampleRecord(eventId))

	if _, ok := s.Take(theirs, eventId, token); ok {
		t.Error("another member undid an action that was not theirs")
	}
	// And mine still stands, rather than having been consumed by their attempt.
	if _, ok := s.Take(mine, eventId, token); !ok {
		t.Error("a failed attempt by somebody else consumed my record")
	}
}

func TestAssignUndoStore_RefusesAStaleToken(t *testing.T) {
	s := newTestUndoStore(t)
	userId, eventId := primitive.NewObjectID(), primitive.NewObjectID()

	first := s.Remember(userId, sampleRecord(eventId))
	// A second action supersedes the first: the older Undo button must not
	// restore the newer action's snapshot.
	second := s.Remember(userId, sampleRecord(eventId))

	if _, ok := s.Take(userId, eventId, first); ok {
		t.Error("a superseded token was honoured")
	}
	if _, ok := s.Take(userId, eventId, second); !ok {
		t.Error("the current token was refused")
	}
}

func TestAssignUndoStore_RefusesAnotherGathering(t *testing.T) {
	s := newTestUndoStore(t)
	userId := primitive.NewObjectID()
	eventId := primitive.NewObjectID()

	token := s.Remember(userId, sampleRecord(eventId))

	if _, ok := s.Take(userId, primitive.NewObjectID(), token); ok {
		t.Error("a record was applied to a different gathering")
	}
}

// Expiry is checked on READ as well as swept by the janitor, so a record can
// never be honoured late just because the ticker has not come round yet.
func TestAssignUndoStore_ExpiresOnRead(t *testing.T) {
	s := newTestUndoStore(t)
	userId, eventId := primitive.NewObjectID(), primitive.NewObjectID()

	token := s.Remember(userId, sampleRecord(eventId))

	// Age the record past the window without sleeping through it.
	s.mu.Lock()
	record := s.records[userId]
	record.At = time.Now().Add(-undoWindow - time.Second)
	s.records[userId] = record
	s.mu.Unlock()

	if _, ok := s.Take(userId, eventId, token); ok {
		t.Error("an expired record was honoured")
	}
	s.mu.Lock()
	_, still := s.records[userId]
	s.mu.Unlock()
	if still {
		t.Error("an expired record was left in the map after being read")
	}
}

func TestAssignUndoStore_EvictsStale(t *testing.T) {
	s := newTestUndoStore(t)
	fresh, stale := primitive.NewObjectID(), primitive.NewObjectID()
	eventId := primitive.NewObjectID()

	s.Remember(fresh, sampleRecord(eventId))
	s.Remember(stale, sampleRecord(eventId))

	s.mu.Lock()
	record := s.records[stale]
	record.At = time.Now().Add(-time.Hour)
	s.records[stale] = record
	s.mu.Unlock()

	s.evictStale(undoWindow)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[stale]; ok {
		t.Error("a stale record survived the sweep")
	}
	if _, ok := s.records[fresh]; !ok {
		t.Error("the sweep took a record that was still live")
	}
}

// The window the server honours must be LONGER than the one the button is shown
// for, or a button still on screen would be routinely refused.
func TestAssignUndoStore_WindowOutlivesTheButton(t *testing.T) {
	const buttonWindow = 7 * time.Second
	if undoWindow <= buttonWindow {
		t.Errorf("undoWindow %v must exceed the %v the UI shows the button for", undoWindow, buttonWindow)
	}
}

func TestAssignUndoStore_StopHaltsTheJanitor(t *testing.T) {
	before := runtime.NumGoroutine()
	s := newAssignUndoStore()
	s.Stop()
	// Stop is idempotent — the singleton never calls it, tests call it twice by
	// way of t.Cleanup, and a second close() would panic.
	s.Stop()

	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if runtime.NumGoroutine() > before {
		t.Error("the janitor goroutine outlived Stop")
	}
}

func TestSnapshotAssignments(t *testing.T) {
	held := primitive.NewObjectID()
	kept, cleared, gone := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()

	list := &models.EventList{Items: []models.EventListItem{
		{Id: kept, AssigneeId: &held, AssigneeName: "Bart"},
		{Id: cleared},
	}}

	// `gone` is not on the list: it was removed between the read and here.
	prior := snapshotAssignments(list, []primitive.ObjectID{kept, cleared, gone})

	if len(prior) != 2 {
		t.Fatalf("got %d entries, want 2 — a removed item must not be recorded: %+v", len(prior), prior)
	}
	if prior[0].AssigneeId == nil || *prior[0].AssigneeId != held || prior[0].AssigneeName != "Bart" {
		t.Errorf("a held entry was not captured: %+v", prior[0])
	}
	// Unassigned is a state worth restoring exactly as much as a name is.
	if prior[1].AssigneeId != nil || prior[1].AssigneeName != "" {
		t.Errorf("an unassigned entry was not captured as unassigned: %+v", prior[1])
	}
}
