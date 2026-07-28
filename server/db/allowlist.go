package db

import (
	"context"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// AccessControlEnforced reports whether the invite-only gate is strictly
// enforced (INVITE_ONLY_ENFORCED). When false (default), the gate fails OPEN
// while the allowlist is empty (bootstrap convenience). Set
// INVITE_ONLY_ENFORCED=true in production so that an accidentally-emptied
// allowlist fails CLOSED (deny) rather than opening the site to everyone.
//
// Since E3 every event route requires a session regardless of this flag, so it
// no longer governs the anonymous-write posture — there are no anonymous writes.
// It remains the fail-closed switch for an empty allowlist.
func AccessControlEnforced() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("INVITE_ONLY_ENFORCED"))) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

func normalizeAllowlistEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsAllowlisted returns true if the email is explicitly on the allowlist.
func IsAllowlisted(email string) bool {
	count, err := AllowlistCollection.CountDocuments(
		context.Background(),
		bson.M{"email": normalizeAllowlistEmail(email)},
	)
	return err == nil && count > 0
}

// IsAccessAllowed is the invite-only sign-in gate. The email is allowed if it is
// explicitly allowlisted. When enforcement is NOT enabled, it also fails open
// while the allowlist is empty (bootstrap "open" state so the first sign-in
// isn't locked out). With INVITE_ONLY_ENFORCED=true the empty-list fail-open is
// disabled, so an accidentally-emptied allowlist denies everyone (fail closed).
func IsAccessAllowed(email string) bool {
	if !AccessControlEnforced() {
		total, err := AllowlistCollection.CountDocuments(context.Background(), bson.M{})
		if err == nil && total == 0 {
			return true
		}
	}
	return IsAllowlisted(email)
}

// AddToAllowlist adds an email to the allowlist with the given role (idempotent
// upsert). If the email is already listed, its role is left unchanged — use
// SetAllowlistRole to change an existing entry's role.
func AddToAllowlist(email string, addedBy string, role models.Role) error {
	e := normalizeAllowlistEmail(email)
	_, err := AllowlistCollection.UpdateOne(
		context.Background(),
		bson.M{"email": e},
		bson.M{"$setOnInsert": bson.M{
			"email":   e,
			"addedBy": addedBy,
			"addedAt": primitive.NewDateTimeFromTime(time.Now()),
			"role":    models.NormalizeRole(role),
		}},
		options.Update().SetUpsert(true),
	)
	return err
}

// GetAllowlistRole returns the role recorded on the allowlist for the given
// email, or "" if the email is not listed. Used to seed a new user's role at
// first sign-in.
func GetAllowlistRole(email string) models.Role {
	var entry models.AllowlistEntry
	err := AllowlistCollection.FindOne(
		context.Background(),
		bson.M{"email": normalizeAllowlistEmail(email)},
	).Decode(&entry)
	if err != nil {
		return ""
	}
	return entry.Role
}

// SetAllowlistRole updates the role on an existing allowlist entry.
func SetAllowlistRole(email string, role models.Role) error {
	_, err := AllowlistCollection.UpdateOne(
		context.Background(),
		bson.M{"email": normalizeAllowlistEmail(email)},
		bson.M{"$set": bson.M{"role": models.NormalizeRole(role)}},
	)
	return err
}

// RemoveFromAllowlist removes an email from the allowlist.
func RemoveFromAllowlist(email string) error {
	_, err := AllowlistCollection.DeleteOne(
		context.Background(),
		bson.M{"email": normalizeAllowlistEmail(email)},
	)
	return err
}

// GetAllowlist returns all allowlist entries.
//
// It reports its errors rather than returning an empty slice for them: an empty
// roll and a failed read are very different things, and conflating them showed
// a broken query as "the Fellowship has no members".
func GetAllowlist() ([]models.AllowlistEntry, error) {
	entries := make([]models.AllowlistEntry, 0)
	cursor, err := AllowlistCollection.Find(context.Background(), bson.M{})
	if err != nil {
		logger.StdErr.Println(err)
		return entries, err
	}
	if err := cursor.All(context.Background(), &entries); err != nil {
		logger.StdErr.Println(err)
		return entries, err
	}
	return entries, nil
}
