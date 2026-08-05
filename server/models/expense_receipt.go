package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// ExpenseReceipt is the stored image for one receipt photo, in its own
// collection keyed by its own id.
//
// Deliberately NOT an array of Binary on the Expense document, for the same
// reason models.Avatar is not a field on User: GET /expenses lists every
// expense on a gathering, and a handful of 300KB photos per row would make the
// ledger read tens of megabytes to render a list of titles and amounts. The
// document carries ExpenseReceiptRef — id, content type, dimensions — and the
// only thing that ever fetches bytes is the one route that serves them.
//
// Stored bytes are always the canonical re-encode the upload handler produced
// (JPEG, long edge capped), never whatever the client sent. EventId is carried
// alongside ExpenseId so a gathering's receipts can be swept in one query when
// the event is hard-deleted, without first reading every expense.
type ExpenseReceipt struct {
	Id        primitive.ObjectID `json:"_id"       bson:"_id"`
	ExpenseId primitive.ObjectID `json:"expenseId" bson:"expenseId"`
	EventId   primitive.ObjectID `json:"eventId"   bson:"eventId"`

	Data        primitive.Binary   `json:"-"           bson:"data"`
	ContentType string             `json:"contentType" bson:"contentType"`
	Width       int                `json:"width"       bson:"width"`
	Height      int                `json:"height"      bson:"height"`
	CreatedAt   primitive.DateTime `json:"createdAt"   bson:"createdAt"`
}
