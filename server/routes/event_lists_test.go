package routes

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

// ownedEvent / ownerlessEvent build the two shapes canManageLists distinguishes.
func ownedEvent(ownerId primitive.ObjectID) *models.Event {
	return &models.Event{Id: primitive.NewObjectID(), OwnerId: ownerId}
}

func ownerlessEvent() *models.Event {
	return &models.Event{Id: primitive.NewObjectID()}
}

func userWithRole(id primitive.ObjectID, role models.Role) *models.User {
	return &models.User{Id: id, Role: role}
}

// The whole point of the feature's permission split, in one table: who may
// create/rename/delete the lists themselves.
func TestCanManageLists(t *testing.T) {
	owner := primitive.NewObjectID()
	other := primitive.NewObjectID()

	cases := []struct {
		name  string
		user  *models.User
		event *models.Event
		want  bool
	}{
		{"owner manages their own event", userWithRole(owner, models.RoleMember), ownedEvent(owner), true},
		{"admin manages someone else's event", userWithRole(other, models.RoleAdmin), ownedEvent(owner), true},
		{"superAdmin manages someone else's event", userWithRole(other, models.RoleSuperAdmin), ownedEvent(owner), true},
		{"plain member cannot manage someone else's event", userWithRole(other, models.RoleMember), ownedEvent(owner), false},
		{"guest cannot manage someone else's event", userWithRole(other, models.RoleGuest), ownedEvent(owner), false},
		// Legacy ownerless events fall back to requireEventManager's rule.
		{"member manages a legacy ownerless event", userWithRole(other, models.RoleMember), ownerlessEvent(), true},
		{"guest cannot manage a legacy ownerless event", userWithRole(other, models.RoleGuest), ownerlessEvent(), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewer := newListViewer(tc.user, tc.event)
			if got := viewer.canManageLists(tc.event); got != tc.want {
				t.Errorf("canManageLists = %v, want %v", got, tc.want)
			}
		})
	}
}

// A signed-out viewer manages nothing, whatever the event looks like.
func TestCanManageListsRejectsAnonymous(t *testing.T) {
	viewer := newListViewer(nil, ownerlessEvent())
	if viewer.canManageLists(ownerlessEvent()) {
		t.Error("an anonymous viewer must not manage lists")
	}
}

// Editing is own-only at every role; deleting has the member+ override. These
// two rules differ deliberately, so they're asserted against the same fixtures.
func TestCanEditAndDeleteItem(t *testing.T) {
	me := primitive.NewObjectID()
	someoneElse := primitive.NewObjectID()
	event := ownerlessEvent()

	mine := models.EventListItem{Id: primitive.NewObjectID(), UserId: me}
	theirs := models.EventListItem{Id: primitive.NewObjectID(), UserId: someoneElse}

	cases := []struct {
		name       string
		role       models.Role
		item       models.EventListItem
		wantEdit   bool
		wantDelete bool
	}{
		{"guest, own item", models.RoleGuest, mine, true, true},
		{"guest, another's item", models.RoleGuest, theirs, false, false},
		{"member, own item", models.RoleMember, mine, true, true},
		{"member, another's item", models.RoleMember, theirs, false, true},
		{"admin, another's item", models.RoleAdmin, theirs, false, true},
		{"superAdmin, another's item", models.RoleSuperAdmin, theirs, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewer := newListViewer(userWithRole(me, tc.role), event)
			if got := viewer.canEditItem(tc.item); got != tc.wantEdit {
				t.Errorf("canEditItem = %v, want %v", got, tc.wantEdit)
			}
			if got := viewer.canDeleteItem(tc.item); got != tc.wantDelete {
				t.Errorf("canDeleteItem = %v, want %v", got, tc.wantDelete)
			}
		})
	}
}

// The event owner's power is over the lists, not over what people wrote in
// them: owning the event does not let them rewrite an entry as someone else.
func TestEventOwnerCannotEditAnotherPersonsItem(t *testing.T) {
	owner := primitive.NewObjectID()
	event := ownedEvent(owner)
	viewer := newListViewer(userWithRole(owner, models.RoleMember), event)

	theirs := models.EventListItem{UserId: primitive.NewObjectID()}
	if viewer.canEditItem(theirs) {
		t.Error("the event owner must not be able to edit another person's item")
	}
	// They may still remove it — as a member, not as the owner.
	if !viewer.canDeleteItem(theirs) {
		t.Error("a member-role event owner should be able to remove another person's item")
	}
}

func TestSanitizeListName(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   string
		wantOk bool
	}{
		{"trims", "  Menu  ", "Menu", true},
		{"empty", "", "", false},
		{"whitespace only", "   \t\n ", "", false},
		{"keeps inner spacing", "Bars to Visit", "Bars to Visit", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := sanitizeListName(tc.in)
			if got != tc.want || ok != tc.wantOk {
				t.Errorf("sanitizeListName(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOk)
			}
		})
	}
}

// The caps bound runes, not bytes — an address full of accents must not be cut
// mid-character (see truncateRunes).
func TestSanitizeListTruncatesByRunes(t *testing.T) {
	long := ""
	for i := 0; i < maxListNameLength+50; i++ {
		long += "é"
	}
	got, ok := sanitizeListName(long)
	if !ok {
		t.Fatal("a long name should still be usable after truncation")
	}
	if runes := []rune(got); len(runes) != maxListNameLength {
		t.Errorf("name truncated to %d runes, want %d", len(runes), maxListNameLength)
	}

	longItem := ""
	for i := 0; i < maxListItemLength+50; i++ {
		longItem += "ü"
	}
	gotItem, itemOk := sanitizeListItemText(longItem)
	if !itemOk {
		t.Fatal("a long item should still be usable after truncation")
	}
	if runes := []rune(gotItem); len(runes) != maxListItemLength {
		t.Errorf("item truncated to %d runes, want %d", len(runes), maxListItemLength)
	}
}

// An unknown kind is rejected rather than defaulted: a typo that silently
// became a text list would render addresses as plain strings with no maps link
// and no way to tell why.
func TestValidListKind(t *testing.T) {
	for _, kind := range []string{models.ListKindText, models.ListKindLocation, models.ListKindChecklist} {
		if !validListKind(kind) {
			t.Errorf("validListKind(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{"", "Text", "LOCATION", "locations", "places", "todo", "Checklist", "check"} {
		if validListKind(kind) {
			t.Errorf("validListKind(%q) = true, want false", kind)
		}
	}
}

func TestFindEventListAndItem(t *testing.T) {
	itemId := primitive.NewObjectID()
	listId := primitive.NewObjectID()
	event := &models.Event{Lists: []models.EventList{
		{Id: primitive.NewObjectID(), Name: "Other"},
		{Id: listId, Name: "Menu", Items: []models.EventListItem{{Id: itemId, Text: "Hotdogs"}}},
	}}

	list, found := findEventList(event, listId.Hex())
	if !found || list.Name != "Menu" {
		t.Fatalf("findEventList did not return the Menu list: %+v, found=%v", list, found)
	}
	if _, found := findEventList(event, primitive.NewObjectID().Hex()); found {
		t.Error("findEventList found a list that isn't there")
	}
	if _, found := findEventList(event, "not-an-object-id"); found {
		t.Error("findEventList matched a malformed id")
	}

	item, itemFound := findListItem(list, itemId.Hex())
	if !itemFound || item.Text != "Hotdogs" {
		t.Fatalf("findListItem did not return the item: %+v, found=%v", item, itemFound)
	}
	if _, found := findListItem(list, primitive.NewObjectID().Hex()); found {
		t.Error("findListItem found an item that isn't there")
	}
}

// findEventList returns a pointer into the event so a handler can pass the real
// list's id to the db layer; if it ever returned a copy the callers would still
// compile and silently write to the wrong list.
func TestFindEventListReturnsAPointerIntoTheEvent(t *testing.T) {
	listId := primitive.NewObjectID()
	event := &models.Event{Lists: []models.EventList{{Id: listId, Name: "Menu"}}}

	list, _ := findEventList(event, listId.Hex())
	list.Name = "Renamed"
	if event.Lists[0].Name != "Renamed" {
		t.Error("findEventList returned a copy, not a pointer into the event")
	}
}

// nestedItem builds one item, optionally under a parent.
func nestedItem(id primitive.ObjectID, parent *primitive.ObjectID) models.EventListItem {
	return models.EventListItem{Id: id, ParentId: parent}
}

// The depth walk, including the two shapes that aren't a plain chain: an item
// whose parent has been deleted, and a cycle no handler can create but a
// hand-edited document could.
func TestListItemDepth(t *testing.T) {
	root := primitive.NewObjectID()
	child := primitive.NewObjectID()
	grandchild := primitive.NewObjectID()
	orphan := primitive.NewObjectID()
	missing := primitive.NewObjectID()

	list := &models.EventList{Items: []models.EventListItem{
		nestedItem(root, nil),
		nestedItem(child, &root),
		nestedItem(grandchild, &child),
		// Its parent is not on the list — deleted out from under it.
		nestedItem(orphan, &missing),
	}}

	cases := []struct {
		name string
		id   primitive.ObjectID
		want int
	}{
		{"top-level item", root, 0},
		{"child", child, 1},
		{"grandchild", grandchild, 2},
		{"orphan counts as top-level", orphan, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item, found := findListItem(list, tc.id.Hex())
			if !found {
				t.Fatal("fixture item missing")
			}
			if got := listItemDepth(list, item); got != tc.want {
				t.Errorf("listItemDepth = %d, want %d", got, tc.want)
			}
		})
	}
}

// A cycle must terminate rather than hang the request — the walk is bounded by
// the item count, so it reports that count as the depth. The exact number is
// meaningless; what matters is that it comes back, and comes back deep enough
// that adding a child underneath is refused. A cycle needs at least two items,
// so the bound is always at least maxListItemDepth-1.
func TestListItemDepthTerminatesOnACycle(t *testing.T) {
	a := primitive.NewObjectID()
	b := primitive.NewObjectID()
	list := &models.EventList{Items: []models.EventListItem{
		nestedItem(a, &b),
		nestedItem(b, &a),
	}}

	item, _ := findListItem(list, a.Hex())
	if got := listItemDepth(list, item); got+1 < maxListItemDepth {
		t.Errorf("listItemDepth on a cycle = %d, want deep enough that adding a child is refused", got)
	}
}

// The id set one $pull removes: the item plus its subtree, and nothing beside
// it. Getting this wrong either orphans grandchildren or deletes a sibling.
func TestCollectDescendantIds(t *testing.T) {
	root := primitive.NewObjectID()
	child := primitive.NewObjectID()
	grandchild := primitive.NewObjectID()
	sibling := primitive.NewObjectID()
	otherRoot := primitive.NewObjectID()
	otherChild := primitive.NewObjectID()

	list := &models.EventList{Items: []models.EventListItem{
		nestedItem(root, nil),
		nestedItem(child, &root),
		nestedItem(grandchild, &child),
		nestedItem(sibling, &root),
		nestedItem(otherRoot, nil),
		nestedItem(otherChild, &otherRoot),
	}}

	cases := []struct {
		name string
		from primitive.ObjectID
		want []primitive.ObjectID
	}{
		{"a leaf is only itself", grandchild, []primitive.ObjectID{grandchild}},
		{"a mid-level item takes its grandchild", child, []primitive.ObjectID{child, grandchild}},
		{"a root takes the whole subtree", root, []primitive.ObjectID{root, child, grandchild, sibling}},
		{"an unrelated branch is untouched", otherRoot, []primitive.ObjectID{otherRoot, otherChild}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := collectDescendantIds(list, tc.from)
			if len(got) != len(tc.want) {
				t.Fatalf("collectDescendantIds = %v (%d ids), want %d", got, len(got), len(tc.want))
			}
			gotSet := make(map[primitive.ObjectID]bool, len(got))
			for _, id := range got {
				if gotSet[id] {
					t.Errorf("id %s appears twice", id.Hex())
				}
				gotSet[id] = true
			}
			for _, id := range tc.want {
				if !gotSet[id] {
					t.Errorf("missing %s from %v", id.Hex(), got)
				}
			}
		})
	}
}

// Same malformed shape as the depth test: a cycle inside the subtree must not
// spin forever building an ever-growing id set.
func TestCollectDescendantIdsTerminatesOnACycle(t *testing.T) {
	root := primitive.NewObjectID()
	a := primitive.NewObjectID()
	b := primitive.NewObjectID()
	list := &models.EventList{Items: []models.EventListItem{
		nestedItem(root, nil),
		nestedItem(a, &root),
		// a and b point at each other as well as hanging off the root.
		nestedItem(b, &a),
	}}
	list.Items[1].ParentId = &b

	done := make(chan []primitive.ObjectID, 1)
	go func() { done <- collectDescendantIds(list, root) }()
	select {
	case got := <-done:
		for _, id := range got {
			if id == root {
				continue
			}
			if id != a && id != b {
				t.Errorf("unexpected id %s", id.Hex())
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("collectDescendantIds did not terminate on a cycle")
	}
}

// orderedItems builds a list whose items carry the given orders, ids aside.
func orderedItems(orders ...float64) *models.EventList {
	items := make([]models.EventListItem, len(orders))
	for i, order := range orders {
		items[i] = models.EventListItem{Id: primitive.NewObjectID(), Order: order}
	}
	return &models.EventList{Items: items}
}

// The append anchor. The zero cases are the ones that matter: an empty list and
// a list written before ordering existed both have to produce a first step of
// listItemOrderStep, and a list dragged below zero still has to append above its
// own last entry rather than leaping back to the step.
func TestMaxOrderInList(t *testing.T) {
	cases := []struct {
		name string
		list *models.EventList
		want float64
	}{
		{"empty list", &models.EventList{}, 0},
		{"legacy items all decode to zero", orderedItems(0, 0, 0), 0},
		{"ordered items", orderedItems(1024, 2048, 3072), 3072},
		{"highest need not be last", orderedItems(3072, 1024, 2048), 3072},
		{"mixed legacy and ordered", orderedItems(0, 0, 1024), 1024},
		{"all negative", orderedItems(-3072, -2048), -2048},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maxOrderInList(tc.list); got != tc.want {
				t.Errorf("maxOrderInList = %v, want %v", got, tc.want)
			}
		})
	}
}

// An append must land strictly above every sibling, which is the whole contract
// addEventListItem relies on. Checked across the awkward shapes rather than
// restating the arithmetic.
func TestMaxOrderInListLeavesRoomToAppend(t *testing.T) {
	for _, list := range []*models.EventList{
		orderedItems(0, 0, 0),
		orderedItems(1024, 2048),
		orderedItems(-3072, -2048),
		orderedItems(1024.5, 1024.75),
	} {
		next := maxOrderInList(list) + listItemOrderStep
		for _, item := range list.Items {
			if next <= item.Order {
				t.Errorf("append order %v does not exceed existing %v", next, item.Order)
			}
		}
	}
}
