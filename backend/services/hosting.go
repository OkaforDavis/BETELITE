package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"betelite-go/db"
	"betelite-go/utils"
)

// Player-hosted cups and bounty matches.
//
// Legal shape: in both, only the HOST puts money in. Players join a cup free
// and a challenger accepts a bounty free, so nobody pays for a chance to win
// a prize. The host pays the prize (held in escrow) plus a hosting fee, the
// platform's only income from these. A host who wins their own bounty simply
// gets their prize back; they can never gain money.

// HostCommissionPercent: when a player who came in through a host's cup buys
// their first membership within HostCommissionWindow, the host gets this
// share of the price, paid by CrestArena out of the membership money (never
// out of prize money, so nobody pays to win).
const HostCommissionPercent = 20

// HostCommissionWindow is how long after joining (or trying to join) a cup
// the player's first membership still earns the host a commission.
const HostCommissionWindow = 30 * 24 * time.Hour

// HostFeePercent is the hosting fee as a percentage of the prize (cup pool
// or bounty amount).
const HostFeePercent = 20

// CupFillWindow is how long an open host cup waits to fill before it is
// cancelled and the host refunded in full (pool and fee).
const CupFillWindow = 7 * 24 * time.Hour

// MaxOpenHostCups is how many unfilled cups one player may host at a time.
const MaxOpenHostCups = 3

// Pool and bounty limits per currency, in minor units.
var (
	MinCupPool = map[string]int64{"NGN": 2_000_00, "GHS": 20_00}
	MaxCupPool = map[string]int64{"NGN": 500_000_00, "GHS": 5_000_00}
	MinBounty  = map[string]int64{"NGN": 500_00, "GHS": 5_00}
	MaxBounty  = map[string]int64{"NGN": 100_000_00, "GHS": 2_000_00}
)

// HostFee is the fee for a prize of the given size.
func HostFee(prize int64) int64 {
	return int64(math.Ceil(float64(prize) * HostFeePercent / 100))
}

// SplitPool divides a pool by percentages; rounding leftovers go to 1st place
// so the prizes always add up to the pool exactly.
func SplitPool(pool int64, split []float64) []int64 {
	out := make([]int64, len(split))
	var sum int64
	for i, pct := range split {
		out[i] = int64(math.Floor(float64(pool) * pct / 100))
		sum += out[i]
	}
	if len(out) > 0 {
		out[0] += pool - sum
	}
	return out
}

// ValidateSplit checks a host's prize split: whole percentages that add up
// to 100, at most `places` paid positions, every place above zero.
func ValidateSplit(split []float64, places int) error {
	if len(split) == 0 || len(split) > places {
		return userErr(400, "Choose how the prize pool is shared (up to %d places)", places)
	}
	var total float64
	for _, p := range split {
		if p <= 0 || p != math.Floor(p) {
			return userErr(400, "Each prize share must be a whole percentage above zero")
		}
		total += p
	}
	if total != 100 {
		return userErr(400, "Prize shares must add up to 100%%")
	}
	return nil
}

// HostCupRequest is what a player sends to host a cup.
type HostCupRequest struct {
	Name       string    `json:"name"`
	Game       string    `json:"game"`
	Format     string    `json:"format"` // knockout | swiss | league
	MaxPlayers int       `json:"maxPlayers"`
	Pool       int64     `json:"pool"` // prize pool in the host's currency, minor units
	Split      []float64 `json:"split"`
	RoundHours int       `json:"roundHours"`
	// MembersOnly: only players with a paid plan can join.
	MembersOnly bool `json:"membersOnly"`
}

// FreeCupAvailable reports whether an Elite member still has this plan
// period's fee-free cup (one per 30-day plan, cancelled cups don't count).
func FreeCupAvailable(ctx context.Context, q querier, uid string) (bool, error) {
	m, err := ActiveMembership(ctx, q, uid)
	if err != nil || m == nil || m.Tier != "elite" {
		return false, err
	}
	var used bool
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tournaments WHERE host_id = $1 AND host_fee_waived AND status <> 'cancelled' AND created_at >= $2)`,
		uid, m.StartsAt).Scan(&used)
	return !used, err
}

// HostCup creates a player-hosted tournament. The pool and the hosting fee
// are taken from the host's wallet now; the pool is paid to the winners when
// the cup finishes, and everything is refunded if it doesn't fill in time.
func HostCup(ctx context.Context, uid string, req HostCupRequest) (string, error) {
	req.Name = strings.TrimSpace(req.Name)
	if n := len([]rune(req.Name)); n < 3 || n > 40 {
		return "", userErr(400, "Give your cup a name (3 to 40 characters)")
	}
	if GameByID(req.Game) == nil {
		return "", userErr(400, "Choose a supported game")
	}
	places := 2
	switch req.Format {
	case "knockout":
		if req.MaxPlayers != 4 && req.MaxPlayers != 8 && req.MaxPlayers != 16 && req.MaxPlayers != 32 && req.MaxPlayers != 64 {
			return "", userErr(400, "Knockout cups take 4, 8, 16, 32 or 64 players")
		}
	case "swiss":
		places = 3
		if req.MaxPlayers != 8 && req.MaxPlayers != 16 && req.MaxPlayers != 32 {
			return "", userErr(400, "Points cups take 8, 16 or 32 players")
		}
	case "league":
		places = 3
		if req.MaxPlayers < 4 || req.MaxPlayers > 8 {
			return "", userErr(400, "Leagues take 4 to 8 players")
		}
	default:
		return "", userErr(400, "Choose a format")
	}
	if err := ValidateSplit(req.Split, places); err != nil {
		return "", err
	}
	switch req.RoundHours {
	case 12, 24, 48:
	default:
		req.RoundHours = 24
	}
	if err := RequireMoneyAccess(ctx, uid); err != nil {
		return "", err
	}
	if err := RequireGameProfile(ctx, db.Pool, uid, req.Game); err != nil {
		return "", err
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var currency string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&currency); err != nil {
		return "", err
	}
	if req.Pool < MinCupPool[currency] || req.Pool > MaxCupPool[currency] {
		return "", userErr(400, "The prize pool must be between %s and %s", FormatMoney(MinCupPool[currency], currency), FormatMoney(MaxCupPool[currency], currency))
	}
	var open int
	tx.QueryRow(ctx, "SELECT COUNT(*) FROM tournaments WHERE host_id = $1 AND status = 'open'", uid).Scan(&open)
	if open >= MaxOpenHostCups {
		return "", userErr(400, "You can host up to %d cups at a time while they fill up", MaxOpenHostCups)
	}
	fee := HostFee(req.Pool)
	waived, err := FreeCupAvailable(ctx, tx, uid)
	if err != nil {
		return "", err
	}
	if waived {
		fee = 0
	}
	id := utils.GenerateTournamentID()
	if err := AdjustBalance(ctx, tx, uid, -req.Pool, "cup_host_hold", id); err != nil {
		if errors.Is(err, ErrInsufficientFunds) {
			return "", userErr(402, "You need %s in your wallet to host this cup (prize pool %s + hosting fee %s)", FormatMoney(req.Pool+fee, currency), FormatMoney(req.Pool, currency), FormatMoney(fee, currency))
		}
		return "", err
	}
	if fee > 0 {
		if err := AdjustBalance(ctx, tx, uid, -fee, "cup_host_fee", id); err != nil {
			if errors.Is(err, ErrInsufficientFunds) {
				return "", userErr(402, "You need %s in your wallet to host this cup (prize pool %s + hosting fee %s)", FormatMoney(req.Pool+fee, currency), FormatMoney(req.Pool, currency), FormatMoney(fee, currency))
			}
			return "", err
		}
	}
	prizes, _ := json.Marshal(map[string][]int64{currency: SplitPool(req.Pool, req.Split)})
	split, _ := json.Marshal(req.Split)
	_, err = tx.Exec(ctx, `INSERT INTO tournaments (id, name, game, mode, format, entry_fee, currency, max_players, prize_pool, prize_split, prizes, round_hours,
			created_by, status, early_tier, host_id, host_pool, host_fee, host_fee_waived, fill_deadline, members_only)
		VALUES ($1,$2,$3,$4,$4,0,$5,$6,0,$7::jsonb,$8::jsonb,$9,$10,'open','pro',$10,$11,$12,$13,$14,$15)`,
		id, req.Name, req.Game, req.Format, currency, req.MaxPlayers, string(split), string(prizes), req.RoundHours, uid, req.Pool, fee, waived, time.Now().Add(CupFillWindow), req.MembersOnly)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	log.Printf("[CUP] %s hosted by %s: %s %s players=%d pool=%d fee=%d waived=%v", id, uid, req.Format, req.Game, req.MaxPlayers, req.Pool, fee, waived)
	return id, nil
}

// RecordHostRef remembers the host whose cup brought a player in, so the host
// can earn a commission on the player's first membership. Only players who
// have never had a membership count, and the first host to bring them wins.
func RecordHostRef(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, uid, host string) {
	if host == "" || host == uid {
		return
	}
	if _, err := q.Exec(ctx, `UPDATE users SET host_ref = $2, host_ref_at = NOW()
		WHERE id = $1 AND host_ref IS NULL AND NOT EXISTS(SELECT 1 FROM memberships WHERE user_id = $1)`, uid, host); err != nil {
		log.Printf("[CUP] host ref %s -> %s: %v", uid, host, err)
	}
}

// RewardHostCommission pays the host their commission once, when a player
// they brought in buys a first membership in time. Safe to call after every
// membership purchase.
func RewardHostCommission(ctx context.Context, uid string) {
	if db.Pool == nil {
		return
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	var host string
	err = tx.QueryRow(ctx, `UPDATE users SET host_commission_at = NOW()
		WHERE id = $1 AND host_ref IS NOT NULL AND host_commission_at IS NULL AND host_ref_at > NOW() - make_interval(secs => $2)
		RETURNING host_ref`, uid, HostCommissionWindow.Seconds()).Scan(&host)
	if err != nil {
		return // no host, already paid, or too late
	}
	var n int
	var memID, currency string
	var price int64
	if err := tx.QueryRow(ctx, "SELECT COUNT(*), MIN(id), MIN(price), MIN(currency) FROM memberships WHERE user_id = $1", uid).Scan(&n, &memID, &price, &currency); err != nil || n != 1 {
		return // only the very first membership counts
	}
	var hostCurrency string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1 AND deleted_at IS NULL", host).Scan(&hostCurrency); err != nil {
		return
	}
	amount, _, err := ConvertOut(ctx, price*HostCommissionPercent/100, currency, hostCurrency)
	if err != nil || amount <= 0 {
		log.Printf("[CUP] commission %s -> %s: %v", uid, host, err)
		return
	}
	if err := AdjustBalance(ctx, tx, host, amount, "host_commission", memID, fmt.Sprintf(`{"player": %q}`, uid)); err != nil {
		log.Printf("[CUP] commission %s -> %s: %v", uid, host, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		return
	}
	Notify(host, "host_commission", "You earned a host commission 💰",
		fmt.Sprintf("%s joined through your cup and got a membership. %s has been added to your wallet.", Username(ctx, uid), FormatMoney(amount, hostCurrency)), "/wallet", nil)
}

// CancelTournament cancels an open tournament: entry fees (paid tournaments)
// and the host's pool and fee (host cups) go back, players are told.
func CancelTournament(ctx context.Context, tid, by, reason string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status, name, host string
	var fee, pool, hostFee int64
	if err := tx.QueryRow(ctx, "SELECT status, entry_fee, name, COALESCE(host_id,''), host_pool, host_fee FROM tournaments WHERE id = $1 FOR UPDATE", tid).
		Scan(&status, &fee, &name, &host, &pool, &hostFee); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return userErr(404, "Tournament not found")
		}
		return err
	}
	if status != "open" {
		return userErr(409, "Only tournaments that haven't started can be cancelled")
	}
	rows, err := tx.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1", tid)
	if err != nil {
		return err
	}
	var players []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			players = append(players, id)
		}
	}
	rows.Close()
	if fee > 0 {
		for _, p := range players {
			if err := AdjustBalance(ctx, tx, p, fee, "tournament_refund", tid); err != nil {
				return err
			}
		}
	}
	if host != "" && pool+hostFee > 0 {
		if err := AdjustBalance(ctx, tx, host, pool+hostFee, "cup_host_refund", tid, fmt.Sprintf(`{"pool": %d, "fee": %d}`, pool, hostFee)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, "DELETE FROM tournament_players WHERE tournament_id = $1", tid); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE tournaments SET status = 'cancelled' WHERE id = $1", tid); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, p := range players {
		msg := name + " was cancelled. " + reason
		if fee > 0 {
			msg += " Your entry fee has been refunded."
		}
		Notify(p, "tournament_cancelled", "Tournament cancelled", msg, "/tournaments", nil)
	}
	if host != "" && by != host {
		Notify(host, "tournament_cancelled", "Your cup was cancelled", name+" was cancelled. "+reason+" The prize pool and hosting fee are back in your wallet.", "/wallet", nil)
	}
	log.Printf("[CUP] %s cancelled by %s: %s", tid, by, reason)
	return nil
}

// ExpireUnfilledCups cancels host cups that didn't fill in time (automation loop).
func ExpireUnfilledCups(ctx context.Context) {
	for _, id := range idsFrom(ctx, "SELECT id FROM tournaments WHERE status = 'open' AND host_id IS NOT NULL AND fill_deadline < NOW() LIMIT 50") {
		if err := CancelTournament(ctx, id, "auto", "Not enough players joined within 7 days."); err != nil {
			log.Printf("[CUP] expire %s: %v", id, err)
		}
	}
}

// ConvertPrize converts a prize held in one currency into a winner's wallet
// currency at the market mid-rate (no margin). Cups and bounties are funded
// in the host's currency, but players from Ghana and Nigeria play together.
func ConvertPrize(ctx context.Context, amount int64, from, to string) (int64, error) {
	if from == to {
		return amount, nil
	}
	r, err := midRate(ctx, from, to)
	if err != nil {
		return 0, err
	}
	return int64(math.Floor(float64(amount)*r + 1e-6)), nil
}
