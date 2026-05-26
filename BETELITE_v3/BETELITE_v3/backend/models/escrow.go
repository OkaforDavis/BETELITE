package models

import (
	"time"
)

type Challenge struct {
	ID          string    `json:"id" db:"challenge_id"`
	CreatorID   string    `json:"creatorId" db:"creator_id"`
	CreatorName string    `json:"creatorName" db:"creator_name"` // usually fetched dynamically, but good for WebSocket
	Game        string    `json:"game" db:"game"`
	Amount      int64     `json:"amount" db:"amount"`
	Currency    string    `json:"currency" db:"currency"`
	Timestamp   int64     `json:"timestamp"`
}

type Escrow struct {
	ID          int64     `json:"id" db:"id"`
	ChallengeID string    `json:"challengeId" db:"challenge_id"`
	CreatorID   string    `json:"creatorId" db:"creator_id"`
	AcceptorID  string    `json:"acceptorId" db:"acceptor_id"`
	Amount      int64     `json:"amount" db:"amount"`
	Pool        int64     `json:"pool" db:"pool"`
	Status      string    `json:"status" db:"status"` // held, paid_out, refunded
	MatchID     string    `json:"matchId" db:"match_id"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
}
