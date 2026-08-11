package db

import (
	"context"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sirtom/server/logger"
)

var Client *mongo.Client
var Db *mongo.Database
var EventsCollection *mongo.Collection
var UsersCollection *mongo.Collection
var DailyUserLogCollection *mongo.Collection
var EventResponsesCollection *mongo.Collection
var FoldersCollection *mongo.Collection
var FolderEventsCollection *mongo.Collection
var OtpCodesCollection *mongo.Collection
var AllowlistCollection *mongo.Collection
var CommentsCollection *mongo.Collection
var ChronicleCollection *mongo.Collection
var AvatarsCollection *mongo.Collection
var PersonalListsCollection *mongo.Collection
var PersonalNotesCollection *mongo.Collection
var ExpensesCollection *mongo.Collection
var ExpenseReceiptsCollection *mongo.Collection

func Init() func() {
	// Establish mongodb connection
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost"
	}

	// NOTE: assignment, not `:=`. A short declaration here would create a local
	// that shadows the package-level Client, leaving db.Client nil forever — a
	// nil-pointer trap for the next caller that reaches for it.
	var err error
	Client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		logger.StdErr.Panicln(err)
	}

	// Define mongodb database + collections
	Db = Client.Database("schej-it")
	EventsCollection = Db.Collection("events")
	UsersCollection = Db.Collection("users")
	DailyUserLogCollection = Db.Collection("dailyuserlogs")
	EventResponsesCollection = Db.Collection("eventResponses")
	FoldersCollection = Db.Collection("folders")
	FolderEventsCollection = Db.Collection("folderEvents")
	OtpCodesCollection = Db.Collection("otpCodes")
	AllowlistCollection = Db.Collection("allowlist")
	CommentsCollection = Db.Collection("comments")
	ChronicleCollection = Db.Collection("chronicle")
	AvatarsCollection = Db.Collection("avatars")
	PersonalListsCollection = Db.Collection("personalLists")
	PersonalNotesCollection = Db.Collection("personalNotes")
	ExpensesCollection = Db.Collection("expenses")
	ExpenseReceiptsCollection = Db.Collection("expenseReceipts")

	// Unique per (eventId, startDate) so a gathering occurrence is captured into
	// the Chronicle at most once (belt-and-suspenders against racing scheduler
	// ticks / re-runs; see db/chronicle.go InsertChronicleEntry).
	chronicleIndexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "eventId", Value: 1}, {Key: "startDate", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	ensureIndex("chronicle (eventId, startDate) unique", ChronicleCollection, chronicleIndexModel)

	// Create TTL index so expired OTP docs are auto-deleted
	otpIndexModel := mongo.IndexModel{
		Keys:    bson.M{"expiresAt": 1},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	ensureIndex("otpCodes expiresAt TTL", OtpCodesCollection, otpIndexModel)

	// Unique index on allowlist email so an address can only be listed once
	allowlistIndexModel := mongo.IndexModel{
		Keys:    bson.M{"email": 1},
		Options: options.Index().SetUnique(true),
	}
	ensureIndex("allowlist email unique", AllowlistCollection, allowlistIndexModel)

	// Ensure each user has at most one default folder per kind ("created" /
	// "received"), so EnsureDefaultFolders is race-safe on concurrent loads.
	defaultFolderIndexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "defaultKind", Value: 1}},
		Options: options.Index().SetUnique(true).SetPartialFilterExpression(
			bson.M{"defaultKind": bson.M{"$exists": true}},
		),
	}
	ensureIndex("folders (userId, defaultKind) unique", FoldersCollection, defaultFolderIndexModel)

	// One private-lists document and one private-note document per person per
	// gathering (F19/F20). The uniqueness is what makes the $setOnInsert upserts
	// in db/personal_lists.go and db/personal_notes.go race-safe: two tabs
	// opening the same tab at once can't seed two documents and then write to
	// different ones.
	personalKeys := bson.D{{Key: "userId", Value: 1}, {Key: "eventId", Value: 1}}
	ensureIndex("personalLists (userId, eventId) unique", PersonalListsCollection, mongo.IndexModel{
		Keys:    personalKeys,
		Options: options.Index().SetUnique(true),
	})
	ensureIndex("personalNotes (userId, eventId) unique", PersonalNotesCollection, mongo.IndexModel{
		Keys:    personalKeys,
		Options: options.Index().SetUnique(true),
	})

	// The Settle Up ledger (F22). NOT unique — a gathering has many expenses and
	// an expense many receipts; these exist so the two queries that run on every
	// event page (list the ledger, sweep the photos on hard delete) are indexed
	// rather than collection scans.
	ensureIndex("expenses (eventId, date)", ExpensesCollection, mongo.IndexModel{
		Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "date", Value: -1}},
	})
	ensureIndex("expenseReceipts expenseId", ExpenseReceiptsCollection, mongo.IndexModel{
		Keys: bson.M{"expenseId": 1},
	})
	ensureIndex("expenseReceipts eventId", ExpenseReceiptsCollection, mongo.IndexModel{
		Keys: bson.M{"eventId": 1},
	})

	// The discussion thread and the availability responses (L11). NOT unique,
	// and not here for speed — at 40 comments and 24 responses in production
	// these queries would be fine as collection scans for years.
	//
	// They are here because these two collections were the only ones whose
	// queries had NO declared index anywhere: not in this function, not in a
	// dated script under scripts/, and — checked against the live database —
	// not in production either, which carried nothing but `_id_` on both. L11
	// filed this as drift between a migration script and a fresh install. It
	// wasn't drift; the indexes simply never existed, so the shape of these
	// queries was written down in no place at all. That is what this fixes: the
	// access pattern is now declared next to every other one, and a fresh
	// install, the restored-dump dev box and production finally agree.
	//
	// Keys follow the queries exactly, and the compound orders are load-bearing:
	//   - comments {eventId, createdAt}: GetComments filters on eventId and
	//     sorts by createdAt (db/comments.go) — a sort served by the index
	//     rather than in memory only if createdAt follows eventId.
	//   - comments {threadId}: the reply count and the cascade delete
	//     (CountCommentsInThread / DeleteCommentsInThread).
	//   - eventResponses {eventId, userId}: serves both the by-event fetch
	//     (db/events.go GetEventResponses) and the (eventId, userId) lookups,
	//     the latter via the leftmost-prefix rule.
	//   - eventResponses {userId}: the "events this person responded to" sweep
	//     in routes/user.go. userId is NOT a prefix of the compound above, so
	//     that index cannot serve this query — it needs its own.
	ensureIndex("comments (eventId, createdAt)", CommentsCollection, mongo.IndexModel{
		Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "createdAt", Value: 1}},
	})
	ensureIndex("comments threadId", CommentsCollection, mongo.IndexModel{
		Keys: bson.M{"threadId": 1},
	})
	ensureIndex("eventResponses (eventId, userId)", EventResponsesCollection, mongo.IndexModel{
		Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "userId", Value: 1}},
	})
	ensureIndex("eventResponses userId", EventResponsesCollection, mongo.IndexModel{
		Keys: bson.M{"userId": 1},
	})

	// Return a function to close the connection
	return func() {
		if err := Client.Disconnect(ctx); err != nil {
			logger.StdErr.Println("mongo disconnect:", err)
		}
	}
}

// ensureIndex creates an index and, crucially, says so when it can't.
//
// Each of these indexes enforces an invariant the code above relies on — the
// Chronicle's at-most-once capture, OTP expiry, one allowlist row per address,
// one default folder per kind. Swallowing the error meant the invariant could
// quietly not be in force (existing duplicate data is enough to make creation
// fail permanently) while the code carried on assuming it was.
//
// Deliberately not fatal: refusing to boot over an index would turn a
// recoverable data problem into an outage. Loud is enough — the call sites
// have their own guards.
func ensureIndex(name string, collection *mongo.Collection, model mongo.IndexModel) {
	if _, err := collection.Indexes().CreateOne(context.Background(), model); err != nil {
		logger.StdErr.Printf("index %q could not be created — the guarantee it enforces is NOT in force: %v", name, err)
	}
}

// MongoDB backup / restore commands

// Backup
// mongodump --uri="mongodb://localhost:27017" --db=schej-it

// Restore
// mongorestore --uri="mongodb://localhost:27017" --drop --db=schej-it ./dump
