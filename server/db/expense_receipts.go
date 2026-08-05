package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// A receipt is two documents in two collections — the bytes here, the reference
// on the expense — so each operation writes them in the order that fails safe,
// the same reasoning as db/avatars.go:
//
//   - uploading stores the bytes first, then attaches the reference. A failure
//     between the two leaves bytes nothing points at: invisible, and swept when
//     the gathering is deleted.
//   - removing detaches the reference first, then drops the bytes. A failure
//     between the two leaves that same harmless orphan, rather than a thumbnail
//     the ledger renders and the serving route 404s.

// InsertExpenseReceipt stores the canonical bytes for one receipt photo.
func InsertExpenseReceipt(receipt models.ExpenseReceipt) error {
	_, err := ExpenseReceiptsCollection.InsertOne(context.Background(), receipt)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// GetExpenseReceipt returns one stored receipt, or (nil, nil) when there is no
// such id — an expense with no photo is an ordinary state, not an error.
func GetExpenseReceipt(receiptId primitive.ObjectID) (*models.ExpenseReceipt, error) {
	var receipt models.ExpenseReceipt
	err := ExpenseReceiptsCollection.FindOne(context.Background(), bson.M{"_id": receiptId}).Decode(&receipt)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}
	return &receipt, nil
}

// DeleteExpenseReceipt drops the bytes. Idempotent.
func DeleteExpenseReceipt(receiptId primitive.ObjectID) error {
	_, err := ExpenseReceiptsCollection.DeleteOne(context.Background(), bson.M{"_id": receiptId})
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// CountExpenseReceipts reports how many photos an expense already carries, so
// the upload route can enforce its cap against the collection rather than
// against a possibly-stale copy of the expense document.
func CountExpenseReceipts(expenseId primitive.ObjectID) (int64, error) {
	count, err := ExpenseReceiptsCollection.CountDocuments(context.Background(), bson.M{"expenseId": expenseId})
	if err != nil {
		logger.StdErr.Println(err)
	}
	return count, err
}
