# 📱 BETELITE - Mobile Gaming & Betting Platform

A real-time mobile-first web app for gaming tournaments (eFootball, Dream League Soccer) with live betting, spectating, and AI-powered goal detection.

## 🎯 Features

- **📱 Mobile-First Design** - Optimized for smartphones, tablets, and desktop
- **⚽ Multi-Game Support** - eFootball, Dream League Soccer (DLS), FIFA (coming soon)
- **💬 Live Chat** - Real-time WebSocket-powered chat without page refresh
- **👁️ Spectating** - Watch live matches and tournaments
- **💰 Betting Pools** - Place bets on tournament outcomes
- **🤖 AI Detection** - Python-powered goal and offside detection
- **🔴 Live Streaming** - Screen sharing and live broadcast for spectators
- **📊 Tournament Management** - Admin dashboard for tournament oversight

## 📋 Quick Start

### Requirements

- **Node.js** 18+
- **Docker & Docker Compose** (for containerized setup)
- **Python** 3.11+ (for detection service)
- **PostgreSQL** 15+ (or use Docker)

### Option 1: Docker (Recommended)

```bash
# Clone and navigate to project
git clone https://github.com/OkaforDavis/BETELITE.git
cd BETELITE

# Copy environment files
cp .env.example .env
cp backend/.env.example backend/.env

# Start all services
docker-compose up

# Services will be available at:
# - Mobile App: http://localhost:3000/mobile/
# - API: http://localhost:3000/api/
# - Detection: http://localhost:5000/
```

### Option 2: Local Development

#### 1. Start Backend

```bash
cd backend
npm install
npm run dev
# Runs on http://localhost:3000
```

#### 2. Start Detection Service

```bash
cd detection_service
pip install -r requirements.txt
python app.py
# Runs on http://localhost:5000
```

#### 3. Open Mobile App

Open your browser to: **http://localhost:3000/mobile/**

On mobile, you can add to home screen:
- **iOS**: Safari → Share → Add to Home Screen
- **Android**: Chrome → Menu → Install app

## 🚀 Deploy to Production

### Push to GitHub

```bash
# Add all changes
git add .

# Commit
git commit -m "Add mobile app with real-time features"

# Push to your repository
git push origin main
```

### Access on Phone

1. **Same Network (WiFi)**:
   - Find your computer's IP: `ipconfig getifaddr en0` (Mac) or `hostname -I` (Linux)
   - On phone, visit: `http://<YOUR_IP>:3000/mobile/`

2. **Different Network (Deploy to Server)**:
   - Use a VPS (AWS, DigitalOcean, Railway, Render)
   - Deploy using Docker
   - Access via domain name

3. **Ngrok (Quick Testing)**:
   ```bash
   # Install ngrok
   ngrok http 3000
   # Visit the provided URL on your phone
   ```

## 📁 Project Structure

```
BETELITE/
├── mobile/                 # Mobile-first web app
│   ├── index.html         # Main app interface
│   ├── css/               # Responsive styling
│   │   ├── mobile.css     # Mobile-first styles
│   │   └── responsive.css # Tablet/desktop responsive
│   ├── js/                # Application logic
│   │   ├── config.js      # Configuration
│   │   ├── api.js         # API client
│   │   ├── socket.js      # WebSocket client
│   │   ├── games/         # Game modules
│   │   │   ├── efootball.js
│   │   │   └── dls.js
│   │   ├── ui.js          # UI controller
│   │   └── main.js        # App initialization
│   └── manifest.json      # PWA manifest

├── backend/               # Node.js + TypeScript API
│   ├── src/
│   │   ├── server.ts      # Main server
│   │   ├── services/
│   │   │   └── socketManager.ts  # WebSocket handling
│   │   ├── config/        # Configuration
│   │   ├── middleware/    # Express middleware
│   │   └── utils/         # Utilities
│   └── package.json

├── detection_service/     # Python detection service
│   ├── app.py            # Flask app with AI
│   ├── requirements.txt
│   └── Dockerfile

├── database/             # Database schema
│   └── schema.sql

├── docker-compose.yml    # Docker orchestration
└── .env.example          # Environment template
```

## 🎮 How It Works

### 1. **User Flow**

```
User → Mobile App → WebSocket Connection → Backend → Database
                          ↓
                    Real-time Updates
                  (Chat, Matches, Bets)
```

### 2. **Game Detection Flow**

```
Match Start → Screen Capture → Detection Service → Event Analysis
                                      ↓
                            Goal/Offside Detected
                                      ↓
                              WebSocket Event
                                      ↓
                            Update All Spectators
```

### 3. **Betting Flow**

```
User Places Bet → Validate → Deduct Wallet → Create Bet Record
                                     ↓
                            Match Ends
                                     ↓
                            Calculate Results
                                     ↓
                            Transfer Winnings
```

## 🎮 Using the App

### Live Tournaments
1. Tap **Tournaments** tab
2. Select game (eFootball or DLS)
3. Choose tournament and join
4. Watch live match or compete

### Betting
1. Tap **Bets** tab
2. View active betting pools
3. Tap "Place Bet"
4. Enter amount and prediction
5. Confirm bet

### Spectating
1. Tap **Watch** tab
2. Choose live match to spectate
3. Join live chat
4. See real-time detection events (goals, offsides)

### Account
1. Tap **Account** tab
2. View balance, winnings, history
3. Deposit/Withdraw funds
4. Manage profile

## ⚙️ Configuration

### Environment Variables

```env
# Backend
NODE_ENV=development
PORT=3000
FRONTEND_URL=http://localhost:3000
DATABASE_URL=postgresql://user:pass@localhost/betelite
REDIS_URL=redis://localhost:6379
JWT_SECRET=your-secret-key

# Detection Service
FLASK_ENV=development
```

### API Endpoints

```
GET  /api/health              # Health check
GET  /api/tournaments         # List tournaments
GET  /api/matches/:id         # Match details
GET  /api/bets/active         # Active betting pools
POST /api/bets                # Place bet
GET  /api/user/profile        # User profile
```

### WebSocket Events

```javascript
// From client
socket.emit('join_match', { matchId })
socket.emit('chat_message', { matchId, message })
socket.emit('place_bet', { matchId, amount, prediction })

// From server
socket.on('match_update', (data) => {})
socket.on('goal_detected', (data) => {})
socket.on('offside_detected', (data) => {})
socket.on('chat_message', (data) => {})
```

## 🔐 Security

- ✅ HTTPS in production (via Docker/reverse proxy)
- ✅ JWT authentication
- ✅ Rate limiting on API
- ✅ WebSocket token validation
- ✅ Input sanitization
- ✅ CORS protection
- ✅ SQL injection prevention (Prisma ORM)

## 📊 Admin Dashboard (Coming Soon)

- Monitor live matches
- Oversee tournaments
- Review betting pools
- Detect cheating via AI analysis
- View user statistics

## 🐛 Troubleshooting

### WebSocket Connection Failed
- Check backend is running: `http://localhost:3000/api/health`
- Verify no firewall blocking port 3000
- Check browser console for errors

### Detection Service Not Working
- Check Python service: `http://localhost:5000/health`
- Verify Flask running: `python detection_service/app.py`
- Check network connectivity

### Mobile App Not Loading
- Clear browser cache
- Check `http://localhost:3000/mobile/` URL
- Verify service worker: DevTools → Application → Service Workers

### Database Connection Error
- Ensure PostgreSQL running
- Check `DATABASE_URL` environment variable
- Run migrations: `npm run prisma:migrate`

## 📱 Mobile Tips

### Add as PWA (Web App)

**iOS (Safari)**:
1. Open app in Safari
2. Tap Share icon
3. Scroll and tap "Add to Home Screen"
4. Name and add

**Android (Chrome)**:
1. Open app in Chrome
2. Tap Menu (three dots)
3. Tap "Install app"
4. Confirm

### Better Performance
- Use WiFi for video streaming
- Close other apps to free RAM
- Enable notifications for match alerts
- Keep app in PWA mode (full screen)

## 🚀 Next Phase - Roadmap

- [ ] Live video streaming (Ant Media Server)
- [ ] Screen sharing for spectators
- [ ] Replay system
- [ ] Admin panel
- [ ] Advanced AI detection (player tracking, heat maps)
- [ ] In-app payments integration
- [ ] Native mobile apps (React Native)
- [ ] More games (FIFA, PES, etc.)

## 📞 Support

- GitHub Issues: [Report Bugs](https://github.com/OkaforDavis/BETELITE/issues)
- Documentation: Check ARCHITECTURE.md for technical details

## 📄 License

MIT License - see LICENSE file

---

**Made with ⚽ for gamers, by gamers**

Happy gaming! 🎮

