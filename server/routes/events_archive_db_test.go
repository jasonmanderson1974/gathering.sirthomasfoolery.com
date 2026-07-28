package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// seedEvent inserts an event owned by ownerId (pass primitive.NilObjectID for a
// legacy ownerless one) and returns its ids.
func seedEvent(t *testing.T, shortId string, ownerId primitive.ObjectID) primitive.ObjectID {
	t.Helper()

	eventId := primitive.NewObjectID()
	if _, err := db.EventsCollection.InsertOne(context.Background(), models.Event{
		Id:      eventId,
		ShortId: &shortId,
		Type:    models.DOW,
		OwnerId: ownerId,
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { cleanupEvent(eventId) })
	return eventId
}

func archiveRouter(user *models.User) *gin.Engine {
	r := newTestRouter()
	r.Use(func(c *gin.Context) { c.Set("authUser", user); c.Next() })
	r.POST("/events/:eventId/archive", archiveEvent)
	return r
}

func postArchive(r *gin.Engine, eventId string, archive bool) *httptest.ResponseRecorder {
	body := `{"archive":true}`
	if !archive {
		body = `{"archive":false}`
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events/"+eventId+"/archive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// archiveEvent resolved the id with ObjectIDFromHex directly, so a short id —
// which every other event route accepts — was rejected as malformed. Same sharp
// edge E2 fixed for delete.
func TestArchiveEvent_ByShortId(t *testing.T) {
	requireDB(t)

	owner := &models.User{Id: primitive.NewObjectID()}
	shortId := "e10arc"
	eventId := seedEvent(t, shortId, owner.Id)

	w := postArchive(archiveRouter(owner), shortId, true)
	if w.Code != http.StatusOK {
		t.Fatalf("archive by short id: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	var stored models.Event
	if err := db.EventsCollection.FindOne(context.Background(), bson.M{"_id": eventId}).Decode(&stored); err != nil {
		t.Fatalf("re-read event: %v", err)
	}
	if stored.IsArchived == nil || !*stored.IsArchived {
		t.Fatal("event should be archived after a 200")
	}
}

// The other half: a non-owner used to match no document on the `ownerId`
// filter, so Decode returned ErrNoDocuments and the handler reported a 500 —
// a server fault for what is really a permission answer.
func TestArchiveEvent_NonOwnerIsForbidden(t *testing.T) {
	requireDB(t)

	owner := &models.User{Id: primitive.NewObjectID()}
	stranger := &models.User{Id: primitive.NewObjectID(), Role: models.RoleMember}
	eventId := seedEvent(t, "e10arf", owner.Id)

	w := postArchive(archiveRouter(stranger), eventId.Hex(), true)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner archive: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}

	// And the event must be untouched.
	var stored models.Event
	if err := db.EventsCollection.FindOne(context.Background(), bson.M{"_id": eventId}).Decode(&stored); err != nil {
		t.Fatalf("re-read event: %v", err)
	}
	if stored.IsArchived != nil && *stored.IsArchived {
		t.Fatal("a forbidden request must not archive the event")
	}
}

func TestArchiveEvent_NotFound(t *testing.T) {
	requireDB(t)

	user := &models.User{Id: primitive.NewObjectID(), Role: models.RoleMember}
	w := postArchive(archiveRouter(user), "nope404", true)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown id: got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
}

func TestDeleteEvent_NonOwnerIsForbidden(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	owner := &models.User{Id: primitive.NewObjectID()}
	stranger := &models.User{Id: primitive.NewObjectID(), Role: models.RoleMember}
	eventId := seedEvent(t, "e10del", owner.Id)

	r := newTestRouter()
	r.Use(func(c *gin.Context) { c.Set("authUser", stranger); c.Next() })
	r.DELETE("/events/:eventId", deleteEvent)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/events/"+eventId.Hex(), nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner delete: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}

	// The event must survive a refused delete.
	count, err := db.EventsCollection.CountDocuments(ctx, bson.M{"_id": eventId})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("a forbidden delete removed the event anyway (%d docs left)", count)
	}
}

// A legacy ownerless event (created before E3 removed anonymous creation) was
// deletable by nobody: the `ownerId: user.Id` filter could never match
// NilObjectID, so every attempt 500'd. requireEventManager gives these the same
// member-or-above rule the edit and schedule routes already use.
func TestDeleteEvent_OwnerlessEventManageableByMember(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	member := &models.User{Id: primitive.NewObjectID(), Role: models.RoleMember}
	eventId := seedEvent(t, "e10orp", primitive.NilObjectID)

	r := newTestRouter()
	r.Use(func(c *gin.Context) { c.Set("authUser", member); c.Next() })
	r.DELETE("/events/:eventId", deleteEvent)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/events/"+eventId.Hex(), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("member deleting an ownerless event: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	count, err := db.EventsCollection.CountDocuments(ctx, bson.M{"_id": eventId})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("ownerless event not deleted: still %d docs", count)
	}
}

// ...but a guest still may not, so the ownerless path didn't become a hole.
func TestDeleteEvent_OwnerlessEventForbiddenToGuest(t *testing.T) {
	requireDB(t)

	guest := &models.User{Id: primitive.NewObjectID(), Role: models.RoleGuest}
	eventId := seedEvent(t, "e10gst", primitive.NilObjectID)

	r := newTestRouter()
	r.Use(func(c *gin.Context) { c.Set("authUser", guest); c.Next() })
	r.DELETE("/events/:eventId", deleteEvent)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/events/"+eventId.Hex(), nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("guest deleting an ownerless event: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
}
