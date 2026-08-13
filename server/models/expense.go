// Shared expense tracking on one gathering — "Settle Up" (F22).
//
// Expenses live in their OWN `expenses` collection keyed by EventId, mirroring
// models.Comment rather than models.PersonalNote. The distinction is the point:
// the personal collections are keyed by (userId, eventId) BECAUSE they are
// private, and that key is what makes a leak impossible. An expense is shared
// with everyone who can see the gathering, so it wants the comment shape —
// keyed by the event, with per-request computed permissions.
//
// Money is integer cents end to end. There is no float anywhere on this path,
// in Go or in JavaScript: a ledger that has to reconcile to zero cannot be
// built on a type where 0.1 + 0.2 != 0.3.
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// Split modes. "even" means the shares were derived from the participant list
// by SplitEvenly and should be re-derived if that list changes; "amount" means
// a human typed each share and they must be preserved verbatim.
const (
	SplitModeEven   = "even"
	SplitModeAmount = "amount"
)

// Actions recorded in an expense's change history.
const (
	ExpenseActionCreated        = "created"
	ExpenseActionEdited         = "edited"
	ExpenseActionDeleted        = "deleted"
	ExpenseActionReceiptAdded   = "receipt-added"
	ExpenseActionReceiptRemoved = "receipt-removed"
)

// Expense is one cost incurred on a gathering.
type Expense struct {
	Id primitive.ObjectID `json:"_id"     bson:"_id,omitempty"`

	// ClientId makes this create replayable — see models.Comment.ClientId for
	// the mechanism. It matters most here: a replayed expense does not merely
	// duplicate a row, it DOUBLE-BOOKS MONEY, and every balance in Settle Up is
	// wrong from then on until someone notices and deletes one by hand.
	ClientId string `json:"clientId,omitempty" bson:"clientId,omitempty"`
	EventId primitive.ObjectID `json:"eventId" bson:"eventId"`

	// CreatedBy is the ownership key for editing ("members may edit their own
	// entries"), and is deliberately NOT the same thing as PaidBy — one member
	// may log an expense another fronted, and admins may log on anyone's behalf.
	CreatedBy     primitive.ObjectID `json:"createdBy"     bson:"createdBy"`
	CreatedByName string             `json:"createdByName" bson:"createdByName"`

	// PaidBy is who is owed. The names on this document are snapshots taken at
	// write time and re-resolved on read (resolveExpenseNames), the same
	// arrangement Comment.AuthorName uses: renaming propagates, but a deleted
	// account still renders as the name it had.
	PaidBy     primitive.ObjectID `json:"paidBy"     bson:"paidBy"`
	PaidByName string             `json:"paidByName" bson:"paidByName"`

	// Date is the day the money was spent, which is not the day the row was
	// created — you settle up on Sunday for a Friday you had already forgotten.
	Date        primitive.DateTime `json:"date"        bson:"date"`
	Title       string             `json:"title"       bson:"title"`
	Description string             `json:"description" bson:"description,omitempty"`

	AmountCents int64 `json:"amountCents" bson:"amountCents"`

	SplitMode string `json:"splitMode" bson:"splitMode"`

	// Splits always holds RESOLVED cents summing to exactly AmountCents,
	// whichever mode produced them. Storing the resolution rather than the
	// recipe is what lets the balance calculation be a plain sum with no
	// knowledge of rounding rules, and what stops an even split silently
	// re-dividing itself when someone is added to the gathering later.
	Splits []ExpenseSplit `json:"splits" bson:"splits"`

	Receipts []ExpenseReceiptRef `json:"receipts,omitempty" bson:"receipts,omitempty"`

	// History is the audit trail, append-only and oldest first.
	History []ExpenseChange `json:"history,omitempty" bson:"history,omitempty"`

	CreatedAt primitive.DateTime  `json:"createdAt" bson:"createdAt"`
	UpdatedAt *primitive.DateTime `json:"updatedAt" bson:"updatedAt,omitempty"`

	// Soft delete. A hard delete would take the change history with it, which
	// is the one thing the feature promises to keep. json:"-" on both: the list
	// endpoint never returns deleted rows, so a client has no use for them.
	DeletedAt *primitive.DateTime `json:"-" bson:"deletedAt,omitempty"`
	DeletedBy primitive.ObjectID  `json:"-" bson:"deletedBy,omitempty"`

	// CanEdit is computed per-request for the calling user and never stored —
	// the same idiom as Comment.CanManageThread. The client renders buttons from
	// it; the server re-derives it on every write, so it is a hint, never a
	// grant.
	CanEdit bool `json:"canEdit" bson:"-"`
}

// ExpenseSplit is one person's share of one expense.
type ExpenseSplit struct {
	UserId      primitive.ObjectID `json:"userId"      bson:"userId"`
	Name        string             `json:"name"        bson:"name"`
	AmountCents int64              `json:"amountCents" bson:"amountCents"`
}

// ExpenseReceiptRef is the metadata half of a receipt photo; the bytes live in
// the `expenseReceipts` collection. Same split, and the same reason, as
// models.Avatar: listing a ledger must not drag image binaries through Mongo.
type ExpenseReceiptRef struct {
	Id          primitive.ObjectID `json:"_id"         bson:"_id"`
	ContentType string             `json:"contentType" bson:"contentType"`
	Width       int                `json:"width"       bson:"width"`
	Height      int                `json:"height"      bson:"height"`
	UploadedAt  primitive.DateTime `json:"uploadedAt"  bson:"uploadedAt"`
	UploadedBy  primitive.ObjectID `json:"uploadedBy"  bson:"uploadedBy"`
}

// ExpenseChange is one entry in the audit trail: who did what, when.
type ExpenseChange struct {
	At     primitive.DateTime `json:"at"     bson:"at"`
	ByUser primitive.ObjectID `json:"byUser" bson:"byUser"`
	ByName string             `json:"byName" bson:"byName"`
	Action string             `json:"action" bson:"action"`

	// Changes carries field-level diffs, and is present only on an "edited"
	// entry. The values are PRE-FORMATTED display strings ("$142.50", "2 Aug
	// 2026", "Tom Foolery") rather than raw values, so rendering the trail needs
	// no knowledge of what type each field happens to be — and so a historical
	// entry keeps reading correctly even if the formatting rules change later.
	Changes []ExpenseFieldChange `json:"changes,omitempty" bson:"changes,omitempty"`
}

// ExpenseFieldChange is one before/after pair inside an "edited" history entry.
type ExpenseFieldChange struct {
	Field string `json:"field" bson:"field"`
	From  string `json:"from"  bson:"from"`
	To    string `json:"to"    bson:"to"`
}
