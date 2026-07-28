package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/middleware"
	"sirtom/server/models"
)

// commentsTestRouter wires the comment + thread routes (behind the same
// AuthRequired middleware production uses) onto a gin engine, plus a test-login.
func commentsTestRouter() *gin.Engine {
	r := newTestRouter()
	registerTestLogin(r)
	r.POST("/events/:eventId/comments", middleware.AuthRequired(), addComment)
	r.PUT("/events/:eventId/comments/:commentId", middleware.AuthRequired(), editComment)
	r.DELETE("/events/:eventId/comments/:commentId", middleware.AuthRequired(), deleteComment)
	r.POST("/events/:eventId/comments/:commentId/thread", middleware.AuthRequired(), tagCommentAsThread)
	r.PATCH("/events/:eventId/comments/:commentId/thread", middleware.AuthRequired(), setThreadMembersOnly)
	r.DELETE("/events/:eventId/comments/:commentId/thread", middleware.AuthRequired(), untagThread)
	return r
}

// insertTestUser creates a user with the given role and registers cleanup.
func insertTestUser(t *testing.T, role models.Role, email string) primitive.ObjectID {
	t.Helper()
	id := primitive.NewObjectID()
	_, err := db.UsersCollection.InsertOne(context.Background(), models.User{
		Id: id, Email: email, FirstName: "Test", LastName: "User", Role: role,
	})
	if err != nil {
		t.Fatalf("insert user %s: %v", email, err)
	}
	t.Cleanup(func() { deleteTestUser(id) })
	return id
}

// insertTestEvent creates an event owned by ownerId and registers cleanup of it
// and all of its comments.
func insertTestEvent(t *testing.T, ownerId primitive.ObjectID) primitive.ObjectID {
	t.Helper()
	id := primitive.NewObjectID()
	_, err := db.EventsCollection.InsertOne(context.Background(), models.Event{
		Id: id, OwnerId: ownerId, Type: models.SPECIFIC_DATES,
	})
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		db.EventsCollection.DeleteOne(ctx, bson.M{"_id": id})
		db.CommentsCollection.DeleteMany(ctx, bson.M{"eventId": id})
	})
	return id
}

// do issues a request, optionally with a session cookie, and returns the recorder.
func do(h *gin.Engine, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	h.ServeHTTP(w, req)
	return w
}

func TestComments_RequiresSignIn(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "owner-signin@example.test")
	eventId := insertTestEvent(t, ownerId)
	h := commentsTestRouter()

	// Anonymous posting is refused — the discussion is sign-in-only.
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments", `{"text":"hello"}`, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous post: got %d, want 401 (body: %s)", w.Code, w.Body.String())
	}
}

func TestComments_MemberPostEditDelete(t *testing.T) {
	requireDB(t)

	memberId := insertTestUser(t, models.RoleMember, "member-ped@example.test")
	eventId := insertTestEvent(t, primitive.NewObjectID())
	h := commentsTestRouter()
	cookie := loginAs(t, h, memberId.Hex())

	// Empty text -> 400.
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments", `{"text":"   "}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty text: got %d, want 400", w.Code)
	}

	// Real comment -> 200, authored by the signed-in user.
	w = do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments",
		`{"text":"Parking is out back"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("member post: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var created models.Comment
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.IsGuest || created.UserId != memberId.Hex() || created.AuthorName != "Test User" {
		t.Fatalf("unexpected created comment: %+v", created)
	}
	cid := created.Id.Hex()

	// Edit own -> updatedAt set.
	w = do(h, http.MethodPut, "/events/"+eventId.Hex()+"/comments/"+cid,
		`{"text":"Parking is out back, gate code 1234"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("edit own: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	edited, _ := db.GetCommentById(cid)
	if edited == nil || edited.UpdatedAt == nil || !strings.Contains(edited.Text, "gate code") {
		t.Fatalf("edit did not persist / set updatedAt: %+v", edited)
	}

	// A different member cannot edit or delete it.
	otherId := insertTestUser(t, models.RoleMember, "other-ped@example.test")
	otherCookie := loginAs(t, h, otherId.Hex())
	w = do(h, http.MethodPut, "/events/"+eventId.Hex()+"/comments/"+cid, `{"text":"nope"}`, otherCookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other member edit: got %d, want 403", w.Code)
	}
	w = do(h, http.MethodDelete, "/events/"+eventId.Hex()+"/comments/"+cid, "", otherCookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other member delete: got %d, want 403", w.Code)
	}

	// Author deletes own -> gone.
	w = do(h, http.MethodDelete, "/events/"+eventId.Hex()+"/comments/"+cid, "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete own: got %d, want 200", w.Code)
	}
	if gone, _ := db.GetCommentById(cid); gone != nil {
		t.Fatal("comment should be deleted")
	}
}

// The event owner can delete another author's comment (moderation).
func TestComments_OwnerDeletesAny(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "owner-del@example.test")
	eventId := insertTestEvent(t, ownerId)

	// A legacy guest-authored comment.
	commentId := primitive.NewObjectID()
	db.CommentsCollection.InsertOne(context.Background(), models.Comment{
		Id: commentId, EventId: eventId, UserId: "Greg", IsGuest: true,
		AuthorName: "Greg", Text: "hello",
	})

	h := commentsTestRouter()
	cookie := loginAs(t, h, ownerId.Hex())

	w := do(h, http.MethodDelete, "/events/"+eventId.Hex()+"/comments/"+commentId.Hex(), "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("owner delete of another's comment: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if gone, _ := db.GetCommentById(commentId.Hex()); gone != nil {
		t.Fatal("owner should have deleted the comment")
	}
}

// --- Threads (C13) ----------------------------------------------------------

// postComment posts a comment as the given session and returns its id hex.
func postComment(t *testing.T, h *gin.Engine, eventId primitive.ObjectID, cookie *http.Cookie, body string) string {
	t.Helper()
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("post comment: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var created models.Comment
	json.Unmarshal(w.Body.Bytes(), &created)
	return created.Id.Hex()
}

func TestThreads_GuestCannotTag(t *testing.T) {
	requireDB(t)

	guestId := insertTestUser(t, models.RoleGuest, "guest-tag@example.test")
	eventId := insertTestEvent(t, primitive.NewObjectID())
	h := commentsTestRouter()
	guestCookie := loginAs(t, h, guestId.Hex())

	// A signed-in guest may comment...
	cid := postComment(t, h, eventId, guestCookie, `{"text":"where are we meeting?"}`)

	// ...but may not promote it to a thread.
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments/"+cid+"/thread",
		`{"membersOnly":false}`, guestCookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest tag: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
}

func TestThreads_TagReplyAndNestingRules(t *testing.T) {
	requireDB(t)

	memberId := insertTestUser(t, models.RoleMember, "member-thread@example.test")
	eventId := insertTestEvent(t, primitive.NewObjectID())
	h := commentsTestRouter()
	cookie := loginAs(t, h, memberId.Hex())

	rootId := postComment(t, h, eventId, cookie, `{"text":"Distilleries to visit"}`)

	// Replying before it's a thread is refused.
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments",
		fmt.Sprintf(`{"text":"too early","threadId":%q}`, rootId), cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("reply to non-thread: got %d, want 400", w.Code)
	}

	// Tag it.
	w = do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments/"+rootId+"/thread",
		`{"membersOnly":false}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("tag thread: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	root, _ := db.GetCommentById(rootId)
	if !root.IsThread || root.MembersOnly || root.ThreadedBy != memberId.Hex() {
		t.Fatalf("thread not tagged as expected: %+v", root)
	}

	// Tagging twice is refused.
	w = do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments/"+rootId+"/thread",
		`{"membersOnly":false}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("double tag: got %d, want 400", w.Code)
	}

	// Reply into it.
	replyId := postComment(t, h, eventId, cookie,
		fmt.Sprintf(`{"text":"Buffalo Trace does tours","threadId":%q}`, rootId))
	reply, _ := db.GetCommentById(replyId)
	if reply.ThreadId == nil || reply.ThreadId.Hex() != rootId {
		t.Fatalf("reply not attached to thread: %+v", reply)
	}

	// A reply cannot itself become a thread (one level only).
	w = do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments/"+replyId+"/thread",
		`{"membersOnly":false}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("tag a reply: got %d, want 400 (body: %s)", w.Code, w.Body.String())
	}

	// Un-tagging is refused once replies exist.
	w = do(h, http.MethodDelete, "/events/"+eventId.Hex()+"/comments/"+rootId+"/thread", "", cookie)
	if w.Code != http.StatusConflict {
		t.Fatalf("untag with replies: got %d, want 409 (body: %s)", w.Code, w.Body.String())
	}

	// Deleting the root cascades to its replies.
	w = do(h, http.MethodDelete, "/events/"+eventId.Hex()+"/comments/"+rootId, "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete root: got %d, want 200", w.Code)
	}
	if gone, _ := db.GetCommentById(replyId); gone != nil {
		t.Fatal("deleting a thread root should delete its replies")
	}
}

func TestThreads_UntagAllowedWhileEmpty(t *testing.T) {
	requireDB(t)

	memberId := insertTestUser(t, models.RoleMember, "member-untag@example.test")
	eventId := insertTestEvent(t, primitive.NewObjectID())
	h := commentsTestRouter()
	cookie := loginAs(t, h, memberId.Hex())

	cid := postComment(t, h, eventId, cookie, `{"text":"Hiking trails"}`)
	do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments/"+cid+"/thread", `{"membersOnly":true}`, cookie)

	w := do(h, http.MethodDelete, "/events/"+eventId.Hex()+"/comments/"+cid+"/thread", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("untag empty thread: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	back, _ := db.GetCommentById(cid)
	if back.IsThread || back.MembersOnly || back.ThreadedBy != "" {
		t.Fatalf("untag should clear all thread state: %+v", back)
	}
}

func TestThreads_MembersOnlyBlocksGuestReply(t *testing.T) {
	requireDB(t)

	memberId := insertTestUser(t, models.RoleMember, "member-mo@example.test")
	guestId := insertTestUser(t, models.RoleGuest, "guest-mo@example.test")
	eventId := insertTestEvent(t, primitive.NewObjectID())
	h := commentsTestRouter()
	memberCookie := loginAs(t, h, memberId.Hex())
	guestCookie := loginAs(t, h, guestId.Hex())

	rootId := postComment(t, h, eventId, memberCookie, `{"text":"Budget talk"}`)
	do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments/"+rootId+"/thread",
		`{"membersOnly":true}`, memberCookie)

	// A guest who somehow learns the id still cannot post into a hidden thread.
	w := do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments",
		fmt.Sprintf(`{"text":"sneaking in","threadId":%q}`, rootId), guestCookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest reply into members-only thread: got %d, want 403 (body: %s)", w.Code, w.Body.String())
	}
}

func TestThreads_ManagePermissions(t *testing.T) {
	requireDB(t)

	taggerId := insertTestUser(t, models.RoleMember, "tagger@example.test")
	otherId := insertTestUser(t, models.RoleMember, "other-member@example.test")
	adminId := insertTestUser(t, models.RoleAdmin, "admin-thread@example.test")
	eventId := insertTestEvent(t, primitive.NewObjectID()) // owned by nobody in this test

	h := commentsTestRouter()
	taggerCookie := loginAs(t, h, taggerId.Hex())
	otherCookie := loginAs(t, h, otherId.Hex())
	adminCookie := loginAs(t, h, adminId.Hex())

	rootId := postComment(t, h, eventId, taggerCookie, `{"text":"Gear list"}`)
	do(h, http.MethodPost, "/events/"+eventId.Hex()+"/comments/"+rootId+"/thread",
		`{"membersOnly":false}`, taggerCookie)

	// An unrelated member cannot flip members-only.
	w := do(h, http.MethodPatch, "/events/"+eventId.Hex()+"/comments/"+rootId+"/thread",
		`{"membersOnly":true}`, otherCookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other member toggle: got %d, want 403", w.Code)
	}

	// The tagger can.
	w = do(h, http.MethodPatch, "/events/"+eventId.Hex()+"/comments/"+rootId+"/thread",
		`{"membersOnly":true}`, taggerCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("tagger toggle: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if root, _ := db.GetCommentById(rootId); !root.MembersOnly {
		t.Fatal("membersOnly should be true after the tagger's toggle")
	}

	// So can an admin.
	w = do(h, http.MethodPatch, "/events/"+eventId.Hex()+"/comments/"+rootId+"/thread",
		`{"membersOnly":false}`, adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("admin toggle: got %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if root, _ := db.GetCommentById(rootId); root.MembersOnly {
		t.Fatal("membersOnly should be false after the admin's toggle")
	}
}
