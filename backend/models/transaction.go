package models

import (
	"encoding/json"
	"time"
)

type Transaction struct {
	ID        int64            `json:"id" db:"id"`
	UserID    string           `json:"userId" db:"user_id"`
	Type      string           `json:"type" db:"type"` // deposit, withdrawal, wager_hold, wager_win, wager_refund, tournament_entry, tournament_prize, bet_place, bet_win, referral
	Amount    int64            `json:"amount" db:"amount"` // positive = credit, negative = debit
	RefID     string           `json:"refId,omitempty" db:"ref_id"`
	Metadata  *json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt time.Time        `json:"createdAt" db:"created_at"`
}
