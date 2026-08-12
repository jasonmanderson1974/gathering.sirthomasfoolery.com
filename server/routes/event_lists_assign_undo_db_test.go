package routes

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

// assignResponse is what the assign route now returns: the count it wrote, plus
// a token when the write cascaded.
type assignResponse struct {
	Affected  int    `json:"affected"`
	UndoToken string `json:"undoToken"`
}

func assignAndRead(t *testing.T, h *gin.Engine, eventId, listId, itemId primitive.ObjectID, body string, cookie *http.Cookie) assignResponse {
	t.Helper()
	w := do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/lists/"+listId.Hex()+"/items/"+itemId.Hex()+"/assignee",
		body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("assign: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var res assignResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode assign response: %v (body: %s)", err, w.Body.String())
	}
	return res
}

func undo(h *gin.Engine, eventId primitive.ObjectID, token string, cookie *http.Cookie) int {
	return do(h, http.MethodPost,
		"/events/"+eventId.Hex()+"/lists/undo-assign",
		`{"undoToken":"`+token+`"}`, cookie).Code
}

// holderOf returns an item's assignee id from a re-read of the stored list.
func holderOf(t *testing.T, eventId, itemId primitive.ObjectID) *primitive.ObjectID {
	t.Helper()
	for _, list := range readEventLists(t, eventId) {
		for _, item := range list.Items {
			if item.Id == itemId {
				return item.AssigneeId
			}
		}
	}
	t.Fatalf("item %s not found", itemId.Hex())
	return nil
}

// The headline case, and the one that rules out letting the client hold the
// snapshot: undo restores a member who is NO LONGER in the assignable pool.
//
// Bart is assignable only because he holds an entry — no availability response,
// no RSVP, not the owner. The cascade takes that entry, which takes his pool
// membership with it, so a client-supplied restore naming him would be refused
// by the assign route's own validation. Restoring the server's own record has
// nothing to validate.
func TestUndo_RestoresAHolderWhoLeftTheAssignablePool(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "undo-pool-owner@example.test")
	bartId := insertTestUser(t, models.RoleMember, "undo-pool-bart@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())

	list := createListFor(t, h, eventId, cookie, `{"name":"Gear","kind":"checklist"}`)
	parent := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Sleeping"}`)
	child := addItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"1 Tent","parentId":"`+parent.Id.Hex()+`"}`)

	// Bart is in the pool right now only via a going RSVP; give him the entry,
	// then take the RSVP away so holding it is the ONLY thing keeping him in.
	setRsvp(t, eventId, bartId, models.RsvpGoing)
	assignAndRead(t, h, eventId, list.Id, child.Id, `{"assigneeId":"`+bartId.Hex()+`"}`, cookie)
	setRsvp(t, eventId, bartId, models.RsvpNo)

	// The cascade takes the entry — and with it his last claim on the pool.
	res := assignAndRead(t, h, eventId, list.Id, parent.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie)
	if res.UndoToken == "" {
		t.Fatal("a cascading assign returned no undo token")
	}
	if res.Affected != 2 {
		t.Errorf("affected = %d, want 2", res.Affected)
	}

	// Proof the naive design would fail here: assigning Bart directly is now
	// refused, because he is no longer in the pool.
	if code := assignItem(h, eventId, list.Id, child.Id, `{"assigneeId":"`+bartId.Hex()+`"}`, cookie); code != http.StatusBadRequest {
		t.Fatalf("expected Bart to have left the pool, but a direct assign got %d", code)
	}

	// Undo restores him anyway.
	if code := undo(h, eventId, res.UndoToken, cookie); code != http.StatusOK {
		t.Fatalf("undo: got %d, want 200", code)
	}
	got := holderOf(t, eventId, child.Id)
	if got == nil || *got != bartId {
		t.Errorf("undo did not restore the prior holder: %v", got)
	}
	if holderOf(t, eventId, parent.Id) != nil {
		t.Error("undo did not return the parent to unassigned")
	}
}

// A branch's prior state is usually mixed — some unassigned, some held by
// different people — and all of it has to come back in one call.
func TestUndo_RestoresAMixedBranch(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "undo-mixed-owner@example.test")
	aId := insertTestUser(t, models.RoleMember, "undo-mixed-a@example.test")
	bId := insertTestUser(t, models.RoleMember, "undo-mixed-b@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	setRsvp(t, eventId, aId, models.RsvpGoing)
	setRsvp(t, eventId, bId, models.RsvpMaybe)

	list := createListFor(t, h, eventId, cookie, `{"name":"Gear","kind":"checklist"}`)
	parent := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Sleeping"}`)
	toA := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"1 Tent","parentId":"`+parent.Id.Hex()+`"}`)
	toB := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Bags","parentId":"`+parent.Id.Hex()+`"}`)
	blank := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"2 Cots","parentId":"`+parent.Id.Hex()+`"}`)

	assignAndRead(t, h, eventId, list.Id, toA.Id, `{"assigneeId":"`+aId.Hex()+`"}`, cookie)
	assignAndRead(t, h, eventId, list.Id, toB.Id, `{"assigneeId":"`+bId.Hex()+`"}`, cookie)

	res := assignAndRead(t, h, eventId, list.Id, parent.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie)
	if code := undo(h, eventId, res.UndoToken, cookie); code != http.StatusOK {
		t.Fatalf("undo: got %d, want 200", code)
	}

	for _, tc := range []struct {
		name string
		id   primitive.ObjectID
		want *primitive.ObjectID
	}{
		{"the entry A held", toA.Id, &aId},
		{"the entry B held", toB.Id, &bId},
		{"the entry nobody held", blank.Id, nil},
		{"the parent", parent.Id, nil},
	} {
		got := holderOf(t, eventId, tc.id)
		switch {
		case tc.want == nil && got != nil:
			t.Errorf("%s: assignee = %v, want nobody", tc.name, got)
		case tc.want != nil && (got == nil || *got != *tc.want):
			t.Errorf("%s: assignee = %v, want %v", tc.name, got, *tc.want)
		}
	}

	// The names came back with the ids, or the byline would read wrong.
	for _, list := range readEventLists(t, eventId) {
		for _, item := range list.Items {
			if item.AssigneeId == nil && item.AssigneeName != "" {
				t.Errorf("item %s has a name with no holder: %q", item.Id.Hex(), item.AssigneeName)
			}
			if item.AssigneeId != nil && item.AssigneeName == "" {
				t.Errorf("item %s has a holder with no name", item.Id.Hex())
			}
		}
	}
}

// Undo works in both directions: a cascading CLEAR is just as destructive, and
// is restored the same way.
func TestUndo_RestoresACascadingClear(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "undo-clear-owner@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())

	list := createListFor(t, h, eventId, cookie, `{"name":"Gear","kind":"checklist"}`)
	parent := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Sleeping"}`)
	child := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"1 Tent","parentId":"`+parent.Id.Hex()+`"}`)

	assignAndRead(t, h, eventId, list.Id, parent.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie)

	res := assignAndRead(t, h, eventId, list.Id, parent.Id, `{"assigneeId":""}`, cookie)
	if res.UndoToken == "" {
		t.Fatal("a cascading clear returned no undo token")
	}
	if holderOf(t, eventId, child.Id) != nil {
		t.Fatal("the clear did not cascade, so this test proves nothing")
	}

	if code := undo(h, eventId, res.UndoToken, cookie); code != http.StatusOK {
		t.Fatalf("undo: got %d, want 200", code)
	}
	for _, id := range []primitive.ObjectID{parent.Id, child.Id} {
		got := holderOf(t, eventId, id)
		if got == nil || *got != ownerId {
			t.Errorf("undo of a clear did not restore %s: %v", id.Hex(), got)
		}
	}
}

// A leaf assign changes one visible row, so it is not remembered and offers no
// Undo — the one condition that decides when the button appears.
func TestUndo_LeafAssignOffersNothing(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "undo-leaf-owner@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())

	list := createListFor(t, h, eventId, cookie, `{"name":"Gear","kind":"checklist"}`)
	leaf := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"1 Tent"}`)

	res := assignAndRead(t, h, eventId, list.Id, leaf.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie)
	if res.UndoToken != "" {
		t.Errorf("a leaf assign offered an undo token: %q", res.UndoToken)
	}
	if res.Affected != 1 {
		t.Errorf("affected = %d, want 1", res.Affected)
	}
	// And nothing was remembered, so an invented token finds nothing.
	if code := undo(h, eventId, primitive.NewObjectID().Hex(), cookie); code != http.StatusNotFound {
		t.Errorf("undo after a leaf assign: got %d, want 404", code)
	}
}

func TestUndo_Refusals(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "undo-refuse-owner@example.test")
	otherId := insertTestUser(t, models.RoleMember, "undo-refuse-other@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "undo-refuse-guest@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	otherCookie := loginAs(t, h, otherId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())

	list := createListFor(t, h, eventId, cookie, `{"name":"Gear","kind":"checklist"}`)
	parent := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Sleeping"}`)
	addItemFor(t, h, eventId, list.Id, cookie, `{"text":"1 Tent","parentId":"`+parent.Id.Hex()+`"}`)

	res := assignAndRead(t, h, eventId, list.Id, parent.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie)

	// A guest cannot undo, for the same reason they cannot assign.
	if code := undo(h, eventId, res.UndoToken, guestCookie); code != http.StatusForbidden {
		t.Errorf("guest undo: got %d, want 403", code)
	}
	// Nor can another member undo somebody else's action.
	if code := undo(h, eventId, res.UndoToken, otherCookie); code != http.StatusNotFound {
		t.Errorf("another member's undo: got %d, want 404", code)
	}
	// A wrong token is refused.
	if code := undo(h, eventId, primitive.NewObjectID().Hex(), cookie); code != http.StatusNotFound {
		t.Errorf("wrong token: got %d, want 404", code)
	}
	// A missing token is a bad request, not a 404 — it never named an action.
	if do(h, http.MethodPost, "/events/"+eventId.Hex()+"/lists/undo-assign", `{}`, cookie).Code != http.StatusBadRequest {
		t.Error("an absent token should be a 400")
	}

	// The real one still works after all of that, then not twice.
	if code := undo(h, eventId, res.UndoToken, cookie); code != http.StatusOK {
		t.Fatalf("undo: got %d, want 200", code)
	}
	if code := undo(h, eventId, res.UndoToken, cookie); code != http.StatusNotFound {
		t.Errorf("undo applied twice: got %d, want 404", code)
	}
}

// A second cascade supersedes the first, so the older button cannot restore the
// newer action's snapshot.
func TestUndo_SecondActionSupersedesTheFirst(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "undo-super-owner@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())

	list := createListFor(t, h, eventId, cookie, `{"name":"Gear","kind":"checklist"}`)
	first := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Sleeping"}`)
	addItemFor(t, h, eventId, list.Id, cookie, `{"text":"1 Tent","parentId":"`+first.Id.Hex()+`"}`)
	second := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Cooking"}`)
	addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Stove","parentId":"`+second.Id.Hex()+`"}`)

	firstRes := assignAndRead(t, h, eventId, list.Id, first.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie)
	secondRes := assignAndRead(t, h, eventId, list.Id, second.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie)

	if code := undo(h, eventId, firstRes.UndoToken, cookie); code != http.StatusNotFound {
		t.Errorf("a superseded token was honoured: got %d, want 404", code)
	}
	if code := undo(h, eventId, secondRes.UndoToken, cookie); code != http.StatusOK {
		t.Errorf("the current token was refused: got %d, want 200", code)
	}
	// The first branch stays assigned: undoing the second must not touch it.
	if holderOf(t, eventId, first.Id) == nil {
		t.Error("undoing the second action reverted the first")
	}
}

// An expired record is refused rather than honoured late.
func TestUndo_ExpiredRecordIsRefused(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "undo-expiry-owner@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())

	list := createListFor(t, h, eventId, cookie, `{"name":"Gear","kind":"checklist"}`)
	parent := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Sleeping"}`)
	addItemFor(t, h, eventId, list.Id, cookie, `{"text":"1 Tent","parentId":"`+parent.Id.Hex()+`"}`)

	res := assignAndRead(t, h, eventId, list.Id, parent.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie)

	// Age it past the window rather than sleeping through it.
	assignUndos.mu.Lock()
	record := assignUndos.records[ownerId]
	record.At = time.Now().Add(-undoWindow - time.Second)
	assignUndos.records[ownerId] = record
	assignUndos.mu.Unlock()

	if code := undo(h, eventId, res.UndoToken, cookie); code != http.StatusNotFound {
		t.Errorf("an expired undo was honoured: got %d, want 404", code)
	}
}
