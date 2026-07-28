/* The /events group contains all the routes to get and edit events */
package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	eventRouter.POST("", middleware.AuthRequiredIfInviteOnly(), createEvent)
	eventRouter.POST("/import", middleware.AuthRequired(), importEvent)
	eventRouter.PUT("/:eventId", editEvent)
	eventRouter.GET("/:eventId/ids", getEventIds)
	eventRouter.GET("/:eventId", getEvent)
	eventRouter.GET("/:eventId/responses", getResponses)
	eventRouter.POST("/:eventId/response", updateEventResponse)
	eventRouter.DELETE("/:eventId/response", deleteEventResponse)
	eventRouter.POST("/:eventId/rename-user", renameUser)
	eventRouter.POST("/:eventId/responded", userResponded)
	eventRouter.DELETE("/:eventId", middleware.AuthRequired(), deleteEvent)
	eventRouter.POST("/:eventId/duplicate", middleware.AuthRequired(), duplicateEvent)
	eventRouter.POST("/:eventId/archive", middleware.AuthRequired(), archiveEvent)
	eventRouter.POST("/:eventId/schedule", scheduleEvent)
	eventRouter.GET("/:eventId/ics", getEventIcs)
	eventRouter.POST("/:eventId/rsvp", rsvpToEvent)
	eventRouter.DELETE("/:eventId/rsvp", deleteRsvp)
	// The discussion is sign-in-only: anonymous callers can neither read nor
	// write comments (getEvent withholds the list from them entirely).
	eventRouter.POST("/:eventId/comments", middleware.AuthRequired(), addComment)
	eventRouter.PUT("/:eventId/comments/:commentId", middleware.AuthRequired(), editComment)
	eventRouter.DELETE("/:eventId/comments/:commentId", middleware.AuthRequired(), deleteComment)
	eventRouter.POST("/:eventId/comments/:commentId/thread", middleware.AuthRequired(), tagCommentAsThread)
	eventRouter.PATCH("/:eventId/comments/:commentId/thread", middleware.AuthRequired(), setThreadMembersOnly)
	eventRouter.DELETE("/:eventId/comments/:commentId/thread", middleware.AuthRequired(), untagThread)
	eventRouter.POST("/:eventId/polls", createPoll)
	eventRouter.DELETE("/:eventId/polls/:pollId", deletePoll)
	eventRouter.POST("/:eventId/polls/:pollId/vote", votePoll)
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

// @Summary Creates a new event
// @Tags events
// @Accept json
// @Produce json
// @Param payload body object{name=string,duration=float32,dates=[]string,type=models.EventType,isSignUpForm=bool,signUpBlocks=[]models.SignUpBlock,notificationsEnabled=bool,blindAvailabilityEnabled=bool,daysOnly=bool,wholeBlockSelection=bool,remindees=[]string,sendEmailAfterXResponses=int,when2meetHref=string,timeIncrement=int} true "Object containing info about the event to create"
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
		IsSignUpForm *bool                 `json:"isSignUpForm"`
		SignUpBlocks *[]models.SignUpBlock `json:"signUpBlocks"`

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
	session := sessions.Default(c)

	// If user logged in, set owner id to their user id, otherwise set owner id to nil
	userIdInterface := session.Get("userId")
	userId, signedIn := userIdInterface.(string)
	var user *models.User
	var ownerId primitive.ObjectID
	if signedIn {
		var userErr error
		user, userErr = db.GetUserById(userId)
		if userErr != nil {
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
		if user == nil {
			signedIn = false
			ownerId = primitive.NilObjectID
		} else {
			// Guests may respond to events but not create them.
			if !user.EffectiveRole().CanCreateEvents() {
				c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
				return
			}
			ownerId = utils.StringToObjectID(userId)
		}
	} else {
		ownerId = primitive.NilObjectID
	}

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
		IsSignUpForm:             payload.IsSignUpForm,
		SignUpBlocks:             payload.SignUpBlocks,
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
		SignUpResponses:          make(map[string]*models.SignUpResponse),
		NumResponses:             &numResponses,
	}

	// Generate short id
	shortId := db.GenerateShortEventId(event.Id)
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

	// Send slackbot message
	// var creator string
	if signedIn {
		// creator = fmt.Sprintf("%s %s (%s)", user.FirstName, user.LastName, user.Email)
		db.UsersCollection.UpdateOne(context.Background(), bson.M{"_id": ownerId}, bson.M{"$inc": bson.M{"numEventsCreated": 1}})
	} else {
		// creator = "Guest :face_with_open_eyes_and_hand_over_mouth:"
	}
	// slackbot.SendEventCreatedMessage(insertedId, creator, event, len(attendees))

	c.JSON(http.StatusCreated, gin.H{"eventId": insertedId, "shortId": event.ShortId})
}

// @Summary Edits an event based on its id
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{name=string,description=string,duration=float32,dates=[]string,type=models.EventType,signUpBlocks=[]models.SignUpBlock,notificationsEnabled=bool,blindAvailabilityEnabled=bool,daysOnly=bool,wholeBlockSelection=bool,remindees=[]string,sendEmailAfterXResponses=int} true "Object containing info about the event to update"
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
		SignUpBlocks *[]models.SignUpBlock `json:"signUpBlocks"`

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

	// If user logged in, set owner id to their user id, otherwise set owner id to nil
	session := sessions.Default(c)
	userIdInterface := session.Get("userId")
	userId, signedIn := userIdInterface.(string)
	var ownerId primitive.ObjectID
	if signedIn {
		ownerId = utils.StringToObjectID(userId)
	} else {
		ownerId = primitive.NilObjectID
	}

	// If event has an owner id, check if user has permissions to edit event
	if event.OwnerId != primitive.NilObjectID {
		if event.OwnerId != ownerId {
			c.JSON(http.StatusForbidden, responses.Error{Error: errs.UserNotEventOwner})
			return
		}
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
	event.SignUpBlocks = payload.SignUpBlocks
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

	// Update event object
	_, err := db.EventsCollection.UpdateOne(
		context.Background(),
		bson.M{
			"_id": event.Id,
		},
		bson.M{
			"$set": event,
		},
	)

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

	// Populate sign up form fields
	for userId, response := range event.SignUpResponses {
		user, userErr := db.GetUserById(userId)
		if userErr != nil {
			logger.StdErr.Println(userErr)
			continue
		}
		if user == nil {
			if len(response.Name) == 0 {
				// User was deleted
				delete(event.SignUpResponses, userId)
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
		}
		event.SignUpResponses[userId] = response
	}

	// Determine if the requester is the event owner
	ownerSesh := event.OwnerId.Hex()
	session := sessions.Default(c)
	userIdInterface := session.Get("userId")
	var userSesh string
	if userIdInterface != nil {
		userSesh = userIdInterface.(string)
	}
	guestName := c.Query("guestName")
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
	for userId, response := range event.SignUpResponses {
		stripSensitiveUserFields(response.User)
		if !showEmails {
			response.Email = ""
			if response.User != nil {
				response.User.Email = ""
			}
		}
		event.SignUpResponses[userId] = response
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
	} else if guestName != "" {
		// Guest name query parameter exists
		privateFields := []string{"numResponses"}
		partialOmissions := []utils.PartialOmission{
			{
				FieldName: "responses",
				KeepKey:   guestName,
			},
		}
		privatizedResponse, err = utils.PrivatizeEventResponse(event, privateFields, partialOmissions)
	} else {
		// No session, no guest name - remove all private fields
		privateFields := []string{"numResponses", "responses", "remindees"}
		privatizedResponse, err = utils.PrivatizeEventResponse(event, privateFields, []utils.PartialOmission{})
	}

	if err != nil {
		logger.StdErr.Printf("Failed to privatize event response: %v\n", err)
		// Fall back to returning the original event if privatization fails
		c.JSON(http.StatusOK, event)
		return
	}

	// Log response body
	responseJSON, err := json.MarshalIndent(privatizedResponse, "", "  ")
	if err != nil {
		logger.StdErr.Printf("Failed to marshal privatized response for logging: %v\n", err)
	}
	_ = responseJSON
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

	userInterface, _ := c.Get("authUser")
	user := userInterface.(*models.User)

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
			"_id":     objectId,
			"ownerId": user.Id,
		}, bson.M{
			"$set": bson.M{
				"isDeleted": true,
			},
		})
		err = result.Decode(&event)
		if err != nil {
			logger.StdErr.Println(err)
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
	} else {
		// If event has no responses, actually delete the event object
		result := db.EventsCollection.FindOneAndDelete(context.Background(), bson.M{
			"_id":     objectId,
			"ownerId": user.Id,
		})
		err = result.Decode(&event)
		if err != nil {
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
	shortId := db.GenerateShortEventId(event.Id)
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

	eventId := c.Param("eventId")

	objectId, err := primitive.ObjectIDFromHex(eventId)
	if err != nil {
		// eventId is malformatted
		c.Status(http.StatusBadRequest)
		return
	}

	userInterface, _ := c.Get("authUser")
	user := userInterface.(*models.User)

	result := db.EventsCollection.FindOneAndUpdate(context.Background(), bson.M{
		"_id":     objectId,
		"ownerId": user.Id,
	}, bson.M{
		"$set": bson.M{
			"isArchived": payload.Archive,
		},
	})
	var event models.Event
	err = result.Decode(&event)
	if err != nil {
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

	// Scheduling is an owner action. If the event has an owner, only that owner
	// may schedule it (mirrors editEvent). Owner-less (guest-created) events have
	// no owner to check against — on an enforced invite-only instance, require a
	// signed-in member so scheduling isn't an anonymous write (E1); on open/dev
	// instances the guest-event flow stays open.
	if event.OwnerId != primitive.NilObjectID {
		session := sessions.Default(c)
		userId, signedIn := session.Get("userId").(string)
		if !signedIn || utils.StringToObjectID(userId) != event.OwnerId {
			c.JSON(http.StatusForbidden, responses.Error{Error: errs.UserNotEventOwner})
			return
		}
	} else if db.AccessControlEnforced() {
		session := sessions.Default(c)
		if _, signedIn := session.Get("userId").(string); !signedIn {
			c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
			return
		}
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
