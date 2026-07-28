package db_test

import (
	"context"
	"io"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"sirtom/server/db"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// TestMain initializes the DB connection for the whole package (MONGODB_URI, or
// mongodb://localhost by default). Requires a reachable Mongo — CI provides an
// ephemeral one; locally use `docker compose -f compose.dev.yaml up -d mongo`.
func TestMain(m *testing.M) {
	// logger.StdOut/StdErr are nil until logger.Init runs, which only main.go
	// does. Any db function that logs on an error path would nil-deref inside a
	// test rather than returning its error — the same trap routes' TestMain
	// already guards against. Discard the output; we assert on behaviour.
	logger.Init(io.Discard)

	closeConn := db.Init()
	code := m.Run()
	closeConn()
	os.Exit(code)
}

// Distinct test addresses (in the reserved .test TLD) so tests never touch real
// allowlist data and always clean up after themselves.
const (
	crudEmail      = "allowlist-crud@example.test"
	roleEmail      = "allowlist-role@example.test"
	normalizeMixed = "  Allowlist-Normalize@Example.TEST  "
	normalizeLower = "allowlist-normalize@example.test"
)

func TestAllowlistAddCheckRemove(t *testing.T) {
	db.RemoveFromAllowlist(crudEmail)
	defer db.RemoveFromAllowlist(crudEmail)

	if db.IsAllowlisted(crudEmail) {
		t.Fatalf("should not be listed before add")
	}
	if err := db.AddToAllowlist(crudEmail, "tester", models.RoleMember); err != nil {
		t.Fatalf("AddToAllowlist: %v", err)
	}
	if !db.IsAllowlisted(crudEmail) {
		t.Fatalf("should be listed after add")
	}
	if !db.IsAccessAllowed(crudEmail) {
		t.Fatalf("a listed email must be access-allowed")
	}
	if got := db.GetAllowlistRole(crudEmail); got != models.RoleMember {
		t.Fatalf("GetAllowlistRole = %q, want member", got)
	}
	if err := db.RemoveFromAllowlist(crudEmail); err != nil {
		t.Fatalf("RemoveFromAllowlist: %v", err)
	}
	if db.IsAllowlisted(crudEmail) {
		t.Fatalf("should not be listed after remove")
	}
}

func TestAllowlistRoleIdempotentAddAndSet(t *testing.T) {
	db.RemoveFromAllowlist(roleEmail)
	defer db.RemoveFromAllowlist(roleEmail)

	db.AddToAllowlist(roleEmail, "tester", models.RoleGuest)
	// Re-inviting must NOT change an existing entry's role (idempotent invite).
	db.AddToAllowlist(roleEmail, "tester", models.RoleAdmin)
	if got := db.GetAllowlistRole(roleEmail); got != models.RoleGuest {
		t.Fatalf("re-add changed the role: got %q, want guest", got)
	}
	// SetAllowlistRole is the explicit way to change it.
	if err := db.SetAllowlistRole(roleEmail, models.RoleAdmin); err != nil {
		t.Fatalf("SetAllowlistRole: %v", err)
	}
	if got := db.GetAllowlistRole(roleEmail); got != models.RoleAdmin {
		t.Fatalf("after SetAllowlistRole: got %q, want admin", got)
	}
}

func TestAllowlistNormalizesEmail(t *testing.T) {
	db.RemoveFromAllowlist(normalizeMixed)
	defer db.RemoveFromAllowlist(normalizeMixed)

	db.AddToAllowlist(normalizeMixed, "tester", models.RoleMember)
	// Added with mixed case + surrounding whitespace; must match when queried
	// with the normalized (lowercased, trimmed) form.
	if !db.IsAllowlisted(normalizeLower) {
		t.Fatalf("email should match after normalization")
	}
}

func TestGetUsersByEmailsKeyedLowercased(t *testing.T) {
	// Insert a throwaway user, then confirm the batch lookup returns it keyed by
	// the lowercased email (used by getAllowlist to join accounts).
	ctx := context.Background()
	email := "allowlist-user@example.test"
	db.UsersCollection.DeleteMany(ctx, bson.M{"email": email})
	_, err := db.UsersCollection.InsertOne(ctx, models.User{Email: email, FirstName: "Test", Role: models.RoleMember})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer db.UsersCollection.DeleteMany(ctx, bson.M{"email": email})

	got := db.GetUsersByEmails([]string{"Allowlist-User@Example.TEST"})
	if _, ok := got[email]; !ok {
		t.Fatalf("GetUsersByEmails did not return the user keyed by %q; got keys %v", email, keys(got))
	}
}

func keys(m map[string]models.User) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
