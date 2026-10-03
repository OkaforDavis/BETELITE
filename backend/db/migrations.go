package db

import (
	"context"
	"fmt"
	"log"
)

// migrations are applied in order, each exactly once, and recorded in
// schema_migrations. Never edit a migration that has shipped — append a new one.
var migrations = []string{
	// 1: original schema (idempotent so existing databases adopt it cleanly)
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
	);
	CREATE TABLE IF NOT EXISTS transactions (
		id SERIAL PRIMARY KEY,
		user_id TEXT REFERENCES users(id),
		type TEXT NOT NULL,
		amount BIGINT NOT NULL,
		ref_id TEXT,
		metadata JSONB,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS escrow (
		id SERIAL PRIMARY KEY,
		challenge_id TEXT NOT NULL,
		creator_id TEXT REFERENCES users(id),
		acceptor_id TEXT REFERENCES users(id),
		amount BIGINT NOT NULL,
		pool BIGINT NOT NULL,
		status TEXT DEFAULT 'held',
		match_id TEXT,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS bets (
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
	);
	CREATE TABLE IF NOT EXISTS tournaments (
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
	);
	CREATE TABLE IF NOT EXISTS tournament_players (
		id SERIAL PRIMARY KEY,
		tournament_id TEXT REFERENCES tournaments(id),
		user_id TEXT REFERENCES users(id),
		wins INT DEFAULT 0,
		losses INT DEFAULT 0,
		goals INT DEFAULT 0,
		points INT DEFAULT 0,
		UNIQUE(tournament_id, user_id)
	);
	CREATE TABLE IF NOT EXISTS fixtures (
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
	);
	CREATE TABLE IF NOT EXISTS notifications (
		id TEXT PRIMARY KEY,
		user_id TEXT REFERENCES users(id),
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		message TEXT NOT NULL,
		metadata JSONB,
		read BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS push_subscriptions (
		id SERIAL PRIMARY KEY,
		user_id TEXT REFERENCES users(id),
		endpoint TEXT NOT NULL UNIQUE,
		p256dh TEXT NOT NULL,
		auth TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);`,

	// 2: single wallet, game profiles, persistent matches, deposits/withdrawals,
	// tournament formats, legal consent fields.
	`ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS birth_date DATE;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_version TEXT;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_accepted_at TIMESTAMPTZ;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS marketing_consent BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();
	DO $$ BEGIN
		ALTER TABLE users ADD CONSTRAINT users_balance_nonneg CHECK (balance >= 0);
	EXCEPTION WHEN duplicate_object THEN NULL; END $$;

	CREATE INDEX IF NOT EXISTS transactions_user_idx ON transactions(user_id, created_at DESC);

	CREATE TABLE IF NOT EXISTS game_profiles (
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		game TEXT NOT NULL,
		gamertag TEXT NOT NULL,
		ingame_id TEXT,
		locked BOOLEAN NOT NULL DEFAULT FALSE,
		updated_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (user_id, game)
	);
	CREATE UNIQUE INDEX IF NOT EXISTS game_profiles_tag_uniq ON game_profiles(game, lower(gamertag));

	ALTER TABLE escrow ADD COLUMN IF NOT EXISTS game TEXT;
	ALTER TABLE escrow ADD COLUMN IF NOT EXISTS currency TEXT DEFAULT 'NGN';
	CREATE UNIQUE INDEX IF NOT EXISTS escrow_challenge_uniq ON escrow(challenge_id);

	ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS format TEXT NOT NULL DEFAULT 'knockout';
	ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS prize_split JSONB;
	ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS round_hours INT NOT NULL DEFAULT 24;
	ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS currency TEXT NOT NULL DEFAULT 'NGN';
	ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ;
	ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ;
	ALTER TABLE tournament_players ADD COLUMN IF NOT EXISTS draws INT NOT NULL DEFAULT 0;
	ALTER TABLE tournament_players ADD COLUMN IF NOT EXISTS goals_against INT NOT NULL DEFAULT 0;
	ALTER TABLE tournament_players ADD COLUMN IF NOT EXISTS eliminated BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE tournament_players ADD COLUMN IF NOT EXISTS final_position INT;
	ALTER TABLE tournament_players ADD COLUMN IF NOT EXISTS joined_at TIMESTAMPTZ DEFAULT NOW();

	CREATE TABLE IF NOT EXISTS matches (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		game TEXT NOT NULL,
		home_id TEXT NOT NULL REFERENCES users(id),
		away_id TEXT REFERENCES users(id),
		challenge_id TEXT,
		tournament_id TEXT REFERENCES tournaments(id),
		round INT,
		slot INT,
		status TEXT NOT NULL DEFAULT 'ready',
		score_home INT,
		score_away INT,
		pens_home INT,
		pens_away INT,
		winner_id TEXT,
		submitted_by TEXT,
		submitted_at TIMESTAMPTZ,
		dispute_deadline TIMESTAMPTZ,
		disputed_by TEXT,
		dispute_reason TEXT,
		dispute_ocr JSONB,
		ingame_match_id TEXT,
		screenshot_hash TEXT,
		ocr_result JSONB,
		play_deadline TIMESTAMPTZ,
		resolved_by TEXT,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		settled_at TIMESTAMPTZ
	);
	CREATE INDEX IF NOT EXISTS matches_home_idx ON matches(home_id, status);
	CREATE INDEX IF NOT EXISTS matches_away_idx ON matches(away_id, status);
	CREATE INDEX IF NOT EXISTS matches_status_idx ON matches(status, dispute_deadline);
	CREATE INDEX IF NOT EXISTS matches_tournament_idx ON matches(tournament_id, round);
	CREATE UNIQUE INDEX IF NOT EXISTS matches_ingame_uniq ON matches(game, ingame_match_id) WHERE ingame_match_id IS NOT NULL;
	CREATE UNIQUE INDEX IF NOT EXISTS matches_shot_uniq ON matches(screenshot_hash) WHERE screenshot_hash IS NOT NULL;

	CREATE TABLE IF NOT EXISTS deposits (
		reference TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id),
		amount BIGINT NOT NULL,
		currency TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at TIMESTAMPTZ DEFAULT NOW(),
		paid_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS withdrawals (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id),
		amount BIGINT NOT NULL,
		currency TEXT NOT NULL,
		bank_code TEXT NOT NULL,
		bank_name TEXT,
		account_number TEXT NOT NULL,
		account_name TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		recipient_code TEXT,
		transfer_code TEXT,
		reviewed_by TEXT,
		note TEXT,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS withdrawals_status_idx ON withdrawals(status, created_at);

	CREATE INDEX IF NOT EXISTS notifications_user_idx ON notifications(user_id, created_at DESC);`,

	// 3: profile photos (stored in the DB so they survive redeploys and host moves).
	`CREATE TABLE IF NOT EXISTS avatars (
		user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
		image BYTEA NOT NULL,
		content_type TEXT NOT NULL,
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);`,

	// 4: deposit reconciliation bookkeeping.
	`ALTER TABLE deposits ADD COLUMN IF NOT EXISTS checked_at TIMESTAMPTZ;
	ALTER TABLE deposits ADD COLUMN IF NOT EXISTS gateway_status TEXT;
	CREATE INDEX IF NOT EXISTS deposits_status_idx ON deposits(status, created_at);`,

	// 5: match screenshot evidence, kept 30 days after a match is final.
	`CREATE TABLE IF NOT EXISTS match_screenshots (
		id BIGSERIAL PRIMARY KEY,
		match_id TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
		uploaded_by TEXT NOT NULL REFERENCES users(id),
		kind TEXT NOT NULL,
		image BYTEA NOT NULL,
		content_type TEXT NOT NULL DEFAULT 'image/jpeg',
		created_at TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS match_screenshots_match_idx ON match_screenshots(match_id);
	CREATE INDEX IF NOT EXISTS match_screenshots_created_idx ON match_screenshots(created_at);`,
}

// RunMigrations applies any migrations not yet recorded in schema_migrations.
func RunMigrations(ctx context.Context) error {
	if Pool == nil {
		log.Println("[INFO] Skipping migrations, no database connection")
		return nil
	}

	if _, err := Pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for i, sql := range migrations {
		version := i + 1
		var exists bool
		if err := Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if exists {
			continue
		}

		tx, err := Pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, sql); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %d: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Printf("[INFO] Applied migration %d", version)
	}

	log.Println("[INFO] Database migrations up to date")
	return nil
}
