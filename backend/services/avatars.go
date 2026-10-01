package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP decoder

	"betelite-go/db"
)

const (
	AvatarMaxUpload = 5 << 20 // bytes accepted from the client
	avatarSize      = 256     // stored as a 256x256 square
)

// ProcessAvatar decodes an uploaded photo, crops it to a centred square and
// re-encodes it as a 256px JPEG. Re-encoding drops all metadata (EXIF GPS
// location, camera details) and anything that isn't a real image.
func ProcessAvatar(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > AvatarMaxUpload {
		return nil, userErr(400, "Choose an image under 5 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, userErr(400, "Use a JPG, PNG or WebP photo")
	}
	if cfg.Width*cfg.Height > 40_000_000 {
		return nil, userErr(400, "That image is too large")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, userErr(400, "We couldn't read that image")
	}

	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2, 0, 0)
	crop.Max = crop.Min.Add(image.Pt(side, side))

	dst := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Src, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// SaveAvatar stores a processed photo and points the profile at it. The URL
// carries a version so browsers fetch the new photo immediately.
func SaveAvatar(ctx context.Context, uid string, jpg []byte) (string, error) {
	url := fmt.Sprintf("/api/avatars/%s?v=%d", uid, time.Now().Unix())
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO avatars (user_id, image, content_type, updated_at) VALUES ($1,$2,'image/jpeg',NOW())
		ON CONFLICT (user_id) DO UPDATE SET image = $2, content_type = 'image/jpeg', updated_at = NOW()`, uid, jpg); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, "UPDATE users SET avatar_url = $2, updated_at = NOW() WHERE id = $1", uid, url); err != nil {
		return "", err
	}
	return url, tx.Commit(ctx)
}

// RemoveAvatar deletes the user's photo.
func RemoveAvatar(ctx context.Context, uid string) error {
	if _, err := db.Pool.Exec(ctx, "DELETE FROM avatars WHERE user_id = $1", uid); err != nil {
		return err
	}
	_, err := db.Pool.Exec(ctx, "UPDATE users SET avatar_url = NULL, updated_at = NOW() WHERE id = $1", uid)
	return err
}

// LoadAvatar returns a stored photo, or nil if the user has none.
func LoadAvatar(ctx context.Context, uid string) ([]byte, string, time.Time, error) {
	var img []byte
	var ctype string
	var updated time.Time
	err := db.Pool.QueryRow(ctx, "SELECT image, content_type, updated_at FROM avatars WHERE user_id = $1", uid).Scan(&img, &ctype, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", time.Time{}, nil
	}
	return img, ctype, updated, err
}
