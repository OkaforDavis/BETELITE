# BETELITE Authentication & Admin System - Testing Guide

## Quick Start

### 1. Backend Setup
```bash
cd backend
npm install  # Already done - packages: bcrypt, jsonwebtoken, nodemailer
cp .env.example .env
# Edit .env with your values:
# - JWT_SECRET=your-secret-key
# - GMAIL_USER=your-email@gmail.com (for password reset emails)
# - GMAIL_PASSWORD=app-specific-password
# - FRONTEND_URL=http://localhost:3000
```

### 2. Start Backend Server
```bash
npm run dev  # or: node server.js
# Server runs on http://localhost:3000
```

### 3. Test Frontend
```bash
# Open in browser:
# http://localhost:3000/mobile/index.html (React-like)
# or access the mobile app directly
```

---

## Authentication Flow Testing

### Test User (Admin)
**Email:** okafordavis8@gmail.com  
**Password:** Create via "Forgot Password" flow on first login

### Test Case 1: Register New User
1. Open mobile app
2. Click "Create one" (switch to register)
3. Fill: Email, Username, Password (min 6 chars)
4. Submit → Creates account in Firebase with:
   - Wallet: {NGN: 0, GHS: 0}
   - Role: "player"
   - Empty stats/achievements
5. Auto-login and shows empty state

### Test Case 2: Login Flow
1. Email: user@example.com
2. Password: correct-password
3. Submit → Returns JWT token (7-day expiry)
4. Token stored in localStorage: `betelite_token`
5. User profile auto-loads

### Test Case 3: Forgotten Password
1. Click "Forgot password?"
2. Enter registered email
3. Check email for reset link (sent via Gmail SMTP)
4. Reset link format: `http://localhost:3000/mobile/index.html?reset_token=...`
5. Auto-fills reset form with new password
6. New password hashed and stored in Firebase
7. Auto-redirect to login

### Test Case 4: Admin Login
1. Use email: okafordavis8@gmail.com
2. After first login, role is set to "admin"
3. Profile shows: "👨‍💼 Admin" badge
4. Can access admin endpoints

---

## Admin Configuration Panel

### Enable Payment Gateway
```bash
# API Endpoint (admin only):
POST /api/admin/config/payment
Headers:
  x-user-id: <admin-user-id>
  authorization: Bearer <admin-token>

Body:
{
  "paystack_enabled": true,
  "flutterwave_enabled": false
}
```

### Frontend Response
- **If payment enabled**: Deposit modal shows input field
- **If payment disabled**: Shows error "⚠️ Payment not yet available"

### Get Current Config
```bash
GET /api/admin/config
# Returns: paymentMethods state + featureFlags
```

### Admin Endpoints
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/admin/config` | GET | Get current config |
| `/api/admin/config/payment` | POST | Set payment methods |
| `/api/admin/config/features` | POST | Set feature flags |
| `/api/admin/users` | GET | List all users |
| `/api/admin/users/:id/role` | POST | Change user role |
| `/api/admin/analytics` | GET | Platform analytics |
| `/api/admin/users/:id/wallet` | POST | Adjust user wallet |

---

## Frontend Empty State Testing

### Initial State (No Games)
✅ **Live Matches** → "No live matches. Check back soon!"  
✅ **Tournaments** → "Tournaments coming soon!"  
✅ **Bets** → "No bets yet"  
✅ **Watch** → "No streams"  
✅ **Account** → Profile card + empty stats

### Payment Status Test
1. **Without payment keys configured:**
   - Open Account tab
   - Click "Deposit" button
   - See: "⚠️ Payment not yet available"

2. **After admin enables payment:**
   - Call: `POST /api/admin/config/payment` with `paystack_enabled: true`
   - Refresh page
   - Click "Deposit" button
   - See: Amount input field + "Proceed" button

---

## Mobile Responsiveness - Infinix Smart 10

### Device Specs
- Screen: 5.7" (380px width)
- Safe area: notched design
- Bottom nav: Uses `env(safe-area-inset-bottom)`

### Test Points
✅ Bottom navigation visible and clickable  
✅ Navigation items don't overlap content  
✅ Modals display full-screen  
✅ Text readable (no overflow)  
✅ Forms usable on small screen  
✅ Header logo visible  

### CSS Media Query (< 380px)
```css
/* Optimizations for very small screens */
.nb { min-height: 55px; padding: 6px 0px; }
.nb svg { width: 19px; height: 19px; }
```

---

## Profile Picture Testing

### Test Case 1: Google OAuth
1. Login with Google account
2. Frontend extracts profile picture from Google
3. Picture auto-displays in:
   - Profile banner (80px avatar)
   - Header avatar (32px)

### Test Case 2: Manual Upload (Planned)
1. Add profile picture upload modal
2. Convert image to base64
3. Send to: `POST /api/auth/profile/:userId`
4. Picture stored in Firebase
5. Updates on next profile load

---

## Data Models in Firebase

### User Structure
```json
{
  "uid": {
    "email": "user@example.com",
    "username": "username",
    "role": "player|admin|moderator",
    "passwordHash": "bcrypt_hash",
    "profilePicture": "url_or_base64",
    "wallet": { "NGN": 0, "GHS": 0 },
    "stats": {
      "totalBets": 0,
      "wins": 0,
      "losses": 0,
      "earnings": 0
    },
    "achievements": [],
    "gameHistory": [],
    "antiCheatLog": [],
    "referralCode": "BET000000",
    "referredBy": "code_or_null",
    "createdAt": "timestamp",
    "lastLogin": "timestamp"
  }
}
```

### Admin Config Structure
```json
{
  "admin": {
    "config": {
      "paymentMethods": {
        "paystack": false,
        "flutterwave": false,
        "updatedAt": "timestamp",
        "updatedBy": "admin_uid"
      },
      "featureFlags": {
        "depositEnabled": false,
        "withdrawEnabled": false,
        "tournamentsEnabled": false
      }
    }
  }
}
```

---

## Common Issues & Fixes

### Issue: "Email and password required"
- Frontend validation error
- Check if both fields filled

### Issue: "Email already registered"
- User exists in Firebase
- Try login or use different email

### Issue: Password reset email not received
- Check Gmail app password configured
- GMAIL_USER and GMAIL_PASSWORD in .env
- May need to enable "Less secure app access"

### Issue: "Payment not yet available" always shows
- Admin config not set yet
- Call `/api/admin/config/payment` endpoint
- Use admin token (x-user-id header)

### Issue: Bottom nav overlaps content on mobile
- Applied: `padding-bottom: max(10px, env(safe-area-inset-bottom))`
- Fixed position with safe-area handling
- Test on actual Infinix Smart 10 device

---

## Next Steps

### Immediate Tasks
- [ ] Start backend: `npm run dev`
- [ ] Test register flow
- [ ] Test login flow
- [ ] Test forgot password (configure Gmail)
- [ ] Test admin payment config
- [ ] Verify bottom nav on mobile

### Feature Expansion
- [ ] Google OAuth full integration (Client ID setup)
- [ ] Profile picture upload modal
- [ ] Game history population
- [ ] Achievements system
- [ ] Referral code system
- [ ] Anti-cheat logging
- [ ] Paystack integration
- [ ] Flutterwave integration

### Testing Devices
- Chrome DevTools (380px width)
- Actual Infinix Smart 10
- iPad (responsive check)
- iPhone (notch handling)

---

## Endpoints Summary

### Authentication
```
POST   /api/auth/register           - User registration
POST   /api/auth/login              - User login
POST   /api/auth/forgot-password    - Request password reset
POST   /api/auth/reset-password     - Complete password reset
GET    /api/auth/verify/:token      - Token verification
GET    /api/auth/profile/:userId    - Get user profile
POST   /api/auth/profile/:userId    - Update user profile
POST   /api/auth/google-oauth       - Google OAuth callback
```

### Admin (Requires: x-user-id header, Bearer token)
```
GET    /api/admin/config            - Get configuration
POST   /api/admin/config/payment    - Set payment methods
POST   /api/admin/config/features   - Set feature flags
GET    /api/admin/users             - List all users
POST   /api/admin/users/:id/role    - Update user role
GET    /api/admin/analytics         - Fetch analytics
POST   /api/admin/users/:id/wallet  - Adjust wallet
```

---

## Environment Variables Checklist
- [ ] JWT_SECRET (for token signing)
- [ ] FRONTEND_URL (for password reset emails)
- [ ] GMAIL_USER (sender email)
- [ ] GMAIL_PASSWORD (app-specific password)
- [ ] GOOGLE_CLIENT_ID (OAuth)
- [ ] GOOGLE_CLIENT_SECRET (OAuth)
- [ ] PAYSTACK_SECRET_KEY (optional)
- [ ] FLUTTERWAVE_SECRET_KEY (optional)

---

**Last Updated:** 2024  
**Version:** 2.0 - Complete Auth + Admin System
