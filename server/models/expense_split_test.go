package models

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ids builds n deterministic, ascending ObjectIDs so a test can reason about
// which participants sort first — SplitEvenly's remainder rule depends on it.
func ids(n int) []primitive.ObjectID {
	out := make([]primitive.ObjectID, 0, n)
	for i := 0; i < n; i++ {
		var id primitive.ObjectID
		id[11] = byte(i + 1)
		out = append(out, id)
	}
	return out
}

func shares(splits []ExpenseSplit) []int64 {
	out := make([]int64, 0, len(splits))
	for _, split := range splits {
		out = append(out, split.AmountCents)
	}
	return out
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSplitEvenly(t *testing.T) {
	cases := []struct {
		name   string
		amount int64
		people int
		want   []int64
	}{
		{"exact division", 14250, 3, []int64{4750, 4750, 4750}},
		{"one cent remainder", 1000, 3, []int64{334, 333, 333}},
		{"n-1 cents remainder", 1001, 3, []int64{334, 334, 333}},
		{"single participant takes everything", 9999, 1, []int64{9999}},
		{"zero amount", 0, 3, []int64{0, 0, 0}},
		{"one cent among four", 1, 4, []int64{1, 0, 0, 0}},
		{"two people odd amount", 501, 2, []int64{251, 250}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitEvenly(c.amount, ids(c.people))
			if !equalInt64s(shares(got), c.want) {
				t.Errorf("SplitEvenly(%d, %d people) = %v, want %v", c.amount, c.people, shares(got), c.want)
			}
			// The postcondition that actually matters: the ledger reconciles.
			if total := SumSplits(got); total != c.amount {
				t.Errorf("shares sum to %d, want %d", total, c.amount)
			}
		})
	}
}

func TestSplitEvenlyIsDeterministicRegardlessOfInputOrder(t *testing.T) {
	people := ids(5)
	forwards := SplitEvenly(1002, people)

	backwards := make([]primitive.ObjectID, len(people))
	for i, id := range people {
		backwards[len(people)-1-i] = id
	}
	reversed := SplitEvenly(1002, backwards)

	if len(forwards) != len(reversed) {
		t.Fatalf("lengths differ: %d vs %d", len(forwards), len(reversed))
	}
	for i := range forwards {
		if forwards[i] != reversed[i] {
			t.Errorf("split %d differs between input orders: %+v vs %+v", i, forwards[i], reversed[i])
		}
	}
}

func TestSplitEvenlyCollapsesDuplicatesAndZeroIds(t *testing.T) {
	people := ids(2)
	withNoise := []primitive.ObjectID{people[0], people[1], people[0], {}}

	got := SplitEvenly(1000, withNoise)
	if len(got) != 2 {
		t.Fatalf("got %d shares, want 2 (duplicate and zero id dropped)", len(got))
	}
	if total := SumSplits(got); total != 1000 {
		t.Errorf("shares sum to %d, want 1000", total)
	}
}

func TestSplitEvenlyRefusesEmptyAndNegative(t *testing.T) {
	if got := SplitEvenly(1000, nil); got != nil {
		t.Errorf("SplitEvenly with no participants = %v, want nil", got)
	}
	if got := SplitEvenly(-500, ids(2)); got != nil {
		t.Errorf("SplitEvenly with a negative amount = %v, want nil", got)
	}
}

func TestSplitUserIds(t *testing.T) {
	people := ids(3)
	got := SplitUserIds(SplitEvenly(300, people))
	if len(got) != 3 {
		t.Fatalf("got %d ids, want 3", len(got))
	}
	for i := range got {
		if got[i] != people[i] {
			t.Errorf("id %d = %s, want %s", i, got[i].Hex(), people[i].Hex())
		}
	}
}
