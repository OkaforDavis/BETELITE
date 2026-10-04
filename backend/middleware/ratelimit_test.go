package middleware

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Many players behind one carrier IP must not block each other, but one
// player making too many requests is limited.
func TestRateLimitPerUserNotPerSharedIP(t *testing.T) {
	app := fiber.New()
	app.Use(RateLimiter(), RateLimiterUser())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })

	call := func(token string) int {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}

	// 40 different players x 20 requests from the same IP: all allowed.
	for p := 0; p < 40; p++ {
		for i := 0; i < 20; i++ {
			if code := call(fmt.Sprintf("player-token-%03d-xxxxxxxxxxxxxxxx", p)); code != 200 {
				t.Fatalf("player %d request %d blocked (%d): shared-IP players must not block each other", p, i, code)
			}
		}
	}
	// One player flooding: blocked after 300 requests a minute.
	blocked := false
	for i := 0; i < 320; i++ {
		if call("flooding-player-token-xxxxxxxxxxxxxxxx") == 429 {
			blocked = true
			break
		}
	}
	if !blocked {
		t.Fatal("a single player sending too many requests must be limited")
	}
}
