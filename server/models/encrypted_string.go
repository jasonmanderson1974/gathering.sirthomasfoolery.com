package models

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"sirtom/server/encryption"
)

// EncryptedString is a string that is encrypted at rest (TODO B7): plaintext in
// memory, ciphertext in Mongo. The conversion happens in the BSON codec rather
// than at each call site, because the call sites are the problem this fixes —
// OAuth tokens are written from four places and read from six, and any one of
// them forgetting is a long-lived credential sitting in the clear. Making it a
// property of the *type* means a new write path cannot get it wrong.
//
// It is a string, so `%s`, len() and comparison all work unchanged; assigning a
// plain string in requires an explicit conversion, which is the one place a
// reader is asked to notice that this value is a secret.
type EncryptedString string

// MarshalBSONValue encrypts on the way to Mongo.
//
// An empty value is written as an empty string rather than as ciphertext: it
// carries no secret, and encrypting it would only make an absent token
// indistinguishable from a present one. In practice `omitempty` on the struct
// fields means this branch is not reached from the models here.
func (s EncryptedString) MarshalBSONValue() (bsontype.Type, []byte, error) {
	if s == "" {
		return bson.MarshalValue("")
	}
	ciphertext, err := encryption.Encrypt(string(s))
	if err != nil {
		// Returning the error aborts the whole write. That is deliberate: the
		// alternative is storing the credential in the clear, which is the bug.
		return 0, nil, fmt.Errorf("encrypting value for storage: %w", err)
	}
	return bson.MarshalValue(ciphertext)
}

// UnmarshalBSONValue decrypts on the way out of Mongo.
//
// Values written before B7 are stored in the clear and are passed through
// unchanged — a tagged value is unambiguous (':' is outside the base64
// alphabet), so there is no guessing involved. The startup sweep in
// db.EncryptPlaintextOAuthTokens migrates them; step 4 removes this branch once
// prod is confirmed to hold none, and the passthrough is the only reason these
// steps are separate deploys.
//
// A value that is tagged but *fails* to decrypt is an error, not an empty
// string. It has to be, because several handlers read the user document,
// change one field and write the whole thing back: degrading a failed decrypt
// to "" would let a wrong ENCRYPTION_KEY quietly destroy every refresh token in
// the database on the next calendar fetch. Failing the decode instead means
// nothing is written back at all.
func (s *EncryptedString) UnmarshalBSONValue(t bsontype.Type, data []byte) error {
	// The interface fixes the parameter as a bsontype.Type; the constants
	// naming its values now live on bson (the bsontype ones are deprecated).
	switch t {
	case bson.TypeNull, bson.TypeUndefined:
		*s = ""
		return nil
	case bson.TypeString:
	default:
		return fmt.Errorf("cannot decode a %s into an encrypted string", t)
	}

	raw, ok := bson.RawValue{Type: t, Value: data}.StringValueOK()
	if !ok {
		return fmt.Errorf("malformed string value for an encrypted field")
	}
	if !encryption.IsCiphertext(raw) {
		// Legacy plaintext (pre-B7).
		*s = EncryptedString(raw)
		return nil
	}

	plain, err := encryption.Decrypt(raw)
	if err != nil {
		return fmt.Errorf("decrypting stored value: %w", err)
	}
	*s = EncryptedString(plain)
	return nil
}
