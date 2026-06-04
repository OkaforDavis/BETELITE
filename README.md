# BETELITE

Competitive esports platform with real-money wagering, tournaments, and AI-powered match verification.

## Project Structure

```
BETELITE/
├── backend/              # Go (Fiber) API server
├── detection_service/    # Python FastAPI — LLM vision OCR for score detection
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

### Detection Service (Python)
```bash
cd detection_service
pip install -r requirements.txt
export GEMINI_API_KEY=your_key
uvicorn app:app --port 5000
```

### Docker (both services)
```bash
docker-compose up --build
```

## Deployment

Deployed on [Render](https://render.com) via `render.yaml` Blueprint.

- **Backend**: Docker (Go binary)
- **Detection Service**: Docker (Python FastAPI)

Set environment variables in the Render dashboard:
- `DATABASE_URL` — PostgreSQL connection string
- `GEMINI_API_KEY` — Google AI key for score detection
- `FIREBASE_SERVICE_ACCOUNT_JSON` — Firebase auth
- `PAYSTACK_SECRET_KEY` — Payment processing

## Tech Stack

- **Backend**: Go + Fiber + PostgreSQL + WebSocket
- **Frontend**: Vanilla JS PWA (single HTML file)
- **AI Detection**: FastAPI + OpenAI-compatible LLM vision (Gemini/OpenAI/Groq)
- **Auth**: Firebase Authentication
- **Payments**: Paystack
