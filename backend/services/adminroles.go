package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
)

// Admin powers. The owner (ADMIN_EMAIL, verified) has all of them plus
// PermAdmins (add/remove admins), which can't be given to anyone else.
const (
	PermCups        = "cups"        // create/edit/cancel cups, rooms, Free-for-All results, sponsors
	PermDisputes    = "disputes"    // disputed matches, screenshots, chats, set score / cancel
	PermWithdrawals = "withdrawals" // approve / reject / mark paid, up to the admin limit
	PermSupport     = "support"     // player complaints
	PermUsers       = "users"       // see players, remove photos
	PermMoney       = "money"       // revenue, wallets, refunds, currency changes, any withdrawal size
	PermAdmins      = "admins"      // owner only
)

// AdminPermList is every power an admin can be given, in display order.
var AdminPermList = []string{PermCups, PermDisputes, PermWithdrawals, PermSupport, PermUsers, PermMoney}

// AdminWithdrawalLimit is the largest withdrawal (minor units) an admin
// without the money power can approve or mark paid.
var AdminWithdrawalLimit = map[string]int64{"NGN": 50_000_00, "GHS": 400_00}

// AdminPerms returns the powers stored for a user (nil if not an admin).
func AdminPerms(ctx context.Context, uid string) []string {
	if db.Pool == nil || uid == "" {
		return nil
	}
	var perms []string
	if err := db.Pool.QueryRow(ctx, "SELECT perms FROM admin_roles WHERE user_id = $1", uid).Scan(&perms); err != nil {
		return nil
	}
	return perms
}

// IsAdmin reports whether the user has any admin power.
func IsAdmin(ctx context.Context, uid string) bool { return len(AdminPerms(ctx, uid)) > 0 }

// EnsureOwnerRole gives the owner's account a full admin row, so admin alerts reach it.
func EnsureOwnerRole(ctx context.Context, uid string) {
	if db.Pool == nil {
		return
	}
	db.Pool.Exec(ctx, `INSERT INTO admin_roles (user_id, perms, added_by) VALUES ($1,$2,'owner')
		ON CONFLICT (user_id) DO UPDATE SET perms = EXCLUDED.perms WHERE admin_roles.perms <> EXCLUDED.perms`, uid, AdminPermList)
	db.Pool.Exec(ctx, "UPDATE users SET is_admin = TRUE WHERE id = $1 AND NOT is_admin", uid)
}

func cleanPerms(in []string) ([]string, error) {
	var out []string
	for _, p := range AdminPermList {
		if slices.Contains(in, p) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, userErr(400, "Pick at least one power")
	}
	return out, nil
}

// AdminView is one admin on the owner's Admins page.
type AdminView struct {
	UserID    string    `json:"userId"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Perms     []string  `json:"perms"`
	AddedBy   string    `json:"addedBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func ListAdmins(ctx context.Context) ([]AdminView, error) {
	rows, err := db.Pool.Query(ctx, `SELECT r.user_id, COALESCE(u.username,''), COALESCE(u.email,''), r.perms,
		COALESCE((SELECT username FROM users a WHERE a.id = r.added_by), r.added_by, ''), r.created_at
		FROM admin_roles r LEFT JOIN users u ON u.id = r.user_id ORDER BY r.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []AdminView{}
	for rows.Next() {
		var a AdminView
		if err := rows.Scan(&a.UserID, &a.Username, &a.Email, &a.Perms, &a.AddedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

// SetAdmin adds an admin (found by email or username) or changes their powers.
func SetAdmin(ctx context.Context, ownerID, who string, perms []string) (*AdminView, error) {
	perms, err := cleanPerms(perms)
	if err != nil {
		return nil, err
	}
	who = strings.TrimSpace(who)
	if who == "" {
		return nil, userErr(400, "Type their email or username")
	}
	var uid, name, email string
	err = db.Pool.QueryRow(ctx, `SELECT id, COALESCE(username,''), COALESCE(email,'') FROM users
		WHERE deleted_at IS NULL AND (id = $1 OR LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)) LIMIT 1`, who).Scan(&uid, &name, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, userErr(404, "No CrestArena account uses %q. Ask them to sign up first, with the same email.", who)
	}
	if err != nil {
		return nil, err
	}
	if uid == ownerID {
		return nil, userErr(400, "That's you. The owner always has every power.")
	}
	if _, err := db.Pool.Exec(ctx, `INSERT INTO admin_roles (user_id, perms, added_by) VALUES ($1,$2,$3)
		ON CONFLICT (user_id) DO UPDATE SET perms = EXCLUDED.perms, updated_at = NOW()`, uid, perms, ownerID); err != nil {
		return nil, err
	}
	db.Pool.Exec(ctx, "UPDATE users SET is_admin = TRUE WHERE id = $1", uid)
	AdminLog(ctx, ownerID, "admin_set", uid, fmt.Sprintf("%s (%s): %s", name, email, strings.Join(perms, ", ")))
	Notify(uid, "admin_added", "You're a CrestArena admin",
		"You can now help with: "+PermsText(perms)+". Open Profile → Admin dashboard. Your email must be verified.", "/admin", nil)
	return &AdminView{UserID: uid, Username: name, Email: email, Perms: perms, CreatedAt: time.Now()}, nil
}

// RemoveAdmin takes away all admin powers.
func RemoveAdmin(ctx context.Context, ownerID, uid string) error {
	if uid == ownerID {
		return userErr(400, "You can't remove yourself")
	}
	tag, err := db.Pool.Exec(ctx, "DELETE FROM admin_roles WHERE user_id = $1", uid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return userErr(404, "Not an admin")
	}
	db.Pool.Exec(ctx, "UPDATE users SET is_admin = FALSE WHERE id = $1", uid)
	AdminLog(ctx, ownerID, "admin_removed", uid, Username(ctx, uid))
	return nil
}

var permNames = map[string]string{
	PermCups: "cups", PermDisputes: "disputes", PermWithdrawals: "withdrawals",
	PermSupport: "complaints", PermUsers: "players", PermMoney: "money and wallets",
}

func PermsText(perms []string) string {
	var out []string
	for _, p := range perms {
		out = append(out, permNames[p])
	}
	return strings.Join(out, ", ")
}

// AdminLog records an admin action.
func AdminLog(ctx context.Context, actorID, action, target, detail string) {
	if db.Pool == nil {
		return
	}
	if _, err := db.Pool.Exec(context.WithoutCancel(ctx), "INSERT INTO admin_log (actor_id, action, target, detail) VALUES ($1,$2,$3,$4)",
		actorID, action, target, detail); err != nil {
		log.Printf("[ADMIN LOG] %v", err)
	}
}

type AdminLogEntry struct {
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}

func RecentAdminLog(ctx context.Context, limit int) ([]AdminLogEntry, error) {
	rows, err := db.Pool.Query(ctx, `SELECT COALESCE(u.username, l.actor_id, ''), l.action, COALESCE(l.target,''), COALESCE(l.detail,''), l.created_at
		FROM admin_log l LEFT JOIN users u ON u.id = l.actor_id ORDER BY l.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []AdminLogEntry{}
	for rows.Next() {
		var e AdminLogEntry
		if err := rows.Scan(&e.Actor, &e.Action, &e.Target, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

// CheckWithdrawalLimit stops admins without the money power from approving
// or marking paid a withdrawal above AdminWithdrawalLimit.
func CheckWithdrawalLimit(ctx context.Context, id string) error {
	var amount int64
	var cur string
	if err := db.Pool.QueryRow(ctx, "SELECT amount, currency FROM withdrawals WHERE id = $1", id).Scan(&amount, &cur); err != nil {
		return userErr(404, "Withdrawal not found")
	}
	if limit, ok := AdminWithdrawalLimit[cur]; ok && amount > limit {
		return userErr(403, "Withdrawals above %s need the owner to approve", FormatMoney(limit, cur))
	}
	return nil
}

// notifyAdmins alerts every admin who has the given power.
func notifyAdmins(ctx context.Context, perm, title, message, matchID string) {
	if db.Pool == nil {
		return
	}
	rows, err := db.Pool.Query(ctx, "SELECT user_id FROM admin_roles WHERE $1 = ANY(perms)", perm)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		Notify(id, "admin_review", title, message, "/admin", map[string]any{"matchId": matchID})
	}
}

// NotifyAdmins sends a notification to every admin with the given power.
func NotifyAdmins(ctx context.Context, perm, title, message string) {
	notifyAdmins(ctx, perm, title, message, "")
}
