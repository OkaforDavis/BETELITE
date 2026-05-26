package models

import (
	"time"
)

type Bet struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"userId" db:"user_id"`
	MatchID      string    `json:"matchId" db:"match_id"`
	Pick         string    `json:"pick" db:"pick"` // home, away, draw
	Odds         float32   `json:"odds" db:"odds"`
	Amount       int64     `json:"amount" db:"amount"`
	PotentialWin int64     `json:"potentialWin" db:"potential_win"`
	Currency     string    `json:"currency" db:"currency"`
	Status       string    `json:"status" db:"status"` // live, won, lost
	PlacedAt     time.Time `json:"placedAt" db:"placed_at"`
}
