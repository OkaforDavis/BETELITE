# BETELITE — Backend + AI Connection Guide

## How the pieces connect

```
Your Phone (Player)
      ↓
mobile/index.html  ←→  Socket.io  ←→  backend/server.js
                                            ↓              ↓
                                       Firebase DB    Anthropic AI
                                            ↓
                                       Paystack/Flutterwave
```

---

## STEP 1 — Set up Firebase (10 minutes)

### 1A — Create Firebase project
1. Go to https://console.firebase.google.com/
2. Click **Add project** → Name it `betelite`
3. Disable Google Analytics → click **Create project**

### 1B — Enable Authentication
1. Left sidebar → **Authentication** → **Get started**
2. Click **Sign-in method** tab
3. Enable **Email/Password** → toggle ON → Save
4. Enable **Google** → toggle ON → add your support email → Save

### 1C — Set up Realtime Database
1. Left sidebar → **Realtime Database** → **Create database**
2. Choose your region (pick closest to Nigeria — `europe-west1`)
3. Select **Start in test mode** (we'll lock it down later)
4. Click **Enable**

### 1D — Get your web config
1. Left sidebar → **Project Settings** (gear icon) → **General** tab
2. Scroll to **Your apps** → click **</>** (web)
3. Register app name: `betelite-mobile`
4. Copy the `firebaseConfig` object — looks like:
```js
const firebaseConfig = {
  apiKey: "AIzaSy...",
  authDomain: "betelite-xxxxx.firebaseapp.com",
  databaseURL: "https://betelite-xxxxx-default-rtdb.firebaseio.com",
  projectId: "betelite-xxxxx",
  storageBucket: "betelite-xxxxx.appspot.com",
  messagingSenderId: "123456789",
  appId: "1:123456789:web:abcdef"
};
```

### 1E — Paste config into mobile/index.html
Find this block near the top of `mobile/index.html`:
```js
const FIREBASE_CONFIG = {
  apiKey:            window.FIREBASE_API_KEY            || "YOUR_API_KEY",
  authDomain:        window.FIREBASE_AUTH_DOMAIN        || "YOUR_PROJECT.firebaseapp.com",
  ...
```
Replace the `"YOUR_..."` values with your real Firebase values:
```js
const FIREBASE_CONFIG = {
  apiKey:            "AIzaSy...",
  authDomain:        "betelite-xxxxx.firebaseapp.com",
  databaseURL:       "https://betelite-xxxxx-default-rtdb.firebaseio.com",
  projectId:         "betelite-xxxxx",
  storageBucket:     "betelite-xxxxx.appspot.com",
  messagingSenderId: "123456789",
  appId:             "1:123456789:web:abcdef",
};
```

### 1F — Get server-side credentials (for backend)
1. **Project Settings** → **Service Accounts** tab
2. Click **Generate new private key** → Download JSON file
3. Keep this file safe — never commit to Git

---

## STEP 2 — Set up Anthropic AI (5 minutes)

### 2A — Get API key
1. Go to https://console.anthropic.com/
2. Sign in → **API Keys** → **Create Key**
3. Name it `betelite-detection`
4. Copy the key (starts with `sk-ant-api03-...`)

### 2B — Add to backend
Add to `backend/.env`:
```
ANTHROPIC_API_KEY=sk-ant-api03-YOUR_KEY_HERE
```

---

## STEP 3 — Set up Payment (Paystack)

### 3A — Create Paystack account
1. Go to https://paystack.com → Sign up
2. Complete business verification (required for payouts)
3. Dashboard → **Settings** → **API Keys & Webhooks**
4. Copy your **Public Key** (starts with `pk_test_...` for test mode)

### 3B — Add to site
In `mobile/index.html`, find:
```js
const PAYMENT_CONFIG = {
  paystackKey:    window.PAYSTACK_PUBLIC_KEY    || null,
```
Change to:
```js
const PAYMENT_CONFIG = {
  paystackKey:    "pk_live_YOUR_KEY_HERE",
```
**Without this**, users see "Payment not yet available" — which is intentional and correct.

---

## STEP 4 — Run backend locally (Firebase Studio)

In Firebase Studio terminal:
```bash
cd backend
cp .env.example .env
# Edit .env with your keys

npm install
npm run dev
```

You should see:
```
╔═══════════════════════════════════════╗
║   BETELITE Backend v2.0               ║
║   Port: 3000                          ║
║   Mobile: http://localhost:3000/mobile║
╚═══════════════════════════════════════╝
```

Open the preview URL → you'll see the login screen.

---

## STEP 5 — First Login as Admin

1. Open the site
2. Click **SIGN IN** tab
3. Enter: `okafordavis8@gmail.com`
4. Click **Forgot password?** → this sends a reset email to your Gmail
5. Check Gmail → click the reset link → set your new password
6. Return to BETELITE → sign in with your new password
7. You'll see **⭐ Admin Mode** in the status bar

---

## STEP 6 — Push to GitHub Pages (live test)

```bash
# From your BETELITE repo
git add mobile/index.html
git commit -m "v3: auth, empty state, full profile"
git push origin main
```

GitHub Pages will serve your `mobile/index.html` at:
`https://okafordavis.github.io/BETELITE/mobile/`

**Note**: GitHub Pages only serves the frontend. For full backend features (real-time scores, AI detection), you need the backend deployed too.

---

## STEP 7 — Deploy backend free (Render.com)

1. Go to https://render.com → Sign in with GitHub
2. **New** → **Web Service** → Connect `BETELITE` repo
3. Settings:
   - **Root Directory**: `backend`
   - **Build Command**: `npm install`
   - **Start Command**: `node server.js`
4. Add environment variables (from your `.env`)
5. **Deploy** — takes ~3 minutes
6. Copy your URL: `https://betelite-backend.onrender.com`

7. In `mobile/index.html`, find:
```js
return window.BETELITE_BACKEND || 'https://betelite-backend.onrender.com';
```
Replace with your actual Render URL.

---

## What works WITHOUT backend

- ✅ Login / Signup / Google OAuth
- ✅ Forgot password (sends real email)
- ✅ Admin detection
- ✅ Profile with photo
- ✅ All account sections (history, achievements, referral, anti-cheat, settings)
- ✅ Currency toggle ₦ / ₵
- ✅ Coming soon screens with promo content
- ✅ Payment error message if no key set

## What needs backend

- 🔄 Live match scores
- 🔄 Real-time socket updates
- 🔄 AI screenshot detection
- 🔄 Live streaming (WebRTC)
- 🔄 Bet placement on real matches

---

## Infinix Smart 10 — Nav Fix

The bottom nav is now fixed with:
- `min-height: 50px` on each nav button (bigger touch targets)
- `padding-bottom: max(env(safe-area-inset-bottom), 6px)` for Android chin
- `height: 100dvh` (dynamic viewport height — accounts for Android browser chrome)
- Toast positioned above nav with proper spacing

These changes ensure buttons are always visible and tappable on the Infinix Smart 10.
