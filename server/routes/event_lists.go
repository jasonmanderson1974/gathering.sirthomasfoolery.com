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

const (
	maxListNameLength = 100
	// Addresses run long, so an item gets more room than a poll option.
	maxListItemLength = 300
	maxListsPerEvent  = 20
	maxItemsPerList   = 100
	// How many levels of nesting an item may sit at: a top-level item, a child
	// and a grandchild. Unlike the caps above this one is exact rather than
	// advisory, because an item's depth can never GROW: it is fixed when the
	// item is added, and the only thing that changes it afterwards is a move,
	// which either keeps it or flattens it to zero (see validateMoveParent). A
	// check against the event as read therefore cannot go stale in the
	// direction that would matter.
	maxListItemDepth = 3
	// The gap left between adjacent items' Order values (F17). Ordering is
	// fractional rather than a 0,1,2,... reindex so that moving an item writes
	// only that item: a drop between two neighbours takes the midpoint
	// (prev+next)/2, above the first takes next-step, below the last takes
	// prev+step, and the first item on an empty list takes step. Negative orders
	// are legal and expected — repeatedly dropping at the top marches downward.
	//
	// The gap is what buys the precision. float64 holds ~52 bits of mantissa, so
	// halving a 1024-wide gap degenerates only after ~60 consecutive drops into
	// that same shrinking gap; a list is capped at maxItemsPerList entries, which
	// puts that out of reach in practice. There is deliberately no rebalance
	// path — it would mean rewriting a whole sibling group, the very thing this
	// scheme exists to avoid.
	listItemOrderStep = 1024
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
	errListTooDeep      = "list-depth-exceeded"
	errNotChecklist     = "not-a-checklist"
	errInvalidMove      = "invalid-item-move"
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
	return kind == models.ListKindText ||
		kind == models.ListKindLocation ||
		kind == models.ListKindChecklist
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

// listItemDepth returns an item's 0-based depth on its list, walking ParentId
// upwards.
//
// A parent that no longer exists counts as no parent at all: the item is an
// orphan — its parent was deleted between someone else's read and write — and
// both this and the frontend treat it as top-level rather than hiding it.
//
// The walk is bounded by the item count so that malformed data terminates. This
// code cannot produce a cycle — a parent is chosen from items that already
// exist, and the only operation that changes an item's parent afterwards is a
// move, which clears it rather than pointing it somewhere new — but a
// hand-edited document could.
func listItemDepth(list *models.EventList, item *models.EventListItem) int {
	depth := 0
	current := item
	for range list.Items {
		if current.ParentId == nil {
			break
		}
		parent, found := findListItem(list, current.ParentId.Hex())
		if !found {
			break
		}
		depth++
		current = parent
	}
	return depth
}

// maxOrderInList returns the highest Order on a list, or 0 for an empty one.
//
// Deliberately list-wide rather than per-sibling-group, even though Order is
// only ever compared within a group: the list-wide maximum is by definition at
// least the group's, so stepping past it puts a new entry last among its own
// siblings — which is where an append belongs — with one pass and no parent
// bookkeeping. A list of items that predate ordering is all zeroes, so the
// first entry added after F17 lands at listItemOrderStep.
//
// The maximum is taken over the values actually present rather than floored at
// zero, so a list dragged entirely into negative territory still appends above
// its last entry instead of jumping to listItemOrderStep.
func maxOrderInList(list *models.EventList) float64 {
	highest := 0.0
	for i := range list.Items {
		if i == 0 || list.Items[i].Order > highest {
			highest = list.Items[i].Order
		}
	}
	return highest
}

// validateMoveParent reports whether a move may keep the parent the payload
// asks for. Extracted from the handler so the rule is testable without a
// request, since it is the invariant the whole depth model rests on.
//
// A move may name a parent ONLY to say "leave me where I am in the tree and
// just reorder me among my siblings" — same list, same parent. Every other move
// flattens the item to the top level. So a move either preserves an item's
// depth exactly or drops it to zero: DEPTH ONLY EVER SHRINKS.
//
// That is what keeps addEventListItem's depth check exact rather than advisory.
// If a move could re-parent an item downward, a parent's depth could grow
// between the read that checked it and the write that used it, and a subtree
// could be pushed past maxListItemDepth by two people acting at once.
func validateMoveParent(item *models.EventListItem, payloadParentId string, sameList bool) bool {
	if payloadParentId == "" {
		return true // flattening to the top level is always allowed
	}
	if !sameList || item.ParentId == nil {
		return false
	}
	return item.ParentId.Hex() == payloadParentId
}

// collectListItems resolves ids to the item structs on a list, preserving the
// order of the ids. Ids that are not on the list are skipped.
func collectListItems(list *models.EventList, ids []primitive.ObjectID) []models.EventListItem {
	items := make([]models.EventListItem, 0, len(ids))
	for _, id := range ids {
		for i := range list.Items {
			if list.Items[i].Id == id {
				items = append(items, list.Items[i])
				break
			}
		}
	}
	return items
}

// collectDescendantIds returns rootId together with every item nested beneath
// it — the id set that one atomic $pull removes when an item is deleted.
//
// The set is computed from the event as read, so an item someone else adds
// under this subtree in the same instant is not in it: that item survives as an
// orphan and reappears at the top level, which is a far better failure than a
// partially deleted subtree. The seen set both dedupes the $in and guarantees
// termination on malformed data.
func collectDescendantIds(list *models.EventList, rootId primitive.ObjectID) []primitive.ObjectID {
	ids := []primitive.ObjectID{rootId}
	seen := map[primitive.ObjectID]bool{rootId: true}

	// Breadth-first over the flat array. maxListItemDepth bounds how deep this
	// can go in practice, but the loop is written against the frontier so it
	// stays correct if that cap ever moves.
	frontier := []primitive.ObjectID{rootId}
	for len(frontier) > 0 {
		var next []primitive.ObjectID
		for _, item := range list.Items {
			if item.ParentId == nil || seen[item.Id] {
				continue
			}
			for _, parentId := range frontier {
				if *item.ParentId == parentId {
					seen[item.Id] = true
					next = append(next, item.Id)
					break
				}
			}
		}
		ids = append(ids, next...)
		frontier = next
	}
	return ids
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
// @Param payload body object{text=string,parentId=string} true "Item text, optionally nested under an existing item"
// @Success 200 {object} models.EventListItem
// @Router /events/{eventId}/lists/{listId}/items [post]
func addEventListItem(c *gin.Context) {
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

	// Nesting, when asked for. The depth check is exact rather than advisory: a
	// parent's depth cannot GROW under us between this read and the write below,
	// since a move only ever preserves an item's depth or flattens it to zero.
	// The worst a concurrent move can do is make this check stricter than it
	// needed to be.
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

	// Identity and credit come from the session, never the payload (E3). Order
	// is stamped here too rather than by the client: an add is always an append,
	// so the list already in hand is all it takes, and no request needs to carry
	// a position it has no say in.
	item := models.EventListItem{
		Id:         primitive.NewObjectID(),
		Text:       text,
		ParentId:   parentId,
		Order:      maxOrderInList(list) + listItemOrderStep,
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

// @Summary Deletes a list item and everything nested under it (own, or any when the caller is a member or above)
// @Description The right to delete an item is the right to delete its subtree, so a reply-like child added by someone else goes with its parent.
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

	// Cascade: the item plus everything beneath it, removed in one update so no
	// one can observe a half-deleted subtree.
	itemIds := collectDescendantIds(list, item.Id)
	if _, err := db.DeleteEventListItems(event.Id, list.Id, itemIds); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Moves a list item to a new position, within its list or onto another (own, or any when the caller is a member or above)
// @Description The right to move an item is the right to delete it, since a move is a delete and a re-add. The item brings everything nested under it. Unless the payload names the item's CURRENT parent for a same-list reorder, the item becomes top-level in its new position — a move never nests an item under a new parent. The client computes `order`; see listItemOrderStep.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param itemId path string true "Item ID"
// @Param payload body object{targetListId=string,order=number,parentId=string} true "Destination list, the new order, and optionally the item's current parent to keep it nested"
// @Success 200
// @Router /events/{eventId}/lists/{listId}/items/{itemId}/move [put]
func moveEventListItem(c *gin.Context) {
	payload := struct {
		TargetListId string `json:"targetListId" binding:"required"`
		// A POINTER with binding:"required", for the same reason Checked is one:
		// a bare float64 would fail the binding on 0, and 0 is exactly what a
		// drop at the top of a migrated list computes.
		Order *float64 `json:"order" binding:"required"`
		// Optional, and only ever the item's existing parent — see
		// validateMoveParent.
		ParentId string `json:"parentId"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	// The client computes the order, so it is the client that could send
	// nonsense. JSON cannot even encode NaN or ±Inf, but an order that is not a
	// real number would poison every later comparison in the sibling group, and
	// the check costs nothing. Any finite value is fair game: order carries no
	// privilege, and by design any mover may place an item anywhere.
	if math.IsNaN(*payload.Order) || math.IsInf(*payload.Order, 0) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errInvalidMove})
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
	// Unlike delete, a move is NOT idempotent when the item has gone: there is
	// nothing to place, and silently reporting success would leave the mover
	// looking at an entry that never arrived.
	item, itemFound := findListItem(list, c.Param("itemId"))
	if !itemFound {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}
	// Moving is delete-and-re-add, so it takes the same right as deleting —
	// which means a member may tidy anyone's entry into place, not just their
	// own.
	if !viewer.canDeleteItem(*item) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}

	sameList := payload.TargetListId == list.Id.Hex()
	target, targetFound := findEventList(event, payload.TargetListId)
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
			moved, moveErr = db.SetEventListItemOrder(event.Id, list.Id, item.Id, *payload.Order)
		} else {
			moved, moveErr = db.FlattenEventListItemToTopLevel(event.Id, list.Id, item.Id, *payload.Order)
		}
		if moveErr != nil {
			logger.StdErr.Println(moveErr)
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			return
		}
		if !moved {
			// The list or the item went away between the read and the write.
			c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
			return
		}
		c.Status(http.StatusOK)
		return
	}

	// Cross-list. Advisory, exactly like the cap on adding: measured against the
	// event as read, so two simultaneous moves at the boundary can both pass.
	if len(target.Items)+len(itemIds) > maxItemsPerList {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errListFull})
		return
	}

	// The structs are taken from the event AS READ, and that is where this
	// feature's accepted races live — the same trade the cascade delete makes:
	//
	//   - A text edit or a check landing between this read and the write below
	//     is lost, because the pre-edit struct is what gets pushed.
	//   - A child added under this subtree in the same instant is not in
	//     itemIds, so it stays on the source list and resurfaces there as a
	//     top-level orphan (flattenListItems renders orphans at the root).
	//
	// Both are better than a partially moved subtree, which is what any
	// multi-write alternative would risk.
	moving := collectListItems(list, itemIds)
	if len(moving) == 0 {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}
	// Only the root is repositioned — matched by id rather than by position, so
	// this does not quietly depend on collectDescendantIds returning the root
	// first. Its descendants keep the parentId and order they had: they compete
	// only with their own siblings, and that group travels with them intact.
	for i := range moving {
		if moving[i].Id == item.Id {
			moving[i].ParentId = nil
			moving[i].Order = *payload.Order
			break
		}
	}

	moved, err := db.MoveEventListItems(event.Id, list.Id, target.Id, itemIds, moving)
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

// @Summary Ticks or unticks a checklist item
// @Description Open to every signed-in user, guests included — the same reasoning as adding an item. Only the last person to change the state is recorded; there is no history.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param listId path string true "List ID"
// @Param itemId path string true "Item ID"
// @Param payload body object{checked=bool} true "New checked state"
// @Success 200
// @Router /events/{eventId}/lists/{listId}/items/{itemId}/checked [put]
func setEventListItemChecked(c *gin.Context) {
	payload := struct {
		// A POINTER with binding:"required": a bare bool would fail the binding
		// on `false`, i.e. on every uncheck.
		Checked *bool `json:"checked" binding:"required"`
	}{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: err.Error()})
		return
	}

	// No gate beyond being signed in: ticking things off is the point of a
	// checklist, and it is as reversible as adding an item.
	event, user, _, ok := loadListContext(c)
	if !ok {
		return
	}
	list, found := findEventList(event, c.Param("listId"))
	if !found {
		c.JSON(http.StatusNotFound, responses.Error{Error: errListNotFound})
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

	// Credit comes from the session, and both directions are recorded — an
	// uncheck is as much a change of state as a check.
	matched, err := db.SetEventListItemChecked(
		event.Id, list.Id, item.Id,
		*payload.Checked,
		user.Id, user.DisplayName(),
		primitive.NewDateTimeFromTime(time.Now()),
	)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if !matched {
		// Removed between the read and the write.
		c.JSON(http.StatusNotFound, responses.Error{Error: errListItemNotFound})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary Returns just the shared lists on an event, display names resolved
// @Description The cheap refresh: getEvent resolves a user per availability response and per sign-up response before it ever reaches the lists, which is far too much work to see one new entry appear.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200 {array} models.EventList
// @Router /events/{eventId}/lists [get]
func getEventLists(c *gin.Context) {
	// Any signed-in user, which is the same access getEvent grants: post-E3
	// every event route requires a session, and the lists are already part of
	// that response.
	event, _, _, ok := loadListContext(c)
	if !ok {
		return
	}

	resolveListDisplayNames(event.Lists)

	lists := event.Lists
	if lists == nil {
		// An event with no lists serializes as [] rather than null, so the
		// client can splice the result in without a nil check.
		lists = []models.EventList{}
	}
	c.JSON(http.StatusOK, lists)
}
