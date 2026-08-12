package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The Swagger UI is served in dev and NOT in release (TODO3 L12).
//
// Asserted by making a real request through a real router rather than by
// checking the bool alone: the bool says what registerSwagger decided, and the
// request says what the server actually answers. Those are the same thing right
// up until someone registers the route somewhere else too, which is exactly the
// regression worth catching — the point of L12 is that this surface was reachable
// without a session, not that a function returned false.
func TestSwaggerServedOnlyOutsideReleaseMode(t *testing.T) {
	// gin.Mode() is process-global, so restore it. Leaking release mode into the
	// rest of the package's tests would silently change what they exercise.
	original := gin.Mode()
	t.Cleanup(func() { gin.SetMode(original) })

	tests := []struct {
		name       string
		mode       string
		wantMount  bool
		wantStatus int
	}{
		// 404 is what an unmatched route returns on a bare engine. The real
		// server has a NoRoute handler that serves the SPA shell instead, which
		// is the behaviour described in registerSwagger's comment; either way the
		// docs are not served.
		{"release mode serves no docs", gin.ReleaseMode, false, http.StatusNotFound},
		{"debug mode serves the docs", gin.DebugMode, true, http.StatusOK},
		{"test mode serves the docs", gin.TestMode, true, http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(tc.mode)

			router := gin.New()
			if got := registerSwagger(router); got != tc.wantMount {
				t.Errorf("registerSwagger() = %v, want %v", got, tc.wantMount)
			}

			// doc.json rather than index.html: index.html is served by the
			// swagger handler's embedded filesystem, but doc.json is the one that
			// actually enumerates every route and model — the thing L12 is about
			// not publishing.
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
			router.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("GET /swagger/doc.json in %s mode = %d, want %d",
					tc.mode, rec.Code, tc.wantStatus)
			}

			// In release the API surface must not be in the response, whatever
			// the status. A status check alone would pass if some future NoRoute
			// handler answered 200 with the docs. Checked by content rather than
			// by length, because the body here is legitimately non-empty — gin's
			// unmatched-route default is the text "404 page not found".
			if tc.mode == gin.ReleaseMode {
				body := strings.ToLower(rec.Body.String())
				for _, marker := range []string{"swagger", "\"paths\"", "definitions"} {
					if strings.Contains(body, marker) {
						t.Errorf("release mode leaked API documentation: body contains %q\n  body: %s",
							marker, rec.Body.String())
					}
				}
			}
		})
	}
}
