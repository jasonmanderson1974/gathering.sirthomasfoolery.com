// A person's private note on a gathering (F20) — the "My Notes" tab.
//
// One free-form markdown document per person per gathering. Registered
// alongside the private lists by initPersonalRoutes (routes/personal_lists.go),
// and private for the same reason: the userId is part of every Mongo filter, so
// there is no permission check here because there is nothing for one to do.
//
// The markdown is stored exactly as typed and rendered on the client. Nothing is
// sanitized on the way IN — sanitizing at write time would silently rewrite what
// someone typed, and the note is only ever displayed to the person who wrote it,
// through DOMPurify (frontend/src/utils/markdown.js).
package routes

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/responses"
)

const (
	// A note is a DOCUMENT, not a field — four pages of prose is a reasonable
	// thing to type here, so the cap is an order of magnitude above the
	// comment's 2,000. Characters, not bytes (see below).
	maxPersonalNoteLength = 20000
)

const errNoteTooLong = "note-too-long"

// personalNoteResponse is what both note routes return.
//
// A dedicated struct rather than models.PersonalNote so that "never written" and
// "written, then cleared" both serialize as text:"" with a null updatedAt — the
// client renders an editor either way, and a nil-vs-empty distinction it can't
// act on is a branch it shouldn't have to carry.
type personalNoteResponse struct {
	Text string `json:"text"`
	// A POINTER so an unwritten note serializes as null rather than as the
	// epoch, which would render as "Saved 01:00" on a note nobody has saved.
	UpdatedAt *primitive.DateTime `json:"updatedAt"`
}

// @Summary Returns the caller's own private note on a gathering
// @Description Private to the signed-in user. A user who has never written one gets 200 with an empty note rather than a 404 — "no note" and "empty note" are the same thing to the person looking at the editor.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200 {object} routes.personalNoteResponse
// @Router /events/{eventId}/my-notes [get]
func getPersonalNote(c *gin.Context) {
	userId, eventId, ok := loadPersonalOwner(c)
	if !ok {
		return
	}

	note, err := db.GetPersonalNote(userId, eventId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if note == nil {
		c.JSON(http.StatusOK, personalNoteResponse{Text: "", UpdatedAt: nil})
		return
	}

	c.JSON(http.StatusOK, personalNoteResponse{Text: note.Text, UpdatedAt: &note.UpdatedAt})
}

// @Summary Replaces the caller's own private note on a gathering
// @Description A whole-document replace, upserted on first save and idempotent thereafter. There is no DELETE: sending an empty string is how a note is cleared, and a second verb for one state transition is a second thing to keep consistent.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{text=string} true "The whole note, as markdown"
// @Success 200 {object} routes.personalNoteResponse
// @Router /events/{eventId}/my-notes [put]
func setPersonalNote(c *gin.Context) {
	payload := struct {
		// A POINTER with binding:"required", for the same reason Checked is one
		// on the checklist route: a bare string fails the binding on "", and ""
		// is exactly what "clear my note" sends. The pointer accepts the empty
		// string while still rejecting a payload that omits the field.
		Text *string `json:"text" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	// Trim first, then measure — padding must not eat the budget (the ordering
	// trimAndTruncate uses, and for the same reason).
	text := strings.TrimSpace(*payload.Text)

	// REJECTED, not truncated, and this is the one place this codebase's free-
	// text handling deliberately differs from trimAndTruncate. A list name or a
	// comment losing its tail is a visible, recoverable annoyance; silently
	// eating the end of four pages someone typed by hand is not. The client caps
	// the field at the same number, so a real browser can't reach this.
	//
	// RUNES, not bytes: a note carrying emoji or accented names would otherwise
	// hit a cap well short of the characters it promises.
	if utf8.RuneCountInString(text) > maxPersonalNoteLength {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errNoteTooLong})
		return
	}

	userId, eventId, ok := loadPersonalOwner(c)
	if !ok {
		return
	}

	note, err := db.SetPersonalNote(userId, eventId, text, primitive.NewDateTimeFromTime(time.Now()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	// What was actually stored, so the panel's "Saved 12:04" comes from the
	// server's clock rather than from what the client hoped it sent.
	c.JSON(http.StatusOK, personalNoteResponse{Text: note.Text, UpdatedAt: &note.UpdatedAt})
}
