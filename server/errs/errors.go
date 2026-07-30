package errs

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Code is the machine-readable error identifier returned to clients as the
// `error` field of responses.Error. It is a distinct type rather than a bare
// string (G1) so a handler cannot pass an arbitrary message where a client is
// parsing a known code — the values are part of the frontend's contract, not
// prose. The underlying type stays string, so the JSON on the wire is
// unchanged.
type Code string

const (
	NotSignedIn           Code = "not-signed-in"
	UserDoesNotExist      Code = "user-does-not-exist"
	EventNotFound         Code = "event-not-found"
	UserNotEventOwner     Code = "user-not-event-owner"
	RemindeeEmailNotFound Code = "remindee-email-not-found"
	InvalidCredentials    Code = "invalid-credentials"
	OtpExpired            Code = "otp-expired"
	OtpInvalidCode        Code = "otp-invalid-code"
	OtpTooManyAttempts    Code = "otp-too-many-attempts"
	OtpSendFailed         Code = "otp-send-failed"
	OtpRateLimited        Code = "otp-rate-limited"
	InvalidIdToken        Code = "invalid-id-token"
	// NotInvited: the email is not on the invite-only allowlist
	NotInvited Code = "not-invited"
	// NotAuthorized: the user is signed in but lacks permission (e.g. not an inviter)
	NotAuthorized Code = "not-authorized"
	// InvalidEmail: the provided email failed validation
	InvalidEmail Code = "invalid-email"
	// CannotRemoveSelf: an admin tried to remove their own access / role
	CannotRemoveSelf Code = "cannot-remove-self"
	// SuperAdminImmutable: attempted to modify or remove a super admin via the app
	SuperAdminImmutable Code = "super-admin-immutable"
	// InvalidRole: the requested role is not grantable by the actor
	InvalidRole Code = "invalid-role"
	// Internal: a generic server-side error (details are logged, not returned)
	Internal Code = "internal-error"
	// EmailUnchanged: the requested new email equals the current one
	EmailUnchanged Code = "email-unchanged"
	// EmailTaken: the requested new email already belongs to another account
	EmailTaken Code = "email-taken"
	// GatheringNotScheduled: the event has no confirmed gathering time yet
	GatheringNotScheduled Code = "gathering-not-scheduled"
	// InvalidEventType: the event type is not one of the defined EventTypes
	InvalidEventType Code = "invalid-event-type"
	// PayloadTooLarge: a create/edit payload exceeded a cardinality cap
	PayloadTooLarge Code = "payload-too-large"
	// InvalidImage: the uploaded avatar was not a decodable JPEG or PNG
	InvalidImage Code = "invalid-image"
	// ImageTooLarge: the uploaded avatar exceeded the upload size cap
	ImageTooLarge Code = "image-too-large"
	// InvalidName: a name field was sent but was blank after trimming. Names
	// may be edited but not erased — DisplayName falls back to them.
	InvalidName Code = "invalid-name"
)

// Sentinel error returned by signInHelper when an email is not allowlisted, so
// callers can distinguish it from other sign-in failures and return NotInvited.
var ErrNotInvited = errors.New(string(NotInvited))

type GoogleAPIError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Status  string      `json:"status"`
	Details interface{} `json:"details"`
	Errors  interface{} `json:"errors"`
}

func (e *GoogleAPIError) Error() string {
	s, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintln("GoogleAPIError: <error parsing json>")
	}

	return fmt.Sprintln("GoogleAPIError: ", string(s))
}
