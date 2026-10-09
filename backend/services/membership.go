package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
	"betelite-go/utils"
)

// Tier is a membership level. Every tier plays the same tournaments for the
// same fixed prizes; higher tiers get more tournament entries a month and perks.
// Prizes never depend on membership money (that would make it a betting pool).
type Tier struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Entries int              `json:"entries"` // tournaments per period
	Prices  map[string]int64 `json:"prices"`  // minor units per currency; empty = free
	Perks   []string         `json:"perks"`
}

const FreeTier = "free"

// MembershipPeriod is how long one paid plan lasts.
const MembershipPeriod = 30 * 24 * time.Hour

// MembershipRenewWindow: a plan can be renewed this close to its end.
const MembershipRenewWindow = 7 * 24 * time.Hour

var Tiers = []Tier{
	{ID: FreeTier, Name: "Free", Entries: 1, Perks: []string{
		"1 tournament a month", "Free 1v1 matches", "Same prizes as everyone"}},
	{ID: "pro", Name: "Pro", Entries: 2, Prices: map[string]int64{"GHS": 30_00, "NGN": 3_000_00}, Perks: []string{
		"2 tournaments a month", "Pro badge", "Early tournament sign-up", "Streaming features when they launch"}},
	{ID: "pro_plus", Name: "Pro Plus", Entries: 3, Prices: map[string]int64{"GHS": 50_00, "NGN": 5_000_00}, Perks: []string{
		"3 tournaments a month", "Pro Plus badge", "Early tournament sign-up", "Streaming features when they launch"}},
	{ID: "elite", Name: "Elite", Entries: 5, Prices: map[string]int64{"GHS": 80_00, "NGN": 8_000_00}, Perks: []string{
		"5 tournaments a month", "Elite badge and profile frame", "Early tournament sign-up", "Streaming features when they launch"}},
}

// TierByID returns a tier, or nil.
func TierByID(id string) *Tier {
	for i := range Tiers {
		if Tiers[i].ID == id {
			return &Tiers[i]
		}
	}
	return nil
}

// Membership is a paid plan.
type Membership struct {
	ID       string    `json:"id"`
	Tier     string    `json:"tier"`
	Price    int64     `json:"price"`
	Currency string    `json:"currency"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ActiveMembership returns the plan covering now, or nil.
func ActiveMembership(ctx context.Context, q querier, uid string) (*Membership, error) {
	var m Membership
	err := q.QueryRow(ctx, `SELECT id, tier, price, currency, starts_at, ends_at FROM memberships
		WHERE user_id = $1 AND starts_at <= NOW() AND ends_at > NOW() ORDER BY ends_at DESC LIMIT 1`, uid).
		Scan(&m.ID, &m.Tier, &m.Price, &m.Currency, &m.StartsAt, &m.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// latestMembership returns the plan that ends last (it may start in the future after a renewal).
func latestMembership(ctx context.Context, q querier, uid string) (*Membership, error) {
	var m Membership
	err := q.QueryRow(ctx, `SELECT id, tier, price, currency, starts_at, ends_at FROM memberships
		WHERE user_id = $1 AND ends_at > NOW() ORDER BY ends_at DESC LIMIT 1`, uid).
		Scan(&m.ID, &m.Tier, &m.Price, &m.Currency, &m.StartsAt, &m.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &m, err
}

// ActiveTier is the tier ID for badges ("free" when there is no plan).
func ActiveTier(ctx context.Context, uid string) string {
	if db.Pool == nil {
		return FreeTier
	}
	m, err := ActiveMembership(ctx, db.Pool, uid)
	if err != nil || m == nil {
		return FreeTier
	}
	return m.Tier
}

// Usage is a player's tournament allowance for the current period.
type Usage struct {
	Tier      string      `json:"tier"`
	Allowed   int         `json:"allowed"`
	Used      int         `json:"used"`
	Left      int         `json:"left"`
	ResetsAt  time.Time   `json:"resetsAt"`
	Plan      *Membership `json:"plan,omitempty"`
	RenewedTo *time.Time  `json:"renewedTo,omitempty"` // a renewal already bought
}

// EntryUsage counts tournament entries in the current period: the paid plan's
// 30 days, or the calendar month for free players. Cancelled tournaments and
// tournaments left before they started don't count.
func EntryUsage(ctx context.Context, q querier, uid string) (*Usage, error) {
	m, err := ActiveMembership(ctx, q, uid)
	if err != nil {
		return nil, err
	}
	u := &Usage{Tier: FreeTier}
	var from time.Time
	if m != nil {
		u.Tier, u.Plan, from, u.ResetsAt = m.Tier, m, m.StartsAt, m.EndsAt
		if latest, err := latestMembership(ctx, q, uid); err == nil && latest != nil && latest.EndsAt.After(m.EndsAt) {
			u.RenewedTo = &latest.EndsAt
		}
	} else {
		now := time.Now().UTC()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		u.ResetsAt = from.AddDate(0, 1, 0)
	}
	if t := TierByID(u.Tier); t != nil {
		u.Allowed = t.Entries
	}
	if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM tournament_players tp JOIN tournaments t ON t.id = tp.tournament_id
		WHERE tp.user_id = $1 AND tp.joined_at >= $2 AND t.status <> 'cancelled' AND t.entry_fee = 0`, uid, from).Scan(&u.Used); err != nil {
		return nil, err
	}
	u.Left = max(u.Allowed-u.Used, 0)
	return u, nil
}

// BuyMembership pays for a plan from the wallet. Buying the same tier near the
// end of a plan renews it from the current end date; switching tier waits
// until the current plan ends.
func BuyMembership(ctx context.Context, uid, tierID string) (*Membership, error) {
	tier := TierByID(tierID)
	if tier == nil || len(tier.Prices) == 0 {
		return nil, userErr(400, "Choose a membership plan")
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currency string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&currency); err != nil {
		return nil, err
	}
	price, ok := tier.Prices[currency]
	if !ok {
		return nil, userErr(400, "This plan isn't available in %s", currency)
	}
	start := time.Now()
	current, err := latestMembership(ctx, tx, uid)
	if err != nil {
		return nil, err
	}
	if err := checkPlanChange(current, tier); err != nil {
		return nil, err
	}
	if current != nil {
		start = current.EndsAt
	}

	m := &Membership{ID: "mem_" + utils.GenerateBaseID(), Tier: tier.ID, Price: price, Currency: currency,
		StartsAt: start, EndsAt: start.Add(MembershipPeriod)}
	if err := AdjustBalance(ctx, tx, uid, -price, "membership", m.ID, fmt.Sprintf(`{"tier": %q}`, tier.ID)); err != nil {
		if errors.Is(err, ErrInsufficientFunds) {
			return nil, userErr(402, "Not enough balance for %s", tier.Name)
		}
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO memberships (id, user_id, tier, price, currency, starts_at, ends_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, m.ID, uid, m.Tier, m.Price, m.Currency, m.StartsAt, m.EndsAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	msg := fmt.Sprintf("You can now enter %d tournaments until %s.", tier.Entries, m.EndsAt.Format("2 Jan"))
	if current != nil {
		msg = fmt.Sprintf("Your %s plan is renewed until %s.", tier.Name, m.EndsAt.Format("2 Jan"))
	}
	Notify(uid, "membership", tier.Name+" membership active ⭐", msg, "/membership", nil)
	go RewardReferral(context.Background(), uid) // a first membership unlocks invite rewards
	return m, nil
}

// RemindExpiringMemberships tells members 3 days before their plan ends
// (unless they have already renewed). Runs from the automation loop.
func RemindExpiringMemberships(ctx context.Context) {
	rows, err := db.Pool.Query(ctx, `UPDATE memberships m SET reminded = TRUE
		WHERE NOT m.reminded AND m.ends_at BETWEEN NOW() AND NOW() + INTERVAL '3 days'
		  AND NOT EXISTS (SELECT 1 FROM memberships n WHERE n.user_id = m.user_id AND n.ends_at > m.ends_at)
		RETURNING m.user_id, m.tier, m.ends_at`)
	if err != nil {
		return
	}
	type due struct {
		uid, tier string
		end       time.Time
	}
	var list []due
	for rows.Next() {
		var d due
		if rows.Scan(&d.uid, &d.tier, &d.end) == nil {
			list = append(list, d)
		}
	}
	rows.Close()
	for _, d := range list {
		name := "Your"
		if t := TierByID(d.tier); t != nil {
			name = "Your " + t.Name
		}
		Notify(d.uid, "membership", "Membership ending soon", name+" plan ends on "+d.end.Format("2 Jan")+". Renew to keep your tournament entries.", "/membership", nil)
	}
}

// checkPlanChange allows a renewal of the same tier in the plan's last days,
// and a different tier only once the current plan has ended.
func checkPlanChange(current *Membership, tier *Tier) error {
	if current == nil {
		return nil
	}
	if current.Tier != tier.ID {
		name := current.Tier
		if t := TierByID(current.Tier); t != nil {
			name = t.Name
		}
		return userErr(409, "You're on %s until %s. You can change plan after it ends.", name, current.EndsAt.Format("2 Jan"))
	}
	if time.Until(current.EndsAt) > MembershipRenewWindow {
		return userErr(409, "Your %s plan runs until %s. You can renew in its last 7 days.", tier.Name, current.EndsAt.Format("2 Jan"))
	}
	return nil
}

// CanBuyMembership reports whether a plan can be bought now (before taking payment).
func CanBuyMembership(ctx context.Context, uid, tierID string) error {
	tier := TierByID(tierID)
	if tier == nil || len(tier.Prices) == 0 {
		return userErr(400, "Choose a membership plan")
	}
	current, err := latestMembership(ctx, db.Pool, uid)
	if err != nil {
		return err
	}
	return checkPlanChange(current, tier)
}
