package db

import (
	"context"
	"fmt"
	"log"
)

// RunMigrations executes CREATE TABLE IF NOT EXISTS statements
func RunMigrations(ctx context.Context) error {
	if Pool == nil {
		log.Println("[INFO] Skipping migrations, no database connection")
		return nil
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			username TEXT NOT NULL,
			avatar_url TEXT,
			balance BIGINT DEFAULT 0,
			country TEXT DEFAULT 'NG',
			currency TEXT DEFAULT 'NGN',
			push_sub JSONB,
			pending_referral TEXT,
			referral_code TEXT UNIQUE,
			referred_by TEXT,
			push_notifications BOOLEAN DEFAULT TRUE,
			email_notifications BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id SERIAL PRIMARY KEY,
			user_id TEXT REFERENCES users(id),
			type TEXT NOT NULL,
			amount BIGINT NOT NULL,
			ref_id TEXT,
			metadata JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS escrow (
			id SERIAL PRIMARY KEY,
			challenge_id TEXT NOT NULL,
			creator_id TEXT REFERENCES users(id),
			acceptor_id TEXT REFERENCES users(id),
			amount BIGINT NOT NULL,
			pool BIGINT NOT NULL,
			status TEXT DEFAULT 'held',
			match_id TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS bets (
			id TEXT PRIMARY KEY,
			user_id TEXT REFERENCES users(id),
			match_id TEXT NOT NULL,
			pick TEXT NOT NULL,
			odds REAL NOT NULL,
			amount BIGINT NOT NULL,
			potential_win BIGINT NOT NULL,
			currency TEXT DEFAULT 'NGN',
			status TEXT DEFAULT 'live',
			placed_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS tournaments (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			game TEXT NOT NULL,
			mode TEXT,
			icon TEXT,
			entry_fee BIGINT DEFAULT 0,
			max_players INT DEFAULT 8,
			prize_pool BIGINT DEFAULT 0,
			status TEXT DEFAULT 'open',
			current_round INT DEFAULT 0,
			winner_id TEXT,
			created_by TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS tournament_players (
			id SERIAL PRIMARY KEY,
			tournament_id TEXT REFERENCES tournaments(id),
			user_id TEXT REFERENCES users(id),
			wins INT DEFAULT 0,
			losses INT DEFAULT 0,
			goals INT DEFAULT 0,
			points INT DEFAULT 0,
			UNIQUE(tournament_id, user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS fixtures (
			id TEXT PRIMARY KEY,
			tournament_id TEXT REFERENCES tournaments(id),
			round INT NOT NULL,
			home_id TEXT REFERENCES users(id),
			home_name TEXT,
			away_id TEXT REFERENCES users(id),
			away_name TEXT,
			score_home INT,
			score_away INT,
			status TEXT DEFAULT 'pending',
			ai_verified BOOLEAN DEFAULT FALSE,
			submitted_by TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			user_id TEXT REFERENCES users(id),
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			metadata JSONB,
			read BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS push_subscriptions (
			id SERIAL PRIMARY KEY,
			user_id TEXT REFERENCES users(id),
			endpoint TEXT NOT NULL UNIQUE,
			p256dh TEXT NOT NULL,
			auth TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);`,
	}

	for _, query := range queries {
		_, err := Pool.Exec(ctx, query)
		if err != nil {
			return fmt.Errorf("migration error: %v", err)
		}
	}

	log.Println("[INFO] Database migrations completed successfully")
	return nil
}
