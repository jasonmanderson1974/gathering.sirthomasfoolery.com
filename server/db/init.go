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
	ChronicleCollection.Indexes().CreateOne(context.Background(), chronicleIndexModel)

	// Create TTL index so expired OTP docs are auto-deleted
	otpIndexModel := mongo.IndexModel{
		Keys:    bson.M{"expiresAt": 1},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	OtpCodesCollection.Indexes().CreateOne(context.Background(), otpIndexModel)

	// Unique index on allowlist email so an address can only be listed once
	allowlistIndexModel := mongo.IndexModel{
		Keys:    bson.M{"email": 1},
		Options: options.Index().SetUnique(true),
	}
	AllowlistCollection.Indexes().CreateOne(context.Background(), allowlistIndexModel)

	// Ensure each user has at most one default folder per kind ("created" /
	// "received"), so EnsureDefaultFolders is race-safe on concurrent loads.
	defaultFolderIndexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}, {Key: "defaultKind", Value: 1}},
		Options: options.Index().SetUnique(true).SetPartialFilterExpression(
			bson.M{"defaultKind": bson.M{"$exists": true}},
		),
	}
	FoldersCollection.Indexes().CreateOne(context.Background(), defaultFolderIndexModel)

	// Return a function to close the connection
	return func() {
		Client.Disconnect(ctx)
	}
}

// MongoDB backup / restore commands

// Backup
// mongodump --uri="mongodb://localhost:27017" --db=schej-it

// Restore
// mongorestore --uri="mongodb://localhost:27017" --drop --db=schej-it ./dump
