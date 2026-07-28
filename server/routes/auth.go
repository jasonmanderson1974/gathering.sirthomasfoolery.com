/* The /auth group contains all the routes to sign in and sign out */
package routes

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"os"
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
	"sirtom/server/services/auth"
	"sirtom/server/services/calendar"
	"sirtom/server/services/microsoftgraph"
	"sirtom/server/utils"
)

// otpLimiter throttles the passwordless email flow: it caps how often a code
// can be sent to a given email (anti email-bombing) and how often check-email
// can be probed from a given client (defense-in-depth against enumeration).
var otpLimiter = utils.NewRateLimiter()

func InitAuth(router *gin.RouterGroup) {
	authRouter := router.Group("/auth")

	authRouter.POST("/sign-in", signIn)
	authRouter.POST("/sign-in-mobile", signInMobile)
	authRouter.POST("/sign-out", signOut)
	authRouter.GET("/status", middleware.AuthRequired(), getStatus)

	authRouter.POST("/otp/check-email", checkEmail)
	authRouter.POST("/otp/send", sendOtp)
	authRouter.POST("/otp/verify", verifyOtp)
}

// @Summary Signs user in
// @Description Signs user in and sets the access token session variable
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body object{code=string,scope=string,calendarType=string,timezoneOffset=int} true "Object containing the Google authorization code, scope, calendar type, and the user's timezone offset"
// @Success 200
// @Router /auth/sign-in [post]
func signIn(c *gin.Context) {
	payload := struct {
		Code           string              `json:"code" binding:"required"`
		Scope          string              `json:"scope" binding:"required"`
		CalendarType   models.CalendarType `json:"calendarType" binding:"required"`
		TimezoneOffset *int                `json:"timezoneOffset" binding:"required"`
		EventsToLink   []string            `json:"eventsToLink"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	tokens, tokensErr := auth.GetTokensFromAuthCode(payload.Code, payload.Scope, utils.GetOrigin(c), payload.CalendarType)
	if tokensErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	user, err := signInHelper(c, tokens, models.WEB, payload.CalendarType, *payload.TimezoneOffset)
	if err == errs.ErrNotInvited {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotInvited})
		return
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.InvalidIdToken})
		return
	}

	// Link events to user
	for _, eventIdString := range payload.EventsToLink {
		eventId, err := primitive.ObjectIDFromHex(eventIdString)
		if err == nil {
			db.EventsCollection.UpdateOne(context.Background(), bson.M{"_id": eventId, "ownerId": nil}, bson.M{"$set": bson.M{"ownerId": user.Id}})
		}
	}

	c.JSON(http.StatusOK, user)
}

// @Summary Signs user in from mobile
// @Description Signs user in and sets the access token session variable
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body object{timezoneOffset=int,accessToken=string,scope=string,idToken=string,expiresIn=int,refreshToken=string,tokenOrigin=string,calendarType=string} true "Object containing the Google authorization code, calendar type, and the user's timezone offset"
// @Success 200
// @Router /auth/sign-in-mobile [post]
func signInMobile(c *gin.Context) {
	payload := struct {
		AccessToken    string                 `json:"accessToken" binding:"required"`
		Scope          string                 `json:"scope" binding:"required"`
		IdToken        string                 `json:"idToken" binding:"required"`
		ExpiresIn      int                    `json:"expiresIn" binding:"required"`
		RefreshToken   string                 `json:"refreshToken" binding:"required"`
		TokenOrigin    models.TokenOriginType `json:"tokenOrigin" binding:"required"`
		CalendarType   models.CalendarType    `json:"calendarType" binding:"required"`
		TimezoneOffset int                    `json:"timezoneOffset" binding:"required"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	_, err := signInHelper(
		c,
		auth.TokenResponse{
			AccessToken:  payload.AccessToken,
			IdToken:      payload.IdToken,
			ExpiresIn:    payload.ExpiresIn,
			RefreshToken: payload.RefreshToken,
			Scope:        payload.Scope,
		},
		payload.TokenOrigin,
		payload.CalendarType,
		payload.TimezoneOffset,
	)
	if err == errs.ErrNotInvited {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotInvited})
		return
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.InvalidIdToken})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

// Helper function to sign user in with the given parameters from the google oauth route
func signInHelper(c *gin.Context, token auth.TokenResponse, tokenOrigin models.TokenOriginType, calendarType models.CalendarType, timezoneOffset int) (models.User, error) {
	// Get access token expire time
	accessTokenExpireDate := utils.GetAccessTokenExpireDate(token.ExpiresIn)

	// Construct calendar auth object
	calendarAuth := models.OAuth2CalendarAuth{
		AccessToken:           token.AccessToken,
		AccessTokenExpireDate: primitive.NewDateTimeFromTime(accessTokenExpireDate),
		RefreshToken:          token.RefreshToken,
		Scope:                 token.Scope,
	}

	var email, firstName, lastName, picture string
	if calendarType == models.GoogleCalendarType {
		// Verify the ID token before trusting any of its claims. The id token
		// can be supplied directly by the client (e.g. the mobile sign-in
		// endpoint), so decoding it without verifying the signature, audience,
		// and issuer would let an attacker forge an identity and sign in as
		// any user. We accept any of our configured client IDs (web/iOS/
		// Android) as a valid audience.
		allowedAuds := []string{
			os.Getenv("CLIENT_ID"),
			os.Getenv("IOS_CLIENT_ID"),
			os.Getenv("ANDROID_CLIENT_ID"),
		}
		info, err := auth.VerifyGoogleIdToken(token.IdToken, allowedAuds)
		if err != nil {
			logger.StdErr.Printf("Failed to verify Google ID token: %v", err)
			return models.User{}, err
		}
		email = info.Email
		firstName = info.GivenName
		lastName = info.FamilyName
		picture = info.Picture
	} else if calendarType == models.OutlookCalendarType {
		// Get user info from microsoft graph
		userInfo, userInfoErr := microsoftgraph.GetUserInfo(nil, &calendarAuth)
		if userInfoErr != nil {
			return models.User{}, userInfoErr
		}
		email = userInfo.Email
		firstName = userInfo.FirstName
		lastName = userInfo.LastName
		picture = ""
	}
	email = utils.NormalizeEmail(email)

	// Invite-only gate: reject non-allowlisted emails before any account is
	// created or updated. Callers surface this as a NotInvited response.
	if !db.IsAccessAllowed(email) {
		return models.User{}, errs.ErrNotInvited
	}

	primaryAccountKey := utils.GetCalendarAccountKey(email, calendarType)

	// Create user object to create new user or update existing user
	userData := models.User{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		Picture:   picture,

		PrimaryAccountKey: &primaryAccountKey,

		TimezoneOffset: timezoneOffset,
		TokenOrigin:    tokenOrigin,
	}

	calendarAccount := models.CalendarAccount{
		CalendarType:       calendarType,
		OAuth2CalendarAuth: &calendarAuth,

		Email:   email,
		Picture: picture,
		Enabled: utils.TruePtr(), // Workaround to pass a boolean pointer
	}
	canonicalKey := utils.GetCalendarAccountKey(email, calendarType)

	var userId primitive.ObjectID
	existing, existingErr := db.GetUserByEmail(email)
	if existingErr != nil {
		return models.User{}, existingErr
	}
	// If user doesn't exist, create a new user
	if existing == nil {
		// Fetch subcalendars
		subCalendars, err := calendar.GetCalendarProvider(calendarAccount).GetCalendarList()
		if err == nil {
			calendarAccount.SubCalendars = &subCalendars
		}

		// Set calendar accounts
		userData.CalendarAccounts = map[string]models.CalendarAccount{
			canonicalKey: calendarAccount,
		}

		// Seed the role from the allowlist entry (guest/member/admin). Only on
		// creation — existing users keep whatever role they've since been given.
		userData.Role = models.NormalizeRole(db.GetAllowlistRole(email))

		// Create user
		res, err := db.UsersCollection.InsertOne(context.Background(), userData)
		if err != nil {
			logger.StdErr.Println(err)
			return models.User{}, err
		}

		userId = res.InsertedID.(primitive.ObjectID)

		// slackbot.SendTextMessage(fmt.Sprintf(":wave: %s %s (%s) has joined schej.it!", firstName, lastName, email))
	} else {
		user := existing
		userId = user.Id

		// If user has custom name, do not override first name and last name
		if user.HasCustomName != nil && *user.HasCustomName {
			userData.FirstName = ""
			userData.LastName = ""
		}

		legacyKey := utils.ActualCalendarAccountMapKey(user, email, calendarType)

		var oldSubCalendars *map[string]models.SubCalendar
		if legacyKey != "" {
			if oldAcc, ok := user.CalendarAccounts[legacyKey]; ok && oldAcc.SubCalendars != nil {
				oldSubCalendars = oldAcc.SubCalendars
			}
		} else if user.CalendarAccounts != nil {
			if existingAcc, ok := user.CalendarAccounts[canonicalKey]; ok && existingAcc.SubCalendars != nil {
				oldSubCalendars = existingAcc.SubCalendars
			}
		}

		var calAccounts map[string]models.CalendarAccount
		if user.CalendarAccounts == nil {
			calAccounts = make(map[string]models.CalendarAccount)
		} else {
			calAccounts = make(map[string]models.CalendarAccount, len(user.CalendarAccounts))
			for k, v := range user.CalendarAccounts {
				calAccounts[k] = v
			}
		}
		if legacyKey != "" && legacyKey != canonicalKey {
			delete(calAccounts, legacyKey)
		}

		if oldSubCalendars != nil {
			calendarAccount.SubCalendars = oldSubCalendars
		} else {
			subCalendars, err := calendar.GetCalendarProvider(calendarAccount).GetCalendarList()
			if err == nil {
				calendarAccount.SubCalendars = &subCalendars
			}
		}

		calAccounts[canonicalKey] = calendarAccount
		userData.CalendarAccounts = calAccounts
		userData.Email = email

		// Update user if exists
		_, err := db.UsersCollection.UpdateByID(
			context.Background(),
			userId,
			bson.M{"$set": userData},
		)
		if err != nil {
			logger.StdErr.Println(err)
			return models.User{}, err
		}
	}

	// Set session variables
	session := sessions.Default(c)
	session.Set("userId", userId.Hex())
	session.Save()

	userData.Id = userId
	return userData, nil
}

// @Summary Signs user out
// @Description Signs user out and deletes the session
// @Tags auth
// @Accept json
// @Produce json
// @Success 200
// @Router /auth/sign-out [post]
func signOut(c *gin.Context) {
	// Delete session
	session := sessions.Default(c)
	session.Delete("userId")
	session.Save()

	c.JSON(http.StatusOK, gin.H{})
}

// @Summary Gets whether the user is signed in or not
// @Description Returns a 401 error if user is not signed in, 200 if they are
// @Tags auth
// @Success 200
// @Failure 401 {object} responses.Error "Error object"
// @Router /auth/status [get]
func getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}

func generateOtpCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		logger.StdErr.Panicln(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// buildOtpEmailBody returns a Fellowship-themed HTML email containing the
// sign-in code. Inline styles only (email clients strip <style>/<head>), and a
// dark leather/brass palette that mirrors the sign-in screen.
func buildOtpEmailBody(code string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background-color:#1c1410;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#1c1410;">
    <tr>
      <td align="center" style="padding:40px 16px;">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:440px;background-color:#241a13;border:1px solid #8a7333;border-radius:14px;">
          <tr>
            <td style="padding:32px 36px;font-family:Georgia,'Times New Roman',serif;color:#ede4d3;">
              <div style="font-size:13px;font-weight:bold;letter-spacing:0.16em;color:#c9a44c;text-transform:uppercase;">The Fellowship</div>
              <div style="height:1px;background-color:#8a7333;margin:18px 0 24px;"></div>
              <div style="font-size:22px;color:#ede4d3;margin-bottom:10px;">Your sign-in code</div>
              <div style="font-size:14px;color:#b8ad97;line-height:1.5;margin-bottom:24px;">
                Present this code to complete your entry to the Gathering. It expires in ten minutes.
              </div>
              <div style="text-align:center;background-color:#2e2117;border:1px solid #8a7333;border-radius:10px;padding:20px;margin-bottom:24px;">
                <span style="font-size:34px;font-weight:bold;letter-spacing:0.32em;color:#e3c578;font-family:'Courier New',monospace;">%s</span>
              </div>
              <div style="font-size:12px;color:#b8ad97;line-height:1.5;">
                If you did not request this code, you may disregard this message. If it doesn't arrive promptly, check your spam folder.
              </div>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, code)
}

// @Summary Checks whether a user with the given email exists
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body object{email=string} true "Email to check"
// @Success 200
// @Router /auth/otp/check-email [post]
func checkEmail(c *gin.Context) {
	payload := struct {
		Email string `json:"email" binding:"required"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	email := utils.NormalizeEmail(payload.Email)

	// Throttle probing from a single client (defense-in-depth vs enumeration).
	if !otpLimiter.Allow("check-email:"+c.ClientIP(), 60, time.Minute) {
		c.JSON(http.StatusTooManyRequests, responses.Error{Error: errs.OtpRateLimited})
		return
	}

	// Invite-only gate: only allowlisted emails may sign in / register.
	if !db.IsAccessAllowed(email) {
		c.JSON(http.StatusOK, gin.H{"invited": false})
		return
	}

	existingUser, existingErr := db.GetUserByEmail(email)
	if existingErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	isNewUser := existingUser == nil
	c.JSON(http.StatusOK, gin.H{"invited": true, "isNewUser": isNewUser})
}

// @Summary Sends an OTP code to the given email
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body object{email=string} true "Email to send OTP to"
// @Success 200
// @Router /auth/otp/send [post]
func sendOtp(c *gin.Context) {
	payload := struct {
		Email string `json:"email" binding:"required"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	email := utils.NormalizeEmail(payload.Email)

	// Invite-only gate: never send a code to a non-allowlisted email
	if !db.IsAccessAllowed(email) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotInvited})
		return
	}

	// Rate-limit code sends per email to prevent inbox flooding / abuse. The
	// client also enforces a 30s resend cooldown; this is the server-side cap.
	if !otpLimiter.Allow("otp-send:"+email, 5, 15*time.Minute) {
		c.JSON(http.StatusTooManyRequests, responses.Error{Error: errs.OtpRateLimited})
		return
	}

	// Delete any existing OTP codes for this email
	db.OtpCodesCollection.DeleteMany(context.Background(), bson.M{"email": email})

	code := generateOtpCode()
	otpDoc := models.OtpCode{
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Attempts:  0,
	}

	_, err := db.OtpCodesCollection.InsertOne(context.Background(), otpDoc)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	// Email the code via Gmail SMTP (utils.SendEmail — GMAIL_APP_PASSWORD +
	// SCHEJ_EMAIL_ADDRESS). Invite-only instance: low volume, well within
	// Gmail's daily cap.
	if sendErr := utils.SendEmail(
		email,
		fmt.Sprintf("%s is your Fellowship sign-in code", code),
		buildOtpEmailBody(code),
		"text/html",
	); sendErr != nil {
		// Delivery failed (e.g. missing/invalid SMTP creds). Remove the code we
		// just stored — it's useless if it never reached the user — and surface
		// the failure so the UI can tell them instead of pretending success.
		db.OtpCodesCollection.DeleteMany(context.Background(), bson.M{"email": email})
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.OtpSendFailed})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

// @Summary Verifies an OTP code and signs the user in
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body object{email=string,code=string,timezoneOffset=int} true "Email, OTP code, and timezone offset"
// @Success 200
// @Router /auth/otp/verify [post]
func verifyOtp(c *gin.Context) {
	payload := struct {
		Email          string `json:"email" binding:"required"`
		Code           string `json:"code" binding:"required"`
		TimezoneOffset *int   `json:"timezoneOffset" binding:"required"`
		FirstName      string `json:"firstName"`
		LastName       string `json:"lastName"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	email := utils.NormalizeEmail(payload.Email)

	// Re-check the allowlist at verify time: the email may have been struck from
	// the roll between /otp/send and /otp/verify while a valid code still exists.
	if !db.IsAccessAllowed(email) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotInvited})
		return
	}

	// Find the OTP document
	var otpDoc models.OtpCode
	err := db.OtpCodesCollection.FindOne(context.Background(), bson.M{
		"email":     email,
		"expiresAt": bson.M{"$gt": time.Now()},
	}).Decode(&otpDoc)

	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.OtpExpired})
		return
	} else if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	// Rate-limit: max 5 attempts per code
	if otpDoc.Attempts >= 5 {
		db.OtpCodesCollection.DeleteOne(context.Background(), bson.M{"_id": otpDoc.Id})
		c.JSON(http.StatusTooManyRequests, responses.Error{Error: errs.OtpTooManyAttempts})
		return
	}

	// Increment attempts
	db.OtpCodesCollection.UpdateByID(context.Background(), otpDoc.Id, bson.M{
		"$inc": bson.M{"attempts": 1},
	})

	if otpDoc.Code != payload.Code {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.OtpInvalidCode})
		return
	}

	// OTP verified — delete it
	db.OtpCodesCollection.DeleteOne(context.Background(), bson.M{"_id": otpDoc.Id})

	// Find or create user
	var userId primitive.ObjectID
	existing, existingErr := db.GetUserByEmail(email)
	if existingErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	if existing == nil {
		firstName := strings.TrimSpace(payload.FirstName)
		lastName := strings.TrimSpace(payload.LastName)

		userData := models.User{
			Email:          email,
			FirstName:      firstName,
			LastName:       lastName,
			TimezoneOffset: *payload.TimezoneOffset,
			TokenOrigin:    models.WEB,
			// Seed the role from the allowlist entry (guest/member/admin).
			Role: models.NormalizeRole(db.GetAllowlistRole(email)),
		}

		res, err := db.UsersCollection.InsertOne(context.Background(), userData)
		if err != nil {
			logger.StdErr.Println(err)
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
		userId = res.InsertedID.(primitive.ObjectID)
	} else {
		userId = existing.Id
	}

	// Set session — same mechanism as OAuth sign-in
	session := sessions.Default(c)
	session.Set("userId", userId.Hex())
	session.Save()

	user, userErr := db.GetUserById(userId.Hex())
	if userErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	c.JSON(http.StatusOK, user)
}
