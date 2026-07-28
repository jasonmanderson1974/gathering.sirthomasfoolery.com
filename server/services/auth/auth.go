package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/utils"
)

// GoogleIdTokenInfo holds the claims returned by Google's tokeninfo endpoint
// after it has validated an ID token's signature and expiry. Google returns
// the numeric claims (aud, exp, ...) as strings.
type GoogleIdTokenInfo struct {
	// Aud is the OAuth client ID the token was issued for.
	Aud string `json:"aud"`
	// Iss is the token issuer (accounts.google.com).
	Iss string `json:"iss"`
	// Email is the verified email address on the account.
	Email string `json:"email"`
	// EmailVerified is "true" when Google has verified the email.
	EmailVerified string `json:"email_verified"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	// Error is set by tokeninfo when the token is invalid.
	Error string `json:"error"`
}

// VerifyGoogleIdToken validates a Google-issued ID token and returns its
// verified claims.
//
// The ID token is validated by calling Google's tokeninfo endpoint, which
// checks the RS256 signature against Google's public keys and rejects expired
// tokens. We then verify that Google is the issuer and that the token's
// audience matches one of *our* OAuth client IDs (web/iOS/Android), so a valid
// Google token minted for some other app can't be replayed here.
//
// This must be used for any ID token that can originate from a client (e.g.
// the mobile sign-in endpoint). Decoding the JWT body without verifying its
// signature would let an attacker forge arbitrary claims (such as another
// user's email) and sign in as anyone.
func VerifyGoogleIdToken(idToken string, allowedAuds []string) (GoogleIdTokenInfo, error) {
	if strings.TrimSpace(idToken) == "" {
		return GoogleIdTokenInfo{}, errors.New("id token is empty")
	}

	// Collect the non-empty client IDs we accept as audiences.
	accepted := make([]string, 0, len(allowedAuds))
	for _, aud := range allowedAuds {
		if strings.TrimSpace(aud) != "" {
			accepted = append(accepted, aud)
		}
	}
	if len(accepted) == 0 {
		return GoogleIdTokenInfo{}, errors.New("no OAuth client IDs are configured to validate the token audience")
	}

	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	resp, err := utils.HTTPClient.Get(endpoint)
	if err != nil {
		return GoogleIdTokenInfo{}, fmt.Errorf("failed to reach token verification endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var info GoogleIdTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return GoogleIdTokenInfo{}, fmt.Errorf("failed to decode token verification response: %w", err)
	}

	// Google returns a non-200 status (and/or an "error" field) for invalid or
	// expired tokens.
	if resp.StatusCode != http.StatusOK || info.Error != "" {
		return GoogleIdTokenInfo{}, fmt.Errorf("id token rejected by Google (status %d): %s", resp.StatusCode, info.Error)
	}

	// Ensure the token was minted for one of our OAuth clients. Without this an
	// attacker could present a valid Google token issued for a different app.
	audOk := false
	for _, aud := range accepted {
		if info.Aud == aud {
			audOk = true
			break
		}
	}
	if !audOk {
		return GoogleIdTokenInfo{}, fmt.Errorf("id token audience mismatch")
	}

	// Ensure the issuer is Google.
	if info.Iss != "accounts.google.com" && info.Iss != "https://accounts.google.com" {
		return GoogleIdTokenInfo{}, fmt.Errorf("unexpected id token issuer: %s", info.Iss)
	}

	return info, nil
}

// Returns access, refresh, and id tokens from the auth code
func GetTokensFromAuthCode(code string, scope string, origin string, calendarType models.CalendarType) (TokenResponse, error) {
	clientId, clientSecret := getCredentialsFromCalendarType(calendarType)
	tokenEndpoint := getTokenEndpointFromCalendarType(calendarType)

	// Call Google oauth token endpoint
	redirectUri := fmt.Sprintf("%s/auth", origin)
	values := url.Values{
		"client_id":     {clientId},
		"client_secret": {clientSecret},
		"code":          {code},
		"scope":         {scope},
		"redirect_uri":  {redirectUri},
		"grant_type":    {"authorization_code"},
	}
	resp, err := utils.HTTPClient.PostForm(
		tokenEndpoint,
		values,
	)
	if err != nil {
		logger.StdErr.Println(err)
		return TokenResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var res TokenResponse

	// A body we can't parse is not an empty token — say so rather than carrying
	// on with zero values and failing further down with no explanation.
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		logger.StdErr.Println(err)
		return TokenResponse{}, fmt.Errorf("token endpoint returned an unparseable body: %w", err)
	}
	if len(res.Error) > 0 {
		data, _ := json.MarshalIndent(res, "", "  ")
		logger.StdErr.Println(string(data))
		return res, fmt.Errorf("token endpoint error: %s", string(data))
	}

	return res, nil
}

func RefreshAccessToken(accountAuth *models.OAuth2CalendarAuth, calendarType models.CalendarType) (AccessTokenResponse, error) {
	clientId, clientSecret := getCredentialsFromCalendarType(calendarType)
	tokenEndpoint := getTokenEndpointFromCalendarType(calendarType)
	values := url.Values{
		"client_id":     {clientId},
		"client_secret": {clientSecret},
		"refresh_token": {string(accountAuth.RefreshToken)},
		"scope":         {accountAuth.Scope},
		"grant_type":    {"refresh_token"},
	}

	resp, err := utils.HTTPClient.PostForm(
		tokenEndpoint,
		values,
	)
	if err != nil {
		return AccessTokenResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var res AccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return AccessTokenResponse{}, fmt.Errorf("access token endpoint returned an unparseable body: %w", err)
	}

	// A refused refresh is a 400 with a normal JSON body, so it decodes cleanly.
	// Without this check the caller gets an empty access token and no reason for
	// it, and only notices when the calendar fetch fails further down.
	if len(res.Error) > 0 {
		if len(res.ErrorDescription) > 0 {
			return res, fmt.Errorf("access token endpoint error: %s: %s", res.Error, res.ErrorDescription)
		}
		return res, fmt.Errorf("access token endpoint error: %s", res.Error)
	}

	return res, nil
}

type RefreshAccessTokenData struct {
	TokenResponse AccessTokenResponse
	Email         string
	CalendarType  models.CalendarType
	Error         *interface{}
}

func RefreshAccessTokenAsync(email string, accountAuth *models.OAuth2CalendarAuth, calendarType models.CalendarType, c chan RefreshAccessTokenData) {
	// Recover from panics. The account is carried on the failure paths too, so
	// the receiver can say *which* account failed and not just that one did.
	defer func() {
		if err := recover(); err != nil {
			c <- RefreshAccessTokenData{Email: email, CalendarType: calendarType, Error: &err}
		}
	}()

	tokenResponse, err := RefreshAccessToken(accountAuth, calendarType)
	if err != nil {
		var errIface interface{} = err
		c <- RefreshAccessTokenData{Email: email, CalendarType: calendarType, Error: &errIface}
		return
	}

	c <- RefreshAccessTokenData{tokenResponse, email, calendarType, nil}
}

// If access token has expired, get a new token for the primary account as well as all other calendar accounts, update the user object, and save it to the database
// `accounts` specifies for which accounts to refresh access tokens. If `accounts` is nil or empty, then update tokens for all accounts
func RefreshUserTokenIfNecessary(u *models.User, accounts models.Set[string]) {
	refreshTokenChan := make(chan RefreshAccessTokenData)
	numAccountsToUpdate := 0

	// If `accounts` is nil, then update tokens for all accounts
	updateAllAccounts := len(accounts) == 0

	// Refresh calendar account access tokens if necessary
	for accountKey, account := range u.CalendarAccounts {
		if account.OAuth2CalendarAuth != nil { // Only refresh access tokens for OAuth2 calendar accounts
			accountAuth := account.OAuth2CalendarAuth

			if _, ok := accounts[accountKey]; ok || updateAllAccounts {
				if time.Now().After(accountAuth.AccessTokenExpireDate.Time()) && len(accountAuth.RefreshToken) > 0 {
					go RefreshAccessTokenAsync(account.Email, accountAuth, account.CalendarType, refreshTokenChan)
					numAccountsToUpdate++
				}
			}
		}
	}

	// Update access tokens as responses are received
	for i := 0; i < numAccountsToUpdate; i++ {
		res := <-refreshTokenChan

		// Log rather than swallow: a refresh that keeps failing (revoked consent,
		// expired refresh token) is invisible from the outside — the user just
		// sees an account with no events — so the reason needs to land somewhere.
		if res.Error != nil {
			logger.StdErr.Printf("failed to refresh access token for %s (%s): %v", res.Email, res.CalendarType, *res.Error)
			continue
		}

		accessTokenExpireDate := utils.GetAccessTokenExpireDate(res.TokenResponse.ExpiresIn)

		calendarAccountKey := utils.ActualCalendarAccountMapKey(u, res.Email, res.CalendarType)
		if calendarAccountKey == "" {
			calendarAccountKey = utils.GetCalendarAccountKey(res.Email, res.CalendarType)
		}
		if calendarAccount, ok := u.CalendarAccounts[calendarAccountKey]; ok {
			calendarAccount.OAuth2CalendarAuth.AccessToken = models.EncryptedString(res.TokenResponse.AccessToken)
			calendarAccount.OAuth2CalendarAuth.AccessTokenExpireDate = primitive.NewDateTimeFromTime(accessTokenExpireDate)
			u.CalendarAccounts[calendarAccountKey] = calendarAccount
		}
	}

	// Update user object if accounts were updated
	if numAccountsToUpdate > 0 {
		// Only the refreshed access tokens changed. Writing the whole user
		// document back would revert anything edited while the refresh was in
		// flight — the lost-update pattern A16 fixed for events.
		if err := db.SetUserCalendarAccounts(u.Id, u.CalendarAccounts); err != nil {
			// The in-memory tokens are still good, so the caller's calendar
			// fetch succeeds; the next request just refreshes again.
			logger.StdErr.Println("failed to persist refreshed access tokens:", err)
		}
	}
}

func getCredentialsFromCalendarType(calendarType models.CalendarType) (string, string) {
	switch calendarType {
	case models.GoogleCalendarType:
		return os.Getenv("CLIENT_ID"), os.Getenv("CLIENT_SECRET")
	case models.OutlookCalendarType:
		return os.Getenv("MICROSOFT_CLIENT_ID"), os.Getenv("MICROSOFT_CLIENT_SECRET")
	}

	return "", ""
}

func getTokenEndpointFromCalendarType(calendarType models.CalendarType) string {
	switch calendarType {
	case models.GoogleCalendarType:
		return "https://oauth2.googleapis.com/token"
	case models.OutlookCalendarType:
		return "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	}

	return ""
}
