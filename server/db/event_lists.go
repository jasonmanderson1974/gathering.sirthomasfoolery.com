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
package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sirtom/server/models"
)

// arrayFilterOptions builds the options for an update whose paths use the given
// positional identifiers.
func arrayFilterOptions(filters ...interface{}) *options.UpdateOptions {
	return options.Update().SetArrayFilters(options.ArrayFilters{Filters: filters})
}

// InsertEventList appends a new list to an event.
func InsertEventList(eventId primitive.ObjectID, list models.EventList) error {
	_, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$push": bson.M{"lists": list}})
	return err
}

// RenameEventList sets a list's name. Reports whether the list was found, so a
// caller can 404 rather than silently succeed.
func RenameEventList(eventId, listId primitive.ObjectID, name string) (bool, error) {
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$set": bson.M{"lists.$[l].name": name}},
		arrayFilterOptions(bson.M{"l._id": listId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// DeleteEventList removes a list and everything on it.
func DeleteEventList(eventId, listId primitive.ObjectID) (bool, error) {
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$pull": bson.M{"lists": bson.M{"_id": listId}}})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// InsertEventListItem appends an item to a list.
func InsertEventListItem(eventId, listId primitive.ObjectID, item models.EventListItem) (bool, error) {
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$push": bson.M{"lists.$[l].items": item}},
		arrayFilterOptions(bson.M{"l._id": listId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// UpdateEventListItemText rewrites one item's text, leaving its author and
// timestamp alone.
func UpdateEventListItemText(eventId, listId, itemId primitive.ObjectID, text string) (bool, error) {
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$set": bson.M{"lists.$[l].items.$[i].text": text}},
		arrayFilterOptions(bson.M{"l._id": listId}, bson.M{"i._id": itemId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// DeleteEventListItems removes a set of items from a list in one update.
//
// The set is how a cascade stays atomic: deleting an item deletes everything
// nested under it, and a single document update either applies in full or not
// at all, so no one can ever observe a half-removed subtree. Deleting a lone
// item is the same call with a set of one — one code path, no drift.
func DeleteEventListItems(eventId, listId primitive.ObjectID, itemIds []primitive.ObjectID) (bool, error) {
	if len(itemIds) == 0 {
		return false, nil
	}
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$pull": bson.M{"lists.$[l].items": bson.M{"_id": bson.M{"$in": itemIds}}}},
		arrayFilterOptions(bson.M{"l._id": listId}),
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

// SetEventListItemChecked stamps a checklist item's state together with who
// changed it and when, in one $set.
//
// Reports whether the item was FOUND (MatchedCount), not whether the document
// changed: ticking a box that is already ticked modifies nothing, and that must
// read as success rather than as a missing item.
func SetEventListItemChecked(
	eventId, listId, itemId primitive.ObjectID,
	checked bool,
	byUserId primitive.ObjectID,
	byName string,
	at primitive.DateTime,
) (bool, error) {
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$set": bson.M{
			"lists.$[l].items.$[i].checked":       checked,
			"lists.$[l].items.$[i].checkedBy":     byUserId,
			"lists.$[l].items.$[i].checkedByName": byName,
			"lists.$[l].items.$[i].checkedAt":     at,
		}},
		arrayFilterOptions(bson.M{"l._id": listId}, bson.M{"i._id": itemId}),
	)
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

// SetEventListItemOrder repositions an item among its existing siblings (F17).
//
// The narrowest of the three move shapes: nothing but the moved item's order
// changes, so a reorder cannot disturb a neighbour someone is editing in the
// same moment. Reports MatchedCount, like SetEventListItemChecked — dropping an
// item back exactly where it started modifies nothing and is still a success.
func SetEventListItemOrder(eventId, listId, itemId primitive.ObjectID, order float64) (bool, error) {
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$set": bson.M{"lists.$[l].items.$[i].order": order}},
		arrayFilterOptions(bson.M{"l._id": listId}, bson.M{"i._id": itemId}),
	)
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

// FlattenEventListItemToTopLevel drops an item out of its parent and places it
// at the given order, in one update.
//
// The $set and $unset are combined deliberately: an item that had lost its
// parent but not yet gained its position — or the reverse — would render in the
// wrong place for as long as the gap lasted. Its children are untouched and
// follow it, since they point at it and not at its old parent.
func FlattenEventListItemToTopLevel(eventId, listId, itemId primitive.ObjectID, order float64) (bool, error) {
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
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

// MoveEventListItems moves a subtree from one list to another on the same event,
// atomically (F17).
//
// The whole move is ONE update: a $pull removing the ids from the source list
// and a $push adding the caller-built structs to the destination, under two
// array filters. Both lists live in the same event document, so there is no
// instant in which the subtree is on both lists or on neither — the same
// guarantee DeleteEventListItems gives a cascade, extended across lists. The
// two paths are lexically distinct (`lists.$[src]` vs `lists.$[dst]`), which is
// what lets Mongo accept them in a single update rather than rejecting them as
// a conflict.
//
// The items are supplied by the caller rather than read here, because only the
// route knows which of them is the moved root — the one whose parentId and
// order change — and which are descendants riding along unmodified.
func MoveEventListItems(
	eventId, srcListId, dstListId primitive.ObjectID,
	itemIds []primitive.ObjectID,
	items []models.EventListItem,
) (bool, error) {
	if len(itemIds) == 0 || len(items) == 0 {
		return false, nil
	}
	res, err := EventsCollection.UpdateByID(context.Background(), eventId,
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
