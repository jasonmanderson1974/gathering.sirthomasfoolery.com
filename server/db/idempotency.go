package db

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"sirtom/server/logger"
)

// FindByClientId returns the document a previous attempt at this create already
// wrote, or nil if there isn't one.
//
// `owner` names the field that must match `ownerId` — `userId` on a comment,
// `createdBy` on an expense. It is not there to prevent a collision (a UUID
// does not collide); it is there so a caller holding someone else's clientId
// cannot use this as a way to read their row back.
func FindByClientId(
	collection *mongo.Collection,
	eventId any,
	clientId string,
	owner string,
	ownerId any,
	into any,
) (bool, error) {
	if clientId == "" {
		return false, nil
	}
	filter := bson.M{"eventId": eventId, "clientId": clientId, owner: ownerId}
	if err := collection.FindOne(context.Background(), filter).Decode(into); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		// An outage is not "no previous attempt" — answering that way would
		// insert a duplicate. Same reasoning as GetCommentById (J7).
		logger.StdErr.Println(err)
		return false, err
	}
	return true, nil
}

// InsertWithClientId inserts `document`, and reports `existed` when the same
// clientId got there first.
//
// The duplicate-key arm is the whole point. A caller that has already checked
// with FindByClientId can still lose the race — two replays of one queued write
// arrive together on a reconnect, both find nothing, and both insert. The
// partial unique index (see EnsureIndexes) fails the second, and the honest
// answer to "your create was rejected because it already happened" is the row
// that already happened, not an error.
func InsertWithClientId(
	collection *mongo.Collection,
	document any,
	eventId any,
	clientId string,
	owner string,
	ownerId any,
	into any,
) (existed bool, err error) {
	if _, err := collection.InsertOne(context.Background(), document); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			logger.StdErr.Println(err)
			return false, err
		}
		// Lost the race. Read back what the winner wrote.
		found, readErr := FindByClientId(collection, eventId, clientId, owner, ownerId, into)
		if readErr != nil {
			return false, readErr
		}
		if !found {
			// A duplicate key on some OTHER unique index, or the winner's row
			// belongs to a different owner. Either way this create cannot be
			// satisfied, and pretending otherwise would be worse than failing.
			logger.StdErr.Println("duplicate key on insert but no matching clientId row:", err)
			return false, err
		}
		return true, nil
	}
	return false, nil
}
