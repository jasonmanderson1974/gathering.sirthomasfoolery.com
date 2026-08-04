package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sirtom/server/db"
	"sirtom/server/responses"
)

// The endpoint this replaces was a phantom: nothing served /api/health, so the
// request fell through main.go's NoRoute handler to index.html and answered 200
// no matter what — including with Mongo down. Both the Docker healthcheck and
// the deploy script gated on that, which made "healthy" mean "the process can
// serve a static file".
//
// So the assertion that earns its keep here is the 503 one. A test that only
// proves 200-when-healthy would pass against the phantom too.

func driveHealth(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	getHealth(c)
	return w
}

func decodeHealth(t *testing.T, w *httptest.ResponseRecorder) responses.Health {
	t.Helper()
	var body responses.Health
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding health body %q: %v", w.Body.String(), err)
	}
	return body
}

func TestHealth_OKWhenMongoReachable(t *testing.T) {
	requireDB(t)

	w := driveHealth(t)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
	body := decodeHealth(t, w)
	if body.Status != "ok" || body.Mongo != "ok" {
		t.Errorf("got status=%q mongo=%q, want both %q", body.Status, body.Mongo, "ok")
	}
	if body.Version == "" {
		t.Error("version is empty; it should be the build stamp, or \"dev\" when unstamped")
	}
}

// Points the db layer at an address nothing listens on, so the ping genuinely
// fails. Needs no Mongo of its own, so it runs everywhere — including the CI
// and local runs where MONGODB_URI is unset and the test above skips.
func TestHealth_ServiceUnavailableWhenMongoUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Port 1 on loopback: connection refused immediately rather than a timeout,
	// which keeps the test fast and its failure mode unambiguous. The short
	// server-selection timeout stops the driver retrying for 30s if some
	// environment swallows the refusal instead.
	dead, err := mongo.Connect(ctx, options.Client().
		ApplyURI("mongodb://127.0.0.1:1").
		SetServerSelectionTimeout(500*time.Millisecond))
	if err != nil {
		t.Fatalf("connecting the deliberately-dead client: %v", err)
	}
	defer func() { _ = dead.Disconnect(context.Background()) }()

	original := db.Db
	db.Db = dead.Database("schej-it")
	t.Cleanup(func() { db.Db = original })

	w := driveHealth(t)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d — an unreachable database must not report healthy (body: %s)",
			w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	body := decodeHealth(t, w)
	if body.Status != "unavailable" || body.Mongo != "down" {
		t.Errorf("got status=%q mongo=%q, want %q/%q", body.Status, body.Mongo, "unavailable", "down")
	}
}

// The route must be registered on the /api group, since that is the path the
// deploy script and the cutover checklist poll. Registering it anywhere else
// would still pass the handler tests above while answering 404 in production.
func TestHealth_RegisteredUnderAPI(t *testing.T) {
	router := gin.New()
	InitHealth(router.Group("/api"))

	for _, r := range router.Routes() {
		if r.Method == http.MethodGet && r.Path == "/api/health" {
			return
		}
	}
	t.Fatal("GET /api/health is not registered; the deploy health gate would hit NoRoute")
}
