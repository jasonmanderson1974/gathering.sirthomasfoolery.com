package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/middleware"
	"sirtom/server/models"
)

// registerTestLogin adds a helper route that writes userId into the session, so
// tests can obtain a valid session cookie the way a real sign-in would.
func registerTestLogin(r *gin.Engine) {
	r.GET("/test-login/:userId", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("userId", c.Param("userId"))
		s.Save()
		c.Status(http.StatusOK)
	})
}

func loginAs(t *testing.T, r *gin.Engine, userId string) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test-login/"+userId, nil)
	r.ServeHTTP(w, req)
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a session cookie from test-login")
	}
	return cookies[0]
}

func okHandler(c *gin.Context) { c.Status(http.StatusOK) }

func deleteTestUser(userId primitive.ObjectID) {
	db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": userId})
}

// --- AuthRequired: session/allowlist gate -----------------------------------

func TestAuthRequired_NotSignedIn(t *testing.T) {
	r := newTestRouter()
	r.GET("/protected", middleware.AuthRequired(), okHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no session: got %d, want 401", w.Code)
	}
}

// A member who is struck from the roll (session still valid, but email no longer
// allowlisted) must be rejected on the very next request.
func TestAuthRequired_StruckOffMemberRejected(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	userId := primitive.NewObjectID()
	email := "struck-off@example.test"
	if _, err := db.UsersCollection.InsertOne(ctx, models.User{Id: userId, Email: email, Role: models.RoleMember}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer deleteTestUser(userId)

	// Ensure the allowlist is non-empty (so IsAccessAllowed doesn't fail open on
	// an empty list) but does NOT contain the struck-off member.
	sentinel := "roll-sentinel@example.test"
	db.AddToAllowlist(sentinel, "tester", models.RoleMember)
	defer db.RemoveFromAllowlist(sentinel)
	db.RemoveFromAllowlist(email)

	r := newTestRouter()
	registerTestLogin(r)
	r.GET("/protected", middleware.AuthRequired(), okHandler)

	cookie := loginAs(t, r, userId.Hex())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("struck-off member: got %d, want 401 (body: %s)", w.Code, w.Body.String())
	}
}

// A member whose email is on the allowlist passes the gate.
func TestAuthRequired_AllowlistedMemberPasses(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	userId := primitive.NewObjectID()
	email := "allowed-member@example.test"
	if _, err := db.UsersCollection.InsertOne(ctx, models.User{Id: userId, Email: email, Role: models.RoleMember}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer deleteTestUser(userId)

	db.AddToAllowlist(email, "tester", models.RoleMember)
	defer db.RemoveFromAllowlist(email)

	r := newTestRouter()
	registerTestLogin(r)
	r.GET("/protected", middleware.AuthRequired(), okHandler)

	cookie := loginAs(t, r, userId.Hex())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("allowlisted member: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

// --- CanInviteRequired: guest gate (no DB needed) ---------------------------

func TestCanInviteRequired_GuestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("authUser", &models.User{Role: models.RoleGuest})

	middleware.CanInviteRequired()(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("guest through CanInviteRequired: got %d, want 403", w.Code)
	}
	if !c.IsAborted() {
		t.Fatal("guest request should be aborted")
	}
}

func TestCanInviteRequired_MemberAllowed(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("authUser", &models.User{Role: models.RoleMember})

	middleware.CanInviteRequired()(c)

	if c.IsAborted() {
		t.Fatal("member should pass CanInviteRequired")
	}
}

// --- Guest cannot create events (handler-level role check) -------------------

func TestCreateEvent_GuestForbidden(t *testing.T) {
	requireDB(t)
	// Since E3, createEvent runs behind AuthRequired and reads its owner from
	// the context, so drive it through the real middleware chain — registering
	// the bare handler would test a configuration that no longer exists.
	userId := insertTestUser(t, models.RoleGuest, "guest-creator@example.test")

	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events", middleware.AuthRequired(), createEvent)

	cookie := loginAs(t, r, userId.Hex())
	w := httptest.NewRecorder()
	// primitive.DateTime unmarshals from an RFC3339 string (not epoch-ms). The
	// guest role check fires before the event is ever built, so the body just
	// needs to bind.
	body := `{"name":"x","duration":1,"dates":["2025-01-01T00:00:00Z"],"type":"dow"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("guest creating an event: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
}
