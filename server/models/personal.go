// Private, per-gathering scratch space: a person's own lists (F19) and their own
// free-form note (F20).
//
// Both live in their OWN collections rather than as fields on the Event, and
// that placement is the whole security design. GET /events/:id hands the entire
// event document to any signed-in caller — so anything private hung off
// models.Event is one forgotten projection away from being everybody's. Keyed by
// (userId, eventId) in a separate collection, the leak isn't guarded against;
// it's impossible.
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// PersonalLists is one user's private lists on one gathering.
//
// The lists themselves are the same EventList/EventListItem types the shared
// lists use, so kinds, nesting, fractional ordering and checklist state all
// behave identically — the only difference is who can reach them. Items on this
// path deliberately carry no AuthorName or CheckedByName: there is exactly one
// author, so the snapshot would be a name that could only ever go stale
// unnoticed (nothing re-resolves it here the way routes/display_names.go does
// for shared lists).
type PersonalLists struct {
	Id primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	// json:"-" on both keys: the client already knows whose event this is, and
	// an id it never sees is an id it can never be tempted to send back as an
	// authorization claim.
	UserId  primitive.ObjectID `json:"-" bson:"userId"`
	EventId primitive.ObjectID `json:"-" bson:"eventId"`

	Lists []EventList `json:"lists" bson:"lists"`
}

// PersonalNote is one user's private markdown note on one gathering — a single
// free-form document, not a collection of named notes.
type PersonalNote struct {
	Id      primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	UserId  primitive.ObjectID `json:"-"   bson:"userId"`
	EventId primitive.ObjectID `json:"-"   bson:"eventId"`

	// NO omitempty: clearing a note back to blank is a legitimate edit, and
	// omitempty would store it as "absent" — which reads back the same here, but
	// would quietly diverge the moment anything distinguished the two.
	Text string `json:"text" bson:"text"`

	UpdatedAt primitive.DateTime `json:"updatedAt" bson:"updatedAt,omitempty"`
}
