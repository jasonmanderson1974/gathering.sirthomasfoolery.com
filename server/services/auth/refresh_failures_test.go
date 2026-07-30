package auth

import (
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
)

// logger.StdOut/StdErr are nil until logger.Init runs, which only main.go does.
// The tests below are the first in this package to reach a log line — the
// failure path logs before it returns — so without this they segfault inside
// log.Printf rather than failing.
func TestMain(m *testing.M) {
	logger.Init(io.Discard)
	os.Exit(m.Run())
}

// H5: B8 made a failed refresh *loggable*; these pin the half that makes it
// actionable — the caller gets the reason back and can stop instead of calling
// an API with a token it already knows is dead.
//
// Every test here drives a user whose refreshes all fail, which is deliberately
// the one path that never reaches Mongo: with no account refreshed there is
// nothing to persist, so the whole function is exercisable without a database.

func expiredGoogleUser(t *testing.T, email string) *models.User {
	t.Helper()
	return &models.User{
		Id:    primitive.NewObjectID(),
		Email: email,
		CalendarAccounts: map[string]models.CalendarAccount{
			email + "_google": {
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
}

func TestRefreshUserTokenIfNecessaryReturnsTheFailureKeyedByAccount(t *testing.T) {
	withStubbedTokenEndpoint(t, http.StatusBadRequest,
		`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)

	user := expiredGoogleUser(t, "revoked@example.test")
	failures := RefreshUserTokenIfNecessary(user, nil)

	if len(failures) != 1 {
		t.Fatalf("expected exactly one failure, got %d: %v", len(failures), failures)
	}
	err, ok := failures["revoked@example.test_google"]
	if !ok {
		t.Fatalf("failure should be keyed by the calendar account key, got keys: %v", failures)
	}
	// The point of returning an error rather than a bool: the caller can say why.
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("returned error should carry the OAuth reason, got: %v", err)
	}
}

// The failure map is the caller's stop signal, so a clean run must leave it
// empty — otherwise every caller would refuse to work.
func TestRefreshUserTokenIfNecessaryReportsNoFailureForAnUnexpiredToken(t *testing.T) {
	withStubbedTokenEndpoint(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)

	user := expiredGoogleUser(t, "unexpired@example.test")
	account := user.CalendarAccounts["unexpired@example.test_google"]
	account.OAuth2CalendarAuth.AccessTokenExpireDate = primitive.NewDateTimeFromTime(time.Now().Add(time.Hour))

	// No refresh is attempted at all, so the stub above is never reached.
	if failures := RefreshUserTokenIfNecessary(user, nil); len(failures) != 0 {
		t.Errorf("a token that has not expired must not be reported as failed, got: %v", failures)
	}
}

// An account excluded by the `accounts` filter must not be refreshed, and so
// must not appear in the failures either — a caller asking about one account
// should not be told a different one is broken.
func TestRefreshUserTokenIfNecessaryHonoursTheAccountsFilter(t *testing.T) {
	withStubbedTokenEndpoint(t, http.StatusBadRequest, `{"error":"invalid_grant"}`)

	user := expiredGoogleUser(t, "filtered@example.test")
	failures := RefreshUserTokenIfNecessary(user, models.Set[string]{"someone.else@example.test_google": {}})

	if len(failures) != 0 {
		t.Errorf("expected no failures for an account outside the filter, got: %v", failures)
	}
}

// asRefreshError exists because the async wrapper carries `*interface{}` to
// accommodate a recovered panic. The normal path is already an error and must
// come through unwrapped, or B8's OAuth reason is reduced to its String().
func TestAsRefreshErrorPreservesTheOriginalError(t *testing.T) {
	original := errors.New("access token endpoint error: invalid_grant")

	got := asRefreshError(original)
	if !errors.Is(got, original) {
		t.Errorf("expected the original error to survive unwrapped, got %v (%T)", got, got)
	}
}

func TestAsRefreshErrorWrapsANonError(t *testing.T) {
	// What a recovered panic looks like: a bare value, not an error.
	got := asRefreshError("runtime error: invalid memory address")
	if got == nil {
		t.Fatal("expected a non-nil error for a panic value")
	}
	if !strings.Contains(got.Error(), "invalid memory address") {
		t.Errorf("panic value should survive into the message, got: %v", got)
	}
}
