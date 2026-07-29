// Mongo access for the shared lists on an event (F13).
//
// Every function here is a single targeted update, never a read-modify-write of
// the whole `lists` array. That is the difference from polls, and it is
// deliberate: polls rewrite `polls` wholesale from a value read earlier in the
// request (routes/polls.go), which loses one of two concurrent votes. Polls get
// away with it because a handful of people vote occasionally; a list says "add
// your dish" to the whole club at once. Positional array filters keep each
// append, edit and removal independent of what anyone else is doing to the same
// list in the same moment.
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
