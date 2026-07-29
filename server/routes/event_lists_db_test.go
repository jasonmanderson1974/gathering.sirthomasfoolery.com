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

// listsTestRouter wires the list routes behind the same AuthRequired middleware
// production uses, plus a test-login.
func listsTestRouter() *gin.Engine {
	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events/:eventId/lists", middleware.AuthRequired(), createEventList)
	r.PATCH("/events/:eventId/lists/:listId", middleware.AuthRequired(), renameEventList)
	r.DELETE("/events/:eventId/lists/:listId", middleware.AuthRequired(), deleteEventList)
	r.POST("/events/:eventId/lists/:listId/items", middleware.AuthRequired(), addEventListItem)
	r.PUT("/events/:eventId/lists/:listId/items/:itemId", middleware.AuthRequired(), editEventListItem)
	r.DELETE("/events/:eventId/lists/:listId/items/:itemId", middleware.AuthRequired(), deleteEventListItem)
	return r
}

// readEventLists re-reads the stored lists, so assertions check what actually
// landed in Mongo rather than what a handler said it did.
func readEventLists(t *testing.T, eventId primitive.ObjectID) []models.EventList {
	t.Helper()
	var event models.Event
	err := db.EventsCollection.FindOne(context.Background(), bson.M{"_id": eventId}).Decode(&event)
	if err != nil {
		t.Fatalf("re-read event: %v", err)
	}
	return event.Lists
}

func createListFor(t *testing.T, h *gin.Engine, eventId primitive.ObjectID, cookie *http.Cookie, body string) models.EventList {
	t.Helper()
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/lists", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("create list: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var list models.EventList
	json.Unmarshal(w.Body.Bytes(), &list)
	return list
}

func addItemFor(t *testing.T, h *gin.Engine, eventId, listId primitive.ObjectID, cookie *http.Cookie, body string) models.EventListItem {
	t.Helper()
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/lists/"+listId.Hex()+"/items", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("add item: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var item models.EventListItem
	json.Unmarshal(w.Body.Bytes(), &item)
	return item
}

func TestLists_RequiresSignIn(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-signin@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()

	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/lists", `{"name":"Menu"}`, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create: got %d, want 401 (body: %s)", w.Code, w.Body.String())
	}
}

// The headline flow: the planner sets up a list, a guest fills it in, and the
// guest can edit and remove what they added.
func TestLists_PlannerCreatesGuestFillsIn(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "lists-guest@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()

	ownerCookie := loginAs(t, h, ownerId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())

	// A guest may not create a list.
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/lists", `{"name":"Menu"}`, guestCookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest create list: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}

	// A blank name is refused.
	w = do(h, http.MethodPost, "/events/"+eventId.Hex()+"/lists", `{"name":"   "}`, ownerCookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("blank name: got %d, want 400", w.Code)
	}

	// An unknown kind is refused rather than defaulted.
	w = do(h, http.MethodPost, "/events/"+eventId.Hex()+"/lists", `{"name":"Menu","kind":"places"}`, ownerCookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad kind: got %d, want 400", w.Code)
	}

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"  Menu  ","kind":"text"}`)
	if list.Name != "Menu" || list.Kind != models.ListKindText {
		t.Fatalf("unexpected created list: %+v", list)
	}

	// The guest adds an item — credited to their account, not the payload.
	item := addItemFor(t, h, eventId, list.Id, guestCookie, `{"text":"Hotdogs"}`)
	if item.UserId != guestId || item.AuthorName != "Test User" {
		t.Fatalf("item not credited to the signed-in guest: %+v", item)
	}
	if item.CreatedAt == 0 {
		t.Error("item has no createdAt")
	}

	// It really landed on the list.
	lists := readEventLists(t, eventId)
	if len(lists) != 1 || len(lists[0].Items) != 1 || lists[0].Items[0].Text != "Hotdogs" {
		t.Fatalf("stored lists wrong after add: %+v", lists)
	}

	// The guest edits their own item.
	w = do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/lists/"+list.Id.Hex()+"/items/"+item.Id.Hex(),
		`{"text":"Bratwurst"}`, guestCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("guest edit own: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	lists = readEventLists(t, eventId)
	if lists[0].Items[0].Text != "Bratwurst" {
		t.Fatalf("edit did not persist: %+v", lists[0].Items)
	}
	// Editing must not re-stamp authorship.
	if lists[0].Items[0].UserId != guestId {
		t.Error("edit changed the item's author")
	}

	// The guest deletes their own item.
	w = do(h, http.MethodDelete,
		"/events/"+eventId.Hex()+"/lists/"+list.Id.Hex()+"/items/"+item.Id.Hex(), "", guestCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("guest delete own: got %d, want 200", w.Code)
	}
	if lists = readEventLists(t, eventId); len(lists[0].Items) != 0 {
		t.Fatalf("item not removed: %+v", lists[0].Items)
	}
	// Deleting again is idempotent, not a 404.
	w = do(h, http.MethodDelete,
		"/events/"+eventId.Hex()+"/lists/"+list.Id.Hex()+"/items/"+item.Id.Hex(), "", guestCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("re-delete item: got %d, want 200 (idempotent)", w.Code)
	}
}

// A guest may not touch someone else's entry; a member may remove it but still
// may not rewrite it.
func TestLists_OtherPeoplesItems(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-oi-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "lists-oi-guest@example.test")
	otherGuestId := insertTestUser(t, models.RoleGuest, "lists-oi-guest2@example.test")
	memberId := insertTestUser(t, models.RoleMember, "lists-oi-member@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()

	ownerCookie := loginAs(t, h, ownerId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())
	otherGuestCookie := loginAs(t, h, otherGuestId.Hex())
	memberCookie := loginAs(t, h, memberId.Hex())

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"Menu","kind":"text"}`)
	item := addItemFor(t, h, eventId, list.Id, guestCookie, `{"text":"Hotdogs"}`)
	itemPath := "/events/" + eventId.Hex() + "/lists/" + list.Id.Hex() + "/items/" + item.Id.Hex()

	// Another guest can neither edit nor delete it.
	if w := do(h, http.MethodPut, itemPath, `{"text":"Tofu"}`, otherGuestCookie); w.Code != http.StatusForbidden {
		t.Fatalf("other guest edit: got %d, want 403", w.Code)
	}
	if w := do(h, http.MethodDelete, itemPath, "", otherGuestCookie); w.Code != http.StatusForbidden {
		t.Fatalf("other guest delete: got %d, want 403", w.Code)
	}

	// A member may not rewrite it either — deleting is the override, not editing.
	if w := do(h, http.MethodPut, itemPath, `{"text":"Tofu"}`, memberCookie); w.Code != http.StatusForbidden {
		t.Fatalf("member edit another's item: got %d, want 403", w.Code)
	}
	if lists := readEventLists(t, eventId); lists[0].Items[0].Text != "Hotdogs" {
		t.Fatalf("a refused edit still changed the text: %+v", lists[0].Items)
	}

	// But a member may remove it.
	if w := do(h, http.MethodDelete, itemPath, "", memberCookie); w.Code != http.StatusOK {
		t.Fatalf("member delete another's item: got %d, want 200", w.Code)
	}
	if lists := readEventLists(t, eventId); len(lists[0].Items) != 0 {
		t.Fatalf("member delete did not persist: %+v", lists[0].Items)
	}
}

// Renaming and deleting a whole list is the planner's and the admin's, not a
// plain member's — even though that member may delete individual entries.
func TestLists_ManageIsPlannerAndAdminOnly(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-mg-owner@example.test")
	memberId := insertTestUser(t, models.RoleMember, "lists-mg-member@example.test")
	adminId := insertTestUser(t, models.RoleAdmin, "lists-mg-admin@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()

	ownerCookie := loginAs(t, h, ownerId.Hex())
	memberCookie := loginAs(t, h, memberId.Hex())
	adminCookie := loginAs(t, h, adminId.Hex())

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"Menu","kind":"text"}`)
	listPath := "/events/" + eventId.Hex() + "/lists/" + list.Id.Hex()

	// A member on someone else's event may not rename or delete the list.
	if w := do(h, http.MethodPatch, listPath, `{"name":"Hijacked"}`, memberCookie); w.Code != http.StatusForbidden {
		t.Fatalf("member rename: got %d, want 403", w.Code)
	}
	if w := do(h, http.MethodDelete, listPath, "", memberCookie); w.Code != http.StatusForbidden {
		t.Fatalf("member delete list: got %d, want 403", w.Code)
	}
	if lists := readEventLists(t, eventId); len(lists) != 1 || lists[0].Name != "Menu" {
		t.Fatalf("a refused manage call still changed the lists: %+v", lists)
	}

	// The owner renames it.
	if w := do(h, http.MethodPatch, listPath, `{"name":"  Supper  "}`, ownerCookie); w.Code != http.StatusOK {
		t.Fatalf("owner rename: got %d, want 200", w.Code)
	}
	if lists := readEventLists(t, eventId); lists[0].Name != "Supper" {
		t.Fatalf("rename did not persist trimmed: %+v", lists)
	}

	// An admin may delete a list on an event they don't own, taking its items.
	addItemFor(t, h, eventId, list.Id, ownerCookie, `{"text":"Hotdogs"}`)
	if w := do(h, http.MethodDelete, listPath, "", adminCookie); w.Code != http.StatusOK {
		t.Fatalf("admin delete list: got %d, want 200", w.Code)
	}
	if lists := readEventLists(t, eventId); len(lists) != 0 {
		t.Fatalf("list not deleted: %+v", lists)
	}
	// Deleting again is idempotent.
	if w := do(h, http.MethodDelete, listPath, "", adminCookie); w.Code != http.StatusOK {
		t.Fatalf("re-delete list: got %d, want 200 (idempotent)", w.Code)
	}
	// Adding to a list that is gone is a 404, not a silent success.
	w := do(h, http.MethodPost, listPath+"/items", `{"text":"Ghost"}`, ownerCookie)
	if w.Code != http.StatusNotFound {
		t.Fatalf("add to deleted list: got %d, want 404", w.Code)
	}
}

// Two lists on one event must stay independent — this is what the targeted
// array update buys over rewriting the whole `lists` array.
func TestLists_AddsToOneListLeaveTheOtherAlone(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-ind-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "lists-ind-guest@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()

	ownerCookie := loginAs(t, h, ownerId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())

	menu := createListFor(t, h, eventId, ownerCookie, `{"name":"Menu","kind":"text"}`)
	bars := createListFor(t, h, eventId, ownerCookie, `{"name":"Bars to Visit","kind":"location"}`)

	addItemFor(t, h, eventId, menu.Id, guestCookie, `{"text":"Hotdogs"}`)
	addItemFor(t, h, eventId, bars.Id, guestCookie, `{"text":"The Fox & Hound, Sacramento CA"}`)
	addItemFor(t, h, eventId, menu.Id, ownerCookie, `{"text":"Hamburgers"}`)

	lists := readEventLists(t, eventId)
	if len(lists) != 2 {
		t.Fatalf("got %d lists, want 2: %+v", len(lists), lists)
	}
	byName := map[string]models.EventList{}
	for _, l := range lists {
		byName[l.Name] = l
	}
	if got := len(byName["Menu"].Items); got != 2 {
		t.Errorf("Menu has %d items, want 2", got)
	}
	if got := len(byName["Bars to Visit"].Items); got != 1 {
		t.Errorf("Bars to Visit has %d items, want 1", got)
	}
	if byName["Bars to Visit"].Kind != models.ListKindLocation {
		t.Errorf("location list lost its kind: %+v", byName["Bars to Visit"])
	}
	// Ordering is insertion order, which is what the UI renders.
	if byName["Menu"].Items[0].Text != "Hotdogs" || byName["Menu"].Items[1].Text != "Hamburgers" {
		t.Errorf("items are not in insertion order: %+v", byName["Menu"].Items)
	}
}

// A nickname set after the fact must reach items written before it, the way it
// does for comments, RSVPs and poll votes (F3).
func TestLists_ItemAuthorNamesResolveAtReadTime(t *testing.T) {
	requireDB(t)

	authorId := insertTestUser(t, models.RoleMember, "lists-nick@example.test")
	eventId := insertTestEvent(t, authorId)
	h := listsTestRouter()
	cookie := loginAs(t, h, authorId.Hex())

	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)
	addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Hotdogs"}`)

	// The stored snapshot is the name at write time.
	if lists := readEventLists(t, eventId); lists[0].Items[0].AuthorName != "Test User" {
		t.Fatalf("snapshot wrong: %+v", lists[0].Items[0])
	}

	// Set a nickname, then resolve as getEvent does.
	_, err := db.UsersCollection.UpdateByID(context.Background(), authorId,
		bson.M{"$set": bson.M{"nickname": "Barkeep"}})
	if err != nil {
		t.Fatalf("set nickname: %v", err)
	}

	event := &models.Event{Id: eventId, Lists: readEventLists(t, eventId)}
	resolveEventDisplayNames(event)
	if got := event.Lists[0].Items[0].AuthorName; got != "Barkeep" {
		t.Errorf("resolved author name = %q, want %q", got, "Barkeep")
	}

	// The stored snapshot is left alone — resolution is read-time only.
	if lists := readEventLists(t, eventId); lists[0].Items[0].AuthorName != "Test User" {
		t.Error("read-time resolution rewrote the stored snapshot")
	}
}
