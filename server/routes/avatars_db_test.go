package routes

import (
	"context"
	"encoding/json"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// insertAvatarTestUser creates a throwaway account, cleaning up both the user
// row and any avatar the test stores for it.
func insertAvatarTestUser(t *testing.T) *models.User {
	t.Helper()
	user := &models.User{
		Id:        primitive.NewObjectID(),
		Email:     "avatar-test-" + primitive.NewObjectID().Hex() + "@example.test",
		FirstName: "Perpetua",
		LastName:  "Stanhope",
	}
	if _, err := db.UsersCollection.InsertOne(context.Background(), user); err != nil {
		t.Fatalf("inserting test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": user.Id})
		_, _ = db.AvatarsCollection.DeleteOne(context.Background(), bson.M{"_id": user.Id})
	})
	return user
}

// newAvatarGetContext drives getUserAvatar, which reads its target from the
// path rather than from a session.
func newAvatarGetContext(userId string, ifNoneMatch string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/users/"+userId+"/avatar", nil)
	if ifNoneMatch != "" {
		c.Request.Header.Set("If-None-Match", ifNoneMatch)
	}
	c.Params = gin.Params{{Key: "userId", Value: userId}}
	return c, w
}

// storedAvatarUpdatedAt reports the raw state of the user's flag, which is what
// every other surface keys off.
func storedAvatarUpdatedAt(t *testing.T, id primitive.ObjectID) (int64, bool) {
	t.Helper()
	var doc bson.M
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("re-reading test user: %v", err)
	}
	raw, present := doc["avatarUpdatedAt"]
	if !present {
		return 0, false
	}
	stamp, ok := raw.(primitive.DateTime)
	if !ok {
		t.Fatalf("avatarUpdatedAt is stored as %T, want primitive.DateTime", raw)
	}
	return int64(stamp), true
}

// TestAvatarRoundTrip walks the whole lifecycle in one test, because the
// interesting assertions are about the relationships between the steps — the
// flag the upload sets is the ETag the GET serves is the validator the 304
// honours.
func TestAvatarRoundTrip(t *testing.T) {
	requireDB(t)
	user := insertAvatarTestUser(t)

	// --- upload
	body, err := json.Marshal(map[string]string{
		"image": dataURL("image/png", encodePNG(t, 400, 400, color.RGBA{R: 200, G: 30, B: 90, A: 255})),
	})
	if err != nil {
		t.Fatalf("building the upload body: %v", err)
	}
	c, w := newAdminTestContext(string(body), user)
	updateAvatar(c)
	if w.Code != http.StatusOK {
		t.Fatalf("uploading an avatar: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	// primitive.DateTime serializes as an RFC 3339 string, the same as every
	// other timestamp this API returns — that string is what the frontend
	// hangs on the serving URL as `?v=`.
	var uploaded struct {
		AvatarUpdatedAt time.Time `json:"avatarUpdatedAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decoding the upload response: %v", err)
	}
	if uploaded.AvatarUpdatedAt.IsZero() {
		t.Fatal("upload returned no avatarUpdatedAt — the client has nothing to cache-bust with")
	}

	stored, present := storedAvatarUpdatedAt(t, user.Id)
	if !present || stored != uploaded.AvatarUpdatedAt.UnixMilli() {
		t.Errorf("user.avatarUpdatedAt = %d (present=%v), want the %d the upload reported — every other surface reads the flag, not the response",
			stored, present, uploaded.AvatarUpdatedAt.UnixMilli())
	}

	// --- serve
	c, w = newAvatarGetContext(user.Id.Hex(), "")
	getUserAvatar(c)
	if w.Code != http.StatusOK {
		t.Fatalf("fetching the avatar: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != avatarContentType {
		t.Errorf("Content-Type = %q, want %q — the pipeline re-encodes everything to JPEG", got, avatarContentType)
	}
	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("Cache-Control = %q, want an immutable directive", got)
	}
	// The route requires a session, so the response must never be storable by a
	// shared cache — Cloudflare would otherwise serve one member's photo to the
	// next caller of the same URL, session or not.
	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "private") || strings.Contains(got, "public") {
		t.Errorf("Cache-Control = %q, want `private` on an authenticated response", got)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the avatar response")
	}
	// What comes back must be the canonical re-encode, not the PNG that went in.
	if img := decodeResult(t, w.Body.Bytes()); img.Bounds().Dx() != avatarSize {
		t.Errorf("served image is %dpx wide, want %d", img.Bounds().Dx(), avatarSize)
	}

	// --- revalidate
	c, w = newAvatarGetContext(user.Id.Hex(), etag)
	getUserAvatar(c)
	if w.Code != http.StatusNotModified {
		t.Errorf("re-fetching with If-None-Match: got %d, want 304", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("304 carried a %d-byte body; it should carry none", w.Body.Len())
	}

	// A stale validator must still get the bytes.
	c, w = newAvatarGetContext(user.Id.Hex(), `"0"`)
	getUserAvatar(c)
	if w.Code != http.StatusOK {
		t.Errorf("fetching with a stale ETag: got %d, want 200", w.Code)
	}

	// --- remove
	c, w = newAdminTestContext("", user)
	deleteAvatar(c)
	if w.Code != http.StatusOK {
		t.Fatalf("removing the avatar: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if _, present := storedAvatarUpdatedAt(t, user.Id); present {
		t.Error("avatarUpdatedAt survived the delete — the UI would keep asking for a photo that is gone")
	}

	c, w = newAvatarGetContext(user.Id.Hex(), "")
	getUserAvatar(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("fetching a removed avatar: got %d, want 404", w.Code)
	}

	// Removing again is a no-op, not an error — the settings screen fires this
	// without knowing whether a photo is there.
	c, w = newAdminTestContext("", user)
	deleteAvatar(c)
	if w.Code != http.StatusOK {
		t.Errorf("removing an absent avatar: got %d, want 200", w.Code)
	}
}

func TestUpdateAvatar_ReplacesRatherThanAccumulates(t *testing.T) {
	requireDB(t)
	user := insertAvatarTestUser(t)

	upload := func(c color.Color) int64 {
		t.Helper()
		body, _ := json.Marshal(map[string]string{
			"image": dataURL("image/png", encodePNG(t, 300, 300, c)),
		})
		ctx, w := newAdminTestContext(string(body), user)
		updateAvatar(ctx)
		if w.Code != http.StatusOK {
			t.Fatalf("uploading: got %d, want 200 (%s)", w.Code, w.Body.String())
		}
		stamp, present := storedAvatarUpdatedAt(t, user.Id)
		if !present {
			t.Fatal("no avatarUpdatedAt after an upload")
		}
		return stamp
	}

	first := upload(color.RGBA{R: 255, A: 255})
	second := upload(color.RGBA{B: 255, A: 255})

	if second <= first {
		t.Errorf("second upload stamped %d, not later than the first's %d — the URL's ?v= would not change and clients would keep serving the old photo from cache",
			second, first)
	}

	// One avatar per account: the upsert is keyed by user id.
	count, err := db.AvatarsCollection.CountDocuments(context.Background(), bson.M{"_id": user.Id})
	if err != nil {
		t.Fatalf("counting avatars: %v", err)
	}
	if count != 1 {
		t.Errorf("%d avatar documents for one user, want 1", count)
	}
}

func TestGetUserAvatar_NotFound(t *testing.T) {
	requireDB(t)

	for name, userId := range map[string]string{
		"no such user": primitive.NewObjectID().Hex(),
		"malformed id": "not-an-object-id",
	} {
		c, w := newAvatarGetContext(userId, "")
		getUserAvatar(c)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", name, w.Code)
		}
	}
}

func TestUpdateAvatar_RejectsBadPayloads(t *testing.T) {
	requireDB(t)
	user := insertAvatarTestUser(t)

	tests := map[string]struct {
		body string
		want int
	}{
		"not an image":  {`{"image":"` + dataURL("image/png", []byte("nope")) + `"}`, http.StatusBadRequest},
		"missing field": {`{}`, http.StatusBadRequest},
		"not json":      {`this is not json`, http.StatusBadRequest},
		"over the cap": {`{"image":"` + strings.Repeat("A", maxAvatarEncodedBytes+8) + `"}`,
			http.StatusRequestEntityTooLarge},
	}
	for name, tc := range tests {
		c, w := newAdminTestContext(tc.body, user)
		updateAvatar(c)
		if w.Code != tc.want {
			t.Errorf("%s: got %d, want %d (%s)", name, w.Code, tc.want, w.Body.String())
		}
	}

	// Nothing rejected should have been stored.
	if _, present := storedAvatarUpdatedAt(t, user.Id); present {
		t.Error("a rejected upload still set avatarUpdatedAt")
	}
}
