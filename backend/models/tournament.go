package models

import (
	"time"
)

type Tournament struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Game         string    `json:"game" db:"game"`
	Mode         string    `json:"mode" db:"mode"`
	Icon         string    `json:"icon" db:"icon"`
	EntryFee     int64     `json:"entryFee" db:"entry_fee"`
	MaxPlayers   int       `json:"maxPlayers" db:"max_players"`
	PrizePool    int64     `json:"prizePool" db:"prize_pool"`
	Status       string    `json:"status" db:"status"` // open, active, finished
	CurrentRound int       `json:"currentRound" db:"current_round"`
	WinnerID     string    `json:"winnerId,omitempty" db:"winner_id"`
	CreatedBy    string    `json:"createdBy,omitempty" db:"created_by"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}

type TournamentPlayer struct {
	ID           int64  `json:"id" db:"id"`
	TournamentID string `json:"tournamentId" db:"tournament_id"`
	UserID       string `json:"userId" db:"user_id"`
	Wins         int    `json:"wins" db:"wins"`
	Losses       int    `json:"losses" db:"losses"`
	Goals        int    `json:"goals" db:"goals"`
	Points       int    `json:"points" db:"points"`
}

type Fixture struct {
	ID           string    `json:"id" db:"id"`
	TournamentID string    `json:"tournamentId" db:"tournament_id"`
	Round        int       `json:"round" db:"round"`
	HomeID       string    `json:"homeId" db:"home_id"`
	HomeName     string    `json:"homeName" db:"home_name"`
	AwayID       string    `json:"awayId" db:"away_id"`
	AwayName     string    `json:"awayName" db:"away_name"`
	ScoreHome    int       `json:"scoreHome" db:"score_home"`
	ScoreAway    int       `json:"scoreAway" db:"score_away"`
	Status       string    `json:"status" db:"status"` // pending, played
	AIVerified   bool      `json:"aiVerified" db:"ai_verified"`
	SubmittedBy  string    `json:"submittedBy,omitempty" db:"submitted_by"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}
