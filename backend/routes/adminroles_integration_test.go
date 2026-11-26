package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/services"
)

// Runs only with a real database: TEST_DATABASE_URL=postgres://... go test ./routes/
func TestAdminRoles(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	config.Load()
	ctx := context.Background()
	if err := db.Connect(ctx, url); err != nil {
		t.Fatal(err)
	}
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatal(err)
	}
	p := fmt.Sprintf("r%d", os.Getpid())
	uid := func(u string) string { return "dev-" + p + u }
	config.Cfg.AdminEmail = uid("owner") + "@example.com" // dev logins use <uid>@example.com

	app := fiber.New()
	api := app.Group("/api")
	SetupAdminRoutes(api)
	SetupMembershipRoutes(api)
	SetupMeRoutes(api)

	call := func(user, method, path string, body any) (int, map[string]any) {
		var r io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			r = strings.NewReader(string(b))
		}
		req := httptest.NewRequest(method, path, r)
		req.Header.Set("Content-Type", "application/json")
		who := p + user
		if strings.HasPrefix(user, "unverified") {
			who = "unverified" + p + user // dev tokens starting "unverified" have an unverified email
		}
		req.Header.Set("Authorization", "Bearer dev:"+who)
		res, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]any{}
		json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	for _, u := range []string{"owner", "thiago", "cupsonly", "unverifiedx"} {
		call(u, "GET", "/api/membership", nil) // creates the user row
	}
	db.Pool.Exec(ctx, "UPDATE users SET email = id || '@example.com' WHERE id LIKE $1", "%"+p+"%")
	unverifiedID := "dev-unverified" + p + "unverifiedx"
	db.Pool.Exec(ctx, "UPDATE users SET email = id || '@example.com' WHERE id = $1", unverifiedID)

	// Not an admin yet.
	if code, _ := call("thiago", "GET", "/api/admin/stats", nil); code != 403 {
		t.Fatalf("non-admin stats: %d", code)
	}
	// Only the owner manages admins.
	if code, _ := call("thiago", "POST", "/api/admin/admins", map[string]any{"who": uid("thiago") + "@example.com", "perms": []string{"money"}}); code != 403 {
		t.Fatalf("non-owner added an admin: %d", code)
	}
	staff := []string{"cups", "disputes", "withdrawals", "support", "users"}
	if code, out := call("owner", "POST", "/api/admin/admins", map[string]any{"who": strings.ToUpper(uid("thiago")) + "@EXAMPLE.COM", "perms": append(staff, "admins", "made_up")}); code != 200 {
		t.Fatalf("owner add: %d %v", code, out)
	}
	if perms := services.AdminPerms(ctx, uid("thiago")); strings.Join(perms, ",") != strings.Join(staff, ",") {
		t.Fatalf("stored perms %v (owner-only and unknown powers must be dropped)", perms)
	}
	if code, _ := call("owner", "POST", "/api/admin/admins", map[string]any{"who": "nobody-" + p + "@example.com", "perms": staff}); code != 404 {
		t.Fatalf("unknown account: %d", code)
	}
	call("owner", "POST", "/api/admin/admins", map[string]any{"who": uid("cupsonly"), "perms": []string{"cups"}})

	// Staff admin: allowed areas.
	for _, path := range []string{"/api/admin/stats", "/api/admin/users", "/api/admin/withdrawals", "/api/admin/support", "/api/admin/matches/review"} {
		if code, _ := call("thiago", "GET", path, nil); code != 200 {
			t.Fatalf("thiago %s: %d", path, code)
		}
	}
	// ...but no money, no revenue, no admin management.
	for _, path := range []string{"/api/admin/money", "/api/admin/transactions", "/api/admin/payout-balances", "/api/admin/admins"} {
		if code, _ := call("thiago", "GET", path, nil); code != 403 {
			t.Fatalf("thiago %s: %d (should be blocked)", path, code)
		}
	}
	if code, _ := call("thiago", "POST", "/api/admin/balance", map[string]any{"userId": uid("thiago"), "amount": 100000, "reason": "free money"}); code != 403 {
		t.Fatalf("thiago adjusted a wallet: %d", code)
	}
	_, out := call("thiago", "GET", "/api/admin/stats", nil)
	if st := out["stats"].(map[string]any); st["walletTotal"] != nil || st["feesToday"] != nil {
		t.Fatalf("revenue shown to staff admin: %v", st)
	}
	_, out = call("owner", "GET", "/api/admin/stats", nil)
	if st := out["stats"].(map[string]any); st["walletTotal"] == nil {
		t.Fatalf("owner can't see totals: %v", st)
	}
	_, out = call("thiago", "GET", "/api/me", nil)
	if pr := out["profile"].(map[string]any); pr["isAdmin"] != true || pr["isOwner"] == true {
		t.Fatalf("profile flags: %v", pr)
	}

	// Withdrawal limit for admins without the money power.
	big, small := p+"-wbig", p+"-wsmall"
	db.Pool.Exec(ctx, `INSERT INTO withdrawals (id, user_id, amount, currency, bank_code, account_number, status) VALUES
		($1,$3,6000000,'NGN','058','0123456789','pending'), ($2,$3,100000,'NGN','058','0123456789','pending')`, big, small, uid("cupsonly"))
	if code, _ := call("thiago", "POST", "/api/admin/withdrawals/"+big+"/mark-paid", map[string]string{"note": "REF123"}); code != 403 {
		t.Fatalf("staff marked a big withdrawal paid: %d", code)
	}
	if code, out := call("thiago", "POST", "/api/admin/withdrawals/"+small+"/mark-paid", map[string]string{"note": "REF124"}); code != 200 {
		t.Fatalf("staff mark small paid: %d %v", code, out)
	}
	if code, _ := call("owner", "POST", "/api/admin/withdrawals/"+big+"/mark-paid", map[string]string{"note": "REF125"}); code != 200 {
		t.Fatalf("owner mark big paid: %d", code)
	}

	// One power only: other areas stay closed (no power leaks between routes).
	if code, _ := call("cupsonly", "GET", "/api/admin/support", nil); code != 403 {
		t.Fatalf("cups-only admin saw complaints: %d", code)
	}
	if code, _ := call("cupsonly", "GET", "/api/admin/stats", nil); code != 200 {
		t.Fatalf("cups-only admin overview: %d", code)
	}

	// Alerts go only to admins with the matching power.
	services.NotifyAdmins(ctx, services.PermMoney, "Money alert "+p, "x")
	services.NotifyAdmins(ctx, services.PermSupport, "Support alert "+p, "x")
	var moneyAlerts, supportAlerts int
	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND title = $2", uid("thiago"), "Money alert "+p).Scan(&moneyAlerts)
	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND title = $2", uid("thiago"), "Support alert "+p).Scan(&supportAlerts)
	if moneyAlerts != 0 || supportAlerts != 1 {
		t.Fatalf("alerts: money %d support %d", moneyAlerts, supportAlerts)
	}

	// Admin powers need a verified email.
	if code, out := call("owner", "POST", "/api/admin/admins", map[string]any{"who": unverifiedID, "perms": staff}); code != 200 {
		t.Fatalf("add unverified: %d %v", code, out)
	}
	if code, _ := call("unverifiedx", "GET", "/api/admin/stats", nil); code != 403 {
		t.Fatalf("unverified admin got in: %d", code)
	}

	// Removing takes everything away; the log records it.
	if code, _ := call("owner", "DELETE", "/api/admin/admins/"+uid("thiago"), nil); code != 200 {
		t.Fatalf("remove: %d", code)
	}
	if code, _ := call("thiago", "GET", "/api/admin/stats", nil); code != 403 {
		t.Fatalf("removed admin still in: %d", code)
	}
	_, out = call("owner", "GET", "/api/admin/admins", nil)
	logText, _ := json.Marshal(out["log"])
	for _, want := range []string{"admin_set", "admin_removed", "withdrawal_marked_paid"} {
		if !strings.Contains(string(logText), want) {
			t.Fatalf("log missing %s: %s", want, logText)
		}
	}
}
