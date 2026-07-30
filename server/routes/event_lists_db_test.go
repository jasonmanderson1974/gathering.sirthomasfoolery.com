package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	r.PUT("/events/:eventId/lists/:listId/items/:itemId/move", middleware.AuthRequired(), moveEventListItem)
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

// Adds append: every new entry is stamped with an order above everything
// already on the list, children included, and the value survives the round trip
// to Mongo.
func TestLists_AddsStampAnIncreasingOrder(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-order@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)

	first := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Beer"}`)
	if first.Order != listItemOrderStep {
		t.Errorf("first item's order = %v, want %v", first.Order, float64(listItemOrderStep))
	}
	second := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Chips"}`)
	// A child competes only with its siblings, but it is still stamped past the
	// list-wide maximum, so the values keep climbing regardless of nesting.
	child := addItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"Salsa","parentId":"`+second.Id.Hex()+`"}`)

	if !(first.Order < second.Order && second.Order < child.Order) {
		t.Errorf("orders are not increasing: %v, %v, %v", first.Order, second.Order, child.Order)
	}

	stored := itemsByText(readEventLists(t, eventId)[0].Items)
	if stored["Beer"].Order != first.Order || stored["Salsa"].Order != child.Order {
		t.Errorf("orders did not persist: %+v", stored)
	}
}

// A zero order has to survive as a stored zero, not vanish. Dropping an entry at
// the top of a list legitimately computes 0, and the F17 migration uses the
// field's ABSENCE to recognize entries written before ordering existed — so an
// omitempty on models.EventListItem.Order would quietly conflate the two. This
// asserts against the raw document, because a decode back into the struct turns
// "absent" into 0 and would pass either way.
func TestLists_ZeroOrderIsStoredNotOmitted(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-zero-order@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)

	item := models.EventListItem{
		Id:     primitive.NewObjectID(),
		Text:   "Dropped at the top",
		Order:  0,
		UserId: ownerId,
	}
	if _, err := db.InsertEventListItem(eventId, list.Id, item); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var raw bson.M
	err := db.EventsCollection.FindOne(context.Background(), bson.M{"_id": eventId}).Decode(&raw)
	if err != nil {
		t.Fatalf("read raw event: %v", err)
	}
	lists := raw["lists"].(primitive.A)
	items := lists[0].(bson.M)["items"].(primitive.A)
	first := items[0].(bson.M)
	order, present := first["order"]
	if !present {
		t.Fatalf("order was omitted from the stored document: %+v", first)
	}
	if order != float64(0) {
		t.Errorf("stored order = %v (%T), want 0", order, order)
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

// movePath builds the move URL for an item.
func movePath(eventId, listId, itemId primitive.ObjectID) string {
	return "/events/" + eventId.Hex() + "/lists/" + listId.Hex() +
		"/items/" + itemId.Hex() + "/move"
}

// listByName picks a list out of the stored event by name.
func listByName(t *testing.T, lists []models.EventList, name string) models.EventList {
	t.Helper()
	for _, list := range lists {
		if list.Name == name {
			return list
		}
	}
	t.Fatalf("list %q is not on the event: %+v", name, lists)
	return models.EventList{}
}

// A same-list move either reorders an item among the siblings it already has —
// naming its current parent — or flattens it to the top level. Both are one
// targeted write, and neither disturbs anything else on the list.
func TestLists_MoveWithinAList(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-move-same@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)

	parent := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Mains"}`)
	first := addItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"Hotdogs","parentId":"`+parent.Id.Hex()+`"}`)
	second := addItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"Burgers","parentId":"`+parent.Id.Hex()+`"}`)

	// Reorder within the parent: Burgers above Hotdogs, parent kept.
	body := `{"targetListId":"` + list.Id.Hex() + `","order":` +
		strconv.FormatFloat(first.Order-1, 'f', -1, 64) +
		`,"parentId":"` + parent.Id.Hex() + `"}`
	if w := do(h, http.MethodPut, movePath(eventId, list.Id, second.Id), body, cookie); w.Code != http.StatusOK {
		t.Fatalf("sibling reorder: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	stored := itemsByText(readEventLists(t, eventId)[0].Items)
	if stored["Burgers"].Order >= stored["Hotdogs"].Order {
		t.Errorf("reorder did not take: Burgers %v, Hotdogs %v",
			stored["Burgers"].Order, stored["Hotdogs"].Order)
	}
	if stored["Burgers"].ParentId == nil || *stored["Burgers"].ParentId != parent.Id {
		t.Errorf("a sibling reorder lost the parent: %+v", stored["Burgers"])
	}
	if stored["Hotdogs"].Order != first.Order {
		t.Errorf("reordering one sibling rewrote another: %+v", stored["Hotdogs"])
	}

	// Naming no parent flattens it out of the subtree.
	body = `{"targetListId":"` + list.Id.Hex() + `","order":9999}`
	if w := do(h, http.MethodPut, movePath(eventId, list.Id, first.Id), body, cookie); w.Code != http.StatusOK {
		t.Fatalf("flatten: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	stored = itemsByText(readEventLists(t, eventId)[0].Items)
	if stored["Hotdogs"].ParentId != nil {
		t.Errorf("flatten left the parentId in place: %+v", stored["Hotdogs"])
	}
	if stored["Hotdogs"].Order != 9999 {
		t.Errorf("flatten did not set the order: %+v", stored["Hotdogs"])
	}
}

// THE cross-list proof: one update carrying a $pull off the source list and a
// $push onto the destination. Mongo has to accept both paths in a single update
// for the subtree to move atomically — if it ever rejects the combination this
// is the test that says so, and F18's drag depends on it.
//
// The moved root is flattened and repositioned; everything under it keeps the
// parentId and order it had, because those are relative to a group travelling
// with it.
func TestLists_MoveAcrossListsCarriesTheSubtree(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-move-cross@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	src := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)
	dst := createListFor(t, h, eventId, cookie, `{"name":"Drinks","kind":"text"}`)

	stay := addItemFor(t, h, eventId, src.Id, cookie, `{"text":"Salad"}`)
	root := addItemFor(t, h, eventId, src.Id, cookie, `{"text":"Mains"}`)
	child := addItemFor(t, h, eventId, src.Id, cookie,
		`{"text":"Hotdogs","parentId":"`+root.Id.Hex()+`"}`)
	grandchild := addItemFor(t, h, eventId, src.Id, cookie,
		`{"text":"Mustard","parentId":"`+child.Id.Hex()+`"}`)
	// Something already on the destination, to prove the $push appends rather
	// than replaces.
	existing := addItemFor(t, h, eventId, dst.Id, cookie, `{"text":"Beer"}`)

	body := `{"targetListId":"` + dst.Id.Hex() + `","order":512}`
	if w := do(h, http.MethodPut, movePath(eventId, src.Id, root.Id), body, cookie); w.Code != http.StatusOK {
		t.Fatalf("cross-list move: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	lists := readEventLists(t, eventId)
	after := itemsByText(listByName(t, lists, "Menu").Items)
	moved := itemsByText(listByName(t, lists, "Drinks").Items)

	if len(after) != 1 || after["Salad"].Id != stay.Id {
		t.Errorf("source list should hold only Salad, has: %+v", after)
	}
	if len(moved) != 4 {
		t.Fatalf("destination should hold 4 entries, has %d: %+v", len(moved), moved)
	}
	if moved["Beer"].Id != existing.Id {
		t.Errorf("the $push disturbed what was already there: %+v", moved)
	}
	if moved["Mains"].ParentId != nil {
		t.Errorf("the moved root did not flatten: %+v", moved["Mains"])
	}
	if moved["Mains"].Order != 512 {
		t.Errorf("the moved root's order = %v, want 512", moved["Mains"].Order)
	}
	if moved["Hotdogs"].ParentId == nil || *moved["Hotdogs"].ParentId != root.Id {
		t.Errorf("a descendant lost its parent: %+v", moved["Hotdogs"])
	}
	if moved["Mustard"].ParentId == nil || *moved["Mustard"].ParentId != child.Id {
		t.Errorf("a grandchild lost its parent: %+v", moved["Mustard"])
	}
	if moved["Hotdogs"].Order != child.Order || moved["Mustard"].Order != grandchild.Order {
		t.Errorf("descendants' orders were rewritten: %+v", moved)
	}
}

// Moving is delete-and-re-add, so it takes the delete right: a member may move
// anyone's entry, an unrelated guest may move nobody's but their own.
func TestLists_MovePermissions(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-move-owner@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "lists-move-guest@example.test")
	otherGuestId := insertTestUser(t, models.RoleGuest, "lists-move-guest2@example.test")
	memberId := insertTestUser(t, models.RoleMember, "lists-move-member@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()

	ownerCookie := loginAs(t, h, ownerId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())
	otherGuestCookie := loginAs(t, h, otherGuestId.Hex())
	memberCookie := loginAs(t, h, memberId.Hex())

	list := createListFor(t, h, eventId, ownerCookie, `{"name":"Menu","kind":"text"}`)
	item := addItemFor(t, h, eventId, list.Id, guestCookie, `{"text":"Hotdogs"}`)
	path := movePath(eventId, list.Id, item.Id)
	body := `{"targetListId":"` + list.Id.Hex() + `","order":2048}`

	if w := do(h, http.MethodPut, path, body, otherGuestCookie); w.Code != http.StatusForbidden {
		t.Fatalf("unrelated guest move: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
	if w := do(h, http.MethodPut, path, body, guestCookie); w.Code != http.StatusOK {
		t.Fatalf("author moves their own: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if w := do(h, http.MethodPut, path, body, memberCookie); w.Code != http.StatusOK {
		t.Fatalf("member moves another's: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

// The refusals: a parent that isn't the item's own, a destination that is full,
// and things that are not there at all.
func TestLists_MoveRejections(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-move-reject@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)

	parent := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Mains"}`)
	stray := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Salad"}`)
	child := addItemFor(t, h, eventId, list.Id, cookie,
		`{"text":"Hotdogs","parentId":"`+parent.Id.Hex()+`"}`)

	cases := []struct {
		name   string
		itemId primitive.ObjectID
		listId primitive.ObjectID
		body   string
		want   int
		errStr string
	}{
		{
			"a parent that is not the item's own",
			child.Id, list.Id,
			`{"targetListId":"` + list.Id.Hex() + `","order":1,"parentId":"` + stray.Id.Hex() + `"}`,
			http.StatusBadRequest, errInvalidMove,
		},
		{
			"a top-level item may not acquire a parent",
			stray.Id, list.Id,
			`{"targetListId":"` + list.Id.Hex() + `","order":1,"parentId":"` + parent.Id.Hex() + `"}`,
			http.StatusBadRequest, errInvalidMove,
		},
		{
			"an item that is not there",
			primitive.NewObjectID(), list.Id,
			`{"targetListId":"` + list.Id.Hex() + `","order":1}`,
			http.StatusNotFound, errListItemNotFound,
		},
		{
			"a destination list that is not there",
			stray.Id, list.Id,
			`{"targetListId":"` + primitive.NewObjectID().Hex() + `","order":1}`,
			http.StatusNotFound, errListNotFound,
		},
		{
			"a source list that is not there",
			stray.Id, primitive.NewObjectID(),
			`{"targetListId":"` + list.Id.Hex() + `","order":1}`,
			http.StatusNotFound, errListNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(h, http.MethodPut, movePath(eventId, tc.listId, tc.itemId), tc.body, cookie)
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d (body: %s)", w.Code, tc.want, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.errStr) {
				t.Errorf("error = %s, want %s", w.Body.String(), tc.errStr)
			}
		})
	}
}

// The destination's cap counts the whole incoming subtree, so a move cannot
// smuggle a list past maxItemsPerList the way a single add never could.
func TestLists_MoveIntoAFullListIsRefused(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-move-full@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	src := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)
	dst := createListFor(t, h, eventId, cookie, `{"name":"Drinks","kind":"text"}`)

	item := addItemFor(t, h, eventId, src.Id, cookie, `{"text":"Mains"}`)

	// Fill the destination to the cap by writing straight to Mongo — going
	// through the endpoint maxItemsPerList times would dominate the test's
	// runtime for no extra coverage.
	full := make([]models.EventListItem, maxItemsPerList)
	for i := range full {
		full[i] = models.EventListItem{Id: primitive.NewObjectID(), Text: "filler", UserId: ownerId}
	}
	_, err := db.EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$set": bson.M{"lists.$[l].items": full}},
		options.Update().SetArrayFilters(options.ArrayFilters{
			Filters: []interface{}{bson.M{"l._id": dst.Id}},
		}),
	)
	if err != nil {
		t.Fatalf("seed a full list: %v", err)
	}

	body := `{"targetListId":"` + dst.Id.Hex() + `","order":1}`
	w := do(h, http.MethodPut, movePath(eventId, src.Id, item.Id), body, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("move into a full list: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), errListFull) {
		t.Errorf("error = %s, want %s", w.Body.String(), errListFull)
	}
	if items := listByName(t, readEventLists(t, eventId), "Menu").Items; len(items) != 1 {
		t.Errorf("a refused move still left the source list: %+v", items)
	}
}

// An order of 0 is a real position — a drop at the top of a migrated list —
// so the binding must accept it rather than treating it as a missing field.
func TestLists_MoveAcceptsAZeroOrder(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "lists-move-zero@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := listsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())
	list := createListFor(t, h, eventId, cookie, `{"name":"Menu","kind":"text"}`)
	item := addItemFor(t, h, eventId, list.Id, cookie, `{"text":"Mains"}`)

	body := `{"targetListId":"` + list.Id.Hex() + `","order":0}`
	if w := do(h, http.MethodPut, movePath(eventId, list.Id, item.Id), body, cookie); w.Code != http.StatusOK {
		t.Fatalf("zero order: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if stored := readEventLists(t, eventId)[0].Items[0]; stored.Order != 0 {
		t.Errorf("stored order = %v, want 0", stored.Order)
	}
}
