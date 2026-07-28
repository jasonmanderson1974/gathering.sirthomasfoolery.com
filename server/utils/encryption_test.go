package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"strings"
	"testing"
)

// B6: v2 is AES-256-GCM, v1 was AES-CFB. Both use ENCRYPTION_KEY's bytes as the
// AES key, so a v2 blob is distinguishable from a v1 blob only by its prefix.

// The prod key is 32 bytes (AES-256); match that here.
const testKey = "0123456789abcdef0123456789abcdef"

func withKey(t *testing.T, key string) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", key)
}

// encryptV1CFB reproduces exactly what Encrypt did before B6, so the dual-read
// path is tested against genuine legacy ciphertext rather than an assumption
// about its shape.
func encryptV1CFB(t *testing.T, text string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(testKey))
	if err != nil {
		t.Fatalf("v1 cipher: %v", err)
	}
	cipherText := make([]byte, aes.BlockSize+len(text))
	iv := cipherText[:aes.BlockSize]
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("v1 iv: %v", err)
	}
	cfb := cipher.NewCFBEncrypter(block, iv) //nolint:staticcheck // SA1019: fixture reproducing v1
	cfb.XORKeyStream(cipherText[aes.BlockSize:], []byte(text))
	return Encode(cipherText)
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	withKey(t, testKey)
	for _, plain := range []string{
		"",
		"hunter2",
		"abcd-efgh-ijkl-mnop", // the shape of an Apple app-specific password
		"a much longer secret with unicode: café 🥃 and a newline\n",
		strings.Repeat("x", 4096),
	} {
		enc, err := Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		got, err := Decrypt(enc)
		if err != nil {
			t.Fatalf("Decrypt of %q: %v", plain, err)
		}
		if got != plain {
			t.Errorf("round trip: got %q, want %q", got, plain)
		}
	}
}

func TestEncrypt_TagsOutputAsV2(t *testing.T) {
	withKey(t, testKey)
	enc, err := Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "v2:") {
		t.Errorf("output not tagged: %q", enc)
	}
}

// A v1 blob can never be mistaken for a tagged one: ':' is not in the base64
// alphabet, so it cannot appear at index 2 of an untagged value.
func TestV1CiphertextIsNeverMistakenForV2(t *testing.T) {
	withKey(t, testKey)
	for i := 0; i < 200; i++ {
		v1 := encryptV1CFB(t, "secret")
		if strings.HasPrefix(v1, "v2:") {
			t.Fatalf("v1 ciphertext looks tagged: %q", v1)
		}
	}
}

// The migration's whole premise: values written before B6 must still decrypt.
func TestDecrypt_ReadsLegacyV1Ciphertext(t *testing.T) {
	withKey(t, testKey)
	const secret = "legacy-apple-app-password"
	got, err := Decrypt(encryptV1CFB(t, secret))
	if err != nil {
		t.Fatalf("legacy decrypt: %v", err)
	}
	if got != secret {
		t.Errorf("got %q, want %q", got, secret)
	}
}

// The point of moving to an AEAD. Flipping a byte of v2 ciphertext must be
// detected; the same edit to v1 silently yields corrupted plaintext.
func TestDecrypt_V2DetectsTampering(t *testing.T) {
	withKey(t, testKey)
	enc, err := Encrypt("transfer £10 to alice")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := decodeBase64(strings.TrimPrefix(enc, "v2:"))
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0x01 // flip a bit in the tag
	if _, err := Decrypt("v2:" + Encode(raw)); err == nil {
		t.Error("tampered v2 ciphertext decrypted without error")
	}

	raw2, err := decodeBase64(strings.TrimPrefix(enc, "v2:"))
	if err != nil {
		t.Fatal(err)
	}
	raw2[aes.BlockSize] ^= 0x01 // flip a bit in the body
	if _, err := Decrypt("v2:" + Encode(raw2)); err == nil {
		t.Error("tampered v2 body decrypted without error")
	}
}

// Contrast, pinned so the reason for the migration stays visible: v1 accepts a
// tampered value and returns altered plaintext, no error.
func TestDecrypt_V1SilentlyAcceptsTampering(t *testing.T) {
	withKey(t, testKey)
	const secret = "transfer to alice"
	v1 := encryptV1CFB(t, secret)

	raw, err := decodeBase64(v1)
	if err != nil {
		t.Fatal(err)
	}
	raw[aes.BlockSize] ^= 0x01

	got, err := Decrypt(Encode(raw))
	if err != nil {
		t.Fatalf("v1 unexpectedly errored: %v", err)
	}
	if got == secret {
		t.Fatal("expected the flipped bit to alter the plaintext")
	}
	// No error, different plaintext — exactly what an AEAD prevents.
}

// A wrong key must fail loudly on v2. (On v1 it cannot: CFB returns garbage.)
func TestDecrypt_V2WrongKeyErrors(t *testing.T) {
	withKey(t, testKey)
	enc, err := Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}

	withKey(t, "ffffffffffffffffffffffffffffffff")
	if _, err := Decrypt(enc); err == nil {
		t.Error("decrypting with the wrong key succeeded")
	}
}

// Ciphertext arrives from the database, so malformed input must error rather
// than panic — utils.Decode would have panicked here.
func TestDecrypt_MalformedInputErrorsWithoutPanic(t *testing.T) {
	withKey(t, testKey)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Decrypt panicked: %v", r)
		}
	}()

	for _, bad := range []string{
		"v2:!!!not base64!!!",
		"!!!not base64!!!",
		"v2:",
		"",
		"v2:" + Encode([]byte("short")), // shorter than a GCM nonce
		Encode([]byte("tiny")),          // shorter than a CFB IV
	} {
		if _, err := Decrypt(bad); err == nil {
			t.Errorf("Decrypt(%q) returned no error", bad)
		}
	}
}

func TestEncrypt_MissingKeyErrors(t *testing.T) {
	withKey(t, "")
	if _, err := Encrypt("secret"); err == nil {
		t.Error("Encrypt with no ENCRYPTION_KEY returned no error")
	}
	if _, err := Decrypt("v2:whatever"); err == nil {
		t.Error("Decrypt with no ENCRYPTION_KEY returned no error")
	}
}

// A wrong-length key is rejected by AES rather than silently mis-encrypting.
// Commit 2 turns this into a startup failure instead of a per-request one.
func TestEncrypt_InvalidKeyLengthErrors(t *testing.T) {
	// 44 chars — what `openssl rand -base64 32` produces, which DEPLOYMENT.md
	// recommended until B6.
	withKey(t, strings.Repeat("a", 44))
	if _, err := Encrypt("secret"); err == nil {
		t.Error("Encrypt with a 44-byte key returned no error")
	}
}

// Every v2 encryption must use a fresh nonce; a repeat under the same key is a
// catastrophic GCM failure, so pin that the output never repeats.
func TestEncrypt_NonceIsFreshEachCall(t *testing.T) {
	withKey(t, testKey)
	seen := make(map[string]bool)
	for i := 0; i < 500; i++ {
		enc, err := Encrypt("same plaintext every time")
		if err != nil {
			t.Fatal(err)
		}
		if seen[enc] {
			t.Fatal("Encrypt produced identical ciphertext twice — nonce reuse")
		}
		seen[enc] = true
	}
}
