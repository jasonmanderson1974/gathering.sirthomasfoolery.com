package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/encryption"
	"sirtom/server/logger"
)

// oauthSecretFields are the fields of oAuth2CalendarAuth that hold a secret.
// The expiry and the scope are not secrets and stay readable.
var oauthSecretFields = []string{"accessToken", "refreshToken"}

// EncryptPlaintextOAuthTokens rewrites any Google/Outlook OAuth token still
// stored in the clear as AES-GCM ciphertext (TODO B7). It is idempotent: once
// every value carries the current version marker, it finds nothing and does
// nothing.
//
// It works on raw BSON rather than on models.User on purpose. Decoding into the
// model would hand back plaintext either way — models.EncryptedString passes a
// legacy value straight through — so the model cannot tell you which documents
// still need migrating. Reading the stored strings answers that exactly, and
// rewriting only the two fields it recognises preserves everything else in the
// document byte for byte.
//
// Callers should run this BEFORE the server starts serving. It rewrites the
// whole calendarAccounts field for each affected user, which is safe with no
// concurrent traffic; the map key is `email_TYPE` and emails contain dots, so
// Mongo would read a scoped path like `calendarAccounts.a.b@c.com_google` as
// four levels of nesting rather than one key.
func EncryptPlaintextOAuthTokens() (scanned int, migrated int, err error) {
	ctx := context.Background()

	// Only users that actually hold calendar accounts.
	cursor, err := UsersCollection.Find(ctx, bson.M{
		"calendarAccounts": bson.M{"$exists": true, "$ne": bson.M{}},
	})
	if err != nil {
		logger.StdErr.Println("token encryption sweep: find users:", err)
		return 0, 0, err
	}
	// Close's error is genuinely uninteresting here — the sweep's outcome is
	// what was written, which is already reported. Matches routes/user.go:417.
	defer func() { _ = cursor.Close(ctx) }()

	for cursor.Next(ctx) {
		var doc struct {
			Id               primitive.ObjectID `bson:"_id"`
			CalendarAccounts bson.M             `bson:"calendarAccounts"`
		}
		if decodeErr := cursor.Decode(&doc); decodeErr != nil {
			// One unreadable document must not abort the sweep.
			logger.StdErr.Println("token encryption sweep: decode user:", decodeErr)
			continue
		}
		scanned++

		changed := false
		for key, rawAccount := range doc.CalendarAccounts {
			account, ok := rawAccount.(bson.M)
			if !ok {
				continue
			}
			auth, ok := account["oAuth2CalendarAuth"].(bson.M)
			if !ok {
				// Apple and ICS accounts have no OAuth tokens.
				continue
			}

			for _, field := range oauthSecretFields {
				value, ok := auth[field].(string)
				if !ok || value == "" || encryption.IsCiphertext(value) {
					continue
				}
				ciphertext, encErr := encryption.Encrypt(value)
				if encErr != nil {
					logger.StdErr.Printf("token encryption sweep: user %s account %s: encrypting %s failed: %v",
						doc.Id.Hex(), key, field, encErr)
					continue
				}
				auth[field] = ciphertext
				changed = true
			}
		}

		if !changed {
			continue
		}
		if _, updateErr := UsersCollection.UpdateByID(ctx, doc.Id, bson.M{
			"$set": bson.M{"calendarAccounts": doc.CalendarAccounts},
		}); updateErr != nil {
			logger.StdErr.Printf("token encryption sweep: user %s: update failed: %v", doc.Id.Hex(), updateErr)
			continue
		}
		migrated++
	}

	if cursorErr := cursor.Err(); cursorErr != nil {
		logger.StdErr.Println("token encryption sweep: cursor:", cursorErr)
		return scanned, migrated, cursorErr
	}
	return scanned, migrated, nil
}
