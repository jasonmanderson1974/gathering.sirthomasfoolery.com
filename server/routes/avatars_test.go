package routes

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

// --- fixtures -------------------------------------------------------------

// encodePNG builds a solid-colour PNG of the given size. Generated rather than
// checked in so the fixtures stay readable and there is nothing binary in the
// tree.
func encodePNG(t *testing.T, width, height int, c color.Color) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding the PNG fixture: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, width, height int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encoding the JPEG fixture: %v", err)
	}
	return buf.Bytes()
}

// pngHeaderDeclaring returns a PNG consisting of nothing but a signature and an
// IHDR announcing the given dimensions. DecodeConfig needs no more than that,
// which is the whole point: it stands in for a decompression bomb — a tiny file
// that would make a full decode allocate gigabytes — without the test itself
// having to allocate them.
func pngHeaderDeclaring(width, height uint32) []byte {
	var ihdr bytes.Buffer
	ihdr.WriteString("IHDR")
	_ = binary.Write(&ihdr, binary.BigEndian, width)
	_ = binary.Write(&ihdr, binary.BigEndian, height)
	ihdr.Write([]byte{8, 6, 0, 0, 0}) // 8-bit RGBA, no compression/filter/interlace

	var out bytes.Buffer
	out.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	_ = binary.Write(&out, binary.BigEndian, uint32(ihdr.Len()-4)) // length excludes the type
	out.Write(ihdr.Bytes())
	_ = binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(ihdr.Bytes()))
	return out.Bytes()
}

func dataURL(mime string, raw []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
}

// --- decodeAvatarPayload --------------------------------------------------

func TestDecodeAvatarPayload_AcceptsDataURLAndBareBase64(t *testing.T) {
	raw := []byte("not really an image, but decodeAvatarPayload doesn't care")

	for name, payload := range map[string]string{
		"data URL":         dataURL("image/png", raw),
		"bare base64":      base64.StdEncoding.EncodeToString(raw),
		"surrounding ws":   "  " + dataURL("image/jpeg", raw) + "\n",
		"uppercase BASE64": "data:image/png;BASE64," + base64.StdEncoding.EncodeToString(raw),
	} {
		got, err := decodeAvatarPayload(payload)
		if err != nil {
			t.Errorf("%s: unexpected error %v", name, err)
			continue
		}
		if !bytes.Equal(got, raw) {
			t.Errorf("%s: decoded %q, want %q", name, got, raw)
		}
	}
}

func TestDecodeAvatarPayload_Rejects(t *testing.T) {
	tests := map[string]struct {
		payload string
		want    error
	}{
		"empty":               {"", errAvatarInvalid},
		"whitespace only":     {"   ", errAvatarInvalid},
		"not base64":          {"@@@ this is not base64 @@@", errAvatarInvalid},
		"empty payload":       {dataURL("image/png", nil), errAvatarInvalid},
		"non-base64 data URL": {"data:image/png,%89PNG", errAvatarInvalid},
		"data URL no comma":   {"data:image/png;base64", errAvatarInvalid},
		// A base64 blob past the encoded cap must be refused before it is
		// expanded, so this is the check that actually bounds memory.
		"over the encoded cap": {strings.Repeat("A", maxAvatarEncodedBytes+4), errAvatarTooLarge},
	}
	for name, tc := range tests {
		_, err := decodeAvatarPayload(tc.payload)
		if err != tc.want {
			t.Errorf("%s: got error %v, want %v", name, err, tc.want)
		}
	}
}

func TestDecodeAvatarPayload_RejectsOverDecodedCap(t *testing.T) {
	// base64 inflates by 4/3, so the two caps leave a band where a payload
	// passes the encoded check and only the decoded one catches it. This
	// exercises that band, which is the only thing the second check is for.
	raw := make([]byte, maxAvatarDecodedBytes+1)
	encoded := base64.StdEncoding.EncodeToString(raw)
	if len(encoded) > maxAvatarEncodedBytes {
		t.Fatalf("fixture is %d encoded bytes, over the %d encoded cap — the first check would fire and this test would be asserting the wrong guard",
			len(encoded), maxAvatarEncodedBytes)
	}
	if _, err := decodeAvatarPayload(encoded); err != errAvatarTooLarge {
		t.Errorf("got %v, want %v — %d encoded bytes decode to %d, over the %d cap",
			err, errAvatarTooLarge, len(encoded), len(raw), maxAvatarDecodedBytes)
	}
}

// --- normalizeAvatar ------------------------------------------------------

// decodeResult reads back what normalizeAvatar produced, asserting it really is
// a JPEG.
func decodeResult(t *testing.T, out []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("the pipeline's output did not decode as JPEG: %v", err)
	}
	return img
}

func TestNormalizeAvatar_DownscalesToSquare(t *testing.T) {
	out, err := normalizeAvatar(encodePNG(t, 800, 800, color.RGBA{R: 12, G: 200, B: 90, A: 255}))
	if err != nil {
		t.Fatalf("normalizing an 800x800 PNG: %v", err)
	}

	img := decodeResult(t, out)
	if got := img.Bounds(); got.Dx() != avatarSize || got.Dy() != avatarSize {
		t.Errorf("output is %dx%d, want %dx%d", got.Dx(), got.Dy(), avatarSize, avatarSize)
	}

	// The colour has to survive the box filter and the JPEG round trip — a
	// resize that shifted hue would be a lot harder to notice in production.
	r, g, b, _ := img.At(avatarSize/2, avatarSize/2).RGBA()
	assertNear(t, "red", r>>8, 12)
	assertNear(t, "green", g>>8, 200)
	assertNear(t, "blue", b>>8, 90)
}

func TestNormalizeAvatar_DoesNotUpscaleSmallImages(t *testing.T) {
	out, err := normalizeAvatar(encodePNG(t, 64, 64, color.RGBA{R: 255, A: 255}))
	if err != nil {
		t.Fatalf("normalizing a 64x64 PNG: %v", err)
	}
	if got := decodeResult(t, out).Bounds().Dx(); got != 64 {
		t.Errorf("output is %dpx wide, want 64 — a small avatar should be left at its own size, not blown up", got)
	}
}

func TestNormalizeAvatar_CentreCropsRectangles(t *testing.T) {
	// A wide image: the crop must be square, and taken from the middle rather
	// than the corner.
	wide := image.NewNRGBA(image.Rect(0, 0, 600, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 600; x++ {
			// Blue edges, red centre column band.
			if x >= 150 && x < 450 {
				wide.Set(x, y, color.RGBA{R: 255, A: 255})
			} else {
				wide.Set(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, wide); err != nil {
		t.Fatalf("encoding the wide fixture: %v", err)
	}

	out, err := normalizeAvatar(buf.Bytes())
	if err != nil {
		t.Fatalf("normalizing a 600x300 PNG: %v", err)
	}
	img := decodeResult(t, out)
	if got := img.Bounds(); got.Dx() != got.Dy() {
		t.Fatalf("output is %dx%d, want a square", got.Dx(), got.Dy())
	}

	// The 300px-wide centre band is exactly the square that should have been
	// kept, so every corner of the output is red, not blue.
	for _, p := range []image.Point{{X: 2, Y: 2}, {X: avatarSize - 3, Y: 2}, {X: 2, Y: avatarSize - 3}} {
		r, _, b, _ := img.At(p.X, p.Y).RGBA()
		if r < b {
			t.Errorf("pixel %v is blue — the crop was taken from the edge rather than the centre", p)
		}
	}
}

func TestNormalizeAvatar_FlattensTransparencyOntoWhite(t *testing.T) {
	// JPEG has no alpha. A transparent PNG left unflattened comes out black,
	// which is a very visible way to ruin someone's photo.
	out, err := normalizeAvatar(encodePNG(t, 300, 300, color.NRGBA{R: 0, G: 0, B: 0, A: 0}))
	if err != nil {
		t.Fatalf("normalizing a fully transparent PNG: %v", err)
	}
	r, g, b, _ := decodeResult(t, out).At(avatarSize/2, avatarSize/2).RGBA()
	assertNear(t, "red", r>>8, 255)
	assertNear(t, "green", g>>8, 255)
	assertNear(t, "blue", b>>8, 255)
}

func TestNormalizeAvatar_AcceptsJPEG(t *testing.T) {
	out, err := normalizeAvatar(encodeJPEG(t, 500, 500, color.RGBA{R: 40, G: 40, B: 220, A: 255}))
	if err != nil {
		t.Fatalf("normalizing a JPEG: %v", err)
	}
	if got := decodeResult(t, out).Bounds().Dx(); got != avatarSize {
		t.Errorf("output is %dpx wide, want %d", got, avatarSize)
	}
}

func TestNormalizeAvatar_RejectsJunk(t *testing.T) {
	for name, raw := range map[string][]byte{
		"plain text":         []byte("this is not an image at all"),
		"truncated PNG":      encodePNG(t, 64, 64, color.White)[:20],
		"GIF (unregistered)": []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
	} {
		if _, err := normalizeAvatar(raw); err != errAvatarInvalid {
			t.Errorf("%s: got %v, want %v", name, err, errAvatarInvalid)
		}
	}
}

func TestNormalizeAvatar_RejectsOversizedCanvas(t *testing.T) {
	// Tiny file, enormous declared canvas. Rejecting on the header is the only
	// thing standing between this and a multi-gigabyte allocation.
	bomb := pngHeaderDeclaring(30000, 30000)
	if len(bomb) > 100 {
		t.Fatalf("the bomb fixture is %d bytes — it is meant to be tiny, or it isn't testing what it claims", len(bomb))
	}
	if _, err := normalizeAvatar(bomb); err != errAvatarTooLarge {
		t.Errorf("got %v, want %v — a 900-megapixel canvas must be refused before Decode allocates it", err, errAvatarTooLarge)
	}
}

// --- boxDownscale ---------------------------------------------------------

func TestBoxDownscale_AveragesRatherThanSamples(t *testing.T) {
	// A 2x2 checkerboard down to 1x1: averaging gives mid-grey, picking one
	// pixel gives black or white. This is the difference between a downscaled
	// face and a moiré pattern.
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	src.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	src.Set(1, 0, color.RGBA{A: 255})
	src.Set(0, 1, color.RGBA{A: 255})

	got := boxDownscale(src, 1)
	r, _, _, a := got.At(0, 0).RGBA()
	if r>>8 != 127 {
		t.Errorf("downscaled red = %d, want 127 (the average of 0 and 255)", r>>8)
	}
	if a>>8 != 255 {
		t.Errorf("downscaled alpha = %d, want 255 — the result feeds a JPEG encoder and must be opaque", a>>8)
	}
}

func TestBoxDownscale_SameSizeIsAPassthrough(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	if got := boxDownscale(src, 4); got != src {
		t.Error("downscaling to the source size copied the image; it should return it untouched")
	}
}

func assertNear(t *testing.T, channel string, got, want uint32) {
	t.Helper()
	// JPEG is lossy and the box filter rounds, so exact equality is the wrong
	// assertion; anything within a few levels is the same colour to a person.
	const tolerance = 6
	diff := int(got) - int(want)
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		t.Errorf("%s = %d, want %d (±%d)", channel, got, want, tolerance)
	}
}
