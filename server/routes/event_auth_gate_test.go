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

// E3 phase 1: every event route requires a session. The ICS feed is the single
// deliberate exception — calendar apps fetch it without cookies.
//
// This drives the REAL InitEvents registration rather than re-listing handlers,
// so a route added later without auth fails here instead of shipping open.

// eventRoutes enumerates every route InitEvents registers, as (method, path).
// Keep in sync with InitEvents — the completeness check below fails if a route
// is registered that isn't listed here.
var eventRoutes = []struct {
	method string
	path   string
	// public marks the deliberate no-auth exception.
	public bool
}{
	{http.MethodGet, "/api/events/abc/ics", true},

	{http.MethodPost, "/api/events", false},
	{http.MethodPost, "/api/events/import", false},
	{http.MethodPut, "/api/events/abc", false},
	{http.MethodGet, "/api/events/abc/ids", false},
	{http.MethodGet, "/api/events/abc", false},
	{http.MethodGet, "/api/events/abc/responses", false},
	{http.MethodPost, "/api/events/abc/response", false},
	{http.MethodDelete, "/api/events/abc/response", false},
	{http.MethodPost, "/api/events/abc/rename-user", false},
	{http.MethodPost, "/api/events/abc/responded", false},
	{http.MethodDelete, "/api/events/abc", false},
	{http.MethodPost, "/api/events/abc/duplicate", false},
	{http.MethodPost, "/api/events/abc/archive", false},
	{http.MethodPost, "/api/events/abc/schedule", false},
	{http.MethodPost, "/api/events/abc/rsvp", false},
	{http.MethodDelete, "/api/events/abc/rsvp", false},
	{http.MethodPost, "/api/events/abc/comments", false},
	{http.MethodPut, "/api/events/abc/comments/c1", false},
	{http.MethodDelete, "/api/events/abc/comments/c1", false},
	{http.MethodPost, "/api/events/abc/comments/c1/thread", false},
	{http.MethodPatch, "/api/events/abc/comments/c1/thread", false},
	{http.MethodDelete, "/api/events/abc/comments/c1/thread", false},
	{http.MethodGet, "/api/events/abc/mentionables", false},
	{http.MethodPost, "/api/events/abc/polls", false},
	{http.MethodDelete, "/api/events/abc/polls/p1", false},
	{http.MethodPost, "/api/events/abc/polls/p1/vote", false},
	{http.MethodPost, "/api/events/abc/lists", false},
	{http.MethodPatch, "/api/events/abc/lists/l1", false},
	{http.MethodDelete, "/api/events/abc/lists/l1", false},
	{http.MethodPost, "/api/events/abc/lists/l1/items", false},
	{http.MethodPut, "/api/events/abc/lists/l1/items/i1", false},
	{http.MethodDelete, "/api/events/abc/lists/l1/items/i1", false},
}

func newEventRouter() *gin.Engine {
	store := cookie.NewStore([]byte("test-session-secret-at-least-32-bytes!"))
	r := gin.New()
	r.Use(sessions.Sessions("session", store))
	InitEvents(r.Group("/api"))
	return r
}

// Every gated route must reject an anonymous caller with 401, before the
// handler runs. DB-free: AuthRequired rejects the missing session first.
func TestEventRoutes_AnonymousGets401(t *testing.T) {
	r := newEventRouter()

	for _, rt := range eventRoutes {
		if rt.public {
			continue
		}
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401 (body: %s)", w.Code, w.Body.String())
			}
		})
	}
}

// The ICS feed must stay reachable without a session — calendar clients poll it
// with no cookies. It 404s here only because the event doesn't exist; the point
// is that it is NOT a 401.
//
// Unlike the 401 test above, this one is NOT DB-free: passing the auth gate is
// the whole point, so the request reaches the handler and the handler queries
// Mongo. Without requireDB it panicked on a nil EventsCollection rather than
// skipping — invisible in CI, which always sets MONGODB_URI, but it broke
// `go test ./routes/` on a machine without Mongo (E12).
func TestEventRoutes_IcsStaysPublic(t *testing.T) {
	requireDB(t)
	r := newEventRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/000000000000000000000000/ics", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Fatal("the ICS feed must not require a session — calendar apps fetch it without cookies")
	}
}

// Guards against a future route being registered outside the authed group: the
// table above must cover exactly what InitEvents registers.
func TestEventRoutes_TableCoversEveryRegisteredRoute(t *testing.T) {
	registered := make(map[string]bool)
	for _, ri := range newEventRouter().Routes() {
		if strings.HasPrefix(ri.Path, "/api/events") {
			registered[ri.Method+" "+ri.Path] = true
		}
	}

	listed := make(map[string]bool)
	for _, rt := range eventRoutes {
		// Normalise the concrete test path back to its param form.
		p := strings.Replace(rt.path, "/abc", "/:eventId", 1)
		p = strings.Replace(p, "/c1", "/:commentId", 1)
		p = strings.Replace(p, "/p1", "/:pollId", 1)
		p = strings.Replace(p, "/l1", "/:listId", 1)
		p = strings.Replace(p, "/i1", "/:itemId", 1)
		listed[rt.method+" "+p] = true
	}

	for route := range registered {
		if !listed[route] {
			t.Errorf("route %q is registered but not covered by the auth-gate table — "+
				"add it (and make sure it belongs in the authed group)", route)
		}
	}
	for route := range listed {
		if !registered[route] {
			t.Errorf("table lists %q but InitEvents does not register it", route)
		}
	}
}
