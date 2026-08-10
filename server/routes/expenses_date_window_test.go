package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/errs"
	"sirtom/server/models"
)

// J10: payload.Date was trusted verbatim — unix ms, unbounded. The ledger sorts
// by date descending (db/expenses.go), so an unbounded date is a sort-order
// weapon: a hand-rolled client could stamp year 9999 and pin a row above every
// real one forever, or go negative and bury it beneath them.
//
// Rejected rather than clamped, matching the neighbouring amount cap. See the
// comment in resolveExpensePayload for why silently rewriting the date is the
// worse answer.
func TestResolveExpensePayload_DateWindow(t *testing.T) {
	payerId := primitive.NewObjectID()
	members := []models.User{{Id: payerId, FirstName: "Pat", LastName: "Payer"}}

	payerHex := payerId.Hex()
	title := "Snacks"
	amount := int64(1250)
	mode := models.SplitModeEven

	ms := func(t time.Time) *int64 {
		v := t.UnixMilli()
		return &v
	}

	cases := []struct {
		name    string
		date    *int64
		wantOK  bool
		wantErr errs.Code
	}{
		{"today", ms(time.Now()), true, ""},
		{"last week", ms(time.Now().AddDate(0, 0, -7)), true, ""},
		{"eleven months back", ms(time.Now().AddDate(0, -11, 0)), true, ""},
		{"eleven months ahead", ms(time.Now().AddDate(0, 11, 0)), true, ""},
		{"absent — defaults to now", nil, true, ""},

		{"year 9999", ms(time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)), false, errs.InvalidDate},
		{"two years ahead", ms(time.Now().AddDate(2, 0, 0)), false, errs.InvalidDate},
		{"two years back", ms(time.Now().AddDate(-2, 0, 0)), false, errs.InvalidDate},
		{"negative epoch", ms(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)), false, errs.InvalidDate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

			resolved, ok := resolveExpensePayload(c, expensePayload{
				Date:         tc.date,
				Title:        &title,
				AmountCents:  &amount,
				PaidBy:       &payerHex,
				SplitMode:    &mode,
				Participants: []string{payerHex},
			}, members)

			if ok != tc.wantOK {
				t.Fatalf("accepted = %v, want %v (response: %s)", ok, tc.wantOK, w.Body.String())
			}
			if !tc.wantOK {
				if w.Code != http.StatusBadRequest {
					t.Errorf("status = %d, want 400", w.Code)
				}
				var body struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("unmarshal error body: %v (%s)", err, w.Body.String())
				}
				if body.Error != string(tc.wantErr) {
					t.Errorf("error = %q, want %q", body.Error, tc.wantErr)
				}
				return
			}

			// An accepted date must be stored as sent, not silently adjusted —
			// and an absent one must land on today rather than the zero epoch.
			if tc.date == nil {
				if delta := time.Since(resolved.Date.Time()); delta > time.Minute || delta < -time.Minute {
					t.Errorf("absent date resolved to %v, want approximately now", resolved.Date.Time())
				}
				return
			}
			if got := resolved.Date.Time().UnixMilli(); got != *tc.date {
				t.Errorf("stored date = %d, want %d (it must not be clamped)", got, *tc.date)
			}
		})
	}
}
