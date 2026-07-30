/* The /events group contains all the routes to get and edit events */
package routes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/middleware"
	"sirtom/server/models"
	"sirtom/server/responses"
	"sirtom/server/services/calendar"
	"sirtom/server/utils"
)

func InitEvents(router *gin.RouterGroup) {
	eventRouter := router.Group("/events")

	// E3: every event route requires a signed-in user (minimum role `guest`).
	// Anonymous visitors get the landing page and the sign-in flow, nothing else.
	//
	// The ONE deliberate exception is the ICS feed below: calendar applications
	// fetch it on a schedule without cookies, so it cannot carry a session. It
	// exposes only the confirmed gathering's name, time and venue, and obtaining
	// the URL now requires signing in. Do not add further bare routes here.
	eventRouter.GET("/:eventId/ics", getEventIcs)

	authed := eventRouter.Group("", middleware.AuthRequired())

	authed.POST("", createEvent)
	authed.POST("/import", importEvent)
	authed.PUT("/:eventId", editEvent)
	authed.GET("/:eventId/ids", getEventIds)
	authed.GET("/:eventId", getEvent)
	authed.GET("/:eventId/responses", getResponses)
	authed.POST("/:eventId/response", updateEventResponse)
	authed.DELETE("/:eventId/response", deleteEventResponse)
	authed.POST("/:eventId/rename-user", renameUser)
	// Reminder emails link here. The link passes through the SPA's sign-in
	// redirect, so the flow survives requiring a session.
	authed.POST("/:eventId/responded", userResponded)
	authed.DELETE("/:eventId", deleteEvent)
	authed.POST("/:eventId/duplicate", duplicateEvent)
	authed.POST("/:eventId/archive", archiveEvent)
	authed.POST("/:eventId/schedule", scheduleEvent)
	authed.POST("/:eventId/rsvp", rsvpToEvent)
	authed.DELETE("/:eventId/rsvp", deleteRsvp)
	authed.POST("/:eventId/comments", addComment)
	authed.PUT("/:eventId/comments/:commentId", editComment)
	authed.DELETE("/:eventId/comments/:commentId", deleteComment)
	authed.POST("/:eventId/comments/:commentId/thread", tagCommentAsThread)
	authed.PATCH("/:eventId/comments/:commentId/thread", setThreadMembersOnly)
	authed.DELETE("/:eventId/comments/:commentId/thread", untagThread)
	authed.GET("/:eventId/mentionables", getMentionables)
	authed.POST("/:eventId/polls", createPoll)
	authed.DELETE("/:eventId/polls/:pollId", deletePoll)
	authed.POST("/:eventId/polls/:pollId/vote", votePoll)
	authed.GET("/:eventId/lists", getEventLists)
	authed.POST("/:eventId/lists", createEventList)
	authed.PATCH("/:eventId/lists/:listId", renameEventList)
	authed.DELETE("/:eventId/lists/:listId", deleteEventList)
	authed.POST("/:eventId/lists/:listId/items", addEventListItem)
	authed.PUT("/:eventId/lists/:listId/items/:itemId", editEventListItem)
	authed.PUT("/:eventId/lists/:listId/items/:itemId/checked", setEventListItemChecked)
	authed.DELETE("/:eventId/lists/:listId/items/:itemId", deleteEventListItem)
}

// trimmedLocation normalizes a venue submitted by a client. A nil pointer means
// the field was absent and stays nil, so callers can still tell "not provided"
// from "cleared"; anything else is trimmed, so that " The Fox & Hound " and
// "The Fox & Hound" are one venue however the event was created or edited.
func trimmedLocation(location *string) *string {
	if location == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*location)
	return &trimmed
}

// Input caps for the event create/edit payloads. Comments and polls already cap
// their inputs (2,000 chars / 20 options); these extend the same discipline to
// events, where an uncapped Name and unbounded Dates/Times/Remindees
// let one request store a multi-megabyte document.
//
// Deliberately generous — no legitimate client comes close. The frontend builds
// one date per selected day (DOW tops out at 7) and never sends `times` at all,
// so these only bite on direct API use.
const (
	maxEventNameLength        = 200
	maxEventDescriptionLength = 2000
	maxEventDates             = 366 // a year of daily options
	maxEventTimes             = 366
	maxEventRemindees         = 200
)

// eventPayloadLimits is the shared shape of the create/edit payload fields that
// need bounding, so both handlers enforce one rule set.
type eventPayloadLimits struct {
	Type      models.EventType
	Dates     []primitive.DateTime
	Times     []primitive.DateTime
	Remindees []string
}

// validateEventPayload rejects an unusable event payload, writing the response
// and returning false if so.
//
// Cardinality violations are rejected rather than truncated: silently dropping
// dates would store a subtly-wrong event that looks fine to the organizer, which
// is worse than an error. Free text is truncated instead (see sanitizeEventText)
// because an over-long name is harmless once bounded.
func validateEventPayload(c *gin.Context, p eventPayloadLimits) bool {
	// The `binding:"required"` tag only rejects "", so the enum needs an
	// explicit check or any string is stored (TODO E11).
	if !models.IsKnownEventType(p.Type) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidEventType})
		return false
	}

	tooMany := func(n, max int) bool { return n > max }
	switch {
	case tooMany(len(p.Dates), maxEventDates),
		tooMany(len(p.Times), maxEventTimes),
		tooMany(len(p.Remindees), maxEventRemindees):
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.PayloadTooLarge})
		return false
	}
	return true
}

// sanitizeEventText trims and bounds an event's free-text fields. A nil
// description stays nil so "absent" remains distinguishable from "cleared",
// matching trimmedLocation.
func sanitizeEventText(name string, description *string) (string, *string) {
	name = trimAndTruncate(name, maxEventNameLength)
	if description != nil {
		d := trimAndTruncate(*description, maxEventDescriptionLength)
		description = &d
	}
	return name, description
}

// authUser returns the caller set by middleware.AuthRequired. Every event route
// except the ICS feed runs behind that middleware (E3), so a miss here means a
// route was registered outside the authed group — a programming error, reported
// as a 500 rather than silently treated as anonymous.
func authUser(c *gin.Context) (*models.User, bool) {
	userInterface, exists := c.Get("authUser")
	if !exists {
		logger.StdErr.Println("authUser missing from context — route registered outside the authed group?")
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, false
	}
	user, ok := userInterface.(*models.User)
	if !ok || user == nil {
		logger.StdErr.Println("authUser in context has unexpected type")
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, false
	}
	return user, true
}

// requireEventManager reports whether the caller may manage this event — edit
// it, schedule the gathering, or create/delete polls. Writes the response and
// returns false if not.
//
// Owned events: the owner alone. Legacy ownerless events (created before E3
// removed anonymous creation, so nobody holds title to them): member or above.
// Previously these were manageable by ANYONE, which is how an ownerless event
// could be taken over.
func requireEventManager(c *gin.Context, event *models.Event) bool {
	user, ok := authUser(c)
	if !ok {
		return false
	}

	if event.OwnerId != primitive.NilObjectID {
		if user.Id != event.OwnerId {
			c.JSON(http.StatusForbidden, responses.Error{Error: errs.UserNotEventOwner})
			return false
		}
		return true
	}

	if !user.EffectiveRole().CanInvite() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return false
	}
	return true
}

// @Summary Creates a new event
// @Tags events
// @Accept json
// @Produce json
// @Param payload body object{name=string,duration=float32,dates=[]string,type=models.EventType,notificationsEnabled=bool,blindAvailabilityEnabled=bool,daysOnly=bool,wholeBlockSelection=bool,remindees=[]string,sendEmailAfterXResponses=int,when2meetHref=string,timeIncrement=int} true "Object containing info about the event to create"
// @Success 201 {object} object{eventId=string}
// @Router /events [post]
func createEvent(c *gin.Context) {
	payload := struct {
		// Required parameters
		Name     string               `json:"name" binding:"required"`
		Duration *float32             `json:"duration" binding:"required"`
		Dates    []primitive.DateTime `json:"dates" binding:"required"`
		Type     models.EventType     `json:"type" binding:"required"`

		// Only for specific times for specific dates events
		HasSpecificTimes    *bool                `json:"hasSpecificTimes"`
		Times               []primitive.DateTime `json:"times"`
		WholeBlockSelection *bool                `json:"wholeBlockSelection"`

		// Only for sign up form events

		// Only for events (not groups)
		StartOnMonday            *bool    `json:"startOnMonday"`
		NotificationsEnabled     *bool    `json:"notificationsEnabled"`
		BlindAvailabilityEnabled *bool    `json:"blindAvailabilityEnabled"`
		DaysOnly                 *bool    `json:"daysOnly"`
		Remindees                []string `json:"remindees"`
		SendEmailAfterXResponses *int     `json:"sendEmailAfterXResponses"`
		When2meetHref            *string  `json:"when2meetHref"`
		CollectEmails            *bool    `json:"collectEmails"`
		TimeIncrement            *int     `json:"timeIncrement"`
		Location                 *string  `json:"location"`

		// Only for availability groups
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}
	if !validateEventPayload(c, eventPayloadLimits{
		Type:      payload.Type,
		Dates:     payload.Dates,
		Times:     payload.Times,
		Remindees: payload.Remindees,
	}) {
		return
	}
	// createEvent takes no description; editEvent adds one later.
	payload.Name, _ = sanitizeEventText(payload.Name, nil)

	// E3: the route is behind AuthRequired, so there is always an authenticated
	// user and every new event has a real owner. The old nil-owner path (which
	// minted ownerless, takeover-able events) is gone.
	user, ok := authUser(c)
	if !ok {
		return
	}
	// Guests may respond to events but not create them.
	if !user.EffectiveRole().CanCreateEvents() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}
	ownerId := user.Id

	// Construct event object
	numResponses := 0
	event := models.Event{
		Id:                       primitive.NewObjectID(),
		OwnerId:                  ownerId,
		Name:                     payload.Name,
		Duration:                 payload.Duration,
		Dates:                    payload.Dates,
		HasSpecificTimes:         payload.HasSpecificTimes,
		Times:                    payload.Times,
		WholeBlockSelection:      payload.WholeBlockSelection,
		StartOnMonday:            payload.StartOnMonday,
		NotificationsEnabled:     payload.NotificationsEnabled,
		BlindAvailabilityEnabled: payload.BlindAvailabilityEnabled,
		DaysOnly:                 payload.DaysOnly,
		SendEmailAfterXResponses: payload.SendEmailAfterXResponses,
		When2meetHref:            payload.When2meetHref,
		CollectEmails:            payload.CollectEmails,
		TimeIncrement:            payload.TimeIncrement,
		Location:                 trimmedLocation(payload.Location),
		Type:                     payload.Type,
		NumResponses:             &numResponses,
	}

	// Generate short id
	shortId, shortIdErr := db.GenerateShortEventId()
	if shortIdErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	event.ShortId = &shortId

	// Record the remindees. The scheduler (services/reminders) picks them up on
	// its next tick and sends the nudges, measured from AddedAt.
	if len(payload.Remindees) > 0 {
		addedAt := primitive.NewDateTimeFromTime(time.Now())
		remindees := make([]models.Remindee, 0)
		for _, email := range payload.Remindees {
			remindees = append(remindees, models.Remindee{
				Email:     email,
				Responded: utils.FalsePtr(),
				AddedAt:   &addedAt,
			})
		}

		event.Remindees = &remindees
	}

	// Insert event
	result, err := db.EventsCollection.InsertOne(context.Background(), event)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	insertedId := result.InsertedID.(primitive.ObjectID).Hex()

	c.JSON(http.StatusCreated, gin.H{"eventId": insertedId, "shortId": event.ShortId})
}

// @Summary Edits an event based on its id
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{name=string,description=string,duration=float32,dates=[]string,type=models.EventType,notificationsEnabled=bool,blindAvailabilityEnabled=bool,daysOnly=bool,wholeBlockSelection=bool,remindees=[]string,sendEmailAfterXResponses=int} true "Object containing info about the event to update"
// @Success 200
// @Router /events/{eventId} [put]
func editEvent(c *gin.Context) {
	payload := struct {
		// Required parameters
		Name     string               `json:"name" binding:"required"`
		Duration *float32             `json:"duration" binding:"required"`
		Dates    []primitive.DateTime `json:"dates" binding:"required"`
		Type     models.EventType     `json:"type" binding:"required"`

		// Only for specific times for specific dates events
		HasSpecificTimes    *bool                `json:"hasSpecificTimes"`
		Times               []primitive.DateTime `json:"times"`
		WholeBlockSelection *bool                `json:"wholeBlockSelection"`

		// For both events and groups
		Description *string `json:"description"`
		Location    *string `json:"location"`

		// Only for sign up form events

		// Only for events (not groups)
		StartOnMonday            *bool    `json:"startOnMonday"`
		NotificationsEnabled     *bool    `json:"notificationsEnabled"`
		BlindAvailabilityEnabled *bool    `json:"blindAvailabilityEnabled"`
		DaysOnly                 *bool    `json:"daysOnly"`
		Remindees                []string `json:"remindees"`
		SendEmailAfterXResponses *int     `json:"sendEmailAfterXResponses"`
		CollectEmails            *bool    `json:"collectEmails"`

		// Only for availability groups
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}
	if !validateEventPayload(c, eventPayloadLimits{
		Type:      payload.Type,
		Dates:     payload.Dates,
		Times:     payload.Times,
		Remindees: payload.Remindees,
	}) {
		return
	}
	payload.Name, payload.Description = sanitizeEventText(payload.Name, payload.Description)

	eventId := c.Param("eventId")
	event, eventErr := db.GetEventByEitherId(eventId)
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

	// Update event
	event.Name = payload.Name
	event.Description = payload.Description
	event.Location = trimmedLocation(payload.Location)
	event.Duration = payload.Duration
	event.Dates = payload.Dates
	event.Times = payload.Times
	event.HasSpecificTimes = payload.HasSpecificTimes
	event.WholeBlockSelection = payload.WholeBlockSelection
	event.StartOnMonday = payload.StartOnMonday
	event.NotificationsEnabled = payload.NotificationsEnabled
	event.BlindAvailabilityEnabled = payload.BlindAvailabilityEnabled
	event.DaysOnly = payload.DaysOnly
	event.SendEmailAfterXResponses = payload.SendEmailAfterXResponses
	event.CollectEmails = payload.CollectEmails
	event.Type = payload.Type

	// Update remindees
	if event.Type == models.DOW || event.Type == models.SPECIFIC_DATES {
		origRemindees := utils.Coalesce(event.Remindees)
		updatedRemindees := make([]models.Remindee, 0)
		// Removed remindees need no handling beyond being left out of
		// updatedRemindees — dropping them stops their nudges.
		added, _, kept := utils.FindAddedRemovedKept(payload.Remindees, utils.Map(origRemindees, func(r models.Remindee) string { return r.Email }))

		for _, keptEmail := range kept {
			updatedRemindees = append(updatedRemindees, origRemindees[keptEmail.Index])
		}

		// Newly added remindees start their own nudge clock from now — an event
		// edited weeks after creation shouldn't fire all three at once.
		addedAt := primitive.NewDateTimeFromTime(time.Now())
		for _, addedEmail := range added {
			updatedRemindees = append(updatedRemindees, models.Remindee{
				Email:     addedEmail.Value,
				Responded: utils.FalsePtr(),
				AddedAt:   &addedAt,
			})
		}

		event.Remindees = &updatedRemindees
	}

	// Persist only the fields the edit form owns (A16). Writing the whole
	// document meant an edit silently reverted any response, RSVP, poll or
	// comment made since the page was loaded.
	err := db.UpdateEditableEventFields(event)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Resolves an event identifier to both short and long IDs
// @Tags events
// @Produce json
// @Param eventId path string true "Event shortId or longId"
// @Success 200 {object} object{shortId=string,longId=string}
// @Failure 404 {object} responses.Error
// @Router /events/{eventId}/ids [get]
func getEventIds(c *gin.Context) {
	eventId := c.Param("eventId")
	event, eventErr := db.GetEventByEitherId(eventId)
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}

	shortId := ""
	if event.ShortId != nil {
		shortId = *event.ShortId
	}

	c.JSON(http.StatusOK, gin.H{
		"shortId": shortId,
		"longId":  event.Id.Hex(),
	})
}

// @Summary Gets an event based on its id
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200 {object} models.Event
// @Router /events/{eventId} [get]
func getEvent(c *gin.Context) {
	eventId := c.Param("eventId")
	event, eventErr := db.GetEventByEitherId(eventId)
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}
	eventResponses, eventResponsesErr := db.GetEventResponses(event.Id.Hex())
	if eventResponsesErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	// Convert to old format for backward compatibility
	utils.ConvertEventToOldFormat(event, eventResponses)

	// Convert responses to map format for JSON response
	responsesMap := getResponsesMap(eventResponses)

	// Populate user fields
	for userId, response := range responsesMap {
		user, userErr := db.GetUserById(userId)
		if userErr != nil {
			logger.StdErr.Println(userErr)
			continue
		}
		if user == nil {
			if len(response.Name) == 0 {
				// User was deleted
				delete(responsesMap, userId)
				continue
			} else {
				// User is guest
				userId = response.Name
				response.User = &models.User{
					FirstName: response.Name,
					Email:     response.Email,
				}
			}
		} else {
			response.User = user
			response.User.CalendarAccounts = nil
		}
		responsesMap[userId] = response

		// Remove availability arrays
		responsesMap[userId].Availability = nil
		responsesMap[userId].IfNeeded = nil
		responsesMap[userId].ManualAvailability = nil
	}

	// Determine if the requester is the event owner
	ownerSesh := event.OwnerId.Hex()
	session := sessions.Default(c)
	userIdInterface := session.Get("userId")
	var userSesh string
	if userIdInterface != nil {
		userSesh = userIdInterface.(string)
	}
	isOwner := userSesh != "" && ownerSesh == userSesh

	// Strip sensitive user info from all responses
	showEmails := isOwner && utils.Coalesce(event.CollectEmails)
	for userId, response := range responsesMap {
		stripSensitiveUserFields(response.User)
		if !showEmails {
			response.Email = ""
			if response.User != nil {
				response.User.Email = ""
			}
		}
		responsesMap[userId] = response
	}

	// RSVPs (C1) carry the responder's email — backfilled from the account for
	// signed-in RSVPs — and bypassed the showEmails gate entirely, so every
	// event viewer could read every attendee's address. Apply the same rule the
	// responses above use. The RSVP UI only renders status/name/guestCount.
	if !showEmails {
		for key, rsvp := range event.Rsvps {
			if rsvp != nil {
				rsvp.Email = ""
			}
			event.Rsvps[key] = rsvp
		}
	}

	// Remindees are the owner's invite list: a bare roll of email addresses,
	// previously serialized to every viewer. Only the owner has a use for it
	// (NewEvent prefills the invite field from it when editing).
	if !isOwner {
		event.Remindees = nil
	}

	// Update event.ResponsesMap to match the final responsesMap
	event.ResponsesMap = responsesMap

	// Attach the discussion (C7) + its threads (C13). Best-effort — non-critical
	// to the page. This is the only path by which comments reach a client, so it
	// is where discussion privacy is actually enforced:
	//   - anonymous callers get nothing at all (the discussion is sign-in-only);
	//   - guests get everything except members-only threads and their replies.
	event.Comments = []models.Comment{}
	if userSesh != "" {
		if commenter, commenterErr := db.GetUserById(userSesh); commenterErr == nil && commenter != nil {
			if comments, commentsErr := db.GetComments(event.Id.Hex()); commentsErr == nil {
				event.Comments = visibleComments(comments, newCommentViewer(commenter, event))
			}
		}
	}

	// Names on comments, RSVPs and poll votes are snapshots taken when each was
	// written, so a member who has since set a nickname would still appear under
	// their old name. Re-resolve them against the accounts as they are now — one
	// batched lookup for all three. Runs AFTER visibleComments so a hidden
	// thread's authors are never even looked up.
	resolveEventDisplayNames(event)

	// Apply privacy logic based on blindAvailabilityEnabled
	if !utils.Coalesce(event.BlindAvailabilityEnabled) {
		// Blind availability is NOT enabled - return response as-is
		c.JSON(http.StatusOK, event)
		return
	}

	// Blind availability IS enabled - apply additional privacy filtering

	var privatizedResponse map[string]interface{}
	var err error

	if userSesh != "" {
		// User session exists (user is logged in)
		if ownerSesh == userSesh {
			// User is the owner - return response as-is
			privatizedResponse, err = utils.PrivatizeEventResponse(event, []string{}, []utils.PartialOmission{})
		} else {
			// User is NOT the owner - privatize response
			privateFields := []string{"numResponses"}
			partialOmissions := []utils.PartialOmission{
				{
					FieldName: "responses",
					KeepKey:   userSesh,
				},
			}
			privatizedResponse, err = utils.PrivatizeEventResponse(event, privateFields, partialOmissions)
		}
	} else {
		// Unreachable in practice — the route is behind AuthRequired — but keep
		// the safe default: no session, nothing private.
		privateFields := []string{"numResponses", "responses", "remindees"}
		privatizedResponse, err = utils.PrivatizeEventResponse(event, privateFields, []utils.PartialOmission{})
	}

	if err != nil {
		logger.StdErr.Printf("Failed to privatize event response: %v\n", err)
		// Fall back to returning the original event if privatization fails
		c.JSON(http.StatusOK, event)
		return
	}

	// Return the privatized response
	c.JSON(http.StatusOK, privatizedResponse)
}

// @Summary Deletes an event based on its id
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200
// @Router /events/{eventId} [delete]
func deleteEvent(c *gin.Context) {
	// Resolve by either the short id or the Mongo _id, consistent with the rest
	// of the event routes (which all use GetEventByEitherId). Previously this
	// handler only accepted the _id and 400'd on a short id (TODO E2).
	resolvedEvent, err := db.GetEventByEitherId(c.Param("eventId"))
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if resolvedEvent == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}
	objectId := resolvedEvent.Id
	eventId := objectId.Hex()

	// Authorize before touching anything. The Mongo filters below used to carry
	// `ownerId: user.Id` as the only check, so a non-owner matched no document
	// and Decode returned ErrNoDocuments — reported as a 500. That both hid the
	// real answer (403) and made a legacy ownerless event undeletable by anyone.
	if !requireEventManager(c, resolvedEvent) {
		return
	}

	user, ok := authUser(c)
	if !ok {
		return
	}

	// Check if the current user responded
	eventResponses, eventResponsesErr := db.GetEventResponses(eventId)
	if eventResponsesErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	hasCurrentUserResponded := false
	for _, resp := range eventResponses {
		if resp.UserId == user.Id.Hex() {
			hasCurrentUserResponded = true
			break
		}
	}
	hasResponses := len(eventResponses) > 0
	if hasCurrentUserResponded {
		// Only set hasResponses to true if there are responses other than the current user's
		hasResponses = len(eventResponses) > 1
	}

	var event models.Event

	if hasResponses {
		// If event has responses, just set isDeleted flag
		result := db.EventsCollection.FindOneAndUpdate(context.Background(), bson.M{
			"_id": objectId,
		}, bson.M{
			"$set": bson.M{
				"isDeleted": true,
			},
		})
		err = result.Decode(&event)
		if err != nil {
			// Authorization already passed, so no document here means the event
			// was removed between the lookup and the write — 404, not 500.
			if errors.Is(err, mongo.ErrNoDocuments) {
				c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
				return
			}
			logger.StdErr.Println(err)
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
	} else {
		// If event has no responses, actually delete the event object
		result := db.EventsCollection.FindOneAndDelete(context.Background(), bson.M{
			"_id": objectId,
		})
		err = result.Decode(&event)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
				return
			}
			logger.StdErr.Println(err)
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}

		// Delete folder associations
		_, err = db.FolderEventsCollection.DeleteMany(context.Background(), bson.M{
			"eventId": objectId,
		})
		if err != nil {
			logger.StdErr.Println(err)
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
	}

	// Nothing to unschedule: a deleted event drops out of the scheduler's query,
	// so its remindees stop being nudged on the next tick.

	c.Status(http.StatusOK)
}

// @Summary Duplicate event
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{eventName=string,copyAvailability=bool} true "Object containing options for the duplicated event"
// @Success 200
// @Router /events/{eventId}/duplicate [post]
func duplicateEvent(c *gin.Context) {
	payload := struct {
		EventName        string `json:"eventName" binding:"required"`
		CopyAvailability *bool  `json:"copyAvailability" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	eventId := c.Param("eventId")
	userInterface, _ := c.Get("authUser")
	user := userInterface.(*models.User)

	// Guests may respond to events but not create them (incl. via duplicate).
	if !user.EffectiveRole().CanCreateEvents() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	// Get event
	event, eventErr := db.GetEventByEitherId(eventId)
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if event == nil {
		c.Status(http.StatusBadRequest)
		return
	}

	// Make sure user has permission to duplicate this event
	if event.OwnerId != user.Id {
		c.Status(http.StatusForbidden)
		return
	}

	// Update event
	event.Id = primitive.NewObjectID()
	event.Name = payload.EventName
	numResponses := 0
	event.NumResponses = &numResponses
	if *payload.CopyAvailability {
		eventResponses, eventResponsesErr := db.GetEventResponses(eventId)
		if eventResponsesErr != nil {
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
		for _, eventResponse := range eventResponses {
			eventResponse.Id = primitive.NewObjectID()
			eventResponse.EventId = event.Id
			_, err := db.EventResponsesCollection.InsertOne(context.Background(), eventResponse)
			if err != nil {
				logger.StdErr.Println(err)
				c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
				return
			}
			*event.NumResponses++
		}
	}

	// Generate short id
	shortId, shortIdErr := db.GenerateShortEventId()
	if shortIdErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	event.ShortId = &shortId

	// Insert new event
	result, err := db.EventsCollection.InsertOne(context.Background(), event)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	insertedId := result.InsertedID.(primitive.ObjectID).Hex()
	c.JSON(http.StatusCreated, gin.H{"eventId": insertedId, "shortId": shortId})
}

// @Summary Archive an event
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{archive=bool} true "Archive status"
// @Success 200
// @Router /events/{eventId}/archive [post]
func archiveEvent(c *gin.Context) {
	payload := struct {
		Archive *bool `json:"archive" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	// Resolve by either id, like every other event route. ObjectIDFromHex alone
	// 400'd on a short id — the same sharp edge E2 fixed for delete.
	resolvedEvent, err := db.GetEventByEitherId(c.Param("eventId"))
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if resolvedEvent == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}

	// As in deleteEvent: authorize explicitly, so a non-owner gets 403 rather
	// than the 500 an unmatched `ownerId` filter used to produce.
	if !requireEventManager(c, resolvedEvent) {
		return
	}

	result := db.EventsCollection.FindOneAndUpdate(context.Background(), bson.M{
		"_id": resolvedEvent.Id,
	}, bson.M{
		"$set": bson.M{
			"isArchived": payload.Archive,
		},
	})
	var event models.Event
	if err := result.Decode(&event); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
			return
		}
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// clampLeadTimeHours bounds the reminder lead time to a sane range, defaulting
// to 24h when unset (<= 0).
func clampLeadTimeHours(h int) int {
	const def, min, max = 24, 1, 168 // 168h = 7 days
	if h <= 0 {
		return def
	}
	if h < min {
		return min
	}
	if h > max {
		return max
	}
	return h
}

// @Summary Confirms (or cancels) a gathering's locked-in time and reminder
// @Description Persists the chosen gathering time on the event's scheduledEvent and, when reminderEnabled, arms a one-time pre-gathering reminder email sent reminderLeadTimeHours before the start. Set recurrenceFrequency (weekly|biweekly|monthly) to make it a repeating gathering. Pass scheduled=false to cancel.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{scheduled=bool,startDate=string,endDate=string,summary=string,reminderEnabled=bool,reminderLeadTimeHours=int,recurrenceFrequency=string,recurrenceUntil=string} true "Gathering schedule + reminder + recurrence options"
// @Success 200
// @Router /events/{eventId}/schedule [post]
func scheduleEvent(c *gin.Context) {
	payload := struct {
		Scheduled             *bool               `json:"scheduled" binding:"required"`
		StartDate             *primitive.DateTime `json:"startDate"`
		EndDate               *primitive.DateTime `json:"endDate"`
		Summary               string              `json:"summary"`
		Timezone              string              `json:"timezone"`
		ReminderEnabled       bool                `json:"reminderEnabled"`
		ReminderLeadTimeHours int                 `json:"reminderLeadTimeHours"`
		// Recurrence (C5): empty/"none" = a one-off gathering.
		RecurrenceFrequency string              `json:"recurrenceFrequency"`
		RecurrenceUntil     *primitive.DateTime `json:"recurrenceUntil"`
		// Optional venue, settable when confirming the time as well as at
		// creation. Absent (nil) leaves whatever the event already has.
		Location *string `json:"location"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	// Validate the recurrence frequency (treat "none" as unset for convenience).
	if payload.RecurrenceFrequency == "none" {
		payload.RecurrenceFrequency = ""
	}
	recurrenceFreq := models.RecurrenceFrequency(payload.RecurrenceFrequency)
	switch recurrenceFreq {
	case models.RecurrenceNone, models.RecurrenceWeekly, models.RecurrenceBiweekly, models.RecurrenceMonthly:
		// ok
	default:
		c.JSON(http.StatusBadRequest, responses.Error{Error: "invalid recurrenceFrequency"})
		return
	}

	eventId := c.Param("eventId")
	event, eventErr := db.GetEventByEitherId(eventId)
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}

	// Scheduling is a manage action, on the same footing as editing the event
	// and running its polls.
	if !requireEventManager(c, event) {
		return
	}

	var update bson.M
	if *payload.Scheduled {
		if payload.StartDate == nil || payload.EndDate == nil {
			c.JSON(http.StatusBadRequest, responses.Error{Error: "startDate and endDate are required when scheduling"})
			return
		}
		summary := payload.Summary
		if summary == "" {
			summary = event.Name
		}
		scheduledEvent := models.CalendarEvent{
			Summary:   summary,
			StartDate: *payload.StartDate,
			EndDate:   *payload.EndDate,
		}
		reminder := models.GatheringReminder{
			Enabled:       payload.ReminderEnabled,
			LeadTimeHours: clampLeadTimeHours(payload.ReminderLeadTimeHours),
			Timezone:      payload.Timezone,
			// SentAt intentionally left nil so (re)scheduling re-arms the reminder
		}
		set := bson.M{
			"scheduledEvent":    scheduledEvent,
			"gatheringReminder": reminder,
		}
		if location := trimmedLocation(payload.Location); location != nil {
			set["location"] = *location
		}
		if recurrenceFreq != models.RecurrenceNone {
			set["gatheringRecurrence"] = models.GatheringRecurrence{
				Frequency: recurrenceFreq,
				Until:     payload.RecurrenceUntil,
			}
			update = bson.M{"$set": set}
		} else {
			// Non-recurring: make sure any prior recurrence is cleared.
			update = bson.M{"$set": set, "$unset": bson.M{"gatheringRecurrence": ""}}
		}
	} else {
		// Cancel the gathering: drop the confirmed time + reminder + recurrence
		update = bson.M{"$unset": bson.M{
			"scheduledEvent":      "",
			"gatheringReminder":   "",
			"gatheringRecurrence": "",
		}}
	}

	if _, err := db.EventsCollection.UpdateOne(context.Background(), bson.M{"_id": event.Id}, update); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// icsFilename turns an event name into a safe .ics download filename.
func icsFilename(name string) string {
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-' || r == '_':
			return '-'
		default:
			return -1
		}
	}, name)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "gathering"
	}
	return slug + ".ics"
}

// @Summary Downloads an .ics calendar file for the event's confirmed gathering
// @Description Universal "add to calendar" — returns a text/calendar file for the gathering's locked-in time. No auth required so any invitee (incl. members without a Google account) can add it. 404 if the event has no confirmed gathering yet.
// @Tags events
// @Produce text/calendar
// @Param eventId path string true "Event ID"
// @Success 200
// @Router /events/{eventId}/ics [get]
func getEventIcs(c *gin.Context) {
	eventId := c.Param("eventId")
	event, err := db.GetEventByEitherId(eventId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return
	}
	if event.ScheduledEvent == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.GatheringNotScheduled})
		return
	}

	ics, err := calendar.GenerateEventICS(event)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", icsFilename(event.Name)))
	c.Data(http.StatusOK, "text/calendar; charset=utf-8", ics)
}
