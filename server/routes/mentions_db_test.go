package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/middleware"
	"sirtom/server/models"
)

// mentionsTestRouter wires the mentionables endpoint plus the comment write
// path, behind the same AuthRequired middleware production uses — the write-path
// stripping is half of what these tests are checking.
func mentionsTestRouter() *gin.Engine {
	r := newTestRouter()
	registerTestLogin(r)
	r.GET("/events/:eventId/mentionables", middleware.AuthRequired(), getMentionables)
	r.POST("/events/:eventId/comments", middleware.AuthRequired(), addComment)
	r.PUT("/events/:eventId/comments/:commentId", middleware.AuthRequired(), editComment)
	return r
}

// getMentionablesFor calls the endpoint and returns the ids it offered.
func getMentionablesFor(t *testing.T, h *gin.Engine, eventId primitive.ObjectID, cookie *http.Cookie) map[string]models.User {
	t.Helper()
	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/mentionables", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("mentionables: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var users []models.User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("decode mentionables: %v (body: %s)", err, w.Body.String())
	}
	byId := make(map[string]models.User, len(users))
	for _, u := range users {
		byId[u.Id.Hex()] = u
	}
	return byId
}

// readComment re-reads a comment from Mongo, so assertions check what actually
// landed rather than what the handler echoed back.
func readComment(t *testing.T, commentId string) *models.Comment {
	t.Helper()
	stored, err := db.GetCommentById(commentId)
	if err != nil || stored == nil {
		t.Fatalf("re-read comment %s: %v", commentId, err)
	}
	return stored
}

// postAndReadComment posts a comment and hands back what was stored.
func postAndReadComment(t *testing.T, h *gin.Engine, eventId primitive.ObjectID, cookie *http.Cookie, body string) *models.Comment {
	t.Helper()
	return readComment(t, postComment(t, h, eventId, cookie, body))
}

// mentionIds renders a comment's stored mentions as hexes for comparison.
func mentionIds(comment *models.Comment) []string {
	ids := make([]string, 0, len(comment.Mentions))
	for _, id := range comment.Mentions {
		ids = append(ids, id.Hex())
	}
	return ids
}

// rsvpAs records an RSVP straight into the event doc, putting that account on
// the event without going through the RSVP handler.
func rsvpAs(t *testing.T, eventId, userId primitive.ObjectID) {
	t.Helper()
	_, err := db.EventsCollection.UpdateByID(context.Background(), eventId, bson.M{
		"$set": bson.M{"rsvps." + userId.Hex(): bson.M{"status": models.RsvpGoing}},
	})
	if err != nil {
		t.Fatalf("rsvp for %s: %v", userId.Hex(), err)
	}
}

// --- the mentionables endpoint ---------------------------------------------

func TestMentionables_RequiresSignIn(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "mention-signin@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := mentionsTestRouter()

	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/mentionables", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: got %d, want 401 (body: %s)", w.Code, w.Body.String())
	}
}

// A member may mention anyone on the roll, including people with no connection
// to this event at all.
func TestMentionables_MemberSeesTheWholeRoll(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "mention-roll-owner@example.test")
	strangerId := insertTestUser(t, models.RoleMember, "mention-roll-stranger@example.test")
	eventId := insertTestEvent(t, ownerId)

	h := mentionsTestRouter()
	got := getMentionablesFor(t, h, eventId, loginAs(t, h, ownerId.Hex()))

	// Containment, not an exact count: the roll is the whole users collection,
	// which on a developer's machine may hold rows this test didn't create.
	if _, ok := got[strangerId.Hex()]; !ok {
		t.Error("a member should be able to mention someone not on the event")
	}
	if _, ok := got[ownerId.Hex()]; !ok {
		t.Error("the caller should appear on the roll too")
	}
}

// The picker is the one endpoint that hands a caller the whole roll, so what it
// omits matters: a mention picker needs a name and a face and nothing else.
func TestMentionables_PayloadCarriesNoContactDetails(t *testing.T) {
	requireDB(t)

	callerId := insertTestUser(t, models.RoleMember, "mention-slim-caller@example.test")
	eventId := insertTestEvent(t, callerId)

	// Give the target every field a slim payload must drop.
	targetId := insertTestUser(t, models.RoleMember, "mention-slim-target@example.test")
	if _, err := db.UsersCollection.UpdateByID(context.Background(), targetId, bson.M{
		"$set": bson.M{"phone": "555-0100", "role": string(models.RoleAdmin)},
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	h := mentionsTestRouter()
	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/mentionables", "", loginAs(t, h, callerId.Hex()))
	if w.Code != http.StatusOK {
		t.Fatalf("mentionables: got %d, want 200", w.Code)
	}

	body := w.Body.String()
	for _, leaked := range []string{"mention-slim-target@example.test", "555-0100", string(models.RoleAdmin)} {
		if strings.Contains(body, leaked) {
			t.Errorf("mentionables payload leaked %q", leaked)
		}
	}

	// ...and still carries what the picker actually needs.
	got := getMentionablesFor(t, h, eventId, loginAs(t, h, callerId.Hex()))
	if target, ok := got[targetId.Hex()]; !ok {
		t.Fatal("the target should be on the roll")
	} else if target.DisplayName() == "" {
		t.Error("a mentionable with no display name is useless to the picker")
	}
}

// A guest gets only the people already visible to them here — otherwise the
// picker would enumerate the whole membership to someone invited to one event.
func TestMentionables_GuestSeesOnlyPeopleOnTheEvent(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "mention-guest-owner@example.test")
	commenterId := insertTestUser(t, models.RoleMember, "mention-guest-commenter@example.test")
	rsvperId := insertTestUser(t, models.RoleMember, "mention-guest-rsvper@example.test")
	strangerId := insertTestUser(t, models.RoleMember, "mention-guest-stranger@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "mention-guest@example.test")
	eventId := insertTestEvent(t, ownerId)

	h := mentionsTestRouter()
	postComment(t, h, eventId, loginAs(t, h, commenterId.Hex()), `{"text":"looking forward to it"}`)
	rsvpAs(t, eventId, rsvperId)

	got := getMentionablesFor(t, h, eventId, loginAs(t, h, guestId.Hex()))

	if _, ok := got[commenterId.Hex()]; !ok {
		t.Error("a guest should be able to mention someone who commented here")
	}
	if _, ok := got[rsvperId.Hex()]; !ok {
		t.Error("a guest should be able to mention someone who RSVP'd here")
	}
	if _, ok := got[strangerId.Hex()]; ok {
		t.Error("a guest must not be offered someone with no presence on this event")
	}
	if _, ok := got[ownerId.Hex()]; ok {
		t.Error("the owner is not rendered by name on the page, so is not mentionable by a guest")
	}
}

// The author of a members-only thread is hidden from a guest on the event page;
// the picker must not hand back the name the thread filter withheld.
func TestMentionables_GuestNeverSeesMembersOnlyThreadAuthors(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "mention-private-owner@example.test")
	privateAuthorId := insertTestUser(t, models.RoleMember, "mention-private-author@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "mention-private-guest@example.test")
	eventId := insertTestEvent(t, ownerId)

	h := mentionsTestRouter()
	h.POST("/events/:eventId/comments/:commentId/thread", middleware.AuthRequired(), tagCommentAsThread)

	// A members-only thread, written by someone otherwise absent from the event.
	authorCookie := loginAs(t, h, privateAuthorId.Hex())
	root := postAndReadComment(t, h, eventId, authorCookie, `{"text":"Budget"}`)
	w := do(h, http.MethodPost,
		"/events/"+eventId.Hex()+"/comments/"+root.Id.Hex()+"/thread",
		`{"membersOnly":true}`, authorCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("tag thread: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	got := getMentionablesFor(t, h, eventId, loginAs(t, h, guestId.Hex()))
	if _, ok := got[privateAuthorId.Hex()]; ok {
		t.Error("the author of a members-only thread leaked into a guest's picker")
	}

	// And a member, who can read that thread, does get them.
	memberGot := getMentionablesFor(t, h, eventId, loginAs(t, h, ownerId.Hex()))
	if _, ok := memberGot[privateAuthorId.Hex()]; !ok {
		t.Error("a member should still be offered the thread's author")
	}
}

// --- write-path validation --------------------------------------------------

func TestComments_MemberMentionIsStored(t *testing.T) {
	requireDB(t)

	authorId := insertTestUser(t, models.RoleMember, "mention-write-author@example.test")
	targetId := insertTestUser(t, models.RoleMember, "mention-write-target@example.test")
	eventId := insertTestEvent(t, authorId)

	h := mentionsTestRouter()
	body := `{"text":"morning @[Target User](` + targetId.Hex() + `), are we on?"}`
	stored := postAndReadComment(t, h, eventId, loginAs(t, h, authorId.Hex()), body)

	if got := mentionIds(stored); len(got) != 1 || got[0] != targetId.Hex() {
		t.Fatalf("stored mentions = %v, want [%s]", got, targetId.Hex())
	}
}

// An id that resolves to no account is not a mention — but the text is the
// author's, so it is left exactly as typed.
func TestComments_MentionOfUnknownAccountIsDropped(t *testing.T) {
	requireDB(t)

	authorId := insertTestUser(t, models.RoleMember, "mention-unknown-author@example.test")
	eventId := insertTestEvent(t, authorId)
	ghost := primitive.NewObjectID()

	h := mentionsTestRouter()
	text := "who is @[Nobody](" + ghost.Hex() + ")?"
	stored := postAndReadComment(t, h, eventId, loginAs(t, h, authorId.Hex()), `{"text":"`+text+`"}`)

	if len(stored.Mentions) != 0 {
		t.Errorf("stored mentions = %v, want none", mentionIds(stored))
	}
	if stored.Text != text {
		t.Errorf("text = %q, want it left untouched (%q)", stored.Text, text)
	}
}

// The rule that matters: a guest who hand-crafts a token for someone they were
// never shown gets it stripped from Mentions — so F8 will never mail that
// person — while the comment still posts with its text intact.
func TestComments_GuestMentionOutsideTheEventIsStripped(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "mention-strip-owner@example.test")
	visibleId := insertTestUser(t, models.RoleMember, "mention-strip-visible@example.test")
	strangerId := insertTestUser(t, models.RoleMember, "mention-strip-stranger@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "mention-strip-guest@example.test")
	eventId := insertTestEvent(t, ownerId)

	h := mentionsTestRouter()
	postComment(t, h, eventId, loginAs(t, h, visibleId.Hex()), `{"text":"I'll bring the maps"}`)

	text := "thanks @[Visible](" + visibleId.Hex() + ") and @[Stranger](" + strangerId.Hex() + ")"
	stored := postAndReadComment(t, h, eventId, loginAs(t, h, guestId.Hex()), `{"text":"`+text+`"}`)

	got := mentionIds(stored)
	if len(got) != 1 || got[0] != visibleId.Hex() {
		t.Fatalf("stored mentions = %v, want only the visible member [%s]", got, visibleId.Hex())
	}
	if stored.Text != text {
		t.Errorf("text = %q, want it left untouched (%q)", stored.Text, text)
	}
}

// The same token from a member is kept — the strip is about the author's role,
// not about the token.
func TestComments_MemberMentionOutsideTheEventIsKept(t *testing.T) {
	requireDB(t)

	authorId := insertTestUser(t, models.RoleMember, "mention-keep-author@example.test")
	strangerId := insertTestUser(t, models.RoleMember, "mention-keep-stranger@example.test")
	eventId := insertTestEvent(t, authorId)

	h := mentionsTestRouter()
	body := `{"text":"ask @[Stranger](` + strangerId.Hex() + `)"}`
	stored := postAndReadComment(t, h, eventId, loginAs(t, h, authorId.Hex()), body)

	if got := mentionIds(stored); len(got) != 1 || got[0] != strangerId.Hex() {
		t.Fatalf("stored mentions = %v, want [%s]", got, strangerId.Hex())
	}
}

// An edit re-parses rather than merging: adding a mention records it, and
// removing every mention clears the field rather than leaving stale ids for
// F8's edit-diff to mistake for already-notified.
func TestComments_EditReplacesStoredMentions(t *testing.T) {
	requireDB(t)

	authorId := insertTestUser(t, models.RoleMember, "mention-edit-author@example.test")
	firstId := insertTestUser(t, models.RoleMember, "mention-edit-first@example.test")
	secondId := insertTestUser(t, models.RoleMember, "mention-edit-second@example.test")
	eventId := insertTestEvent(t, authorId)

	h := mentionsTestRouter()
	cookie := loginAs(t, h, authorId.Hex())
	stored := postAndReadComment(t, h, eventId, cookie, `{"text":"hi @[First](`+firstId.Hex()+`)"}`)

	edit := func(text string) *models.Comment {
		t.Helper()
		payload, _ := json.Marshal(map[string]string{"text": text})
		w := do(h, http.MethodPut, "/events/"+eventId.Hex()+"/comments/"+stored.Id.Hex(), string(payload), cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("edit: got %d, want 200 (body: %s)", w.Code, w.Body.String())
		}
		updated, err := db.GetCommentById(stored.Id.Hex())
		if err != nil || updated == nil {
			t.Fatalf("re-read edited comment: %v", err)
		}
		return updated
	}

	swapped := edit("hi @[Second](" + secondId.Hex() + ")")
	if got := mentionIds(swapped); len(got) != 1 || got[0] != secondId.Hex() {
		t.Fatalf("after swap: mentions = %v, want [%s]", got, secondId.Hex())
	}

	cleared := edit("never mind")
	if len(cleared.Mentions) != 0 {
		t.Fatalf("after clearing: mentions = %v, want none", mentionIds(cleared))
	}

	// $unset, not an empty array left behind.
	var raw bson.M
	if err := db.CommentsCollection.FindOne(context.Background(), bson.M{"_id": stored.Id}).Decode(&raw); err != nil {
		t.Fatalf("raw re-read: %v", err)
	}
	if _, present := raw["mentions"]; present {
		t.Error("the mentions field should be unset once the last mention is edited out")
	}
}
