package routes

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

// Who may hand a checklist entry to somebody (N1). The interesting line is the
// guest one: a guest may add entries and tick boxes, so "can write to the list"
// is not the same right as "can divide up the work".
func TestCanAssign(t *testing.T) {
	owner := primitive.NewObjectID()
	other := primitive.NewObjectID()
	event := ownedEvent(owner)

	cases := []struct {
		name string
		user *models.User
		want bool
	}{
		{"owner", userWithRole(owner, models.RoleMember), true},
		{"plain member on someone else's event", userWithRole(other, models.RoleMember), true},
		{"admin", userWithRole(other, models.RoleAdmin), true},
		{"superAdmin", userWithRole(other, models.RoleSuperAdmin), true},
		{"guest", userWithRole(other, models.RoleGuest), false},
		{"anonymous", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newListViewer(tc.user, event).canAssign(); got != tc.want {
				t.Errorf("canAssign = %v, want %v", got, tc.want)
			}
		})
	}
}

// assignedList is the whole "Assigned" feature on the read side, so this covers
// what it keeps, what it drops and what it rewrites.
func TestAssignedList(t *testing.T) {
	me := primitive.NewObjectID()
	someoneElse := primitive.NewObjectID()

	parentId := primitive.NewObjectID()
	menuId := primitive.NewObjectID()
	tasksId := primitive.NewObjectID()

	assignedTo := func(id primitive.ObjectID) *primitive.ObjectID { return &id }

	event := &models.Event{
		Id: primitive.NewObjectID(),
		Lists: []models.EventList{
			{
				Id:   menuId,
				Name: "Menu",
				Kind: models.ListKindChecklist,
				Items: []models.EventListItem{
					{Id: parentId, Text: "Drinks", Order: 1024},
					// A SUB-entry, assigned to me while its parent is not.
					{
						Id: primitive.NewObjectID(), Text: "Bring the port",
						ParentId: &parentId, Order: 2048,
						AssigneeId: assignedTo(me), AssigneeName: "Me",
					},
					{
						Id: primitive.NewObjectID(), Text: "Bring the cheese",
						Order: 3072, AssigneeId: assignedTo(someoneElse), AssigneeName: "Bart",
					},
					{Id: primitive.NewObjectID(), Text: "Nobody's yet", Order: 4096},
				},
			},
			{
				Id:   tasksId,
				Name: "Tasks",
				Kind: models.ListKindChecklist,
				Items: []models.EventListItem{
					{
						Id: primitive.NewObjectID(), Text: "Book the room",
						Order: 1024, AssigneeId: assignedTo(me), AssigneeName: "Me",
					},
				},
			},
			{
				// A non-checklist list is skipped entirely: there is nothing to
				// finish on it, so the assignee field is meaningless there even if
				// a hand-edited document carried one.
				Id: primitive.NewObjectID(), Name: "Places", Kind: models.ListKindLocation,
				Items: []models.EventListItem{
					{
						Id: primitive.NewObjectID(), Text: "The Eagle",
						Order: 1024, AssigneeId: assignedTo(me), AssigneeName: "Me",
					},
				},
			},
		},
	}

	list := assignedList(event, me)
	if list == nil {
		t.Fatal("assignedList = nil, want a list")
	}
	if !list.Virtual {
		t.Error("the derived list must be marked virtual — it is what makes it read-only on the client")
	}
	if list.Id != primitive.NilObjectID {
		t.Errorf("derived list id = %v, want the nil id", list.Id)
	}
	if list.Kind != models.ListKindChecklist {
		t.Errorf("derived list kind = %q, want a checklist", list.Kind)
	}
	if list.Name != assignedListName {
		t.Errorf("derived list name = %q, want %q", list.Name, assignedListName)
	}

	if len(list.Items) != 2 {
		t.Fatalf("got %d items, want 2 (mine on two checklists, nobody else's, nothing off a location list): %+v", len(list.Items), list.Items)
	}

	for i, item := range list.Items {
		// FLAT: the sub-entry's parent is not assigned to me and is not in this
		// list, so carrying the pointer would render an orphan.
		if item.ParentId != nil {
			t.Errorf("item %d kept its parentId — the derived list must be flat", i)
		}
		// Restamped: the source orders came from different sibling groups.
		if item.Order != float64(i) {
			t.Errorf("item %d order = %v, want %d", i, item.Order, i)
		}
		if item.SourceListId == nil {
			t.Fatalf("item %d has no sourceListId — the client cannot write a tick back without it", i)
		}
	}

	if *list.Items[0].SourceListId != menuId || list.Items[0].SourceListName != "Menu" {
		t.Errorf("first item points at the wrong source list: %+v", list.Items[0])
	}
	if *list.Items[1].SourceListId != tasksId || list.Items[1].SourceListName != "Tasks" {
		t.Errorf("second item points at the wrong source list: %+v", list.Items[1])
	}

	// Deriving must not disturb the event it read from — the items are copies.
	if event.Lists[0].Items[1].ParentId == nil {
		t.Error("deriving cleared the parentId on the REAL item")
	}
}

// Nothing assigned means no list at all, rather than an empty one: the tab count
// counts lists, and an empty pseudo-list would follow every member around
// forever.
func TestAssignedListOmittedWhenNothingIsAssigned(t *testing.T) {
	me := primitive.NewObjectID()
	event := &models.Event{
		Id: primitive.NewObjectID(),
		Lists: []models.EventList{{
			Id: primitive.NewObjectID(), Name: "Menu", Kind: models.ListKindChecklist,
			Items: []models.EventListItem{{Id: primitive.NewObjectID(), Text: "Unclaimed"}},
		}},
	}

	if got := assignedList(event, me); got != nil {
		t.Errorf("assignedList = %+v, want nil", got)
	}
	if got := assignedList(nil, me); got != nil {
		t.Errorf("assignedList(nil event) = %+v, want nil", got)
	}
	if got := assignedList(event, primitive.NilObjectID); got != nil {
		t.Errorf("assignedList(no user) = %+v, want nil", got)
	}
}
