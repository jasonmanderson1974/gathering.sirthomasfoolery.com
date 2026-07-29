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
	r.PUT("/events/:eventId/lists/:listId/items/:itemId/checked", middleware.AuthRequired(), setEventListItemChecked)
	r.DELETE("/events/:eventId/lists/:listId/items/:itemId", middleware.AuthRequired(), deleteEventListItem)
	r.GET("/events/:eventId/lists", middleware.AuthRequired(), getEventLists)
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

// itemsByText indexes a list's items for assertions that don't care about
// position.
func itemsByText(items []models.EventListItem) map[string]models.EventListItem {
	byText := make(map[string]models.EventListItem, len(items))
	for _, item := range items {
		byText[item.Text] = item
	}
	return byText
}

// Nesting end to end: three levels are allowed, the fourth is refused, and an
// unknown parent is a 404 rather than a silently top-level item.
func TestLists_NestedItems(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-nest@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)
	itemsPath := "/events/" + eventId.Hex() + "/lists/" + list.Id.Hex() + "/items"

	root := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Mains"}`)
	if root.ParentId != nil {
		t.Errorf("a top-level item got a parent: %+v", root)
	}
	child := addItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"Hotdogs","parentId":"`+root.Id.Hex()+`"}`)
	if child.ParentId == nil || *child.ParentId != root.Id {
		t.Fatalf("child's parent = %v, want %s", child.ParentId, root.Id.Hex())
	}
	grandchild := addItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"Mustard","parentId":"`+child.Id.Hex()+`"}`)

	// A fourth level is where it stops.
	w := do(h, http.MethodPost, itemsPath, `{"text":"Dijon","parentId":"`+grandchild.Id.Hex()+`"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("4th level: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), errListTooDeep) {
		t.Errorf("4th level error = %s, want %s", w.Body.String(), errListTooDeep)
	}

	// A parent that isn't there is a 404, not a top-level item.
	w = do(h, http.MethodPost, itemsPath,
		`{"text":"Orphan","parentId":"`+primitive.NewObjectID().Hex()+`"}`, cookie)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown parent: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}

	stored := itemsByText(readEventLists(t, eventId)[0].Items)
	if len(stored) != 3 {
		t.Fatalf("stored %d items, want 3: %+v", len(stored), stored)
	}
	if stored["Mustard"].ParentId == nil || *stored["Mustard"].ParentId != child.Id {
		t.Errorf("grandchild's parent did not persist: %+v", stored["Mustard"])
	}
}

// Deleting an item takes its subtree with it — including children someone else
// added — and leaves everything beside it alone.
func TestLists_DeleteCascadesToTheSubtree(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-cascade-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "lists-cascade-guest@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	ownerCookie := loginAs(t, h, ownerId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"Menu","kind":"text"}`)
	root := addItemFor(t, h, eventId, list.Id, ownerCookie, `{"text":"Mains"}`)
	child := addItemFor(t, h, eventId, list.Id, ownerCookie,
		`{"text":"Hotdogs","parentId":"`+root.Id.Hex()+`"}`)
	// Someone else's item, nested under the one about to be deleted.
	addItemFor(t, h, eventId, list.Id, guestCookie,
		`{"text":"Mustard","parentId":"`+child.Id.Hex()+`"}`)
	// A sibling that must survive.
	addItemFor(t, h, eventId, list.Id, ownerCookie,
		`{"text":"Salad","parentId":"`+root.Id.Hex()+`"}`)

	itemPath := "/events/" + eventId.Hex() + "/lists/" + list.Id.Hex() + "/items/" + child.Id.Hex()
	if w := do(h, http.MethodDelete, itemPath, "", ownerCookie); w.Code != http.StatusOK {
		t.Fatalf("delete: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	stored := itemsByText(readEventLists(t, eventId)[0].Items)
	if len(stored) != 2 {
		t.Fatalf("stored %d items, want 2 (Mains + Salad): %+v", len(stored), stored)
	}
	if _, ok := stored["Mustard"]; ok {
		t.Error("the guest's nested item survived its parent being deleted")
	}
	if _, ok := stored["Salad"]; !ok {
		t.Error("a sibling was deleted along with the subtree")
	}

	// Still idempotent.
	if w := do(h, http.MethodDelete, itemPath, "", ownerCookie); w.Code != http.StatusOK {
		t.Errorf("re-delete: got %d, want 200", w.Code)
	}
}

// Anyone signed in may tick a box, both directions are attributed, and only the
// last change is kept.
func TestLists_CheckAndUncheck(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-check-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "lists-check-guest@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	ownerCookie := loginAs(t, h, ownerId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"Jobs","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, ownerCookie, `{"text":"Bring ice"}`)
	checkedPath := "/events/" + eventId.Hex() + "/lists/" + list.Id.Hex() +
		"/items/" + item.Id.Hex() + "/checked"

	// Untouched: no state at all, so the UI renders nothing rather than
	// "unchecked by" someone who never saw it.
	if stored := readEventLists(t, eventId)[0].Items[0]; stored.Checked || stored.CheckedBy != nil {
		t.Fatalf("a new item carries checklist state: %+v", stored)
	}

	// A guest — the lowest role — may tick it.
	if w := do(h, http.MethodPut, checkedPath, `{"checked":true}`, guestCookie); w.Code != http.StatusOK {
		t.Fatalf("guest check: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	stored := readEventLists(t, eventId)[0].Items[0]
	if !stored.Checked || stored.CheckedBy == nil || *stored.CheckedBy != guestId {
		t.Fatalf("check did not record the guest: %+v", stored)
	}
	if stored.CheckedByName != "Test User" || stored.CheckedAt == 0 {
		t.Errorf("check did not stamp name and time: %+v", stored)
	}

	// Unchecking is a state change too, credited to whoever did it.
	if w := do(h, http.MethodPut, checkedPath, `{"checked":false}`, ownerCookie); w.Code != http.StatusOK {
		t.Fatalf("owner uncheck: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	stored = readEventLists(t, eventId)[0].Items[0]
	if stored.Checked {
		t.Error("uncheck did not clear the checked flag")
	}
	if stored.CheckedBy == nil || *stored.CheckedBy != ownerId {
		t.Errorf("uncheck kept the previous person: %+v", stored)
	}

	// Re-applying the same state is success, not a 404 — nothing was modified.
	if w := do(h, http.MethodPut, checkedPath, `{"checked":false}`, ownerCookie); w.Code != http.StatusOK {
		t.Errorf("re-uncheck: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

// The checkbox belongs to checklists. Offering it on a text list would imply a
// state the UI has nowhere to show.
func TestLists_CheckRejectsANonChecklist(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-check-kind@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())

	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)
	item := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Hotdogs"}`)

	w := do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/lists/"+list.Id.Hex()+"/items/"+item.Id.Hex()+"/checked",
		`{"checked":true}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("check on a text list: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), errNotChecklist) {
		t.Errorf("error = %s, want %s", w.Body.String(), errNotChecklist)
	}
}

// The cheap refresh: the lists alone, names resolved, without paying for the
// whole event.
func TestLists_GetListsEndpoint(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-get-owner@example.test")
	checkerId := insertTestUser(t, models.RoleGuest, "lists-get-checker@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	ownerCookie := loginAs(t, h, ownerId.Hex())
	checkerCookie := loginAs(t, h, checkerId.Hex())
	listsPath := "/events/" + eventId.Hex() + "/lists"

	if w := do(h, http.MethodGet, listsPath, "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read: got %d, want 401", w.Code)
	}

	// An event with no lists answers [], not null, so the client can splice the
	// result in without a nil check.
	w := do(h, http.MethodGet, listsPath, "", ownerCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("empty read: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if body := strings.TrimSpace(w.Body.String()); body != "[]" {
		t.Errorf("empty read body = %s, want []", body)
	}

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"Jobs","kind":"checklist"}`)
	item := addItemFor(t, h, eventId, list.Id, ownerCookie, `{"text":"Bring ice"}`)
	if w := do(h, http.MethodPut,
		"/events/"+eventId.Hex()+"/lists/"+list.Id.Hex()+"/items/"+item.Id.Hex()+"/checked",
		`{"checked":true}`, checkerCookie); w.Code != http.StatusOK {
		t.Fatalf("check: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	// Both names follow a nickname change, as they do through getEvent.
	if _, err := db.UsersCollection.UpdateByID(context.Background(), checkerId,
		bson.M{"$set": bson.M{"nickname": "Barkeep"}}); err != nil {
		t.Fatalf("set nickname: %v", err)
	}

	// A guest reads the same shape as the planner.
	w = do(h, http.MethodGet, listsPath, "", checkerCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("read: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var lists []models.EventList
	if err := json.Unmarshal(w.Body.Bytes(), &lists); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, w.Body.String())
	}
	if len(lists) != 1 || len(lists[0].Items) != 1 {
		t.Fatalf("got %+v, want one list with one item", lists)
	}
	got := lists[0].Items[0]
	if got.AuthorName != "Test User" {
		t.Errorf("author name = %q", got.AuthorName)
	}
	if !got.Checked || got.CheckedByName != "Barkeep" {
		t.Errorf("checked state did not resolve: %+v", got)
	}
	if lists[0].Kind != models.ListKindChecklist {
		t.Errorf("kind = %q, want checklist", lists[0].Kind)
	}
}
