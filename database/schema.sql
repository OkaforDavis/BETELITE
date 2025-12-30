-- BETELITE DATABASE SCHEMA
-- CORE_LOCK: Critical tables with immutable audit trails
-- Version: 1.0
-- Date: 2025-12-30

SET SQL_MODE = 'STRICT_ALL_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';
SET time_zone = '+00:00';

CREATE DATABASE IF NOT EXISTS betelite CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE betelite;

-- ============================================
-- CORE USER SYSTEM
-- ============================================

CREATE TABLE users (
    user_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    user_type ENUM('player', 'team', 'organizer', 'admin') NOT NULL DEFAULT 'player',
    account_status ENUM('active', 'suspended', 'banned', 'pending_verification') NOT NULL DEFAULT 'pending_verification',
    email_verified BOOLEAN DEFAULT FALSE,
    kyc_verified BOOLEAN DEFAULT FALSE,
    mfa_enabled BOOLEAN DEFAULT FALSE,
    mfa_secret VARCHAR(255) NULL,
    country_code CHAR(2) NULL,
    date_of_birth DATE NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    last_login TIMESTAMP NULL,
    INDEX idx_email (email),
    INDEX idx_username (username),
    INDEX idx_status (account_status),
    INDEX idx_type (user_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_profiles (
    profile_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL UNIQUE,
    display_name VARCHAR(100) NULL,
    avatar_url VARCHAR(500) NULL,
    bio TEXT NULL,
    timezone VARCHAR(50) DEFAULT 'UTC',
    preferred_games JSON NULL,
    social_links JSON NULL,
    privacy_settings JSON NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_sessions (
    session_id VARCHAR(128) PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    user_agent TEXT NOT NULL,
    csrf_token VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    INDEX idx_user (user_id),
    INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- GAMES SYSTEM
-- ============================================

CREATE TABLE games (
    game_id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    game_name VARCHAR(100) NOT NULL UNIQUE,
    game_code VARCHAR(20) NOT NULL UNIQUE,
    game_category ENUM('sports', 'fps', 'moba', 'battle_royale', 'fighting', 'racing', 'strategy') NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    supports_teams BOOLEAN DEFAULT TRUE,
    max_team_size INT UNSIGNED DEFAULT 1,
    icon_url VARCHAR(500) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_code (game_code),
    INDEX idx_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- TEAM SYSTEM
-- ============================================

CREATE TABLE teams (
    team_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    team_name VARCHAR(100) NOT NULL,
    team_tag VARCHAR(10) NOT NULL,
    owner_user_id BIGINT UNSIGNED NOT NULL,
    game_id INT UNSIGNED NOT NULL,
    team_status ENUM('active', 'disbanded', 'suspended') DEFAULT 'active',
    logo_url VARCHAR(500) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (game_id) REFERENCES games(game_id) ON DELETE CASCADE,
    UNIQUE KEY unique_team_game (team_name, game_id),
    INDEX idx_owner (owner_user_id),
    INDEX idx_game (game_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE team_members (
    member_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    team_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    member_role ENUM('captain', 'player', 'substitute') DEFAULT 'player',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (team_id) REFERENCES teams(team_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    UNIQUE KEY unique_team_user (team_id, user_id),
    INDEX idx_team (team_id),
    INDEX idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- TOURNAMENT SYSTEM
-- ============================================

CREATE TABLE tournaments (
    tournament_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tournament_name VARCHAR(200) NOT NULL,
    tournament_slug VARCHAR(200) NOT NULL UNIQUE,
    game_id INT UNSIGNED NOT NULL,
    organizer_id BIGINT UNSIGNED NOT NULL,
    tournament_type ENUM('solo', 'team') NOT NULL,
    tournament_format ENUM('single_elimination', 'double_elimination', 'round_robin', 'swiss') DEFAULT 'single_elimination',
    max_participants INT UNSIGNED NOT NULL,
    current_participants INT UNSIGNED DEFAULT 0,
    entry_fee DECIMAL(10,2) DEFAULT 0.00,
    prize_pool DECIMAL(12,2) DEFAULT 0.00,
    prize_distribution JSON NULL,
    tournament_status ENUM('draft', 'registration', 'in_progress', 'completed', 'cancelled') DEFAULT 'draft',
    registration_start TIMESTAMP NULL,
    registration_end TIMESTAMP NULL,
    tournament_start TIMESTAMP NULL,
    tournament_end TIMESTAMP NULL,
    rules TEXT NULL,
    streaming_enabled BOOLEAN DEFAULT TRUE,
    betting_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (game_id) REFERENCES games(game_id) ON DELETE CASCADE,
    FOREIGN KEY (organizer_id) REFERENCES users(user_id) ON DELETE CASCADE,
    INDEX idx_game (game_id),
    INDEX idx_status (tournament_status),
    INDEX idx_organizer (organizer_id),
    INDEX idx_dates (registration_start, registration_end)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE tournament_participants (
    participant_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tournament_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NULL,
    team_id BIGINT UNSIGNED NULL,
    seed_number INT UNSIGNED NULL,
    registration_status ENUM('pending', 'confirmed', 'withdrawn', 'disqualified') DEFAULT 'pending',
    entry_fee_paid BOOLEAN DEFAULT FALSE,
    registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tournament_id) REFERENCES tournaments(tournament_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (team_id) REFERENCES teams(team_id) ON DELETE CASCADE,
    UNIQUE KEY unique_tournament_user (tournament_id, user_id),
    UNIQUE KEY unique_tournament_team (tournament_id, team_id),
    INDEX idx_tournament (tournament_id),
    INDEX idx_status (registration_status),
    CHECK ((user_id IS NOT NULL AND team_id IS NULL) OR (user_id IS NULL AND team_id IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- MATCH SYSTEM
-- ============================================

CREATE TABLE matches (
    match_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tournament_id BIGINT UNSIGNED NOT NULL,
    match_round VARCHAR(50) NOT NULL,
    match_number INT UNSIGNED NOT NULL,
    participant1_id BIGINT UNSIGNED NULL,
    participant2_id BIGINT UNSIGNED NULL,
    winner_id BIGINT UNSIGNED NULL,
    match_status ENUM('scheduled', 'live', 'completed', 'disputed', 'cancelled') DEFAULT 'scheduled',
    score_p1 INT UNSIGNED DEFAULT 0,
    score_p2 INT UNSIGNED DEFAULT 0,
    scheduled_time TIMESTAMP NULL,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    stream_url VARCHAR(500) NULL,
    replay_url VARCHAR(500) NULL,
    referee_id BIGINT UNSIGNED NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (tournament_id) REFERENCES tournaments(tournament_id) ON DELETE CASCADE,
    FOREIGN KEY (participant1_id) REFERENCES tournament_participants(participant_id) ON DELETE SET NULL,
    FOREIGN KEY (participant2_id) REFERENCES tournament_participants(participant_id) ON DELETE SET NULL,
    FOREIGN KEY (winner_id) REFERENCES tournament_participants(participant_id) ON DELETE SET NULL,
    FOREIGN KEY (referee_id) REFERENCES users(user_id) ON DELETE SET NULL,
    INDEX idx_tournament (tournament_id),
    INDEX idx_status (match_status),
    INDEX idx_scheduled (scheduled_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE match_disputes (
    dispute_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    match_id BIGINT UNSIGNED NOT NULL,
    reported_by BIGINT UNSIGNED NOT NULL,
    dispute_reason TEXT NOT NULL,
    dispute_status ENUM('pending', 'under_review', 'resolved', 'rejected') DEFAULT 'pending',
    resolution TEXT NULL,
    resolved_by BIGINT UNSIGNED NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP NULL,
    FOREIGN KEY (match_id) REFERENCES matches(match_id) ON DELETE CASCADE,
    FOREIGN KEY (reported_by) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (resolved_by) REFERENCES users(user_id) ON DELETE SET NULL,
    INDEX idx_match (match_id),
    INDEX idx_status (dispute_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- WALLET SYSTEM (CORE_LOCK + AUDIT_TRACE)
-- ============================================

CREATE TABLE wallets (
    wallet_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL UNIQUE,
    balance_available DECIMAL(12,2) DEFAULT 0.00,
    balance_locked DECIMAL(12,2) DEFAULT 0.00,
    currency_code CHAR(3) DEFAULT 'USD',
    wallet_status ENUM('active', 'frozen', 'suspended') DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    INDEX idx_status (wallet_status),
    CHECK (balance_available >= 0),
    CHECK (balance_locked >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE wallet_transactions (
    transaction_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    wallet_id BIGINT UNSIGNED NOT NULL,
    transaction_type ENUM('deposit', 'withdrawal', 'bet_placed', 'bet_won', 'bet_lost', 'bet_refund', 'tournament_entry', 'tournament_prize', 'tournament_refund', 'admin_adjustment') NOT NULL,
    amount DECIMAL(12,2) NOT NULL,
    balance_before DECIMAL(12,2) NOT NULL,
    balance_after DECIMAL(12,2) NOT NULL,
    reference_type ENUM('bet', 'tournament', 'payment', 'manual') NULL,
    reference_id BIGINT UNSIGNED NULL,
    payment_gateway VARCHAR(50) NULL,
    payment_reference VARCHAR(255) NULL,
    transaction_status ENUM('pending', 'completed', 'failed', 'reversed') DEFAULT 'pending',
    notes TEXT NULL,
    created_by BIGINT UNSIGNED NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (wallet_id) REFERENCES wallets(wallet_id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(user_id) ON DELETE SET NULL,
    INDEX idx_wallet (wallet_id),
    INDEX idx_type (transaction_type),
    INDEX idx_status (transaction_status),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- BETTING SYSTEM (VERSION_GATE: 2.0)
-- ============================================

CREATE TABLE bets (
    bet_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    match_id BIGINT UNSIGNED NOT NULL,
    bet_type ENUM('match_winner', 'score_prediction', 'handicap', 'over_under') DEFAULT 'match_winner',
    bet_on_participant_id BIGINT UNSIGNED NULL,
    bet_amount DECIMAL(10,2) NOT NULL,
    odds DECIMAL(8,4) NOT NULL,
    potential_payout DECIMAL(12,2) NOT NULL,
    bet_status ENUM('pending', 'won', 'lost', 'void', 'refunded') DEFAULT 'pending',
    placed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    settled_at TIMESTAMP NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (match_id) REFERENCES matches(match_id) ON DELETE CASCADE,
    FOREIGN KEY (bet_on_participant_id) REFERENCES tournament_participants(participant_id) ON DELETE SET NULL,
    INDEX idx_user (user_id),
    INDEX idx_match (match_id),
    INDEX idx_status (bet_status),
    INDEX idx_placed (placed_at),
    CHECK (bet_amount > 0),
    CHECK (odds > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE betting_odds (
    odds_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    match_id BIGINT UNSIGNED NOT NULL,
    participant_id BIGINT UNSIGNED NOT NULL,
    odds_type ENUM('match_winner', 'handicap', 'over_under') DEFAULT 'match_winner',
    odds_value DECIMAL(8,4) NOT NULL,
    total_staked DECIMAL(12,2) DEFAULT 0.00,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (match_id) REFERENCES matches(match_id) ON DELETE CASCADE,
    FOREIGN KEY (participant_id) REFERENCES tournament_participants(participant_id) ON DELETE CASCADE,
    INDEX idx_match (match_id),
    INDEX idx_active (is_active),
    CHECK (odds_value > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- STREAMING SYSTEM (SAFE_SWAP)
-- ============================================

CREATE TABLE streams (
    stream_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    match_id BIGINT UNSIGNED NULL,
    streamer_id BIGINT UNSIGNED NOT NULL,
    stream_title VARCHAR(200) NOT NULL,
    stream_provider VARCHAR(50) DEFAULT 'obs',
    stream_key VARCHAR(255) NULL,
    stream_url VARCHAR(500) NULL,
    playback_url VARCHAR(500) NULL,
    stream_status ENUM('idle', 'live', 'ended', 'error') DEFAULT 'idle',
    viewer_count INT UNSIGNED DEFAULT 0,
    started_at TIMESTAMP NULL,
    ended_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (match_id) REFERENCES matches(match_id) ON DELETE SET NULL,
    FOREIGN KEY (streamer_id) REFERENCES users(user_id) ON DELETE CASCADE,
    INDEX idx_match (match_id),
    INDEX idx_streamer (streamer_id),
    INDEX idx_status (stream_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE stream_chat (
    chat_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    stream_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    message TEXT NOT NULL,
    is_deleted BOOLEAN DEFAULT FALSE,
    deleted_by BIGINT UNSIGNED NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (stream_id) REFERENCES streams(stream_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (deleted_by) REFERENCES users(user_id) ON DELETE SET NULL,
    INDEX idx_stream (stream_id),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- SECURITY & AUDIT SYSTEM (AUDIT_TRACE)
-- ============================================

CREATE TABLE audit_logs (
    log_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NULL,
    action_type VARCHAR(100) NOT NULL,
    table_name VARCHAR(100) NULL,
    record_id BIGINT UNSIGNED NULL,
    old_values JSON NULL,
    new_values JSON NULL,
    ip_address VARCHAR(45) NOT NULL,
    user_agent TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user (user_id),
    INDEX idx_action (action_type),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE security_events (
    event_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_type ENUM('failed_login', 'suspicious_activity', 'rate_limit_hit', 'account_locked', 'payment_fraud_detected', 'match_fixing_detected') NOT NULL,
    user_id BIGINT UNSIGNED NULL,
    ip_address VARCHAR(45) NOT NULL,
    severity ENUM('low', 'medium', 'high', 'critical') DEFAULT 'medium',
    details JSON NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE SET NULL,
    INDEX idx_type (event_type),
    INDEX idx_severity (severity),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE rate_limits (
    limit_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    ip_address VARCHAR(45) NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    request_count INT UNSIGNED DEFAULT 1,
    window_start TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    locked_until TIMESTAMP NULL,
    UNIQUE KEY unique_ip_endpoint (ip_address, endpoint),
    INDEX idx_ip (ip_address),
    INDEX idx_locked (locked_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- SYSTEM CONFIGURATION (VERSION_GATE)
-- ============================================

CREATE TABLE system_settings (
    setting_id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    setting_key VARCHAR(100) NOT NULL UNIQUE,
    setting_value TEXT NOT NULL,
    setting_type ENUM('string', 'int', 'float', 'bool', 'json') DEFAULT 'string',
    is_public BOOLEAN DEFAULT FALSE,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    updated_by BIGINT UNSIGNED NULL,
    FOREIGN KEY (updated_by) REFERENCES users(user_id) ON DELETE SET NULL,
    INDEX idx_key (setting_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE feature_flags (
    flag_id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    flag_name VARCHAR(100) NOT NULL UNIQUE,
    flag_version VARCHAR(20) NOT NULL,
    is_enabled BOOLEAN DEFAULT FALSE,
    rollout_percentage INT UNSIGNED DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_name (flag_name),
    INDEX idx_enabled (is_enabled),
    CHECK (rollout_percentage BETWEEN 0 AND 100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;