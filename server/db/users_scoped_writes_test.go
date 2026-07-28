package db_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// B7/A16: addCalendarAccount, getCalendars and RefreshUserTokenIfNecessary each
// change one thing about a member's calendar accounts and used to persist it by
// writing the whole user document back. These pin the guarantee that replaced
// it — the first one is the concurrent edit that the old write reverted.

func insertScopedTestUser(t *testing.T, user models.User) primitive.ObjectID {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", tokenMigrationTestKey)
	if user.Id.IsZero() {
		user.Id = primitive.NewObjectID()
	}
	if _, err := db.UsersCollection.InsertOne(context.Background(), user); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": user.Id}); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
	return user.Id
}

// The lost update: a calendar fetch that began before the member renamed
// themselves must not write the stale name back when it finishes.
func TestSetUserCalendarAccounts_LeavesTheRestOfTheDocumentAlone(t *testing.T) {
	const email = "scoped.write@example.test"
	id := insertScopedTestUser(t, models.User{
		Email:     email,
		FirstName: "Old",
		Phone:     "555-0100",
		CalendarAccounts: map[string]models.CalendarAccount{
			email + "_google": {
				CalendarType:       models.GoogleCalendarType,
				Email:              email,
				OAuth2CalendarAuth: &models.OAuth2CalendarAuth{RefreshToken: "refresh-1"},
			},
		},
	})

	// Someone else renames the member while our caller holds a stale snapshot.
	if _, err := db.UsersCollection.UpdateByID(context.Background(), id, bson.M{
		"$set": bson.M{"firstName": "New"},
	}); err != nil {
		t.Fatalf("concurrent rename: %v", err)
	}

	// The stale snapshot persists its one genuine change.
	stale := map[string]models.CalendarAccount{
		email + "_google": {
			CalendarType:       models.GoogleCalendarType,
			Email:              email,
			OAuth2CalendarAuth: &models.OAuth2CalendarAuth{RefreshToken: "refresh-2"},
		},
	}
	if err := db.SetUserCalendarAccounts(id, stale); err != nil {
		t.Fatalf("SetUserCalendarAccounts: %v", err)
	}

	var got models.User
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&got); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.FirstName != "New" {
		t.Errorf("the concurrent rename was reverted: got %q", got.FirstName)
	}
	if got.Phone != "555-0100" {
		t.Errorf("an untouched field changed: got %q", got.Phone)
	}
	if tok := got.CalendarAccounts[email+"_google"].OAuth2CalendarAuth.RefreshToken; tok != "refresh-2" {
		t.Errorf("the calendar account was not updated: got %q", tok)
	}
}

// The write goes through the model, so the token it stores must be ciphertext —
// this is the path addCalendarAccount and the token refresh both take.
func TestSetUserCalendarAccounts_StoresTokensEncrypted(t *testing.T) {
	const email = "scoped.encrypted@example.test"
	key := email + "_google"
	id := insertScopedTestUser(t, models.User{Email: email})

	if err := db.SetUserCalendarAccounts(id, map[string]models.CalendarAccount{
		key: {
			CalendarType:       models.GoogleCalendarType,
			Email:              email,
			OAuth2CalendarAuth: &models.OAuth2CalendarAuth{RefreshToken: "a-real-refresh-token"},
		},
	}); err != nil {
		t.Fatalf("SetUserCalendarAccounts: %v", err)
	}

	if stored, _ := storedAuth(t, id, key)["refreshToken"].(string); stored == "a-real-refresh-token" {
		t.Error("the refresh token was written in the clear")
	}

	var got models.User
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&got); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if tok := got.CalendarAccounts[key].OAuth2CalendarAuth.RefreshToken; tok != "a-real-refresh-token" {
		t.Errorf("token did not round trip: got %q", tok)
	}
}
