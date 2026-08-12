// Mongo access for the shared lists on an event (F13).
//
// Every function here is a single targeted update, never a read-modify-write of
// the whole `lists` array. That is the difference from polls, and it is
// deliberate: polls rewrite `polls` wholesale from a value read earlier in the
// request (routes/polls.go), which loses one of two concurrent votes. Polls get
// away with it because a handful of people vote occasionally; a list says "add
// your dish" to the whole club at once. Positional array filters keep each
// append, edit, removal and move independent of what anyone else is doing to
// the same list in the same moment.
//
// Moving (F17) is the case that most wanted to become a read-modify-write, and
// does not: fractional ordering means repositioning an item writes that item
// alone, and even a cross-list move — which does have to carry the subtree it
// was read with — stays a single update combining a $pull with a $push.
//
// The update bodies are parameterised by (collection, document filter) because
// private lists (F19) store the identical `lists` array in their own collection
// under a (userId, eventId) key. Everything below is therefore shared with
// db/personal_lists.go, which is the point: two copies of positional-array
// surgery this fiddly would drift, and only one of them has the tests.
package db

import (
	"context"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sirtom/server/models"
)

// arrayFilterOptions builds the options for an update whose paths use the given
// positional identifiers.
func arrayFilterOptions(filters ...interface{}) *options.UpdateOptions {
	return options.Update().SetArrayFilters(options.ArrayFilters{Filters: filters})
}

// eventFilter selects one event document by id — what every exported function in
// this file passes to its shared implementation.
func eventFilter(eventId primitive.ObjectID) bson.M {
	return bson.M{"_id": eventId}
}

// --- Shared implementations, parameterised by collection + document filter ---

// insertList appends a new list to the document the filter selects.
func insertList(coll *mongo.Collection, filter bson.M, list models.EventList) error {
	_, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$push": bson.M{"lists": list}})
	return err
}

// renameList sets a list's name. Reports whether the list was found, so a caller
// can 404 rather than silently succeed.
func renameList(coll *mongo.Collection, filter bson.M, listId primitive.ObjectID, name string) (bool, error) {
	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$set": bson.M{"lists.$[l].name": name}},
		arrayFilterOptions(bson.M{"l._id": listId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// deleteList removes a list and everything on it.
func deleteList(coll *mongo.Collection, filter bson.M, listId primitive.ObjectID) (bool, error) {
	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$pull": bson.M{"lists": bson.M{"_id": listId}}})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// insertListItem appends an item to a list.
func insertListItem(coll *mongo.Collection, filter bson.M, listId primitive.ObjectID, item models.EventListItem) (bool, error) {
	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$push": bson.M{"lists.$[l].items": item}},
		arrayFilterOptions(bson.M{"l._id": listId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// updateListItemText rewrites one item's text, leaving its author and timestamp
// alone.
func updateListItemText(coll *mongo.Collection, filter bson.M, listId, itemId primitive.ObjectID, text string) (bool, error) {
	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$set": bson.M{"lists.$[l].items.$[i].text": text}},
		arrayFilterOptions(bson.M{"l._id": listId}, bson.M{"i._id": itemId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// deleteListItems removes a set of items from a list in one update.
//
// The set is how a cascade stays atomic: deleting an item deletes everything
// nested under it, and a single document update either applies in full or not
// at all, so no one can ever observe a half-removed subtree. Deleting a lone
// item is the same call with a set of one — one code path, no drift.
func deleteListItems(coll *mongo.Collection, filter bson.M, listId primitive.ObjectID, itemIds []primitive.ObjectID) (bool, error) {
	if len(itemIds) == 0 {
		return false, nil
	}
	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$pull": bson.M{"lists.$[l].items": bson.M{"_id": bson.M{"$in": itemIds}}}},
		arrayFilterOptions(bson.M{"l._id": listId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// setListItemFields $sets the given fields on one item, in one update. The keys
// are field names relative to the item, so callers never spell out the
// positional path.
//
// Reports whether the item was FOUND (MatchedCount), not whether the document
// changed: ticking a box that is already ticked modifies nothing, and that must
// read as success rather than as a missing item.
//
// A field map rather than a fixed argument list because the two callers write
// different sets: the shared lists record who ticked the box, private lists have
// nobody to record. Passing zero values for the absent fields would store an
// all-zeros ObjectID and an empty name — junk that reads back as data.
func setListItemFields(
	coll *mongo.Collection,
	filter bson.M,
	listId, itemId primitive.ObjectID,
	fields bson.M,
) (bool, error) {
	return setListItemsFields(coll, filter, listId, []primitive.ObjectID{itemId}, fields)
}

// setListItemsFields is the same write across SEVERAL items of one list, still
// as a single update — the `$in` in the array filter is what keeps it one.
//
// That matters more here than it looks: assigning a parent entry cascades to its
// whole subtree (N1), and doing that as a loop of single updates would leave a
// branch half-assigned whenever one of them failed, with no way for the client
// to tell. The same reason deleteListItems takes a slice.
//
// Reports MatchedCount on the DOCUMENT, so it says whether the event was there,
// not how many items were touched. Callers have already resolved the ids from
// the list they read, and an id that has since been removed is a no-op rather
// than an error — the same tolerance every other write here has.
func setListItemsFields(
	coll *mongo.Collection,
	filter bson.M,
	listId primitive.ObjectID,
	itemIds []primitive.ObjectID,
	fields bson.M,
) (bool, error) {
	if len(itemIds) == 0 {
		return false, nil
	}

	set := bson.M{}
	for name, value := range fields {
		set["lists.$[l].items.$[i]."+name] = value
	}

	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$set": set},
		arrayFilterOptions(
			bson.M{"l._id": listId},
			bson.M{"i._id": bson.M{"$in": itemIds}},
		),
	)
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

// ListItemAssignee is one item's assignment. A nil AssigneeId means unassigned,
// which is a state worth restoring as precisely as a name is.
type ListItemAssignee struct {
	ItemId     primitive.ObjectID
	AssigneeId *primitive.ObjectID
	Name       string
}

// restoreListItemAssignees writes a DIFFERENT assignee to each of several items,
// still in one update.
//
// setListItemsFields cannot do this: it fans one value across everything its
// `$in` matches. Restoring a branch (N2) needs "this one back to Bart, that one
// back to nobody", so each item gets its own array-filter identifier — i0, i1,
// … — and its own $set paths. arrayFilterOptions is variadic, so this is still a
// single UpdateOne, which matters for the same reason the cascade is one write:
// a half-restored branch is precisely the state undo exists to avoid.
//
// The identifiers and the $set keys are built in the SAME loop on purpose. Mongo
// rejects the whole update if an identifier is declared in arrayFilters and used
// by no path, so the two must not be able to drift apart.
func restoreListItemAssignees(
	coll *mongo.Collection,
	filter bson.M,
	listId primitive.ObjectID,
	assignees []ListItemAssignee,
) (bool, error) {
	if len(assignees) == 0 {
		return false, nil
	}

	set := bson.M{}
	filters := []interface{}{bson.M{"l._id": listId}}
	for i, assignee := range assignees {
		// Identifiers must be alphanumeric and start with a lowercase letter.
		id := "i" + strconv.Itoa(i)
		set["lists.$[l].items.$["+id+"].assigneeId"] = assignee.AssigneeId
		set["lists.$[l].items.$["+id+"].assigneeName"] = assignee.Name
		filters = append(filters, bson.M{id + "._id": assignee.ItemId})
	}

	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$set": set},
		arrayFilterOptions(filters...),
	)
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

// setListItemOrder repositions an item among its existing siblings (F17).
//
// The narrowest of the three move shapes: nothing but the moved item's order
// changes, so a reorder cannot disturb a neighbour someone is editing in the
// same moment. Reports MatchedCount, like setListItemChecked — dropping an item
// back exactly where it started modifies nothing and is still a success.
func setListItemOrder(coll *mongo.Collection, filter bson.M, listId, itemId primitive.ObjectID, order float64) (bool, error) {
	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{"$set": bson.M{"lists.$[l].items.$[i].order": order}},
		arrayFilterOptions(bson.M{"l._id": listId}, bson.M{"i._id": itemId}),
	)
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

// flattenListItemToTopLevel drops an item out of its parent and places it at the
// given order, in one update.
//
// The $set and $unset are combined deliberately: an item that had lost its
// parent but not yet gained its position — or the reverse — would render in the
// wrong place for as long as the gap lasted. Its children are untouched and
// follow it, since they point at it and not at its old parent.
func flattenListItemToTopLevel(coll *mongo.Collection, filter bson.M, listId, itemId primitive.ObjectID, order float64) (bool, error) {
	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{
			"$set":   bson.M{"lists.$[l].items.$[i].order": order},
			"$unset": bson.M{"lists.$[l].items.$[i].parentId": ""},
		},
		arrayFilterOptions(bson.M{"l._id": listId}, bson.M{"i._id": itemId}),
	)
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

// moveListItems moves a subtree from one list to another within the same
// document, atomically (F17).
//
// The whole move is ONE update: a $pull removing the ids from the source list
// and a $push adding the caller-built structs to the destination, under two
// array filters. Both lists live in the same document, so there is no instant in
// which the subtree is on both lists or on neither — the same guarantee
// deleteListItems gives a cascade, extended across lists. The two paths are
// lexically distinct (`lists.$[src]` vs `lists.$[dst]`), which is what lets Mongo
// accept them in a single update rather than rejecting them as a conflict.
//
// The items are supplied by the caller rather than read here, because only the
// route knows which of them is the moved root — the one whose parentId and
// order change — and which are descendants riding along unmodified.
func moveListItems(
	coll *mongo.Collection,
	filter bson.M,
	srcListId, dstListId primitive.ObjectID,
	itemIds []primitive.ObjectID,
	items []models.EventListItem,
) (bool, error) {
	if len(itemIds) == 0 || len(items) == 0 {
		return false, nil
	}
	res, err := coll.UpdateOne(context.Background(), filter,
		bson.M{
			"$pull": bson.M{"lists.$[src].items": bson.M{"_id": bson.M{"$in": itemIds}}},
			"$push": bson.M{"lists.$[dst].items": bson.M{"$each": items}},
		},
		arrayFilterOptions(bson.M{"src._id": srcListId}, bson.M{"dst._id": dstListId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// --- The event-scoped API (F13) ---

// InsertEventList appends a new list to an event.
func InsertEventList(eventId primitive.ObjectID, list models.EventList) error {
	return insertList(EventsCollection, eventFilter(eventId), list)
}

// RenameEventList sets a list's name.
func RenameEventList(eventId, listId primitive.ObjectID, name string) (bool, error) {
	return renameList(EventsCollection, eventFilter(eventId), listId, name)
}

// DeleteEventList removes a list and everything on it.
func DeleteEventList(eventId, listId primitive.ObjectID) (bool, error) {
	return deleteList(EventsCollection, eventFilter(eventId), listId)
}

// InsertEventListItem appends an item to a list.
func InsertEventListItem(eventId, listId primitive.ObjectID, item models.EventListItem) (bool, error) {
	return insertListItem(EventsCollection, eventFilter(eventId), listId, item)
}

// UpdateEventListItemText rewrites one item's text.
func UpdateEventListItemText(eventId, listId, itemId primitive.ObjectID, text string) (bool, error) {
	return updateListItemText(EventsCollection, eventFilter(eventId), listId, itemId, text)
}

// DeleteEventListItems removes a set of items from a list in one update.
func DeleteEventListItems(eventId, listId primitive.ObjectID, itemIds []primitive.ObjectID) (bool, error) {
	return deleteListItems(EventsCollection, eventFilter(eventId), listId, itemIds)
}

// SetEventListItemChecked stamps a checklist item's state, who changed it, and
// when. All four are always written together, in both directions — so
// Checked=false alongside a CheckedByName renders "Unchecked by Bart".
func SetEventListItemChecked(
	eventId, listId, itemId primitive.ObjectID,
	checked bool,
	byUserId primitive.ObjectID,
	byName string,
	at primitive.DateTime,
) (bool, error) {
	return setListItemFields(EventsCollection, eventFilter(eventId), listId, itemId, bson.M{
		"checked":       checked,
		"checkedBy":     byUserId,
		"checkedByName": byName,
		"checkedAt":     at,
	})
}

// SetEventListItemAssignee records who a set of checklist items is for (N1), or
// clears them when assigneeId is nil.
//
// Takes a SLICE because assigning an entry assigns its sub-entries too: a parent
// is a heading over work, so handing over "Sleeping" hands over the tent and the
// cots with it. The caller resolves the subtree (collectDescendantIds, which
// already includes the root); this writes the whole branch in ONE update, so it
// can never land half-applied.
//
// Both fields are written every time, in both directions, for the same reason
// the checked quartet is: an unassign that only cleared the id would leave a
// name behind, and the name is what renders. Clearing $sets BSON null rather
// than $unset-ing, which decodes straight back into the pointer as nil — so this
// stays one write shape, with no second one to keep correct.
//
// Reports MatchedCount, like every sibling here: re-assigning to the member who
// already holds it modifies nothing and is still a success.
func SetEventListItemAssignee(
	eventId, listId primitive.ObjectID,
	itemIds []primitive.ObjectID,
	assigneeId *primitive.ObjectID,
	name string,
) (bool, error) {
	return setListItemsFields(EventsCollection, eventFilter(eventId), listId, itemIds, bson.M{
		"assigneeId":   assigneeId,
		"assigneeName": name,
	})
}

// RestoreEventListItemAssignees puts a branch's assignments back exactly as they
// were (N2) — each item to its own prior holder, or to nobody.
func RestoreEventListItemAssignees(
	eventId, listId primitive.ObjectID,
	assignees []ListItemAssignee,
) (bool, error) {
	return restoreListItemAssignees(EventsCollection, eventFilter(eventId), listId, assignees)
}

// SetEventListItemOrder repositions an item among its existing siblings.
func SetEventListItemOrder(eventId, listId, itemId primitive.ObjectID, order float64) (bool, error) {
	return setListItemOrder(EventsCollection, eventFilter(eventId), listId, itemId, order)
}

// FlattenEventListItemToTopLevel drops an item out of its parent and places it
// at the given order.
func FlattenEventListItemToTopLevel(eventId, listId, itemId primitive.ObjectID, order float64) (bool, error) {
	return flattenListItemToTopLevel(EventsCollection, eventFilter(eventId), listId, itemId, order)
}

// MoveEventListItems moves a subtree from one list to another on the same event.
func MoveEventListItems(
	eventId, srcListId, dstListId primitive.ObjectID,
	itemIds []primitive.ObjectID,
	items []models.EventListItem,
) (bool, error) {
	return moveListItems(EventsCollection, eventFilter(eventId), srcListId, dstListId, itemIds, items)
}
