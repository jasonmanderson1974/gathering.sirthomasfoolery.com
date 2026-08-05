// A person's private lists on a gathering (F19) — the "My Lists" tab.
//
// These behave exactly like the shared lists (F13–F18): the same three kinds,
// the same three levels of nesting, the same fractional ordering, the same
// atomic cascade delete and cross-list move. Everything below therefore reuses
// the validators and the caps in event_lists.go rather than restating them, so
// the two features can never disagree about what a legal list is.
//
// What is different is who may touch them, and the answer is: only you. There is
// no permission check in this file, and that absence is deliberate rather than
// an oversight. The userId goes into every Mongo filter (db/personal_lists.go),
// so a caller holding someone else's list id matches no document and gets the
// same 404 as a caller holding a fictional one. A check layered on top of that
// could only ever be redundant — or, worse, become the thing people trust while
// the filter quietly stops carrying the userId.
package routes

import (
	"encoding/json"
	"math"
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

// initPersonalRoutes registers the private lists and note on the events group.
//
// Registered on the SAME group and under the SAME :eventId parameter name as the
// shared lists, rather than in a group of its own. Gin rejects two different
// wildcard names at one position in the tree, and "my-lists" beside "lists" is
// just another static segment under a parameter that already exists.
func initPersonalRoutes(authed *gin.RouterGroup) {
	authed.GET("/:eventId/my-lists", getPersonalLists)
	authed.POST("/:eventId/my-lists", createPersonalList)
	authed.PATCH("/:eventId/my-lists/:listId", renamePersonalList)
	authed.DELETE("/:eventId/my-lists/:listId", deletePersonalList)
	authed.POST("/:eventId/my-lists/:listId/items", addPersonalListItem)
	authed.PUT("/:eventId/my-lists/:listId/items/:itemId", editPersonalListItem)
	authed.PUT("/:eventId/my-lists/:listId/items/:itemId/checked", setPersonalListItemChecked)
	authed.PUT("/:eventId/my-lists/:listId/items/:itemId/move", movePersonalListItem)
	authed.DELETE("/:eventId/my-lists/:listId/items/:itemId", deletePersonalListItem)

	authed.GET("/:eventId/my-notes", getPersonalNote)
	authed.PUT("/:eventId/my-notes", setPersonalNote)
}

// personalContext is everything a private-lists handler needs: whose data it is,
// which gathering it hangs off, and the lists as read.
type personalContext struct {
	UserId  primitive.ObjectID
	EventId primitive.ObjectID
	Lists   []models.EventList
}

// loadPersonalOwner resolves the signed-in user and the gathering, writing the
// error response itself when either is missing.
//
// The event is looked up but nothing about it is consulted — no ownership, no
// role, not even whether the caller can see it. It is resolved only so that a
// stale or invented link can't seed private documents against an id that was
// never a gathering.
func loadPersonalOwner(c *gin.Context) (primitive.ObjectID, primitive.ObjectID, bool) {
	user := authUserFrom(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
		return primitive.NilObjectID, primitive.NilObjectID, false
	}

	event, eventErr := db.GetEventByEitherId(c.Param("eventId"))
	if eventErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return primitive.NilObjectID, primitive.NilObjectID, false
	}
	if event == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.EventNotFound})
		return primitive.NilObjectID, primitive.NilObjectID, false
	}

	return user.Id, event.Id, true
}

// loadPersonalContext adds the caller's lists to loadPersonalOwner.
func loadPersonalContext(c *gin.Context) (personalContext, bool) {
	userId, eventId, ok := loadPersonalOwner(c)
	if !ok {
		return personalContext{}, false
	}

	lists, err := db.GetPersonalLists(userId, eventId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return personalContext{}, false
	}

	return personalContext{UserId: userId, EventId: eventId, Lists: lists}, true
}

// findPersonalList resolves :listId against the caller's own lists, writing the
// 404 itself. `idempotent` decides what a missing list means: a delete says
// "already gone, fine", everything else says "not found".
func (ctx personalContext) findList(c *gin.Context, idempotent bool) (*models.EventList, bool) {
	list, found := findListIn(ctx.Lists, c.Param("listId"))
	if !found {
		if idempotent {
			c.Status(http.StatusOK)
		} else {
			c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
		}
		return nil, false
	}
	return list, true
}

// @Summary Returns the caller's own private lists on a gathering
// @Description Private to the signed-in user: the userId is part of the query, so this can only ever return the caller's own lists. Unlike the shared lists, no display names are resolved — there is one author and the UI never shows their name.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200 {array} models.EventList
// @Router /events/{eventId}/my-lists [get]
func getPersonalLists(c *gin.Context) {
	ctx, ok := loadPersonalContext(c)
	if !ok {
		return
	}
	// Never null: someone who has never opened the tab has no document at all,
	// and the client splices the result in without a nil check.
	c.JSON(http.StatusOK, ctx.Lists)
}

// @Summary Creates one of the caller's own private lists
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param payload body object{name=string,kind=string} true "List name and kind (text|location|checklist)"
// @Success 200 {object} models.EventList
// @Router /events/{eventId}/my-lists [post]
func createPersonalList(c *gin.Context) {
	payload := struct {
		Name string `json:"name" binding:"required"`
		Kind string `json:"kind"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	ctx, ok := loadPersonalContext(c)
	if !ok {
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
	if len(ctx.Lists) >= maxListsPerEvent {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errTooManyLists})
		return
	}

	// The one place the document has to be conjured into existence: every other
	// write is a positional update, which matches nothing when there is no
	// document to match. Idempotent, so this is also the repair path for a
	// document that somehow never got made.
	if err := db.EnsurePersonalLists(ctx.UserId, ctx.EventId); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	list := models.EventList{
		Id:    primitive.NewObjectID(),
		Name:  name,
		Kind:  kind,
		Items: []models.EventListItem{},
	}
	if err := db.InsertPersonalList(ctx.UserId, ctx.EventId, list); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, list)
}

// @Summary Renames one of the caller's own private lists
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param payload body object{name=string} true "New list name"
// @Success 200
// @Router /events/{eventId}/my-lists/{listId} [patch]
func renamePersonalList(c *gin.Context) {
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

	ctx, ok := loadPersonalContext(c)
	if !ok {
		return
	}
	list, found := ctx.findList(c, false)
	if !found {
		return
	}

	if _, err := db.RenamePersonalList(ctx.UserId, ctx.EventId, list.Id, name); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Deletes one of the caller's own private lists and everything on it
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Success 200
// @Router /events/{eventId}/my-lists/{listId} [delete]
func deletePersonalList(c *gin.Context) {
	ctx, ok := loadPersonalContext(c)
	if !ok {
		return
	}
	list, found := ctx.findList(c, true) // already gone — idempotent
	if !found {
		return
	}

	if _, err := db.DeletePersonalList(ctx.UserId, ctx.EventId, list.Id); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Adds an item to one of the caller's own private lists
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param payload body object{text=string,parentId=string} true "Item text, optionally nested under an existing item"
// @Success 200 {object} models.EventListItem
// @Router /events/{eventId}/my-lists/{listId}/items [post]
func addPersonalListItem(c *gin.Context) {
	payload := struct {
		Text string `json:"text" binding:"required"`
		// Optional: the item this one hangs under. Absent means top-level.
		ParentId string `json:"parentId"`
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

	ctx, ok := loadPersonalContext(c)
	if !ok {
		return
	}
	list, found := ctx.findList(c, false)
	if !found {
		return
	}
	if len(list.Items) >= maxItemsPerList {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errListFull})
		return
	}

	// Nesting, when asked for — the same exact depth check the shared lists make,
	// and exact for the same reason: a move only ever preserves an item's depth
	// or flattens it, so a parent's depth cannot grow between this read and the
	// write below.
	var parentId *primitive.ObjectID
	if payload.ParentId != "" {
		parent, parentFound := findListItem(list, payload.ParentId)
		if !parentFound {
			c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
			return
		}
		if listItemDepth(list, parent)+1 >= maxListItemDepth {
			c.JSON(http.StatusBadRequest, responses.Error{Error: errListTooDeep})
			return
		}
		parentId = &parent.Id
	}

	// No AuthorName: there is one author, and a display-name snapshot that
	// nothing ever re-resolves (the way routes/display_names.go does for shared
	// lists) could only go stale unnoticed. UserId is still stamped, so the
	// documents stay self-describing.
	item := models.EventListItem{
		Id:        primitive.NewObjectID(),
		Text:      text,
		ParentId:  parentId,
		Order:     maxOrderInList(list) + listItemOrderStep,
		UserId:    ctx.UserId,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	modified, err := db.InsertPersonalListItem(ctx.UserId, ctx.EventId, list.Id, item)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if !modified {
		// The list was deleted between the read and the write — from another tab,
		// since there is only one person who could have done it.
		c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
		return
	}

	c.JSON(http.StatusOK, item)
}

// @Summary Edits an item on one of the caller's own private lists
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param itemId path string true "Item ID"
// @Param payload body object{text=string} true "New item text"
// @Success 200
// @Router /events/{eventId}/my-lists/{listId}/items/{itemId} [put]
func editPersonalListItem(c *gin.Context) {
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

	ctx, ok := loadPersonalContext(c)
	if !ok {
		return
	}
	list, found := ctx.findList(c, false)
	if !found {
		return
	}
	item, itemFound := findListItem(list, c.Param("itemId"))
	if !itemFound {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}

	if _, err := db.UpdatePersonalListItemText(ctx.UserId, ctx.EventId, list.Id, item.Id, text); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Deletes an item on one of the caller's own private lists, and everything nested under it
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param itemId path string true "Item ID"
// @Success 200
// @Router /events/{eventId}/my-lists/{listId}/items/{itemId} [delete]
func deletePersonalListItem(c *gin.Context) {
	ctx, ok := loadPersonalContext(c)
	if !ok {
		return
	}
	list, found := ctx.findList(c, true) // already gone — idempotent
	if !found {
		return
	}
	item, itemFound := findListItem(list, c.Param("itemId"))
	if !itemFound {
		c.Status(http.StatusOK) // already gone — idempotent
		return
	}

	// Cascade: the item plus everything beneath it, removed in one update so no
	// one can observe a half-deleted subtree.
	itemIds := collectDescendantIds(list, item.Id)
	if _, err := db.DeletePersonalListItems(ctx.UserId, ctx.EventId, list.Id, itemIds); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Moves an item on the caller's own private lists, within its list or onto another of theirs
// @Description The item brings everything nested under it. Unless the payload names the item's CURRENT parent for a same-list reorder, the item becomes top-level in its new position — a move never nests an item under a new parent. The client computes `order`; see listItemOrderStep.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param itemId path string true "Item ID"
// @Param payload body object{targetListId=string,order=number,parentId=string} true "Destination list, the new order, and optionally the item's current parent to keep it nested"
// @Success 200
// @Router /events/{eventId}/my-lists/{listId}/items/{itemId}/move [put]
func movePersonalListItem(c *gin.Context) {
	payload := struct {
		TargetListId string `json:"targetListId" binding:"required"`
		// A POINTER with binding:"required", and json.Number rather than float64,
		// for the reasons spelled out on moveEventListItem: a bare value fails the
		// binding on 0, which is exactly what a drop at the top of a migrated list
		// computes, and a float64 field would make the DECODER refuse an
		// unrepresentable literal before this handler ever runs.
		Order *json.Number `json:"order" binding:"required"`
		// Optional, and only ever the item's existing parent — see
		// validateMoveParent.
		ParentId string `json:"parentId"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	order, orderErr := payload.Order.Float64()
	if orderErr != nil || math.IsNaN(order) || math.IsInf(order, 0) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errInvalidMove})
		return
	}

	ctx, ok := loadPersonalContext(c)
	if !ok {
		return
	}
	list, found := ctx.findList(c, false)
	if !found {
		return
	}
	// Unlike delete, a move is NOT idempotent when the item has gone: there is
	// nothing to place, and reporting success would leave the mover looking at an
	// entry that never arrived.
	item, itemFound := findListItem(list, c.Param("itemId"))
	if !itemFound {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}

	sameList := payload.TargetListId == list.Id.Hex()
	// findListIn over the caller's OWN lists, which is what confines a move to
	// them: naming someone else's list as the target resolves to nothing here,
	// long before any write.
	target, targetFound := findListIn(ctx.Lists, payload.TargetListId)
	if !targetFound {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
		return
	}
	if !validateMoveParent(item, payload.ParentId, sameList) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errInvalidMove})
		return
	}

	itemIds := collectDescendantIds(list, item.Id)

	if sameList {
		var moveErr error
		var moved bool
		if payload.ParentId != "" {
			// A reorder among the siblings it already has.
			moved, moveErr = db.SetPersonalListItemOrder(ctx.UserId, ctx.EventId, list.Id, item.Id, order)
		} else {
			moved, moveErr = db.FlattenPersonalListItemToTopLevel(ctx.UserId, ctx.EventId, list.Id, item.Id, order)
		}
		if moveErr != nil {
			logger.StdErr.Println(moveErr)
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
		if !moved {
			c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
			return
		}
		c.Status(http.StatusOK)
		return
	}

	if len(target.Items)+len(itemIds) > maxItemsPerList {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errListFull})
		return
	}

	moving := collectListItems(list, itemIds)
	if len(moving) == 0 {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}
	// Only the root is repositioned — matched by id, not by position. Its
	// descendants keep the parentId and order they had: they compete only with
	// their own siblings, and that group travels with them intact.
	for i := range moving {
		if moving[i].Id == item.Id {
			moving[i].ParentId = nil
			moving[i].Order = order
			break
		}
	}

	moved, err := db.MovePersonalListItems(ctx.UserId, ctx.EventId, list.Id, target.Id, itemIds, moving)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if !moved {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Ticks or unticks an item on one of the caller's own private checklists
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param itemId path string true "Item ID"
// @Param payload body object{checked=bool} true "New checked state"
// @Success 200
// @Router /events/{eventId}/my-lists/{listId}/items/{itemId}/checked [put]
func setPersonalListItemChecked(c *gin.Context) {
	payload := struct {
		// A POINTER with binding:"required": a bare bool would fail the binding
		// on `false`, i.e. on every uncheck.
		Checked *bool `json:"checked" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	ctx, ok := loadPersonalContext(c)
	if !ok {
		return
	}
	list, found := ctx.findList(c, false)
	if !found {
		return
	}
	if list.Kind != models.ListKindChecklist {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errNotChecklist})
		return
	}
	item, itemFound := findListItem(list, c.Param("itemId"))
	if !itemFound {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}

	matched, err := db.SetPersonalListItemChecked(
		ctx.UserId, ctx.EventId, list.Id, item.Id,
		*payload.Checked,
		primitive.NewDateTimeFromTime(time.Now()),
	)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if !matched {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}

	c.Status(http.StatusOK)
}
