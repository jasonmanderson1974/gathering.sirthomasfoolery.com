package utils

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brianvoe/sjwt"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// Returns whether running on production server
func isRelease() bool {
	mode := os.Getenv("GIN_MODE")
	return mode == "release"
}

func ParseJWT(jwt string) sjwt.Claims {
	claims, err := sjwt.Parse(jwt)
	if err != nil {
		logger.StdErr.Panicln(err)
	}

	return claims
}

func StringToObjectID(s string) primitive.ObjectID {
	objectID, err := primitive.ObjectIDFromHex(s)
	if err != nil {
		logger.StdErr.Panicln(err)
	}

	return objectID
}

// Returns the currently signed in user
func GetAuthUser(c *gin.Context) *models.User {
	userInterface, _ := c.Get("authUser")
	user := userInterface.(*models.User)
	return user
}

// Gets the access token expire date from an "expiresIn" int representing the number of seconds
// after which the access token will expire
func GetAccessTokenExpireDate(expiresIn int) time.Time {
	expireDuration, err := time.ParseDuration(fmt.Sprintf("%ds", expiresIn))
	if err != nil {
		logger.StdErr.Panicln(err)
	}
	return time.Now().Add(expireDuration)
}

// Returns the ISO date string for the given date
func getDateString(date time.Time) string {
	s, _ := date.UTC().MarshalText()
	return string(s)[:10]
}

// Returns a time object with the given date and a time string in the form of "00:00:00"
func GetDateAtTime(date time.Time, timeString string) time.Time {
	utcDateString := getDateString(date)
	newDate, err := time.Parse(time.RFC3339, fmt.Sprintf("%sT%sZ", utcDateString, timeString))
	if err != nil {
		logger.StdErr.Panicln(err)
	}
	return newDate
}

// Returns the correct base url, based on whether we're on dev or prod
func GetBaseUrl() string {
	var baseUrl string
	if isRelease() {
		baseUrl = "https://gathering.sirthomasfoolery.com"
	} else {
		baseUrl = "http://localhost:8080"
	}
	return baseUrl
}

// Returns the value of the first non nil pointer in `args`.
// Otherwise, just return the zero value
func Coalesce[T any](args ...*T) T {
	for _, val := range args {
		if val != nil {
			return *val
		}
	}

	var val T
	return val
}

// Return a pointer to true
func TruePtr() *bool {
	b := true
	return &b
}

// Return a pointer to false
func FalsePtr() *bool {
	b := false
	return &b
}

// NormalizeEmail returns the email in a canonical form for lookups and storage (trim + ASCII lower).
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// GetCalendarAccountKey builds the map key for calendarAccounts. Email-like identifiers are lowercased;
// ICS uses the feed label as the first segment and is only trimmed, not lowercased.
func GetCalendarAccountKey(ident string, calendarType models.CalendarType) string {
	keyPart := strings.TrimSpace(ident)
	if calendarType != models.ICSCalendarType {
		keyPart = NormalizeEmail(keyPart)
	}
	return fmt.Sprintf("%s_%s", keyPart, calendarType)
}

// ActualCalendarAccountMapKey returns the key already present in user.CalendarAccounts for this
// account, or "" if none. Prefer this over recomputing from email when reading legacy documents
// whose map keys used mixed-case emails.
func ActualCalendarAccountMapKey(user *models.User, ident string, calendarType models.CalendarType) string {
	if user == nil || user.CalendarAccounts == nil {
		return ""
	}
	canonical := GetCalendarAccountKey(ident, calendarType)
	if _, ok := user.CalendarAccounts[canonical]; ok {
		return canonical
	}
	for k, acc := range user.CalendarAccounts {
		if acc.CalendarType != calendarType {
			continue
		}
		if calendarType == models.ICSCalendarType {
			if strings.TrimSpace(acc.Email) == strings.TrimSpace(ident) {
				return k
			}
			continue
		}
		if NormalizeEmail(acc.Email) == NormalizeEmail(ident) {
			return k
		}
	}
	return ""
}

// ConvertEventToOldFormat converts an event's responses from ResponsesList to ResponsesMap format
// for backward compatibility with older code.
//
// A row with no `response` field is skipped, for the same reason getResponsesMap
// skips it (F12): it would otherwise serialize as a null the clients dereference.
func ConvertEventToOldFormat(event *models.Event, eventResponses []models.EventResponse) {
	responsesMap := make(map[string]*models.Response)
	for _, resp := range eventResponses {
		if resp.Response == nil {
			continue
		}
		responsesMap[resp.UserId] = resp.Response
	}
	event.ResponsesMap = responsesMap
}
