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

// Default (system) folder kinds and their display names. Every user gets one
// of each; new events land in them automatically (see AssignUnfiledEventsToDefaults).
const (
	DefaultFolderKindCreated  = "created"
	DefaultFolderKindReceived = "received"
	defaultFolderNameCreated  = "Invites created"
	defaultFolderNameReceived = "Invites received"
)

// ErrCannotDeleteDefaultFolder is returned when a delete targets a system folder.
var ErrCannotDeleteDefaultFolder = errors.New("cannot delete a default folder")

// EnsureDefaultFolders returns the user's two default folders ("created" and
// "received"), creating them if they don't exist yet. Idempotent; safe to call
// on every dashboard load. A unique index on {userId, defaultKind} (see db.Init)
// prevents duplicates under concurrent calls.
func EnsureDefaultFolders(userId primitive.ObjectID) (created *models.Folder, received *models.Folder, err error) {
	created, err = ensureDefaultFolder(userId, DefaultFolderKindCreated, defaultFolderNameCreated)
	if err != nil {
		return nil, nil, err
	}
	received, err = ensureDefaultFolder(userId, DefaultFolderKindReceived, defaultFolderNameReceived)
	if err != nil {
		return nil, nil, err
	}
	return created, received, nil
}

func ensureDefaultFolder(userId primitive.ObjectID, kind string, name string) (*models.Folder, error) {
	ctx := context.Background()
	trueVal := true
	filter := bson.M{"userId": userId, "defaultKind": kind}
	update := bson.M{
		"$setOnInsert": bson.M{
			"userId":      userId,
			"name":        name,
			"defaultKind": kind,
			"isDefault":   trueVal,
		},
	}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var folder models.Folder
	err := FoldersCollection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&folder)
	if mongo.IsDuplicateKeyError(err) {
		// Lost an upsert race; the folder now exists, so just read it.
		err = FoldersCollection.FindOne(ctx, filter).Decode(&folder)
	}
	if err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}
	return &folder, nil
}

// AssignUnfiledEventsToDefaults files any of the given events that aren't yet in
// a folder (for this user) into the appropriate default folder: events the user
// owns go to "created", the rest to "received". Best-effort; existing folder
// assignments (including user overrides) are left untouched.
func AssignUnfiledEventsToDefaults(userId primitive.ObjectID, events []models.Event, createdFolderId primitive.ObjectID, receivedFolderId primitive.ObjectID) error {
	ctx := context.Background()

	// Collect event ids already mapped to some folder for this user
	cursor, err := FolderEventsCollection.Find(ctx, bson.M{"userId": userId},
		options.Find().SetProjection(bson.M{"eventId": 1}))
	if err != nil {
		return err
	}
	var mappedRows []struct {
		EventId primitive.ObjectID `bson:"eventId"`
	}
	if err = cursor.All(ctx, &mappedRows); err != nil {
		return err
	}
	mapped := make(map[primitive.ObjectID]struct{}, len(mappedRows))
	for _, row := range mappedRows {
		mapped[row.EventId] = struct{}{}
	}

	docs := make([]interface{}, 0)
	for _, event := range events {
		if _, ok := mapped[event.Id]; ok {
			continue
		}
		folderId := receivedFolderId
		if event.OwnerId == userId {
			folderId = createdFolderId
		}
		docs = append(docs, models.FolderEvent{
			UserId:   userId,
			FolderId: folderId,
			EventId:  event.Id,
		})
	}

	if len(docs) > 0 {
		if _, err := FolderEventsCollection.InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	return nil
}

func CreateFolder(folder *models.Folder) (primitive.ObjectID, error) {
	result, err := FoldersCollection.InsertOne(context.Background(), folder)
	if err != nil {
		logger.StdErr.Println(err)
		return primitive.NilObjectID, err
	}
	return result.InsertedID.(primitive.ObjectID), nil
}

func GetFolderById(folderId primitive.ObjectID, userId primitive.ObjectID) (*models.Folder, error) {
	var folder models.Folder
	err := FoldersCollection.FindOne(context.Background(), bson.M{
		"_id":    folderId,
		"userId": userId,
		"$or": bson.A{
			bson.M{"isDeleted": bson.M{"$exists": false}},
			bson.M{"isDeleted": false},
		},
	}).Decode(&folder)
	if err != nil {
		return nil, err
	}

	return &folder, nil
}

func GetAllFolders(userId primitive.ObjectID) ([]models.Folder, error) {
	cursor, err := FoldersCollection.Find(context.Background(), bson.M{
		"userId": userId,
		"$or": bson.A{
			bson.M{"isDeleted": bson.M{"$exists": false}},
			bson.M{"isDeleted": false},
		},
	})
	if err != nil {
		return nil, err
	}

	var folders []models.Folder
	if err = cursor.All(context.Background(), &folders); err != nil {
		return nil, err
	}

	for i, folder := range folders {
		events, err := GetEventsInFolder(folder.Id, userId)
		if err != nil {
			return nil, err
		}
		if events != nil {
			folders[i].EventIds = events
		} else {
			folders[i].EventIds = []primitive.ObjectID{}
		}
	}

	return folders, nil
}

func GetEventsInFolder(folderId primitive.ObjectID, userId primitive.ObjectID) ([]primitive.ObjectID, error) {
	cursor, err := FolderEventsCollection.Find(context.Background(), bson.M{
		"folderId": folderId,
		"userId":   userId,
	}, options.Find().SetProjection(bson.M{"eventId": 1}))
	if err != nil {
		return nil, err
	}

	var eventIdsResponse []struct {
		EventId primitive.ObjectID `bson:"eventId"`
	}
	if err = cursor.All(context.Background(), &eventIdsResponse); err != nil {
		return nil, err
	}

	eventIds := make([]primitive.ObjectID, len(eventIdsResponse))
	for i, eventId := range eventIdsResponse {
		eventIds[i] = eventId.EventId
	}

	return eventIds, nil
}

func UpdateFolder(folderId primitive.ObjectID, userId primitive.ObjectID, updates bson.M) error {
	_, err := FoldersCollection.UpdateOne(context.Background(), bson.M{"_id": folderId, "userId": userId}, bson.M{"$set": updates})
	return err
}

func SetEventFolder(eventId primitive.ObjectID, folderId *primitive.ObjectID, userId primitive.ObjectID) error {
	ctx := context.Background()

	// Remove any existing mapping for this event
	_, err := FolderEventsCollection.DeleteMany(ctx, bson.M{"eventId": eventId, "userId": userId})
	if err != nil {
		return err
	}

	// If folderId is nil, we just un-assign it. Otherwise, create a new mapping
	if folderId != nil {
		_, err = FolderEventsCollection.InsertOne(ctx, models.FolderEvent{
			FolderId: *folderId,
			EventId:  eventId,
			UserId:   userId,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteFolder(folderId primitive.ObjectID, userId primitive.ObjectID) error {
	ctx := context.Background()

	// Never delete a default (system) folder
	folder, err := GetFolderById(folderId, userId)
	if err != nil {
		return err
	}
	if folder.IsDefault != nil && *folder.IsDefault {
		return ErrCannotDeleteDefaultFolder
	}

	// Mark this folder as deleted
	_, err = FoldersCollection.UpdateOne(ctx, bson.M{"_id": folderId, "userId": userId}, bson.M{"$set": bson.M{"isDeleted": true}})
	if err != nil {
		return err
	}

	// Find all event mappings for this folder
	eventIds, err := GetEventsInFolder(folderId, userId)
	if err != nil {
		return err
	}

	// Mark all events in this folder as deleted (if the user owns the event)
	if len(eventIds) > 0 {
		_, err = EventsCollection.UpdateMany(ctx, bson.M{"_id": bson.M{"$in": eventIds}, "ownerId": userId}, bson.M{"$set": bson.M{"isDeleted": true}})
		if err != nil {
			return err
		}
	}

	// Delete the mappings
	_, err = FolderEventsCollection.DeleteMany(ctx, bson.M{"folderId": folderId, "userId": userId})
	if err != nil {
		return err
	}

	return nil
}
