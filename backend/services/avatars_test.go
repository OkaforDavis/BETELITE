package services

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	return img
}

// withEXIF inserts an APP1 EXIF segment (with a fake GPS marker) after the
// JPEG start-of-image marker, like a phone camera photo.
func withEXIF(jpg []byte) []byte {
	payload := append([]byte("Exif\x00\x00"), []byte("GPSLatitude=6.5244N GPSLongitude=3.3792E")...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte((len(payload) + 2) & 0xFF)}
	seg = append(seg, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

func TestProcessAvatarStripsLocationAndSquares(t *testing.T) {
	var buf bytes.Buffer
	jpeg.Encode(&buf, testImage(400, 300), nil)
	in := withEXIF(buf.Bytes())
	if !bytes.Contains(in, []byte("GPSLatitude")) {
		t.Fatal("test setup: EXIF not inserted")
	}

	out, err := ProcessAvatar(in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPS")) {
		t.Fatal("location metadata must be removed")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "jpeg" || cfg.Width != 256 || cfg.Height != 256 {
		t.Fatalf("want 256x256 jpeg, got %dx%d %s (%v)", cfg.Width, cfg.Height, format, err)
	}
}

func TestProcessAvatarAcceptsPNGRejectsJunk(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, testImage(300, 500))
	if _, err := ProcessAvatar(buf.Bytes()); err != nil {
		t.Fatalf("PNG should be accepted: %v", err)
	}
	for _, junk := range [][]byte{nil, []byte("<svg onload=alert(1)>"), []byte("GIF89a....")} {
		if _, err := ProcessAvatar(junk); err == nil {
			t.Errorf("%q should be rejected", junk)
		}
	}
}
