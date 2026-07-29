package routes

import (
	"testing"

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
	for _, kind := range []string{models.ListKindText, models.ListKindLocation} {
		if !validListKind(kind) {
			t.Errorf("validListKind(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{"", "Text", "LOCATION", "locations", "places", "todo"} {
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
