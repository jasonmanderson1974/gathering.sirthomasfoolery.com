package responses

import "sirtom/server/models"

type Error struct {
	Error interface{} `json:"error" binding:"required"`
}

// Health is the health check body. Status is "ok" or "unavailable", Mongo is
// "ok" or "down", and Version is the release the binary was built from ("dev"
// for an unstamped build) so a deploy can confirm which one is actually
// serving.
type Health struct {
	Status  string `json:"status"`
	Mongo   string `json:"mongo"`
	Version string `json:"version"`
}

// SearchContacts is the result of a contacts search. HasContactsAccess is false when
// the user has no Google account linked or hasn't granted the contacts scope; Contacts
// is empty in that case rather than the request failing.
type SearchContacts struct {
	Contacts          []models.User `json:"contacts"`
	HasContactsAccess bool          `json:"hasContactsAccess"`
}
