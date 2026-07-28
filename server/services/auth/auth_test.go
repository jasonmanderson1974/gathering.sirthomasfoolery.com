package auth

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"sirtom/server/models"
	"sirtom/server/utils"
)

// stubTransport answers every outbound request with one canned response, so
// these tests exercise the decode/error path without touching the network.
type stubTransport struct {
	status int
	body   string
}

func (s stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(bytes.NewBufferString(s.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

// withStubbedTokenEndpoint points the shared HTTP client at the stub for the
// duration of one test and restores the real transport afterwards.
func withStubbedTokenEndpoint(t *testing.T, status int, body string) {
	t.Helper()

	original := utils.HTTPClient.Transport
	utils.HTTPClient.Transport = stubTransport{status: status, body: body}
	t.Cleanup(func() { utils.HTTPClient.Transport = original })
}

// The bug B8 fixes: Google and Microsoft report a refused refresh as
// `"error": "invalid_grant"` — a string. Typing the field as an object made the
// decode fail, so the caller was told `json: cannot unmarshal string into Go
// struct field ...` and the real reason was lost.
func TestRefreshAccessTokenReportsTheOAuthErrorCode(t *testing.T) {
	withStubbedTokenEndpoint(t, http.StatusBadRequest,
		`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)

	_, err := RefreshAccessToken(&models.OAuth2CalendarAuth{RefreshToken: "stale"}, models.GoogleCalendarType)
	if err == nil {
		t.Fatal("expected an error when the token endpoint refuses the refresh, got nil")
	}

	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error should name the OAuth error code, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Token has been expired or revoked.") {
		t.Errorf("error should carry the description, got: %v", err)
	}
	// The regression guard: the old typing produced exactly this.
	if strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("the OAuth reason was replaced by a JSON decode error: %v", err)
	}
}

// Microsoft returns the same shape without a description on some failures.
func TestRefreshAccessTokenReportsAnErrorWithNoDescription(t *testing.T) {
	withStubbedTokenEndpoint(t, http.StatusUnauthorized, `{"error":"invalid_client"}`)

	_, err := RefreshAccessToken(&models.OAuth2CalendarAuth{RefreshToken: "stale"}, models.OutlookCalendarType)
	if err == nil {
		t.Fatal("expected an error when the token endpoint refuses the refresh, got nil")
	}
	if !strings.Contains(err.Error(), "invalid_client") {
		t.Errorf("error should name the OAuth error code, got: %v", err)
	}
}

// The other half of the guard: a successful refresh must still succeed, so the
// new error check can't be passing by rejecting everything.
func TestRefreshAccessTokenReturnsTheTokenOnSuccess(t *testing.T) {
	withStubbedTokenEndpoint(t, http.StatusOK,
		`{"access_token":"fresh-token","expires_in":3599,"scope":"calendar","token_type":"Bearer"}`)

	res, err := RefreshAccessToken(&models.OAuth2CalendarAuth{RefreshToken: "good"}, models.GoogleCalendarType)
	if err != nil {
		t.Fatalf("expected a successful refresh, got: %v", err)
	}

	if res.AccessToken != "fresh-token" {
		t.Errorf("access token = %q, want %q", res.AccessToken, "fresh-token")
	}
	if res.ExpiresIn != 3599 {
		t.Errorf("expires_in = %d, want 3599", res.ExpiresIn)
	}
	if res.Error != "" {
		t.Errorf("expected no error field on a successful refresh, got %q", res.Error)
	}
}

// A body that isn't JSON at all (an HTML error page from a proxy, say) must
// still be distinguishable from a refused refresh.
func TestRefreshAccessTokenReportsAnUnparseableBody(t *testing.T) {
	withStubbedTokenEndpoint(t, http.StatusBadGateway, `<html>502 Bad Gateway</html>`)

	_, err := RefreshAccessToken(&models.OAuth2CalendarAuth{RefreshToken: "good"}, models.GoogleCalendarType)
	if err == nil {
		t.Fatal("expected an error for a non-JSON body, got nil")
	}
	if !strings.Contains(err.Error(), "unparseable body") {
		t.Errorf("error should say the body was unparseable, got: %v", err)
	}
}

// RefreshUserTokenIfNecessary logs which account failed, which it can only do
// if the async wrapper carries the account on its failure paths.
func TestRefreshAccessTokenAsyncCarriesTheAccountOnFailure(t *testing.T) {
	withStubbedTokenEndpoint(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)

	c := make(chan RefreshAccessTokenData, 1)
	RefreshAccessTokenAsync("someone@example.com", &models.OAuth2CalendarAuth{RefreshToken: "stale"},
		models.GoogleCalendarType, c)

	res := <-c
	if res.Error == nil {
		t.Fatal("expected the failure to be reported on the channel")
	}
	if res.Email != "someone@example.com" {
		t.Errorf("email = %q, want the account that failed", res.Email)
	}
	if res.CalendarType != models.GoogleCalendarType {
		t.Errorf("calendar type = %q, want %q", res.CalendarType, models.GoogleCalendarType)
	}
}
