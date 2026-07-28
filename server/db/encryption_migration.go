package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/utils"
)

// ReEncryptLegacyCalendarSecrets rewrites any stored calendar secret still in
// the pre-B6 AES-CFB format as AES-GCM. It is idempotent: once every value
// carries the current version marker, it finds nothing and does nothing.
//
// Only Apple calendar app-specific passwords are affected — they are the sole
// thing that has ever passed through utils.Encrypt. (Google and Outlook OAuth
// tokens are stored unencrypted; that is a separate problem, tracked as B7.)
//
// Why a sweep rather than re-encrypting where the value is read: the read site
// is services/calendar, which is handed only the credentials themselves, with
// no user id or account key to write back through — and that package
// deliberately does not import db. Threading identity through the calendar
// interface to save one boot-time query would be the wrong trade.
//
// Callers should run this BEFORE the server starts serving. It rewrites the
// whole calendarAccounts field for each affected user, which is safe with no
// concurrent traffic; the map key is `email_TYPE` and emails contain dots, so a
// scoped dotted-path update to a single password is not available.
func ReEncryptLegacyCalendarSecrets() (scanned int, migrated int, err error) {
	ctx := context.Background()

	// Only users that actually hold calendar accounts.
	cursor, err := UsersCollection.Find(ctx, bson.M{
		"calendarAccounts": bson.M{"$exists": true, "$ne": bson.M{}},
	})
	if err != nil {
		logger.StdErr.Println("re-encrypt sweep: find users:", err)
		return 0, 0, err
	}
	// Close's error is genuinely uninteresting here — the sweep's outcome is
	// what was written, which is already reported. Matches routes/user.go:417.
	defer func() { _ = cursor.Close(ctx) }()

	for cursor.Next(ctx) {
		var user models.User
		if decodeErr := cursor.Decode(&user); decodeErr != nil {
			// One unreadable document must not abort the sweep.
			logger.StdErr.Println("re-encrypt sweep: decode user:", decodeErr)
			continue
		}
		scanned++

		changed := false
		for key, account := range user.CalendarAccounts {
			if account.AppleCalendarAuth == nil {
				continue
			}
			if !utils.IsLegacyCiphertext(account.AppleCalendarAuth.Password) {
				continue
			}

			plain, decErr := utils.Decrypt(account.AppleCalendarAuth.Password)
			if decErr != nil {
				// Leave it alone. An undecryptable value is already broken, and
				// overwriting it would destroy any chance of recovering it.
				logger.StdErr.Printf("re-encrypt sweep: user %s account %s: decrypt failed, left as-is: %v",
					user.Id.Hex(), key, decErr)
				continue
			}
			reEncrypted, encErr := utils.Encrypt(plain)
			if encErr != nil {
				logger.StdErr.Printf("re-encrypt sweep: user %s account %s: encrypt failed: %v",
					user.Id.Hex(), key, encErr)
				continue
			}

			account.AppleCalendarAuth.Password = reEncrypted
			user.CalendarAccounts[key] = account
			changed = true
		}

		if !changed {
			continue
		}
		if _, updateErr := UsersCollection.UpdateByID(ctx, user.Id, bson.M{
			"$set": bson.M{"calendarAccounts": user.CalendarAccounts},
		}); updateErr != nil {
			logger.StdErr.Printf("re-encrypt sweep: user %s: update failed: %v", user.Id.Hex(), updateErr)
			continue
		}
		migrated++
	}

	if cursorErr := cursor.Err(); cursorErr != nil {
		logger.StdErr.Println("re-encrypt sweep: cursor:", cursorErr)
		return scanned, migrated, cursorErr
	}
	return scanned, migrated, nil
}
