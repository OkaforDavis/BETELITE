package routes

import (
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/livekit/protocol/auth"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
	"betelite-go/ws"
)

// Stream is a live broadcast hosted by a player (LiveKit room).
type Stream struct {
	ID        string    `json:"id"`
	HostID    string    `json:"hostId"`
	HostName  string    `json:"hostName"`
	Title     string    `json:"title"`
	Game      string    `json:"game"`
	MatchID   string    `json:"matchId,omitempty"`
	StartedAt time.Time `json:"startedAt"`
}

var (
	streamsMu sync.RWMutex
	streams   = map[string]*Stream{}
)

func SetupStreamRoutes(api fiber.Router, hub *ws.Hub) {
	api.Get("/streams", func(c *fiber.Ctx) error {
		streamsMu.RLock()
		list := make([]*Stream, 0, len(streams))
		for _, s := range streams {
			list = append(list, s)
		}
		streamsMu.RUnlock()
		return utils.SendSuccess(c, fiber.Map{"streams": list})
	})

	// Tips: free thank-yous to a host or player, sent from a cup, match or stream.
	tips := api.Group("/tips", middleware.AuthRequired(), dbRequired)
	tips.Get("/options", func(c *fiber.Ctx) error {
		var currency string
		var balance int64
		db.Pool.QueryRow(c.Context(), "SELECT COALESCE(currency,'NGN'), balance FROM users WHERE id = $1", middleware.GetUID(c)).Scan(&currency, &balance)
		return utils.SendSuccess(c, fiber.Map{"currency": currency, "balance": balance, "min": services.MinTip[currency], "max": services.MaxTip[currency],
			"daily": services.MaxTipDaily[currency], "presets": services.TipPresets[currency]})
	})
	tips.Post("/", func(c *fiber.Ctx) error {
		var req services.TipRequest
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		if req.Context == "stream" {
			streamsMu.RLock()
			s, ok := streams[req.RefID]
			streamsMu.RUnlock()
			if !ok || s.HostID != req.To {
				return utils.SendError(c, 400, "This stream has ended")
			}
		} else if err := services.TipAllowed(c.Context(), req); err != nil {
			return fail(c, err)
		}
		res, err := services.SendTip(c.Context(), middleware.GetUID(c), req)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"tip": res})
	})

	st := api.Group("/streams", middleware.AuthRequired())

	st.Post("/", func(c *fiber.Ctx) error {
		var req struct {
			Title   string `json:"title"`
			Game    string `json:"game"`
			MatchID string `json:"matchId"`
		}
		c.BodyParser(&req)
		uid := middleware.GetUID(c)
		// Going live is a membership perk.
		if !services.HasPaidPlan(c.Context(), uid) && !middleware.IsAdminCtx(c) {
			return utils.SendError(c, 403, "Going live is for members. Get a plan to stream your matches.")
		}
		title := strings.TrimSpace(req.Title)
		if title == "" || len(title) > 60 {
			title = "Live match"
		}
		s := &Stream{
			ID: "stream_" + utils.GenerateBaseID(), HostID: uid, HostName: services.Username(c.Context(), uid),
			Title: title, Game: req.Game, MatchID: req.MatchID, StartedAt: time.Now(),
		}
		streamsMu.Lock()
		for id, old := range streams { // one live stream per host
			if old.HostID == uid {
				delete(streams, id)
			}
		}
		streams[s.ID] = s
		streamsMu.Unlock()
		ws.BroadcastEvent(hub, "stream_start", s)
		return utils.SendSuccess(c, fiber.Map{"stream": s})
	})

	st.Delete("/:id", func(c *fiber.Ctx) error {
		streamsMu.Lock()
		s, ok := streams[c.Params("id")]
		if ok && (s.HostID == middleware.GetUID(c) || middleware.IsAdminCtx(c)) {
			delete(streams, s.ID)
		} else {
			ok = false
		}
		streamsMu.Unlock()
		if !ok {
			return utils.SendError(c, 404, "Stream not found")
		}
		ws.BroadcastEvent(hub, "stream_end", fiber.Map{"id": s.ID})
		return utils.SendSuccess(c, fiber.Map{})
	})

	// LiveKit token: only the host may publish video; viewers may chat.
	st.Post("/:id/token", func(c *fiber.Ctx) error {
		if config.Cfg.LiveKitAPIKey == "" || config.Cfg.LiveKitAPISecret == "" {
			return utils.SendError(c, 503, "Streaming is not available right now")
		}
		streamsMu.RLock()
		s, ok := streams[c.Params("id")]
		streamsMu.RUnlock()
		if !ok {
			return utils.SendError(c, 404, "This stream has ended")
		}
		uid := middleware.GetUID(c)
		isHost := s.HostID == uid
		yes, no := true, false
		grant := &auth.VideoGrant{RoomJoin: true, Room: s.ID, CanPublishData: &yes, CanPublish: &no}
		if isHost {
			grant.CanPublish = &yes
		}
		token, err := auth.NewAccessToken(config.Cfg.LiveKitAPIKey, config.Cfg.LiveKitAPISecret).
			AddGrant(grant).SetIdentity(uid).SetName(services.Username(c.Context(), uid)).SetValidFor(4 * time.Hour).ToJWT()
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"token": token, "isHost": isHost, "url": config.Cfg.LiveKitURL})
	})
}
