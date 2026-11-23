package services

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/image/draw"

	"betelite-go/db"
)

// Cup sponsors: a host (often a business) can show a sponsor's name, a link,
// a few pictures and one short video on their cup. An admin approves it
// before anyone else sees it. It shows while the cup runs and for a day
// after it ends, then it is deleted.

const (
	SponsorMaxImages     = 4
	SponsorImageMaxLen   = 5 << 20 // upload limit; stored re-encoded and much smaller
	SponsorVideoMaxLen   = 5 << 20 // short, low-resolution clips only
	SponsorShowAfterEnd  = 24 * time.Hour
	sponsorImageMaxSide  = 1280
	sponsorImageQuality  = 72
	sponsorNameMaxLength = 40
)

// SponsorMedia is one picture or video.
type SponsorMedia struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"` // image | video
}

// Sponsor is what the cup page shows.
type Sponsor struct {
	Name   string         `json:"name"`
	Link   string         `json:"link,omitempty"`
	Status string         `json:"status"` // pending | approved | rejected
	Reason string         `json:"reason,omitempty"`
	Media  []SponsorMedia `json:"media"`
}

// SponsorUpload is a file the host sends.
type SponsorUpload struct {
	Data        []byte
	ContentType string
}

// cleanSponsorLink accepts an http(s) link and returns it normalised, or "".
func cleanSponsorLink(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !strings.Contains(u.Host, ".") || len(raw) > 200 {
		return "", userErr(400, "Enter a valid website or social link (e.g. instagram.com/yourbrand)")
	}
	return u.String(), nil
}

// sponsorJPEG shrinks a picture to at most 1280px and re-encodes it, which
// also drops metadata such as GPS location.
func sponsorJPEG(data []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, userErr(400, "One of the pictures isn't a readable image (use JPG or PNG)")
	}
	b := src.Bounds()
	if longest := max(b.Dx(), b.Dy()); longest > sponsorImageMaxSide {
		scale := float64(sponsorImageMaxSide) / float64(longest)
		dst := image.NewRGBA(image.Rect(0, 0, max(int(float64(b.Dx())*scale), 1), max(int(float64(b.Dy())*scale), 1)))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
		src = dst
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, src, &jpeg.Options{Quality: sponsorImageQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// videoType recognises MP4/MOV and WebM files by their first bytes.
func videoType(data []byte) string {
	switch {
	case len(data) > 12 && string(data[4:8]) == "ftyp":
		return "video/mp4"
	case len(data) > 4 && bytes.Equal(data[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "video/webm"
	}
	return ""
}

// SetSponsor adds or replaces the sponsor on a cup. A host's sponsor waits
// for an admin to approve it; an admin's (on any cup, including CrestArena's
// own cups like the End-of-Month Platter) shows straight away.
func SetSponsor(ctx context.Context, uid string, admin bool, tid, name, link string, images []SponsorUpload, video *SponsorUpload) error {
	name = strings.TrimSpace(name)
	if n := len([]rune(name)); n < 2 || n > sponsorNameMaxLength {
		return userErr(400, "Enter the sponsor's name (2 to %d characters)", sponsorNameMaxLength)
	}
	link, err := cleanSponsorLink(link)
	if err != nil {
		return err
	}
	if len(images) == 0 && video == nil {
		return userErr(400, "Add at least one picture or a short video")
	}
	if len(images) > SponsorMaxImages {
		return userErr(400, "You can add up to %d pictures", SponsorMaxImages)
	}
	var host, status string
	err = db.Pool.QueryRow(ctx, "SELECT COALESCE(host_id,''), status FROM tournaments WHERE id = $1", tid).Scan(&host, &status)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !admin && (host == "" || host != uid)) {
		return userErr(403, "Only the cup's host can add a sponsor")
	}
	if err != nil {
		return err
	}
	if status != "open" && status != "active" {
		return userErr(409, "Sponsors can only be added before the cup ends")
	}

	type item struct {
		kind, ctype string
		data        []byte
	}
	var items []item
	for _, im := range images {
		jpg, err := sponsorJPEG(im.Data)
		if err != nil {
			return err
		}
		items = append(items, item{"image", "image/jpeg", jpg})
	}
	if video != nil {
		if len(video.Data) > SponsorVideoMaxLen {
			return userErr(400, "The video is too large (max 5 MB). Use a short, low-quality clip.")
		}
		ct := videoType(video.Data)
		if ct == "" {
			return userErr(400, "The video must be an MP4 or WebM file")
		}
		items = append(items, item{"video", ct, video.Data})
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "DELETE FROM cup_sponsors WHERE tournament_id = $1", tid); err != nil {
		return err
	}
	status = "pending"
	var reviewer any
	if admin {
		status, reviewer = "approved", uid
	}
	if _, err := tx.Exec(ctx, `INSERT INTO cup_sponsors (tournament_id, name, link, status, reviewed_by, reviewed_at)
		VALUES ($1,$2,$3,$4,$5::text, CASE WHEN $5::text IS NULL THEN NULL ELSE NOW() END)`, tid, name, link, status, reviewer); err != nil {
		return err
	}
	for i, it := range items {
		if _, err := tx.Exec(ctx, "INSERT INTO cup_sponsor_media (tournament_id, kind, content_type, data, position) VALUES ($1,$2,$3,$4,$5)",
			tid, it.kind, it.ctype, it.data, i); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if !admin {
		NotifyAdmins(ctx, "Sponsor to review", name+" on cup "+tid+". Check it in Admin → Sponsors.")
	}
	return nil
}

// RemoveSponsor deletes a cup's sponsor (host or admin).
func RemoveSponsor(ctx context.Context, tid string) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM cup_sponsors WHERE tournament_id = $1", tid)
	return err
}

// GetSponsor returns a cup's sponsor. Players see it only once approved and
// while it's showing; the host and admins also see pending or rejected ones.
func GetSponsor(ctx context.Context, tid string, privileged bool) *Sponsor {
	s := &Sponsor{Media: []SponsorMedia{}}
	err := db.Pool.QueryRow(ctx, `SELECT s.name, s.link, s.status, s.reason FROM cup_sponsors s JOIN tournaments t ON t.id = s.tournament_id
		WHERE s.tournament_id = $1 AND (t.status IN ('open','active') OR (t.status = 'finished' AND t.finished_at > NOW() - make_interval(secs => $2)))`,
		tid, SponsorShowAfterEnd.Seconds()).Scan(&s.Name, &s.Link, &s.Status, &s.Reason)
	if err != nil || (!privileged && s.Status != "approved") {
		return nil
	}
	if !privileged {
		s.Reason = ""
	}
	rows, err := db.Pool.Query(ctx, "SELECT id, kind FROM cup_sponsor_media WHERE tournament_id = $1 ORDER BY position", tid)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var m SponsorMedia
		if rows.Scan(&m.ID, &m.Kind) == nil {
			s.Media = append(s.Media, m)
		}
	}
	return s
}

// LoadSponsorMedia returns one file. Unless privileged, only approved
// sponsors that are still showing are served.
func LoadSponsorMedia(ctx context.Context, id int64, privileged bool) (data []byte, ctype string, err error) {
	var status, tid string
	err = db.Pool.QueryRow(ctx, `SELECT m.data, m.content_type, s.status, m.tournament_id FROM cup_sponsor_media m
		JOIN cup_sponsors s ON s.tournament_id = m.tournament_id WHERE m.id = $1`, id).Scan(&data, &ctype, &status, &tid)
	if err != nil {
		return nil, "", userErr(404, "Not found")
	}
	if !privileged && (status != "approved" || GetSponsor(ctx, tid, false) == nil) {
		return nil, "", userErr(404, "Not found")
	}
	return data, ctype, nil
}

// ReviewSponsor approves or rejects a sponsor and tells the host.
func ReviewSponsor(ctx context.Context, tid, adminID string, approve bool, reason string) error {
	status := "rejected"
	if approve {
		status, reason = "approved", ""
	} else if len(strings.TrimSpace(reason)) < 3 {
		return userErr(400, "Give the host a short reason")
	}
	var name, host, cup string
	err := db.Pool.QueryRow(ctx, `UPDATE cup_sponsors s SET status = $2, reason = $3, reviewed_by = $4, reviewed_at = NOW()
		FROM tournaments t WHERE s.tournament_id = $1 AND t.id = s.tournament_id RETURNING s.name, COALESCE(t.host_id,''), t.name`,
		tid, status, strings.TrimSpace(reason), adminID).Scan(&name, &host, &cup)
	if err != nil {
		return userErr(404, "Sponsor not found")
	}
	if approve {
		Notify(host, "sponsor", "Sponsor approved ✅", name+" is now showing on "+cup+".", "/tournaments/"+tid, nil)
	} else {
		Notify(host, "sponsor", "Sponsor not approved", name+" can't be shown on "+cup+": "+reason, "/tournaments/"+tid, nil)
	}
	return nil
}

// PendingSponsor is a sponsor waiting for review.
type PendingSponsor struct {
	TournamentID string         `json:"tournamentId"`
	Cup          string         `json:"cup"`
	Host         string         `json:"host"`
	Name         string         `json:"name"`
	Link         string         `json:"link"`
	Status       string         `json:"status"`
	CreatedAt    time.Time      `json:"createdAt"`
	Media        []SponsorMedia `json:"media"`
}

// ListSponsors returns sponsors for admins: pending first, then live ones.
func ListSponsors(ctx context.Context) ([]PendingSponsor, error) {
	rows, err := db.Pool.Query(ctx, `SELECT s.tournament_id, t.name, COALESCE(u.username,''), s.name, s.link, s.status, s.created_at
		FROM cup_sponsors s JOIN tournaments t ON t.id = s.tournament_id LEFT JOIN users u ON u.id = t.host_id
		ORDER BY (s.status = 'pending') DESC, s.created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	out := []PendingSponsor{}
	for rows.Next() {
		var p PendingSponsor
		if err := rows.Scan(&p.TournamentID, &p.Cup, &p.Host, &p.Name, &p.Link, &p.Status, &p.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	rows.Close()
	for i := range out {
		out[i].Media = []SponsorMedia{}
		mr, err := db.Pool.Query(ctx, "SELECT id, kind FROM cup_sponsor_media WHERE tournament_id = $1 ORDER BY position", out[i].TournamentID)
		if err != nil {
			return nil, err
		}
		for mr.Next() {
			var m SponsorMedia
			if mr.Scan(&m.ID, &m.Kind) == nil {
				out[i].Media = append(out[i].Media, m)
			}
		}
		mr.Close()
	}
	return out, nil
}

// purgeSponsors deletes sponsors a day after their cup ends, and at once for
// cancelled cups.
func purgeSponsors(ctx context.Context) {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM cup_sponsors s USING tournaments t WHERE t.id = s.tournament_id
		AND (t.status = 'cancelled' OR (t.status = 'finished' AND COALESCE(t.finished_at, s.created_at) < NOW() - make_interval(secs => $1)))`,
		SponsorShowAfterEnd.Seconds())
	if err != nil {
		log.Printf("[SPONSOR] purge: %v", err)
		return
	}
	if n := tag.RowsAffected(); n > 0 {
		log.Printf("[SPONSOR] deleted %d ended sponsors", n)
	}
}
