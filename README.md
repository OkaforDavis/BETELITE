# ⚽ BETELITE

**Mobile Gaming & Betting Platform**

Real-time gaming tournaments with AI-powered detection, live betting, spectating, and instant chat.

---

## 🚀 Quick Start

### Using Docker (Recommended)

```bash
git clone https://github.com/OkaforDavis/BETELITE.git
cd BETELITE
docker-compose up
```

Then open your browser:
- **Mobile App**: http://localhost:3000/mobile/
- **API**: http://localhost:3000/api/
- **Detection**: http://localhost:5000/

### Manual Setup

**Terminal 1 - Backend:**
```bash
cd backend
npm install
npm run dev
```

**Terminal 2 - Detection Service:**
```bash
cd detection_service
pip install -r requirements.txt
python app.py
```

**Open**: http://localhost:3000/mobile/

---

## ✨ Features

- 📱 **Mobile-First Design** - Optimized for phones, tablets, and desktop
- ⚽ **Multi-Game Support** - eFootball, Dream League Soccer (more coming)
- 💬 **Live Chat** - Real-time WebSocket communication
- 💰 **Betting Pools** - Place bets on tournaments
- 👁️ **Spectator Mode** - Watch live matches
- 🤖 **AI Detection** - Python-powered goal/offside detection
- 🔐 **Secure** - JWT authentication, rate limiting, CORS protection

---

## 📁 Project Structure

```
BETELITE/
├── mobile/              # Frontend web app
│   ├── index.html      # Main app
│   ├── css/            # Responsive styles
│   └── js/             # Game logic
│
├── backend/            # Node.js API
│   ├── src/
│   │   ├── server.ts   # Express server
│   │   └── services/
│   └── package.json
│
├── detection_service/  # Python AI
│   ├── app.py
│   └── requirements.txt
│
├── database/           # Schema
│   └── schema.sql
│
└── docker-compose.yml  # All services
```

---

## 🎮 How to Use

### Launch App
Visit: http://localhost:3000/mobile/

### Games
- Select **eFootball** or **Dream League Soccer**
- Join a tournament
- Watch real-time matches with AI detection

### Betting
- Place bets on match outcomes
- View odds and predictions
- Track your winnings

### Spectating
- Watch live matches
- Chat with other spectators
- See AI detection events (goals, offsides)

---

## 🛠️ Technology Stack

- **Frontend**: HTML5, CSS3, JavaScript (Vanilla)
- **Backend**: Node.js, Express, TypeScript
- **Real-time**: WebSocket (Socket.IO)
- **Database**: PostgreSQL, Prisma ORM
- **Cache**: Redis
- **Detection**: Python, Flask, OpenCV
- **Deployment**: Docker, Docker Compose

---

## 📱 Mobile Access

### On Your Phone

**Same Network (WiFi):**
```
Find your computer IP, then visit:
http://<YOUR_IP>:3000/mobile/
```

**Add to Home Screen:**
- iOS: Safari → Share → Add to Home Screen
- Android: Chrome → Menu → Install app

### Over Internet
Use ngrok or deploy to a server

---

## 📖 Documentation

- [Mobile README](MOBILE_README.md) - Full setup & features guide
- [Architecture](ARCHITECTURE.md) - System design details
- [GitHub](https://github.com/OkaforDavis/BETELITE) - Source code

---

## 🔐 Security

- ✅ HTTPS in production
- ✅ JWT token authentication
- ✅ WebSocket token validation
- ✅ Input sanitization
- ✅ Rate limiting
- ✅ CORS protection
- ✅ SQL injection prevention

---

## 🚀 Next Phase

- [ ] Live video streaming (Ant Media Server)
- [ ] Screen sharing for spectators
- [ ] Admin dashboard
- [ ] Advanced AI detection
- [ ] In-app payments
- [ ] Native mobile apps
- [ ] More games (FIFA, PES, etc.)

---

## 📊 Status

- ✅ Version: 1.0.0
- ✅ Status: Production Ready
- ✅ License: MIT
- ✅ Last Updated: March 2026

---

## 💡 Support

- GitHub Issues: [Report Bugs](https://github.com/OkaforDavis/BETELITE/issues)
- Documentation: [Full Docs](MOBILE_README.md)

---

**Made with ⚽ for gamers, by gamers**

[View on GitHub](https://github.com/OkaforDavis/BETELITE) | [Open Mobile App](http://localhost:3000/mobile/)
