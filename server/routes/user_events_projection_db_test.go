package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// J6: GET /user/events returned whole event documents — embedded rsvps, polls
// with their votes, lists with their items, and the owner's remindees roll — for
// every event the member had ever touched. The dashboard renders a name, a date
// range, a response count and a folder.
//
// Two separate problems: payload size (it grew with club history and with every
// feature that made the document fatter) and disclosure (getEvent strips other
// people's RSVP emails and hides remindees from non-owners, but that is
// per-event logic which never ran on this path).
//
// The assertions are split deliberately: the fields the dashboard needs must be
// present, and the embedded collections must not be — one without the other
// would pass for a projection that is either too narrow or absent.
func TestGetEvents_ProjectsAwayEmbeddedCollections(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	owner := &models.User{Id: primitive.NewObjectID(), Role: models.RoleMember}
	eventId := primitive.NewObjectID()
	shortId := "j6tst"
	isArchived := false
	daysOnly := false

	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:           eventId,
		ShortId:      &shortId,
		Type:         models.SPECIFIC_DATES,
		Name:         "Dashboard Event",
		OwnerId:      owner.Id,
		NumResponses: intPtrTest(4),
		IsArchived:   &isArchived,
		DaysOnly:     &daysOnly,
		Dates:        []primitive.DateTime{primitive.NewDateTimeFromTime(time.Now())},
		Rsvps: map[string]*models.Rsvp{
			"Zoe": {Status: models.RsvpGoing, Name: "Zoe", Email: "zoe@example.com"},
		},
		Polls: []models.Poll{{
			Id:    primitive.NewObjectID(),
			Title: "Where?",
			Options: []models.PollOption{
				{Id: primitive.NewObjectID(), Label: "The pub", Votes: map[string]string{"Zoe": "Zoe"}},
			},
		}},
		Lists: []models.EventList{{
			Id:    primitive.NewObjectID(),
			Name:  "Menu",
			Kind:  models.ListKindText,
			Items: []models.EventListItem{{Id: primitive.NewObjectID(), Text: "Potato salad"}},
		}},
		Remindees: &[]models.Remindee{{Email: "invitee@example.com"}},
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	defer cleanupEvent(eventId)
	defer func() {
		_, _ = db.FoldersCollection.DeleteMany(ctx, bson.M{"userId": owner.Id})
		_, _ = db.FolderEventsCollection.DeleteMany(ctx, bson.M{"userId": owner.Id})
	}()

	r := newTestRouter()
	r.Use(func(c *gin.Context) { c.Set("authUser", owner); c.Next() })
	r.GET("/user/events", getEvents)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/user/events", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	var got []map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v (%s)", err, w.Body.String())
	}

	var event map[string]json.RawMessage
	for _, e := range got {
		var id string
		if err := json.Unmarshal(e["_id"], &id); err == nil && id == eventId.Hex() {
			event = e
			break
		}
	}
	if event == nil {
		t.Fatalf("the seeded event is missing from the dashboard payload: %s", w.Body.String())
	}

	// Everything the dashboard actually renders must survive the projection.
	// EventItem.vue reads all of these; dates feeds getDateRangeStringForEvent.
	for _, field := range []string{"_id", "shortId", "ownerId", "name", "isArchived", "type", "daysOnly", "numResponses", "dates"} {
		raw, ok := event[field]
		if !ok || string(raw) == "null" {
			t.Errorf("dashboard field %q is missing from the projection (got %s)", field, string(raw))
		}
	}

	// The embedded collections must not ride along. They serialize as `null`
	// rather than disappearing — models.Event's json tags carry no omitempty —
	// so absent-or-null is the assertion, not key absence.
	for _, field := range []string{"rsvps", "polls", "lists", "remindees"} {
		raw, ok := event[field]
		if ok && string(raw) != "null" {
			t.Errorf("%q was shipped to the dashboard: %s", field, string(raw))
		}
	}
}
