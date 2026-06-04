package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID              string           `json:"id" db:"id"`
	Email           string           `json:"email" db:"email"`
	Username        string           `json:"username" db:"username"`
	AvatarURL       string           `json:"avatarUrl" db:"avatar_url"`
	Balance         int64            `json:"balance" db:"balance"`
	Country         string           `json:"country" db:"country"`
	Currency        string           `json:"currency" db:"currency"`
	PushSub         *json.RawMessage `json:"pushSub,omitempty" db:"push_sub"`
	PendingReferral string           `json:"pendingReferral,omitempty" db:"pending_referral"`
	CreatedAt       time.Time        `json:"createdAt" db:"created_at"`
}
