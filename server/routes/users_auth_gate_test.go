package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// The /users group is mixed: the avatar is club data and requires a session,
// the public profile deliberately does not. Same shape as the event gate
// (event_auth_gate_test.go): drive the REAL InitUsers registration, so a route
// added later on the wrong side of the gate fails here instead of shipping.

var usersRoutes = []struct {
	method string
	path   string
	// public marks the deliberate no-auth exception.
	public bool
}{
	// Reached while signed out, by design: invite and sign-up screens read it
	// before anyone has a session.
	{http.MethodGet, "/api/users/abc", true},

	{http.MethodGet, "/api/users/abc/avatar", false},
}

func newUsersRouter() *gin.Engine {
	store := cookie.NewStore([]byte("test-session-secret-at-least-32-bytes!"))
	r := gin.New()
	r.Use(sessions.Sessions("session", store))
	InitUsers(r.Group("/api"))
	return r
}

// A member's photo must not be fetchable by an anonymous caller holding a user
// id. DB-free: AuthRequired rejects the missing session before the handler runs.
func TestUsersRoutes_AnonymousGets401(t *testing.T) {
	r := newUsersRouter()

	for _, rt := range usersRoutes {
		if rt.public {
			continue
		}
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(rt.method, rt.path, nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401 (body: %s)", w.Code, w.Body.String())
			}
		})
	}
}

// Gating the avatar must not have gated the public profile with it — they share
// a path prefix, so a group-level middleware would have taken both. It 404s here
// only because the id isn't a real account; the point is that it is NOT a 401.
func TestUsersRoutes_PublicProfileStaysPublic(t *testing.T) {
	requireDB(t)
	r := newUsersRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users/000000000000000000000000", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Fatal("the public profile must stay reachable without a session — " +
			"sign-up and invite screens read it before anyone is signed in")
	}
}

// Guards against a future /users route being registered outside the table: the
// table must cover exactly what InitUsers registers, so adding one forces a
// decision about which side of the gate it belongs on.
func TestUsersRoutes_TableCoversEveryRegisteredRoute(t *testing.T) {
	registered := make(map[string]bool)
	for _, ri := range newUsersRouter().Routes() {
		if strings.HasPrefix(ri.Path, "/api/users") {
			registered[ri.Method+" "+ri.Path] = true
		}
	}

	listed := make(map[string]bool)
	for _, rt := range usersRoutes {
		listed[rt.method+" "+strings.Replace(rt.path, "/abc", "/:userId", 1)] = true
	}

	for route := range registered {
		if !listed[route] {
			t.Errorf("route %q is registered but not covered by the users auth-gate table — "+
				"add it (and decide whether it should require a session)", route)
		}
	}
	for route := range listed {
		if !registered[route] {
			t.Errorf("table lists %q but InitUsers does not register it", route)
		}
	}
}
