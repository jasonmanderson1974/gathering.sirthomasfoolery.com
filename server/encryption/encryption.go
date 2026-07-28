// Package encryption holds the at-rest encryption primitives (TODO B6/B7).
//
// It lives in its own package rather than in utils because `models` needs it —
// models.EncryptedString encrypts and decrypts as part of BSON marshalling —
// and utils imports models. This package deliberately imports nothing internal,
// so it can never take part in a cycle.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

// v2 is AES-256-GCM. v1 was AES-CFB, which is *unauthenticated*: anyone able to
// write to the database could flip plaintext bits undetected, and a wrong key
// yields plausible-looking garbage rather than an error. Go deprecated the mode
// in 1.24, and B6 step 4 removed the last read path for it.
//
// The key comes from ENCRYPTION_KEY's bytes directly. The key is already a valid
// AES key (its length is checked at startup, see main.validateEncryptionKey), so
// introducing a derivation step would buy nothing.
//
// Stored ciphertext carries its version, so formats can live side by side while
// values migrate. The marker is the textual prefix "v2:" rather than a leading
// version byte, because a v1 blob began with a 16-byte random IV whose first
// byte could be *any* value — sniffing it is unreliable in principle, not merely
// awkward. ':' is not in the base64 alphabet, so no untagged value can be
// mistaken for a tagged one.
const v2Prefix = "v2:"

// IsCiphertext reports whether a stored value carries the current version
// marker. A false answer means the value is stored in the clear.
func IsCiphertext(s string) bool {
	return strings.HasPrefix(s, v2Prefix)
}

func key() ([]byte, error) {
	k := os.Getenv("ENCRYPTION_KEY")
	if k == "" {
		return nil, errors.New("ENCRYPTION_KEY is not set")
	}
	return []byte(k), nil
}

func encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// decode is the inverse of encode. It returns an error rather than panicking:
// everything it decodes arrives from the database.
func decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// Encrypt encrypts text with AES-256-GCM, returning a version-tagged,
// base64-encoded string.
func Encrypt(text string) (string, error) {
	k, err := key()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k)
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
	return v2Prefix + encode(sealed), nil
}

// Decrypt reads a v2 (AES-GCM) value.
//
// The v1 AES-CFB read path is gone (B6 step 4). An untagged value fails loudly
// rather than being decrypted by a mode that cannot tell tampered ciphertext
// from genuine.
//
// It never panics. The ciphertext comes from the database, so a truncated or
// corrupt value has to surface as an error rather than take down the request.
func Decrypt(text string) (string, error) {
	if !IsCiphertext(text) {
		return "", errors.New("ciphertext is not in the current format (expected a v2 prefix)")
	}
	return decryptV2GCM(strings.TrimPrefix(text, v2Prefix))
}

func decryptV2GCM(encoded string) (string, error) {
	k, err := key()
	if err != nil {
		return "", err
	}
	raw, err := decode(encoded)
	if err != nil {
		return "", fmt.Errorf("ciphertext is not valid base64: %w", err)
	}
	block, err := aes.NewCipher(k)
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
