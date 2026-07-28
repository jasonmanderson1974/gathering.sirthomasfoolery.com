// Venue / activity polls on an event (C6). Routes are registered under the
// /events group by InitEvents. The owner creates/deletes polls; members and
// guests vote (same trust model as RSVP/comments).
package routes

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/responses"
)

const (
	maxPollTitleLength  = 200
	maxPollOptionLength = 200
	maxPollOptions      = 20
)

// sanitizePollInput trims + validates a poll's title and option labels. Returns
// the cleaned title, the cleaned non-empty option labels, and ok=false if the
// poll is unusable (no title, or fewer than 2 distinct options).
func sanitizePollInput(title string, options []string) (string, []string, bool) {
	t := strings.TrimSpace(title)
	if len(t) > maxPollTitleLength {
		t = t[:maxPollTitleLength]
	}

	seen := make(map[string]bool)
	cleaned := make([]string, 0, len(options))
	for _, o := range options {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if len(o) > maxPollOptionLength {
			o = o[:maxPollOptionLength]
		}
		if seen[o] {
			continue // drop duplicate labels
		}
		seen[o] = true
		cleaned = append(cleaned, o)
		if len(cleaned) >= maxPollOptions {
			break
		}
	}

	if t == "" || len(cleaned) < 2 {
		return "", nil, false
	}
	return t, cleaned, true
}

// applyPollVote records `key`'s vote for the given option ids on the poll,
// clearing its vote from every other option (so re-voting replaces the previous
// choice). An empty optionIds clears the voter's vote. Returns an error if an
// option id is unknown, or if multiple options are chosen on a single-choice poll.
func applyPollVote(poll *models.Poll, key, displayName string, optionIds []string) error {
	valid := make(map[string]bool, len(poll.Options))
	for _, opt := range poll.Options {
		valid[opt.Id.Hex()] = true
	}

	chosen := make(map[string]bool, len(optionIds))
	for _, id := range optionIds {
		if !valid[id] {
			return fmt.Errorf("invalid-option")
		}
		chosen[id] = true
	}
	if !poll.AllowMultiple && len(chosen) > 1 {
		return fmt.Errorf("single-choice-poll")
	}

	for i := range poll.Options {
		idHex := poll.Options[i].Id.Hex()
		if chosen[idHex] {
			if poll.Options[i].Votes == nil {
				poll.Options[i].Votes = make(map[string]string)
			}
			poll.Options[i].Votes[key] = displayName
		} else if poll.Options[i].Votes != nil {
			delete(poll.Options[i].Votes, key)
		}
	}
	return nil
}


// @Summary Creates a venue/activity poll on an event (owner only)
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{title=string,allowMultiple=bool,options=[]string} true "Poll title + options"
// @Success 200 {object} models.Poll
// @Router /events/{eventId}/polls [post]
func createPoll(c *gin.Context) {
	payload := struct {
		Title         string   `json:"title" binding:"required"`
		AllowMultiple bool     `json:"allowMultiple"`
		Options       []string `json:"options"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	event, eventErr := db.GetEventByEitherId(c.Param("eventId"))
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}
	if !requireEventManager(c, event) {
		return
	}

	title, options, ok := sanitizePollInput(payload.Title, payload.Options)
	if !ok {
		c.JSON(http.StatusBadRequest, responses.Error{Error: "invalid-poll"})
		return
	}

	poll := models.Poll{
		Id:            primitive.NewObjectID(),
		Title:         title,
		AllowMultiple: payload.AllowMultiple,
	}
	for _, label := range options {
		poll.Options = append(poll.Options, models.PollOption{
			Id:    primitive.NewObjectID(),
			Label: label,
		})
	}

	event.Polls = append(event.Polls, poll)
	if _, err := db.EventsCollection.UpdateByID(context.Background(), event.Id, bson.M{"$set": bson.M{"polls": event.Polls}}); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, poll)
}

// @Summary Deletes a poll from an event (owner only)
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param pollId path string true "Poll ID"
// @Success 200
// @Router /events/{eventId}/polls/{pollId} [delete]
func deletePoll(c *gin.Context) {
	event, eventErr := db.GetEventByEitherId(c.Param("eventId"))
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}
	if !requireEventManager(c, event) {
		return
	}

	pollId := c.Param("pollId")
	kept := make([]models.Poll, 0, len(event.Polls))
	for _, p := range event.Polls {
		if p.Id.Hex() != pollId {
			kept = append(kept, p)
		}
	}
	if len(kept) == len(event.Polls) {
		c.Status(http.StatusOK) // already gone — idempotent
		return
	}

	if _, err := db.EventsCollection.UpdateByID(context.Background(), event.Id, bson.M{"$set": bson.M{"polls": kept}}); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Casts (or updates) the caller's vote in a poll
// @Description Records the caller's chosen option(s) in a poll, replacing any previous vote. Open to signed-in users and guests (by name), like RSVP. An empty optionIds clears the caller's vote.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param pollId path string true "Poll ID"
// @Param payload body object{optionIds=[]string} true "Chosen option ids"
// @Success 200
// @Router /events/{eventId}/polls/{pollId}/vote [post]
func votePoll(c *gin.Context) {
	// E3: the vote is keyed to the session. `guest`/`name` are gone — they let
	// any caller vote as anyone, and stuff the roster with arbitrary names.
	payload := struct {
		OptionIds []string `json:"optionIds"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	event, eventErr := db.GetEventByEitherId(c.Param("eventId"))
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}

	pollId := c.Param("pollId")
	pollIdx := -1
	for i := range event.Polls {
		if event.Polls[i].Id.Hex() == pollId {
			pollIdx = i
			break
		}
	}
	if pollIdx == -1 {
		c.JSON(http.StatusNotFound, responses.Error{Error: "poll-not-found"})
		return
	}

	key, _, keyOk := responderKey(c)
	if !keyOk {
		return
	}

	// The roster name comes from the account, never from the request body.
	displayName := key
	if user, err := db.GetUserById(key); err == nil && user != nil {
		displayName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}

	if err := applyPollVote(&event.Polls[pollIdx], key, displayName, payload.OptionIds); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	if _, err := db.EventsCollection.UpdateByID(context.Background(), event.Id, bson.M{"$set": bson.M{"polls": event.Polls}}); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}
