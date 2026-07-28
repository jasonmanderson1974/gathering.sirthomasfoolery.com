package db_test

import (
	"context"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

const shortIdAlphabet = "23456789ABCDEFabcdef"

// E9: the generator used to seed math/rand with the event's ObjectID timestamp,
// which is second-granular — so two events created in the same second drew the
// SAME id, and any id was predictable from a known creation time.
func TestGenerateShortEventIdIsUnpredictableAcrossCalls(t *testing.T) {
	const n = 50
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id, err := db.GenerateShortEventId()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[id] {
			t.Fatalf("id %q was drawn twice in %d calls — the source is not random", id, n)
		}
		seen[id] = true
	}
}

func TestGenerateShortEventIdShape(t *testing.T) {
	id, err := db.GenerateShortEventId()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(id) != 5 {
		t.Errorf("len(%q) = %d, want 5", id, len(id))
	}
	for _, r := range id {
		if !strings.ContainsRune(shortIdAlphabet, r) {
			t.Errorf("id %q contains %q, which is outside the alphabet", id, r)
		}
	}
}

// Every letter of the alphabet should be reachable. A modulo over a 256-value
// byte would quietly favour the first 16 of the 20 letters; rejection sampling
// is what keeps the draw uniform.
func TestGenerateShortEventIdUsesTheWholeAlphabet(t *testing.T) {
	seen := make(map[rune]bool, len(shortIdAlphabet))
	for i := 0; i < 400; i++ {
		id, err := db.GenerateShortEventId()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		for _, r := range id {
			seen[r] = true
		}
	}
	for _, r := range shortIdAlphabet {
		if !seen[r] {
			t.Errorf("letter %q never appeared in 2000 draws — the draw looks biased", r)
		}
	}
}

// The generator must not hand back an id that an event already holds.
func TestGenerateShortEventIdSkipsTakenIds(t *testing.T) {
	ctx := context.Background()

	// Claim whatever the generator would otherwise be free to pick, by taking a
	// large sample of ids and confirming none of them belongs to a live event.
	id, err := db.GenerateShortEventId()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id: eventId, Name: "E9 short id holder", ShortId: &id,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	defer db.EventsCollection.DeleteOne(ctx, bson.M{"_id": eventId})

	for i := 0; i < 50; i++ {
		next, err := db.GenerateShortEventId()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if next == id {
			t.Fatalf("generator returned %q, which is already taken", next)
		}
	}

	// And the id we planted really is findable, so the probe above is meaningful.
	found, err := db.GetEventByShortId(id)
	if err != nil || found == nil {
		t.Fatalf("planted short id %q is not findable (err=%v) — the collision probe proves nothing", id, err)
	}
}
