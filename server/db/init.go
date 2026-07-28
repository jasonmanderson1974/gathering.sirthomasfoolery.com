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
var FriendRequestsCollection *mongo.Collection
var EventResponsesCollection *mongo.Collection
var FoldersCollection *mongo.Collection
var FolderEventsCollection *mongo.Collection
var OtpCodesCollection *mongo.Collection
var AllowlistCollection *mongo.Collection
var CommentsCollection *mongo.Collection
var ChronicleCollection *mongo.Collection

func Init() func() {
	// Establish mongodb connection
	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost"
	}

	Client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		logger.StdErr.Panicln(err)
	}

	// Define mongodb database + collections
	Db = Client.Database("schej-it")
	EventsCollection = Db.Collection("events")
	UsersCollection = Db.Collection("users")
	DailyUserLogCollection = Db.Collection("dailyuserlogs")
	FriendRequestsCollection = Db.Collection("friendrequests")
	EventResponsesCollection = Db.Collection("eventResponses")
	FoldersCollection = Db.Collection("folders")
	FolderEventsCollection = Db.Collection("folderEvents")
	OtpCodesCollection = Db.Collection("otpCodes")
	AllowlistCollection = Db.Collection("allowlist")
	CommentsCollection = Db.Collection("comments")
	ChronicleCollection = Db.Collection("chronicle")

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
