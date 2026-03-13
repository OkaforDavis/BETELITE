# BETELITE — Complete Setup Guide

## What this backend gives you

| Feature | How it works |
|---|---|
| Live match scores | Game engine ticks every 30s, Socket.io pushes to all clients instantly |
| Match endings | Football: 90min + added time → "Updating scores..." → FT. COD: first to 6 kills → ends |
| AI score detection | Screenshot upload → Claude Vision reads score, teams, time, detects cheats |
| AI stream scanning | WebRTC screen share → frames sent to Claude every 5s → score auto-updates |
| Player score submit | Player taps "Submit Score" → server validates → broadcasts to all watchers |
| Live streaming | WebRTC P2P via Socket.io signaling — no external service needed |
| Real football data | API-Football free tier (100 calls/day) polled every 15s |
| Bet settlement | Auto-resolves when match ends, wallet updated in Firebase |
| Anti-cheat | Score jump detection, stat anomaly checks, AI visual verification |

---

## Step 1 — Get your API keys (5 minutes)

### A) Anthropic API Key (AI score detection)
1. Go to https://console.anthropic.com/
2. Sign in → API Keys → Create Key
3. Copy the key (starts with `sk-ant-...`)

### B) API-Football Key (real football scores — optional)
1. Go to https://dashboard.api-football.com/register
2. Free tier: 100 calls/day (enough for testing)
3. Copy the key

### C) Firebase (real-time database)
1. Go to https://console.firebase.google.com/
2. Create project → call it "betelite"
3. Go to Realtime Database → Create database → Start in test mode
4. Go to Project Settings → Service Accounts → Generate new private key
5. Download the JSON file

---

## Step 2 — Local setup

```bash
# Clone your repo
git clone https://github.com/OkaforDavis/BETELITE.git
cd BETELITE

# Go into backend
cd backend

# Copy env template
cp .env.example .env

# Edit .env with your keys
nano .env   # or open in your editor
```

Fill in `.env`:
```
ANTHROPIC_API_KEY=sk-ant-YOUR_KEY_HERE
API_FOOTBALL_KEY=YOUR_FOOTBALL_KEY
FIREBASE_PROJECT_ID=betelite-xxxxx
FIREBASE_DATABASE_URL=https://betelite-xxxxx-default-rtdb.firebaseio.com
FIREBASE_SERVICE_ACCOUNT_JSON={"type":"service_account","project_id":"...paste entire JSON here as one line..."}
```

```bash
# Install dependencies
npm install

# Start backend
npm run dev
```

Open http://localhost:3000/mobile — you should see BETELITE with "● Connected to BETELITE Live"

---

## Step 3 — Test live features

### Test AI score detection:
1. Take a screenshot of any mobile game (eFootball, COD, etc.)
2. Open http://localhost:3000/mobile
3. Go to Live tab → tap the AI scanner → upload screenshot
4. You should see: game detected, teams, score, time

### Test live score updates:
- Scores update automatically every 10 seconds
- Watch the match cards — scores change in real time
- Check the mini strip at top updates too

### Test match endings:
- Football matches run to 90' then enter added time (90+1, 90+2, 90+3)
- After added time: card shows "Updating scores..." briefly
- Then shows "✅ FULL TIME — HOME WIN" (or AWAY/DRAW)
- Your bets auto-settle and wallet updates

### Test screen share streaming:
1. Go to Watch tab → tap "Go Live — Share Your Screen"
2. Allow screen capture
3. Open a second browser tab to http://localhost:3000/mobile
4. Go to Watch tab → tap Watch Now on any match
5. The stream appears in the second tab

### Test player score submit:
1. Go to Live tab → tap "Submit My Game Score"
2. Select a match, enter scores, select "Game ended"
3. All connected clients see the update instantly

---

## Step 4 — Deploy to GitHub for live testing

```bash
# From project root
cd mobile
# mobile/index.html is your frontend — it auto-detects backend URL

# Push everything
git add .
git commit -m "Full backend + AI integration"
git push origin main
```

For the backend, deploy to **Render.com** (free):
1. Go to https://render.com → New Web Service
2. Connect your GitHub repo → select `/backend` as root
3. Build: `npm install` | Start: `node server.js`
4. Add environment variables (paste from your .env)
5. Copy your Render URL (e.g. `https://betelite-backend.onrender.com`)

Update `mobile/index.html` line ~12:
```js
return window.BETELITE_BACKEND || 'https://betelite-backend.onrender.com';
```

GitHub Pages (frontend only — works for testing UI):
- Settings → Pages → Deploy from main → /mobile folder
- Your frontend at `https://okafordavis.github.io/BETELITE/mobile/`
- Will work in demo mode without backend, full features with backend

---

## Step 5 — Firebase Studio deployment

Since you're in Firebase Studio (IDX):
```bash
# Terminal 1 — Backend
cd backend && npm install && npm run dev

# Terminal 2 — the mobile frontend is served by the backend
# Open the preview URL Firebase Studio gives you
```

For Firebase Hosting:
```bash
npm install -g firebase-tools
firebase login
firebase init hosting
# Public dir: mobile
# Single page app: No
firebase deploy
```

---

## File structure

```
BETELITE/
├── backend/
│   ├── server.js          ← Main Express + Socket.io server
│   ├── services/
│   │   ├── gameEngine.js  ← Match logic, time, endings, AI
│   │   ├── firebase.js    ← Firebase connection
│   │   └── footballAPI.js ← API-Football integration
│   ├── routes/
│   │   ├── matches.js     ← GET/POST match data
│   │   ├── tournaments.js ← Join, bracket, prizes
│   │   ├── bets.js        ← Place/retrieve bets
│   │   ├── detect.js      ← AI screenshot endpoint
│   │   ├── stream.js      ← Stream rooms
│   │   └── football.js    ← Real football data
│   ├── .env.example       ← Copy to .env
│   ├── package.json
│   └── Dockerfile
├── mobile/
│   └── index.html         ← Complete mobile frontend
├── docker-compose.yml
└── SETUP.md               ← This file
```

---

## API Reference

| Method | Endpoint | Description |
|---|---|---|
| GET | /health | Server status |
| GET | /api/matches | All live matches |
| GET | /api/matches/:id | Single match |
| POST | /api/matches/:id/submit | Player submits score |
| GET | /api/tournaments | All tournaments |
| POST | /api/tournaments/:id/join | Join tournament |
| POST | /api/bets/place | Place a bet |
| POST | /api/detect/screenshot | AI screenshot analysis |
| POST | /api/detect/frame | AI stream frame analysis |
| GET | /api/football/live | Real football scores |

## Socket.io Events

| Event | Direction | Description |
|---|---|---|
| identify | client→server | Register user |
| join_match | client→server | Subscribe to match updates |
| submit_score | client→server | Player submits score |
| stream_start | client→server | Host starts stream |
| stream_join | client→server | Viewer joins stream |
| stream_frame | client→server | Send frame for AI analysis |
| match_tick | server→client | Score/time update (every 30s) |
| score_update | server→client | Score changed |
| match_finished | server→client | Match ended with result |
| score_updating | server→client | "Updating scores..." state |
| goal_scored | server→client | Goal event broadcast |
| cheat_detected | server→client | Anti-cheat flag |
| bet_won | server→client | User's bet won |
| webrtc_offer/answer/ice | both | WebRTC signaling |
