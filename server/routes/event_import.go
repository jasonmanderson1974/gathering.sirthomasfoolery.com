package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/responses"
)

// @Summary Import an event from a remote instance
// @Tags events
// @Accept json
// @Produce json
// @Param payload body object{url=string} true "Object containing the URL of the remote event"
// @Success 201 {object} object{eventId=string,shortId=string}
// @Router /events/import [post]
func importEvent(c *gin.Context) {
	payload := struct {
		URL string `json:"url" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	userInterface, _ := c.Get("authUser")
	user := userInterface.(*models.User)

	// Guests may respond to events but not create them (incl. via import).
	if !user.EffectiveRole().CanCreateEvents() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	// Parse the URL to extract base URL and event ID
	parsed, err := url.Parse(payload.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		c.JSON(http.StatusBadRequest, responses.Error{Error: "invalid-url"})
		return
	}

	// Block private/internal IP addresses to prevent SSRF
	hostname := parsed.Hostname()
	ips, err := net.LookupIP(hostname)
	if err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: "invalid-url"})
		return
	}
	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			c.JSON(http.StatusBadRequest, responses.Error{Error: "private-address"})
			return
		}
	}

	// Extract event ID from path (e.g., /e/abc123 or /g/abc123)
	pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(pathParts) < 2 || (pathParts[0] != "e" && pathParts[0] != "g") {
		c.JSON(http.StatusBadRequest, responses.Error{Error: "invalid-url"})
		return
	}
	remoteEventId := pathParts[1]
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Fetch the remote event
	eventResp, err := httpClient.Get(fmt.Sprintf("%s/api/events/%s", baseURL, remoteEventId))
	if err != nil {
		c.JSON(http.StatusBadGateway, responses.Error{Error: "remote-fetch-failed"})
		return
	}
	defer func() { _ = eventResp.Body.Close() }()

	if eventResp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, responses.Error{Error: "remote-event-not-found"})
		return
	}

	eventBody, err := io.ReadAll(eventResp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, responses.Error{Error: "remote-fetch-failed"})
		return
	}

	var remoteEvent models.Event
	if err := json.Unmarshal(eventBody, &remoteEvent); err != nil {
		c.JSON(http.StatusBadGateway, responses.Error{Error: "remote-fetch-failed"})
		return
	}

	// Build a name lookup from the event's responses map (remote authenticated users)
	remoteNameMap := make(map[string]string)
	for key, resp := range remoteEvent.ResponsesMap {
		if resp != nil && resp.User != nil && resp.User.FirstName != "" {
			remoteNameMap[key] = resp.User.FirstName
		}
	}

	// Fetch remote responses with availability data
	var timeMin, timeMax time.Time
	for i, d := range remoteEvent.Dates {
		t := d.Time()
		if i == 0 || t.Before(timeMin) {
			timeMin = t
		}
		if i == 0 || t.After(timeMax) {
			timeMax = t
		}
	}
	// Extend timeMax by 1 day to cover the full range
	timeMax = timeMax.AddDate(0, 0, 1)

	responsesURL := fmt.Sprintf("%s/api/events/%s/responses?timeMin=%s&timeMax=%s",
		baseURL, remoteEventId,
		url.QueryEscape(timeMin.Format(time.RFC3339)),
		url.QueryEscape(timeMax.Format(time.RFC3339)),
	)
	respResp, err := httpClient.Get(responsesURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, responses.Error{Error: "remote-fetch-failed"})
		return
	}
	defer func() { _ = respResp.Body.Close() }()

	respBody, err := io.ReadAll(respResp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, responses.Error{Error: "remote-fetch-failed"})
		return
	}

	remoteResponses := make(map[string]*models.Response)
	if respResp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, responses.Error{Error: "remote-responses-failed"})
		return
	}
	if err := json.Unmarshal(respBody, &remoteResponses); err != nil {
		c.JSON(http.StatusBadGateway, responses.Error{Error: "remote-fetch-failed"})
		return
	}

	// Create local event with new identity
	newId := primitive.NewObjectID()
	shortId, shortIdErr := db.GenerateShortEventId()
	if shortIdErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	numResponses := 0

	remoteEvent.Id = newId
	remoteEvent.OwnerId = user.Id
	remoteEvent.ShortId = &shortId
	remoteEvent.NumResponses = &numResponses
	remoteEvent.Remindees = nil
	remoteEvent.ResponsesMap = nil
	remoteEvent.Comments = nil
	remoteEvent.When2meetHref = nil
	remoteEvent.ScheduledEvent = nil
	remoteEvent.CalendarEventId = ""

	_, err = db.EventsCollection.InsertOne(context.Background(), remoteEvent)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	// Import responses as guest entries
	for key, resp := range remoteResponses {
		name := resp.Name
		if name == "" {
			if n, ok := remoteNameMap[key]; ok {
				name = n
			} else {
				name = key
			}
		}

		eventResponse := models.EventResponse{
			Id:      primitive.NewObjectID(),
			EventId: newId,
			UserId:  name,
			Response: &models.Response{
				Name:               name,
				Availability:       resp.Availability,
				IfNeeded:           resp.IfNeeded,
				ManualAvailability: resp.ManualAvailability,
			},
		}

		_, err := db.EventResponsesCollection.InsertOne(context.Background(), eventResponse)
		if err != nil {
			logger.StdErr.Println(err)
			continue
		}
		*remoteEvent.NumResponses++
	}

	// Update NumResponses on the event. Best-effort: the import itself has
	// succeeded by this point, and a stale count is not worth discarding it.
	if _, err := db.EventsCollection.UpdateOne(context.Background(),
		bson.M{"_id": newId},
		bson.M{"$set": bson.M{"numResponses": remoteEvent.NumResponses}},
	); err != nil {
		logger.StdErr.Println("failed to set numResponses on the imported event:", err)
	}

	c.JSON(http.StatusCreated, gin.H{"eventId": newId.Hex(), "shortId": shortId})
}
