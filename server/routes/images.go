// The shared half of every image upload: turning whatever a client sends into
// bytes this server produced itself.
//
// Two features upload photos — profile avatars and Settle Up receipts (F22) —
// and they want different shapes (a 256px centre-cropped square; a 2000px
// aspect-preserving page you have to be able to *read*) but exactly the same
// defences: cap the encoded payload before decoding it, cap the decoded bytes,
// refuse an oversized canvas before Go's decoders allocate it, flatten alpha so
// a transparent PNG doesn't come out black, and re-encode as JPEG.
//
// Those defences live here rather than in either caller, because a second copy
// of them is a second place for one of them to be quietly left out.
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
	"strings"

	"github.com/gin-gonic/gin"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/responses"
)

// Sentinel errors so the pipeline can report *why* it refused without knowing
// anything about HTTP — several route groups map them to the same responses.
var (
	errImageInvalid  = errors.New(string(errs.InvalidImage))
	errImageTooLarge = errors.New(string(errs.ImageTooLarge))
)

// jpegContentType is the type of everything these pipelines store. It is a
// constant rather than something carried from the upload precisely because the
// upload's own type is discarded.
const jpegContentType = "image/jpeg"

// decodeImagePayload turns the `image` field of a request into raw bytes. It
// accepts a data URL (what FileReader/canvas produce, and what the frontend
// sends) or a bare base64 string, and enforces the caller's caps on both the
// encoded and the decoded form.
func decodeImagePayload(payload string, maxEncodedBytes, maxDecodedBytes int) ([]byte, error) {
	encoded := strings.TrimSpace(payload)
	if encoded == "" {
		return nil, errImageInvalid
	}

	// data:image/png;base64,<...> — only the base64 form is accepted. A
	// percent-encoded data URL is legal but nothing produces one for an image,
	// and guessing would mean decoding two ways.
	if strings.HasPrefix(encoded, "data:") {
		comma := strings.IndexByte(encoded, ',')
		if comma < 0 || !strings.Contains(strings.ToLower(encoded[:comma]), ";base64") {
			return nil, errImageInvalid
		}
		encoded = encoded[comma+1:]
	}

	// Checked before decoding: base64 is the form we actually received, so this
	// is the cap that keeps a huge payload from being expanded at all.
	if len(encoded) > maxEncodedBytes {
		return nil, errImageTooLarge
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errImageInvalid
	}
	if len(raw) > maxDecodedBytes {
		return nil, errImageTooLarge
	}
	if len(raw) == 0 {
		return nil, errImageInvalid
	}
	return raw, nil
}

// decodeImageWithin decodes a JPEG or PNG, refusing anything whose declared
// canvas exceeds maxPixels.
//
// The header is read first and the bounds checked BEFORE Decode runs, because
// Go's decoders allocate from the declared bounds before reading any pixels — a
// 200KB PNG of flat colour can declare an enormous canvas, so a byte cap does
// not bound memory on its own.
func decodeImageWithin(raw []byte, maxPixels int) (image.Image, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, errImageInvalid
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, errImageInvalid
	}
	if config.Width*config.Height > maxPixels {
		return nil, errImageTooLarge
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, errImageInvalid
	}
	return src, nil
}

// flattenOnWhite copies the given area of src onto an opaque white canvas.
//
// One pass, and not optional: JPEG has no alpha channel, so a transparent PNG
// left as-is comes out with black wherever it was clear.
func flattenOnWhite(src image.Image, area image.Rectangle) *image.RGBA {
	flat := image.NewRGBA(image.Rect(0, 0, area.Dx(), area.Dy()))
	draw.Draw(flat, flat.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), src, area.Min, draw.Over)
	return flat
}

// boxDownscaleTo reduces an opaque RGBA image to width x height by averaging
// each destination pixel's source footprint.
//
// A box filter, not nearest-neighbour: dropping pixels at these ratios makes a
// face look like it was photographed through a screen door, and makes the small
// print on a receipt unreadable. It is only ever used to shrink (callers clamp
// the target to the source), which is the case a box filter handles well — the
// stdlib has no resampler, and this is a couple of dozen lines against a new
// dependency.
func boxDownscaleTo(src *image.RGBA, width, height int) *image.RGBA {
	sourceWidth, sourceHeight := src.Bounds().Dx(), src.Bounds().Dy()
	if sourceWidth == width && sourceHeight == height {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		y0, y1 := span(y, height, sourceHeight)
		for x := 0; x < width; x++ {
			x0, x1 := span(x, width, sourceWidth)

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

// span returns the half-open source range one destination pixel averages over,
// guaranteeing at least one source pixel and never running past the source.
func span(i, dstSize, srcSize int) (int, int) {
	lo, hi := i*srcSize/dstSize, (i+1)*srcSize/dstSize
	if hi <= lo {
		hi = lo + 1
	}
	if hi > srcSize {
		hi = srcSize
	}
	if lo >= hi {
		lo = hi - 1
	}
	return lo, hi
}

// encodeJPEGAt re-encodes an image, mapping an encoder failure onto the
// pipeline's own sentinel so callers never have to reason about it.
func encodeJPEGAt(img image.Image, quality int, what string) ([]byte, error) {
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
		logger.StdErr.Printf("re-encoding %s: %v", what, err)
		return nil, errImageInvalid
	}
	return out.Bytes(), nil
}

// respondToImageError maps the pipeline's sentinels onto HTTP. Anything else is
// a storage failure — logged by the db layer, reported as internal.
func respondToImageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errImageInvalid):
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidImage})
	case errors.Is(err, errImageTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, responses.Error{Error: errs.ImageTooLarge})
	default:
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
	}
}

// bindImagePayload reads the `image` field of an upload request, capping the
// body before binding reads it — the byte caps inside the pipeline only run
// once the whole payload is already in memory, which is too late if the body is
// a gigabyte.
//
// Returns false having already written the response when the payload is
// unusable, so callers read as `if !ok { return }`.
func bindImagePayload(c *gin.Context, maxRequestBytes int64) (string, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)

	payload := struct {
		Image string `json:"image" binding:"required"`
	}{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		// A body over the cap fails here rather than in decodeImagePayload, so
		// it has to be reported as the size problem it is.
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, responses.Error{Error: errs.ImageTooLarge})
			return "", false
		}
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidImage})
		return "", false
	}
	return payload.Image, true
}
