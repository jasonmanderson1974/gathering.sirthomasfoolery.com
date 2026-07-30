package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// F12: an event response document with no `response` field unmarshals to a nil
// *Response. getEvent used to key that nil into the responses map and then
// dereference it, so one legacy or hand-edited row 500'd the whole event — the
// app's hottest endpoint — not just that row.
//
// The row is inserted as raw bson rather than a models.EventResponse: the
// struct's `response` tag carries no omitempty, so marshalling the struct would
// write an explicit null instead of omitting the key.
func TestGetEvent_SkipsResponseRowWithNoResponseField(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:           eventId,
		Type:         models.DOW,
		OwnerId:      primitive.NewObjectID(),
		Duration:     float32PtrTest(1),
		NumResponses: intPtrTest(1),
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	defer cleanupEvent(eventId)

	// The bad row: no `response` key at all.
	if _, err := db.EventResponsesCollection.InsertOne(ctx, bson.M{
		"_id":     primitive.NewObjectID(),
		"eventId": eventId,
		"userId":  "legacy-row",
	}); err != nil {
		t.Fatalf("insert legacy response: %v", err)
	}

	// A well-formed guest row alongside it, to prove the good data survives.
	if _, err := db.EventResponsesCollection.InsertOne(ctx, models.EventResponse{
		Id:       primitive.NewObjectID(),
		EventId:  eventId,
		UserId:   "Zoe",
		Response: &models.Response{Name: "Zoe"},
	}); err != nil {
		t.Fatalf("insert guest response: %v", err)
	}

	r := newTestRouter()
	r.GET("/events/:eventId", getEvent)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/events/"+eventId.Hex(), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 — a row with no response field should not fail the event (body: %s)",
			w.Code, w.Body.String())
	}

	var got struct {
		Responses map[string]json.RawMessage `json:"responses"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v (%s)", err, w.Body.String())
	}
	if _, ok := got.Responses["legacy-row"]; ok {
		t.Fatal("the row with no response field should be omitted, not returned as null")
	}
	if _, ok := got.Responses["Zoe"]; !ok {
		t.Fatalf("the well-formed response should still be returned, got %v", got.Responses)
	}
}
