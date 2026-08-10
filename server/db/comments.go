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

// GetComments returns an event's discussion thread, oldest first (C7).
func GetComments(eventId string) ([]models.Comment, error) {
	objectId, err := primitive.ObjectIDFromHex(eventId)
	if err != nil {
		// eventId is malformatted
		return []models.Comment{}, nil
	}

	result, err := CommentsCollection.Find(
		context.Background(),
		bson.M{"eventId": objectId},
		options.Find().SetSort(bson.M{"createdAt": 1}),
	)
	if err != nil {
		logger.StdErr.Println(err)
		return []models.Comment{}, err
	}

	var comments []models.Comment
	if err := result.All(context.Background(), &comments); err != nil {
		logger.StdErr.Println(err)
		return []models.Comment{}, err
	}

	return comments, nil
}

// GetCommentById returns a single comment, or nil if it doesn't exist. A
// malformatted id is "not found"; a Mongo failure is a real error (J7).
func GetCommentById(commentId string) (*models.Comment, error) {
	objectId, err := primitive.ObjectIDFromHex(commentId)
	if err != nil {
		return nil, nil
	}
	result := CommentsCollection.FindOne(context.Background(), bson.M{"_id": objectId})
	var comment models.Comment
	if err := result.Decode(&comment); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // not found
		}
		// An outage is not a missing comment (J7). Collapsing every Decode
		// failure to "not found" made deleteComment answer 200 "already gone"
		// while Mongo was down, which is the one wrong answer for an idempotent
		// delete: it tells the client the row is gone when nothing was read.
		logger.StdErr.Println(err)
		return nil, err
	}
	return &comment, nil
}

func InsertComment(comment models.Comment) error {
	_, err := CommentsCollection.InsertOne(context.Background(), comment)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// UpdateCommentText rewrites a comment's text and the mentions parsed out of it
// (F7).
//
// An edit that removes every mention must clear the field rather than leave the
// old ids behind — they would then be stale for the edit-diff that decides who
// gets notified, and would outlive the tokens that justified them. Hence the
// $unset arm, which also keeps the stored document matching the `omitempty` tag.
func UpdateCommentText(commentId primitive.ObjectID, text string, mentions []primitive.ObjectID, updatedAt primitive.DateTime) error {
	update := bson.M{"$set": bson.M{"text": text, "updatedAt": updatedAt}}
	if len(mentions) > 0 {
		update["$set"].(bson.M)["mentions"] = mentions
	} else {
		update["$unset"] = bson.M{"mentions": ""}
	}

	_, err := CommentsCollection.UpdateByID(context.Background(), commentId, update)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

func DeleteComment(commentId primitive.ObjectID) error {
	_, err := CommentsCollection.DeleteOne(context.Background(), bson.M{"_id": commentId})
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// SetCommentThread tags a top-level comment as a thread root (C13), recording
// who tagged it and whether it's members-only.
func SetCommentThread(commentId primitive.ObjectID, membersOnly bool, threadedBy string) error {
	_, err := CommentsCollection.UpdateByID(
		context.Background(),
		commentId,
		bson.M{"$set": bson.M{
			"isThread":    true,
			"membersOnly": membersOnly,
			"threadedBy":  threadedBy,
		}},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// SetCommentMembersOnly flips the members-only flag on an existing thread root.
func SetCommentMembersOnly(commentId primitive.ObjectID, membersOnly bool) error {
	_, err := CommentsCollection.UpdateByID(
		context.Background(),
		commentId,
		bson.M{"$set": bson.M{"membersOnly": membersOnly}},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// ClearCommentThread un-tags a thread root, returning it to an ordinary
// top-level comment. Callers must first confirm it has no replies
// (CountThreadReplies) — un-tagging a thread with replies would orphan them.
func ClearCommentThread(commentId primitive.ObjectID) error {
	_, err := CommentsCollection.UpdateByID(
		context.Background(),
		commentId,
		bson.M{"$unset": bson.M{"isThread": "", "membersOnly": "", "threadedBy": ""}},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// CountThreadReplies returns how many replies hang off a thread root.
func CountThreadReplies(threadId primitive.ObjectID) (int64, error) {
	count, err := CommentsCollection.CountDocuments(context.Background(), bson.M{"threadId": threadId})
	if err != nil {
		logger.StdErr.Println(err)
	}
	return count, err
}

// DeleteCommentsByThreadId removes every reply under a thread root. Paired with
// DeleteComment when the root itself is deleted, so replies aren't left
// unreachable in the collection.
func DeleteCommentsByThreadId(threadId primitive.ObjectID) error {
	_, err := CommentsCollection.DeleteMany(context.Background(), bson.M{"threadId": threadId})
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}
