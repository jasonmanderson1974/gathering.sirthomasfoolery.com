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

// Mongo accessors for the Settle Up ledger (F22).
//
// Every mutation below pairs its `$set` with the `$push` of the matching
// history entry in ONE update. That is not tidiness: the audit trail is the
// feature's promise, and two writes would admit a state where the amount had
// changed but nothing recorded that it did — precisely the state the trail
// exists to rule out.
//
// maxExpenseHistory caps the trail with `$slice`. Fifty entries is far more
// than a dinner bill will ever accumulate; the cap is there so a script in a
// loop cannot grow one document past Mongo's 16MB limit.
const maxExpenseHistory = 50

// activeExpenses matches the rows a client may see: everything not soft-deleted.
func activeExpenses(filter bson.M) bson.M {
	filter["deletedAt"] = bson.M{"$exists": false}
	return filter
}

// GetExpenses returns a gathering's ledger, newest first.
//
// Sorted by `date` (the day the money was spent, which is what the row
// displays) with `createdAt` as the tiebreaker, so two expenses entered for the
// same day still fall in a stable, sensible order rather than Mongo's natural
// one.
func GetExpenses(eventId string) ([]models.Expense, error) {
	objectId, err := primitive.ObjectIDFromHex(eventId)
	if err != nil {
		// eventId is malformatted — no such ledger, which is not an error.
		return []models.Expense{}, nil
	}

	result, err := ExpensesCollection.Find(
		context.Background(),
		activeExpenses(bson.M{"eventId": objectId}),
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}, {Key: "createdAt", Value: -1}}),
	)
	if err != nil {
		logger.StdErr.Println(err)
		return []models.Expense{}, err
	}

	var expenses []models.Expense
	if err := result.All(context.Background(), &expenses); err != nil {
		logger.StdErr.Println(err)
		return []models.Expense{}, err
	}
	if expenses == nil {
		expenses = []models.Expense{}
	}

	return expenses, nil
}

// GetExpenseById returns a single live expense, or nil if it doesn't exist or
// has been deleted. A malformatted id is "not found", not an error — the same
// contract GetCommentById offers.
//
// A Mongo outage is NOT "not found" (J7). Every Decode failure used to collapse
// to `nil, nil`, so a connection error mid-request surfaced as a 404
// `expense-not-found` — telling a member their expense had been deleted when the
// database was merely unreachable. Only mongo.ErrNoDocuments means absent;
// anything else is a real error, and the route layer already has the 500 branch
// wired for it.
func GetExpenseById(expenseId string) (*models.Expense, error) {
	objectId, err := primitive.ObjectIDFromHex(expenseId)
	if err != nil {
		return nil, nil
	}

	var expense models.Expense
	if err := ExpensesCollection.FindOne(
		context.Background(),
		activeExpenses(bson.M{"_id": objectId}),
	).Decode(&expense); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		logger.StdErr.Println(err)
		return nil, err
	}
	return &expense, nil
}

func InsertExpense(expense models.Expense) error {
	_, err := ExpensesCollection.InsertOne(context.Background(), expense)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// FindExpenseByClientId returns an expense this member already entered on this
// event under `clientId` — an earlier attempt at the same queued create (O4).
//
// Note the owner field is `createdBy`, not `paidBy`: who queued the write is
// who may replay it, and those are deliberately different people sometimes (one
// member may log an expense another fronted).
func FindExpenseByClientId(
	eventId primitive.ObjectID,
	clientId string,
	createdBy primitive.ObjectID,
	into *models.Expense,
) (bool, error) {
	return FindByClientId(ExpensesCollection, eventId, clientId, "createdBy", createdBy, into)
}

// InsertExpenseIdempotent inserts, unless the same clientId won a race first —
// in which case `raced` is filled with the winner and `existed` is true.
func InsertExpenseIdempotent(expense models.Expense, raced *models.Expense) (bool, error) {
	if expense.ClientId == "" {
		return false, InsertExpense(expense)
	}
	return InsertWithClientId(
		ExpensesCollection, expense,
		expense.EventId, expense.ClientId, "createdBy", expense.CreatedBy, raced,
	)
}

// UpdateExpense applies the changed fields and appends one history entry
// atomically. `fields` holds only what actually changed — a no-op edit calls
// this with nothing and is rejected by the caller before it gets here.
func UpdateExpense(expenseId primitive.ObjectID, fields bson.M, change models.ExpenseChange) error {
	_, err := ExpensesCollection.UpdateByID(
		context.Background(),
		expenseId,
		bson.M{
			"$set":  fields,
			"$push": pushHistory(change),
		},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// SoftDeleteExpense marks an expense deleted rather than removing it, so the
// change history survives. Guarded on it not already being deleted, so a double
// submit cannot stamp a second, later deletedAt over the first.
func SoftDeleteExpense(expenseId primitive.ObjectID, deletedBy primitive.ObjectID, at primitive.DateTime, change models.ExpenseChange) error {
	_, err := ExpensesCollection.UpdateOne(
		context.Background(),
		activeExpenses(bson.M{"_id": expenseId}),
		bson.M{
			"$set":  bson.M{"deletedAt": at, "deletedBy": deletedBy},
			"$push": pushHistory(change),
		},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// AddExpenseReceiptRef attaches a stored receipt's metadata to its expense.
// Called only after the bytes are safely written — see db/expense_receipts.go
// for why that order is the one that fails safe.
func AddExpenseReceiptRef(expenseId primitive.ObjectID, ref models.ExpenseReceiptRef, change models.ExpenseChange) error {
	push := pushHistory(change)
	push["receipts"] = ref

	_, err := ExpensesCollection.UpdateByID(
		context.Background(),
		expenseId,
		bson.M{"$push": push},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// RemoveExpenseReceiptRef detaches a receipt. `$pull` and `$push` touch
// lexically distinct paths, so Mongo accepts them in one update — the same
// trick db/event_lists.go uses to move a list item between lists atomically.
func RemoveExpenseReceiptRef(expenseId, receiptId primitive.ObjectID, change models.ExpenseChange) error {
	_, err := ExpensesCollection.UpdateByID(
		context.Background(),
		expenseId,
		bson.M{
			"$pull": bson.M{"receipts": bson.M{"_id": receiptId}},
			"$push": pushHistory(change),
		},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// DeleteExpensesForEvent removes a gathering's whole ledger, receipts included.
// Called only on HARD delete (routes/events.go), never on soft delete — the
// same rule DeletePersonalDataForEvent follows.
func DeleteExpensesForEvent(eventId primitive.ObjectID) error {
	if _, err := ExpenseReceiptsCollection.DeleteMany(context.Background(), bson.M{"eventId": eventId}); err != nil {
		logger.StdErr.Println(err)
		return err
	}
	if _, err := ExpensesCollection.DeleteMany(context.Background(), bson.M{"eventId": eventId}); err != nil {
		logger.StdErr.Println(err)
		return err
	}
	return nil
}

// pushHistory builds the `$push` arm that appends one trail entry and keeps the
// trail bounded.
func pushHistory(change models.ExpenseChange) bson.M {
	return bson.M{
		"history": bson.M{"$each": []models.ExpenseChange{change}, "$slice": -maxExpenseHistory},
	}
}
