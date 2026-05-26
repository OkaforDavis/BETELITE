package services

import (
	"context"
	"encoding/json"
	"log"

	webpush "github.com/SherClockHolmes/webpush-go"
	"betelite-go/config"
	"betelite-go/db"
)

type NotificationPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Icon  string `json:"icon,omitempty"`
	URL   string `json:"url,omitempty"`
}

// SendPushNotification sends a push notification to all subscriptions of a user
func SendPushNotification(userID string, payload NotificationPayload) error {
	if config.Cfg.VAPIDPrivateKey == "" {
		// Web push not configured, silently ignore or log
		log.Println("[WARN] VAPID keys not configured. Skipping push notification.")
		return nil
	}

	payloadBytes, _ := json.Marshal(payload)

	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, "SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1", userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var sub webpush.Subscription
		err := rows.Scan(&sub.Endpoint, &sub.Keys.P256dh, &sub.Keys.Auth)
		if err != nil {
			continue
		}

		// Send notification
		resp, err := webpush.SendNotification(payloadBytes, &sub, &webpush.Options{
			Subscriber:      "mailto:" + config.Cfg.AdminEmail,
			VAPIDPublicKey:  config.Cfg.VAPIDPublicKey,
			VAPIDPrivateKey: config.Cfg.VAPIDPrivateKey,
			TTL:             30,
		})

		if err != nil {
			log.Printf("Failed to send push: %v", err)
			continue
		}
		defer resp.Body.Close()
		
		// If endpoint is expired or invalid, remove it from DB
		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			db.Pool.Exec(ctx, "DELETE FROM push_subscriptions WHERE endpoint = $1", sub.Endpoint)
		}
	}
	return nil
}
