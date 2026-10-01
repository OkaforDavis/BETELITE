# CrestArena

Competitive mobile football (EA SPORTS FC Mobile, eFootball, Dream League Soccer) with real-money 1v1 challenges and tournaments, AI-verified results and automatic payouts. 18+, Nigeria and Ghana.

## Structure

```
backend/            Go (Fiber) server — API, WebSocket, automation, OCR, serves the PWA
  db/               PostgreSQL connection + versioned migrations
  middleware/       Firebase auth, CORS, rate limits
  routes/           HTTP handlers (me, wallet, lobby, matches, tournaments, admin, …)
  services/         Business logic (matches, OCR, tournaments, wallet, Paystack, notifications)
  ws/               WebSocket hub
  static/           PWA: index.html, css/, js/ (ES modules, no build step), sw.js, legal/
brand/              Source logo files
render.yaml         Render blueprint
```

## How a match works

1. A player posts a challenge (stake held in escrow) or joins a tournament.
2. When matched, a **match** is created with an ID tied to both players.
3. After playing, either player uploads the full-time screenshot **for that match ID**.
   Gemini checks it is a final, player-vs-player result screen, reads names and scores,
   and the server matches the names to both players' saved Game IDs. Screenshots and
   in-game match IDs can only be used once.
4. The opponent has **15 minutes** to confirm or dispute. Then it settles automatically:
   1v1 winner gets 80% of the pot (draw refunds); tournaments update the table/bracket
   and pay prizes when finished (knockout 70% to the winner; league 30/10/4.5/4.5 by default).
5. Disputes and overdue matches go to the admin review queue.

## Run locally

Needs Go 1.26+ and PostgreSQL.

```bash
cd backend
cp .env.example .env   # or create .env with the variables below
go run .
```

Without `FIREBASE_PROJECT_ID` and outside production, the server accepts test logins:
open `http://localhost:3000/?dev=alice` to sign in as a test user called alice
(set `ADMIN_EMAIL=dev-admin@example.com` and use `?dev=admin` for the admin).

Tests: `go test ./...`

## Environment variables

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string |
| `GO_ENV` | `production` on Render (disables test logins) |
| `FIREBASE_PROJECT_ID`, `FIREBASE_SERVICE_ACCOUNT_JSON` | Sign-in verification |
| `GEMINI_API_KEY`, `GEMINI_MODEL` | Screenshot reading (default model `gemini-3.8-flash`) |
| `PAYSTACK_PUBLIC_KEY`, `PAYSTACK_SECRET_KEY` (+ `_GH`) | Deposits and withdrawals |
| `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY` | Web Push notifications |
| `ADMIN_EMAIL` | Verified email that gets admin access |
| `APP_URL` | Public URL (Paystack return link) |
| `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`, `LIVEKIT_URL` | Live streams |
| `FORCE_UPDATE` | `true` forces every installed app to update before continuing |

## Releases and app updates

Every deploy stamps the git commit into `index.html` and `sw.js`. Installed apps detect
the new service worker (and poll `/api/version`) and show **"New version available — Update"**.
Set `FORCE_UPDATE=true` for a release that must be installed before anyone can keep playing.

Paystack webhook URL: `https://<your-app>/api/payments/webhook`
