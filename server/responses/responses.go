package responses

import "sirtom/server/models"

type Error struct {
	Error interface{} `json:"error" binding:"required"`
}

// SearchContacts is the result of a contacts search. HasContactsAccess is false when
// the user has no Google account linked or hasn't granted the contacts scope; Contacts
// is empty in that case rather than the request failing.
type SearchContacts struct {
	Contacts          []models.User `json:"contacts"`
	HasContactsAccess bool          `json:"hasContactsAccess"`
}
