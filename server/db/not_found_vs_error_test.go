package db_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
)

// J7: GetCommentById and GetExpenseById collapsed EVERY Decode failure to
// `nil, nil`, so anything that wasn't a clean read — a Mongo outage, a document
// that won't unmarshal — was reported to the route layer as "not found". The
// route then answered 404 comment-not-found / expense-not-found, telling a
// member their row had been deleted when in fact nothing had been read.
//
// A genuine connection outage can't be staged against the shared test Mongo, so
// these use the other member of the same error class: a stored document whose
// field types don't match the model. It fails to Decode with a type error
// rather than mongo.ErrNoDocuments, which is exactly the distinction the fix
// introduced — before it, both returned `nil, nil` indistinguishably.
//
// The contract being pinned, in both directions:
//   - absent row      -> (nil, nil)   ... a real 404
//   - malformatted id -> (nil, nil)   ... also a real 404
//   - undecodable row -> (nil, error) ... a 500, not a 404

func TestGetCommentById_UndecodableRowIsAnErrorNotNotFound(t *testing.T) {
	ctx := context.Background()
	commentId := primitive.NewObjectID()

	// userId is a string on the model; store an int so Decode fails on type.
	if _, err := db.CommentsCollection.InsertOne(ctx, bson.M{
		"_id":     commentId,
		"eventId": primitive.NewObjectID(),
		"userId":  12345,
	}); err != nil {
		t.Fatalf("insert malformed comment: %v", err)
	}
	defer func() { _, _ = db.CommentsCollection.DeleteOne(ctx, bson.M{"_id": commentId}) }()

	comment, err := db.GetCommentById(commentId.Hex())
	if err == nil {
		t.Fatal("an undecodable comment reported success — the route layer would answer 404 for a row that exists")
	}
	if comment != nil {
		t.Errorf("expected no comment alongside the error, got %+v", comment)
	}
}

func TestGetCommentById_AbsentAndMalformedStayNotFound(t *testing.T) {
	comment, err := db.GetCommentById(primitive.NewObjectID().Hex())
	if err != nil || comment != nil {
		t.Errorf("absent comment: got (%+v, %v), want (nil, nil)", comment, err)
	}

	comment, err = db.GetCommentById("not-an-object-id")
	if err != nil || comment != nil {
		t.Errorf("malformatted id: got (%+v, %v), want (nil, nil)", comment, err)
	}
}

func TestGetExpenseById_UndecodableRowIsAnErrorNotNotFound(t *testing.T) {
	ctx := context.Background()
	expenseId := primitive.NewObjectID()

	// description is a string on the model; store a document so Decode fails.
	if _, err := db.ExpensesCollection.InsertOne(ctx, bson.M{
		"_id":         expenseId,
		"eventId":     primitive.NewObjectID(),
		"description": bson.M{"nested": "object"},
	}); err != nil {
		t.Fatalf("insert malformed expense: %v", err)
	}
	defer func() { _, _ = db.ExpensesCollection.DeleteOne(ctx, bson.M{"_id": expenseId}) }()

	expense, err := db.GetExpenseById(expenseId.Hex())
	if err == nil {
		t.Fatal("an undecodable expense reported success — the route layer would answer 404 for a row that exists")
	}
	if expense != nil {
		t.Errorf("expected no expense alongside the error, got %+v", expense)
	}
}

func TestGetExpenseById_AbsentAndMalformedStayNotFound(t *testing.T) {
	expense, err := db.GetExpenseById(primitive.NewObjectID().Hex())
	if err != nil || expense != nil {
		t.Errorf("absent expense: got (%+v, %v), want (nil, nil)", expense, err)
	}

	expense, err = db.GetExpenseById("not-an-object-id")
	if err != nil || expense != nil {
		t.Errorf("malformatted id: got (%+v, %v), want (nil, nil)", expense, err)
	}
}
