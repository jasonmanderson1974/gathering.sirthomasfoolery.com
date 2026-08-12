package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/middleware"
	"sirtom/server/models"
)

// assignTestRouter wires the shared-list routes the assignment flow touches
// alongside the private ones, because the point of the feature is that the two
// meet: an assignment made on the event appears in somebody's My Lists.
func assignTestRouter() *gin.Engine {
	r := newTestRouter()
	registerTestLogin(r)
	r.GET("/events/:eventId/lists", middleware.AuthRequired(), getEventLists)
	r.GET("/events/:eventId/lists/assignees", middleware.AuthRequired(), getListAssignees)
	r.POST("/events/:eventId/lists", middleware.AuthRequired(), createEventList)
	r.POST("/events/:eventId/lists/:listId/items", middleware.AuthRequired(), addEventListItem)
	r.PUT("/events/:eventId/lists/:listId/items/:itemId/checked", middleware.AuthRequired(), setEventListItemChecked)
	r.PUT("/events/:eventId/lists/:listId/items/:itemId/assignee", middleware.AuthRequired(), setEventListItemAssignee)
	r.PUT("/events/:eventId/lists/:listId/items/:itemId/move", middleware.AuthRequired(), moveEventListItem)
	initPersonalRoutes(r.Group("/events", middleware.AuthRequired()))
	return r
}

// setRsvp writes an RSVP straight into the event, which is what the assignable
// set is derived from.
func setRsvp(t *testing.T, eventId, userId primitive.ObjectID, status models.RsvpStatus) {
	t.Helper()
	_, err := db.EventsCollection.UpdateOne(context.Background(),
		bson.M{"_id": eventId},
		bson.M{"$set": bson.M{"rsvps." + userId.Hex(): bson.M{"status": status}}})
	if err != nil {
		t.Fatalf("set rsvp: %v", err)
	}
}

func assignItem(h *gin.Engine, eventId, listId, itemId primitive.ObjectID, body string, cookie *http.Cookie) int {
	w := do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/lists/"+listId.Hex()+"/items/"+itemId.Hex()+"/assignee",
		body, cookie)
	return w.Code
}

func readMyLists(t *testing.T, h *gin.Engine, eventId primitive.ObjectID, cookie *http.Cookie) []models.EventList {
	t.Helper()
	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/my-lists", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("get my-lists: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var lists []models.EventList
	if err := json.Unmarshal(w.Body.Bytes(), &lists); err != nil {
		t.Fatalf("decode my-lists: %v (body: %s)", err, w.Body.String())
	}
	return lists
}

// The headline flow, end to end: a member assigns an entry, it appears in the
// assignee's My Lists, and a tick made THERE lands on the shared entry.
func TestAssign_AppearsInMyListsAndTicksThrough(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-owner@example.test")
	mateId := insertTestUser(t, models.RoleMember, "assign-mate@example.test")
	eventId := insertTestEvent(t, ownerId)
	cleanupPersonal(t, eventId)
	h := assignTestRouter()

	ownerCookie := loginAs(t, h, ownerId.Hex())
	mateCookie := loginAs(t, h, mateId.Hex())
	setRsvp(t, eventId, mateId, models.RsvpGoing)
	// The owner deliberately gets NO RSVP: the reassign at the end of this test
	// is what proves they are assignable anyway, which is the rule that keeps a
	// planner able to put their own name on something.

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"Menu","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, ownerCookie, `{"text":"Bring the port"}`)

	// Before assignment, the mate's My Lists holds nothing at all.
	if lists := readMyLists(t, h, eventId, mateCookie); len(lists) != 0 {
		t.Fatalf("unassigned My Lists = %+v, want empty", lists)
	}

	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+mateId.Hex()+`"}`, ownerCookie); code != http.StatusOK {
		t.Fatalf("assign: got %d, want 200", code)
	}

	stored := readEventLists(t, eventId)
	if stored[0].Items[0].AssigneeId == nil || *stored[0].Items[0].AssigneeId != mateId {
		t.Fatalf("assignment did not persist: %+v", stored[0].Items[0])
	}
	if stored[0].Items[0].AssigneeName != "Test User" {
		t.Errorf("assigneeName = %q, want the account's display name", stored[0].Items[0].AssigneeName)
	}

	// It now shows up in the mate's private panel, first, marked derived.
	lists := readMyLists(t, h, eventId, mateCookie)
	if len(lists) != 1 {
		t.Fatalf("My Lists = %+v, want just the derived list", lists)
	}
	assigned := lists[0]
	if !assigned.Virtual || assigned.Name != assignedListName {
		t.Fatalf("first list is not the derived one: %+v", assigned)
	}
	if len(assigned.Items) != 1 || assigned.Items[0].Id != item.Id {
		t.Fatalf("derived list holds the wrong entries: %+v", assigned.Items)
	}
	if assigned.Items[0].SourceListId == nil || *assigned.Items[0].SourceListId != list.Id {
		t.Fatalf("derived item has no usable sourceListId: %+v", assigned.Items[0])
	}
	if assigned.Items[0].SourceListName != "Menu" {
		t.Errorf("sourceListName = %q, want \"Menu\"", assigned.Items[0].SourceListName)
	}

	// Ticking it from the private panel means writing to the SOURCE list — that
	// is the whole reason sourceListId is carried, and it is what makes the two
	// views agree without anything being synchronized.
	w := do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/lists/"+list.Id.Hex()+"/items/"+item.Id.Hex()+"/checked",
		`{"checked":true}`, mateCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("tick from the assignee: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	stored = readEventLists(t, eventId)
	if !stored[0].Items[0].Checked || stored[0].Items[0].CheckedBy == nil || *stored[0].Items[0].CheckedBy != mateId {
		t.Fatalf("the tick did not land on the shared entry: %+v", stored[0].Items[0])
	}
	// And the derived list reflects it immediately, because it IS that entry.
	if lists = readMyLists(t, h, eventId, mateCookie); !lists[0].Items[0].Checked {
		t.Error("the derived list does not show the tick it just made")
	}

	// Reassigning takes it out of the first member's list, with no cleanup write.
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, ownerCookie); code != http.StatusOK {
		t.Fatalf("reassign: got %d, want 200", code)
	}
	if lists = readMyLists(t, h, eventId, mateCookie); len(lists) != 0 {
		t.Fatalf("reassigned entry is still in the old assignee's My Lists: %+v", lists)
	}
}

// Unassigning clears BOTH fields. An id cleared without its name would leave the
// byline naming somebody who no longer holds the entry.
func TestAssign_UnassignClearsTheNameToo(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-un-owner@example.test")
	eventId := insertTestEvent(t, ownerId)
	cleanupPersonal(t, eventId)
	h := assignTestRouter()

	cookie := loginAs(t, h, ownerId.Hex())
	setRsvp(t, eventId, ownerId, models.RsvpGoing)

	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Bring the port"}`)

	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie); code != http.StatusOK {
		t.Fatalf("assign: got %d, want 200", code)
	}
	// An empty id is the unassign, and so is an absent one.
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":""}`, cookie); code != http.StatusOK {
		t.Fatalf("unassign: got %d, want 200", code)
	}

	stored := readEventLists(t, eventId)
	if stored[0].Items[0].AssigneeId != nil {
		t.Errorf("assigneeId = %v, want nil after unassign", stored[0].Items[0].AssigneeId)
	}
	if stored[0].Items[0].AssigneeName != "" {
		t.Errorf("assigneeName = %q, want empty after unassign", stored[0].Items[0].AssigneeName)
	}
	if lists := readMyLists(t, h, eventId, cookie); len(lists) != 0 {
		t.Fatalf("unassigned entry still derives a list: %+v", lists)
	}

	// An absent field means the same thing, and is not a binding error.
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie); code != http.StatusOK {
		t.Fatalf("re-assign: got %d, want 200", code)
	}
	if code := assignItem(h, eventId, list.Id, item.Id, `{}`, cookie); code != http.StatusOK {
		t.Fatalf("unassign with no field: got %d, want 200", code)
	}
	if stored = readEventLists(t, eventId); stored[0].Items[0].AssigneeId != nil {
		t.Error("an absent assigneeId must unassign, like an empty one")
	}
}

// The picker is a hint; the server is the rule. Everything it refuses would
// otherwise put work on somebody who cannot be asked to do it.
func TestAssign_RefusesAnyoneOutsideTheAssignableSet(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-set-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "assign-set-guest@example.test")
	notComingId := insertTestUser(t, models.RoleMember, "assign-set-no@example.test")
	absentId := insertTestUser(t, models.RoleMember, "assign-set-absent@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()

	cookie := loginAs(t, h, ownerId.Hex())
	setRsvp(t, eventId, ownerId, models.RsvpGoing)
	setRsvp(t, eventId, guestId, models.RsvpGoing)
	setRsvp(t, eventId, notComingId, models.RsvpNo)
	// absentId has no RSVP at all.

	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Bring the port"}`)

	refused := []struct {
		name string
		id   string
	}{
		{"a guest, however definitely they are coming", guestId.Hex()},
		{"a member who said no", notComingId.Hex()},
		{"a member who has not answered", absentId.Hex()},
		{"an account with no connection to the club", primitive.NewObjectID().Hex()},
		{"something that is not an id at all", "not-an-id"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+tc.id+`"}`, cookie); code != http.StatusBadRequest {
				t.Errorf("assign to %s: got %d, want 400", tc.name, code)
			}
		})
	}

	// Nothing was written by any of those.
	if stored := readEventLists(t, eventId); stored[0].Items[0].AssigneeId != nil {
		t.Errorf("a refused assignment was stored anyway: %+v", stored[0].Items[0])
	}
}

// The gathering's owner is assignable with no RSVP at all, and even having
// declined their own gathering.
//
// This is the rule that made the feature usable: measured against production the
// day N1 shipped, one of thirteen gatherings had any going/maybe RSVP, so an
// attendance-only pool left the single gathering with checklists showing an
// empty picker.
func TestAssign_OwnerIsAlwaysAssignable(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-owner-rule@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()

	cookie := loginAs(t, h, ownerId.Hex())
	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Book the room"}`)

	// No RSVP anywhere on this event.
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie); code != http.StatusOK {
		t.Fatalf("assign to the owner with no RSVP: got %d, want 200", code)
	}

	// And still, having said no.
	setRsvp(t, eventId, ownerId, models.RsvpNo)
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":""}`, cookie); code != http.StatusOK {
		t.Fatalf("unassign: got %d, want 200", code)
	}
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie); code != http.StatusOK {
		t.Errorf("assign to an owner who declined: got %d, want 200", code)
	}

	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/lists/assignees", "", cookie)
	var users []models.User
	json.Unmarshal(w.Body.Bytes(), &users)
	if len(users) != 1 || users[0].Id != ownerId {
		t.Errorf("picker should hold the owner alone, got %d entries", len(users))
	}
}

// Someone already holding an entry stays in the picker even after changing their
// RSVP to "no" — otherwise reopening the menu on that entry would offer no way
// back to the name the page is already showing.
func TestAssign_CurrentHolderStaysInThePicker(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-holder-owner@example.test")
	mateId := insertTestUser(t, models.RoleMember, "assign-holder-mate@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()

	cookie := loginAs(t, h, ownerId.Hex())
	setRsvp(t, eventId, mateId, models.RsvpGoing)

	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Bring the port"}`)
	other := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Bring the cheese"}`)

	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+mateId.Hex()+`"}`, cookie); code != http.StatusOK {
		t.Fatalf("assign: got %d, want 200", code)
	}

	// They now pull out of the gathering, but keep the entry.
	setRsvp(t, eventId, mateId, models.RsvpNo)

	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/lists/assignees", "", cookie)
	var users []models.User
	json.Unmarshal(w.Body.Bytes(), &users)
	held := false
	for _, u := range users {
		if u.Id == mateId {
			held = true
		}
	}
	if !held {
		t.Error("a current holder dropped out of the picker when their RSVP changed")
	}

	// Reproducible, which is the whole point.
	if code := assignItem(h, eventId, list.Id, other.Id, `{"assigneeId":"`+mateId.Hex()+`"}`, cookie); code != http.StatusOK {
		t.Errorf("re-assign to the current holder: got %d, want 200", code)
	}

	// But once they hold nothing, the declined RSVP is all that is left of them.
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":""}`, cookie); code != http.StatusOK {
		t.Fatalf("unassign first: got %d, want 200", code)
	}
	if code := assignItem(h, eventId, list.Id, other.Id, `{"assigneeId":""}`, cookie); code != http.StatusOK {
		t.Fatalf("unassign second: got %d, want 200", code)
	}
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+mateId.Hex()+`"}`, cookie); code != http.StatusBadRequest {
		t.Errorf("assign to a declined member holding nothing: got %d, want 400", code)
	}
}

// The member-or-above filter applies to all three sources, not just the RSVPs:
// a guest holds no responsibilities here whether they RSVP'd, own the gathering,
// or were assigned something before a role change.
func TestAssign_RoleFilterAppliesToOwnerAndHolder(t *testing.T) {
	requireDB(t)

	guestOwnerId := insertTestUser(t, models.RoleGuest, "assign-guest-owner2@example.test")
	adminId := insertTestUser(t, models.RoleAdmin, "assign-role-admin@example.test")
	eventId := insertTestEvent(t, guestOwnerId)
	h := assignTestRouter()

	cookie := loginAs(t, h, adminId.Hex())
	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Bring the port"}`)

	// The owner is a guest, so being the owner does not let them in.
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+guestOwnerId.Hex()+`"}`, cookie); code != http.StatusBadRequest {
		t.Errorf("assign to a guest owner: got %d, want 400", code)
	}

	// Nor does holding an entry: plant one directly, as a role change after the
	// fact would leave behind, and check it is not re-selectable.
	if _, err := db.SetEventListItemAssignee(eventId, list.Id, item.Id, &guestOwnerId, "Test User"); err != nil {
		t.Fatalf("plant assignment: %v", err)
	}
	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/lists/assignees", "", cookie)
	var users []models.User
	json.Unmarshal(w.Body.Bytes(), &users)
	for _, u := range users {
		if u.Id == guestOwnerId {
			t.Error("a guest reached the picker by holding an entry")
		}
	}
	// The name still RENDERS, though — it is on the item, and hiding it would
	// make the entry look unclaimed.
	lists := readEventLists(t, eventId)
	if lists[0].Items[0].AssigneeName == "" {
		t.Error("the planted assignment stopped rendering")
	}
}

// A guest sees who each entry is for and changes none of it. This is the
// difference between the assign right and every other list right: a guest may
// add entries and tick boxes.
func TestAssign_GuestsReadButDoNotWrite(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-guest-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "assign-guest@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()

	ownerCookie := loginAs(t, h, ownerId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())
	setRsvp(t, eventId, ownerId, models.RsvpGoing)

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"Menu","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, ownerCookie, `{"text":"Bring the port"}`)
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, ownerCookie); code != http.StatusOK {
		t.Fatalf("assign: got %d, want 200", code)
	}

	// The guest may not assign, nor unassign.
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, guestCookie); code != http.StatusForbidden {
		t.Errorf("guest assign: got %d, want 403", code)
	}
	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":""}`, guestCookie); code != http.StatusForbidden {
		t.Errorf("guest unassign: got %d, want 403", code)
	}
	// Nor may they see the picker.
	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/lists/assignees", "", guestCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("guest assignees: got %d, want 403", w.Code)
	}

	// But they DO see who it is for, and can still tick it.
	w = do(h, http.MethodGet, "/events/"+eventId.Hex()+"/lists", "", guestCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("guest read lists: got %d, want 200", w.Code)
	}
	var lists []models.EventList
	json.Unmarshal(w.Body.Bytes(), &lists)
	if lists[0].Items[0].AssigneeName != "Test User" {
		t.Errorf("a guest cannot see who the entry is for: %+v", lists[0].Items[0])
	}
	w = do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/lists/"+list.Id.Hex()+"/items/"+item.Id.Hex()+"/checked",
		`{"checked":true}`, guestCookie)
	if w.Code != http.StatusOK {
		t.Errorf("guest tick: got %d, want 200 — assigning is the only thing they lose", w.Code)
	}
}

// The picker's own contents: attending members, alphabetized, guests and
// non-attendees dropped.
func TestAssign_AssigneesEndpoint(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assignees-owner@example.test")
	maybeId := insertTestUser(t, models.RoleMember, "assignees-maybe@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "assignees-guest@example.test")
	noId := insertTestUser(t, models.RoleMember, "assignees-no@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()

	cookie := loginAs(t, h, ownerId.Hex())
	setRsvp(t, eventId, ownerId, models.RsvpGoing)
	// "Maybe" counts: someone weighing it up is exactly who you want to be able
	// to give a job to.
	setRsvp(t, eventId, maybeId, models.RsvpMaybe)
	setRsvp(t, eventId, guestId, models.RsvpGoing)
	setRsvp(t, eventId, noId, models.RsvpNo)
	// A legacy name-keyed row: no account behind it, so no candidate.
	_, err := db.EventsCollection.UpdateOne(context.Background(), bson.M{"_id": eventId},
		bson.M{"$set": bson.M{"rsvps.Cousin Ed": bson.M{"status": models.RsvpGoing}}})
	if err != nil {
		t.Fatalf("set legacy rsvp: %v", err)
	}

	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/lists/assignees", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("assignees: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var users []models.User
	json.Unmarshal(w.Body.Bytes(), &users)

	got := map[primitive.ObjectID]bool{}
	for _, u := range users {
		got[u.Id] = true
		// Slimmed like every other picker: identity and picture, no email, no
		// role, no calendars.
		if u.Email != "" {
			t.Errorf("assignee %s carries an email", u.Id.Hex())
		}
	}
	if !got[ownerId] || !got[maybeId] {
		t.Errorf("attending members missing from the picker: %v", got)
	}
	if got[guestId] {
		t.Error("a guest is in the picker")
	}
	if got[noId] {
		t.Error("a member who said no is in the picker")
	}
	if len(users) != 2 {
		t.Errorf("got %d assignees, want 2", len(users))
	}
}

// Assignment is a property of the list's KIND, exactly as the checkbox is.
func TestAssign_RefusesANonChecklist(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-kind-owner@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()

	cookie := loginAs(t, h, ownerId.Hex())
	setRsvp(t, eventId, ownerId, models.RsvpGoing)

	list := createListFor(t, h, eventId, cookie, `{"name":"Bars","kind":"location"}`)
	item := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"The Eagle"}`)

	if code := assignItem(h, eventId, list.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie); code != http.StatusBadRequest {
		t.Errorf("assign on a location list: got %d, want 400", code)
	}
}

// SourceListId/SourceListName are `bson:"-"`, and this is what that tag is for:
// moveListItems $pushes whole EventListItem values, so an untagged field would
// be written onto the real entry the first time anyone dragged one. AssigneeId
// is stored and must survive the same move.
func TestAssign_MoveCarriesTheAssigneeButNotTheDerivedFields(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-move-owner@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := assignTestRouter()

	cookie := loginAs(t, h, ownerId.Hex())
	setRsvp(t, eventId, ownerId, models.RsvpGoing)

	src := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"checklist"}`)
	dst := createListFor(t, h, eventId, cookie, `{"name":"Tasks","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, src.Id, cookie, `{"text":"Bring the port"}`)

	if code := assignItem(h, eventId, src.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie); code != http.StatusOK {
		t.Fatalf("assign: got %d, want 200", code)
	}

	w := do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/lists/"+src.Id.Hex()+"/items/"+item.Id.Hex()+"/move",
		`{"targetListId":"`+dst.Id.Hex()+`","order":1024}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("move: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	// Read the RAW document: struct decoding would hide a stored sourceListId
	// behind the `bson:"-"` that is exactly what is under test.
	var raw bson.M
	if err := db.EventsCollection.FindOne(context.Background(), bson.M{"_id": eventId}).Decode(&raw); err != nil {
		t.Fatalf("re-read event: %v", err)
	}
	for _, listVal := range raw["lists"].(bson.A) {
		list := listVal.(bson.M)
		items, _ := list["items"].(bson.A)
		for _, itemVal := range items {
			stored := itemVal.(bson.M)
			if _, leaked := stored["sourceListId"]; leaked {
				t.Error("sourceListId was persisted — the bson:\"-\" tag is not holding")
			}
			if _, leaked := stored["sourceListName"]; leaked {
				t.Error("sourceListName was persisted — the bson:\"-\" tag is not holding")
			}
			if _, leaked := stored["virtual"]; leaked {
				t.Error("EventList.Virtual was persisted")
			}
		}
	}

	// The assignment itself is stored, and a move must not drop it.
	moved := readEventLists(t, eventId)
	found := false
	for _, list := range moved {
		if list.Id != dst.Id {
			continue
		}
		for _, stored := range list.Items {
			if stored.Id != item.Id {
				continue
			}
			found = true
			if stored.AssigneeId == nil || *stored.AssigneeId != ownerId {
				t.Errorf("the move dropped the assignment: %+v", stored)
			}
		}
	}
	if !found {
		t.Fatalf("moved item not found on the target list: %+v", moved)
	}
}

// A member's own list called "Assigned" is untouched by the derived one, and
// keeps working as a normal list.
func TestAssign_CoexistsWithAnOwnListOfTheSameName(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "assign-clash-owner@example.test")
	eventId := insertTestEvent(t, ownerId)
	cleanupPersonal(t, eventId)
	h := assignTestRouter()

	cookie := loginAs(t, h, ownerId.Hex())
	setRsvp(t, eventId, ownerId, models.RsvpGoing)

	mine := createPersonalListFor(t, h, eventId.Hex(), cookie, `{"name":"Assigned","kind":"checklist"}`)
	shared := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, shared.Id, cookie, `{"text":"Bring the port"}`)
	if code := assignItem(h, eventId, shared.Id, item.Id, `{"assigneeId":"`+ownerId.Hex()+`"}`, cookie); code != http.StatusOK {
		t.Fatalf("assign: got %d, want 200", code)
	}

	lists := readMyLists(t, h, eventId, cookie)
	if len(lists) != 2 {
		t.Fatalf("got %d lists, want the derived one plus the viewer's own: %+v", len(lists), lists)
	}
	if !lists[0].Virtual {
		t.Error("the derived list must come first")
	}
	if lists[1].Virtual || lists[1].Id != mine.Id {
		t.Errorf("the viewer's own list was displaced: %+v", lists[1])
	}
}
