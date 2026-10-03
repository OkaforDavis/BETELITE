package services

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/image/draw"

	"betelite-go/db"
)

// Screenshot evidence is kept so players and admins can see the real image
// behind a result or a dispute. It is deleted 30 days after the match is final.
const (
	EvidenceRetention = 30 * 24 * time.Hour
	evidenceMaxSide   = 1920
)

// Screenshot is evidence metadata (the image is fetched separately).
type Screenshot struct {
	ID         int64     `json:"id"`
	Kind       string    `json:"kind"` // result | dispute
	UploadedBy string    `json:"uploadedBy"`
	Uploader   string    `json:"uploader"`
	CreatedAt  time.Time `json:"createdAt"`
}

// evidenceJPEG re-encodes an upload as a JPEG no wider than 1920px. Like
// profile photos, this drops metadata such as GPS location.
func evidenceJPEG(data []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, userErr(400, "That file isn't a readable image")
	}
	b := src.Bounds()
	if longest := max(b.Dx(), b.Dy()); longest > evidenceMaxSide {
		scale := float64(evidenceMaxSide) / float64(longest)
		dst := image.NewRGBA(image.Rect(0, 0, int(float64(b.Dx())*scale), int(float64(b.Dy())*scale)))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
		src = dst
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, src, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// SaveEvidence stores a screenshot for a match.
func SaveEvidence(ctx context.Context, matchID, uid, kind string, data []byte) error {
	jpg, err := evidenceJPEG(data)
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx, "INSERT INTO match_screenshots (match_id, uploaded_by, kind, image) VALUES ($1,$2,$3,$4)", matchID, uid, kind, jpg)
	return err
}

// ListEvidence returns the screenshots for a match, oldest first.
func ListEvidence(ctx context.Context, matchID string) ([]Screenshot, error) {
	rows, err := db.Pool.Query(ctx, `SELECT s.id, s.kind, s.uploaded_by, COALESCE(u.username,''), s.created_at
		FROM match_screenshots s LEFT JOIN users u ON u.id = s.uploaded_by WHERE s.match_id = $1 ORDER BY s.id`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Screenshot{}
	for rows.Next() {
		var s Screenshot
		if err := rows.Scan(&s.ID, &s.Kind, &s.UploadedBy, &s.Uploader, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LoadEvidence returns one screenshot's image and the match it belongs to.
func LoadEvidence(ctx context.Context, id int64) (img []byte, ctype, matchID string, err error) {
	err = db.Pool.QueryRow(ctx, "SELECT image, content_type, match_id FROM match_screenshots WHERE id = $1", id).Scan(&img, &ctype, &matchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", "", userErr(404, "Screenshot not found (screenshots are deleted after 30 days)")
	}
	return img, ctype, matchID, err
}

// CanViewEvidence: the two players and admins may view a match's screenshots.
func CanViewEvidence(ctx context.Context, matchID, uid string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	var ok bool
	db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM matches WHERE id = $1 AND (home_id = $2 OR away_id = $2))", matchID, uid).Scan(&ok)
	return ok
}

// purgeOldEvidence deletes screenshots 30 days after their match became final.
// Evidence for matches still open or under review is kept.
func purgeOldEvidence(ctx context.Context) {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM match_screenshots s USING matches m
		WHERE s.match_id = m.id AND m.status IN ('confirmed','void')
		  AND COALESCE(m.settled_at, s.created_at) < NOW() - make_interval(secs => $1)`, EvidenceRetention.Seconds())
	if err != nil {
		log.Printf("[EVIDENCE] purge: %v", err)
		return
	}
	if n := tag.RowsAffected(); n > 0 {
		log.Printf("[EVIDENCE] deleted %d screenshots older than 30 days", n)
	}
}
