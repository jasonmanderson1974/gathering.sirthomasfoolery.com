// Discussion-thread comments on an event (C7) and the threads within it (C13).
// Routes are registered under the /events group by InitEvents.
//
// The discussion is sign-in-only: every route here sits behind
// middleware.AuthRequired, and getEvent withholds the comment list entirely from
// anonymous callers. Legacy guest-authored rows (IsGuest) still render, but no
// new ones can be written.
package routes

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/models"
	"sirtom/server/responses"
)

const maxCommentLength = 2000

// Error codes specific to the discussion.
const (
	errEmptyComment     = "empty-comment"
	errCommentNotFound  = "comment-not-found"
	errThreadNotFound   = "thread-not-found"
	errNotAThread       = "not-a-thread"
	errAlreadyThread    = "already-a-thread"
	errThreadHasReplies = "thread-has-replies"
	errNestedThread     = "nested-thread"
)

// sanitizeCommentText trims a comment and reports whether it's usable (non-empty
// after trimming). Over-long text is truncated to maxCommentLength runes.
func sanitizeCommentText(text string) (string, bool) {
	trimmed := trimAndTruncate(text, maxCommentLength)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

// commentViewer is the caller's identity as it bears on the discussion: which
// comments they may read, and which threads they may restructure. Kept as a
// plain struct so the filtering logic below stays unit-testable without a
// request or a database.
type commentViewer struct {
	UserId            string
	CanSeeMembersOnly bool
	IsEventOwner      bool
	IsAdmin           bool
}

// canManageThread reports whether this viewer may untag a thread root or flip
// its members-only flag: the person who tagged it, the event owner, or an admin.
func (v commentViewer) canManageThread(root models.Comment) bool {
	if v.UserId == "" {
		return false
	}
	return root.ThreadedBy == v.UserId || v.IsEventOwner || v.IsAdmin
}

// newCommentViewer builds the viewer for a signed-in user against a given event.
func newCommentViewer(user *models.User, event *models.Event) commentViewer {
	if user == nil {
		return commentViewer{}
	}
	role := user.EffectiveRole()
	userId := user.Id.Hex()
	isOwner := event != nil && event.OwnerId != primitive.NilObjectID && event.OwnerId.Hex() == userId
	return commentViewer{
		UserId:            userId,
		CanSeeMembersOnly: role.CanSeeMembersOnly(),
		IsEventOwner:      isOwner,
		IsAdmin:           role.CanManageUsers(),
	}
}

// visibleComments filters a discussion for a viewer and annotates thread roots
// with whether that viewer may manage them.
//
// A members-only thread must disappear for anyone below member — both the root
// AND every reply hanging off it, which is why replies are resolved against
// their root rather than judged on their own (replies never carry the flag
// themselves). This is the only place that decides what reaches a client, so
// filtering here is what actually enforces privacy; hiding in the UI would still
// ship the text in the JSON payload.
func visibleComments(comments []models.Comment, viewer commentViewer) []models.Comment {
	// Roots that this viewer is not allowed to see.
	hiddenRoots := make(map[primitive.ObjectID]bool)
	for _, comment := range comments {
		if comment.IsThread && comment.MembersOnly && !viewer.CanSeeMembersOnly {
			hiddenRoots[comment.Id] = true
		}
	}

	visible := make([]models.Comment, 0, len(comments))
	for _, comment := range comments {
		if hiddenRoots[comment.Id] {
			continue
		}
		if comment.ThreadId != nil && hiddenRoots[*comment.ThreadId] {
			continue
		}
		if comment.IsThread {
			comment.CanManageThread = viewer.canManageThread(comment)
		}
		visible = append(visible, comment)
	}
	return visible
}

// authUserFrom pulls the user set by middleware.AuthRequired. The middleware
// guarantees it exists on every route in this file.
func authUserFrom(c *gin.Context) *models.User {
	userInterface, exists := c.Get("authUser")
	if !exists {
		return nil
	}
	user, _ := userInterface.(*models.User)
	return user
}

// loadCommentContext resolves the event, the signed-in user, and the viewer for
// a comment route, writing the error response itself when anything is missing.
func loadCommentContext(c *gin.Context) (*models.Event, *models.User, commentViewer, bool) {
	user := authUserFrom(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
		return nil, nil, commentViewer{}, false
	}

	event, eventErr := db.GetEventByEitherId(c.Param("eventId"))
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, nil, commentViewer{}, false
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return nil, nil, commentViewer{}, false
	}

	return event, user, newCommentViewer(user, event), true
}

// loadThreadRoot fetches the comment named by :commentId and confirms it belongs
// to this event. Writes the error response itself when it doesn't resolve.
func loadThreadRoot(c *gin.Context, event *models.Event) (*models.Comment, bool) {
	comment, err := db.GetCommentById(c.Param("commentId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, false
	}
	if comment == nil || comment.EventId != event.Id {
		c.JSON(http.StatusNotFound, responses.Error{Error: errCommentNotFound})
		return nil, false
	}
	return comment, true
}

// @Summary Posts a comment to an event's discussion, optionally as a thread reply
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{text=string,threadId=string} true "Comment text, plus the thread root to reply under"
// @Success 200 {object} models.Comment
// @Router /events/{eventId}/comments [post]
func addComment(c *gin.Context) {
	payload := struct {
		Text     string `json:"text" binding:"required"`
		ThreadId string `json:"threadId"`
		// Optional. Sent by the offline write queue so a replayed create makes
		// one comment rather than two (O4). Absent from every other caller,
		// which behaves exactly as before.
		ClientId string `json:"clientId"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	text, ok := sanitizeCommentText(payload.Text)
	if !ok {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errEmptyComment})
		return
	}

	event, user, viewer, ctxOk := loadCommentContext(c)
	if !ctxOk {
		return
	}

	// Resolve the thread this reply belongs to, if any.
	var threadId *primitive.ObjectID
	if strings.TrimSpace(payload.ThreadId) != "" {
		root, err := db.GetCommentById(payload.ThreadId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
		if root == nil || root.EventId != event.Id {
			c.JSON(http.StatusNotFound, responses.Error{Error: errThreadNotFound})
			return
		}
		if !root.IsThread {
			c.JSON(http.StatusBadRequest, responses.Error{Error: errNotAThread})
			return
		}
		// Replying into a members-only thread requires being able to see it —
		// otherwise a guest who learned an id could post into a hidden thread.
		if root.MembersOnly && !viewer.CanSeeMembersOnly {
			c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
			return
		}
		threadId = &root.Id
	}

	clientId := strings.TrimSpace(payload.ClientId)

	// Already posted? Return that one rather than making a second (O4). Checked
	// before any work is done, and note this must answer with the SAME shape as
	// a fresh create — the client is reconciling a queued write, and a bare 200
	// would leave it unable to map its temporary id onto the real one.
	if clientId != "" {
		var existing models.Comment
		found, err := db.FindCommentByClientId(event.Id, clientId, viewer.UserId, &existing)
		if err != nil {
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
		if found {
			c.JSON(http.StatusOK, existing)
			return
		}
	}

	comment := models.Comment{
		Id:         primitive.NewObjectID(),
		EventId:    event.Id,
		UserId:     viewer.UserId,
		IsGuest:    false,
		AuthorName: user.DisplayName(),
		Text:       text,
		CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
		ThreadId:   threadId,
		ClientId:   clientId,
		// Parsed from the sanitized text, so truncation can't leave a half token
		// counting as a mention (F7).
		Mentions: validateMentions(parseMentions(text), event, user),
	}
	var raced models.Comment
	existed, err := db.InsertCommentIdempotent(comment, &raced)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if existed {
		// Two replays landed together and the other one won. Nothing was
		// written here, so notifyMentions must NOT run — the winner already
		// sent them, and sending again is the duplicate this is all preventing.
		c.JSON(http.StatusOK, raced)
		return
	}

	// After the write, never before: a mention email must only ever describe a
	// comment that exists (F8). Nothing this does can fail the request.
	notifyMentions(event, comment, user, nil)

	c.JSON(http.StatusOK, comment)
}

// @Summary Edits the caller's own comment
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param commentId path string true "Comment ID"
// @Param payload body object{text=string} true "New text"
// @Success 200
// @Router /events/{eventId}/comments/{commentId} [put]
func editComment(c *gin.Context) {
	payload := struct {
		Text string `json:"text" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	text, ok := sanitizeCommentText(payload.Text)
	if !ok {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errEmptyComment})
		return
	}

	event, user, viewer, ctxOk := loadCommentContext(c)
	if !ctxOk {
		return
	}

	comment, found := loadThreadRoot(c, event)
	if !found {
		return
	}

	// Editing is own-only. Legacy guest rows have no signed-in owner, so they
	// can't be edited by anyone (they remain deletable by the event owner).
	if comment.IsGuest || comment.UserId != viewer.UserId {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	// Re-parsed from the new text rather than merged with what was stored: a
	// mention removed by the edit must stop being a mention (F7).
	mentions := validateMentions(parseMentions(text), event, user)

	if err := db.UpdateCommentText(comment.Id, text, mentions, primitive.NewDateTimeFromTime(time.Now())); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	// Only the newly added are mailed: comment.Mentions is still what was stored
	// before this edit, so fixing a typo re-notifies nobody (F8).
	updated := *comment
	updated.Text = text
	updated.Mentions = mentions
	notifyMentions(event, updated, user, comment.Mentions)

	c.Status(http.StatusOK)
}

// @Summary Deletes a comment (own, or any when the caller is the event owner)
// @Description Deleting a thread root also deletes every reply within it.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param commentId path string true "Comment ID"
// @Success 200
// @Router /events/{eventId}/comments/{commentId} [delete]
func deleteComment(c *gin.Context) {
	event, _, viewer, ctxOk := loadCommentContext(c)
	if !ctxOk {
		return
	}

	comment, err := db.GetCommentById(c.Param("commentId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if comment == nil || comment.EventId != event.Id {
		c.Status(http.StatusOK) // already gone — idempotent
		return
	}

	// Allowed if it's the caller's own comment, or the caller runs the event.
	isOwn := !comment.IsGuest && comment.UserId == viewer.UserId
	if !isOwn && !viewer.IsEventOwner && !viewer.IsAdmin {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	// Cascade: a thread root takes its replies with it, so they aren't left
	// unreachable in the collection.
	if comment.IsThread {
		if err := db.DeleteCommentsByThreadId(comment.Id); err != nil {
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
	}

	if err := db.DeleteComment(comment.Id); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Tags a comment as a discussion thread
// @Description Members and admins may promote a top-level comment to a thread, optionally hiding it from guests. Replies then hang off it.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param commentId path string true "Comment ID"
// @Param payload body object{membersOnly=bool} true "Whether the thread is hidden from guests"
// @Success 200
// @Router /events/{eventId}/comments/{commentId}/thread [post]
func tagCommentAsThread(c *gin.Context) {
	payload := struct {
		MembersOnly bool `json:"membersOnly"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	event, _, viewer, ctxOk := loadCommentContext(c)
	if !ctxOk {
		return
	}

	// Any member or admin may tag a thread; guests may not.
	if !viewer.CanSeeMembersOnly {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	comment, found := loadThreadRoot(c, event)
	if !found {
		return
	}
	if comment.IsReply() {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errNestedThread})
		return
	}
	if comment.IsThread {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errAlreadyThread})
		return
	}

	if err := db.SetCommentThread(comment.Id, payload.MembersOnly, viewer.UserId); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Toggles whether a thread is members-only
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param commentId path string true "Comment ID"
// @Param payload body object{membersOnly=bool} true "New members-only setting"
// @Success 200
// @Router /events/{eventId}/comments/{commentId}/thread [patch]
func setThreadMembersOnly(c *gin.Context) {
	payload := struct {
		MembersOnly *bool `json:"membersOnly" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	event, _, viewer, ctxOk := loadCommentContext(c)
	if !ctxOk {
		return
	}

	comment, found := loadThreadRoot(c, event)
	if !found {
		return
	}
	if !comment.IsThread {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errNotAThread})
		return
	}
	if !viewer.canManageThread(*comment) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	if err := db.SetCommentMembersOnly(comment.Id, *payload.MembersOnly); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Un-tags a thread, returning it to an ordinary comment
// @Description Only possible while the thread has no replies — un-tagging a thread with replies would scatter them into the top-level discussion out of context.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param commentId path string true "Comment ID"
// @Success 200
// @Failure 409 {object} responses.Error "thread-has-replies"
// @Router /events/{eventId}/comments/{commentId}/thread [delete]
func untagThread(c *gin.Context) {
	event, _, viewer, ctxOk := loadCommentContext(c)
	if !ctxOk {
		return
	}

	comment, found := loadThreadRoot(c, event)
	if !found {
		return
	}
	if !comment.IsThread {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errNotAThread})
		return
	}
	if !viewer.canManageThread(*comment) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	// Once a conversation has started, the tag is permanent.
	replies, err := db.CountThreadReplies(comment.Id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if replies > 0 {
		c.JSON(http.StatusConflict, responses.Error{Error: errThreadHasReplies})
		return
	}

	if err := db.ClearCommentThread(comment.Id); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}
