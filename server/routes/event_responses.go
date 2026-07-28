package routes

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/responses"
	"sirtom/server/utils"
)

// @Summary Gets responses for an event, filtering availability to be within the date ranges
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Param timeMin query string true "Lower bound for start time to filter availability by"
// @Param timeMax query string true "Upper bound for end time to filter availability by"
// @Success 200 {object} map[string]models.Response
// @Router /events/{eventId}/responses [get]
func getResponses(c *gin.Context) {
	// Bind query parameters
	payload := struct {
		TimeMin time.Time `form:"timeMin" binding:"required"`
		TimeMax time.Time `form:"timeMax" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	// Fetch event
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

	// Convert to map format and filter availability
	eventResponses, eventResponsesErr := db.GetEventResponses(event.Id.Hex())
	if eventResponsesErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	responsesMap := getResponsesMap(eventResponses)

	// Filter availability slice based on timeMin and timeMax
	for userId, response := range responsesMap {
		subsetAvailability := make([]primitive.DateTime, 0)
		for _, timestamp := range response.Availability {
			if timestamp.Time().Compare(payload.TimeMin) >= 0 && timestamp.Time().Compare(payload.TimeMax) <= 0 {
				subsetAvailability = append(subsetAvailability, timestamp)
			}
		}
		response.Availability = subsetAvailability

		subsetIfNeeded := make([]primitive.DateTime, 0)
		for _, timestamp := range response.IfNeeded {
			if timestamp.Time().Compare(payload.TimeMin) >= 0 && timestamp.Time().Compare(payload.TimeMax) <= 0 {
				subsetIfNeeded = append(subsetIfNeeded, timestamp)
			}
		}
		response.IfNeeded = subsetIfNeeded

		subsetManualAvailability := make(map[primitive.DateTime][]primitive.DateTime)
		for timestamp := range utils.Coalesce(response.ManualAvailability) {
			if timestamp.Time().Compare(payload.TimeMin) >= 0 && timestamp.Time().Compare(payload.TimeMax) <= 0 {
				subsetManualAvailability[timestamp] = (*response.ManualAvailability)[timestamp]
			}
		}
		response.ManualAvailability = &subsetManualAvailability
		responsesMap[userId] = response
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

	// Apply blind-availability privacy filtering, then return.
	c.JSON(http.StatusOK, filterResponsesForBlindAvailability(event, responsesMap, userSesh))
}

// filterResponsesForBlindAvailability applies the blind-availability privacy
// rule and returns the response map that should be sent to the requester. When
// blind availability is off, everyone sees every response. When it's on, the
// owner sees all and everyone else sees only their own response.
//
// E3 removed the `?guestName=` branch: it let anyone read a named respondent's
// availability just by claiming their name, which was the acknowledged
// incognito bypass of blind availability. Callers are always signed in now, so
// the session is the only identity.
func filterResponsesForBlindAvailability(event *models.Event, responsesMap map[string]*models.Response, userSesh string) map[string]*models.Response {
	if !utils.Coalesce(event.BlindAvailabilityEnabled) {
		return responsesMap
	}

	if userSesh == "" {
		return make(map[string]*models.Response)
	}
	if event.OwnerId.Hex() == userSesh {
		return responsesMap
	}

	filteredMap := make(map[string]*models.Response)
	if userResponse, exists := responsesMap[userSesh]; exists {
		filteredMap[userSesh] = userResponse
	}
	return filteredMap
}

// @Summary Updates the current user's availability
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{availability=[]string,ifNeeded=[]string,guest=bool,name=string,useCalendarAvailability=bool,enabledCalendars=map[string][]string,manualAvailability=map[string][]string,calendarOptions=models.CalendarOptions,signUpBlockIds=[]string} true "Object containing info about the event response to update"
// @Success 200
// @Router /events/{eventId}/response [post]
func updateEventResponse(c *gin.Context) {
	payload := struct {
		Availability []primitive.DateTime `json:"availability"`
		IfNeeded     []primitive.DateTime `json:"ifNeeded"`

		// Guest information
		Guest *bool  `json:"guest" binding:"required"`
		Name  string `json:"name"`
		Email string `json:"email"`

		// Calendar availability variables for Availability Groups feature
		UseCalendarAvailability *bool                                        `json:"useCalendarAvailability"`
		EnabledCalendars        *map[string][]string                         `json:"enabledCalendars"`
		ManualAvailability      *map[primitive.DateTime][]primitive.DateTime `json:"manualAvailability"`
		CalendarOptions         *models.CalendarOptions                      `json:"calendarOptions"`

		// Sign up form variables
		SignUpBlockIds []primitive.ObjectID `json:"signUpBlockIds"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}
	payload.Name = sanitizeResponderName(payload.Name)
	// On-behalf entry (a member filling in availability for a spouse without an
	// account) stays; only ANONYMOUS use is gone. But responses are keyed by
	// user-id for members and by display name for guests in the same map, so an
	// ObjectID-shaped name would overwrite a member's response.
	if payload.Guest != nil && *payload.Guest && !validOnBehalfName(payload.Name) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: "invalid-guest-name"})
		return
	}

	session := sessions.Default(c)
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

	// Security check: If blindAvailabilityEnabled is true, non-owners cannot set guest availability
	//NOTE: this ONLY stops a user from setting guest availability from their account (via setSlots), somebody could still
	// go on incognito and set guest availability.
	if utils.Coalesce(event.BlindAvailabilityEnabled) {
		ownerSesh := event.OwnerId.Hex()
		userIdInterface := session.Get("userId")
		var userSesh string
		if userIdInterface != nil {
			userSesh = userIdInterface.(string)
		}

		// If user is logged in and NOT the owner, and they're trying to set guest availability, block it
		if userSesh != "" && ownerSesh != userSesh && *payload.Guest {
			c.JSON(http.StatusForbidden, responses.Error{Error: errs.UserNotEventOwner})
			c.Abort()
			return
		}
	}

	eventResponses, eventResponsesErr := db.GetEventResponses(event.Id.Hex())
	if eventResponsesErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	var userIdString string
	var userHasResponded bool
	// A16: what this request actually changes on the event document, so the
	// write at the end can be scoped to it rather than re-writing the whole
	// event over whatever anyone else has done in the meantime.
	numResponsesDelta := 0
	var signUpResponseKey string
	var signUpResponse *models.SignUpResponse
	if !utils.Coalesce(event.IsSignUpForm) {
		// Populate response differently if guest vs signed in user
		var response models.Response
		if *payload.Guest {
			userIdString = payload.Name

			response = models.Response{
				Name:         payload.Name,
				Email:        payload.Email,
				Availability: payload.Availability,
				IfNeeded:     payload.IfNeeded,
			}
		} else {
			userIdInterface := session.Get("userId")
			if userIdInterface == nil {
				c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
				c.Abort()
				return
			}
			userIdString = userIdInterface.(string)
			userId := utils.StringToObjectID(userIdString)

			response = models.Response{
				UserId:                  userId,
				Availability:            payload.Availability,
				IfNeeded:                payload.IfNeeded,
				UseCalendarAvailability: payload.UseCalendarAvailability,
				EnabledCalendars:        payload.EnabledCalendars,
				CalendarOptions:         payload.CalendarOptions,
			}

		}

		// Check if user has responded to event before (edit response) or not (new response)
		idx, _ := findResponse(eventResponses, userIdString)
		userHasResponded = idx != -1

		// Update event responses
		if userHasResponded {
			// This IS the availability being saved — swallowing the error meant
			// answering 200 to a member whose response was never stored.
			if _, err := db.EventResponsesCollection.UpdateOne(context.Background(), bson.M{
				"_id": eventResponses[idx].Id,
			}, bson.M{
				"$set": bson.M{
					"response": &response,
				},
			}); err != nil {
				logger.StdErr.Println(err)
				c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
				return
			}
		} else {
			if _, err := db.EventResponsesCollection.InsertOne(context.Background(), models.EventResponse{
				UserId:   userIdString,
				Response: &response,
				EventId:  event.Id,
			}); err != nil {
				logger.StdErr.Println(err)
			} else {
				*event.NumResponses++
				numResponsesDelta = 1
			}
		}
	} else {
		var response models.SignUpResponse
		var userIdString string
		// Populate response differently if guest vs signed in user
		if *payload.Guest {
			userIdString = payload.Name
			response = models.SignUpResponse{
				Name:  payload.Name,
				Email: payload.Email,
			}
		} else {
			userIdInterface := session.Get("userId")
			if userIdInterface == nil {
				c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
				c.Abort()
				return
			}
			userIdString = userIdInterface.(string)
			response = models.SignUpResponse{
				UserId: utils.StringToObjectID(userIdString),
			}
		}

		// Enforce block capacity server-side: confirmed within capacity, the rest
		// waitlisted (C9). Authoritative — the client can't overfill a slot.
		response.SignUpBlockIds, response.WaitlistBlockIds = assignSignUpBlocks(event, userIdString, payload.SignUpBlockIds)

		// Check if user has responded to event before (edit response) or not (new response)
		_, userHasResponded = event.SignUpResponses[userIdString]

		// Update event responses
		if event.SignUpResponses == nil {
			event.SignUpResponses = make(map[string]*models.SignUpResponse)
		}
		event.SignUpResponses[userIdString] = &response
		signUpResponseKey, signUpResponse = userIdString, &response
	}

	// Send notification emails
	if utils.Coalesce(event.NotificationsEnabled) && !userHasResponded && userIdString != event.OwnerId.Hex() {
		// Send email asynchronously
		go func() {
			// Recover from panics
			defer func() {
				if err := recover(); err != nil {
					logger.StdErr.Println(err)
				}
			}()

			creator, creatorErr := db.GetUserById(event.OwnerId.Hex())
			if creatorErr != nil {
				logger.StdErr.Println(creatorErr)
				return
			}
			if creator == nil {
				return
			}

			var respondentName string
			if *payload.Guest {
				respondentName = payload.Name
			} else {
				respondent, respondentErr := db.GetUserById(userIdString)
				if respondentErr != nil {
					logger.StdErr.Println(respondentErr)
					return
				}
				if respondent == nil {
					return
				}
				respondentName = fmt.Sprintf("%s %s", respondent.FirstName, respondent.LastName)
			}

			subject, body := buildSomeoneRespondedEmail(
				creator.FirstName, respondentName, event.Name, eventURLFor(event),
			)
			if err := utils.SendEmail(creator.Email, subject, body, "text/html"); err != nil {
				logger.StdErr.Println("someone-responded email failed:", err)
			}
		}()
	}

	// Send email after X responses
	sendEmailAfterXResponses := utils.Coalesce(event.SendEmailAfterXResponses)
	// A16: claim the send with a compare-and-set instead of flipping the field
	// in memory and hoping the write lands first. Two people responding at the
	// same moment could both read the threshold as unmet-but-now-met; only the
	// one that actually flips it in the database sends.
	disarmed := false
	if sendEmailAfterXResponses > 0 && !userHasResponded && sendEmailAfterXResponses == len(eventResponses)+1 { // We add 1 because eventResponses is the old event responses before the current user is added
		won, err := db.DisarmSendEmailAfterXResponses(event.Id, sendEmailAfterXResponses)
		if err == nil && won {
			disarmed = true
			*event.SendEmailAfterXResponses = -1
		}
	}
	if disarmed {
		// Send email asynchronously
		go func() {
			// Recover from panics
			defer func() {
				if err := recover(); err != nil {
					logger.StdErr.Println(err)
				}
			}()

			creator, creatorErr := db.GetUserById(event.OwnerId.Hex())
			if creatorErr != nil {
				logger.StdErr.Println(creatorErr)
				return
			}
			if creator == nil {
				return
			}

			// +1 because eventResponses is the set from before this response
			subject, body := buildXResponsesEmail(
				creator.FirstName, event.Name, eventURLFor(event), len(eventResponses)+1,
			)
			if err := utils.SendEmail(creator.Email, subject, body, "text/html"); err != nil {
				logger.StdErr.Println("x-responses email failed:", err)
			}
		}()
	}

	// Persist only what this request changed. Writing the whole event back here
	// meant a second responder's snapshot silently undid the first's — and took
	// any RSVP, poll or comment made in between with it (A16).
	if numResponsesDelta != 0 {
		if err := db.IncrementNumResponses(event.Id, numResponsesDelta); err != nil {
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
	}
	if signUpResponse != nil {
		if err := db.SetSignUpResponse(event.Id, signUpResponseKey, signUpResponse, event.SignUpResponses); err != nil {
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{})
}

// @Summary Delete the current user's availability
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{userId=string,guest=bool,name=string} true "Object containing info about the event response to delete"
// @Success 200
// @Router /events/{eventId}/response [delete]
func deleteEventResponse(c *gin.Context) {
	payload := struct {
		UserId string `json:"userId"`
		Guest  *bool  `json:"guest" binding:"required"`
		Name   string `json:"name"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}
	session := sessions.Default(c)
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

	// A16: track what this delete actually changed, so the write below is
	// scoped to it.
	numResponsesDelta := 0
	removedSignUpKey := ""

	if *payload.Guest {
		// Deleting a by-name response is acting on someone else's data, so it
		// is an owner action. This branch previously had NO authorization: any
		// caller could delete the first response matching a supplied name.
		if !requireResponseManager(c, event) {
			return
		}
		if utils.Coalesce(event.IsSignUpForm) {
			delete(event.SignUpResponses, payload.Name)
			removedSignUpKey = payload.Name
		} else {
			// Remove response from array
			for i := range eventResponses {
				if eventResponses[i].Response.Name == payload.Name {
					// Only adjust the count if the response actually went —
					// otherwise numResponses drifts away from the responses.
					if _, err := db.EventResponsesCollection.DeleteOne(context.Background(), bson.M{
						"_id": eventResponses[i].Id,
					}); err != nil {
						logger.StdErr.Println(err)
						c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
						return
					}
					*event.NumResponses--
					numResponsesDelta = -1
					break
				}
			}
		}
	} else {
		userIdInterface := session.Get("userId")
		if userIdInterface == nil {
			c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
			c.Abort()
			return
		}
		userIdString := userIdInterface.(string)

		// Don't allow user to delete availability of other users if they aren't the owner of the event
		if payload.UserId != userIdString && event.OwnerId.Hex() != userIdString {
			c.JSON(http.StatusForbidden, responses.Error{Error: errs.UserNotEventOwner})
			c.Abort()
			return
		}

		if utils.Coalesce(event.IsSignUpForm) {
			delete(event.SignUpResponses, payload.UserId)
			removedSignUpKey = payload.UserId
		} else {
			// Remove response from array
			for i := range eventResponses {
				if eventResponses[i].UserId == payload.UserId {
					// Only adjust the count if the response actually went —
					// otherwise numResponses drifts away from the responses.
					if _, err := db.EventResponsesCollection.DeleteOne(context.Background(), bson.M{
						"_id": eventResponses[i].Id,
					}); err != nil {
						logger.StdErr.Println(err)
						c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
						return
					}
					*event.NumResponses--
					numResponsesDelta = -1
					break
				}
			}
		}

	}

	// Scoped to what was actually removed, so a concurrent responder isn't
	// undone by this delete (A16).
	var err error
	if numResponsesDelta != 0 {
		err = db.IncrementNumResponses(event.Id, numResponsesDelta)
	}
	if err == nil && removedSignUpKey != "" {
		err = db.DeleteSignUpResponse(event.Id, removedSignUpKey, event.SignUpResponses)
	}
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

// @Summary Rename a guest response
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{oldName=string,newName=string} true "Object containing info about the guest response to rename"
// @Success 200
// @Failure 400 {object} responses.Error "Guest name already exists"
// @Router /events/{eventId}/rename-user [post]
func renameUser(c *gin.Context) {
	payload := struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
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

	// Renaming someone else's response is an owner action; this was previously
	// unauthenticated entirely.
	if !requireResponseManager(c, event) {
		return
	}

	payload.NewName = sanitizeResponderName(payload.NewName)
	if !validOnBehalfName(payload.NewName) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: "invalid-guest-name"})
		return
	}

	// Check if the new name already exists (only if it's different from the old name)
	if payload.NewName != payload.OldName {
		if db.GuestNameExists(event.Id.Hex(), payload.NewName) {
			c.JSON(http.StatusBadRequest, responses.Error{Error: "A guest with this name already exists for this event"})
			return
		}
	}

	// Check if old name is a guest response
	db.UpdateGuestResponseName(event.Id.Hex(), payload.OldName, payload.NewName)

	c.JSON(http.StatusOK, gin.H{})
}

// @Summary Mark the user as having responded to this event
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{email=string} true "Object containing the user's email"
// @Success 200
// @Router /events/{eventId}/responded [post]
func userResponded(c *gin.Context) {
	payload := struct {
		Email string `json:"email" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	// Fetch event
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

	// Update responded boolean for the given email
	if event.Remindees == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.RemindeeEmailNotFound})
		return
	}
	index := utils.Find(*event.Remindees, func(r models.Remindee) bool {
		return r.Email == payload.Email
	})
	if index == -1 {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.RemindeeEmailNotFound})
		return
	}
	if remindeeResponded((*event.Remindees)[index]) {
		// If remindee has already responded, just return and don't update db
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	// Marking them responded is also what stops any further nudges: the
	// scheduler's query skips remindees who have answered.
	//
	// A16: a guarded, positional write. Two remindees clicking their links at
	// once used to write the whole event each, so one flag could be lost — and
	// with it the nudges kept coming. The guard also means only the caller that
	// actually flips it can go on to send the "everyone responded" mail.
	flipped, flipErr := db.MarkRemindeeResponded(event.Id, payload.Email)
	if flipErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if !flipped {
		// Someone else got there first; nothing more to do.
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	// Email owner of event if all remindees have responded. Re-read rather than
	// trusting the snapshot: another remindee may have answered since it was
	// loaded, and deciding "everyone" from stale data is how this either misses
	// the last one or fires early.
	fresh, freshErr := db.GetEventById(event.Id.Hex())
	if freshErr != nil || fresh == nil || fresh.Remindees == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	everyoneResponded := true
	for _, remindee := range *fresh.Remindees {
		if !remindeeResponded(remindee) {
			everyoneResponded = false
			break
		}
	}
	if everyoneResponded {
		// The remindee's own update is already committed above, so a missing
		// owner or a failed send must not turn this into an error for them —
		// an event created by a guest has no owner account at all.
		owner, ownerErr := db.GetUserById(event.OwnerId.Hex())
		if ownerErr != nil || owner == nil || owner.Email == "" {
			logger.StdErr.Println("everyone-responded email skipped:", ownerErr)
		} else {
			subject, body := buildEveryoneRespondedEmail(owner.FirstName, event.Name, eventURLFor(event))
			utils.SendEmailAsync(owner.Email, subject, body, "text/html")
		}
	}

	c.JSON(http.StatusOK, gin.H{})
}

// Helper function to find a response by userId
func findResponse(responses []models.EventResponse, userId string) (int, *models.Response) {
	for i, resp := range responses {
		if resp.UserId == userId {
			return i, resp.Response
		}
	}
	return -1, nil
}

// stripSensitiveUserFields removes fields from a User that should never be
// exposed in the event page API response (calendar accounts, billing info, etc.).
// Email is NOT stripped here as callers handle email visibility separately based
// on the collectEmails setting and owner status.
//
// Phone and Role are stripped unconditionally: an event response only needs to
// identify the respondent (name + picture). A member's phone number belongs to
// the Fellowship directory (/admin/allowlist), not to every event viewer, and
// their access level is nobody's business outside member admin. Neither field
// is read from event responses by the frontend.
func stripSensitiveUserFields(user *models.User) {
	if user == nil {
		return
	}
	user.CalendarAccounts = nil
	user.CalendarOptions = nil
	user.PrimaryAccountKey = nil
	user.Phone = ""
	user.Role = ""
}

// Helper function to get all responses as a map (for backward compatibility)
func getResponsesMap(responses []models.EventResponse) map[string]*models.Response {
	result := make(map[string]*models.Response)
	for _, resp := range responses {
		result[resp.UserId] = resp.Response
	}
	return result
}

// assignSignUpBlocks splits the blocks a user requested into confirmed (within
// the block's capacity) and waitlisted (block already full) — the server-side
// enforcement + waitlist for C9. A block with no capacity set is unlimited. The
// user's own existing signup is excluded from the counts, and any block they
// were already confirmed for keeps its spot (a re-submit never bumps you to the
// waitlist). Order follows the requested slice for determinism.
func assignSignUpBlocks(event *models.Event, userIdString string, requested []primitive.ObjectID) (confirmed, waitlisted []primitive.ObjectID) {
	capacityByBlock := make(map[primitive.ObjectID]*int)
	if event.SignUpBlocks != nil {
		for _, b := range *event.SignUpBlocks {
			capacityByBlock[b.Id] = b.Capacity
		}
	}

	// Confirmed count per block across OTHER users, plus which blocks this user
	// was already confirmed for.
	confirmedCount := make(map[primitive.ObjectID]int)
	alreadyConfirmed := make(map[primitive.ObjectID]bool)
	for uid, resp := range event.SignUpResponses {
		if resp == nil {
			continue
		}
		if uid == userIdString {
			for _, bid := range resp.SignUpBlockIds {
				alreadyConfirmed[bid] = true
			}
			continue // exclude self from the counts
		}
		for _, bid := range resp.SignUpBlockIds {
			confirmedCount[bid]++
		}
	}

	for _, bid := range requested {
		capacity, known := capacityByBlock[bid]
		if !known || capacity == nil || alreadyConfirmed[bid] || confirmedCount[bid] < *capacity {
			confirmed = append(confirmed, bid)
			confirmedCount[bid]++ // reserve the seat for any later duplicate
		} else {
			waitlisted = append(waitlisted, bid)
		}
	}
	return confirmed, waitlisted
}

// clampGuestCount bounds a plus-one count to a sane range. A plus-one only
// makes sense when the responder is (tentatively) attending, so decliners are
// forced to 0.
func clampGuestCount(status models.RsvpStatus, count int) int {
	if status == models.RsvpNo || count < 0 {
		return 0
	}
	const maxGuests = 20
	if count > maxGuests {
		return maxGuests
	}
	return count
}

// objectIDShaped matches a 24-character hex string — the wire form of a Mongo
// ObjectID, and therefore of a member's response key.
var objectIDShaped = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// validOnBehalfName reports whether name is usable as an on-behalf (guest)
// response key. Responses are keyed by user-id for members and by display name
// for guests in the SAME map, so a guest name that looks like an ObjectID can
// collide with — and overwrite — a member's response. Rejecting the shape
// closes that, and is why on-behalf entry can stay open to signed-in users.
func validOnBehalfName(name string) bool {
	return name != "" && !objectIDShaped.MatchString(name)
}

// requireResponseManager reports whether the caller may act on SOMEONE ELSE's
// response — delete it, or rename it. Admins always may; otherwise it is
// whoever manages the event, which is the question requireEventManager already
// answers. Writes the response and returns false if not.
//
// Deliberately delegated rather than re-derived. The first version compared
// user.Id to event.OwnerId directly, which quietly locked members out of legacy
// ownerless events — a nil OwnerId never equals a real id — even though those
// same members CAN edit the event itself. Two spellings of "manages this event"
// drift; one does not.
func requireResponseManager(c *gin.Context, event *models.Event) bool {
	user, ok := authUser(c)
	if !ok {
		return false
	}
	if user.EffectiveRole().CanManageUsers() {
		return true
	}
	return requireEventManager(c, event)
}

// maxResponderNameLength bounds a guest / on-behalf display name. These names
// become map keys on the event document (responses, RSVPs, poll votes), so an
// uncapped one inflates the document on every write path a guest can reach.
const maxResponderNameLength = 100

// sanitizeResponderName trims a display name and bounds it. Rune-aware so a cut
// can't land mid-character.
func sanitizeResponderName(name string) string {
	name = strings.TrimSpace(name)
	r := []rune(name)
	if len(r) > maxResponderNameLength {
		return string(r[:maxResponderNameLength])
	}
	return name
}

// responderKey resolves the caller's identity key for an RSVP or a poll vote.
//
// E3: keyed to the session, always. The old branch that took a caller-supplied
// name meant anyone could RSVP or vote AS anyone — the name WAS the identity
// claim, with nothing to back it. Legacy name-keyed entries still render; they
// simply can't be created or impersonated any more.
func responderKey(c *gin.Context) (key string, userId primitive.ObjectID, ok bool) {
	user, uok := authUser(c)
	if !uok {
		return "", primitive.NilObjectID, false
	}
	return user.Id.Hex(), user.Id, true
}

// @Summary RSVP to a confirmed gathering (going / maybe / no)
// @Description Records the caller's attendance for the event's confirmed gathering. Requires the event to have a locked-in time (scheduledEvent). Open to signed-in users and guests (by name), like availability responses.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{status=string,guestCount=int} true "RSVP status"
// @Success 200
// @Router /events/{eventId}/rsvp [post]
func rsvpToEvent(c *gin.Context) {
	// E3: the RSVP is keyed to the session. `guest`/`name` are gone — they let
	// any caller RSVP as anyone.
	payload := struct {
		Status     models.RsvpStatus `json:"status" binding:"required"`
		GuestCount int               `json:"guestCount"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	if payload.Status != models.RsvpGoing && payload.Status != models.RsvpMaybe && payload.Status != models.RsvpNo {
		c.JSON(http.StatusBadRequest, responses.Error{Error: "invalid-rsvp-status"})
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
	if event.ScheduledEvent == nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.GatheringNotScheduled})
		return
	}

	key, userId, ok := responderKey(c)
	if !ok {
		return
	}

	rsvp := models.Rsvp{
		Status:      payload.Status,
		GuestCount:  clampGuestCount(payload.Status, payload.GuestCount),
		UserId:      userId,
		RespondedAt: primitive.NewDateTimeFromTime(time.Now()),
	}
	// Identity comes from the account, never from the request body.
	if user, err := db.GetUserById(key); err == nil && user != nil {
		rsvp.Email = user.Email
		rsvp.Name = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}

	if event.Rsvps == nil {
		event.Rsvps = make(map[string]*models.Rsvp)
	}
	event.Rsvps[key] = &rsvp

	if _, err := db.EventsCollection.UpdateByID(context.Background(), event.Id, bson.M{"$set": bson.M{"rsvps": event.Rsvps}}); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

// @Summary Remove the caller's RSVP to a gathering
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200
// @Router /events/{eventId}/rsvp [delete]
func deleteRsvp(c *gin.Context) {
	// E3: no payload — you may only clear your OWN RSVP, keyed by session.
	payload := struct{}{}
	if err := c.ShouldBindJSON(&payload); err != nil && err.Error() != "EOF" {
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

	key, _, ok := responderKey(c)
	if !ok {
		return
	}

	if event.Rsvps != nil {
		delete(event.Rsvps, key)
	}

	if _, err := db.EventsCollection.UpdateByID(context.Background(), event.Id, bson.M{"$set": bson.M{"rsvps": event.Rsvps}}); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}
