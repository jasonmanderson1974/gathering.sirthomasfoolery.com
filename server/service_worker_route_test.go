package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The service worker is served with headers that keep it REPLACEABLE.
//
// This is the regression guard on the one failure mode that makes shipping a
// worker at all a risk here. A browser only ever refetches a worker at the URL
// it registered under, and it decides whether to accept the answer partly on
// the MIME type. Two things therefore have to hold forever:
//
//   - the response must not be cacheable without revalidation, or a client can
//     sit on a stale worker and never see its replacement;
//   - it must be served as JavaScript, because a worker answered with
//     `text/html` fails its update check rather than updating — and this server
//     answers any unmatched path with the SPA shell as text/html, so getting
//     this wrong is not hypothetical.
//
// If both fail together, the client is pinned to whatever build the stale
// worker precached, and deleting the file makes it permanent rather than
// fixing it. deploy/kill-service-worker.js is the way out and it depends
// entirely on this route behaving.
func TestServiceWorkerServedWithReplaceableHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dist := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dist, serviceWorkerFile),
		[]byte("/* worker */\n"),
		0o600,
	); err != nil {
		t.Fatalf("writing fixture worker: %v", err)
	}

	router := gin.New()
	if !registerServiceWorkerRoute(router, dist) {
		t.Fatal("registerServiceWorkerRoute() = false, want true when the file exists")
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+serviceWorkerFile, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /%s = %d, want 200", serviceWorkerFile, rec.Code)
	}

	// no-store would work too, but no-cache is what we want: revalidate, don't
	// refuse to keep. What must never appear is a max-age that lets a client
	// skip the check entirely.
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "javascript") {
		t.Errorf("Content-Type = %q, want a JavaScript type", contentType)
	}
	if strings.Contains(contentType, "text/html") {
		t.Errorf("Content-Type = %q — an HTML worker fails its update check", contentType)
	}

	if got := rec.Header().Get("Service-Worker-Allowed"); got != "/" {
		t.Errorf("Service-Worker-Allowed = %q, want %q", got, "/")
	}
}

// HEAD is registered alongside GET, matching what gin's StaticFile would have
// done for this file before it was pulled out of the static walk.
func TestServiceWorkerAnswersHead(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dist := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dist, serviceWorkerFile), []byte("/* worker */\n"), 0o600,
	); err != nil {
		t.Fatalf("writing fixture worker: %v", err)
	}

	router := gin.New()
	registerServiceWorkerRoute(router, dist)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/"+serviceWorkerFile, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("HEAD /%s = %d, want 200", serviceWorkerFile, rec.Code)
	}
}

// A build made before the worker existed — or one where it was deliberately
// removed — must not register the route at all. Registering a handler for a
// file that isn't there would answer 500s in place of the SPA fallback.
func TestServiceWorkerRouteAbsentWhenNotBuilt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if registerServiceWorkerRoute(router, t.TempDir()) {
		t.Error("registerServiceWorkerRoute() = true, want false with no worker in dist")
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+serviceWorkerFile, nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /%s on a bare engine = %d, want 404", serviceWorkerFile, rec.Code)
	}
}
