# BETELITE Authentication & Score Detection Implementation Guide

## Overview

This document outlines the implementation of:
1. **Google OAuth Authentication** - Automatic login with Google
2. **JWT-based Session Management** - Secure authentication tokens
3. **Bet Access Control** - SQL-based authentication check before betting
4. **AI Score Detection** - Automated score detection using OCR

---

## 1. Google OAuth Implementation

### Features
- One-click Google login
- Automatic account creation for new users
- Email verification through Google
- Session management with JWT tokens

### Setup

#### 1.1 Google Cloud Console Setup
1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create a new project
3. Enable Google+ API
4. Create OAuth 2.0 credentials (Web Application)
5. Add authorized redirect URIs:
   - `http://localhost:5000/api/auth/google/callback` (development)
   - `https://your-domain.com/api/auth/google/callback` (production)

#### 1.2 Environment Variables
Add to `.env`:
```bash
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URI=http://localhost:5000/api/auth/google/callback
```

#### 1.3 Frontend Integration
```html
<!-- Add Google Login Button -->
<a href="/api/auth/google" class="btn btn-google">
  <img src="google-logo.svg" alt="Google">
  Login with Google
</a>

<!-- Or use the traditional method -->
<form method="POST" action="/api/auth/login">
  <input type="email" name="email" required>
  <input type="password" name="password" required>
  <button type="submit">Login</button>
</form>
```

### API Endpoints

#### GET /api/auth/google
Redirects user to Google OAuth login

#### GET /api/auth/google/callback
Handles OAuth callback and creates/updates user in database

#### POST /api/auth/login
Traditional email/password login
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

#### POST /api/auth/register
Register new user
```json
{
  "email": "user@example.com",
  "username": "username",
  "password": "password123"
}
```

#### GET /api/auth/me
Get current user info (requires JWT token in Authorization header)

---

## 2. Bet Access Control

### Implementation

The system uses multiple layers of authentication:

1. **JWT Token Verification** - User must provide valid JWT token
2. **Account Status Check** - Account must be active (not banned/suspended)
3. **Email Verification** - User must verify email before betting
4. **KYC Verification** - Required for high-value bets

### Middleware Usage

```typescript
// In your bet routes file
import { requireBetAccess, requireKYCForHighBets, logBetActivity } from '../middleware/betAuth';
import authRoutes from '../routes/authV2';
import scoreRoutes from '../routes/scoresV2';

// Apply middleware to betting routes
app.use('/api/bets', requireBetAccess);
app.use('/api/bets', requireKYCForHighBets(1000)); // Max $1000 without KYC
app.use('/api/bets', logBetActivity);
app.post('/api/bets/place', placeBet);

// Apply auth routes
app.use('/api/auth', authRoutes);
app.use('/api/scores', scoreRoutes);
```

### SQL Schema Updates

The system uses the existing `users` table with these key fields:
- `email_verified` - BOOLEAN (must be true for betting)
- `kyc_verified` - BOOLEAN (for high-value bets)
- `account_status` - ENUM('active', 'suspended', 'banned', 'pending_verification')

### Authentication Flow

```
User Logs In (Google or Email/Password)
    ↓
JWT Token Generated
    ↓
User Attempts to Place Bet
    ↓
requireBetAccess Middleware Checks:
  - Valid JWT token?
  - Account status is 'active'?
  - Email verified?
  - For amounts > $1000: KYC verified?
    ↓
If All Checks Pass → Allow Bet
If Any Check Fails → Return 401/403 with specific error
```

---

## 3. AI Score Detection System

### Supported Games

#### Call of Duty 15-Player Tournament
- **Type**: 15 people team tournament
- **Detection**: Scans for "Call of Duty", "kills", "deaths", "score"
- **Data Extracted**: Player name, kills, deaths, score

#### 1v1 Games
- **Detection**: Scans for "1v1", "vs", "head to head"
- **Data Extracted**: Both player names, final score

#### Tournament Brackets
- **Detection**: Scans for "tournament", "bracket", "standings"
- **Data Extracted**: All players, their scores, match results

### How It Works

1. **Upload Screenshot** → Send to `/api/scores/detect`
2. **OCR Processing** → Tesseract extracts text from image
3. **Score Detection** → Pattern matching finds scores, player names, game type
4. **Confirmation** → User confirms detection is correct
5. **Board Update** → Scores automatically update in database

### API Endpoints

#### POST /api/scores/detect
Upload a single screenshot for score detection

```bash
curl -X POST http://localhost:5000/api/scores/detect \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "screenshot=@game-screenshot.png"
```

Response:
```json
{
  "success": true,
  "detection": {
    "gameType": "call_of_duty",
    "matchType": "15_player",
    "playerName": "ProPlayer123",
    "scores": {
      "player": 45,
      "opponent": 32
    },
    "kills": 45,
    "deaths": 12,
    "assists": 8,
    "confidence": 92
  }
}
```

#### POST /api/scores/detect-url
Detect scores from image URL

```json
{
  "imageURL": "https://example.com/screenshot.png"
}
```

#### POST /api/scores/detect-batch
Upload multiple screenshots at once

```bash
curl -X POST http://localhost:5000/api/scores/detect-batch \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "screenshots=@screenshot1.png" \
  -F "screenshots=@screenshot2.png" \
  -F "screenshots=@screenshot3.png"
```

#### POST /api/scores/{detectionId}/confirm
Confirm detection and update match/tournament

```json
{
  "matchId": "match_123",
  "tournamentId": "tournament_456",
  "updateBoard": true
}
```

#### GET /api/scores/detections
Get user's score detections

```bash
curl http://localhost:5000/api/scores/detections?limit=20&offset=0 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Setup OCR

#### Option 1: Tesseract (Recommended)

**Installation**

Windows:
```bash
# Download from: https://github.com/UB-Mannheim/tesseract/wiki
# Or use chocolatey
choco install tesseract
```

Linux/Mac:
```bash
sudo apt-get install tesseract-ocr  # Ubuntu/Debian
brew install tesseract              # Mac
```

Environment Variable:
```bash
# Add to .env
TESSERACT_PATH=/usr/bin/tesseract  # Linux/Mac
# or
TESSERACT_PATH=C:\\Program Files\\Tesseract-OCR\\tesseract.exe  # Windows
```

#### Option 2: EasyOCR

Already included in package.json. No additional setup needed.

### Score Detection Patterns

The system detects:

```
Call of Duty 15-Player Tournament:
- "Call of Duty" or "COD"
- Kills: 45, Deaths: 12
- Score: 45-32
- Player Name: ProPlayer123

Fortnite:
- "Fortnite"
- Eliminations: 8
- Placement: 1st
- Player name from HUD

Valorant:
- "Valorant"
- Agent names
- Round score: 13-8

League of Legends:
- "League of Legends"
- Gold: 15,234
- Turrets: 5
- KDA: 12/2/8
```

### Customizing Detection

Edit `backend/services/scoreDetection.ts`:

```typescript
// Add new game pattern
const GAME_PATTERNS: GamePattern[] = [
  {
    gameCode: 'your_game',
    gameName: 'Your Game Name',
    patterns: [
      /Your Game|YourGame/i,
      /specific_text_in_game/i,
    ],
  },
];

// Add new score pattern
const SCORE_PATTERNS = {
  scores: [
    /your_score_pattern_here/i,
  ],
  playerName: [
    /player_name_pattern/i,
  ],
};
```

---

## 4. Installation & Setup

### Prerequisites
- Node.js 16+ 
- MySQL 8.0+
- Firebase project
- Google Cloud project

### Backend Setup

```bash
cd backend

# Install dependencies
npm install

# Install Tesseract (if using OCR)
npm install tesseract.js

# Copy environment template
cp ../.env.example .env

# Edit .env with your credentials
nano .env

# Create uploads directory
mkdir -p ../../uploads/scores

# Start development server
npm run dev
```

### Database Setup

```bash
mysql -u root -p
source ../database/schema.sql
```

### Environment Variables

See `.env.example` for all required variables.

Key variables:
```bash
GOOGLE_CLIENT_ID=xxx
GOOGLE_CLIENT_SECRET=xxx
JWT_SECRET=your-super-secret-key
FIREBASE_DATABASE_URL=xxx
GMAIL_USER=your-email@gmail.com
GMAIL_PASSWORD=app-specific-password
```

---

## 5. Security Considerations

### Password Storage
- Passwords hashed with bcrypt (10 salt rounds)
- Never store plain text passwords
- Passwords minimum 8 characters

### JWT Tokens
- Tokens expire after 7 days
- Tokens include user ID, email, username, issue time
- Refresh token mechanism recommended for production

### Bet Access
- All bet endpoints require valid JWT
- IP address logging for fraud detection
- Account status checks prevent banned users from betting
- Email verification required
- High bet amounts require KYC

### OCR Data
- Images temporarily stored on server
- Images deleted after processing
- Raw OCR text stored for audit trail
- Confidence score included for validation

### Rate Limiting
- Recommended: 100 requests per 15 minutes
- Apply to `/api/auth` and `/api/bets` endpoints
- Prevent brute force attacks

---

## 6. Testing

### Test Login Flow
```bash
# Register
curl -X POST http://localhost:5000/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "username": "testuser",
    "password": "Test1234!"
  }'

# Login
curl -X POST http://localhost:5000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test1234!"
  }'

# Copy returned token and use for protected endpoints
```

### Test Score Detection
```bash
# Take a screenshot of a game
# Upload it for detection
curl -X POST http://localhost:5000/api/scores/detect \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -F "screenshot=@game.png"
```

---

## 7. Troubleshooting

### "No token provided" on protected endpoints
- Check Authorization header format: `Authorization: Bearer YOUR_TOKEN`
- Token must be from `/api/auth/login` or Google OAuth callback

### "Email not verified" for betting
- User must verify email before placing bets
- Send verification email in `/api/auth/register` response
- Check `email_verified` field in users table

### OCR Low Confidence
- Ensure screenshot is clear and in focus
- Game UI must be visible with readable text
- Check OCR_CONFIDENCE_THRESHOLD in .env
- Try different screenshot angles or brightness

### Google OAuth Redirect Error
- Verify GOOGLE_REDIRECT_URI in .env matches Google Cloud console
- Check FRONTEND_URL is set correctly
- Clear browser cache and try again

---

## 8. Next Steps

1. **Email Verification** - Implement email verification links
2. **Password Reset** - Add forgot password functionality
3. **2FA** - Two-factor authentication for accounts
4. **Advanced OCR** - Use EasyOCR for better accuracy
5. **Real-time Updates** - WebSocket for live score updates
6. **Webhook Integration** - API for game score providers
7. **Mobile App** - React Native app for iOS/Android

---

## Support

For issues or questions:
1. Check logs in `./logs/app.log`
2. Review console output for error messages
3. Verify environment variables are set correctly
4. Check database connection and permissions
5. Test endpoints with provided curl examples

Last Updated: May 1, 2026
