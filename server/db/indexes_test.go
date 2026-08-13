package db_test

import (
	"context"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"sirtom/server/db"
)

// Every index db.Init declares actually exists after Init has run.
//
// WHY THIS EXISTS (TODO3 L11): `comments` and `eventResponses` were queried on
// fields that had no index anywhere — not in db/init.go, not in a dated script
// under server/scripts/, and not in the live database, which carried nothing but
// `_id_` on both. Nothing failed, because nothing checks: an unindexed query
// returns exactly the right answer, just by reading every document. The access
// pattern was recorded in no place at all, so there was nothing to drift *from*
// and nothing to notice was missing.
//
// This turns the declaration into an assertion. Deleting an ensureIndex call, or
// typo'ing a key name, or reversing a compound order now fails a test instead of
// silently costing a collection scan.
//
// It asserts the KEYS, not the index names: Mongo derives a default name from
// the keys, and pinning names here would fail on any pre-existing index that has
// the same keys under a different name — which is a naming difference, not a
// missing index.
func TestInitCreatesDeclaredIndexes(t *testing.T) {
	cases := []struct {
		collection *mongo.Collection
		name       string // what it's for, for the failure message
		keys       bson.D
	}{
		// The two L11 added, and the reason this test exists.
		{db.CommentsCollection, "comments: thread listing, sorted", bson.D{{Key: "eventId", Value: 1}, {Key: "createdAt", Value: 1}}},
		{db.CommentsCollection, "comments: reply count + cascade delete", bson.D{{Key: "threadId", Value: 1}}},
		{db.EventResponsesCollection, "eventResponses: by event, and by (event, user)", bson.D{{Key: "eventId", Value: 1}, {Key: "userId", Value: 1}}},
		{db.EventResponsesCollection, "eventResponses: events a person responded to", bson.D{{Key: "userId", Value: 1}}},

		// The ones that were already declared. Included so this test covers the
		// whole of ensureIndex rather than only the newest entries — several of
		// these enforce an invariant the code relies on, and a silently absent
		// unique index is worse than a silently absent one for speed.
		{db.ChronicleCollection, "chronicle: at-most-once capture", bson.D{{Key: "eventId", Value: 1}, {Key: "startDate", Value: 1}}},
		{db.OtpCodesCollection, "otpCodes: TTL expiry", bson.D{{Key: "expiresAt", Value: 1}}},
		{db.AllowlistCollection, "allowlist: one row per address", bson.D{{Key: "email", Value: 1}}},
		{db.FoldersCollection, "folders: one default per kind", bson.D{{Key: "userId", Value: 1}, {Key: "defaultKind", Value: 1}}},
		{db.PersonalListsCollection, "personalLists: one per person per gathering", bson.D{{Key: "userId", Value: 1}, {Key: "eventId", Value: 1}}},
		{db.PersonalNotesCollection, "personalNotes: one per person per gathering", bson.D{{Key: "userId", Value: 1}, {Key: "eventId", Value: 1}}},
		{db.ExpensesCollection, "expenses: the ledger listing", bson.D{{Key: "eventId", Value: 1}, {Key: "date", Value: -1}}},
		{db.ExpenseReceiptsCollection, "expenseReceipts: by expense", bson.D{{Key: "expenseId", Value: 1}}},
		{db.ExpenseReceiptsCollection, "expenseReceipts: photo sweep on delete", bson.D{{Key: "eventId", Value: 1}}},

		// O4. These two are not for speed — they ARE the idempotency guarantee.
		// db.InsertWithClientId's whole duplicate-key arm is unreachable without
		// them, so a replayed create would quietly insert a second row: a
		// duplicate comment, or a double-booked expense.
		{db.CommentsCollection, "comments: one per clientId (idempotent create)", bson.D{{Key: "eventId", Value: 1}, {Key: "clientId", Value: 1}}},
		{db.ExpensesCollection, "expenses: one per clientId (idempotent create)", bson.D{{Key: "eventId", Value: 1}, {Key: "clientId", Value: 1}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := indexKeys(tc.collection)
			if err != nil {
				t.Fatalf("listing indexes on %s: %v", tc.collection.Name(), err)
			}
			want := keyString(tc.keys)
			for _, k := range got {
				if k == want {
					return
				}
			}
			t.Errorf("no index with keys %s on %q — the query it supports is a collection scan\n  have: %v",
				want, tc.collection.Name(), got)
		})
	}
}

// indexKeys returns every index's key spec on a collection, normalized to a
// comparable string.
func indexKeys(c *mongo.Collection) ([]string, error) {
	cursor, err := c.Indexes().List(context.Background())
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var out []string
	for cursor.Next(context.Background()) {
		// Decoded into bson.D, not bson.M: key ORDER is the whole point of a
		// compound index, and a map would throw it away — {eventId, createdAt}
		// and {createdAt, eventId} are different indexes serving different
		// queries, and only one of them serves ours.
		var idx struct {
			Key bson.D `bson:"key"`
		}
		if err := cursor.Decode(&idx); err != nil {
			return nil, err
		}
		out = append(out, keyString(idx.Key))
	}
	return out, cursor.Err()
}

func keyString(d bson.D) string {
	s := ""
	for i, e := range d {
		if i > 0 {
			s += ","
		}
		// Mongo reports index directions as int32; the models declare them as
		// int. Format the value rather than comparing typed values so the two
		// spellings of "1" compare equal.
		s += fmt.Sprintf("%s:%v", e.Key, e.Value)
	}
	return "{" + s + "}"
}
