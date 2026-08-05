// Settle Up (F22): the shared expense ledger on one gathering.
//
// Three rules shape everything here.
//
// Reading is open, writing is not. Everyone who can see the gathering can see
// what it cost — guests included, because a guest who was at the dinner has a
// legitimate interest in the bill. Only members and up may add an expense, and
// only the person who entered it (or an admin) may change it.
//
// The server owns the arithmetic. A client sends a total and who it is shared
// between; it never sends the shares for an even split, and when it does send
// per-person amounts they are checked to the cent against the total before
// anything is stored. Everything downstream — the balances in the sidebar, the
// audit trail — assumes Splits sums to AmountCents, so that invariant is
// established at the only place it can be enforced.
//
// Every write records who did it. The change history is the feature's promise,
// so the db layer pairs each mutation with its trail entry in one update; the
// diffing that produces those entries is here, pure and table-tested.
package routes

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/models"
	"sirtom/server/responses"
)

const (
	maxExpenseTitleLength       = 120
	maxExpenseDescriptionLength = 2000

	// maxExpenseAmountCents caps a single expense at $100,000. Not a business
	// rule — a guard against a fat finger turning £42 into a number that makes
	// the balance column unreadable for everyone.
	maxExpenseAmountCents = 100_000_00

	// maxExpenseParticipants bounds the split list. The roll is far smaller than
	// this; the cap exists so a hand-rolled client cannot make one document
	// enormous.
	maxExpenseParticipants = 200
)

// initExpenseRoutes registers the ledger on the events group.
//
// Registered on the SAME group as comments and lists rather than a group of its
// own: Gin refuses two different wildcard names at one position in the tree, so
// everything hanging off :eventId has to share one registration point.
func initExpenseRoutes(authed *gin.RouterGroup) {
	authed.GET("/:eventId/expenses", getExpenses)
	authed.GET("/:eventId/expenses/participants", getExpenseParticipants)
	authed.POST("/:eventId/expenses", createExpense)
	authed.PUT("/:eventId/expenses/:expenseId", updateExpense)
	authed.DELETE("/:eventId/expenses/:expenseId", deleteExpense)
	authed.POST("/:eventId/expenses/:expenseId/receipts", addExpenseReceipt)
	authed.GET("/:eventId/expenses/:expenseId/receipts/:receiptId", getExpenseReceipt)
	authed.DELETE("/:eventId/expenses/:expenseId/receipts/:receiptId", deleteExpenseReceipt)
}

// expenseViewer is the caller's identity as it bears on the ledger. A plain
// struct with no request and no database behind it, so the permission rules
// below stay unit-testable — the same arrangement commentViewer and listViewer
// use.
type expenseViewer struct {
	UserId primitive.ObjectID
	// IsMember is member-and-up. Guests read the ledger and write nothing.
	IsMember bool
	IsAdmin  bool
}

func newExpenseViewer(user *models.User) expenseViewer {
	role := user.EffectiveRole()
	return expenseViewer{
		UserId:   user.Id,
		IsMember: role.CanSeeMembersOnly(),
		IsAdmin:  role.CanManageUsers(),
	}
}

// canCreate: members and up may add an expense.
func (v expenseViewer) canCreate() bool { return v.IsMember }

// canEdit: the member who entered it, or any admin.
//
// Ownership is CreatedBy, not PaidBy. Someone who logs an expense on another
// member's behalf keeps the ability to correct their own typo; being the person
// owed does not by itself confer the right to rewrite the record.
func (v expenseViewer) canEdit(expense *models.Expense) bool {
	if v.IsAdmin {
		return true
	}
	return v.IsMember && expense.CreatedBy == v.UserId
}

// loadExpenseContext resolves the event and the caller for a ledger route,
// writing the error response itself when anything is missing.
func loadExpenseContext(c *gin.Context) (*models.Event, *models.User, expenseViewer, bool) {
	user := authUserFrom(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
		return nil, nil, expenseViewer{}, false
	}

	event, err := db.GetEventByEitherId(c.Param("eventId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, nil, expenseViewer{}, false
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return nil, nil, expenseViewer{}, false
	}

	return event, user, newExpenseViewer(user), true
}

// loadExpense fetches the expense named by :expenseId and confirms it belongs to
// this event. A row from another gathering is "not found", not "forbidden" — the
// id is not evidence of anything.
func loadExpense(c *gin.Context, event *models.Event) (*models.Expense, bool) {
	expense, err := db.GetExpenseById(c.Param("expenseId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, false
	}
	if expense == nil || expense.EventId != event.Id {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.ExpenseNotFound})
		return nil, false
	}
	return expense, true
}

// requireEditableExpense is the whole write-path preamble: context, expense,
// permission. Every mutating handler starts with it.
func requireEditableExpense(c *gin.Context) (*models.Event, *models.User, *models.Expense, bool) {
	event, user, viewer, ok := loadExpenseContext(c)
	if !ok {
		return nil, nil, nil, false
	}
	expense, found := loadExpense(c, event)
	if !found {
		return nil, nil, nil, false
	}
	if !viewer.canEdit(expense) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return nil, nil, nil, false
	}
	return event, user, expense, true
}

// gatheringMembers is the pool an expense may name: every account that took part
// in this gathering and is a member or above.
//
// Reuses mentionableUserIds rather than inventing a second definition of "who
// was here" — it already means *everyone who marked availability, RSVP'd, voted
// in a poll, or wrote a comment*, and it is already tested as a table. The
// caller is unioned in so a member who organised the whole thing but never
// RSVP'd can still be named as the payer.
//
// Guests are filtered out: they can read the ledger but never appear in a split.
func gatheringMembers(event *models.Event, caller *models.User) []models.User {
	responsesForEvent, err := db.GetEventResponses(event.Id.Hex())
	if err != nil {
		responsesForEvent = nil // best-effort: a narrower set, never a wider one
	}

	var comments []models.Comment
	if stored, err := db.GetComments(event.Id.Hex()); err == nil {
		comments = stored
	}

	ids := mentionableUserIds(event, responsesForEvent, comments)
	if caller != nil {
		ids[caller.Id] = true
	}

	idList := make([]primitive.ObjectID, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}

	members := make([]models.User, 0, len(idList))
	for _, user := range db.GetUsersByIds(idList) {
		if user.EffectiveRole().CanSeeMembersOnly() {
			members = append(members, user)
		}
	}
	sortMentionables(members)
	return members
}

// memberIndex keys a member pool by id hex for the O(1) membership checks the
// write path needs.
func memberIndex(members []models.User) map[primitive.ObjectID]models.User {
	index := make(map[primitive.ObjectID]models.User, len(members))
	for _, member := range members {
		index[member.Id] = member
	}
	return index
}

// resolveExpenseNames overwrites the denormalized names on a ledger with each
// account's CURRENT DisplayName, exactly as resolveEventDisplayNames does for
// comments and RSVPs. One batched lookup for the whole page.
//
// The stored values are never rewritten — they stay as the fallback for ids that
// no longer resolve, so an expense split with someone who has since left the
// Fellowship still renders under the name they had.
func resolveExpenseNames(expenses []models.Expense) {
	seen := make(map[primitive.ObjectID]bool)
	ids := make([]primitive.ObjectID, 0, len(expenses)*3)
	note := func(id primitive.ObjectID) {
		if id.IsZero() || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}

	for _, expense := range expenses {
		note(expense.PaidBy)
		note(expense.CreatedBy)
		for _, split := range expense.Splits {
			note(split.UserId)
		}
		for _, change := range expense.History {
			note(change.ByUser)
		}
	}

	accounts := db.GetUsersByIds(ids)
	name := func(id primitive.ObjectID, stored string) string {
		if account, ok := accounts[id.Hex()]; ok {
			return account.DisplayName()
		}
		return stored
	}

	for i := range expenses {
		expenses[i].PaidByName = name(expenses[i].PaidBy, expenses[i].PaidByName)
		expenses[i].CreatedByName = name(expenses[i].CreatedBy, expenses[i].CreatedByName)
		for j := range expenses[i].Splits {
			expenses[i].Splits[j].Name = name(expenses[i].Splits[j].UserId, expenses[i].Splits[j].Name)
		}
		for j := range expenses[i].History {
			expenses[i].History[j].ByName = name(expenses[i].History[j].ByUser, expenses[i].History[j].ByName)
		}
	}
}

// @Summary Lists a gathering's expenses, newest first
// @Description Readable by anyone signed in who can see the gathering, guests included. Each row carries a computed `canEdit` for the calling user.
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200 {array} models.Expense
// @Router /events/{eventId}/expenses [get]
func getExpenses(c *gin.Context) {
	event, _, viewer, ok := loadExpenseContext(c)
	if !ok {
		return
	}

	expenses, err := db.GetExpenses(event.Id.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	resolveExpenseNames(expenses)
	for i := range expenses {
		expenses[i].CanEdit = viewer.canEdit(&expenses[i])
	}

	c.JSON(http.StatusOK, expenses)
}

// @Summary Lists the members an expense may be split between
// @Description Everyone who took part in this gathering and is a member or above, plus the caller. Guests are excluded — they read the ledger but never share an expense — and are refused this endpoint outright.
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200 {array} models.User
// @Router /events/{eventId}/expenses/participants [get]
func getExpenseParticipants(c *gin.Context) {
	event, user, viewer, ok := loadExpenseContext(c)
	if !ok {
		return
	}
	if !viewer.canCreate() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	members := gatheringMembers(event, user)
	slim := make([]*models.User, 0, len(members))
	for _, member := range members {
		slim = append(slim, slimUserForDisplay(member))
	}

	c.JSON(http.StatusOK, slim)
}

// expensePayload is the create/edit body. Pointers on every field that has a
// meaningful zero: an amount of 0 and a blank description both have to be
// distinguishable from "not sent", or an edit could not clear a description and
// a zero amount would slip past validation as an absent one.
type expensePayload struct {
	Date        *int64  `json:"date"` // unix milliseconds, as JS produces
	Title       *string `json:"title"`
	Description *string `json:"description"`
	AmountCents *int64  `json:"amountCents"`
	PaidBy      *string `json:"paidBy"`
	SplitMode   *string `json:"splitMode"`
	// Participants drives an even split; Splits carries typed per-person
	// amounts. Exactly one is used, chosen by SplitMode.
	Participants []string `json:"participants"`
	Splits       []struct {
		UserId      string `json:"userId"`
		AmountCents int64  `json:"amountCents"`
	} `json:"splits"`
}

// resolvedExpense is a validated payload: everything checked, names filled in,
// shares resolved to cents that sum to the total.
type resolvedExpense struct {
	Date        primitive.DateTime
	Title       string
	Description string
	AmountCents int64
	PaidBy      primitive.ObjectID
	PaidByName  string
	SplitMode   string
	Splits      []models.ExpenseSplit
}

// resolveExpensePayload validates a create/edit body against the gathering's
// member pool and resolves the split, writing the error response itself and
// returning ok=false on any failure.
//
// This is where the server takes the arithmetic away from the client. An even
// split is computed here from the participant list; a by-amount split is
// accepted only if it names known members and reconciles to the cent.
func resolveExpensePayload(c *gin.Context, payload expensePayload, members []models.User) (resolvedExpense, bool) {
	fail := func(code errs.Code) (resolvedExpense, bool) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: code})
		return resolvedExpense{}, false
	}

	var out resolvedExpense

	title := trimAndTruncate(derefString(payload.Title), maxExpenseTitleLength)
	if title == "" {
		return fail(errs.InvalidTitle)
	}
	out.Title = title
	out.Description = trimAndTruncate(derefString(payload.Description), maxExpenseDescriptionLength)

	if payload.AmountCents == nil {
		return fail(errs.InvalidAmount)
	}
	amount := *payload.AmountCents
	if amount <= 0 || amount > maxExpenseAmountCents {
		return fail(errs.InvalidAmount)
	}
	out.AmountCents = amount

	// A missing date means "today" rather than the zero epoch — the client
	// always sends one, so this only catches a hand-rolled caller, and 1970 in
	// the ledger would be worse than an approximation.
	if payload.Date == nil {
		out.Date = primitive.NewDateTimeFromTime(time.Now())
	} else {
		out.Date = primitive.DateTime(*payload.Date)
	}

	index := memberIndex(members)

	payer, ok := objectIdOrNil(derefString(payload.PaidBy))
	if !ok {
		return fail(errs.NotAParticipant)
	}
	payerAccount, isMember := index[payer]
	if !isMember {
		return fail(errs.NotAParticipant)
	}
	out.PaidBy = payer
	out.PaidByName = payerAccount.DisplayName()

	mode := derefString(payload.SplitMode)
	if mode == "" {
		mode = models.SplitModeEven
	}
	if mode != models.SplitModeEven && mode != models.SplitModeAmount {
		return fail(errs.SplitMismatch)
	}
	out.SplitMode = mode

	// Both modes go through the same membership check, so the two paths cannot
	// drift on who is allowed to appear in a split.
	var participants []primitive.ObjectID
	var typed map[primitive.ObjectID]int64

	if mode == models.SplitModeEven {
		if len(payload.Participants) > maxExpenseParticipants {
			return fail(errs.NoParticipants)
		}
		for _, hex := range payload.Participants {
			id, valid := objectIdOrNil(hex)
			if !valid {
				return fail(errs.NotAParticipant)
			}
			if _, isMember := index[id]; !isMember {
				return fail(errs.NotAParticipant)
			}
			participants = append(participants, id)
		}
	} else {
		if len(payload.Splits) > maxExpenseParticipants {
			return fail(errs.NoParticipants)
		}
		typed = make(map[primitive.ObjectID]int64, len(payload.Splits))
		for _, split := range payload.Splits {
			id, valid := objectIdOrNil(split.UserId)
			if !valid {
				return fail(errs.NotAParticipant)
			}
			if _, isMember := index[id]; !isMember {
				return fail(errs.NotAParticipant)
			}
			if split.AmountCents < 0 {
				return fail(errs.SplitMismatch)
			}
			// A repeated id is a client bug; summing would silently accept it.
			if _, seen := typed[id]; seen {
				return fail(errs.SplitMismatch)
			}
			typed[id] = split.AmountCents
			participants = append(participants, id)
		}
	}

	if len(participants) == 0 {
		return fail(errs.NoParticipants)
	}

	if mode == models.SplitModeEven {
		out.Splits = models.SplitEvenly(amount, participants)
	} else {
		out.Splits = make([]models.ExpenseSplit, 0, len(participants))
		for _, id := range participants {
			out.Splits = append(out.Splits, models.ExpenseSplit{UserId: id, AmountCents: typed[id]})
		}
		// The invariant everything downstream relies on. Checked here because
		// this is the only place it can be: once stored, a ledger that does not
		// reconcile can never be told apart from one that does.
		if models.SumSplits(out.Splits) != amount {
			return fail(errs.SplitMismatch)
		}
	}

	// Names snapshotted from the pool we just validated against.
	for i := range out.Splits {
		member := index[out.Splits[i].UserId]
		out.Splits[i].Name = member.DisplayName()
	}
	// Stable order so a re-save that changed nothing produces an identical
	// document, and the edit diff below stays quiet.
	sort.Slice(out.Splits, func(i, j int) bool {
		return out.Splits[i].UserId.Hex() < out.Splits[j].UserId.Hex()
	})

	return out, true
}

// @Summary Adds an expense to a gathering's ledger
// @Description Members and up only. The server resolves the split itself: an even split is computed from the participant list, and a by-amount split must sum to the total exactly.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{date=int,title=string,description=string,amountCents=int,paidBy=string,splitMode=string,participants=[]string,splits=[]object} true "The expense"
// @Success 200 {object} models.Expense
// @Failure 400 {object} responses.Error "invalid-amount / invalid-title / split-mismatch / no-participants / not-a-participant"
// @Router /events/{eventId}/expenses [post]
func createExpense(c *gin.Context) {
	var payload expensePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	event, user, viewer, ok := loadExpenseContext(c)
	if !ok {
		return
	}
	if !viewer.canCreate() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	resolved, valid := resolveExpensePayload(c, payload, gatheringMembers(event, user))
	if !valid {
		return
	}

	now := primitive.NewDateTimeFromTime(time.Now())
	expense := models.Expense{
		Id:            primitive.NewObjectID(),
		EventId:       event.Id,
		CreatedBy:     user.Id,
		CreatedByName: user.DisplayName(),
		PaidBy:        resolved.PaidBy,
		PaidByName:    resolved.PaidByName,
		Date:          resolved.Date,
		Title:         resolved.Title,
		Description:   resolved.Description,
		AmountCents:   resolved.AmountCents,
		SplitMode:     resolved.SplitMode,
		Splits:        resolved.Splits,
		CreatedAt:     now,
		History:       []models.ExpenseChange{newExpenseChange(user, models.ExpenseActionCreated, now, nil)},
	}

	if err := db.InsertExpense(expense); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	expense.CanEdit = true
	c.JSON(http.StatusOK, expense)
}

// @Summary Edits an expense
// @Description The member who entered it, or any admin. An edit that changes nothing is refused rather than recorded, so the change history stays a record of real changes.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param expenseId path string true "Expense ID"
// @Param payload body object{date=int,title=string,description=string,amountCents=int,paidBy=string,splitMode=string,participants=[]string,splits=[]object} true "The expense"
// @Success 200
// @Failure 400 {object} responses.Error "invalid-amount / invalid-title / split-mismatch / no-changes"
// @Failure 403 {object} responses.Error "not-authorized"
// @Router /events/{eventId}/expenses/{expenseId} [put]
func updateExpense(c *gin.Context) {
	var payload expensePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	event, user, expense, ok := requireEditableExpense(c)
	if !ok {
		return
	}

	resolved, valid := resolveExpensePayload(c, payload, gatheringMembers(event, user))
	if !valid {
		return
	}

	fields, changes := diffExpense(expense, resolved)
	if len(changes) == 0 {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.NoChanges})
		return
	}

	now := primitive.NewDateTimeFromTime(time.Now())
	fields["updatedAt"] = now

	if err := db.UpdateExpense(expense.Id, fields, newExpenseChange(user, models.ExpenseActionEdited, now, changes)); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Deletes an expense
// @Description Soft delete — the row leaves the ledger and the balances, but its change history is retained. The member who entered it, or any admin.
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Param expenseId path string true "Expense ID"
// @Success 200
// @Failure 403 {object} responses.Error "not-authorized"
// @Router /events/{eventId}/expenses/{expenseId} [delete]
func deleteExpense(c *gin.Context) {
	_, user, expense, ok := requireEditableExpense(c)
	if !ok {
		return
	}

	now := primitive.NewDateTimeFromTime(time.Now())
	change := newExpenseChange(user, models.ExpenseActionDeleted, now, nil)
	if err := db.SoftDeleteExpense(expense.Id, user.Id, now, change); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// newExpenseChange stamps one audit-trail entry.
func newExpenseChange(user *models.User, action string, at primitive.DateTime, changes []models.ExpenseFieldChange) models.ExpenseChange {
	return models.ExpenseChange{
		At:      at,
		ByUser:  user.Id,
		ByName:  user.DisplayName(),
		Action:  action,
		Changes: changes,
	}
}

// diffExpense compares a stored expense against a validated payload, returning
// the Mongo `$set` for what actually changed and the human-readable diff for the
// audit trail.
//
// Both come from one pass, so a field can never be written without being
// recorded. Pure — no request, no database — which is what makes the trail
// testable as a table.
func diffExpense(before *models.Expense, after resolvedExpense) (bson.M, []models.ExpenseFieldChange) {
	fields := bson.M{}
	var changes []models.ExpenseFieldChange

	record := func(field, from, to string) {
		changes = append(changes, models.ExpenseFieldChange{Field: field, From: from, To: to})
	}

	if before.Title != after.Title {
		fields["title"] = after.Title
		record("title", before.Title, after.Title)
	}
	if before.Description != after.Description {
		fields["description"] = after.Description
		record("description", before.Description, after.Description)
	}
	if before.AmountCents != after.AmountCents {
		fields["amountCents"] = after.AmountCents
		record("amount", formatCents(before.AmountCents), formatCents(after.AmountCents))
	}
	if before.Date != after.Date {
		fields["date"] = after.Date
		record("date", formatExpenseDate(before.Date), formatExpenseDate(after.Date))
	}
	if before.PaidBy != after.PaidBy {
		fields["paidBy"] = after.PaidBy
		fields["paidByName"] = after.PaidByName
		record("paidBy", before.PaidByName, after.PaidByName)
	}
	if !sameSplits(before.Splits, after.Splits) || before.SplitMode != after.SplitMode {
		fields["splits"] = after.Splits
		fields["splitMode"] = after.SplitMode
		record("split", describeSplits(before.Splits), describeSplits(after.Splits))
	}

	return fields, changes
}

// sameSplits compares two resolved splits. Both sides are sorted by id by the
// time they get here, so this is a straight positional walk.
func sameSplits(before, after []models.ExpenseSplit) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		if before[i].UserId != after[i].UserId || before[i].AmountCents != after[i].AmountCents {
			return false
		}
	}
	return true
}

// describeSplits renders a split for the audit trail: "Jason $47.50, Tom $47.50".
func describeSplits(splits []models.ExpenseSplit) string {
	parts := make([]string, 0, len(splits))
	for _, split := range splits {
		name := split.Name
		if name == "" {
			name = "someone"
		}
		parts = append(parts, name+" "+formatCents(split.AmountCents))
	}
	return strings.Join(parts, ", ")
}

// formatCents renders integer cents as "$142.50". Deliberately not locale-aware:
// this string is frozen into the audit trail, where it has to keep meaning the
// same thing years later regardless of who reads it.
func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return sign + "$" + strconv.FormatInt(cents/100, 10) + "." + pad2(cents%100)
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

// formatExpenseDate renders a stored date for the audit trail, in UTC so the
// recorded string does not depend on where the server happens to be running.
func formatExpenseDate(at primitive.DateTime) string {
	return at.Time().UTC().Format("2 Jan 2006")
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
