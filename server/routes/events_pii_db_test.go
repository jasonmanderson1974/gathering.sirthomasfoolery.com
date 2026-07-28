package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// E6: getEvent used to serialize RSVP email addresses and the owner's remindee
// invite list to every viewer, and stripSensitiveUserFields left respondents'
// phone numbers and roles in place. These tests pin the rule that only the
// owner (and, for emails, only with collectEmails on) sees any of it.

func boolPtrPII(b bool) *bool { return &b }

// seedPIIEvent inserts an event that carries all three PII channels: an RSVP
// with an email, a remindee list, and a respondent user with a phone + role.
func seedPIIEvent(t *testing.T, collectEmails bool) (eventId, ownerId, responderId primitive.ObjectID) {
	t.Helper()
	ctx := context.Background()

	ownerId = primitive.NewObjectID()
	responderId = primitive.NewObjectID()
	eventId = primitive.NewObjectID()

	if _, err := db.UsersCollection.InsertOne(ctx, models.User{
		Id:        responderId,
		Email:     "responder@example.test",
		FirstName: "Res",
		LastName:  "Ponder",
		Phone:     "+15555550123",
		Role:      models.RoleMember,
	}); err != nil {
		t.Fatalf("insert responder: %v", err)
	}
	t.Cleanup(func() { deleteTestUser(responderId) })

	remindees := []models.Remindee{{Email: "invitee@example.test"}}
	start := primitive.NewDateTimeFromTime(time.Now().Add(48 * time.Hour))
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:            eventId,
		Type:          models.DOW,
		OwnerId:       ownerId,
		Name:          "PII test event",
		CollectEmails: boolPtrPII(collectEmails),
		Remindees:     &remindees,
		ScheduledEvent: &models.CalendarEvent{
			StartDate: start,
			EndDate:   primitive.NewDateTimeFromTime(start.Time().Add(2 * time.Hour)),
		},
		Rsvps: map[string]*models.Rsvp{
			responderId.Hex(): {
				Status: models.RsvpGoing,
				Name:   "Res Ponder",
				Email:  "responder@example.test",
				UserId: responderId,
			},
		},
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { cleanupEvent(eventId) })

	if _, err := db.EventResponsesCollection.InsertOne(ctx, models.EventResponse{
		Id:      primitive.NewObjectID(),
		EventId: eventId,
		UserId:  responderId.Hex(),
		Response: &models.Response{
			UserId: responderId,
		},
	}); err != nil {
		t.Fatalf("insert response: %v", err)
	}

	return eventId, ownerId, responderId
}

// fetchEvent drives getEvent as the given user (empty userId = no session) and
// returns the decoded JSON body.
func fetchEvent(t *testing.T, eventId primitive.ObjectID, asUserId string) map[string]interface{} {
	t.Helper()
	r := newTestRouter()
	registerTestLogin(r)
	r.GET("/events/:eventId", getEvent)

	req := httptest.NewRequest(http.MethodGet, "/events/"+eventId.Hex(), nil)
	if asUserId != "" {
		req.AddCookie(loginAs(t, r, asUserId))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("getEvent: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func rsvpEmail(t *testing.T, body map[string]interface{}, key string) string {
	t.Helper()
	rsvps, ok := body["rsvps"].(map[string]interface{})
	if !ok {
		t.Fatalf("rsvps missing or wrong shape: %#v", body["rsvps"])
	}
	rsvp, ok := rsvps[key].(map[string]interface{})
	if !ok {
		t.Fatalf("rsvp %q missing: %#v", key, rsvps)
	}
	email, _ := rsvp["email"].(string)
	return email
}

// A non-owner must not receive RSVP email addresses or the remindee list, but
// must still get the RSVP itself (status/name drive the headcount UI).
func TestGetEvent_HidesRsvpEmailAndRemindeesFromNonOwner(t *testing.T) {
	requireDB(t)
	eventId, _, responderId := seedPIIEvent(t, true)

	body := fetchEvent(t, eventId, responderId.Hex())

	if got := rsvpEmail(t, body, responderId.Hex()); got != "" {
		t.Errorf("non-owner saw RSVP email %q, want it stripped", got)
	}
	if rem := body["remindees"]; rem != nil {
		t.Errorf("non-owner saw remindees %#v, want nil", rem)
	}
	// The RSVP itself must survive — only the email is confidential.
	rsvps := body["rsvps"].(map[string]interface{})
	rsvp := rsvps[responderId.Hex()].(map[string]interface{})
	if rsvp["name"] != "Res Ponder" || rsvp["status"] != string(models.RsvpGoing) {
		t.Errorf("RSVP identity/status must survive stripping, got %#v", rsvp)
	}
}

// Anonymous viewers get the same treatment (no session at all).
func TestGetEvent_HidesRsvpEmailAndRemindeesFromAnonymous(t *testing.T) {
	requireDB(t)
	eventId, _, responderId := seedPIIEvent(t, true)

	body := fetchEvent(t, eventId, "")

	if got := rsvpEmail(t, body, responderId.Hex()); got != "" {
		t.Errorf("anonymous viewer saw RSVP email %q, want it stripped", got)
	}
	if rem := body["remindees"]; rem != nil {
		t.Errorf("anonymous viewer saw remindees %#v, want nil", rem)
	}
}

// The owner keeps both — remindees unconditionally (the edit form prefills from
// them), RSVP emails only when collectEmails is on, matching the responses rule.
func TestGetEvent_OwnerKeepsRemindeesAndRsvpEmailWhenCollecting(t *testing.T) {
	requireDB(t)
	eventId, ownerId, responderId := seedPIIEvent(t, true)

	body := fetchEvent(t, eventId, ownerId.Hex())

	if got := rsvpEmail(t, body, responderId.Hex()); got != "responder@example.test" {
		t.Errorf("owner with collectEmails on should see RSVP email, got %q", got)
	}
	if body["remindees"] == nil {
		t.Error("owner should still receive the remindee list")
	}
}

// collectEmails off means even the owner gets no addresses — same gate the
// response emails already used.
func TestGetEvent_OwnerLosesRsvpEmailWhenNotCollecting(t *testing.T) {
	requireDB(t)
	eventId, ownerId, responderId := seedPIIEvent(t, false)

	body := fetchEvent(t, eventId, ownerId.Hex())

	if got := rsvpEmail(t, body, responderId.Hex()); got != "" {
		t.Errorf("collectEmails off: owner saw RSVP email %q, want it stripped", got)
	}
	if body["remindees"] == nil {
		t.Error("owner should still receive the remindee list regardless of collectEmails")
	}
}

// The respondent's phone number and role must never reach an event viewer.
func TestGetEvent_StripsResponderPhoneAndRole(t *testing.T) {
	requireDB(t)
	eventId, ownerId, _ := seedPIIEvent(t, true)

	// Check as the owner — the most privileged viewer. If it's stripped here it
	// is stripped for everyone.
	body := fetchEvent(t, eventId, ownerId.Hex())

	responses, ok := body["responses"].(map[string]interface{})
	if !ok {
		t.Fatalf("responses missing or wrong shape: %#v", body["responses"])
	}
	for key, raw := range responses {
		resp, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		user, ok := resp["user"].(map[string]interface{})
		if !ok {
			continue
		}
		if phone, _ := user["phone"].(string); phone != "" {
			t.Errorf("response %q leaked phone %q", key, phone)
		}
		if role, _ := user["role"].(string); role != "" {
			t.Errorf("response %q leaked role %q", key, role)
		}
	}
}
