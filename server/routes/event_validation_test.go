package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/middleware"
	"sirtom/server/models"
)

// E11 + E8: the create/edit payloads bound Type with `binding:"required"`, which
// only rejects "", and left Name/Dates/Times/Remindees/SignUpBlocks unbounded.

func TestIsKnownEventType(t *testing.T) {
	for _, tc := range []struct {
		in   models.EventType
		want bool
	}{
		{models.SPECIFIC_DATES, true},
		{models.DOW, true},
		{"", false},
		{"specificDates", false}, // the camelCase spelling that surfaced this
		{"garbage", false},
	} {
		if got := models.IsKnownEventType(tc.in); got != tc.want {
			t.Errorf("IsKnownEventType(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Errorf("under the cap should pass through, got %q", got)
	}
	if got := truncateRunes("hello", 3); got != "hel" {
		t.Errorf("over the cap: got %q, want %q", got, "hel")
	}
	// Rune-aware: cutting a multi-byte string must not leave invalid UTF-8.
	emoji := strings.Repeat("🥃", 5)
	got := truncateRunes(emoji, 3)
	if got != "🥃🥃🥃" {
		t.Errorf("emoji truncation: got %q, want 3 glasses", got)
	}
	if !utf8Valid(got) {
		t.Error("truncation produced invalid UTF-8")
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestSanitizeEventText(t *testing.T) {
	name, desc := sanitizeEventText("  Poker Night  ", nil)
	if name != "Poker Night" {
		t.Errorf("name should be trimmed, got %q", name)
	}
	if desc != nil {
		t.Error("nil description must stay nil so 'absent' stays distinct from 'cleared'")
	}

	long := strings.Repeat("x", maxEventNameLength+50)
	d := strings.Repeat("y", maxEventDescriptionLength+50)
	name, desc = sanitizeEventText(long, &d)
	if len([]rune(name)) != maxEventNameLength {
		t.Errorf("name len = %d, want %d", len([]rune(name)), maxEventNameLength)
	}
	if desc == nil || len([]rune(*desc)) != maxEventDescriptionLength {
		t.Errorf("description not capped to %d", maxEventDescriptionLength)
	}
}

func TestSanitizeResponderName(t *testing.T) {
	if got := sanitizeResponderName("  Alice  "); got != "Alice" {
		t.Errorf("got %q, want %q", got, "Alice")
	}
	if got := sanitizeResponderName("   "); got != "" {
		t.Errorf("whitespace-only should collapse to empty, got %q", got)
	}
	long := strings.Repeat("n", maxResponderNameLength+40)
	if got := sanitizeResponderName(long); len([]rune(got)) != maxResponderNameLength {
		t.Errorf("len = %d, want %d", len([]rune(got)), maxResponderNameLength)
	}
}

// validateEventPayload writes the response itself, so drive it through a real
// context and assert on both the status and the error code.
func runValidate(t *testing.T, p eventPayloadLimits) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/events", nil)

	ok := validateEventPayload(c, p)
	if ok {
		return http.StatusOK, ""
	}
	var body struct {
		Error string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body.Error
}

func TestValidateEventPayload_RejectsUnknownType(t *testing.T) {
	for _, bad := range []models.EventType{"", "specificDates", "nonsense"} {
		code, errCode := runValidate(t, eventPayloadLimits{Type: bad})
		if code != http.StatusBadRequest || errCode != errs.InvalidEventType {
			t.Errorf("type %q: got (%d, %q), want (400, %q)", bad, code, errCode, errs.InvalidEventType)
		}
	}
}

func TestValidateEventPayload_AcceptsKnownTypes(t *testing.T) {
	for _, good := range []models.EventType{models.SPECIFIC_DATES, models.DOW} {
		if code, errCode := runValidate(t, eventPayloadLimits{Type: good}); code != http.StatusOK {
			t.Errorf("type %q should pass, got (%d, %q)", good, code, errCode)
		}
	}
}

func TestValidateEventPayload_RejectsOversizedCollections(t *testing.T) {
	dates := make([]primitive.DateTime, maxEventDates+1)
	times := make([]primitive.DateTime, maxEventTimes+1)
	remindees := make([]string, maxEventRemindees+1)
	blocks := make([]models.SignUpBlock, maxEventSignUpBlocks+1)

	for name, p := range map[string]eventPayloadLimits{
		"dates":        {Type: models.DOW, Dates: dates},
		"times":        {Type: models.DOW, Times: times},
		"remindees":    {Type: models.DOW, Remindees: remindees},
		"signUpBlocks": {Type: models.DOW, SignUpBlocks: &blocks},
	} {
		code, errCode := runValidate(t, p)
		if code != http.StatusBadRequest || errCode != errs.PayloadTooLarge {
			t.Errorf("oversized %s: got (%d, %q), want (400, %q)", name, code, errCode, errs.PayloadTooLarge)
		}
	}
}

// Exactly at the cap must pass — the caps are generous by design and an
// off-by-one here would reject legitimate events.
func TestValidateEventPayload_AllowsExactlyAtCap(t *testing.T) {
	blocks := make([]models.SignUpBlock, maxEventSignUpBlocks)
	code, errCode := runValidate(t, eventPayloadLimits{
		Type:         models.SPECIFIC_DATES,
		Dates:        make([]primitive.DateTime, maxEventDates),
		Times:        make([]primitive.DateTime, maxEventTimes),
		Remindees:    make([]string, maxEventRemindees),
		SignUpBlocks: &blocks,
	})
	if code != http.StatusOK {
		t.Errorf("at-cap payload should pass, got (%d, %q)", code, errCode)
	}
}

// A nil SignUpBlocks pointer (the common case — most events aren't sign-up
// forms) must not be dereferenced.
func TestValidateEventPayload_NilSignUpBlocksSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil SignUpBlocks panicked: %v", r)
		}
	}()
	if code, _ := runValidate(t, eventPayloadLimits{Type: models.DOW}); code != http.StatusOK {
		t.Errorf("nil SignUpBlocks should pass, got %d", code)
	}
}

// The pure tests above can't catch a handler that forgets to call the
// validator, so drive the real createEvent once with the exact payload that
// surfaced E11: the camelCase "specificDates" instead of "specific_dates".
func TestCreateEvent_RejectsUnknownTypeEndToEnd(t *testing.T) {
	requireDB(t)
	// createEvent runs behind AuthRequired since E3 and takes its owner from the
	// context, so drive the real chain.
	userId := insertTestUser(t, models.RoleMember, "e11-reject@example.test")
	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events", middleware.AuthRequired(), createEvent)

	body := `{"name":"Poker Night","duration":2,"dates":["2026-08-01T00:00:00Z"],"type":"specificDates"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginAs(t, r, userId.Hex()))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != errs.InvalidEventType {
		t.Errorf("error = %q, want %q", resp.Error, errs.InvalidEventType)
	}
}

// The valid spelling must still create the event — proof the guard isn't
// rejecting everything.
func TestCreateEvent_AcceptsKnownTypeEndToEnd(t *testing.T) {
	requireDB(t)
	userId := insertTestUser(t, models.RoleMember, "e11-accept@example.test")
	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events", middleware.AuthRequired(), createEvent)

	body := `{"name":"Poker Night","duration":2,"dates":["2026-08-01T00:00:00Z"],"type":"specific_dates"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginAs(t, r, userId.Hex()))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201 (body: %s)", w.Code, w.Body.String())
	}
	var resp struct {
		EventId string `json:"eventId"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.EventId == "" {
		t.Fatal("expected an eventId back")
	}
	if id, err := primitive.ObjectIDFromHex(resp.EventId); err == nil {
		t.Cleanup(func() { cleanupEvent(id) })
	}
}

// --- E3 phase 2: anonymous guest semantics removed ---------------------------

// Responses are keyed by user-id for members and by display name for guests in
// the SAME map, so an ObjectID-shaped on-behalf name would collide with, and
// overwrite, a member's response.
func TestValidOnBehalfName(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"Greg", true},
		{"Mary-Anne O'Brien", true},
		{"", false},
		{"507f1f77bcf86cd799439011", false}, // lowercase hex ObjectID
		{"507F1F77BCF86CD799439011", false}, // uppercase
		{"507f1f77bcf86cd79943901", true},   // 23 chars — not an ObjectID
		{"507f1f77bcf86cd7994390111", true}, // 25 chars — not an ObjectID
		{"507f1f77bcf86cd79943901g", true},  // 24 chars but not hex
	} {
		if got := validOnBehalfName(tc.name); got != tc.want {
			t.Errorf("validOnBehalfName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A signed-in caller must not be able to overwrite a member's response by
// submitting that member's user id as an on-behalf "guest" name.
func TestUpdateEventResponse_RejectsObjectIDShapedGuestName(t *testing.T) {
	requireDB(t)
	victimId := primitive.NewObjectID()
	callerId := insertTestUser(t, models.RoleMember, "onbehalf-caller@example.test")

	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(context.Background(), models.Event{
		Id: eventId, Type: models.SPECIFIC_DATES, OwnerId: primitive.NewObjectID(),
		NumResponses: intPtrTest(0),
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { cleanupEvent(eventId) })

	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events/:eventId/response", middleware.AuthRequired(), updateEventResponse)

	body := `{"guest":true,"name":"` + victimId.Hex() + `","availability":[]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+eventId.Hex()+"/response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginAs(t, r, callerId.Hex()))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ObjectID-shaped guest name: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
}

// A plain on-behalf name still works — only anonymous use and impersonation are
// gone, not the spouse-without-an-account flow.
func TestUpdateEventResponse_AllowsPlainOnBehalfName(t *testing.T) {
	requireDB(t)
	callerId := insertTestUser(t, models.RoleMember, "onbehalf-ok@example.test")

	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(context.Background(), models.Event{
		Id: eventId, Type: models.SPECIFIC_DATES, OwnerId: primitive.NewObjectID(),
		NumResponses: intPtrTest(0),
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { cleanupEvent(eventId) })

	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events/:eventId/response", middleware.AuthRequired(), updateEventResponse)

	body := `{"guest":true,"name":"Mary","availability":[]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+eventId.Hex()+"/response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginAs(t, r, callerId.Hex()))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("on-behalf entry should still work: got %d (body: %s)", w.Code, w.Body.String())
	}
}

// Deleting a by-name response is acting on someone else's data. Previously this
// branch had no authorization at all.
func TestDeleteEventResponse_GuestBranchRequiresOwner(t *testing.T) {
	requireDB(t)
	strangerId := insertTestUser(t, models.RoleMember, "stranger@example.test")

	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(context.Background(), models.Event{
		Id: eventId, Type: models.SPECIFIC_DATES, OwnerId: primitive.NewObjectID(),
		NumResponses: intPtrTest(1),
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { cleanupEvent(eventId) })

	r := newTestRouter()
	registerTestLogin(r)
	r.DELETE("/events/:eventId/response", middleware.AuthRequired(), deleteEventResponse)

	body := `{"guest":true,"name":"Mary","userId":""}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/events/"+eventId.Hex()+"/response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginAs(t, r, strangerId.Hex()))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner deleting a named response: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
}

// Same for renaming, which was unauthenticated entirely.
func TestRenameUser_RequiresOwner(t *testing.T) {
	requireDB(t)
	strangerId := insertTestUser(t, models.RoleMember, "rename-stranger@example.test")

	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(context.Background(), models.Event{
		Id: eventId, Type: models.SPECIFIC_DATES, OwnerId: primitive.NewObjectID(),
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { cleanupEvent(eventId) })

	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events/:eventId/rename-user", middleware.AuthRequired(), renameUser)

	body := `{"oldName":"Mary","newName":"Mary S"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+eventId.Hex()+"/rename-user", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginAs(t, r, strangerId.Hex()))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner renaming: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
}

// The RSVP is keyed to the session, so a spoofed name in the body is ignored.
func TestRsvp_IgnoresSpoofedName(t *testing.T) {
	requireDB(t)
	callerId := insertTestUser(t, models.RoleMember, "rsvp-caller@example.test")

	eventId := primitive.NewObjectID()
	start := primitive.NewDateTimeFromTime(time.Now().Add(48 * time.Hour))
	if _, err := db.EventsCollection.InsertOne(context.Background(), models.Event{
		Id: eventId, Type: models.SPECIFIC_DATES, OwnerId: primitive.NewObjectID(),
		ScheduledEvent: &models.CalendarEvent{StartDate: start, EndDate: start},
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { cleanupEvent(eventId) })

	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events/:eventId/rsvp", middleware.AuthRequired(), rsvpToEvent)

	body := `{"status":"going","guest":true,"name":"NotMe","email":"attacker@example.test"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+eventId.Hex()+"/rsvp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginAs(t, r, callerId.Hex()))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("rsvp: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	var reloaded models.Event
	if err := db.EventsCollection.FindOne(context.Background(),
		bson.M{"_id": eventId}).Decode(&reloaded); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, spoofed := reloaded.Rsvps["NotMe"]; spoofed {
		t.Error("the request-body name was used as the RSVP key — RSVPs must be session-keyed")
	}
	rsvp, ok := reloaded.Rsvps[callerId.Hex()]
	if !ok {
		t.Fatalf("expected an RSVP keyed to the caller, got keys %v", reloaded.Rsvps)
	}
	if rsvp.Email == "attacker@example.test" {
		t.Error("email came from the request body — it must come from the account")
	}
}
