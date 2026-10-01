package services

import (
	"context"
	"strings"
	"sync"

	"betelite-go/db"
)

// knownUsers caches UIDs already present in Postgres so EnsureUser costs one
// query per user per process lifetime.
var knownUsers sync.Map

// EnsureUser creates the Postgres row for a Firebase user on first contact.
// Postgres is the only wallet; Firebase is used for sign-in only.
func EnsureUser(ctx context.Context, uid, email, name string) error {
	if db.Pool == nil || uid == "" {
		return nil
	}
	if _, ok := knownUsers.Load(uid); ok {
		return nil
	}
	if email == "" {
		email = uid + "@users.crestarena.invalid"
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, username) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, uid, email, name)
	if err != nil {
		return err
	}
	knownUsers.Store(uid, true)
	return nil
}

// ForgetUser drops a UID from the cache (after account deletion).
func ForgetUser(uid string) { knownUsers.Delete(uid) }

// IsAdmin reports whether the user is flagged as admin in the database.
func IsAdmin(ctx context.Context, uid string) bool {
	if db.Pool == nil || uid == "" {
		return false
	}
	var admin bool
	db.Pool.QueryRow(ctx, "SELECT is_admin FROM users WHERE id = $1", uid).Scan(&admin)
	return admin
}

// Username returns the display name for a user, or "Player" if unknown.
func Username(ctx context.Context, uid string) string {
	var name string
	if db.Pool != nil {
		db.Pool.QueryRow(ctx, "SELECT username FROM users WHERE id = $1", uid).Scan(&name)
	}
	if name == "" {
		return "Player"
	}
	return name
}

// IsDeleted reports whether the account was deleted at the user's request.
func IsDeleted(ctx context.Context, uid string) bool {
	if db.Pool == nil {
		return false
	}
	var deleted bool
	db.Pool.QueryRow(ctx, "SELECT deleted_at IS NOT NULL FROM users WHERE id = $1", uid).Scan(&deleted)
	return deleted
}
