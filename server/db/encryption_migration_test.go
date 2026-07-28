package db_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/encryption"
	"sirtom/server/models"
)

const tokenMigrationTestKey = "0123456789abcdef0123456789abcdef"

// insertLegacyUser writes a user document the way it looked before B7 — tokens
// in the clear. It has to go in as raw BSON: models.User now encrypts on
// marshal, so the model cannot express the state being migrated from.
//
// The account is keyed the way production keys it, by an email containing dots.
// That is exactly why the sweep rewrites the whole calendarAccounts field
// rather than a dotted path to one token.
func insertLegacyUser(t *testing.T, email string, extraAccountFields bson.M) (primitive.ObjectID, string) {
	t.Helper()
	id := primitive.NewObjectID()
	key := email + "_google"

	account := bson.M{
		"calendarType": string(models.GoogleCalendarType),
		"email":        email,
		"enabled":      true,
		"oAuth2CalendarAuth": bson.M{
			"accessToken":           "plaintext-access-token",
			"refreshToken":          "plaintext-refresh-token",
			"accessTokenExpireDate": primitive.NewDateTimeFromTime(testExpiry()),
			"scope":                 "https://www.googleapis.com/auth/calendar.readonly",
		},
	}
	for k, v := range extraAccountFields {
		account[k] = v
	}

	if _, err := db.UsersCollection.InsertOne(context.Background(), bson.M{
		"_id":              id,
		"email":            email,
		"calendarAccounts": bson.M{key: account},
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": id}); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
	return id, key
}

func testExpiry() time.Time {
	return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
}

// storedAuth returns the raw oAuth2CalendarAuth subdocument, before any
// decryption — what Mongo actually holds.
func storedAuth(t *testing.T, id primitive.ObjectID, key string) bson.M {
	t.Helper()
	var doc struct {
		CalendarAccounts bson.M `bson:"calendarAccounts"`
	}
	if err := db.UsersCollection.FindOne(context.Background(),
		bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("reload user: %v", err)
	}
	account, ok := doc.CalendarAccounts[key].(bson.M)
	if !ok {
		t.Fatalf("account %q missing after sweep: %v", key, doc.CalendarAccounts)
	}
	auth, ok := account["oAuth2CalendarAuth"].(bson.M)
	if !ok {
		t.Fatalf("oAuth2CalendarAuth missing after sweep: %v", account)
	}
	return auth
}

func TestEncryptPlaintextOAuthTokens_MigratesPlaintext(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", tokenMigrationTestKey)

	id, key := insertLegacyUser(t, "legacy.tokens@example.test", nil)

	if _, _, err := db.EncryptPlaintextOAuthTokens(); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	auth := storedAuth(t, id, key)
	for field, want := range map[string]string{
		"accessToken":  "plaintext-access-token",
		"refreshToken": "plaintext-refresh-token",
	} {
		stored, _ := auth[field].(string)
		if !encryption.IsCiphertext(stored) {
			t.Errorf("%s was not encrypted: %q", field, stored)
			continue
		}
		// The point of the exercise: the token must survive intact.
		plain, err := encryption.Decrypt(stored)
		if err != nil {
			t.Errorf("%s does not decrypt: %v", field, err)
			continue
		}
		if plain != want {
			t.Errorf("%s changed: got %q, want %q", field, plain, want)
		}
	}

	// And the whole way back through the model, which is how every caller
	// actually reads it.
	var user models.User
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&user); err != nil {
		t.Fatalf("reload through the model: %v", err)
	}
	if got := user.CalendarAccounts[key].OAuth2CalendarAuth.RefreshToken; got != "plaintext-refresh-token" {
		t.Errorf("model read back %q", got)
	}
}

// The sweep runs on every boot, so a second pass must neither rewrite nor
// double-encrypt an already-migrated value.
func TestEncryptPlaintextOAuthTokens_Idempotent(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", tokenMigrationTestKey)

	id, key := insertLegacyUser(t, "idempotent.tokens@example.test", nil)

	if _, _, err := db.EncryptPlaintextOAuthTokens(); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	first, _ := storedAuth(t, id, key)["refreshToken"].(string)

	if _, _, err := db.EncryptPlaintextOAuthTokens(); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	second, _ := storedAuth(t, id, key)["refreshToken"].(string)

	if second != first {
		t.Error("an already-encrypted token was rewritten on the second pass")
	}
	plain, err := encryption.Decrypt(second)
	if err != nil || plain != "plaintext-refresh-token" {
		t.Errorf("token damaged by the second pass: %q, %v", plain, err)
	}
}

// The sweep rewrites the whole calendarAccounts field, so everything it does
// *not* understand has to come back byte for byte — including fields no Go
// model mentions.
func TestEncryptPlaintextOAuthTokens_PreservesEverythingElse(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", tokenMigrationTestKey)

	id, key := insertLegacyUser(t, "preserve.me@example.test", bson.M{
		"picture": "https://example.test/avatar.png",
		"subCalendars": bson.M{
			"primary": bson.M{"name": "Primary", "enabled": true},
		},
		"someFieldNoModelKnowsAbout": "keep me",
	})

	if _, _, err := db.EncryptPlaintextOAuthTokens(); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	var doc struct {
		CalendarAccounts bson.M `bson:"calendarAccounts"`
	}
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("reload: %v", err)
	}
	account, _ := doc.CalendarAccounts[key].(bson.M)
	if got := account["someFieldNoModelKnowsAbout"]; got != "keep me" {
		t.Errorf("an unmodelled field was lost: %v", got)
	}
	if got := account["picture"]; got != "https://example.test/avatar.png" {
		t.Errorf("picture changed: %v", got)
	}
	if account["enabled"] != true {
		t.Errorf("enabled changed: %v", account["enabled"])
	}
	subs, ok := account["subCalendars"].(bson.M)
	if !ok || subs["primary"] == nil {
		t.Errorf("subCalendars lost: %v", account["subCalendars"])
	}

	auth := storedAuth(t, id, key)
	if auth["scope"] != "https://www.googleapis.com/auth/calendar.readonly" {
		t.Errorf("scope should stay in the clear: %v", auth["scope"])
	}
	expiry, ok := auth["accessTokenExpireDate"].(primitive.DateTime)
	if !ok || !expiry.Time().Equal(testExpiry()) {
		t.Errorf("expiry changed: %v", auth["accessTokenExpireDate"])
	}
}

// Apple and ICS accounts hold no OAuth tokens. The Apple password has its own
// encryption (B6) and must not be touched by this sweep.
func TestEncryptPlaintextOAuthTokens_IgnoresNonOAuthAccounts(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", tokenMigrationTestKey)

	id := primitive.NewObjectID()
	const email = "apple.only@example.test"
	appleKey := email + "_apple"
	applePassword, err := encryption.Encrypt("abcd-efgh-ijkl-mnop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UsersCollection.InsertOne(context.Background(), bson.M{
		"_id":   id,
		"email": email,
		"calendarAccounts": bson.M{appleKey: bson.M{
			"calendarType": string(models.AppleCalendarType),
			"email":        email,
			"appleCalendarAuth": bson.M{
				"email":    email,
				"password": applePassword,
			},
		}},
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": id}); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	if _, _, err := db.EncryptPlaintextOAuthTokens(); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	var doc struct {
		CalendarAccounts bson.M `bson:"calendarAccounts"`
	}
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("reload: %v", err)
	}
	account, _ := doc.CalendarAccounts[appleKey].(bson.M)
	auth, _ := account["appleCalendarAuth"].(bson.M)
	if got, _ := auth["password"].(string); got != applePassword {
		t.Errorf("the Apple password was modified by the OAuth sweep: %q", got)
	}
}

// A token stored in the clear that cannot be encrypted (no usable key) must be
// left as it is rather than blanked. Nothing is lost by waiting for the next
// boot; blanking would disconnect the member's calendar.
func TestEncryptPlaintextOAuthTokens_UnusableKeyLeavesTokensAlone(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", tokenMigrationTestKey)
	id, key := insertLegacyUser(t, "no.key@example.test", nil)

	t.Setenv("ENCRYPTION_KEY", "")
	if _, migrated, err := db.EncryptPlaintextOAuthTokens(); err != nil {
		t.Fatalf("sweep: %v", err)
	} else if migrated > 0 {
		t.Errorf("reported %d migrated with no usable key", migrated)
	}

	if got, _ := storedAuth(t, id, key)["refreshToken"].(string); got != "plaintext-refresh-token" {
		t.Errorf("token was damaged with no usable key: %q", got)
	}
}

// Guards the assumption the sweep is built on: the account subdocuments decode
// as bson.M, not bson.D, so the walk can index them by field name.
func TestEncryptPlaintextOAuthTokens_SubdocumentsDecodeAsMaps(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", tokenMigrationTestKey)
	id, key := insertLegacyUser(t, "shape.check@example.test", nil)

	var doc struct {
		CalendarAccounts bson.M `bson:"calendarAccounts"`
	}
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, ok := doc.CalendarAccounts[key].(bson.M); !ok {
		t.Fatalf("account decoded as %T, not bson.M — the sweep would silently skip it",
			doc.CalendarAccounts[key])
	}
	if !strings.Contains(key, ".") {
		t.Fatal("the fixture key should contain a dot; that is the case the sweep exists to handle")
	}
}
