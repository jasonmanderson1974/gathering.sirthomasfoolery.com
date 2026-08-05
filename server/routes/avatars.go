// Avatar upload: turning whatever a client sends into one canonical image.
//
// Everything that reaches the avatars collection goes through
// saveAvatarForUser, so the bytes on disk are always a 256x256 JPEG the server
// produced itself. That is deliberate — it is what lets the serving route hand
// out a fixed content type, keeps a member's photo from being a hundred
// different sizes, and drops EXIF (which can carry the location the photo was
// taken) as a side effect of re-encoding rather than as a step someone has to
// remember.
//
// The helper is exported to the package rather than inlined in the handler
// because F6's admin-on-behalf upload runs the identical pipeline against a
// different user id.
//
// The generic defences — payload caps, the decode-bomb guard, alpha flattening,
// the box filter — live in routes/images.go and are shared with Settle Up's
// receipt photos (F22). What stays here is only what is specific to a face in a
// small circle: the square crop and the 256px target.
package routes

import (
	"image"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/models"
	"sirtom/server/responses"
	"sirtom/server/utils"
)

const (
	// avatarSize is the edge of the stored square, in pixels. Big enough for
	// the largest place an avatar appears (the 96px settings block on a 2x
	// display) with room to spare, small enough that the whole thing is a
	// ~15KB JPEG.
	avatarSize = 256

	// avatarJPEGQuality is a deliberate notch below the default 90: at 256px
	// the difference is invisible and the file is meaningfully smaller.
	avatarJPEGQuality = 85

	// maxAvatarEncodedBytes / maxAvatarDecodedBytes bound the upload. The
	// frontend crops to 256x256 before sending (~15KB), so these only ever
	// catch a hand-rolled client — but they have to be enforced here, since
	// that is exactly the client that would send a 12MB original.
	maxAvatarEncodedBytes = 300 * 1024
	maxAvatarDecodedBytes = 200 * 1024

	// maxAvatarRequestBytes caps the whole JSON body before it is read into
	// memory. The size checks above happen after binding, which is too late if
	// the body is a gigabyte.
	maxAvatarRequestBytes = 512 * 1024

	// maxAvatarPixels bounds what the decoders are allowed to allocate. A
	// 200KB PNG of flat colour can declare an enormous canvas, and Go's
	// decoders allocate from the declared bounds before reading any pixels —
	// so the byte caps above do not bound memory on their own. 16 megapixels
	// still admits a full-resolution phone photo (a 4032x3024 shot is 12.2MP).
	maxAvatarPixels = 16 * 1000 * 1000
)

// decodeAvatarPayload turns the `image` field of a request into raw bytes,
// under the avatar caps.
func decodeAvatarPayload(payload string) ([]byte, error) {
	return decodeImagePayload(payload, maxAvatarEncodedBytes, maxAvatarDecodedBytes)
}

// normalizeAvatar decodes an uploaded JPEG or PNG and re-encodes it as the
// canonical 256x256 JPEG: centre-cropped to a square, flattened onto white,
// box-filtered down, stripped of every bit of metadata the original carried.
//
// Note that re-encoding also discards an EXIF orientation tag, so a photo that
// relied on one arrives rotated. That is why the upload dialog offers rotate
// buttons and bakes the rotation into the pixels before sending.
func normalizeAvatar(raw []byte) ([]byte, error) {
	src, err := decodeImageWithin(raw, maxAvatarPixels)
	if err != nil {
		return nil, err
	}

	square := centeredSquare(src)
	size := square.Dx()
	if size > avatarSize {
		size = avatarSize
	}

	return encodeJPEGAt(
		boxDownscale(flattenOnWhite(src, square), size),
		avatarJPEGQuality,
		"an avatar",
	)
}

// centeredSquare returns the largest centred square within an image's bounds,
// so a rectangular upload is cropped rather than squashed.
func centeredSquare(img image.Image) image.Rectangle {
	bounds := img.Bounds()
	size := bounds.Dx()
	if bounds.Dy() < size {
		size = bounds.Dy()
	}
	return image.Rect(
		bounds.Min.X+(bounds.Dx()-size)/2,
		bounds.Min.Y+(bounds.Dy()-size)/2,
		bounds.Min.X+(bounds.Dx()-size)/2+size,
		bounds.Min.Y+(bounds.Dy()-size)/2+size,
	)
}

// boxDownscale reduces a square image to size x size — the square case of
// boxDownscaleTo, which is all an avatar ever needs.
func boxDownscale(src *image.RGBA, size int) *image.RGBA {
	return boxDownscaleTo(src, size, size)
}

// saveAvatarForUser validates, canonicalizes and stores an avatar for the given
// account, returning the timestamp the client should use as the cache-busting
// `?v=` on the serving URL.
func saveAvatarForUser(userId primitive.ObjectID, payload string) (primitive.DateTime, error) {
	raw, err := decodeAvatarPayload(payload)
	if err != nil {
		return 0, err
	}
	canonical, err := normalizeAvatar(raw)
	if err != nil {
		return 0, err
	}
	return db.UpsertAvatar(userId, canonical, avatarContentType)
}

// avatarContentType is the type of everything the pipeline stores. It is a
// constant rather than something carried from the upload precisely because the
// upload's type is discarded.
const avatarContentType = jpegContentType

// respondToAvatarError maps the pipeline's sentinels onto HTTP.
func respondToAvatarError(c *gin.Context, err error) {
	respondToImageError(c, err)
}

// @Summary Sets the signed-in user's profile photo
// @Description Accepts a base64 data URL (JPEG or PNG). The image is centre-cropped,
// @Description downscaled to 256x256 and re-encoded as JPEG before storage, so EXIF
// @Description metadata on the original is discarded.
// @Tags user
// @Accept json
// @Produce json
// @Param payload body object{image=string} true "Data URL of the image to store"
// @Success 200 {object} object{avatarUpdatedAt=int}
// @Failure 400 {object} responses.Error "invalid-image"
// @Failure 413 {object} responses.Error "image-too-large"
// @Router /user/avatar [put]
func updateAvatar(c *gin.Context) {
	payload, ok := bindImagePayload(c, maxAvatarRequestBytes)
	if !ok {
		return
	}

	authUser := utils.GetAuthUser(c)
	updatedAt, err := saveAvatarForUser(authUser.Id, payload)
	if err != nil {
		respondToAvatarError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"avatarUpdatedAt": updatedAt})
}

// @Summary Removes the signed-in user's profile photo
// @Description Idempotent — removing a photo that isn't there succeeds.
// @Tags user
// @Produce json
// @Success 200
// @Router /user/avatar [delete]
func deleteAvatar(c *gin.Context) {
	authUser := utils.GetAuthUser(c)
	if err := db.DeleteAvatar(authUser.Id); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

// avatarETag is the stored timestamp in milliseconds, quoted as RFC 9110
// requires. The bytes for a given timestamp never change — an upload always
// writes a fresh one — so it is a strong validator.
//
// Milliseconds rather than the RFC 3339 string the API serializes
// avatarUpdatedAt as: an ETag is opaque, only ever echoed back in
// If-None-Match, so the compact form is the better one. The client builds its
// `?v=` from the serialized field and never needs to agree with this.
func avatarETag(avatar *models.Avatar) string {
	return `"` + strconv.FormatInt(int64(avatar.UpdatedAt), 10) + `"`
}
