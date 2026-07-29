package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// The avatar bytes and the user's avatarUpdatedAt flag are two documents in two
// collections, so each of these functions writes them in the order that fails
// safe:
//
//   - saving writes the bytes first, then raises the flag. A failure between
//     the two leaves bytes nothing points at — invisible, and overwritten by
//     the next upload.
//   - deleting lowers the flag first, then drops the bytes. A failure between
//     the two leaves the same harmless orphan rather than a flag advertising an
//     avatar that no longer exists (which would render as a broken image).
//
// The opposite order in either case would produce a 404 on a photo the UI has
// been told to show.

// UpsertAvatar stores the canonical avatar bytes for a user and stamps
// avatarUpdatedAt on their account. It returns the timestamp it wrote, which is
// what the client uses as the cache-busting `?v=` on the serving URL.
func UpsertAvatar(userId primitive.ObjectID, data []byte, contentType string) (primitive.DateTime, error) {
	updatedAt := primitive.NewDateTimeFromTime(time.Now().UTC())

	if _, err := AvatarsCollection.UpdateByID(
		context.Background(),
		userId,
		bson.M{"$set": bson.M{
			"data":        primitive.Binary{Subtype: 0x00, Data: data},
			"contentType": contentType,
			"updatedAt":   updatedAt,
		}},
		options.Update().SetUpsert(true),
	); err != nil {
		logger.StdErr.Println(err)
		return 0, err
	}

	if _, err := UsersCollection.UpdateByID(
		context.Background(),
		userId,
		bson.M{"$set": bson.M{"avatarUpdatedAt": updatedAt}},
	); err != nil {
		logger.StdErr.Println(err)
		return 0, err
	}

	return updatedAt, nil
}

// GetAvatar returns the stored avatar for a user, or (nil, nil) when they have
// none — a member without a photo is an ordinary state, not an error.
func GetAvatar(userId primitive.ObjectID) (*models.Avatar, error) {
	var avatar models.Avatar
	err := AvatarsCollection.FindOne(context.Background(), bson.M{"_id": userId}).Decode(&avatar)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}
	return &avatar, nil
}

// DeleteAvatar removes a user's photo and clears the flag. Idempotent: deleting
// an avatar that isn't there succeeds.
func DeleteAvatar(userId primitive.ObjectID) error {
	if _, err := UsersCollection.UpdateByID(
		context.Background(),
		userId,
		bson.M{"$unset": bson.M{"avatarUpdatedAt": ""}},
	); err != nil {
		logger.StdErr.Println(err)
		return err
	}

	if _, err := AvatarsCollection.DeleteOne(context.Background(), bson.M{"_id": userId}); err != nil {
		logger.StdErr.Println(err)
		return err
	}

	return nil
}
