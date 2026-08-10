package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// J2: Rsvps, Polls and Lists are embedded in the event document, so
// duplicateEvent — which takes the document as read and swaps a few fields —
// used to carry every RSVP, every poll vote and the whole old menu onto the new
// gathering, rendered as if those people had answered for it. Only availability
// was ever opt-in.
//
// The rule the test pins: participation is cleared, the owner's scaffolding
// survives. A poll keeps its title and options and loses its votes; a list keeps
// its name and kind and loses its items. Same for the per-gathering lifecycle
// flags — an inherited GatheringReminder.SentAt would suppress the new
// gathering's reminder email, and an inherited Chronicled would stop the
// scheduler ever capturing it.
func TestDuplicateEvent_DropsLastGatheringsAnswers(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	owner := &models.User{Id: primitive.NewObjectID(), Role: models.RoleMember}
	sourceId := primitive.NewObjectID()
	sentAt := primitive.NewDateTimeFromTime(time.Now())

	source := models.Event{
		Id:      sourceId,
		Type:    models.DOW,
		Name:    "Old Gathering",
		OwnerId: owner.Id,
		Rsvps: map[string]*models.Rsvp{
			"Zoe": {Status: models.RsvpGoing, Name: "Zoe", GuestCount: 2},
		},
		Polls: []models.Poll{{
			Id:    primitive.NewObjectID(),
			Title: "Where?",
			Options: []models.PollOption{
				{Id: primitive.NewObjectID(), Label: "The pub", Votes: map[string]string{"Zoe": "Zoe"}},
				{Id: primitive.NewObjectID(), Label: "The park"},
			},
		}},
		Lists: []models.EventList{{
			Id:   primitive.NewObjectID(),
			Name: "Menu",
			Kind: models.ListKindText,
			Items: []models.EventListItem{
				{Id: primitive.NewObjectID(), Text: "Potato salad", AuthorName: "Zoe"},
			},
		}},
		GatheringReminder: &models.GatheringReminder{Enabled: true, LeadTimeHours: 24, SentAt: &sentAt},
		Chronicled:        true,
	}
	if _, err := db.EventsCollection.InsertOne(ctx, source); err != nil {
		t.Fatalf("insert source event: %v", err)
	}
	defer cleanupEvent(sourceId)

	r := newTestRouter()
	r.Use(func(c *gin.Context) { c.Set("authUser", owner); c.Next() })
	r.POST("/events/:eventId/duplicate", duplicateEvent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+sourceId.Hex()+"/duplicate",
		strings.NewReader(`{"eventName":"New Gathering","copyAvailability":false}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("duplicate: got %d, want 201 (body: %s)", w.Code, w.Body.String())
	}

	dup := findDuplicateForTest(t, ctx, sourceId, "New Gathering")
	defer cleanupEvent(dup.Id)

	if len(dup.Rsvps) != 0 {
		t.Errorf("duplicate inherited %d RSVP(s): %v", len(dup.Rsvps), dup.Rsvps)
	}
	if dup.Chronicled {
		t.Error("duplicate inherited Chronicled=true; the new gathering would never be captured")
	}
	if dup.GatheringReminder == nil {
		t.Fatal("duplicate lost its reminder config entirely; only SentAt should be cleared")
	}
	if dup.GatheringReminder.SentAt != nil {
		t.Error("duplicate inherited GatheringReminder.SentAt; its reminder email would be suppressed")
	}
	if !dup.GatheringReminder.Enabled || dup.GatheringReminder.LeadTimeHours != 24 {
		t.Errorf("reminder config should survive, got %+v", dup.GatheringReminder)
	}

	// Poll scaffolding survives, votes do not.
	if len(dup.Polls) != 1 {
		t.Fatalf("poll should carry over, got %d polls", len(dup.Polls))
	}
	if dup.Polls[0].Title != "Where?" || len(dup.Polls[0].Options) != 2 {
		t.Errorf("poll title/options should survive, got %+v", dup.Polls[0])
	}
	for _, opt := range dup.Polls[0].Options {
		if len(opt.Votes) != 0 {
			t.Errorf("option %q inherited %d vote(s): %v", opt.Label, len(opt.Votes), opt.Votes)
		}
	}

	// List scaffolding survives, items do not.
	if len(dup.Lists) != 1 {
		t.Fatalf("list should carry over, got %d lists", len(dup.Lists))
	}
	if dup.Lists[0].Name != "Menu" || dup.Lists[0].Kind != models.ListKindText {
		t.Errorf("list name/kind should survive, got %+v", dup.Lists[0])
	}
	if len(dup.Lists[0].Items) != 0 {
		t.Errorf("list inherited %d item(s): %v", len(dup.Lists[0].Items), dup.Lists[0].Items)
	}

	// The source is untouched — the handler mutates its in-memory copy only.
	var reread models.Event
	if err := db.EventsCollection.FindOne(ctx, bson.M{"_id": sourceId}).Decode(&reread); err != nil {
		t.Fatalf("re-read source: %v", err)
	}
	if len(reread.Rsvps) != 1 || len(reread.Lists[0].Items) != 1 || len(reread.Polls[0].Options[0].Votes) != 1 {
		t.Error("duplicating an event must not strip the ORIGINAL's answers")
	}
}

// copyAvailability is the one thing that is opt-in, and it now inserts the
// copied responses with a single InsertMany. NumResponses must still match the
// number of rows actually written.
func TestDuplicateEvent_CopyAvailabilityInsertsAllResponses(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	owner := &models.User{Id: primitive.NewObjectID(), Role: models.RoleMember}
	sourceId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:           sourceId,
		Type:         models.DOW,
		Name:         "Sourced",
		OwnerId:      owner.Id,
		NumResponses: intPtrTest(2),
	}); err != nil {
		t.Fatalf("insert source event: %v", err)
	}
	defer cleanupEvent(sourceId)

	for _, name := range []string{"Zoe", "Ari"} {
		if _, err := db.EventResponsesCollection.InsertOne(ctx, models.EventResponse{
			Id:       primitive.NewObjectID(),
			EventId:  sourceId,
			UserId:   name,
			Response: &models.Response{Name: name},
		}); err != nil {
			t.Fatalf("insert response: %v", err)
		}
	}

	r := newTestRouter()
	r.Use(func(c *gin.Context) { c.Set("authUser", owner); c.Next() })
	r.POST("/events/:eventId/duplicate", duplicateEvent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+sourceId.Hex()+"/duplicate",
		strings.NewReader(`{"eventName":"Copied","copyAvailability":true}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("duplicate: got %d, want 201 (body: %s)", w.Code, w.Body.String())
	}

	dup := findDuplicateForTest(t, ctx, sourceId, "Copied")
	defer cleanupEvent(dup.Id)

	count, err := db.EventResponsesCollection.CountDocuments(ctx, bson.M{"eventId": dup.Id})
	if err != nil {
		t.Fatalf("count copied responses: %v", err)
	}
	if count != 2 {
		t.Errorf("copied response rows: got %d, want 2", count)
	}
	if dup.NumResponses == nil || *dup.NumResponses != 2 {
		t.Errorf("NumResponses: got %v, want 2", dup.NumResponses)
	}
}

// An event with nothing to copy must not trip InsertMany's empty-batch error.
func TestDuplicateEvent_CopyAvailabilityWithNoResponses(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	owner := &models.User{Id: primitive.NewObjectID(), Role: models.RoleMember}
	sourceId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:      sourceId,
		Type:    models.DOW,
		Name:    "Empty",
		OwnerId: owner.Id,
	}); err != nil {
		t.Fatalf("insert source event: %v", err)
	}
	defer cleanupEvent(sourceId)

	r := newTestRouter()
	r.Use(func(c *gin.Context) { c.Set("authUser", owner); c.Next() })
	r.POST("/events/:eventId/duplicate", duplicateEvent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+sourceId.Hex()+"/duplicate",
		strings.NewReader(`{"eventName":"Empty Copy","copyAvailability":true}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("duplicating an event with no responses: got %d, want 201 (body: %s)",
			w.Code, w.Body.String())
	}

	dup := findDuplicateForTest(t, ctx, sourceId, "Empty Copy")
	defer cleanupEvent(dup.Id)

	if dup.NumResponses == nil || *dup.NumResponses != 0 {
		t.Errorf("NumResponses: got %v, want 0", dup.NumResponses)
	}
}

// findDuplicateForTest fetches the event duplicateEvent just created. It matches
// on the new name rather than the response body so the stored document — the
// thing that actually carries the embedded answers — is what gets asserted on.
func findDuplicateForTest(t *testing.T, ctx context.Context, sourceId primitive.ObjectID, name string) models.Event {
	t.Helper()
	var dup models.Event
	err := db.EventsCollection.FindOne(ctx, bson.M{
		"name": name,
		"_id":  bson.M{"$ne": sourceId},
	}).Decode(&dup)
	if err != nil {
		t.Fatalf("find duplicate %q: %v", name, err)
	}
	return dup
}
