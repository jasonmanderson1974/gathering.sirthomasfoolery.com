package models

import (
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Representation of a User in the mongoDB database
type User struct {
	TimezoneOffset int `json:"timezoneOffset" bson:"timezoneOffset"`

	// Profile info
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Email     string             `json:"email" bson:"email,omitempty"`
	FirstName string             `json:"firstName" bson:"firstName,omitempty"`
	LastName  string             `json:"lastName" bson:"lastName,omitempty"`
	Phone     string             `json:"phone" bson:"phone,omitempty"`
	Picture   string             `json:"picture" bson:"picture,omitempty"`

	// Nickname is what the club actually calls this person. When set it stands
	// in for the name everywhere it is displayed — see DisplayName. The real
	// name is kept, not replaced, so the roll and the directory can still show
	// both.
	Nickname string `json:"nickname" bson:"nickname,omitempty"`

	// AvatarUpdatedAt is set when the user uploads a profile photo and unset
	// when they remove it, so it serves as both the has-an-avatar flag and the
	// cache-buster on the (immutably cached) /users/:userId/avatar URL. The
	// bytes themselves live in the avatars collection — see models/avatar.go.
	//
	// A pointer because absent must be distinguishable from the zero time: the
	// UI shows the uploaded photo only when this is non-nil, otherwise it falls
	// back to the Google `picture` URL and then to a monogram.
	AvatarUpdatedAt *primitive.DateTime `json:"avatarUpdatedAt" bson:"avatarUpdatedAt,omitempty"`

	// Whether the user has set a custom name for themselves, i.e. don't change their name when they sign in
	HasCustomName *bool `json:"hasCustomName" bson:"hasCustomName,omitempty"`

	// Role is the user's access level in the invite-only Fellowship
	// (superAdmin/admin/member/guest). Empty ⇒ treated as member. See roles.go.
	Role Role `json:"role" bson:"role,omitempty"`

	// CalendarAccounts is a mapping from {`email_CALENDARTYPE` => CalendarAccount} that contains all the
	// additional accounts the user wants to see google calendar events for
	CalendarAccounts map[string]CalendarAccount `json:"calendarAccounts" bson:"calendarAccounts,omitempty"`

	// The calendarAccountKey of the account the user first signed in with
	PrimaryAccountKey *string `json:"primaryAccountKey" bson:"primaryAccountKey,omitempty"`

	// Google OAuth stuff
	TokenOrigin TokenOriginType `json:"-" bson:"tokenOrigin,omitempty"`

	// Calendar options
	CalendarOptions *CalendarOptions `json:"calendarOptions" bson:"calendarOptions,omitempty"`
}

// DisplayName is the name to show for this user: the nickname when they have
// one, otherwise "First Last".
//
// Every name a user sees should come from here rather than concatenating the
// two name fields, so setting a nickname takes effect everywhere at once. The
// exceptions are deliberate: the roll and the directory pair the nickname with
// the real name so admins can still tell who is who.
//
// A user with neither a nickname nor a name yields "" — the caller decides what
// to fall back to (a stored snapshot, an email, "Guest"), because the sensible
// answer differs by surface.
func (u *User) DisplayName() string {
	if nickname := strings.TrimSpace(u.Nickname); nickname != "" {
		return nickname
	}
	return strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
}

// Declare the possible types of TokenOrigin
type TokenOriginType string

const (
	Undefined TokenOriginType = ""
	IOS       TokenOriginType = "ios"
	ANDROID   TokenOriginType = "android"
	WEB       TokenOriginType = "web"
)

type UserStatus string

const (
	FREE UserStatus = "free"
	BUSY UserStatus = "busy"
)
