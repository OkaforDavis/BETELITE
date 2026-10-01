# BETELITE

Competitive esports platform with real-money wagering, tournaments, and AI-powered match verification.

## Project Structure

```
BETELITE/
├── backend/              # Go (Fiber) API server with native Gemini OCR
│   ├── config/           # Environment and app configuration
│   ├── db/               # PostgreSQL connection and migrations
│   ├── middleware/        # Auth, CORS, rate limiting
│   ├── models/           # Data models (match, user, escrow, etc.)
│   ├── routes/           # HTTP route handlers
│   ├── services/         # Business logic (OCR, engine, escrow, automation)
│   ├── static/           # PWA frontend files
│   ├── utils/            # Response helpers, ID generation
│   ├── ws/               # WebSocket hub and client handlers
│   └── Dockerfile        # Single-stage Go build
├── mobile/               # PWA frontend (single-page app)
├── render.yaml           # Render deployment blueprint
└── docker-compose.yml    # Local development
```

## Quick Start

### Backend (Go)
```bash
cd backend
cp .env.example .env  # fill in your keys
go run .
```

### Required Environment Variables
```bash
DATABASE_URL=postgres://...          # PostgreSQL connection string
GEMINI_API_KEY=your_gemini_key       # Google AI key for score detection (OCR)
FIREBASE_SERVICE_ACCOUNT_JSON=...    # Firebase auth service account
PAYSTACK_SECRET_KEY=...              # Paystack payment processing
```

### Docker
```bash
docker-compose up --build
```

## Deployment

Deployed on [Render](https://render.com) via `render.yaml` Blueprint.

- **Backend**: Docker (single Go binary, serves frontend + API + OCR)

Set environment variables in the Render dashboard:
- `DATABASE_URL` — PostgreSQL connection string
- `GEMINI_API_KEY` — Google AI key for native score detection
- `FIREBASE_SERVICE_ACCOUNT_JSON` — Firebase auth
- `PAYSTACK_SECRET_KEY` — Payment processing

## Tech Stack

- **Backend**: Go + Fiber + PostgreSQL + WebSocket
- **Frontend**: Vanilla JS PWA (single HTML file)
- **AI Detection**: Native Go → Gemini 2.0 Flash vision (structured JSON output)
- **Auth**: Firebase Authentication
- **Payments**: Paystack (NG + GH multi-currency)
- **Streaming**: LiveKit WebRTC

## Architecture

### AI Score Detection (OCR)
The platform uses Google's Gemini 2.0 Flash model with vision capabilities to
automatically detect game scores from screenshots. This runs **natively in Go**
using the `google.golang.org/genai` SDK — no separate Python service required.

The detection pipeline:
1. Player uploads a game screenshot
2. Image is sent to Gemini vision with structured JSON output schema
3. AI returns detected scores, gamertags, and game type
4. Engine verifies and settles the match (escrow payout)

### Background Automation
- **Match timeout**: P2P matches auto-expire after 2 hours with escrow refund
- **Challenge cleanup**: Waiting challenges auto-cancel after 24 hours
- **Engine ticker**: Simulated matches progress every 30 seconds
