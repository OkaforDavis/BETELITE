package services

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func pngOf(w, h int) []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)))
	return b.Bytes()
}

func TestProcessCover(t *testing.T) {
	for _, size := range [][2]int{{1000, 1000}, {1920, 600}, {700, 1400}} {
		out, err := ProcessCover(pngOf(size[0], size[1]))
		if err != nil {
			t.Fatalf("%v: %v", size, err)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
		if err != nil || format != "jpeg" || cfg.Width != 1200 || cfg.Height != 675 {
			t.Errorf("%v: got %s %dx%d (%v)", size, format, cfg.Width, cfg.Height, err)
		}
	}
	if _, err := ProcessCover(pngOf(100, 100)); err == nil {
		t.Error("tiny picture should be refused")
	}
	if _, err := ProcessCover([]byte("<svg></svg>")); err == nil {
		t.Error("non-image should be refused")
	}
}
