package utils

import (
	"strings"

	"github.com/google/uuid"
)

// GenerateBaseID generates a fast, URL-safe UUID string without hyphens
func GenerateBaseID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

// GenerateMatchID generates a prefixed ID for matches (e.g. match_xxx)
func GenerateMatchID() string {
	return "match_" + GenerateBaseID()
}

// GenerateTournamentID generates a prefixed ID for tournaments
func GenerateTournamentID() string {
	return "tourn_" + GenerateBaseID()
}

// GenerateChallengeID generates a prefixed ID for wager challenges
func GenerateChallengeID() string {
	return "chal_" + GenerateBaseID()
}

// GenerateBetID generates a prefixed ID for spectator bets
func GenerateBetID() string {
	return "bet_" + GenerateBaseID()
}

// GenerateStreamID generates a prefixed ID for streams
func GenerateStreamID() string {
	return "stream_" + GenerateBaseID()
}
