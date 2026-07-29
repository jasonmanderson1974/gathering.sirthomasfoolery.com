package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// Avatar is a member's uploaded profile photo, stored in its own collection
// keyed by the user's id.
//
// Deliberately NOT a field on the User document. GetUserById runs on every
// authenticated request and getEvent embeds a whole *models.User per
// respondent, so a couple of hundred kilobytes of Binary sitting on the user
// row would be read constantly by code that only ever wants a name. Keeping
// the bytes in a separate collection means the only thing that fetches them is
// the one route that serves them; everything else works from
// User.AvatarUpdatedAt, which doubles as the has-an-avatar flag and the
// cache-buster.
//
// Stored bytes are always the canonical re-encode produced by the upload
// handler (256x256 JPEG), never whatever the client sent.
type Avatar struct {
	// Id is the owning user's id — one avatar per account, upserted in place.
	Id          primitive.ObjectID `json:"_id" bson:"_id"`
	Data        primitive.Binary   `json:"-" bson:"data"`
	ContentType string             `json:"contentType" bson:"contentType"`
	UpdatedAt   primitive.DateTime `json:"updatedAt" bson:"updatedAt"`
}
