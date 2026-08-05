package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/middleware"
	"sirtom/server/models"
)

// DB-backed tests for the Settle Up ledger (F22).
//
// Two things here are the point and must not be deleted: that the server
// resolves the split itself and refuses one that does not reconcile, and that
// reading is open to guests while writing is not.

func expensesTestRouter() *gin.Engine {
	r := newTestRouter()
	registerTestLogin(r)
	initExpenseRoutes(r.Group("/events", middleware.AuthRequired()))
	return r
}

// insertExpenseTestEvent creates an event whose attendees are the given users,
// recorded as RSVPs — which is what makes them "participants" as far as
// gatheringMembers is concerned.
func insertExpenseTestEvent(t *testing.T, ownerId primitive.ObjectID, attendees ...primitive.ObjectID) primitive.ObjectID {
	t.Helper()

	rsvps := make(map[string]*models.Rsvp, len(attendees))
	for _, id := range attendees {
		rsvps[id.Hex()] = &models.Rsvp{Status: models.RsvpGoing, UserId: id}
	}

	id := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(context.Background(), models.Event{
		Id: id, OwnerId: ownerId, Type: models.SPECIFIC_DATES, Rsvps: rsvps,
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		db.EventsCollection.DeleteOne(ctx, bson.M{"_id": id})
		db.ExpensesCollection.DeleteMany(ctx, bson.M{"eventId": id})
		db.ExpenseReceiptsCollection.DeleteMany(ctx, bson.M{"eventId": id})
	})
	return id
}

// evenSplitBody builds a create/edit payload for an even split.
func evenSplitBody(amountCents int64, payer primitive.ObjectID, participants ...primitive.ObjectID) string {
	hexes := make([]string, 0, len(participants))
	for _, id := range participants {
		hexes = append(hexes, `"`+id.Hex()+`"`)
	}
	return fmt.Sprintf(
		`{"title":"Dinner","description":"at the club","amountCents":%d,"paidBy":"%s","splitMode":"even","participants":[%s],"date":%d}`,
		amountCents, payer.Hex(), joinComma(hexes), 1786000000000,
	)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

func createExpenseFor(t *testing.T, h *gin.Engine, eventId primitive.ObjectID, cookie *http.Cookie, body string) models.Expense {
	t.Helper()
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/expenses", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("create expense: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var expense models.Expense
	if err := json.Unmarshal(w.Body.Bytes(), &expense); err != nil {
		t.Fatalf("decoding the created expense: %v", err)
	}
	return expense
}

func listExpenses(t *testing.T, h *gin.Engine, eventId primitive.ObjectID, cookie *http.Cookie) []models.Expense {
	t.Helper()
	w := do(h, http.MethodGet, "/events/"+eventId.Hex()+"/expenses", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("list expenses: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var expenses []models.Expense
	if err := json.Unmarshal(w.Body.Bytes(), &expenses); err != nil {
		t.Fatalf("decoding the ledger: %v", err)
	}
	return expenses
}

// errorCode pulls the machine-readable code out of an error response.
func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var parsed struct {
		Error string `json:"error"`
	}
	json.Unmarshal(body, &parsed)
	return parsed.Error
}

// --- the split the server resolves ----------------------------------------

func TestExpenses_ServerResolvesTheEvenSplit(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "split-author@example.test")
	second := insertTestUser(t, models.RoleMember, "split-second@example.test")
	third := insertTestUser(t, models.RoleMember, "split-third@example.test")
	eventId := insertExpenseTestEvent(t, author, author, second, third)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())

	// $10.00 across three people cannot divide evenly — the shares must still
	// reconcile to the cent.
	expense := createExpenseFor(t, h, eventId, cookie, evenSplitBody(1000, author, author, second, third))

	if len(expense.Splits) != 3 {
		t.Fatalf("got %d shares, want 3", len(expense.Splits))
	}
	if total := models.SumSplits(expense.Splits); total != 1000 {
		t.Errorf("shares sum to %d, want 1000 — the ledger would never reconcile", total)
	}
	for _, split := range expense.Splits {
		if split.AmountCents != 333 && split.AmountCents != 334 {
			t.Errorf("share of %d is neither 333 nor 334", split.AmountCents)
		}
		if split.Name == "" {
			t.Error("a share was stored without a name snapshot")
		}
	}
	if expense.History[0].Action != models.ExpenseActionCreated {
		t.Errorf("first history entry = %q, want %q", expense.History[0].Action, models.ExpenseActionCreated)
	}
}

func TestExpenses_ByAmountSplitMustReconcile(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "amount-author@example.test")
	second := insertTestUser(t, models.RoleMember, "amount-second@example.test")
	eventId := insertExpenseTestEvent(t, author, author, second)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())

	// $100 total, $60 + $30 claimed. Accepting this would put a permanent
	// phantom $10 into everyone's balances.
	short := fmt.Sprintf(
		`{"title":"Cab","amountCents":10000,"paidBy":"%s","splitMode":"amount","splits":[{"userId":"%s","amountCents":6000},{"userId":"%s","amountCents":3000}]}`,
		author.Hex(), author.Hex(), second.Hex(),
	)
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/expenses", short, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("short split: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if code := errorCode(t, w.Body.Bytes()); code != "split-mismatch" {
		t.Errorf("error = %q, want split-mismatch", code)
	}

	// The same payload, corrected, is accepted and stored verbatim.
	exact := fmt.Sprintf(
		`{"title":"Cab","amountCents":10000,"paidBy":"%s","splitMode":"amount","splits":[{"userId":"%s","amountCents":6000},{"userId":"%s","amountCents":4000}]}`,
		author.Hex(), author.Hex(), second.Hex(),
	)
	expense := createExpenseFor(t, h, eventId, cookie, exact)
	if total := models.SumSplits(expense.Splits); total != 10000 {
		t.Errorf("shares sum to %d, want 10000", total)
	}
}

func TestExpenses_RejectsInvalidAmountsAndTitles(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "invalid-author@example.test")
	eventId := insertExpenseTestEvent(t, author, author)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())

	cases := map[string]struct {
		body string
		code string
	}{
		"zero amount": {
			fmt.Sprintf(`{"title":"Free","amountCents":0,"paidBy":"%s","participants":["%s"]}`, author.Hex(), author.Hex()),
			"invalid-amount",
		},
		"negative amount": {
			fmt.Sprintf(`{"title":"Refund","amountCents":-500,"paidBy":"%s","participants":["%s"]}`, author.Hex(), author.Hex()),
			"invalid-amount",
		},
		"absurd amount": {
			fmt.Sprintf(`{"title":"Yacht","amountCents":99999999999,"paidBy":"%s","participants":["%s"]}`, author.Hex(), author.Hex()),
			"invalid-amount",
		},
		"blank title": {
			fmt.Sprintf(`{"title":"   ","amountCents":500,"paidBy":"%s","participants":["%s"]}`, author.Hex(), author.Hex()),
			"invalid-title",
		},
		"nobody to split with": {
			fmt.Sprintf(`{"title":"Solo","amountCents":500,"paidBy":"%s","participants":[]}`, author.Hex()),
			"no-participants",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/expenses", c.body, cookie)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400 (body: %s)", w.Code, w.Body.String())
			}
			if code := errorCode(t, w.Body.Bytes()); code != c.code {
				t.Errorf("error = %q, want %q", code, c.code)
			}
		})
	}
}

// A guest may be at the gathering but never shares an expense — naming one in a
// split has to be refused, not silently dropped.
func TestExpenses_GuestsCannotAppearInASplit(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "pool-author@example.test")
	guest := insertTestUser(t, models.RoleGuest, "pool-guest@example.test")
	eventId := insertExpenseTestEvent(t, author, author, guest)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())

	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/expenses",
		evenSplitBody(1000, author, author, guest), cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("splitting with a guest: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if code := errorCode(t, w.Body.Bytes()); code != "not-a-participant" {
		t.Errorf("error = %q, want not-a-participant", code)
	}

	// And the participants endpoint never offers them in the first place.
	w = do(h, http.MethodGet, "/events/"+eventId.Hex()+"/expenses/participants", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("participants: got %d, want 200", w.Code)
	}
	var participants []models.User
	json.Unmarshal(w.Body.Bytes(), &participants)
	for _, p := range participants {
		if p.Id == guest {
			t.Error("the participants list offered a guest")
		}
	}
}

// --- who may read and who may write ---------------------------------------

func TestExpenses_GuestsReadButCannotWrite(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "rw-author@example.test")
	guest := insertTestUser(t, models.RoleGuest, "rw-guest@example.test")
	eventId := insertExpenseTestEvent(t, author, author, guest)

	h := expensesTestRouter()
	authorCookie := loginAs(t, h, author.Hex())
	guestCookie := loginAs(t, h, guest.Hex())

	expense := createExpenseFor(t, h, eventId, authorCookie, evenSplitBody(5000, author, author))

	// Reading is open — a guest who was at the dinner may see the bill.
	ledger := listExpenses(t, h, eventId, guestCookie)
	if len(ledger) != 1 {
		t.Fatalf("guest saw %d expenses, want 1", len(ledger))
	}
	if ledger[0].CanEdit {
		t.Error("the ledger told a guest they could edit an expense")
	}

	// Writing is not.
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/expenses",
		evenSplitBody(1000, author, author), guestCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("guest create: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}

	w = do(h, http.MethodPut, "/events/"+eventId.Hex()+"/expenses/"+expense.Id.Hex(),
		evenSplitBody(6000, author, author), guestCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("guest edit: got %d, want 403", w.Code)
	}

	// Nor may they even see the picker.
	w = do(h, http.MethodGet, "/events/"+eventId.Hex()+"/expenses/participants", "", guestCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("guest participants: got %d, want 403", w.Code)
	}
}

func TestExpenses_OwnEntriesOnlyUnlessAdmin(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "own-author@example.test")
	other := insertTestUser(t, models.RoleMember, "own-other@example.test")
	admin := insertTestUser(t, models.RoleAdmin, "own-admin@example.test")
	eventId := insertExpenseTestEvent(t, author, author, other, admin)

	h := expensesTestRouter()
	authorCookie := loginAs(t, h, author.Hex())
	otherCookie := loginAs(t, h, other.Hex())
	adminCookie := loginAs(t, h, admin.Hex())

	expense := createExpenseFor(t, h, eventId, authorCookie, evenSplitBody(9000, author, author, other))
	path := "/events/" + eventId.Hex() + "/expenses/" + expense.Id.Hex()

	// Another member — even one sharing the expense — may not rewrite it.
	w := do(h, http.MethodPut, path, evenSplitBody(100, other, author, other), otherCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("another member editing: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
	w = do(h, http.MethodDelete, path, "", otherCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("another member deleting: got %d, want 403", w.Code)
	}

	// The ledger tells each of them the same thing.
	for _, seen := range listExpenses(t, h, eventId, otherCookie) {
		if seen.CanEdit {
			t.Error("canEdit was true for a member who did not enter the expense")
		}
	}
	for _, seen := range listExpenses(t, h, eventId, adminCookie) {
		if !seen.CanEdit {
			t.Error("canEdit was false for an admin")
		}
	}

	// An admin may.
	w = do(h, http.MethodPut, path, evenSplitBody(12000, author, author, other), adminCookie)
	if w.Code != http.StatusOK {
		t.Errorf("admin editing: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

// --- the audit trail -------------------------------------------------------

func TestExpenses_EditRecordsWhoChangedWhat(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "trail-author@example.test")
	admin := insertTestUser(t, models.RoleAdmin, "trail-admin@example.test")
	eventId := insertExpenseTestEvent(t, author, author, admin)

	h := expensesTestRouter()
	authorCookie := loginAs(t, h, author.Hex())
	adminCookie := loginAs(t, h, admin.Hex())

	expense := createExpenseFor(t, h, eventId, authorCookie, evenSplitBody(14250, author, author))
	path := "/events/" + eventId.Hex() + "/expenses/" + expense.Id.Hex()

	// An edit that changes nothing is refused rather than recorded — a trail
	// full of empty entries is worse than no trail.
	w := do(h, http.MethodPut, path, evenSplitBody(14250, author, author), authorCookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("no-op edit: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if code := errorCode(t, w.Body.Bytes()); code != "no-changes" {
		t.Errorf("error = %q, want no-changes", code)
	}

	// A real edit, by somebody other than the author, is attributed to them.
	w = do(h, http.MethodPut, path, evenSplitBody(16000, author, author), adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("admin edit: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	ledger := listExpenses(t, h, eventId, authorCookie)
	if len(ledger) != 1 {
		t.Fatalf("got %d expenses, want 1", len(ledger))
	}
	history := ledger[0].History
	if len(history) != 2 {
		t.Fatalf("history has %d entries, want 2: %+v", len(history), history)
	}
	if history[1].Action != models.ExpenseActionEdited {
		t.Errorf("second entry action = %q, want edited", history[1].Action)
	}
	if history[1].ByUser != admin {
		t.Error("the edit was not attributed to the admin who made it")
	}
	if len(history[1].Changes) != 2 {
		// amount, and the even split re-derived from it.
		t.Errorf("edit recorded %d field changes, want 2: %+v", len(history[1].Changes), history[1].Changes)
	}
	if ledger[0].UpdatedAt == nil {
		t.Error("updatedAt was not stamped by the edit")
	}
}

func TestExpenses_DeleteIsSoftAndKeepsTheTrail(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "del-author@example.test")
	eventId := insertExpenseTestEvent(t, author, author)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())

	expense := createExpenseFor(t, h, eventId, cookie, evenSplitBody(2500, author, author))

	w := do(h, http.MethodDelete, "/events/"+eventId.Hex()+"/expenses/"+expense.Id.Hex(), "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	// Gone from the ledger, and from the balances that derive from it.
	if ledger := listExpenses(t, h, eventId, cookie); len(ledger) != 0 {
		t.Errorf("deleted expense still in the ledger: %+v", ledger)
	}

	// But still on disk, with its history — which is why the delete is soft.
	var stored models.Expense
	if err := db.ExpensesCollection.FindOne(context.Background(), bson.M{"_id": expense.Id}).Decode(&stored); err != nil {
		t.Fatalf("the deleted expense was removed outright: %v", err)
	}
	if stored.DeletedAt == nil || stored.DeletedBy != author {
		t.Errorf("deletion not stamped: deletedAt=%v deletedBy=%v", stored.DeletedAt, stored.DeletedBy)
	}
	if len(stored.History) != 2 || stored.History[1].Action != models.ExpenseActionDeleted {
		t.Errorf("history = %+v, want a trailing deleted entry", stored.History)
	}

	// A soft-deleted row cannot be reached again by id.
	w = do(h, http.MethodDelete, "/events/"+eventId.Hex()+"/expenses/"+expense.Id.Hex(), "", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("re-deleting: got %d, want 404", w.Code)
	}
}

// --- receipts --------------------------------------------------------------

func TestExpenses_ReceiptRoundTrip(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "receipt-author@example.test")
	guest := insertTestUser(t, models.RoleGuest, "receipt-guest@example.test")
	eventId := insertExpenseTestEvent(t, author, author, guest)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())
	guestCookie := loginAs(t, h, guest.Hex())

	expense := createExpenseFor(t, h, eventId, cookie, evenSplitBody(4200, author, author))
	receiptsPath := "/events/" + eventId.Hex() + "/expenses/" + expense.Id.Hex() + "/receipts"

	// A tall photo, well over the long-edge cap.
	photo := dataURL("image/png", encodePNG(t, 1200, 3000, color.RGBA{R: 200, G: 200, B: 200, A: 255}))
	w := do(h, http.MethodPost, receiptsPath, `{"image":"`+photo+`"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var ref models.ExpenseReceiptRef
	json.Unmarshal(w.Body.Bytes(), &ref)

	if ref.ContentType != "image/jpeg" {
		t.Errorf("stored content type = %q, want image/jpeg — everything is re-encoded", ref.ContentType)
	}
	if ref.Height != receiptMaxEdge {
		t.Errorf("stored height = %d, want %d", ref.Height, receiptMaxEdge)
	}
	if ref.Width != 800 {
		t.Errorf("stored width = %d, want 800 — the aspect ratio must be preserved", ref.Width)
	}

	// The reference is attached to the expense, and the upload is in the trail.
	ledger := listExpenses(t, h, eventId, cookie)
	if len(ledger[0].Receipts) != 1 || ledger[0].Receipts[0].Id != ref.Id {
		t.Fatalf("receipt not attached to the expense: %+v", ledger[0].Receipts)
	}
	last := ledger[0].History[len(ledger[0].History)-1]
	if last.Action != models.ExpenseActionReceiptAdded {
		t.Errorf("last history entry = %q, want receipt-added", last.Action)
	}

	// A guest can look at it — the ledger is theirs to read.
	one := receiptsPath + "/" + ref.Id.Hex()
	w = do(h, http.MethodGet, one, "", guestCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("guest fetching a receipt: got %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("served content type = %q, want image/jpeg", got)
	}
	if w.Body.Len() == 0 {
		t.Error("served an empty receipt")
	}

	// ...but not delete it.
	if w := do(h, http.MethodDelete, one, "", guestCookie); w.Code != http.StatusForbidden {
		t.Errorf("guest deleting a receipt: got %d, want 403", w.Code)
	}

	// The author can, and the bytes go with the reference.
	if w := do(h, http.MethodDelete, one, "", cookie); w.Code != http.StatusOK {
		t.Fatalf("delete receipt: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	stored, err := db.GetExpenseReceipt(ref.Id)
	if err != nil || stored != nil {
		t.Errorf("the receipt bytes outlived the reference: %v, %v", stored, err)
	}
	if w := do(h, http.MethodGet, one, "", cookie); w.Code != http.StatusNotFound {
		t.Errorf("fetching a deleted receipt: got %d, want 404", w.Code)
	}
}

func TestExpenses_ReceiptCap(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "cap-author@example.test")
	eventId := insertExpenseTestEvent(t, author, author)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())

	expense := createExpenseFor(t, h, eventId, cookie, evenSplitBody(1200, author, author))
	receiptsPath := "/events/" + eventId.Hex() + "/expenses/" + expense.Id.Hex() + "/receipts"
	photo := `{"image":"` + dataURL("image/png", encodePNG(t, 40, 40, color.RGBA{B: 255, A: 255})) + `"}`

	for i := 0; i < maxReceiptsPerExpense; i++ {
		if w := do(h, http.MethodPost, receiptsPath, photo, cookie); w.Code != http.StatusOK {
			t.Fatalf("upload %d: got %d, want 200 (body: %s)", i+1, w.Code, w.Body.String())
		}
	}

	w := do(h, http.MethodPost, receiptsPath, photo, cookie)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("upload past the cap: got %d, want 413 (body: %s)", w.Code, w.Body.String())
	}
	if code := errorCode(t, w.Body.Bytes()); code != "too-many-receipts" {
		t.Errorf("error = %q, want too-many-receipts", code)
	}
}

// --- cascade ---------------------------------------------------------------

func TestExpenses_HardDeletingAnEventSweepsTheLedger(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "cascade-author@example.test")
	eventId := insertExpenseTestEvent(t, author, author)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())

	expense := createExpenseFor(t, h, eventId, cookie, evenSplitBody(3300, author, author))
	receiptsPath := "/events/" + eventId.Hex() + "/expenses/" + expense.Id.Hex() + "/receipts"
	photo := `{"image":"` + dataURL("image/png", encodePNG(t, 30, 30, color.RGBA{G: 255, A: 255})) + `"}`
	if w := do(h, http.MethodPost, receiptsPath, photo, cookie); w.Code != http.StatusOK {
		t.Fatalf("upload: got %d, want 200", w.Code)
	}

	if err := db.DeleteExpensesForEvent(eventId); err != nil {
		t.Fatalf("cascade: %v", err)
	}

	ctx := context.Background()
	if n, _ := db.ExpensesCollection.CountDocuments(ctx, bson.M{"eventId": eventId}); n != 0 {
		t.Errorf("%d expenses survived the cascade", n)
	}
	// The receipts are the part worth real storage — stranding them is exactly
	// what the cascade exists to prevent.
	if n, _ := db.ExpenseReceiptsCollection.CountDocuments(ctx, bson.M{"eventId": eventId}); n != 0 {
		t.Errorf("%d receipt images survived the cascade", n)
	}
}

// --- scoping ---------------------------------------------------------------

// An expense id from another gathering must not be reachable through this one's
// URL, even by someone who could edit it in its own context.
func TestExpenses_AreScopedToTheirEvent(t *testing.T) {
	requireDB(t)

	author := insertTestUser(t, models.RoleMember, "scope-author@example.test")
	firstEvent := insertExpenseTestEvent(t, author, author)
	secondEvent := insertExpenseTestEvent(t, author, author)

	h := expensesTestRouter()
	cookie := loginAs(t, h, author.Hex())

	expense := createExpenseFor(t, h, firstEvent, cookie, evenSplitBody(700, author, author))

	w := do(h, http.MethodPut, "/events/"+secondEvent.Hex()+"/expenses/"+expense.Id.Hex(),
		evenSplitBody(800, author, author), cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-event edit: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}

	if ledger := listExpenses(t, h, secondEvent, cookie); len(ledger) != 0 {
		t.Errorf("the other gathering's ledger showed %d expenses, want 0", len(ledger))
	}
}
