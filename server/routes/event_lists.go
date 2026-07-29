// Shared lists on an event (F13) — "Menu", "Bars to Visit". Routes are
// registered under the /events group by InitEvents.
//
// The split of rights is the point of the feature: the planner (or an admin)
// creates, renames and deletes the lists themselves, and everyone signed in
// fills them in. An entry belongs to whoever added it — they alone may edit it —
// while members and above may remove anyone's entry, so a list can be tidied
// without going through the planner.
package routes

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/responses"
)

const (
	maxListNameLength = 100
	// Addresses run long, so an item gets more room than a poll option.
	maxListItemLength = 300
	maxListsPerEvent  = 20
	maxItemsPerList   = 100
)

// Error codes specific to lists.
const (
	errEmptyListName    = "empty-list-name"
	errInvalidListKind  = "invalid-list-kind"
	errEmptyListItem    = "empty-list-item"
	errListNotFound     = "list-not-found"
	errListItemNotFound = "list-item-not-found"
	errTooManyLists     = "too-many-lists"
	errListFull         = "list-full"
)

// listViewer is the caller's identity as it bears on lists. Kept as a plain
// struct, like commentViewer, so the permission rules below stay unit-testable
// without a request or a database.
type listViewer struct {
	UserId       string
	IsEventOwner bool
	IsAdmin      bool
	IsMember     bool
}

// newListViewer builds the viewer for a signed-in user against a given event.
func newListViewer(user *models.User, event *models.Event) listViewer {
	if user == nil {
		return listViewer{}
	}
	role := user.EffectiveRole()
	userId := user.Id.Hex()
	isOwner := event != nil && event.OwnerId != primitive.NilObjectID && event.OwnerId.Hex() == userId
	return listViewer{
		UserId:       userId,
		IsEventOwner: isOwner,
		IsAdmin:      role.CanManageUsers(),
		IsMember:     role.CanInvite(),
	}
}

// canManageLists reports whether this viewer may create, rename or delete the
// lists on this event.
//
// This is requireEventManager's rule plus admins. Admins are added here rather
// than by widening requireEventManager, which deliberately does NOT grant them
// another person's event — polls, editing and scheduling all depend on that
// meaning, and lists are the one surface where the club's moderators were asked
// to be able to tidy up.
func (v listViewer) canManageLists(event *models.Event) bool {
	if v.UserId == "" {
		return false
	}
	if v.IsAdmin {
		return true
	}
	if event != nil && event.OwnerId != primitive.NilObjectID {
		return v.IsEventOwner
	}
	// Legacy ownerless event (created before E3 removed anonymous creation):
	// member or above, matching requireEventManager.
	return v.IsMember
}

// canEditItem reports whether this viewer may change an item's text. Own items
// only — there is no edit-anyone right at any role, because editing words
// someone else is credited with misrepresents them. Removing is the override.
func (v listViewer) canEditItem(item models.EventListItem) bool {
	return v.UserId != "" && item.UserId.Hex() == v.UserId
}

// canDeleteItem reports whether this viewer may remove an item: their own
// always, anyone's from member upwards.
func (v listViewer) canDeleteItem(item models.EventListItem) bool {
	if v.UserId == "" {
		return false
	}
	if item.UserId.Hex() == v.UserId {
		return true
	}
	return v.IsMember
}

// sanitizeListName trims a list name and reports whether it's usable.
func sanitizeListName(name string) (string, bool) {
	trimmed := trimAndTruncate(name, maxListNameLength)
	return trimmed, trimmed != ""
}

// sanitizeListItemText trims an item and reports whether it's usable.
func sanitizeListItemText(text string) (string, bool) {
	trimmed := trimAndTruncate(text, maxListItemLength)
	return trimmed, trimmed != ""
}

// validListKind reports whether kind is one this feature renders.
func validListKind(kind string) bool {
	return kind == models.ListKindText || kind == models.ListKindLocation
}

// findEventList locates a list on an event by hex id.
func findEventList(event *models.Event, listId string) (*models.EventList, bool) {
	for i := range event.Lists {
		if event.Lists[i].Id.Hex() == listId {
			return &event.Lists[i], true
		}
	}
	return nil, false
}

// findListItem locates an item on a list by hex id.
func findListItem(list *models.EventList, itemId string) (*models.EventListItem, bool) {
	for i := range list.Items {
		if list.Items[i].Id.Hex() == itemId {
			return &list.Items[i], true
		}
	}
	return nil, false
}

// loadListContext resolves the event, the signed-in user and the viewer for a
// list route, writing the error response itself when anything is missing.
func loadListContext(c *gin.Context) (*models.Event, *models.User, listViewer, bool) {
	user := authUserFrom(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
		return nil, nil, listViewer{}, false
	}

	event, eventErr := db.GetEventByEitherId(c.Param("eventId"))
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, nil, listViewer{}, false
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return nil, nil, listViewer{}, false
	}

	return event, user, newListViewer(user, event), true
}

// loadManageableList resolves the list named by :listId on an event the caller
// may manage. Writes the error response itself.
func loadManageableList(c *gin.Context) (*models.Event, *models.EventList, bool) {
	event, _, viewer, ok := loadListContext(c)
	if !ok {
		return nil, nil, false
	}
	if !viewer.canManageLists(event) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return nil, nil, false
	}
	list, found := findEventList(event, c.Param("listId"))
	if !found {
		return event, nil, true // caller decides: 404 or idempotent success
	}
	return event, list, true
}

// @Summary Creates a shared list on an event (planner or admin)
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{name=string,kind=string} true "List name and kind (text|location)"
// @Success 200 {object} models.EventList
// @Router /events/{eventId}/lists [post]
func createEventList(c *gin.Context) {
	payload := struct {
		Name string `json:"name" binding:"required"`
		Kind string `json:"kind"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	event, _, viewer, ok := loadListContext(c)
	if !ok {
		return
	}
	if !viewer.canManageLists(event) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	name, nameOk := sanitizeListName(payload.Name)
	if !nameOk {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errEmptyListName})
		return
	}
	kind := payload.Kind
	if kind == "" {
		kind = models.ListKindText
	}
	if !validListKind(kind) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errInvalidListKind})
		return
	}
	if len(event.Lists) >= maxListsPerEvent {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errTooManyLists})
		return
	}

	list := models.EventList{
		Id:    primitive.NewObjectID(),
		Name:  name,
		Kind:  kind,
		Items: []models.EventListItem{},
	}
	if err := db.InsertEventList(event.Id, list); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, list)
}

// @Summary Renames a shared list (planner or admin)
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param payload body object{name=string} true "New list name"
// @Success 200
// @Router /events/{eventId}/lists/{listId} [patch]
func renameEventList(c *gin.Context) {
	payload := struct {
		Name string `json:"name" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	name, nameOk := sanitizeListName(payload.Name)
	if !nameOk {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errEmptyListName})
		return
	}

	event, list, ok := loadManageableList(c)
	if !ok {
		return
	}
	if list == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
		return
	}

	if _, err := db.RenameEventList(event.Id, list.Id, name); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Deletes a shared list and everything on it (planner or admin)
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Success 200
// @Router /events/{eventId}/lists/{listId} [delete]
func deleteEventList(c *gin.Context) {
	event, list, ok := loadManageableList(c)
	if !ok {
		return
	}
	if list == nil {
		c.Status(http.StatusOK) // already gone — idempotent
		return
	}

	if _, err := db.DeleteEventList(event.Id, list.Id); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Adds an item to a shared list
// @Description Open to every signed-in user, guests included — filling in the lists is the point of them.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param payload body object{text=string} true "Item text"
// @Success 200 {object} models.EventListItem
// @Router /events/{eventId}/lists/{listId}/items [post]
func addEventListItem(c *gin.Context) {
	payload := struct {
		Text string `json:"text" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	text, textOk := sanitizeListItemText(payload.Text)
	if !textOk {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errEmptyListItem})
		return
	}

	// No gate beyond being signed in: filling in the lists is the point of them.
	event, user, _, ok := loadListContext(c)
	if !ok {
		return
	}
	list, found := findEventList(event, c.Param("listId"))
	if !found {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
		return
	}
	// Advisory cap: it is measured against the event as read, so two
	// simultaneous adds at the boundary can both pass and land at 101. That is
	// a guardrail against a runaway list, not a security boundary, and the
	// alternative — a read-modify-write of the whole array — is exactly the
	// lost-update bug this feature was designed to avoid.
	if len(list.Items) >= maxItemsPerList {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errListFull})
		return
	}

	// Identity and credit come from the session, never the payload (E3).
	item := models.EventListItem{
		Id:         primitive.NewObjectID(),
		Text:       text,
		UserId:     user.Id,
		AuthorName: user.DisplayName(),
		CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
	}

	modified, err := db.InsertEventListItem(event.Id, list.Id, item)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if !modified {
		// The list was deleted between the read and the write.
		c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
		return
	}

	c.JSON(http.StatusOK, item)
}

// @Summary Edits one of the caller's own list items
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param itemId path string true "Item ID"
// @Param payload body object{text=string} true "New item text"
// @Success 200
// @Router /events/{eventId}/lists/{listId}/items/{itemId} [put]
func editEventListItem(c *gin.Context) {
	payload := struct {
		Text string `json:"text" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	text, textOk := sanitizeListItemText(payload.Text)
	if !textOk {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errEmptyListItem})
		return
	}

	event, _, viewer, ok := loadListContext(c)
	if !ok {
		return
	}
	list, found := findEventList(event, c.Param("listId"))
	if !found {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
		return
	}
	item, itemFound := findListItem(list, c.Param("itemId"))
	if !itemFound {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}
	if !viewer.canEditItem(*item) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	if _, err := db.UpdateEventListItemText(event.Id, list.Id, item.Id, text); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Deletes a list item (own, or any when the caller is a member or above)
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param itemId path string true "Item ID"
// @Success 200
// @Router /events/{eventId}/lists/{listId}/items/{itemId} [delete]
func deleteEventListItem(c *gin.Context) {
	event, _, viewer, ok := loadListContext(c)
	if !ok {
		return
	}
	list, found := findEventList(event, c.Param("listId"))
	if !found {
		c.Status(http.StatusOK) // already gone — idempotent
		return
	}
	item, itemFound := findListItem(list, c.Param("itemId"))
	if !itemFound {
		c.Status(http.StatusOK) // already gone — idempotent
		return
	}
	if !viewer.canDeleteItem(*item) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	if _, err := db.DeleteEventListItem(event.Id, list.Id, item.Id); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}
