package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"sirtom/server/db"
	"sirtom/server/logger"
	"sirtom/server/models"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	// logger.StdOut/StdErr are nil until logger.Init runs (main.go does it in
	// production). Any handler error path that logs would otherwise nil-deref
	// inside a test rather than returning its intended status. Discard the
	// output — we assert on responses, not logs.
	logger.Init(io.Discard)
	// DB-backed handler tests need a Mongo connection. mongo.Connect is lazy (no
	// ping), so calling Init is safe even without a running server; the tests that
	// actually touch Mongo gate on MONGODB_URI via requireDB (CI sets it, local
	// devs may not). See event_responses_db_test.go.
	if os.Getenv("MONGODB_URI") != "" {
		db.Init()
		dbReady = true
	}
	os.Exit(m.Run())
}

// newAdminTestContext builds a gin context with a JSON body and an authUser set,
// for driving the /admin handlers directly. These tests exercise the permission
// GUARDS, which all reject before any DB access — so no Mongo is required.
func newAdminTestContext(body string, authUser *models.User) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if authUser != nil {
		c.Set("authUser", authUser)
	}
	return c, w
}

func member() *models.User {
	return &models.User{Email: "member@example.test", Role: models.RoleMember}
}
func admin() *models.User {
	return &models.User{Email: "admin@example.test", Role: models.RoleAdmin}
}

func TestSetMemberRole_MemberForbidden(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"someone@example.test","role":"admin"}`, member())
	setMemberRole(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member changing roles: got %d, want 403", w.Code)
	}
}

func TestSetMemberRole_CannotChangeOwnRole(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"admin@example.test","role":"guest"}`, admin())
	setMemberRole(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("admin changing own role: got %d, want 400", w.Code)
	}
}

func TestSetMemberRole_InvalidRoleRejected(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"someone@example.test","role":"wizard"}`, admin())
	setMemberRole(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid role: got %d, want 400", w.Code)
	}
}

func TestSetMemberRole_CannotGrantSuperAdmin(t *testing.T) {
	// Granting super admin is blocked before the DB super-admin-immutable check
	// via payload.Role.IsSuperAdmin(); but IsKnownRole(superAdmin) is true, so it
	// reaches the immutability guard which 403s. Either way it must be rejected.
	c, w := newAdminTestContext(`{"email":"someone@example.test","role":"superAdmin"}`, admin())
	setMemberRole(c)
	if w.Code == http.StatusOK {
		t.Fatalf("granting super admin must be rejected, got 200")
	}
}

func TestAddAllowlistEmail_MemberCannotGrantAdmin(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"someone@example.test","role":"admin"}`, member())
	addAllowlistEmail(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member inviting an admin: got %d, want 403", w.Code)
	}
}

func TestAddAllowlistEmail_InvalidEmailRejected(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"not-an-email","role":"guest"}`, admin())
	addAllowlistEmail(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid email: got %d, want 400", w.Code)
	}
}

func TestRemoveAllowlistEmail_MemberForbidden(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"someone@example.test"}`, member())
	removeAllowlistEmail(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member removing from roll: got %d, want 403", w.Code)
	}
}
