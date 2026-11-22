package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
)

// MaxCupDescription is the longest cup description / rules text.
const MaxCupDescription = 600

// EditTournamentRequest is what a host (or an admin) can change after posting
// a cup. Prizes, entry fee, game and format never change: players joined for
// those. Round length, size and members-only can only change before it starts.
type EditTournamentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RoundHours  int    `json:"roundHours"`
	MaxPlayers  int    `json:"maxPlayers"`
	MembersOnly bool   `json:"membersOnly"`
}

// validCupSize reports whether n players fits a host cup's format.
func validCupSize(format string, n int) error {
	switch format {
	case "knockout":
		if n != 4 && n != 8 && n != 16 && n != 32 && n != 64 {
			return userErr(400, "Knockout cups take 4, 8, 16, 32 or 64 players")
		}
	case "swiss":
		if n != 8 && n != 16 && n != 32 {
			return userErr(400, "Points cups take 8, 16 or 32 players")
		}
	case "league":
		if n < 4 || n > 8 {
			return userErr(400, "Leagues take 4 to 8 players")
		}
	default:
		return userErr(400, "Choose a format")
	}
	return nil
}

// EditTournament saves changes to a cup. Only its host or an admin can edit.
// It returns a short list of what changed, for the players' notification.
func EditTournament(ctx context.Context, tid, uid string, admin bool, req EditTournamentRequest) ([]string, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if n := len([]rune(req.Name)); n < 3 || n > 40 {
		return nil, userErr(400, "The name must be 3 to 40 characters")
	}
	if len([]rune(req.Description)) > MaxCupDescription {
		return nil, userErr(400, "Keep the description under %d characters", MaxCupDescription)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var name, desc, status, format, host string
	var roundHours, maxPlayers, players int
	var entryFee int64
	var membersOnly bool
	// FOR UPDATE: a player joining at the same moment waits for this edit.
	err = tx.QueryRow(ctx, `SELECT name, COALESCE(description,''), status, format, COALESCE(host_id,''), round_hours, max_players, entry_fee, members_only,
		(SELECT COUNT(*) FROM tournament_players p WHERE p.tournament_id = t.id) FROM tournaments t WHERE id = $1 FOR UPDATE`, tid).
		Scan(&name, &desc, &status, &format, &host, &roundHours, &maxPlayers, &entryFee, &membersOnly, &players)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, userErr(404, "Tournament not found")
	}
	if err != nil {
		return nil, err
	}
	if !admin && (host == "" || host != uid) {
		return nil, userErr(403, "Only the host can edit this cup")
	}
	if status == "finished" || status == "cancelled" {
		return nil, userErr(409, "This cup is over and can't be edited")
	}

	sizeChanged := req.MaxPlayers != maxPlayers
	hoursChanged := req.RoundHours != roundHours
	membersChanged := req.MembersOnly != membersOnly && host != ""
	if status != "open" && (sizeChanged || hoursChanged || membersChanged) {
		return nil, userErr(409, "The cup has started: only the name and description can change now")
	}
	if hoursChanged {
		if host != "" && req.RoundHours != 12 && req.RoundHours != 24 && req.RoundHours != 48 {
			return nil, userErr(400, "Rounds last 12, 24 or 48 hours")
		}
		if req.RoundHours < 1 || req.RoundHours > 168 {
			return nil, userErr(400, "Rounds last 1 to 168 hours")
		}
	}
	if sizeChanged {
		if host == "" || entryFee > 0 {
			// Prizes for CrestArena cups and paid entries depend on the size.
			return nil, userErr(400, "The number of players can't change for this cup")
		}
		if err := validCupSize(format, req.MaxPlayers); err != nil {
			return nil, err
		}
		if req.MaxPlayers <= players {
			return nil, userErr(400, "%d players have joined already. Choose a bigger size", players)
		}
	}
	if membersChanged && req.MembersOnly && players > 0 {
		// Don't lock out players without a plan who joined while it was open to all.
		rows, err := tx.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1", tid)
		if err != nil {
			return nil, err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			m, err := ActiveMembership(ctx, tx, id)
			if err != nil {
				return nil, err
			}
			if m == nil {
				return nil, userErr(400, "Players without a plan have already joined, so it can't become members-only now")
			}
		}
	}
	if !membersChanged {
		req.MembersOnly = membersOnly
	}

	var changes []string
	if req.Name != name {
		changes = append(changes, "new name")
	}
	if req.Description != desc {
		changes = append(changes, "description and rules")
	}
	if hoursChanged {
		changes = append(changes, fmt.Sprintf("%d hours per round", req.RoundHours))
	}
	if sizeChanged {
		changes = append(changes, fmt.Sprintf("%d players", req.MaxPlayers))
	}
	if membersChanged {
		if req.MembersOnly {
			changes = append(changes, "members only")
		} else {
			changes = append(changes, "open to everyone")
		}
	}
	if len(changes) == 0 {
		return nil, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE tournaments SET name = $2, description = NULLIF($3,''), round_hours = $4, max_players = $5, members_only = $6 WHERE id = $1`,
		tid, req.Name, req.Description, req.RoundHours, req.MaxPlayers, req.MembersOnly); err != nil {
		return nil, err
	}
	return changes, tx.Commit(ctx)
}
