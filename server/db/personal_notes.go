// Mongo access for a person's private note on a gathering (F20).
//
// One document per (userId, eventId) holding one free-form markdown string. Like
// db/personal_lists.go, the userId in the filter IS the access control.
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

// GetPersonalNote returns one person's note for one gathering, or (nil, nil)
// when they have never written one.
//
// Absence is not an error and not an empty struct: the route turns nil into
// {text: "", updatedAt: null}, and keeping the two distinguishable here means a
// caller can still tell "never written" from "written, then cleared".
func GetPersonalNote(userId, eventId primitive.ObjectID) (*models.PersonalNote, error) {
	var note models.PersonalNote
	err := PersonalNotesCollection.FindOne(
		context.Background(),
		personalFilter(userId, eventId),
	).Decode(&note)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		logger.StdErr.Println(err)
		return nil, err
	}
	return &note, nil
}

// SetPersonalNote writes the note, creating the document on first save.
//
// Upsert with the keys in $setOnInsert and the content in $set, so the two can
// never disagree about whose note this is. Returns what was actually stored —
// the route hands that straight back so the client's "Saved 12:04" reflects the
// database rather than what it hoped it sent.
func SetPersonalNote(userId, eventId primitive.ObjectID, text string, at primitive.DateTime) (models.PersonalNote, error) {
	note := models.PersonalNote{UserId: userId, EventId: eventId, Text: text, UpdatedAt: at}

	_, err := PersonalNotesCollection.UpdateOne(
		context.Background(),
		personalFilter(userId, eventId),
		bson.M{
			"$set":         bson.M{"text": text, "updatedAt": at},
			"$setOnInsert": bson.M{"userId": userId, "eventId": eventId},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		logger.StdErr.Println(err)
		return models.PersonalNote{}, err
	}
	return note, nil
}
