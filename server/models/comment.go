package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// Comment is a message in an event's discussion thread (C7). Stored in its own
// `comments` collection (keyed by EventId), mirroring EventResponse.
//
// Threads (C13): any top-level comment can be tagged as a thread root, after
// which replies hang off it via ThreadId. Nesting is one level deep — a comment
// with a ThreadId can never itself become a root. A root tagged MembersOnly is
// hidden, along with all of its replies, from anyone below member (see
// Role.CanSeeMembersOnly).
type Comment struct {
	Id      primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	EventId primitive.ObjectID `json:"eventId" bson:"eventId"`

	// UserId is the author's user id hex. Legacy rows may instead hold a guest's
	// typed-in name, with IsGuest true — the discussion is sign-in-only now, so
	// no new guest rows are written, but old ones still render.
	UserId  string `json:"userId" bson:"userId"`
	IsGuest bool   `json:"isGuest" bson:"isGuest"`

	// Denormalized author display name (guest name, or the account's "First Last").
	AuthorName string `json:"authorName" bson:"authorName"`

	Text      string              `json:"text" bson:"text"`
	CreatedAt primitive.DateTime  `json:"createdAt" bson:"createdAt"`
	UpdatedAt *primitive.DateTime `json:"updatedAt" bson:"updatedAt,omitempty"` // set when edited

	// IsThread marks this top-level comment as a thread root: it collapses in the
	// UI and accepts replies. Only ever true on a comment with no ThreadId.
	IsThread bool `json:"isThread" bson:"isThread,omitempty"`

	// MembersOnly hides this thread and every reply in it from guests. Only
	// meaningful on a thread root; replies inherit the root's setting rather than
	// carrying their own.
	MembersOnly bool `json:"membersOnly" bson:"membersOnly,omitempty"`

	// ThreadId is the root comment this is a reply to. Nil for top-level comments.
	ThreadId *primitive.ObjectID `json:"threadId" bson:"threadId,omitempty"`

	// ThreadedBy is the user id hex of whoever tagged this as a thread. With the
	// event owner and admins, it authorizes untagging and members-only toggling.
	// Not serialized — the API sends a computed `canManageThread` instead, so the
	// client never needs the raw id.
	ThreadedBy string `json:"-" bson:"threadedBy,omitempty"`

	// CanManageThread is computed per-request for the calling user (never stored):
	// true when the caller may untag this root or flip its members-only flag.
	CanManageThread bool `json:"canManageThread" bson:"-"`
}

// IsReply reports whether this comment hangs off a thread root.
func (c *Comment) IsReply() bool { return c.ThreadId != nil }
