package models

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// The prod key is 32 bytes (AES-256); match that here.
const testEncryptionKey = "0123456789abcdef0123456789abcdef"

func withKey(t *testing.T, key string) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", key)
}

// storedTokens returns what Mongo would actually hold for a user document —
// the raw strings, before any decryption.
func storedTokens(t *testing.T, raw bson.Raw, mapKey string) (access, refresh string) {
	t.Helper()
	auth := raw.Lookup("calendarAccounts", mapKey, "oAuth2CalendarAuth")
	if auth.Type == 0 {
		t.Fatalf("no oAuth2CalendarAuth in stored document: %v", raw)
	}
	if v, err := auth.Document().LookupErr("accessToken"); err == nil {
		access = v.StringValue()
	}
	if v, err := auth.Document().LookupErr("refreshToken"); err == nil {
		refresh = v.StringValue()
	}
	return access, refresh
}

func userWithTokens(access, refresh string) User {
	return User{
		CalendarAccounts: map[string]CalendarAccount{
			// A dotted key, as real ones are: the map key is `email_TYPE`.
			"a.b@example.com_google": {
				CalendarType: GoogleCalendarType,
				Email:        "a.b@example.com",
				OAuth2CalendarAuth: &OAuth2CalendarAuth{
					AccessToken:  EncryptedString(access),
					RefreshToken: EncryptedString(refresh),
					Scope:        "https://www.googleapis.com/auth/calendar.readonly",
				},
			},
		},
	}
}

// The whole point of B7: what lands in Mongo must not be the token. This goes
// through a full User document, in a map, because that is the shape every real
// write takes — and a map value is not addressable, which is exactly where a
// codec is easiest to get wrong.
func TestEncryptedString_TokensAreCiphertextInMongo(t *testing.T) {
	withKey(t, testEncryptionKey)

	raw, err := bson.Marshal(userWithTokens("ya29.a0-access-token", "1//0e-refresh-token"))
	if err != nil {
		t.Fatal(err)
	}

	access, refresh := storedTokens(t, raw, "a.b@example.com_google")
	for name, stored := range map[string]string{"accessToken": access, "refreshToken": refresh} {
		if !strings.HasPrefix(stored, "v2:") {
			t.Errorf("%s is not tagged ciphertext: %q", name, stored)
		}
	}
	if strings.Contains(string(raw), "ya29.a0-access-token") || strings.Contains(string(raw), "1//0e-refresh-token") {
		t.Error("a token appears in the marshalled document in the clear")
	}
	// The non-secret neighbours must still be readable — encrypting the whole
	// struct would have broken queries and cost more than it bought.
	if got := bson.Raw(raw).Lookup("calendarAccounts", "a.b@example.com_google", "oAuth2CalendarAuth").Document().Lookup("scope").StringValue(); got == "" {
		t.Error("scope should be stored in the clear")
	}
}

func TestEncryptedString_RoundTripsThroughBSON(t *testing.T) {
	withKey(t, testEncryptionKey)

	raw, err := bson.Marshal(userWithTokens("access-abc", "refresh-xyz"))
	if err != nil {
		t.Fatal(err)
	}

	var got User
	if err := bson.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	auth := got.CalendarAccounts["a.b@example.com_google"].OAuth2CalendarAuth
	if auth.AccessToken != "access-abc" {
		t.Errorf("access token: got %q", auth.AccessToken)
	}
	if auth.RefreshToken != "refresh-xyz" {
		t.Errorf("refresh token: got %q", auth.RefreshToken)
	}
}

// Values written before B7 are stored in the clear. They must keep working
// until the sweep has moved them; step 4 removes this.
func TestEncryptedString_ReadsLegacyPlaintext(t *testing.T) {
	withKey(t, testEncryptionKey)

	var s EncryptedString
	typ, data, err := bson.MarshalValue("plaintext-refresh-token")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UnmarshalBSONValue(typ, data); err != nil {
		t.Fatalf("legacy plaintext should be readable: %v", err)
	}
	if s != "plaintext-refresh-token" {
		t.Errorf("got %q", s)
	}
}

// A tampered or wrong-key value must fail the decode rather than degrade to "".
// Several handlers read the user document, change one field and write the whole
// thing back; a silent "" would let a bad key destroy every stored refresh
// token on the next calendar fetch. Failing here means nothing is written back.
func TestEncryptedString_UndecryptableValueFailsTheDecode(t *testing.T) {
	withKey(t, testEncryptionKey)
	raw, err := bson.Marshal(userWithTokens("access-abc", "refresh-xyz"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("wrong key", func(t *testing.T) {
		withKey(t, "ffffffffffffffffffffffffffffffff")
		var got User
		if err := bson.Unmarshal(raw, &got); err == nil {
			t.Fatalf("decoding with the wrong key succeeded: %+v", got.CalendarAccounts)
		}
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		withKey(t, testEncryptionKey)
		var s EncryptedString
		enc, err := bson.Marshal(bson.M{"v": EncryptedString("secret")})
		if err != nil {
			t.Fatal(err)
		}
		stored := []byte(bson.Raw(enc).Lookup("v").StringValue())
		stored[len(stored)-1] ^= 0x01 // flip a bit of the base64 body

		typ, data, err := bson.MarshalValue(string(stored))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.UnmarshalBSONValue(typ, data); err == nil {
			t.Errorf("tampered ciphertext decoded to %q", s)
		}
	})
}

// omitempty must still fire, so an absent token stays absent rather than
// becoming ciphertext that decrypts to "".
func TestEncryptedString_EmptyTokenIsOmitted(t *testing.T) {
	withKey(t, testEncryptionKey)

	raw, err := bson.Marshal(userWithTokens("", ""))
	if err != nil {
		t.Fatal(err)
	}
	access, refresh := storedTokens(t, raw, "a.b@example.com_google")
	if access != "" || refresh != "" {
		t.Errorf("empty tokens were stored: %q / %q", access, refresh)
	}
}

// If the key is unusable the write must fail. Storing the token in the clear
// instead is the bug this whole item exists to fix, so there is no fallback.
func TestEncryptedString_UnusableKeyFailsTheWrite(t *testing.T) {
	withKey(t, "")

	raw, err := bson.Marshal(userWithTokens("access-abc", "refresh-xyz"))
	if err == nil {
		t.Fatalf("marshalling with no ENCRYPTION_KEY succeeded: %v", bson.Raw(raw))
	}
	if strings.Contains(string(raw), "access-abc") {
		t.Error("the token was serialised in the clear on the failure path")
	}
}
