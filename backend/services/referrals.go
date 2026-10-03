package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
)

// Referral rewards in minor units, paid once after the invited friend's first
// paid match (1v1, or a tournament with an entry fee).
var (
	ReferralInviterReward = map[string]int64{"NGN": 200_00, "GHS": 2_00}
	ReferralFriendReward  = map[string]int64{"NGN": 100_00, "GHS": 1_00}
)

// ReferralClaimWindow: a code can only be added this soon after sign-up.
const ReferralClaimWindow = 7 * 24 * time.Hour

// ReferralInfo is what /api/referrals returns.
type ReferralInfo struct {
	Code        string `json:"code"`
	Joined      int    `json:"referrals"` // friends who used your code
	Rewarded    int    `json:"rewarded"`  // of those, how many earned you a reward
	Earned      int64  `json:"earned"`
	Currency    string `json:"currency"`
	InviterGets int64  `json:"inviterReward"`
	FriendGets  int64  `json:"friendReward"`
	ReferredBy  string `json:"referredBy,omitempty"` // name of who invited you
	CanClaim    bool   `json:"canClaim"`             // may still add a code
}

func referralCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I to avoid typos
	b := make([]byte, 8)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// GetReferralInfo returns (and creates on first use) the user's invite code
// and referral stats.
func GetReferralInfo(ctx context.Context, uid string) (*ReferralInfo, error) {
	info := &ReferralInfo{}
	var referredBy *string
	var createdAt time.Time
	err := db.Pool.QueryRow(ctx, "SELECT COALESCE(referral_code,''), referred_by, COALESCE(currency,'NGN'), created_at FROM users WHERE id = $1", uid).
		Scan(&info.Code, &referredBy, &info.Currency, &createdAt)
	if err != nil {
		return nil, err
	}
	for attempt := 0; info.Code == "" && attempt < 5; attempt++ {
		_, err := db.Pool.Exec(ctx, "UPDATE users SET referral_code = $2 WHERE id = $1 AND referral_code IS NULL", uid, referralCode())
		if err != nil && !strings.Contains(err.Error(), "23505") { // retry on the rare duplicate code
			return nil, err
		}
		// Re-read: covers our write and a concurrent request that set it first.
		db.Pool.QueryRow(ctx, "SELECT COALESCE(referral_code,'') FROM users WHERE id = $1", uid).Scan(&info.Code)
	}
	db.Pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(referral_rewarded_at) FROM users WHERE referred_by = $1`, uid).Scan(&info.Joined, &info.Rewarded)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0) FROM transactions WHERE user_id = $1 AND type = 'referral'`, uid).Scan(&info.Earned)
	info.InviterGets = ReferralInviterReward[info.Currency]
	info.FriendGets = ReferralFriendReward[info.Currency]
	if referredBy != nil {
		info.ReferredBy = Username(ctx, *referredBy)
	} else {
		info.CanClaim = time.Since(createdAt) < ReferralClaimWindow && !hasPaidMatch(ctx, uid)
	}
	return info, nil
}

func hasPaidMatch(ctx context.Context, uid string) bool {
	var played bool
	db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM matches m LEFT JOIN tournaments t ON t.id = m.tournament_id
		WHERE (m.home_id = $1 OR m.away_id = $1) AND m.status = 'confirmed'
		  AND (m.kind = 'p2p' OR COALESCE(t.entry_fee,0) > 0))`, uid).Scan(&played)
	return played
}

// ClaimReferral links a new user to the friend who invited them. No money
// moves yet: rewards are paid after the new user's first paid match.
func ClaimReferral(ctx context.Context, uid, code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return "", userErr(400, "Enter an invite code")
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var referredBy *string
	var createdAt time.Time
	if err := tx.QueryRow(ctx, "SELECT referred_by, created_at FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&referredBy, &createdAt); err != nil {
		return "", err
	}
	if referredBy != nil {
		return "", userErr(409, "You've already used an invite code")
	}
	if time.Since(createdAt) > ReferralClaimWindow {
		return "", userErr(409, "Invite codes can only be added in your first 7 days")
	}
	if hasPaidMatch(ctx, uid) {
		return "", userErr(409, "Invite codes must be added before your first paid match")
	}
	var inviter string
	err = tx.QueryRow(ctx, "SELECT id FROM users WHERE referral_code = $1 AND deleted_at IS NULL", code).Scan(&inviter)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", userErr(404, "That invite code doesn't exist. Check it and try again.")
	}
	if err != nil {
		return "", err
	}
	if inviter == uid {
		return "", userErr(400, "You can't use your own invite code")
	}
	if _, err := tx.Exec(ctx, "UPDATE users SET referred_by = $2 WHERE id = $1", uid, inviter); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	name := Username(ctx, inviter)
	Notify(inviter, "referral_joined", "A friend joined with your code",
		Username(ctx, uid)+" signed up with your invite. You'll get your reward after their first paid match.", "/profile", nil)
	return name, nil
}

// RewardReferral pays the inviter and the friend once, the first time the
// friend finishes a paid match. Safe to call after every settled match.
func RewardReferral(ctx context.Context, uid string) {
	if db.Pool == nil || !hasPaidMatch(ctx, uid) {
		return
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	var inviter, currency string
	err = tx.QueryRow(ctx, `UPDATE users SET referral_rewarded_at = NOW()
		WHERE id = $1 AND referred_by IS NOT NULL AND referral_rewarded_at IS NULL
		RETURNING referred_by, COALESCE(currency,'NGN')`, uid).Scan(&inviter, &currency)
	if err != nil {
		return // not referred, or already rewarded
	}
	var inviterCurrency string
	tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1 AND deleted_at IS NULL", inviter).Scan(&inviterCurrency)

	ref := "ref_" + uid
	if inviterCurrency != "" {
		if err := AdjustBalance(ctx, tx, inviter, ReferralInviterReward[inviterCurrency], "referral", ref); err != nil {
			log.Printf("[REFERRAL] reward inviter %s: %v", inviter, err)
			return
		}
	}
	if err := AdjustBalance(ctx, tx, uid, ReferralFriendReward[currency], "referral_bonus", ref); err != nil {
		log.Printf("[REFERRAL] reward friend %s: %v", uid, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		return
	}
	friend := Username(ctx, uid)
	if inviterCurrency != "" {
		Notify(inviter, "referral_reward", "Invite reward earned 🎉",
			fmt.Sprintf("%s played their first paid match. %s has been added to your wallet.", friend, FormatMoney(ReferralInviterReward[inviterCurrency], inviterCurrency)), "/wallet", nil)
	}
	Notify(uid, "referral_reward", "Welcome bonus 🎉",
		fmt.Sprintf("You played your first paid match. Your %s invite bonus has been added to your wallet.", FormatMoney(ReferralFriendReward[currency], currency)), "/wallet", nil)
}
