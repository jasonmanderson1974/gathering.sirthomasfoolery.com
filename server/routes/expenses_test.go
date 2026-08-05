package routes

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

// DB-free tests for the Settle Up ledger (F22): the permission rule, the audit
// diff, and the money/date formatting frozen into the trail. These are the
// pieces that must be right regardless of what Mongo does with them.

func expenseUser(id primitive.ObjectID) *models.User {
	return &models.User{Id: id, FirstName: "Test", LastName: "Member"}
}

// --- expenseViewer ---------------------------------------------------------

func TestExpenseViewer_CanCreate(t *testing.T) {
	cases := map[models.Role]bool{
		models.RoleSuperAdmin: true,
		models.RoleAdmin:      true,
		models.RoleMember:     true,
		models.RoleGuest:      false,
		"":                    true, // legacy accounts normalize to member
	}
	for role, want := range cases {
		user := expenseUser(primitive.NewObjectID())
		user.Role = role
		if got := newExpenseViewer(user).canCreate(); got != want {
			t.Errorf("role %q canCreate() = %v, want %v", role, got, want)
		}
	}
}

func TestExpenseViewer_CanEdit(t *testing.T) {
	author := primitive.NewObjectID()
	other := primitive.NewObjectID()
	expense := &models.Expense{CreatedBy: author, PaidBy: other}

	cases := []struct {
		name string
		id   primitive.ObjectID
		role models.Role
		want bool
	}{
		{"the member who entered it", author, models.RoleMember, true},
		{"another member", other, models.RoleMember, false},
		{"an admin who did not enter it", other, models.RoleAdmin, true},
		{"a super admin", other, models.RoleSuperAdmin, true},
		{"a guest, even as author", author, models.RoleGuest, false},
		// Being the person owed is not the same as owning the record.
		{"the payer, who did not enter it", other, models.RoleMember, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user := expenseUser(c.id)
			user.Role = c.role
			if got := newExpenseViewer(user).canEdit(expense); got != c.want {
				t.Errorf("canEdit() = %v, want %v", got, c.want)
			}
		})
	}
}

// --- formatCents -----------------------------------------------------------

func TestFormatCents(t *testing.T) {
	cases := map[int64]string{
		0:      "$0.00",
		5:      "$0.05",
		50:     "$0.50",
		100:    "$1.00",
		14250:  "$142.50",
		100000: "$1000.00",
		-4750:  "-$47.50",
		-5:     "-$0.05",
	}
	for cents, want := range cases {
		if got := formatCents(cents); got != want {
			t.Errorf("formatCents(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestFormatExpenseDate(t *testing.T) {
	at := primitive.NewDateTimeFromTime(time.Date(2026, 8, 2, 23, 30, 0, 0, time.UTC))
	if got := formatExpenseDate(at); got != "2 Aug 2026" {
		t.Errorf("formatExpenseDate = %q, want %q", got, "2 Aug 2026")
	}
}

// --- diffExpense -----------------------------------------------------------

func sampleExpense() (*models.Expense, resolvedExpense) {
	alice, bob := primitive.NewObjectID(), primitive.NewObjectID()
	if alice.Hex() > bob.Hex() {
		alice, bob = bob, alice
	}
	date := primitive.NewDateTimeFromTime(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))

	splits := []models.ExpenseSplit{
		{UserId: alice, Name: "Alice", AmountCents: 5000},
		{UserId: bob, Name: "Bob", AmountCents: 5000},
	}
	before := &models.Expense{
		Title:       "Dinner",
		Description: "at the club",
		AmountCents: 10000,
		Date:        date,
		PaidBy:      alice,
		PaidByName:  "Alice",
		SplitMode:   models.SplitModeEven,
		Splits:      splits,
	}
	after := resolvedExpense{
		Title:       "Dinner",
		Description: "at the club",
		AmountCents: 10000,
		Date:        date,
		PaidBy:      alice,
		PaidByName:  "Alice",
		SplitMode:   models.SplitModeEven,
		// A distinct slice with equal contents — the diff must compare values,
		// not identity.
		Splits: append([]models.ExpenseSplit(nil), splits...),
	}
	return before, after
}

func TestDiffExpense_NoChange(t *testing.T) {
	before, after := sampleExpense()

	fields, changes := diffExpense(before, after)
	if len(changes) != 0 {
		t.Errorf("re-saving an untouched expense recorded %d change(s): %+v", len(changes), changes)
	}
	if len(fields) != 0 {
		t.Errorf("re-saving an untouched expense would write %v", fields)
	}
}

func TestDiffExpense_RecordsWhatChanged(t *testing.T) {
	before, after := sampleExpense()
	after.AmountCents = 16000
	after.Title = "Dinner and drinks"

	fields, changes := diffExpense(before, after)

	if fields["amountCents"] != int64(16000) {
		t.Errorf("amountCents not written: %v", fields["amountCents"])
	}
	if fields["title"] != "Dinner and drinks" {
		t.Errorf("title not written: %v", fields["title"])
	}
	// Unchanged fields must not be written — an update that touches everything
	// would clobber a concurrent edit to a field this caller never looked at.
	if _, wrote := fields["date"]; wrote {
		t.Error("an unchanged date was written anyway")
	}
	if _, wrote := fields["splits"]; wrote {
		t.Error("an unchanged split was written anyway")
	}

	recorded := map[string][2]string{}
	for _, change := range changes {
		recorded[change.Field] = [2]string{change.From, change.To}
	}
	if got := recorded["amount"]; got != [2]string{"$100.00", "$160.00"} {
		t.Errorf("amount trail = %v, want [$100.00 $160.00]", got)
	}
	if got := recorded["title"]; got != [2]string{"Dinner", "Dinner and drinks"} {
		t.Errorf("title trail = %v", got)
	}
	if len(changes) != 2 {
		t.Errorf("recorded %d changes, want 2: %+v", len(changes), changes)
	}
}

func TestDiffExpense_PayerChangeRecordsNames(t *testing.T) {
	before, after := sampleExpense()
	newPayer := primitive.NewObjectID()
	after.PaidBy = newPayer
	after.PaidByName = "Bob"

	fields, changes := diffExpense(before, after)

	if fields["paidBy"] != newPayer {
		t.Errorf("paidBy not written: %v", fields["paidBy"])
	}
	// The snapshot name must move with the id, or the ledger renders the old
	// name against the new payer until something re-resolves it.
	if fields["paidByName"] != "Bob" {
		t.Errorf("paidByName not written alongside paidBy: %v", fields["paidByName"])
	}
	if len(changes) != 1 || changes[0].Field != "paidBy" {
		t.Fatalf("changes = %+v, want one paidBy entry", changes)
	}
	if changes[0].From != "Alice" || changes[0].To != "Bob" {
		t.Errorf("paidBy trail = %q → %q, want Alice → Bob", changes[0].From, changes[0].To)
	}
}

func TestDiffExpense_SplitChangeIsDescribed(t *testing.T) {
	before, after := sampleExpense()
	after.Splits = []models.ExpenseSplit{
		{UserId: before.Splits[0].UserId, Name: "Alice", AmountCents: 6000},
		{UserId: before.Splits[1].UserId, Name: "Bob", AmountCents: 4000},
	}
	after.SplitMode = models.SplitModeAmount

	_, changes := diffExpense(before, after)
	if len(changes) != 1 || changes[0].Field != "split" {
		t.Fatalf("changes = %+v, want one split entry", changes)
	}
	if changes[0].From != "Alice $50.00, Bob $50.00" {
		t.Errorf("split trail from = %q", changes[0].From)
	}
	if changes[0].To != "Alice $60.00, Bob $40.00" {
		t.Errorf("split trail to = %q", changes[0].To)
	}
}

// A split whose members change but whose amounts happen to match must still
// register — otherwise swapping who owes what would go unrecorded.
func TestDiffExpense_SameAmountsDifferentPeopleIsAChange(t *testing.T) {
	before, after := sampleExpense()
	after.Splits = []models.ExpenseSplit{
		{UserId: primitive.NewObjectID(), Name: "Carol", AmountCents: 5000},
		{UserId: primitive.NewObjectID(), Name: "Dave", AmountCents: 5000},
	}

	_, changes := diffExpense(before, after)
	if len(changes) != 1 || changes[0].Field != "split" {
		t.Fatalf("changes = %+v, want one split entry", changes)
	}
}

func TestSameSplits(t *testing.T) {
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	base := []models.ExpenseSplit{{UserId: a, AmountCents: 100}, {UserId: b, AmountCents: 200}}

	if !sameSplits(base, []models.ExpenseSplit{{UserId: a, AmountCents: 100}, {UserId: b, AmountCents: 200}}) {
		t.Error("identical splits reported as different")
	}
	if sameSplits(base, base[:1]) {
		t.Error("splits of different lengths reported as the same")
	}
	if sameSplits(base, []models.ExpenseSplit{{UserId: a, AmountCents: 100}, {UserId: b, AmountCents: 201}}) {
		t.Error("a changed amount reported as the same")
	}
	// Name is a display snapshot, not part of the split's identity — a rename
	// must not read as an edit to the expense.
	renamed := []models.ExpenseSplit{{UserId: a, Name: "New", AmountCents: 100}, {UserId: b, Name: "Names", AmountCents: 200}}
	if !sameSplits(base, renamed) {
		t.Error("a name change reported as a split change")
	}
}

// --- receipt geometry ------------------------------------------------------

func TestFitWithin(t *testing.T) {
	cases := []struct {
		name         string
		w, h, max    int
		wantW, wantH int
	}{
		{"already small enough", 800, 600, 2000, 800, 600},
		{"exactly at the cap", 2000, 1000, 2000, 2000, 1000},
		{"tall portrait receipt", 3024, 4032, 2000, 1500, 2000},
		{"wide landscape", 4000, 1000, 2000, 2000, 500},
		{"square", 4000, 4000, 2000, 2000, 2000},
		// Degenerate but must not produce a zero-height canvas, which would
		// panic the encoder.
		{"one pixel tall", 6000, 1, 2000, 2000, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotW, gotH := fitWithin(c.w, c.h, c.max)
			if gotW != c.wantW || gotH != c.wantH {
				t.Errorf("fitWithin(%d, %d, %d) = %dx%d, want %dx%d",
					c.w, c.h, c.max, gotW, gotH, c.wantW, c.wantH)
			}
			if gotW > c.max || gotH > c.max {
				t.Errorf("result %dx%d exceeds the cap %d", gotW, gotH, c.max)
			}
			if gotW < 1 || gotH < 1 {
				t.Errorf("result %dx%d has a zero edge", gotW, gotH)
			}
		})
	}
}
