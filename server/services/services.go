package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/services/auth"
)

// refreshFailureFor reports the reason this account's token refresh failed, or
// nil if it did not fail.
//
// calendarAuth carries no account key, so pointer identity is what maps it back
// to one: callers hand us the very pointer stored on user.CalendarAccounts, and
// RefreshUserTokenIfNecessary writes refreshed tokens through it. Matching on
// the auth's email instead would be guesswork — an OAuth2CalendarAuth doesn't
// carry one.
func refreshFailureFor(user *models.User, calendarAuth *models.OAuth2CalendarAuth, failures map[string]error) error {
	for key, err := range failures {
		if account, ok := user.CalendarAccounts[key]; ok && account.OAuth2CalendarAuth == calendarAuth {
			return err
		}
	}
	return nil
}

// Calls the given url with the given method using the user's OAuth 2 access token.
// Set user to nil if refreshing the token is not necessary
func CallApi(user *models.User, calendarAuth *models.OAuth2CalendarAuth, method string, url string, body *bson.M) (*http.Response, error) {
	if user != nil {
		failures := auth.RefreshUserTokenIfNecessary(user, nil)

		// A refresh is only attempted for an access token that has already
		// expired, so one that failed leaves nothing usable behind: the call
		// below would spend a round trip to come back as a provider 401 that
		// names no cause. Report the cause we already hold instead (H5).
		if err := refreshFailureFor(user, calendarAuth, failures); err != nil {
			logger.StdErr.Println("skipping API call, token refresh failed:", err)
			return nil, err
		}
	}

	// Format body as a buffer if not nil
	var bodyBuffer *bytes.Buffer
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			// The function already returns an error; swallowing this one sent
			// an empty body upstream and made the failure the provider's fault.
			logger.StdErr.Println(err)
			return nil, err
		}
		bodyBuffer = bytes.NewBuffer(bodyBytes)
	} else {
		bodyBuffer = nil
	}

	// Construct request
	var req *http.Request
	var err error
	if bodyBuffer != nil {
		req, err = http.NewRequest(method, url, bodyBuffer)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", calendarAuth.AccessToken))

	// Execute request
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}

	return response, nil
}
