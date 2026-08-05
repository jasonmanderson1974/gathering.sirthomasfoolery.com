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

// DB-backed tests for the private lists and note (F19/F20).
//
// The privacy tests are the point of this file. Everything else here — nesting,
// ordering, cascade delete — is shared machinery already covered by
// event_lists_db_test.go; what is genuinely new is that a second person can
// never see any of it, and those are the tests that must never be deleted.

// personalTestRouter wires the private routes behind the same AuthRequired
// middleware production uses, plus a test-login.
func personalTestRouter() *gin.Engine {
	r := newTestRouter()
	registerTestLogin(r)
	initPersonalRoutes(r.Group("/events", middleware.AuthRequired()))
	return r
}

// readPersonalLists re-reads what actually landed in Mongo, rather than
// trusting what a handler said it did.
func readPersonalLists(t *testing.T, userId, eventId primitive.ObjectID) []models.EventList {
	t.Helper()
	lists, err := db.GetPersonalLists(userId, eventId)
	if err != nil {
		t.Fatalf("re-read personal lists: %v", err)
	}
	return lists
}

// cleanupPersonal removes both private collections for an event once a test is
// done. insertTestEvent's own cleanup doesn't know about them.
func cleanupPersonal(t *testing.T, eventId primitive.ObjectID) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		db.PersonalListsCollection.DeleteMany(ctx, bson.M{"eventId": eventId})
		db.PersonalNotesCollection.DeleteMany(ctx, bson.M{"eventId": eventId})
	})
}

// eventId is a STRING, not an ObjectID, so a test can reach the same gathering
// by its short id — which is exactly what
// TestPersonal_ShortIdAndLongIdShareOneDocument needs to do.
func createPersonalListFor(t *testing.T, h *gin.Engine, eventId string, cookie *http.Cookie, body string) models.EventList {
	t.Helper()
	w := do(h, http.MethodPost, "/events/"+eventId+"/my-lists", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("create personal list: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var list models.EventList
	json.Unmarshal(w.Body.Bytes(), &list)
	return list
}

func addPersonalItemFor(t *testing.T, h *gin.Engine, eventId, listId primitive.ObjectID, cookie *http.Cookie, body string) models.EventListItem {
	t.Helper()
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/my-lists/"+listId.Hex()+"/items", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("add personal item: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var item models.EventListItem
	json.Unmarshal(w.Body.Bytes(), &item)
	return item
}

func TestPersonal_RequiresSignIn(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "personal-signin@example.test")
	eventId := insertTestEvent(t, ownerId)
	cleanupPersonal(t, eventId)
	h := personalTestRouter()

	for _, path := range []string{"/my-lists", "/my-notes"} {
		w := do(h, http.MethodGet, "/events/"+eventId.Hex()+path, "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s: got %d, want 401 (body: %s)", path, w.Code, w.Body.String())
		}
	}
}

// The headline flow, and the reason the feature exists: a guest keeps their own
// list on someone else's gathering, and nobody else can see a trace of it.
func TestPersonal_ListsAreInvisibleToAnotherUser(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "personal-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "personal-guest@example.test")
	eventId := insertTestEvent(t, ownerId)
	cleanupPersonal(t, eventId)

	h := personalTestRouter()
	guest := loginAs(t, h, guestId.Hex())
	owner := loginAs(t, h, ownerId.Hex())

	list := createPersonalListFor(t, h, eventId.Hex(), guest, `{"name":"Things to bring","kind":"checklist"}`)
	addPersonalItemFor(t, h, eventId, list.Id, guest, `{"text":"Islay bottle"}`)

	// The guest sees their own.
	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/my-lists", "", guest)
	if w.Code != http.StatusOK {
		t.Fatalf("guest GET: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Islay bottle") {
		t.Fatalf("guest should see their own entry, got: %s", w.Body.String())
	}

	// The event's own planner — the most privileged person on this gathering —
	// sees nothing at all. Not a redacted version: nothing.
	w = do(h, http.MethodGet, "/events/"+eventId.Hex()+"/my-lists", "", owner)
	if w.Code != http.StatusOK {
		t.Fatalf("owner GET: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if got := strings.TrimSpace(w.Body.String()); got != "[]" {
		t.Fatalf("the planner must not see a guest's private lists, got: %s", got)
	}

	// And holding the id is not enough. 404, not 403: the filter is
	// {userId: owner, eventId}, so the list genuinely isn't there — which is
	// both the correct answer and the one that leaks least (403 would confirm
	// the id names something real).
	w = do(h, http.MethodPatch,
		"/events/"+eventId.Hex()+"/my-lists/"+list.Id.Hex(), `{"name":"Renamed"}`, owner)
	if w.Code != http.StatusNotFound {
		t.Fatalf("owner renaming a guest's private list: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}

	// The rename must not have landed anywhere.
	lists := readPersonalLists(t, guestId, eventId)
	if len(lists) != 1 || lists[0].Name != "Things to bring" {
		t.Fatalf("the guest's list was modified by someone else: %+v", lists)
	}
}

// Private data must not ride out on the event document — the whole reason these
// live in their own collections.
func TestPersonal_NotReachableThroughTheEventDocument(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "personal-leak@example.test")
	eventId := insertTestEvent(t, ownerId)
	cleanupPersonal(t, eventId)

	h := personalTestRouter()
	owner := loginAs(t, h, ownerId.Hex())

	list := createPersonalListFor(t, h, eventId.Hex(), owner, `{"name":"Secret errands"}`)
	addPersonalItemFor(t, h, eventId, list.Id, owner, `{"text":"collect the ring"}`)
	if w := do(h, http.MethodPut, "/events/"+eventId.Hex()+"/my-notes",
		`{"text":"what I owe Bart"}`, owner); w.Code != http.StatusOK {
		t.Fatalf("save note: got %d (body: %s)", w.Code, w.Body.String())
	}

	// Asserted against the raw stored event, not a handler's response: this is
	// meant to catch a FUTURE field added anywhere in the event tree, not just
	// today's serialization.
	var raw bson.M
	if err := db.EventsCollection.FindOne(context.Background(), bson.M{"_id": eventId}).Decode(&raw); err != nil {
		t.Fatalf("re-read event: %v", err)
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, secret := range []string{"Secret errands", "collect the ring", "what I owe Bart"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("private text %q found on the event document: %s", secret, encoded)
		}
	}
}

// The likeliest bug in the whole feature: personal documents are keyed by the
// canonical event._id, but a route param may be a SHORT id. Arriving by one URL
// and then the other must find ONE document, not two.
func TestPersonal_ShortIdAndLongIdShareOneDocument(t *testing.T) {
	requireDB(t)

	userId := insertTestUser(t, models.RoleMember, "personal-shortid@example.test")
	eventId := insertTestEvent(t, userId)
	cleanupPersonal(t, eventId)

	// At most 10 characters, or GetEventByEitherId treats it as an ObjectID hex
	// and never reaches the short-id lookup this test exists to exercise.
	shortId := "p" + eventId.Hex()[:8]
	if _, err := db.EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$set": bson.M{"shortId": shortId}}); err != nil {
		t.Fatalf("set shortId: %v", err)
	}

	h := personalTestRouter()
	cookie := loginAs(t, h, userId.Hex())

	// Created via the short id...
	createPersonalListFor(t, h, shortId, cookie, `{"name":"Packing"}`)

	// ...read back via the canonical id.
	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/my-lists", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("GET by long id: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var lists []models.EventList
	json.Unmarshal(w.Body.Bytes(), &lists)
	if len(lists) != 1 || lists[0].Name != "Packing" {
		t.Fatalf("short id and long id must reach the same document, got: %s", w.Body.String())
	}

	count, err := db.PersonalListsCollection.CountDocuments(context.Background(),
		bson.M{"userId": userId, "eventId": eventId})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("got %d personal documents, want exactly 1", count)
	}
}

// Nesting, the depth cap, and the cascade — the shared machinery, exercised once
// on this path to prove the personal routes really do reach it.
func TestPersonal_NestingDepthAndCascade(t *testing.T) {
	requireDB(t)

	userId := insertTestUser(t, models.RoleMember, "personal-nesting@example.test")
	eventId := insertTestEvent(t, userId)
	cleanupPersonal(t, eventId)

	h := personalTestRouter()
	cookie := loginAs(t, h, userId.Hex())
	base := "/events/" + eventId.Hex() + "/my-lists"

	list := createPersonalListFor(t, h, eventId.Hex(), cookie, `{"name":"Plans"}`)
	root := addPersonalItemFor(t, h, eventId, list.Id, cookie, `{"text":"root"}`)
	child := addPersonalItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"child","parentId":"`+root.Id.Hex()+`"}`)
	grandchild := addPersonalItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"grandchild","parentId":"`+child.Id.Hex()+`"}`)

	// A fourth level is refused — the same exact check the shared lists make.
	w := do(h, http.MethodPost, base+"/"+list.Id.Hex()+"/items",
		`{"text":"too deep","parentId":"`+grandchild.Id.Hex()+`"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("4th level: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), errListTooDeep) {
		t.Fatalf("4th level should report %q, got: %s", errListTooDeep, w.Body.String())
	}

	// Deleting the root takes its whole subtree with it, in one update.
	w = do(h, http.MethodDelete, base+"/"+list.Id.Hex()+"/items/"+root.Id.Hex(), "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("cascade delete: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	lists := readPersonalLists(t, userId, eventId)
	if len(lists) != 1 {
		t.Fatalf("got %d lists, want 1", len(lists))
	}
	if len(lists[0].Items) != 0 {
		t.Fatalf("the subtree should be gone, %d items remain: %+v", len(lists[0].Items), lists[0].Items)
	}
}

// Ticking a private checklist records the state and the time, and NOT a name —
// checkStateLabel on the frontend keys off checkedByName precisely so a private
// item renders no "Checked by …" line.
func TestPersonal_CheckRecordsNoAttribution(t *testing.T) {
	requireDB(t)

	userId := insertTestUser(t, models.RoleMember, "personal-check@example.test")
	eventId := insertTestEvent(t, userId)
	cleanupPersonal(t, eventId)

	h := personalTestRouter()
	cookie := loginAs(t, h, userId.Hex())

	list := createPersonalListFor(t, h, eventId.Hex(), cookie, `{"name":"Errands","kind":"checklist"}`)
	item := addPersonalItemFor(t, h, eventId, list.Id, cookie, `{"text":"post the letter"}`)

	path := "/events/" + eventId.Hex() + "/my-lists/" + list.Id.Hex() + "/items/" + item.Id.Hex() + "/checked"
	if w := do(h, http.MethodPut, path, `{"checked":true}`, cookie); w.Code != http.StatusOK {
		t.Fatalf("tick: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	stored := readPersonalLists(t, userId, eventId)[0].Items[0]
	if !stored.Checked {
		t.Fatal("the item should be checked")
	}
	if stored.CheckedAt == 0 {
		t.Fatal("checkedAt should be stamped")
	}
	if stored.CheckedByName != "" {
		t.Fatalf("a private check must record no name, got %q", stored.CheckedByName)
	}
	if stored.CheckedBy != nil {
		t.Fatalf("a private check must record no checkedBy, got %v", *stored.CheckedBy)
	}

	// Re-ticking an already-ticked box modifies nothing and must still succeed.
	if w := do(h, http.MethodPut, path, `{"checked":true}`, cookie); w.Code != http.StatusOK {
		t.Fatalf("re-tick: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	// Unticking is as much a change of state as ticking.
	if w := do(h, http.MethodPut, path, `{"checked":false}`, cookie); w.Code != http.StatusOK {
		t.Fatalf("untick: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if readPersonalLists(t, userId, eventId)[0].Items[0].Checked {
		t.Fatal("the item should be unchecked")
	}
}

// A move may only ever land on one of the caller's OWN lists.
func TestPersonal_MoveCannotTargetSomeoneElsesList(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "personal-move-owner@example.test")
	otherId := insertTestUser(t, models.RoleMember, "personal-move-other@example.test")
	eventId := insertTestEvent(t, ownerId)
	cleanupPersonal(t, eventId)

	h := personalTestRouter()
	mine := loginAs(t, h, ownerId.Hex())
	theirs := loginAs(t, h, otherId.Hex())

	myList := createPersonalListFor(t, h, eventId.Hex(), mine, `{"name":"Mine"}`)
	myItem := addPersonalItemFor(t, h, eventId, myList.Id, mine, `{"text":"an entry"}`)
	theirList := createPersonalListFor(t, h, eventId.Hex(), theirs, `{"name":"Theirs"}`)

	w := do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/my-lists/"+myList.Id.Hex()+"/items/"+myItem.Id.Hex()+"/move",
		`{"targetListId":"`+theirList.Id.Hex()+`","order":1024}`, mine)
	if w.Code != http.StatusNotFound {
		t.Fatalf("move onto another user's list: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}

	// Neither side changed.
	if items := readPersonalLists(t, ownerId, eventId)[0].Items; len(items) != 1 {
		t.Fatalf("the entry should still be on its own list, got %d items", len(items))
	}
	if items := readPersonalLists(t, otherId, eventId)[0].Items; len(items) != 0 {
		t.Fatalf("nothing should have landed on the other user's list, got %d items", len(items))
	}
}

func TestPersonalNote_RoundTripAndIsolation(t *testing.T) {
	requireDB(t)

	aId := insertTestUser(t, models.RoleMember, "note-a@example.test")
	bId := insertTestUser(t, models.RoleGuest, "note-b@example.test")
	eventId := insertTestEvent(t, aId)
	cleanupPersonal(t, eventId)

	h := personalTestRouter()
	a := loginAs(t, h, aId.Hex())
	b := loginAs(t, h, bId.Hex())
	path := "/events/" + eventId.Hex() + "/my-notes"

	// Never written: 200 with an empty note and a null timestamp, not a 404.
	w := do(h, http.MethodGet, path, "", a)
	if w.Code != http.StatusOK {
		t.Fatalf("unwritten note: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var fresh personalNoteResponse
	json.Unmarshal(w.Body.Bytes(), &fresh)
	if fresh.Text != "" || fresh.UpdatedAt != nil {
		t.Fatalf("an unwritten note should be empty with no timestamp, got %+v", fresh)
	}

	// Written, and echoed back with the server's own timestamp.
	w = do(h, http.MethodPut, path, `{"text":"## Whisky Night\n- ask **Bart**"}`, a)
	if w.Code != http.StatusOK {
		t.Fatalf("save: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var saved personalNoteResponse
	json.Unmarshal(w.Body.Bytes(), &saved)
	if saved.UpdatedAt == nil || *saved.UpdatedAt == 0 {
		t.Fatalf("a saved note must carry a timestamp, got %+v", saved)
	}
	if !strings.Contains(saved.Text, "ask **Bart**") {
		t.Fatalf("markdown must be stored verbatim, got %q", saved.Text)
	}

	// B sees their own empty note, not A's.
	w = do(h, http.MethodGet, path, "", b)
	var others personalNoteResponse
	json.Unmarshal(w.Body.Bytes(), &others)
	if others.Text != "" {
		t.Fatalf("another user must not see this note, got %q", others.Text)
	}

	// B writing does not disturb A.
	if w := do(h, http.MethodPut, path, `{"text":"mine"}`, b); w.Code != http.StatusOK {
		t.Fatalf("B save: got %d (body: %s)", w.Code, w.Body.String())
	}
	w = do(h, http.MethodGet, path, "", a)
	var reread personalNoteResponse
	json.Unmarshal(w.Body.Bytes(), &reread)
	if !strings.Contains(reread.Text, "Bart") {
		t.Fatalf("A's note was disturbed by B's write, got %q", reread.Text)
	}

	// Clearing is a PUT of "" — there is no DELETE, and an empty string must not
	// fail the binding.
	if w := do(h, http.MethodPut, path, `{"text":""}`, a); w.Code != http.StatusOK {
		t.Fatalf("clear: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	w = do(h, http.MethodGet, path, "", a)
	var cleared personalNoteResponse
	json.Unmarshal(w.Body.Bytes(), &cleared)
	if cleared.Text != "" {
		t.Fatalf("the note should be cleared, got %q", cleared.Text)
	}
	// Cleared, not absent: it was written once, so it still has a timestamp.
	if cleared.UpdatedAt == nil {
		t.Fatal("a cleared note keeps its timestamp — it was saved, it just says nothing")
	}

	// A payload with no text field at all is still refused.
	if w := do(h, http.MethodPut, path, `{}`, a); w.Code != http.StatusBadRequest {
		t.Fatalf("missing text field: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
}

// Deleting an event that nobody responded to takes everyone's private data with
// it.
func TestPersonal_HardDeletingAnEventClearsPrivateData(t *testing.T) {
	requireDB(t)

	userId := insertTestUser(t, models.RoleMember, "personal-cleanup@example.test")
	eventId := insertTestEvent(t, userId)
	cleanupPersonal(t, eventId)

	h := personalTestRouter()
	cookie := loginAs(t, h, userId.Hex())
	createPersonalListFor(t, h, eventId.Hex(), cookie, `{"name":"Doomed"}`)
	if w := do(h, http.MethodPut, "/events/"+eventId.Hex()+"/my-notes",
		`{"text":"also doomed"}`, cookie); w.Code != http.StatusOK {
		t.Fatalf("save note: got %d (body: %s)", w.Code, w.Body.String())
	}

	if err := db.DeletePersonalDataForEvent(eventId); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	ctx := context.Background()
	listCount, err := db.PersonalListsCollection.CountDocuments(ctx, bson.M{"eventId": eventId})
	if err != nil {
		t.Fatalf("count lists: %v", err)
	}
	noteCount, err := db.PersonalNotesCollection.CountDocuments(ctx, bson.M{"eventId": eventId})
	if err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if listCount != 0 || noteCount != 0 {
		t.Fatalf("private data survived the event: %d lists, %d notes", listCount, noteCount)
	}
}
