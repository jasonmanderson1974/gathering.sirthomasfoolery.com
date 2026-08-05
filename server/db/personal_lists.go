// Mongo access for a person's private lists on a gathering (F19).
//
// Every write is the same positional-array update the shared lists use — the
// implementations live in db/event_lists.go and are shared verbatim, only the
// collection and the document filter differ. That reuse is deliberate: nesting,
// fractional ordering and atomic subtree moves are fiddly enough that a second
// copy would drift from the tested one.
//
// The filter is the access control. Every function here takes a userId and puts
// it in the query, so a caller holding someone else's list id still matches no
// document. There is no permission check anywhere above this layer because
// there is nothing left for one to do.
package db

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// personalFilter selects one person's document for one gathering — the key that
// makes these lists private.
func personalFilter(userId, eventId primitive.ObjectID) bson.M {
	return bson.M{"userId": userId, "eventId": eventId}
}

// EnsurePersonalLists creates the empty (userId, eventId) document if it isn't
// there yet, and does nothing if it is.
//
// $setOnInsert rather than $set, so a concurrent call can never blank out lists
// that another request just wrote; the unique (userId, eventId) index turns the
// losing side of a genuine race into a duplicate-key error, which is treated as
// the success it is. Mirrors EnsureDefaultFolders.
func EnsurePersonalLists(userId, eventId primitive.ObjectID) error {
	_, err := PersonalListsCollection.UpdateOne(
		context.Background(),
		personalFilter(userId, eventId),
		bson.M{"$setOnInsert": bson.M{
			"userId":  userId,
			"eventId": eventId,
			"lists":   []models.EventList{},
		}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil // someone else seeded it first — which is what we wanted
		}
		logger.StdErr.Println(err)
		return err
	}
	return nil
}

// GetPersonalLists returns one person's lists for one gathering. Never nil: a
// person who has never opened the tab has no document, and an empty slice is
// the honest answer rather than an error.
func GetPersonalLists(userId, eventId primitive.ObjectID) ([]models.EventList, error) {
	var doc models.PersonalLists
	err := PersonalListsCollection.FindOne(
		context.Background(),
		personalFilter(userId, eventId),
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return []models.EventList{}, nil
		}
		logger.StdErr.Println(err)
		return []models.EventList{}, err
	}
	if doc.Lists == nil {
		return []models.EventList{}, nil
	}
	return doc.Lists, nil
}

// InsertPersonalList appends a new list. Call EnsurePersonalLists first — a
// $push against a document that doesn't exist matches nothing.
func InsertPersonalList(userId, eventId primitive.ObjectID, list models.EventList) error {
	return insertList(PersonalListsCollection, personalFilter(userId, eventId), list)
}

// RenamePersonalList sets a list's name.
func RenamePersonalList(userId, eventId, listId primitive.ObjectID, name string) (bool, error) {
	return renameList(PersonalListsCollection, personalFilter(userId, eventId), listId, name)
}

// DeletePersonalList removes a list and everything on it.
func DeletePersonalList(userId, eventId, listId primitive.ObjectID) (bool, error) {
	return deleteList(PersonalListsCollection, personalFilter(userId, eventId), listId)
}

// InsertPersonalListItem appends an item to a list.
func InsertPersonalListItem(userId, eventId, listId primitive.ObjectID, item models.EventListItem) (bool, error) {
	return insertListItem(PersonalListsCollection, personalFilter(userId, eventId), listId, item)
}

// UpdatePersonalListItemText rewrites one item's text.
func UpdatePersonalListItemText(userId, eventId, listId, itemId primitive.ObjectID, text string) (bool, error) {
	return updateListItemText(PersonalListsCollection, personalFilter(userId, eventId), listId, itemId, text)
}

// DeletePersonalListItems removes a set of items — an item and its whole
// subtree — in one atomic update.
func DeletePersonalListItems(userId, eventId, listId primitive.ObjectID, itemIds []primitive.ObjectID) (bool, error) {
	return deleteListItems(PersonalListsCollection, personalFilter(userId, eventId), listId, itemIds)
}

// SetPersonalListItemChecked ticks or unticks a checklist item.
//
// Writes ONLY the state and the time — never the "who changed it" fields the
// shared path stamps. There is one person who could have done it, so a name
// here would be noise, and the frontend's checkStateLabel keys off
// checkedByName precisely so an item without one renders no attribution line.
// Reports MatchedCount, so re-ticking an already-ticked box is a success.
func SetPersonalListItemChecked(
	userId, eventId, listId, itemId primitive.ObjectID,
	checked bool,
	at primitive.DateTime,
) (bool, error) {
	return setListItemFields(
		PersonalListsCollection, personalFilter(userId, eventId), listId, itemId,
		bson.M{"checked": checked, "checkedAt": at},
	)
}

// SetPersonalListItemOrder repositions an item among its existing siblings.
func SetPersonalListItemOrder(userId, eventId, listId, itemId primitive.ObjectID, order float64) (bool, error) {
	return setListItemOrder(PersonalListsCollection, personalFilter(userId, eventId), listId, itemId, order)
}

// FlattenPersonalListItemToTopLevel drops an item out of its parent and places
// it at the given order.
func FlattenPersonalListItemToTopLevel(userId, eventId, listId, itemId primitive.ObjectID, order float64) (bool, error) {
	return flattenListItemToTopLevel(PersonalListsCollection, personalFilter(userId, eventId), listId, itemId, order)
}

// MovePersonalListItems moves a subtree from one of this person's lists to
// another, atomically.
func MovePersonalListItems(
	userId, eventId, srcListId, dstListId primitive.ObjectID,
	itemIds []primitive.ObjectID,
	items []models.EventListItem,
) (bool, error) {
	return moveListItems(
		PersonalListsCollection, personalFilter(userId, eventId),
		srcListId, dstListId, itemIds, items,
	)
}

// DeletePersonalDataForEvent removes everyone's private lists and notes for a
// gathering. Called only when an event is genuinely deleted, never when it is
// soft-deleted — a soft-deleted event still resolves, so its private data is
// still reachable and still theirs.
func DeletePersonalDataForEvent(eventId primitive.ObjectID) error {
	filter := bson.M{"eventId": eventId}
	if _, err := PersonalListsCollection.DeleteMany(context.Background(), filter); err != nil {
		logger.StdErr.Println(err)
		return err
	}
	if _, err := PersonalNotesCollection.DeleteMany(context.Background(), filter); err != nil {
		logger.StdErr.Println(err)
		return err
	}
	return nil
}
