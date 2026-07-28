package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/middleware"
	"sirtom/server/models"
)

// --- pure logic (no DB) -----------------------------------------------------

func TestSanitizePollInput(t *testing.T) {
	cases := []struct {
		name       string
		title      string
		options    []string
		wantOk     bool
		wantTitle  string
		wantOptLen int
	}{
		{"happy", "  Where? ", []string{"Tom's", " The Lodge ", ""}, true, "Where?", 2},
		{"dedupes options", "Venue", []string{"A", "A", "B"}, true, "Venue", 2},
		{"empty title", "  ", []string{"A", "B"}, false, "", 0},
		{"too few options", "Venue", []string{"A", "", "  "}, false, "", 0},
		{"one option only", "Venue", []string{"only"}, false, "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			title, opts, ok := sanitizePollInput(c.title, c.options)
			if ok != c.wantOk {
				t.Fatalf("ok = %v, want %v", ok, c.wantOk)
			}
			if !ok {
				return
			}
			if title != c.wantTitle {
				t.Errorf("title = %q, want %q", title, c.wantTitle)
			}
			if len(opts) != c.wantOptLen {
				t.Errorf("options len = %d, want %d (%v)", len(opts), c.wantOptLen, opts)
			}
		})
	}
}

func TestSanitizePollInput_CapsOptions(t *testing.T) {
	many := make([]string, 30)
	for i := range many {
		many[i] = string(rune('a' + i)) // distinct labels
	}
	_, opts, ok := sanitizePollInput("Poll", many)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(opts) != maxPollOptions {
		t.Errorf("expected options capped at %d, got %d", maxPollOptions, len(opts))
	}
}

func newTestPoll(allowMultiple bool) (*models.Poll, string, string) {
	optA := primitive.NewObjectID()
	optB := primitive.NewObjectID()
	poll := &models.Poll{
		Id:            primitive.NewObjectID(),
		Title:         "Where?",
		AllowMultiple: allowMultiple,
		Options: []models.PollOption{
			{Id: optA, Label: "Tom's"},
			{Id: optB, Label: "The Lodge"},
		},
	}
	return poll, optA.Hex(), optB.Hex()
}

func TestApplyPollVote_SingleChoiceReplaces(t *testing.T) {
	poll, a, b := newTestPoll(false)

	if err := applyPollVote(poll, "u1", "Alice", []string{a}); err != nil {
		t.Fatalf("vote A: %v", err)
	}
	if _, ok := poll.Options[0].Votes["u1"]; !ok {
		t.Error("expected Alice's vote on option A")
	}

	// Re-voting for B must clear the A vote (single choice).
	if err := applyPollVote(poll, "u1", "Alice", []string{b}); err != nil {
		t.Fatalf("vote B: %v", err)
	}
	if _, ok := poll.Options[0].Votes["u1"]; ok {
		t.Error("A vote should have been cleared after switching to B")
	}
	if _, ok := poll.Options[1].Votes["u1"]; !ok {
		t.Error("expected Alice's vote on option B")
	}
}

func TestApplyPollVote_SingleChoiceRejectsMultiple(t *testing.T) {
	poll, a, b := newTestPoll(false)
	if err := applyPollVote(poll, "u1", "Alice", []string{a, b}); err == nil {
		t.Fatal("expected error picking 2 options on a single-choice poll")
	}
}

func TestApplyPollVote_MultipleAllowed(t *testing.T) {
	poll, a, b := newTestPoll(true)
	if err := applyPollVote(poll, "u1", "Alice", []string{a, b}); err != nil {
		t.Fatalf("multi vote: %v", err)
	}
	if len(poll.Options[0].Votes) != 1 || len(poll.Options[1].Votes) != 1 {
		t.Error("expected Alice on both options")
	}
}

func TestApplyPollVote_InvalidOption(t *testing.T) {
	poll, _, _ := newTestPoll(false)
	if err := applyPollVote(poll, "u1", "Alice", []string{primitive.NewObjectID().Hex()}); err == nil {
		t.Fatal("expected error for unknown option id")
	}
}

func TestApplyPollVote_EmptyClears(t *testing.T) {
	poll, a, _ := newTestPoll(false)
	_ = applyPollVote(poll, "u1", "Alice", []string{a})
	if err := applyPollVote(poll, "u1", "Alice", nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if len(poll.Options[0].Votes) != 0 {
		t.Error("expected vote cleared with empty optionIds")
	}
}

// --- DB-backed handlers -----------------------------------------------------

func TestCreatePoll_OwnerCreatesAndGuestVotes(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	// The poll routes run behind AuthRequired (E3), and requireEventManager
	// reads the caller from the context, so the owner must be a real
	// allowlisted user rather than a bare ObjectID.
	ownerId := insertTestUser(t, models.RoleMember, "poll-owner@example.test")
	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:      eventId,
		Type:    models.SPECIFIC_DATES,
		OwnerId: ownerId,
		Name:    "Poll Event",
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	defer cleanupEvent(eventId)

	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events/:eventId/polls", middleware.AuthRequired(), createPoll)
	r.POST("/events/:eventId/polls/:pollId/vote", middleware.AuthRequired(), votePoll)

	// Owner creates a poll.
	cookie := loginAs(t, r, ownerId.Hex())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+eventId.Hex()+"/polls",
		strings.NewReader(`{"title":"Where?","options":["Tom's","The Lodge"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("createPoll: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	var poll models.Poll
	if err := json.Unmarshal(w.Body.Bytes(), &poll); err != nil {
		t.Fatalf("unmarshal poll: %v", err)
	}
	if len(poll.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(poll.Options))
	}

	// A signed-in member votes on behalf of a guest by name. The route is gated
	// now, so the request carries a session; E3 phase 2 removes the by-name
	// path entirely and keys votes to the session.
	voterId := insertTestUser(t, models.RoleMember, "poll-voter@example.test")
	optId := poll.Options[0].Id.Hex()
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/events/"+eventId.Hex()+"/polls/"+poll.Id.Hex()+"/vote",
		strings.NewReader(`{"guest":true,"name":"Greg","optionIds":["`+optId+`"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginAs(t, r, voterId.Hex()))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("votePoll: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	// Verify the vote persisted on the option.
	var reloaded models.Event
	if err := db.EventsCollection.FindOne(ctx, bson.M{"_id": eventId}).Decode(&reloaded); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.Polls) != 1 {
		t.Fatalf("expected 1 poll, got %d", len(reloaded.Polls))
	}
	if got := reloaded.Polls[0].Options[0].Votes["Greg"]; got != "Greg" {
		t.Errorf("expected Greg's vote on option 0, got %q", got)
	}
}

func TestCreatePoll_NonOwnerForbidden(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	ownerId := insertTestUser(t, models.RoleMember, "poll-realowner@example.test")
	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:      eventId,
		Type:    models.SPECIFIC_DATES,
		OwnerId: ownerId,
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	defer cleanupEvent(eventId)

	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events/:eventId/polls", middleware.AuthRequired(), createPoll)

	// A different signed-in user must not create a poll on someone else's event.
	otherId := insertTestUser(t, models.RoleMember, "poll-nonowner@example.test")
	cookie := loginAs(t, r, otherId.Hex())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+eventId.Hex()+"/polls",
		strings.NewReader(`{"title":"Where?","options":["A","B"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner create: got %d, want 403 (%s)", w.Code, w.Body.String())
	}
}

func TestDeletePoll_OwnerRemoves(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	ownerId := insertTestUser(t, models.RoleMember, "poll-deleter@example.test")
	eventId := primitive.NewObjectID()
	pollId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:      eventId,
		Type:    models.SPECIFIC_DATES,
		OwnerId: ownerId,
		Polls: []models.Poll{
			{Id: pollId, Title: "Where?", Options: []models.PollOption{
				{Id: primitive.NewObjectID(), Label: "A"},
				{Id: primitive.NewObjectID(), Label: "B"},
			}},
		},
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	defer cleanupEvent(eventId)

	r := newTestRouter()
	registerTestLogin(r)
	r.DELETE("/events/:eventId/polls/:pollId", middleware.AuthRequired(), deletePoll)

	cookie := loginAs(t, r, ownerId.Hex())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/events/"+eventId.Hex()+"/polls/"+pollId.Hex(), nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("deletePoll: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	var reloaded models.Event
	db.EventsCollection.FindOne(ctx, bson.M{"_id": eventId}).Decode(&reloaded)
	if len(reloaded.Polls) != 0 {
		t.Errorf("expected poll removed, got %d polls", len(reloaded.Polls))
	}
}
