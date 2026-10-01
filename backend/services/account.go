package services

import (
	"context"
	"strings"
	"time"

	"betelite-go/db"
)

// TermsVersion is the current Terms/Privacy version users must accept.
// Bump it when the legal documents change materially.
const TermsVersion = "2026-10-01"

// MinAge is the minimum age to use real-money features.
const MinAge = 18

// DBReady reports whether a database connection is configured.
func DBReady() bool { return db.Pool != nil }

// Profile is the signed-in user's account as returned by /api/me.
type Profile struct {
	ID               string        `json:"id"`
	Email            string        `json:"email"`
	Username         string        `json:"username"`
	AvatarURL        string        `json:"avatarUrl"`
	Balance          int64         `json:"balance"`
	Country          string        `json:"country"`
	Currency         string        `json:"currency"`
	ReferralCode     string        `json:"referralCode"`
	PushEnabled      bool          `json:"pushEnabled"`
	EmailEnabled     bool          `json:"emailEnabled"`
	MarketingConsent bool          `json:"marketingConsent"`
	TermsVersion     string        `json:"termsVersion"`
	TermsCurrent     bool          `json:"termsCurrent"`
	AgeVerified      bool          `json:"ageVerified"`
	IsAdmin          bool          `json:"isAdmin"`
	EmailVerified    bool          `json:"emailVerified"`
	AdminPending     bool          `json:"adminPending"` // admin email, not yet verified
	GameProfiles     []GameProfile `json:"gameProfiles"`
	CreatedAt        time.Time     `json:"createdAt"`
}

// GameProfile is a player's saved name/ID for one game.
type GameProfile struct {
	Game     string `json:"game"`
	Gamertag string `json:"gamertag"`
	InGameID string `json:"inGameId"`
	Locked   bool   `json:"locked"`
}

// LoadProfile reads the full account for /api/me.
func LoadProfile(ctx context.Context, uid string) (*Profile, error) {
	var p Profile
	var birth *time.Time
	err := db.Pool.QueryRow(ctx, `SELECT id, email, username, COALESCE(avatar_url,''), balance, COALESCE(country,'NG'), COALESCE(currency,'NGN'),
		COALESCE(referral_code,''), COALESCE(push_notifications,true), COALESCE(email_notifications,true), marketing_consent,
		COALESCE(terms_version,''), birth_date, is_admin, created_at FROM users WHERE id = $1`, uid).
		Scan(&p.ID, &p.Email, &p.Username, &p.AvatarURL, &p.Balance, &p.Country, &p.Currency, &p.ReferralCode,
			&p.PushEnabled, &p.EmailEnabled, &p.MarketingConsent, &p.TermsVersion, &birth, &p.IsAdmin, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	p.TermsCurrent = p.TermsVersion == TermsVersion
	p.AgeVerified = birth != nil && age(*birth) >= MinAge
	if strings.HasSuffix(p.Email, ".invalid") {
		p.Email = ""
	}
	p.GameProfiles, err = ListGameProfiles(ctx, uid)
	return &p, err
}

// ListGameProfiles returns all saved game names for a user.
func ListGameProfiles(ctx context.Context, uid string) ([]GameProfile, error) {
	rows, err := db.Pool.Query(ctx, "SELECT game, gamertag, COALESCE(ingame_id,''), locked FROM game_profiles WHERE user_id = $1 ORDER BY game", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GameProfile{}
	for rows.Next() {
		var g GameProfile
		if err := rows.Scan(&g.Game, &g.Gamertag, &g.InGameID, &g.Locked); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func age(birth time.Time) int {
	now := time.Now()
	years := now.Year() - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		years--
	}
	return years
}

// RequireMoneyAccess blocks real-money actions until the user has accepted the
// current terms and confirmed they are 18+.
func RequireMoneyAccess(ctx context.Context, uid string) error {
	var terms string
	var birth *time.Time
	db.Pool.QueryRow(ctx, "SELECT COALESCE(terms_version,''), birth_date FROM users WHERE id = $1", uid).Scan(&terms, &birth)
	if terms != TermsVersion {
		return userErr(403, "Please accept the latest Terms and Privacy Policy to continue")
	}
	if birth == nil || age(*birth) < MinAge {
		return userErr(403, "You must be 18 or older to play for money")
	}
	return nil
}

// AcceptTerms records consent to the current terms and the user's birth date.
func AcceptTerms(ctx context.Context, uid string, birth time.Time, marketing bool) error {
	if age(birth) < MinAge {
		return userErr(403, "You must be 18 or older to use CrestArena")
	}
	if birth.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return userErr(400, "Please enter a valid date of birth")
	}
	_, err := db.Pool.Exec(ctx, `UPDATE users SET terms_version = $2, terms_accepted_at = NOW(), birth_date = $3,
		marketing_consent = $4, updated_at = NOW() WHERE id = $1`, uid, TermsVersion, birth, marketing)
	return err
}

// ExportUserData collects everything stored about a user (data portability).
func ExportUserData(ctx context.Context, uid string) (map[string]any, error) {
	out := map[string]any{"exportedAt": time.Now().UTC()}
	p, err := LoadProfile(ctx, uid)
	if err != nil {
		return nil, err
	}
	out["account"] = p
	queries := map[string]string{
		"transactions":  "SELECT type, amount, ref_id, created_at FROM transactions WHERE user_id = $1 ORDER BY created_at",
		"deposits":      "SELECT reference, amount, currency, status, created_at FROM deposits WHERE user_id = $1 ORDER BY created_at",
		"withdrawals":   "SELECT id, amount, currency, bank_name, account_number, status, created_at FROM withdrawals WHERE user_id = $1 ORDER BY created_at",
		"matches":       "SELECT id, kind, game, home_id, away_id, status, score_home, score_away, created_at FROM matches WHERE home_id = $1 OR away_id = $1 ORDER BY created_at",
		"notifications": "SELECT type, title, message, read, created_at FROM notifications WHERE user_id = $1 ORDER BY created_at",
	}
	for key, q := range queries {
		rows, err := db.Pool.Query(ctx, q, uid)
		if err != nil {
			return nil, err
		}
		list := []map[string]any{}
		cols := rows.FieldDescriptions()
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				rows.Close()
				return nil, err
			}
			row := map[string]any{}
			for i, c := range cols {
				row[c.Name] = vals[i]
			}
			list = append(list, row)
		}
		rows.Close()
		out[key] = list
	}
	return out, nil
}

// DeleteAccount anonymises a user. Financial records are kept (anonymised) for
// the period required by law; personal data is removed. It refuses while money
// is in the wallet or matches are unfinished, so nobody loses funds.
func DeleteAccount(ctx context.Context, uid string) error {
	var balance int64
	db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", uid).Scan(&balance)
	if balance > 0 {
		return userErr(409, "Withdraw your remaining balance before deleting your account")
	}
	var open int
	db.Pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM matches WHERE (home_id = $1 OR away_id = $1) AND status NOT IN ('confirmed','void')) +
		(SELECT COUNT(*) FROM escrow WHERE creator_id = $1 AND status = 'waiting') +
		(SELECT COUNT(*) FROM withdrawals WHERE user_id = $1 AND status IN ('pending','processing'))`, uid).Scan(&open)
	if open > 0 {
		return userErr(409, "Finish your open matches, challenges and withdrawals before deleting your account")
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	stmts := []string{
		`UPDATE users SET email = 'deleted-' || id || '@deleted.invalid', username = 'Deleted user', avatar_url = NULL,
			push_sub = NULL, birth_date = NULL, referral_code = NULL, marketing_consent = FALSE, deleted_at = NOW() WHERE id = $1`,
		"DELETE FROM game_profiles WHERE user_id = $1",
		"DELETE FROM push_subscriptions WHERE user_id = $1",
		"DELETE FROM notifications WHERE user_id = $1",
		"UPDATE withdrawals SET account_number = '****' || RIGHT(account_number, 4), account_name = NULL WHERE user_id = $1",
	}
	for _, s := range stmts {
		if _, err := tx.Exec(ctx, s, uid); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	ForgetUser(uid)
	return nil
}
