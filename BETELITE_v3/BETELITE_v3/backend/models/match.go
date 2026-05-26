package models

// Match represents a live game in the game engine.
// In Phase 1, this exists mostly in-memory (in the engine).
type Match struct {
	ID           string       `json:"id"`
	Game         string       `json:"game"`
	Label        string       `json:"label"`
	Home         string       `json:"home"`
	HomeID       string       `json:"homeId"`
	Away         string       `json:"away"`
	AwayID       string       `json:"awayId"`
	ScoreHome    int          `json:"scoreHome"`
	ScoreAway    int          `json:"scoreAway"`
	Status       string       `json:"status"` // waiting, live, finished
	Minute       int          `json:"minute"`
	DisplayTime  string       `json:"displayTime"`
	WagerPool    int64        `json:"wagerPool"`
	WagerAmount  int64        `json:"wagerAmount"`
	IsP2P        bool         `json:"isP2p"`
	IsLockedRoom bool         `json:"isLockedRoom"`
	ChallengeID  string       `json:"challengeId,omitempty"`
	Result       *MatchResult `json:"result,omitempty"`
	VerifiedBy   string       `json:"verifiedBy,omitempty"`
}

type MatchResult struct {
	Winner         string `json:"winner"`
	FinalScoreHome int    `json:"finalScoreHome"`
	FinalScoreAway int    `json:"finalScoreAway"`
	Duration       int    `json:"duration"`
}
