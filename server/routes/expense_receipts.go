// Receipt photos for the Settle Up ledger (F22).
//
// The same pipeline as avatars — cap, decode, flatten, downscale, re-encode —
// with the one difference that matters: no crop, and a much larger target. An
// avatar is a face in a small circle, so a centred square loses nothing. A
// receipt is a document, and the whole point of attaching it is that somebody
// can read the line items three weeks later; squaring it off would cut the total
// off the bottom.
//
// Re-encoding also strips EXIF, which on a phone photo of a restaurant bill
// includes the GPS coordinates of the restaurant. That is a privacy property of
// this route, not an incidental one — a member attaching a receipt is not
// volunteering where they were.
package routes

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/models"
	"sirtom/server/responses"
)

const (
	// receiptMaxEdge caps the long edge of a stored receipt. 2000px keeps the
	// small print on a full-page till receipt legible when zoomed, at roughly
	// 200-400KB a photo.
	receiptMaxEdge = 2000

	// receiptJPEGQuality is a little above the avatar's: at this size, text is
	// what suffers first from ringing around high-contrast edges.
	receiptJPEGQuality = 82

	// The upload caps. The frontend downscales to receiptMaxEdge before sending,
	// so a well-behaved client lands far under these — they exist for the client
	// that sends the 12MB original straight off the camera roll.
	maxReceiptEncodedBytes = 4 * 1024 * 1024
	maxReceiptDecodedBytes = 3 * 1024 * 1024
	maxReceiptRequestBytes = 6 * 1024 * 1024

	// maxReceiptPixels bounds what the decoders may allocate, independently of
	// the byte caps above (a small PNG can declare an enormous canvas). 40MP
	// comfortably admits any phone camera.
	maxReceiptPixels = 40 * 1000 * 1000

	// maxReceiptsPerExpense keeps one expense's photos bounded. A bill that
	// needs more than five pages wants to be more than one expense.
	maxReceiptsPerExpense = 5
)

// normalizeReceipt turns an upload payload into the canonical receipt image:
// aspect preserved, long edge capped, flattened onto white, stripped of
// metadata. Returns the bytes and their final dimensions, which the ledger
// stores so a client can lay out a thumbnail without decoding anything.
func normalizeReceipt(payload string) ([]byte, int, int, error) {
	raw, err := decodeImagePayload(payload, maxReceiptEncodedBytes, maxReceiptDecodedBytes)
	if err != nil {
		return nil, 0, 0, err
	}

	src, err := decodeImageWithin(raw, maxReceiptPixels)
	if err != nil {
		return nil, 0, 0, err
	}

	bounds := src.Bounds()
	width, height := fitWithin(bounds.Dx(), bounds.Dy(), receiptMaxEdge)

	encoded, err := encodeJPEGAt(
		boxDownscaleTo(flattenOnWhite(src, bounds), width, height),
		receiptJPEGQuality,
		"a receipt",
	)
	if err != nil {
		return nil, 0, 0, err
	}
	return encoded, width, height, nil
}

// fitWithin scales width x height down so neither edge exceeds maxEdge, keeping
// the aspect ratio and never scaling up. Both edges are floored at 1 — a 3000x1
// scan is degenerate but must not round to a zero-height canvas.
func fitWithin(width, height, maxEdge int) (int, int) {
	longest := width
	if height > longest {
		longest = height
	}
	if longest <= maxEdge {
		return width, height
	}

	scaled := func(edge int) int {
		out := edge * maxEdge / longest
		if out < 1 {
			return 1
		}
		return out
	}
	return scaled(width), scaled(height)
}

// @Summary Attaches a receipt photo to an expense
// @Description Accepts a base64 data URL (JPEG or PNG). The image is downscaled to a 2000px long edge and re-encoded as JPEG before storage, so EXIF metadata — including the GPS tag on a phone photo — is discarded. The member who entered the expense, or any admin.
// @Tags events
// @Accept json
// @Produce json
// @Param eventId path string true "Event ID"
// @Param expenseId path string true "Expense ID"
// @Param payload body object{image=string} true "Data URL of the image to store"
// @Success 200 {object} models.ExpenseReceiptRef
// @Failure 400 {object} responses.Error "invalid-image"
// @Failure 413 {object} responses.Error "image-too-large / too-many-receipts"
// @Router /events/{eventId}/expenses/{expenseId}/receipts [post]
func addExpenseReceipt(c *gin.Context) {
	// The body cap has to come first — before the expense lookup, before
	// anything — or a gigabyte arrives in memory regardless of the answer.
	payload, ok := bindImagePayload(c, maxReceiptRequestBytes)
	if !ok {
		return
	}

	event, user, expense, allowed := requireEditableExpense(c)
	if !allowed {
		return
	}

	// Counted against the collection rather than len(expense.Receipts): two
	// uploads racing would each read the same document and each see room.
	count, err := db.CountExpenseReceipts(expense.Id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if count >= maxReceiptsPerExpense {
		c.JSON(http.StatusRequestEntityTooLarge, responses.Error{Error: errs.TooManyReceipts})
		return
	}

	canonical, width, height, err := normalizeReceipt(payload)
	if err != nil {
		respondToImageError(c, err)
		return
	}

	now := primitive.NewDateTimeFromTime(time.Now())
	receipt := models.ExpenseReceipt{
		Id:          primitive.NewObjectID(),
		ExpenseId:   expense.Id,
		EventId:     event.Id,
		Data:        primitive.Binary{Subtype: 0x00, Data: canonical},
		ContentType: jpegContentType,
		Width:       width,
		Height:      height,
		CreatedAt:   now,
	}

	// Bytes first, then the reference — a failure between the two leaves an
	// orphan nobody can see rather than a thumbnail that 404s. See the header of
	// db/expense_receipts.go.
	if err := db.InsertExpenseReceipt(receipt); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	ref := models.ExpenseReceiptRef{
		Id:          receipt.Id,
		ContentType: receipt.ContentType,
		Width:       receipt.Width,
		Height:      receipt.Height,
		UploadedAt:  now,
		UploadedBy:  user.Id,
	}
	change := newExpenseChange(user, models.ExpenseActionReceiptAdded, now, nil)
	if err := db.AddExpenseReceiptRef(expense.Id, ref, change); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, ref)
}

// @Summary Serves a receipt photo
// @Description Readable by anyone signed in who can see the gathering, guests included — the ledger is theirs to read too.
// @Tags events
// @Produce jpeg
// @Param eventId path string true "Event ID"
// @Param expenseId path string true "Expense ID"
// @Param receiptId path string true "Receipt ID"
// @Success 200
// @Failure 404 {object} responses.Error "receipt-not-found"
// @Router /events/{eventId}/expenses/{expenseId}/receipts/{receiptId} [get]
func getExpenseReceipt(c *gin.Context) {
	event, _, _, ok := loadExpenseContext(c)
	if !ok {
		return
	}
	expense, found := loadExpense(c, event)
	if !found {
		return
	}

	receipt, found := loadReceipt(c, expense)
	if !found {
		return
	}

	// The bytes for a receipt id never change — an edit uploads a new one — so
	// the id itself is a strong validator and the response can be cached hard.
	etag := `"` + receipt.Id.Hex() + `"`
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Data(http.StatusOK, receipt.ContentType, receipt.Data.Data)
}

// @Summary Removes a receipt photo from an expense
// @Description The member who entered the expense, or any admin.
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Param expenseId path string true "Expense ID"
// @Param receiptId path string true "Receipt ID"
// @Success 200
// @Failure 403 {object} responses.Error "not-authorized"
// @Router /events/{eventId}/expenses/{expenseId}/receipts/{receiptId} [delete]
func deleteExpenseReceipt(c *gin.Context) {
	_, user, expense, ok := requireEditableExpense(c)
	if !ok {
		return
	}

	receipt, found := loadReceipt(c, expense)
	if !found {
		return
	}

	// Reference first, then the bytes — the opposite order would leave the
	// ledger rendering a thumbnail whose image is already gone.
	now := primitive.NewDateTimeFromTime(time.Now())
	change := newExpenseChange(user, models.ExpenseActionReceiptRemoved, now, nil)
	if err := db.RemoveExpenseReceiptRef(expense.Id, receipt.Id, change); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if err := db.DeleteExpenseReceipt(receipt.Id); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.Status(http.StatusOK)
}

// loadReceipt fetches the receipt named by :receiptId and confirms it belongs to
// this expense, writing the 404 itself when it doesn't.
func loadReceipt(c *gin.Context, expense *models.Expense) (*models.ExpenseReceipt, bool) {
	receiptId, valid := objectIdOrNil(c.Param("receiptId"))
	if !valid {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.ReceiptNotFound})
		return nil, false
	}

	receipt, err := db.GetExpenseReceipt(receiptId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, false
	}
	if receipt == nil || receipt.ExpenseId != expense.Id {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.ReceiptNotFound})
		return nil, false
	}
	return receipt, true
}
