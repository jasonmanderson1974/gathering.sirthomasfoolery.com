package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/brianvoe/sjwt"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// Returns whether running on production server
func IsRelease() bool {
	mode := os.Getenv("GIN_MODE")
	return mode == "release"
}

func PrintJson(s interface{}) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		logger.StdErr.Panicln(err)
	}

	fmt.Println(string(data))
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
func GetDateString(date time.Time) string {
	s, _ := date.UTC().MarshalText()
	return string(s)[:10]
}

// Returns a time object with the given date and a time string in the form of "00:00:00"
func GetDateAtTime(date time.Time, timeString string) time.Time {
	utcDateString := GetDateString(date)
	newDate, err := time.Parse(time.RFC3339, fmt.Sprintf("%sT%sZ", utcDateString, timeString))
	if err != nil {
		logger.StdErr.Panicln(err)
	}
	return newDate
}

// Escapes regex for a string
func EscapeRegExp(str string) string {
	check := regexp.MustCompile(`([.*+?^${}()|[\]\\])`)
	return check.ReplaceAllString(str, "\\${1}")
}

// Returns the correct client id given the token origin
func GetClientIdFromTokenOrigin(tokenOrigin models.TokenOriginType) string {
	switch tokenOrigin {
	case models.ANDROID:
		return os.Getenv("ANDROID_CLIENT_ID")
	case models.IOS:
		return os.Getenv("IOS_CLIENT_ID")
	default:
		return os.Getenv("CLIENT_ID")
	}
}

// Prints the http response as a string
func PrintHttpResponse(resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	logger.StdOut.Println(string(body))
	resp.Body = io.NopCloser(bytes.NewBuffer(body))
}

// Returns the correct base url, based on whether we're on dev or prod
func GetBaseUrl() string {
	var baseUrl string
	if IsRelease() {
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

func GetPrimaryAccountKey(user *models.User) string {
	// Before primary account key was added, primary account was always the user's google calendar
	if user.PrimaryAccountKey == nil {
		return ActualCalendarAccountMapKey(user, user.Email, models.GoogleCalendarType)
	}

	return *user.PrimaryAccountKey
}

func Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// decodeBase64 is the inverse of Encode. It returns an error rather than
// panicking: everything it decodes arrives from the database.
//
// (This replaced an exported `Decode` that panicked on malformed input — one of
// the bare panics TODO E10 lists. Decrypt was its only caller, so it went with
// the rewrite rather than being left behind as dead code that panics.)
func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// --- Encryption at rest (TODO B6) --------------------------------------------
//
// v2 is AES-256-GCM. v1 was AES-CFB, which is *unauthenticated*: anyone able to
// write to the database could flip plaintext bits undetected, and a wrong key
// yields plausible-looking garbage rather than an error. Go deprecated the mode
// in 1.24.
//
// Both versions use ENCRYPTION_KEY's bytes directly as the AES key — unchanged
// from v1, deliberately. The key is already a valid 32-byte AES-256 key, so
// introducing a derivation step would buy nothing and leave two key concepts
// coexisting mid-migration.
//
// Stored ciphertext therefore carries its version, so the two formats can live
// side by side while values migrate. The marker is the textual prefix "v2:"
// rather than a leading version byte, because a v1 blob begins with a 16-byte
// random IV whose first byte can be *any* value — sniffing it is unreliable in
// principle, not merely awkward. ':' is not in the base64 alphabet, so no v1
// value can be mistaken for a tagged one.
const encV2Prefix = "v2:"

func encryptionKey() ([]byte, error) {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		return nil, errors.New("ENCRYPTION_KEY is not set")
	}
	return []byte(key), nil
}

// Encrypt encrypts text with AES-256-GCM, returning a version-tagged,
// base64-encoded string.
func Encrypt(text string) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	// Seal appends the ciphertext and its authentication tag to the nonce, so
	// the nonce travels with the value it was used for.
	sealed := gcm.Seal(nonce, nonce, []byte(text), nil)
	return encV2Prefix + Encode(sealed), nil
}

// Decrypt accepts either version: v2 (AES-GCM, tagged) or v1 (AES-CFB,
// untagged) for values written before the migration.
//
// It never panics. The ciphertext comes from the database, so a truncated or
// corrupt value has to surface as an error rather than take down the request.
func Decrypt(text string) (string, error) {
	if strings.HasPrefix(text, encV2Prefix) {
		return decryptV2GCM(strings.TrimPrefix(text, encV2Prefix))
	}
	return decryptV1CFB(text)
}

func decryptV2GCM(encoded string) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return "", err
	}
	raw, err := decodeBase64(encoded)
	if err != nil {
		return "", fmt.Errorf("ciphertext is not valid base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, sealed := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	// Open fails on a tampered value or the wrong key — the property CFB lacked.
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

// IsLegacyCiphertext reports whether a stored value is still v1 (AES-CFB), so a
// caller can rewrite it in the current format. Empty is not legacy — there is
// nothing to migrate.
func IsLegacyCiphertext(text string) bool {
	return text != "" && !strings.HasPrefix(text, encV2Prefix)
}

// decryptV1CFB reads ciphertext written before B6. It stays only until no v1
// values remain; see TODO B6 for the retirement condition.
func decryptV1CFB(text string) (string, error) {
	key, err := encryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	cipherText, err := decodeBase64(text)
	if err != nil {
		return "", fmt.Errorf("ciphertext is not valid base64: %w", err)
	}
	if len(cipherText) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv) //nolint:staticcheck // SA1019: v1 read path, see B6
	plainText := make([]byte, len(cipherText))
	cfb.XORKeyStream(plainText, cipherText)
	return string(plainText), nil
}

// ConvertEventToOldFormat converts an event's responses from ResponsesList to ResponsesMap format
// for backward compatibility with older code
func ConvertEventToOldFormat(event *models.Event, eventResponses []models.EventResponse) {
	responsesMap := make(map[string]*models.Response)
	for _, resp := range eventResponses {
		responsesMap[resp.UserId] = resp.Response
	}
	event.ResponsesMap = responsesMap
}
