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
package routes

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // register PNG with image.Decode; we only ever encode JPEG
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
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

// Sentinel errors so the shared helper can report *why* it refused without
// knowing anything about HTTP — F6 maps them to the same responses from a
// different route group.
var (
	errAvatarInvalid  = errors.New(errs.InvalidImage)
	errAvatarTooLarge = errors.New(errs.ImageTooLarge)
)

// decodeAvatarPayload turns the `image` field of a request into raw bytes. It
// accepts a data URL (what FileReader/canvas produce, and what the frontend
// sends) or a bare base64 string, and enforces the size caps on both the
// encoded and decoded forms.
func decodeAvatarPayload(payload string) ([]byte, error) {
	encoded := strings.TrimSpace(payload)
	if encoded == "" {
		return nil, errAvatarInvalid
	}

	// data:image/png;base64,<...> — only the base64 form is accepted. A
	// percent-encoded data URL is legal but nothing produces one for an image,
	// and guessing would mean decoding two ways.
	if strings.HasPrefix(encoded, "data:") {
		comma := strings.IndexByte(encoded, ',')
		if comma < 0 || !strings.Contains(strings.ToLower(encoded[:comma]), ";base64") {
			return nil, errAvatarInvalid
		}
		encoded = encoded[comma+1:]
	}

	// Checked before decoding: base64 is the form we actually received, so
	// this is the cap that keeps a huge payload from being expanded at all.
	if len(encoded) > maxAvatarEncodedBytes {
		return nil, errAvatarTooLarge
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errAvatarInvalid
	}
	if len(raw) > maxAvatarDecodedBytes {
		return nil, errAvatarTooLarge
	}
	if len(raw) == 0 {
		return nil, errAvatarInvalid
	}
	return raw, nil
}

// normalizeAvatar decodes an uploaded JPEG or PNG and re-encodes it as the
// canonical 256x256 JPEG: centre-cropped to a square, flattened onto white,
// box-filtered down, stripped of every bit of metadata the original carried.
//
// Note that re-encoding also discards an EXIF orientation tag, so a photo that
// relied on one arrives rotated. That is why the upload dialog offers rotate
// buttons and bakes the rotation into the pixels before sending.
func normalizeAvatar(raw []byte) ([]byte, error) {
	// Read the header first and refuse an oversized canvas before Decode gets
	// the chance to allocate it.
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, errAvatarInvalid
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, errAvatarInvalid
	}
	if config.Width*config.Height > maxAvatarPixels {
		return nil, errAvatarTooLarge
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, errAvatarInvalid
	}

	square := centeredSquare(src)
	size := square.Dx()
	if size > avatarSize {
		size = avatarSize
	}

	// Flatten onto white in one pass: JPEG has no alpha channel, so a
	// transparent PNG left as-is would come out with black where it was clear.
	flat := image.NewRGBA(image.Rect(0, 0, square.Dx(), square.Dy()))
	draw.Draw(flat, flat.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), src, square.Min, draw.Over)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, boxDownscale(flat, size), &jpeg.Options{Quality: avatarJPEGQuality}); err != nil {
		logger.StdErr.Println("re-encoding an avatar:", err)
		return nil, errAvatarInvalid
	}
	return out.Bytes(), nil
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

// boxDownscale reduces a square opaque RGBA image to size x size by averaging
// each destination pixel's source footprint.
//
// A box filter, not nearest-neighbour: dropping pixels at these ratios makes a
// face look like it was photographed through a screen door. It is only ever
// used to shrink (the caller clamps size to the source), which is the case a
// box filter handles well — the stdlib has no resampler, and this is a dozen
// lines against a new dependency.
func boxDownscale(src *image.RGBA, size int) *image.RGBA {
	sourceSize := src.Bounds().Dx()
	if sourceSize == size {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		y0, y1 := y*sourceSize/size, (y+1)*sourceSize/size
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < size; x++ {
			x0, x1 := x*sourceSize/size, (x+1)*sourceSize/size
			if x1 <= x0 {
				x1 = x0 + 1
			}

			var r, g, b, n uint32
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					i := sx * 4
					r += uint32(row[i])
					g += uint32(row[i+1])
					b += uint32(row[i+2])
					n++
				}
			}

			o := dst.PixOffset(x, y)
			dst.Pix[o] = uint8(r / n)
			dst.Pix[o+1] = uint8(g / n)
			dst.Pix[o+2] = uint8(b / n)
			dst.Pix[o+3] = 0xff
		}
	}
	return dst
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
const avatarContentType = "image/jpeg"

// respondToAvatarError maps the pipeline's sentinels onto HTTP. Anything else
// is a storage failure — logged by the db layer, reported as internal.
func respondToAvatarError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errAvatarInvalid):
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidImage})
	case errors.Is(err, errAvatarTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, responses.Error{Error: errs.ImageTooLarge})
	default:
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
	}
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
	// Cap the body before binding reads it — the byte caps inside the pipeline
	// only run once the whole payload is already in memory.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarRequestBytes)

	payload := struct {
		Image string `json:"image" binding:"required"`
	}{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		// A body over the cap fails here rather than in decodeAvatarPayload, so
		// it has to be reported as the size problem it is.
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, responses.Error{Error: errs.ImageTooLarge})
			return
		}
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidImage})
		return
	}

	authUser := utils.GetAuthUser(c)
	updatedAt, err := saveAvatarForUser(authUser.Id, payload.Image)
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
