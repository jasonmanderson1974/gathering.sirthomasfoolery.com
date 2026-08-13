package db_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"sirtom/server/db"
	"sirtom/server/models"
)

/*
Replaying a queued create must produce ONE row (TODO3 O4).

The offline write queue has to be able to send the same create twice — the first
attempt may well have reached the server and had only its response lost — and
before O4 every replay made a second comment, or worse, booked the same expense
again and made every balance in Settle Up wrong.

These run against a real Mongo (MONGODB_URI) because the guarantee is a
database-level one: the lookup-then-insert in the route is only the fast path,
and what actually stops a duplicate under concurrency is the partial unique
index. A test with a faked collection would exercise the arm that doesn't
matter.
*/

func expenseFixture(eventId primitive.ObjectID, createdBy primitive.ObjectID, clientId string) models.Expense {
	now := primitive.NewDateTimeFromTime(time.Now())
	return models.Expense{
		Id:            primitive.NewObjectID(),
		EventId:       eventId,
		ClientId:      clientId,
		CreatedBy:     createdBy,
		CreatedByName: "Bilbo",
		PaidBy:        createdBy,
		PaidByName:    "Bilbo",
		Date:          now,
		Title:         "Port",
		AmountCents:   4200,
		SplitMode:     "even",
		CreatedAt:     now,
	}
}

// Cleans up whatever the test wrote, so a shared dev Mongo doesn't accumulate.
func cleanupExpenses(t *testing.T, eventId primitive.ObjectID) {
	t.Cleanup(func() {
		_, _ = db.ExpensesCollection.DeleteMany(
			context.Background(), bson.M{"eventId": eventId},
		)
	})
}

func TestInsertExpenseIdempotent_ReplayInsertsOnce(t *testing.T) {
	eventId := primitive.NewObjectID()
	userId := primitive.NewObjectID()
	cleanupExpenses(t, eventId)

	first := expenseFixture(eventId, userId, "queued-write-1")
	var raced models.Expense
	existed, err := db.InsertExpenseIdempotent(first, &raced)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if existed {
		t.Fatal("first insert reported an existing row; nothing was there")
	}

	// The replay: a NEW ObjectID, as the route would mint, but the same
	// clientId. This is exactly what a queue flush sends after a lost response.
	second := expenseFixture(eventId, userId, "queued-write-1")
	existed, err = db.InsertExpenseIdempotent(second, &raced)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !existed {
		t.Error("replay was not recognised as one — this double-books the money")
	}
	if raced.Id != first.Id {
		t.Errorf("replay returned id %s, want the original %s — the client cannot "+
			"map its temporary id onto the real row", raced.Id.Hex(), first.Id.Hex())
	}

	count, err := db.ExpensesCollection.CountDocuments(
		context.Background(), bson.M{"eventId": eventId},
	)
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d expenses after a replay, want 1", count)
	}
}

// The case the unique index exists for. A lookup-then-insert cannot survive
// this on its own: both goroutines read nothing and both try to write.
func TestInsertExpenseIdempotent_ConcurrentReplaysInsertOnce(t *testing.T) {
	eventId := primitive.NewObjectID()
	userId := primitive.NewObjectID()
	cleanupExpenses(t, eventId)

	const attempts = 8
	var wg sync.WaitGroup
	errs := make([]error, attempts)

	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func(i int) {
			defer wg.Done()
			var raced models.Expense
			_, errs[i] = db.InsertExpenseIdempotent(
				expenseFixture(eventId, userId, "queued-write-concurrent"), &raced,
			)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("attempt %d failed: %v", i, err)
		}
	}

	count, err := db.ExpensesCollection.CountDocuments(
		context.Background(), bson.M{"eventId": eventId},
	)
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d expenses from %d concurrent replays, want 1", count, attempts)
	}
}

// Everything written before O4, and every ordinary create from a client that
// doesn't send a clientId, must keep working — and must not collapse into "one
// row per event", which is what a non-partial unique index would have done.
func TestInsertExpenseIdempotent_WithoutClientIdIsUnaffected(t *testing.T) {
	eventId := primitive.NewObjectID()
	userId := primitive.NewObjectID()
	cleanupExpenses(t, eventId)

	for i := 0; i < 3; i++ {
		var raced models.Expense
		existed, err := db.InsertExpenseIdempotent(expenseFixture(eventId, userId, ""), &raced)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
		if existed {
			t.Fatalf("insert %d was treated as a replay; a create with no clientId cannot be one", i)
		}
	}

	count, err := db.ExpensesCollection.CountDocuments(
		context.Background(), bson.M{"eventId": eventId},
	)
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 3 {
		t.Errorf("got %d expenses, want 3 — three separate expenses were entered", count)
	}
}

// A clientId is scoped to its gathering, so the same device replaying its queue
// across two gatherings must not have the second create swallowed by the first.
func TestInsertExpenseIdempotent_ScopedToTheEvent(t *testing.T) {
	userId := primitive.NewObjectID()
	eventA := primitive.NewObjectID()
	eventB := primitive.NewObjectID()
	cleanupExpenses(t, eventA)
	cleanupExpenses(t, eventB)

	var raced models.Expense
	if _, err := db.InsertExpenseIdempotent(expenseFixture(eventA, userId, "same-id"), &raced); err != nil {
		t.Fatalf("event A: %v", err)
	}
	existed, err := db.InsertExpenseIdempotent(expenseFixture(eventB, userId, "same-id"), &raced)
	if err != nil {
		t.Fatalf("event B: %v", err)
	}
	if existed {
		t.Error("a clientId from another gathering swallowed this create")
	}
}

// FindByClientId matches on the owner as well as the id — not because a UUID
// might collide, but so holding someone else's clientId is not a way to read
// their row back.
func TestFindExpenseByClientId_DoesNotCrossOwners(t *testing.T) {
	eventId := primitive.NewObjectID()
	mine := primitive.NewObjectID()
	theirs := primitive.NewObjectID()
	cleanupExpenses(t, eventId)

	var raced models.Expense
	if _, err := db.InsertExpenseIdempotent(expenseFixture(eventId, mine, "mine"), &raced); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var found models.Expense
	ok, err := db.FindExpenseByClientId(eventId, "mine", theirs, &found)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if ok {
		t.Error("another member's clientId resolved to their expense")
	}
}

func TestInsertCommentIdempotent_ReplayInsertsOnce(t *testing.T) {
	eventId := primitive.NewObjectID()
	userId := primitive.NewObjectID().Hex()
	t.Cleanup(func() {
		_, _ = db.CommentsCollection.DeleteMany(
			context.Background(), bson.M{"eventId": eventId},
		)
	})

	comment := func() models.Comment {
		return models.Comment{
			Id:         primitive.NewObjectID(),
			EventId:    eventId,
			UserId:     userId,
			ClientId:   "queued-comment-1",
			AuthorName: "Bilbo",
			Text:       "Bring the good port",
			CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
		}
	}

	first := comment()
	var raced models.Comment
	if _, err := db.InsertCommentIdempotent(first, &raced); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	existed, err := db.InsertCommentIdempotent(comment(), &raced)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !existed {
		t.Error("replay was not recognised as one — the discussion gets it twice")
	}
	if raced.Id != first.Id {
		t.Errorf("replay returned id %s, want %s", raced.Id.Hex(), first.Id.Hex())
	}

	count, err := db.CommentsCollection.CountDocuments(
		context.Background(), bson.M{"eventId": eventId},
	)
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d comments after a replay, want 1", count)
	}
}
