package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/image/draw"

	"betelite-go/db"
)

// A cup's cover picture: a wide 16:9 banner shown on the cup card and page.
const (
	CoverMaxUpload = 5 << 20
	coverWidth     = 1200
	coverHeight    = 675
)

// ProcessCover crops an uploaded picture to a centred 16:9 banner and
// re-encodes it as a 1200x675 JPEG. Re-encoding drops metadata (such as GPS
// location) and anything that isn't a real image.
func ProcessCover(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > CoverMaxUpload {
		return nil, userErr(400, "Choose an image under 5 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, userErr(400, "Use a JPG, PNG or WebP picture")
	}
	if cfg.Width*cfg.Height > 40_000_000 {
		return nil, userErr(400, "That image is too large")
	}
	if cfg.Width < 320 || cfg.Height < 180 {
		return nil, userErr(400, "That picture is too small. Use one at least 640 pixels wide")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, userErr(400, "We couldn't read that image")
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dx()*9/16
	if h > b.Dy() {
		w, h = b.Dy()*16/9, b.Dy()
	}
	crop := image.Rect(0, 0, w, h).Add(image.Pt(b.Min.X+(b.Dx()-w)/2, b.Min.Y+(b.Dy()-h)/2))
	dst := image.NewRGBA(image.Rect(0, 0, coverWidth, coverHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Src, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// canEditCover checks that uid hosts the cup (or is an admin) and that it
// hasn't been cancelled.
func canEditCover(ctx context.Context, tid, uid string, admin bool) error {
	var host, status string
	err := db.Pool.QueryRow(ctx, "SELECT COALESCE(host_id,''), status FROM tournaments WHERE id = $1", tid).Scan(&host, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return userErr(404, "Tournament not found")
	}
	if err != nil {
		return err
	}
	if !admin && (host == "" || host != uid) {
		return userErr(403, "Only the host can change this cup's picture")
	}
	if status == "cancelled" {
		return userErr(409, "This cup was cancelled")
	}
	return nil
}

// SaveCover stores a processed cover and points the cup at it. The URL
// carries a version so browsers show the new picture straight away.
func SaveCover(ctx context.Context, tid, uid string, admin bool, jpg []byte) (string, error) {
	if err := canEditCover(ctx, tid, uid, admin); err != nil {
		return "", err
	}
	url := fmt.Sprintf("/api/tournament-covers/%s?v=%d", tid, time.Now().Unix())
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO tournament_covers (tournament_id, image, updated_at) VALUES ($1,$2,NOW())
		ON CONFLICT (tournament_id) DO UPDATE SET image = $2, updated_at = NOW()`, tid, jpg); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, "UPDATE tournaments SET cover_url = $2 WHERE id = $1", tid, url); err != nil {
		return "", err
	}
	return url, tx.Commit(ctx)
}

// RemoveCover deletes a cup's cover (the host, or an admin removing a bad picture).
func RemoveCover(ctx context.Context, tid, uid string, admin bool) error {
	if err := canEditCover(ctx, tid, uid, admin); err != nil && !admin {
		return err
	}
	if _, err := db.Pool.Exec(ctx, "DELETE FROM tournament_covers WHERE tournament_id = $1", tid); err != nil {
		return err
	}
	_, err := db.Pool.Exec(ctx, "UPDATE tournaments SET cover_url = NULL WHERE id = $1", tid)
	return err
}

// LoadCover returns a cup's cover, or nil if it has none.
func LoadCover(ctx context.Context, tid string) ([]byte, time.Time, error) {
	var img []byte
	var updated time.Time
	err := db.Pool.QueryRow(ctx, "SELECT image, updated_at FROM tournament_covers WHERE tournament_id = $1", tid).Scan(&img, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, time.Time{}, nil
	}
	return img, updated, err
}
