# BETELITE Mobile - Setup & Run Guide

## Option 1: Run on GitHub Pages (Instant, No Setup)

Simply visit: **https://okafordavis.github.io/BETELITE/**

Click **"Launch App"** to play with demo data (tournaments, betting, spectating).

✅ No installation needed
✅ Works on any phone with a browser
✅ Full demo functionality included

---

## Option 2: Run with Docker (Full Backend + Mobile)

Run the complete platform locally and access it from your phone.

### Prerequisites
- **Docker** installed ([Download](https://www.docker.com/products/docker-desktop))
- Phone on same WiFi network as your computer

### Step 1: Start Docker Services

```bash
cd c:\Users\DELL\Desktop\betelite

# Start all services (backend, database, Redis)
docker-compose up --build
```

Wait for output showing:
```
backend   | ✓ Server running on http://localhost:3000
postgres  | ready to accept connections
redis     | Ready to accept connections
```

### Step 2: Find Your Computer's IP Address

**On Windows PowerShell:**
```powershell
ipconfig
```

Look for **IPv4 Address** (usually `192.168.x.x` or `10.0.x.x`)

**Example:** `192.168.1.100`

### Step 3: Access from Your Phone

On your phone's browser, go to:
```
http://YOUR_COMPUTER_IP:3000/mobile
```

**Example:** `http://192.168.1.100:3000/mobile`

### Step 4: Play the Game

- ✅ See live tournaments
- ✅ Place real bets (with demo wallet)
- ✅ Watch live matches
- ✅ Real-time chat
- ✅ Goal/Offside AI detection events

---

## Option 3: Direct Mobile Access (Without Containers)

### Prerequisites
- Node.js 16+ installed
- PostgreSQL running locally
- Redis running locally

### Step 1: Install Dependencies

```bash
cd backend
npm install
```

### Step 2: Setup Database

```bash
# Create database
psql -U postgres -c "CREATE DATABASE betelite"

# Run schema
psql -U postgres -d betelite < ../database/schema.sql
```

### Step 3: Start Backend Server

```bash
cd backend
npm run dev
```

Server will start on `http://localhost:3000`

### Step 4: Access from Phone

Follow **Step 2-4** from Option 2 above.

---

## Troubleshooting

### Mobile can't connect to backend

1. **Check Backend is Running**
   ```bash
   curl http://localhost:3000/health
   ```
   Should return `OK`

2. **Check IP Address**
   - Don't use `localhost` - use your actual computer IP
   - Phone and computer must be on SAME WiFi network
   - Try: `http://192.168.1.100:3000` (replace with your IP)

3. **Check Firewall**
   - Windows might block port 3000
   - Add exception: Settings > Firewall > Allow app through firewall
   - Or run: `netsh advfirewall firewall add rule name="Node" dir=in action=allow program="C:\Path\To\node.exe"`

### Mobile shows demo data only

This means it's in demo mode (no backend connection):
- ✅ All features work with mock data
- ✅ You can still play tournaments & place bets
- ⚠️ Data doesn't persist after refresh
- To fix: Ensure backend is running and accessible

### Docker won't start

```bash
# Check if port 3000 is in use
netstat -ano | findstr :3000

# Kill the process (replace PID with actual process ID)
taskkill /PID <PID> /F

# Try again
docker-compose up --build
```

---

## Mobile Features Checklist

- ✅ **Responsive Design** - Works on all phone sizes
- ✅ **Game Tournaments** - eFootball & Dream League Soccer
- ✅ **Betting System** - Place bets with odds
- ✅ **Live Spectating** - Watch matches in real-time
- ✅ **Real-time Chat** - Chat during matches
- ✅ **AI Detection** - Goal/Offside detection events
- ✅ **Wallet System** - ₦50,000 starting balance
- ✅ **PWA Ready** - Can be installed as app on phone

---

## How Backend Differs from Demo Mode

| Feature | GitHub Pages (Demo) | Docker/Backend |
|---------|-------------------|-----------------|
| Data | Mock tournaments | Real from database |
| Persistence | No | Yes (PostgreSQL) |
| Multiplayer | No | Yes (WebSocket) |
| Chat | No | Yes (Real-time) |
| Wallet | Demo (₦50K) | Real (Database) |
| Setup | None | Docker required |
| Speed | Instant | Slightly slower |

---

## Quick Start Commands

### GitHub Pages (Instant)
```
Just open: https://okafordavis.github.io/BETELITE/
```

### Docker Full Stack
```bash
cd c:\Users\DELL\Desktop\betelite
docker-compose up --build
# Then visit: http://YOUR_IP:3000/mobile
```

### Backend Only (No Containers)
```bash
cd backend
npm install && npm run dev
# Then visit: http://YOUR_IP:3000/mobile
```

---

## For Production

To deploy BETELITE to the cloud:

1. **Heroku/Railway** - Push Docker image
2. **AWS/GCP** - Deploy backend + database
3. **Vercel** - Host mobile app as static site

See `DEPLOYMENT.md` for detailed instructions.
