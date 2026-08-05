package models

import (
	"sort"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SplitEvenly divides amountCents across participants so that the shares sum to
// EXACTLY amountCents — which is the whole difficulty. $10 across three people
// is not three shares of $3.33; it is two of $3.33 and one of $3.34, or the
// ledger never reconciles and the sidebar shows a permanent stray cent.
//
// The remainder (amountCents % n) is handed out one cent at a time to the
// participants whose ids sort first. That rule is arbitrary but it must be
// *deterministic*: the same input has to yield the same shares every time, or
// re-saving an expense without touching it would produce a spurious "split
// changed" entry in the audit trail, and the client's preview would disagree
// with what the server stored.
//
// Duplicate ids are collapsed — a participant list that names someone twice
// means one share, not two. Returns nil for an empty participant list or a
// negative amount; callers validate both before reaching here (Go's % keeps the
// sign of the dividend, so a negative amount would distribute negative cents).
//
// Names are left blank: this function knows about arithmetic, not accounts. The
// route fills them in from the resolved users.
func SplitEvenly(amountCents int64, participants []primitive.ObjectID) []ExpenseSplit {
	ids := dedupeIds(participants)
	if len(ids) == 0 || amountCents < 0 {
		return nil
	}

	// Sorted by hex so the cent-allocation order does not depend on the order
	// the client happened to tick the checkboxes in.
	sort.Slice(ids, func(i, j int) bool { return ids[i].Hex() < ids[j].Hex() })

	n := int64(len(ids))
	base, remainder := amountCents/n, amountCents%n

	splits := make([]ExpenseSplit, 0, len(ids))
	for i, id := range ids {
		share := base
		if int64(i) < remainder {
			share++
		}
		splits = append(splits, ExpenseSplit{UserId: id, AmountCents: share})
	}
	return splits
}

// SumSplits totals the shares. Used both to validate a client-supplied
// by-amount split against the expense total and to assert SplitEvenly's own
// postcondition in tests.
func SumSplits(splits []ExpenseSplit) int64 {
	var total int64
	for _, split := range splits {
		total += split.AmountCents
	}
	return total
}

// SplitUserIds returns just the participant ids, order preserved.
func SplitUserIds(splits []ExpenseSplit) []primitive.ObjectID {
	ids := make([]primitive.ObjectID, 0, len(splits))
	for _, split := range splits {
		ids = append(ids, split.UserId)
	}
	return ids
}

// dedupeIds drops repeats and zero ids while preserving first-seen order.
func dedupeIds(ids []primitive.ObjectID) []primitive.ObjectID {
	seen := make(map[primitive.ObjectID]bool, len(ids))
	out := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		if id.IsZero() || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
