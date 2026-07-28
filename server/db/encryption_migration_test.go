package db_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
	"sirtom/server/utils"
)

const migrationTestKey = "0123456789abcdef0123456789abcdef"

// encryptV1CFB reproduces the pre-B6 Encrypt exactly, so the sweep is tested
// against genuine legacy ciphertext rather than an assumption about its shape.
func encryptV1CFB(t *testing.T, text string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(migrationTestKey))
	if err != nil {
		t.Fatalf("v1 cipher: %v", err)
	}
	buf := make([]byte, aes.BlockSize+len(text))
	iv := buf[:aes.BlockSize]
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("v1 iv: %v", err)
	}
	cfb := cipher.NewCFBEncrypter(block, iv) //nolint:staticcheck // SA1019: fixture reproducing v1
	cfb.XORKeyStream(buf[aes.BlockSize:], []byte(text))
	return utils.Encode(buf)
}

// insertUserWithApplePassword stores a user whose Apple account holds the given
// (already-encrypted) password, keyed the way production keys them — an email,
// which contains dots. That is exactly why the sweep rewrites the whole
// calendarAccounts field rather than a dotted path to one password.
func insertUserWithApplePassword(t *testing.T, email, storedPassword string) primitive.ObjectID {
	t.Helper()
	id := primitive.NewObjectID()
	_, err := db.UsersCollection.InsertOne(context.Background(), models.User{
		Id:    id,
		Email: email,
		CalendarAccounts: map[string]models.CalendarAccount{
			email + "_apple": {
				CalendarType: models.AppleCalendarType,
				Email:        email,
				AppleCalendarAuth: &models.AppleCalendarAuth{
					Email:    email,
					Password: storedPassword,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": id})
	})
	return id
}

func storedPassword(t *testing.T, id primitive.ObjectID, key string) string {
	t.Helper()
	var user models.User
	if err := db.UsersCollection.FindOne(context.Background(),
		bson.M{"_id": id}).Decode(&user); err != nil {
		t.Fatalf("reload user: %v", err)
	}
	acc, ok := user.CalendarAccounts[key]
	if !ok || acc.AppleCalendarAuth == nil {
		t.Fatalf("account %q missing after sweep", key)
	}
	return acc.AppleCalendarAuth.Password
}

func TestReEncryptLegacyCalendarSecrets_MigratesV1(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", migrationTestKey)

	const email = "legacy.user@example.test" // dots in the key, deliberately
	const secret = "abcd-efgh-ijkl-mnop"
	id := insertUserWithApplePassword(t, email, encryptV1CFB(t, secret))

	if _, _, err := db.ReEncryptLegacyCalendarSecrets(); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	got := storedPassword(t, id, email+"_apple")
	if !strings.HasPrefix(got, "v2:") {
		t.Errorf("password was not re-encrypted: %q", got)
	}
	// The point of the exercise: the secret must survive intact.
	plain, err := utils.Decrypt(got)
	if err != nil {
		t.Fatalf("re-encrypted value does not decrypt: %v", err)
	}
	if plain != secret {
		t.Errorf("secret changed: got %q, want %q", plain, secret)
	}
}

// Running twice must not corrupt anything — the sweep runs on every boot.
func TestReEncryptLegacyCalendarSecrets_Idempotent(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", migrationTestKey)

	const email = "idempotent@example.test"
	const secret = "keep-me-intact"
	id := insertUserWithApplePassword(t, email, encryptV1CFB(t, secret))

	if _, _, err := db.ReEncryptLegacyCalendarSecrets(); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	first := storedPassword(t, id, email+"_apple")

	_, migrated, err := db.ReEncryptLegacyCalendarSecrets()
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	// Nothing left in v1 form, so the second pass must report no work for this
	// user. (Other tests' fixtures could contribute, so assert on the value.)
	second := storedPassword(t, id, email+"_apple")
	if second != first {
		t.Error("an already-migrated value was rewritten on the second pass")
	}
	plain, err := utils.Decrypt(second)
	if err != nil || plain != secret {
		t.Errorf("secret damaged by the second pass: %q, %v", plain, err)
	}
	_ = migrated
}

// An undecryptable value must be left alone, not overwritten — overwriting
// would destroy any chance of recovering it.
func TestReEncryptLegacyCalendarSecrets_LeavesUndecryptableAlone(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", migrationTestKey)

	const email = "corrupt@example.test"
	const garbage = "!!! not base64 at all !!!"
	id := insertUserWithApplePassword(t, email, garbage)

	if _, _, err := db.ReEncryptLegacyCalendarSecrets(); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if got := storedPassword(t, id, email+"_apple"); got != garbage {
		t.Errorf("undecryptable value was modified: got %q, want it untouched", got)
	}
}

// Non-Apple accounts hold no encrypted secret and must be ignored entirely.
func TestReEncryptLegacyCalendarSecrets_IgnoresOtherProviders(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", migrationTestKey)

	id := primitive.NewObjectID()
	const email = "google.only@example.test"
	_, err := db.UsersCollection.InsertOne(context.Background(), models.User{
		Id:    id,
		Email: email,
		CalendarAccounts: map[string]models.CalendarAccount{
			email + "_google": {
				CalendarType: models.GoogleCalendarType,
				Email:        email,
				OAuth2CalendarAuth: &models.OAuth2CalendarAuth{
					AccessToken:  "plaintext-access",
					RefreshToken: "plaintext-refresh",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": id})
	})

	if _, _, err := db.ReEncryptLegacyCalendarSecrets(); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	var user models.User
	if err := db.UsersCollection.FindOne(context.Background(),
		bson.M{"_id": id}).Decode(&user); err != nil {
		t.Fatalf("reload: %v", err)
	}
	acc := user.CalendarAccounts[email+"_google"]
	if acc.OAuth2CalendarAuth == nil ||
		acc.OAuth2CalendarAuth.RefreshToken != "plaintext-refresh" {
		t.Error("a non-Apple account was modified by the sweep")
	}
}
