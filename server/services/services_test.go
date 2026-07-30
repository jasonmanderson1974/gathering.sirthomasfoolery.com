package services

import (
	"bytes"
	"errors"
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

// countingTransport answers every request with one canned response and records
// how many it saw, which is how "the doomed round trip was skipped" is asserted
// rather than assumed.
type countingTransport struct {
	status int
	body   string
	calls  int
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.calls++
	return &http.Response{
		StatusCode: c.status,
		Body:       io.NopCloser(bytes.NewBufferString(c.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

// userWithExpiredGoogleToken returns a user and the very OAuth2CalendarAuth
// pointer stored on their account — which is what the real callers
// (contacts.SearchContacts, microsoftgraph.GetUserInfo) hand to CallApi, and
// what refreshFailureFor matches on.
func userWithExpiredGoogleToken(email string) (*models.User, *models.OAuth2CalendarAuth) {
	calendarAuth := &models.OAuth2CalendarAuth{
		AccessToken:           "stale-access-token",
		RefreshToken:          "revoked-refresh-token",
		AccessTokenExpireDate: primitive.NewDateTimeFromTime(time.Now().Add(-time.Hour)),
	}
	user := &models.User{
		Id:    primitive.NewObjectID(),
		Email: email,
		CalendarAccounts: map[string]models.CalendarAccount{
			email + "_google": {
				CalendarType:       models.GoogleCalendarType,
				Email:              email,
				OAuth2CalendarAuth: calendarAuth,
			},
		},
	}
	return user, calendarAuth
}

// H5: the whole point. A refresh is only attempted for an already-expired
// token, so a failed one leaves nothing usable — calling anyway spends a round
// trip to get a provider 401 that names no cause.
func TestCallApiReportsAFailedRefreshInsteadOfCallingWithADeadToken(t *testing.T) {
	tokenEndpoint := &countingTransport{
		status: http.StatusBadRequest,
		body:   `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`,
	}
	originalRefresh := utils.HTTPClient.Transport
	utils.HTTPClient.Transport = tokenEndpoint
	t.Cleanup(func() { utils.HTTPClient.Transport = originalRefresh })

	api := &countingTransport{status: http.StatusUnauthorized, body: `{"error":"unauthorized"}`}
	originalAPI := http.DefaultClient.Transport
	http.DefaultClient.Transport = api
	t.Cleanup(func() { http.DefaultClient.Transport = originalAPI })

	user, calendarAuth := userWithExpiredGoogleToken("revoked@example.test")

	response, err := CallApi(user, calendarAuth, "GET", "https://people.googleapis.com/v1/people:searchContacts", nil)
	if err == nil {
		t.Fatal("expected an error when the token refresh failed, got nil")
	}
	if response != nil {
		t.Error("expected no response when the call is skipped")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error should carry the refresh reason, not an opaque 401, got: %v", err)
	}
	if api.calls != 0 {
		t.Errorf("expected the API call to be skipped, it was made %d time(s)", api.calls)
	}
}

// The other side of the guard: an account whose token is still valid is never
// refreshed, so it must not be caught by the new check.
func TestCallApiStillCallsWhenNoRefreshWasNeeded(t *testing.T) {
	api := &countingTransport{status: http.StatusOK, body: `{"results":[]}`}
	originalAPI := http.DefaultClient.Transport
	http.DefaultClient.Transport = api
	t.Cleanup(func() { http.DefaultClient.Transport = originalAPI })

	user, calendarAuth := userWithExpiredGoogleToken("valid@example.test")
	calendarAuth.AccessTokenExpireDate = primitive.NewDateTimeFromTime(time.Now().Add(time.Hour))

	response, err := CallApi(user, calendarAuth, "GET", "https://people.googleapis.com/v1/people:searchContacts", nil)
	if err != nil {
		t.Fatalf("expected the call to proceed, got: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if api.calls != 1 {
		t.Errorf("expected exactly one API call, got %d", api.calls)
	}
}

// A failure on one account must not block a call made with another account's
// auth — the map is keyed per account and so is the check.
func TestRefreshFailureForIgnoresAnotherAccountsFailure(t *testing.T) {
	user, calendarAuth := userWithExpiredGoogleToken("mine@example.test")
	other := &models.OAuth2CalendarAuth{AccessToken: "other"}
	user.CalendarAccounts["other@example.test_google"] = models.CalendarAccount{
		CalendarType:       models.GoogleCalendarType,
		Email:              "other@example.test",
		OAuth2CalendarAuth: other,
	}

	failures := map[string]error{"other@example.test_google": errors.New("invalid_grant")}

	if err := refreshFailureFor(user, calendarAuth, failures); err != nil {
		t.Errorf("another account's failure must not stop this one, got: %v", err)
	}
	if err := refreshFailureFor(user, other, failures); err == nil {
		t.Error("the failing account's own auth should report the failure")
	}
}

// Pointer identity is what maps an OAuth2CalendarAuth back to an account key.
// An equal-by-value copy is a different account's business, and treating it as
// this one's would stop a call that had no reason to be stopped.
func TestRefreshFailureForMatchesOnIdentityNotValue(t *testing.T) {
	user, calendarAuth := userWithExpiredGoogleToken("copy@example.test")
	failures := map[string]error{"copy@example.test_google": errors.New("invalid_grant")}

	if err := refreshFailureFor(user, calendarAuth, failures); err == nil {
		t.Fatal("the stored pointer should match its own account")
	}

	duplicate := *calendarAuth
	if err := refreshFailureFor(user, &duplicate, failures); err != nil {
		t.Errorf("a copy is not the stored auth and must not match, got: %v", err)
	}
}

// The empty case has to be cheap and silent: no failures means every call
// proceeds exactly as before.
func TestRefreshFailureForReturnsNilWhenNothingFailed(t *testing.T) {
	user, calendarAuth := userWithExpiredGoogleToken("clean@example.test")

	if err := refreshFailureFor(user, calendarAuth, map[string]error{}); err != nil {
		t.Errorf("expected nil for an empty failure map, got: %v", err)
	}
}
