package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Folder struct {
	Id     primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	UserId primitive.ObjectID `json:"userId" bson:"userId"`

	Name      string  `json:"name,omitempty" bson:"name,omitempty"`
	Color     *string `json:"color,omitempty" bson:"color,omitempty"`
	IsDeleted *bool   `json:"isDeleted,omitempty" bson:"isDeleted,omitempty"`

	// System "default" folders where events land automatically ("Invites
	// created" / "Invites received"). IsDefault folders can't be deleted;
	// DefaultKind is "created" or "received".
	IsDefault   *bool   `json:"isDefault,omitempty" bson:"isDefault,omitempty"`
	DefaultKind *string `json:"defaultKind,omitempty" bson:"defaultKind,omitempty"`

	EventIds []primitive.ObjectID `json:"eventIds" bson:"-"`
}
