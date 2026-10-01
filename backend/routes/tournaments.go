package routes

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

type tournamentView struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Game         string     `json:"game"`
	GameName     string     `json:"gameName"`
	Format       string     `json:"format"`
	Icon         string     `json:"icon"`
	EntryFee     int64      `json:"entryFee"`
	Currency     string     `json:"currency"`
	MaxPlayers   int        `json:"maxPlayers"`
	PlayerCount  int        `json:"playerCount"`
	Status       string     `json:"status"`
	CurrentRound int        `json:"currentRound"`
	RoundHours   int        `json:"roundHours"`
	PrizeSplit   []float64  `json:"prizeSplit"`
	Prizes       []int64    `json:"prizes"` // projected at full capacity
	Joined       bool       `json:"joined"`
	WinnerID     string     `json:"winnerId,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
}

const tournamentSelect = `SELECT t.id, t.name, t.game, t.format, COALESCE(t.icon,''), t.entry_fee, t.currency, t.max_players,
	(SELECT COUNT(*) FROM tournament_players p WHERE p.tournament_id = t.id), t.status, t.current_round, t.round_hours,
	t.prize_split, t.prize_pool, EXISTS(SELECT 1 FROM tournament_players p WHERE p.tournament_id = t.id AND p.user_id = $1),
	COALESCE(t.winner_id,''), t.created_at, t.started_at FROM tournaments t`

func scanTournament(row pgx.Row) (*tournamentView, error) {
	var t tournamentView
	var split []byte
	var seed int64
	if err := row.Scan(&t.ID, &t.Name, &t.Game, &t.Format, &t.Icon, &t.EntryFee, &t.Currency, &t.MaxPlayers, &t.PlayerCount,
		&t.Status, &t.CurrentRound, &t.RoundHours, &split, &seed, &t.Joined, &t.WinnerID, &t.CreatedAt, &t.StartedAt); err != nil {
		return nil, err
	}
	t.PrizeSplit = services.DefaultSplit(t.Format)
	if len(split) > 0 {
		var s []float64
		if json.Unmarshal(split, &s) == nil && len(s) > 0 {
			t.PrizeSplit = s
		}
	}
	t.Prizes = services.PrizeAmounts(t.EntryFee, int64(t.MaxPlayers), seed, t.PrizeSplit)
	if g := services.GameByID(t.Game); g != nil {
		t.GameName = g.Short
	}
	return &t, nil
}

func SetupTournamentRoutes(api fiber.Router) {
	tr := api.Group("/tournaments", middleware.AuthRequired(), dbRequired)

	tr.Get("/", func(c *fiber.Ctx) error {
		rows, err := db.Pool.Query(c.Context(), tournamentSelect+`
			WHERE t.status <> 'finished' OR t.finished_at > NOW() - INTERVAL '7 days'
			ORDER BY (t.status = 'open') DESC, t.created_at DESC LIMIT 50`, middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []*tournamentView{}
		for rows.Next() {
			t, err := scanTournament(rows)
			if err != nil {
				return fail(c, err)
			}
			list = append(list, t)
		}
		return utils.SendSuccess(c, fiber.Map{"tournaments": list})
	})

	tr.Get("/:id", func(c *fiber.Ctx) error {
		ctx := c.Context()
		t, err := scanTournament(db.Pool.QueryRow(ctx, tournamentSelect+" WHERE t.id = $2", middleware.GetUID(c), c.Params("id")))
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if err != nil {
			return fail(c, err)
		}
		table, err := services.Standings(ctx, db.Pool, t.ID)
		if err != nil {
			return fail(c, err)
		}
		matches, err := services.TournamentMatches(ctx, t.ID)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"tournament": t, "standings": table, "matches": matches})
	})

	tr.Post("/:id/join", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		ctx := c.Context()
		tid := c.Params("id")

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)

		var fee int64
		var maxPlayers int
		var status, game, currency, name string
		err = tx.QueryRow(ctx, "SELECT entry_fee, max_players, status, game, currency, name FROM tournaments WHERE id = $1 FOR UPDATE", tid).
			Scan(&fee, &maxPlayers, &status, &game, &currency, &name)
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if err != nil {
			return fail(c, err)
		}
		if status != "open" {
			return utils.SendError(c, 409, "Registration for this tournament is closed")
		}
		if err := services.RequireGameProfile(ctx, tx, uid, game); err != nil {
			return fail(c, err)
		}
		var count int
		var joined bool
		tx.QueryRow(ctx, "SELECT COUNT(*), COALESCE(BOOL_OR(user_id = $2), false) FROM tournament_players WHERE tournament_id = $1", tid, uid).Scan(&count, &joined)
		if joined {
			return utils.SendError(c, 409, "You're already registered")
		}
		if count >= maxPlayers {
			return utils.SendError(c, 409, "This tournament is full")
		}
		if fee > 0 {
			if err := services.RequireMoneyAccess(ctx, uid); err != nil {
				return fail(c, err)
			}
			var myCurrency string
			tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&myCurrency)
			if myCurrency != currency {
				return utils.SendError(c, 400, "This tournament is played in "+currency)
			}
			if err := services.AdjustBalance(ctx, tx, uid, -fee, "tournament_entry", tid); err != nil {
				if errors.Is(err, services.ErrInsufficientFunds) {
					return utils.SendError(c, 400, "Insufficient balance for the entry fee")
				}
				return fail(c, err)
			}
		}
		if _, err := tx.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1,$2)", tid, uid); err != nil {
			return fail(c, err)
		}
		started := count+1 == maxPlayers
		if started {
			if err := services.StartTournament(ctx, tx, tid); err != nil {
				return fail(c, err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}
		if started {
			go notifyTournamentStart(tid, name)
		}
		return utils.SendSuccess(c, fiber.Map{"started": started})
	})

	// Leave before it starts (entry fee refunded).
	tr.Post("/:id/leave", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		ctx := c.Context()
		tid := c.Params("id")
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)
		var fee int64
		var status string
		if err := tx.QueryRow(ctx, "SELECT entry_fee, status FROM tournaments WHERE id = $1 FOR UPDATE", tid).Scan(&fee, &status); err != nil {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if status != "open" {
			return utils.SendError(c, 409, "You can't leave after the tournament has started")
		}
		tag, err := tx.Exec(ctx, "DELETE FROM tournament_players WHERE tournament_id = $1 AND user_id = $2", tid, uid)
		if err != nil {
			return fail(c, err)
		}
		if tag.RowsAffected() == 0 {
			return utils.SendError(c, 404, "You're not registered")
		}
		if fee > 0 {
			if err := services.AdjustBalance(ctx, tx, uid, fee, "tournament_refund", tid); err != nil {
				return fail(c, err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})
}

func notifyTournamentStart(tid, name string) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1", tid)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		services.Notify(id, "tournament_started", name+" has started! 🏁",
			"Your first match is ready. Open the tournament to see your opponent and deadline.", "/tournaments/"+tid, map[string]any{"tournamentId": tid})
	}
	// Round 1 may contain only byes (odd player counts); move on if so.
	services.AdvanceTournament(ctx, tid)
}
