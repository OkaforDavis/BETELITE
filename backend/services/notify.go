package services

import (
	"context"
	"encoding/json"
	"log"

	webpush "github.com/SherClockHolmes/webpush-go"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/utils"
	"betelite-go/ws"
)

// Hub is the WebSocket hub used to push live updates; set at startup.
var Hub *ws.Hub

// Notify stores an in-app notification, pushes it live over WebSocket and
// sends a Web Push to the user's devices (when they allow push). url is the
// in-app route to open when the notification is tapped.
func Notify(uid, kind, title, message, url string, meta map[string]any) {
	if uid == "" {
		return
	}
	ctx := context.Background()
	id := "ntf_" + utils.GenerateBaseID()
	if meta == nil {
		meta = map[string]any{}
	}
	meta["url"] = url
	metaJSON, _ := json.Marshal(meta)

	if db.Pool != nil {
		if _, err := db.Pool.Exec(ctx,
			"INSERT INTO notifications (id, user_id, type, title, message, metadata) VALUES ($1,$2,$3,$4,$5,$6::jsonb)",
			id, uid, kind, title, message, string(metaJSON)); err != nil {
			log.Printf("[NOTIFY] store failed for %s: %v", uid, err)
		}
	}

	if Hub != nil {
		ws.SendEvent(Hub, uid, "notification", map[string]any{
			"id": id, "type": kind, "title": title, "message": message, "metadata": meta,
		})
	}

	go sendPush(uid, map[string]any{"title": title, "body": message, "url": url, "tag": kind, "id": id})
}

func sendPush(uid string, payload map[string]any) {
	if config.Cfg.VAPIDPrivateKey == "" || db.Pool == nil {
		return
	}
	ctx := context.Background()

	var enabled bool = true
	db.Pool.QueryRow(ctx, "SELECT push_notifications FROM users WHERE id = $1", uid).Scan(&enabled)
	if !enabled {
		return
	}

	body, _ := json.Marshal(payload)
	rows, err := db.Pool.Query(ctx, "SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1", uid)
	if err != nil {
		return
	}
	var subs []webpush.Subscription
	for rows.Next() {
		var s webpush.Subscription
		if rows.Scan(&s.Endpoint, &s.Keys.P256dh, &s.Keys.Auth) == nil {
			subs = append(subs, s)
		}
	}
	rows.Close()

	for _, s := range subs {
		resp, err := webpush.SendNotification(body, &s, &webpush.Options{
			Subscriber:      "mailto:" + config.Cfg.AdminEmail,
			VAPIDPublicKey:  config.Cfg.VAPIDPublicKey,
			VAPIDPrivateKey: config.Cfg.VAPIDPrivateKey,
			TTL:             60 * 60 * 24,
			Urgency:         webpush.UrgencyHigh,
		})
		if err != nil {
			log.Printf("[PUSH] send failed: %v", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			db.Pool.Exec(ctx, "DELETE FROM push_subscriptions WHERE endpoint = $1", s.Endpoint)
		}
	}
}
