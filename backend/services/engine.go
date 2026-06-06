package services

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"betelite-go/models"
	"betelite-go/ws"
)

// GameEngine handles all active matches and ticking logic
type GameEngine struct {
	Matches map[string]*models.Match
	mu      sync.RWMutex
	Hub     *ws.Hub
}

var Engine *GameEngine

func InitEngine(hub *ws.Hub) {
	Engine = &GameEngine{
		Matches: make(map[string]*models.Match),
		Hub:     hub,
	}
	// Start the global ticker loop
	go Engine.StartTicker()
}

func (e *GameEngine) StartTicker() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		e.tickAll()
	}
}

func (e *GameEngine) tickAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for id, match := range e.Matches {
		if match.Status != "live" {
			continue
		}

		match.Minute += 4 // Simulate 4 mins every 30s
		if match.Minute > 90 {
			match.Minute = 90
		}
		
		// Simulate random goals if it's an AI/simulated match
		// For P2P matches, goals come from OCR/Manual input, but for this basic version:
		if !match.IsP2P {
			if rand.Float32() < 0.05 {
				match.ScoreHome++
				ws.BroadcastEvent(e.Hub, "match_goal", map[string]interface{}{"matchId": id, "team": "home", "score": match.ScoreHome})
			} else if rand.Float32() < 0.05 {
				match.ScoreAway++
				ws.BroadcastEvent(e.Hub, "match_goal", map[string]interface{}{"matchId": id, "team": "away", "score": match.ScoreAway})
			}
		}

		if match.Minute >= 90 {
			match.Status = "finished"
			e.HandleMatchEnd(match)
		} else {
			ws.BroadcastEvent(e.Hub, "match_tick", map[string]interface{}{"matchId": id, "minute": match.Minute, "scoreHome": match.ScoreHome, "scoreAway": match.ScoreAway})
		}
	}
}

func (e *GameEngine) AddMatch(match *models.Match) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Matches[match.ID] = match
	ws.BroadcastEvent(e.Hub, "match_created", match)
}

func (e *GameEngine) GetMatch(id string) *models.Match {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Matches[id]
}

func (e *GameEngine) HandleMatchEnd(match *models.Match) {
	winner := "draw"
	if match.ScoreHome > match.ScoreAway {
		winner = "home"
	} else if match.ScoreAway > match.ScoreHome {
		winner = "away"
	}

	match.Result = &models.MatchResult{
		Winner:         winner,
		FinalScoreHome: match.ScoreHome,
		FinalScoreAway: match.ScoreAway,
		Duration:       90,
	}

	log.Printf("Match %s finished. Winner: %s", match.ID, winner)
	ws.BroadcastEvent(e.Hub, "match_ended", match)

	// If this was an escrowed match, trigger payout
	if match.ChallengeID != "" {
		go HandleEscrowPayout(match)
	}

	// In a real app, you might also settle bets here, 
	// or wait for AI verification.
}

func (e *GameEngine) GetActiveMatches() []*models.Match {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var activeMatches []*models.Match
	for _, match := range e.Matches {
		activeMatches = append(activeMatches, match)
	}
	return activeMatches
}
