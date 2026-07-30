package calendar

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/utils"
)

// logger.StdOut/StdErr are nil until logger.Init runs, which only main.go does.
func TestMain(m *testing.M) {
	logger.Init(io.Discard)
	os.Exit(m.Run())
}

// recordingTransport answers the token endpoint with a canned refusal and
// records every URL it is asked for. The token refresh and the Google calendar
// provider share utils.HTTPClient, so the URLs — not the call count — are what
// distinguish "refreshed and then fetched" from "refreshed and stopped".
type recordingTransport struct {
	urls []string
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.urls = append(r.urls, req.URL.String())
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(bytes.NewBufferString(
			`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)),
		Header:  make(http.Header),
		Request: req,
	}, nil
}

func (r *recordingTransport) sawCalendarFetch() bool {
	for _, u := range r.urls {
		if strings.Contains(u, "googleapis.com/calendar") {
			return true
		}
	}
	return false
}

// H5: a refresh that fails leaves an expired access token behind, so fetching
// with it is a guaranteed 401 whose message says nothing about the cause. The
// account's slot in the map should carry the refresh reason instead.
func TestGetUsersCalendarEventsReportsAFailedRefreshWithoutFetching(t *testing.T) {
	transport := &recordingTransport{}
	original := utils.HTTPClient.Transport
	utils.HTTPClient.Transport = transport
	t.Cleanup(func() { utils.HTTPClient.Transport = original })

	const email = "revoked@example.test"
	const key = email + "_google"
	user := &models.User{
		Id:    primitive.NewObjectID(),
		Email: email,
		CalendarAccounts: map[string]models.CalendarAccount{
			key: {
				CalendarType: models.GoogleCalendarType,
				Email:        email,
				OAuth2CalendarAuth: &models.OAuth2CalendarAuth{
					AccessToken:           "stale-access-token",
					RefreshToken:          "revoked-refresh-token",
					AccessTokenExpireDate: primitive.NewDateTimeFromTime(time.Now().Add(-time.Hour)),
				},
			},
		},
	}

	eventsMap, editedCalendarAccounts := GetUsersCalendarEvents(
		user, models.Set[string]{}, time.Now(), time.Now().Add(24*time.Hour))

	entry, ok := eventsMap[key]
	if !ok {
		t.Fatalf("the account should still appear in the map, got keys: %v", eventsMap)
	}
	if entry.Error == nil {
		t.Fatal("expected the refresh failure to be reported as this account's error")
	}
	if !strings.Contains(entry.Error.Error(), "invalid_grant") {
		t.Errorf("error should name the refresh reason, got: %v", entry.Error)
	}
	// The frontend distinguishes "no events" from "broken account" by the error
	// alone (Event.vue's calendarPermissionGranted check), so the events slice
	// must still be the empty array rather than nil.
	if entry.CalendarEvents == nil {
		t.Error("expected an empty events slice, not nil")
	}
	if len(entry.CalendarEvents) != 0 {
		t.Errorf("expected no events for a broken account, got %d", len(entry.CalendarEvents))
	}

	if transport.sawCalendarFetch() {
		t.Errorf("the calendar fetch should have been skipped, requests were: %v", transport.urls)
	}
	// Nothing was refreshed and no sub-calendar list came back, so there is
	// nothing for the caller to persist.
	if editedCalendarAccounts {
		t.Error("a failed refresh should not report the calendar accounts as edited")
	}
}
