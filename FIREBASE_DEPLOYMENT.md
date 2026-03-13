# Firebase Deployment Guide

## 📦 Auto-Deployment Status

Your BETELITE project has been fully configured for Firebase Hosting deployment.

### What's Ready:
✅ **firebase.json** - Hosting configuration created  
✅ **GitHub Actions** - CI/CD workflow in `.github/workflows/deploy.yml`  
✅ **.firebaserc** - Project ID configured to `betelite-60181385`  
✅ **All files** - Backend, frontend, and documentation ready  

---

## 🚀 Deploy to Firebase (Manual Steps)

### Step 1: Install Firebase CLI
```bash
npm install -g firebase-tools
```

### Step 2: Authenticate with Firebase
```bash
firebase login
```
This opens a browser where you authenticate with your Google account for Firebase.

### Step 3: Deploy
```bash
firebase deploy
```

Or specifically to your project:
```bash
firebase deploy --project=betelite-60181385
```

---

## 🔄 GitHub Actions Auto-Deploy

The `.github/workflows/deploy.yml` workflow is configured to automatically deploy on every push to `main` branch.

### Setup GitHub Actions:
1. Go to https://github.com/OkaforDavis/BETELITE/settings/secrets/actions
2. Add secret: `FIREBASE_TOKEN`
   - Get token: `firebase login:ci`
   - This generates a token that GitHub Actions can use to deploy

3. Add secret: `FIREBASE_EMAIL` (your Firebase account email)

After setting up secrets, all future pushes to `main` will auto-deploy.

---

## 📍 Access Your Deployed App

Once deployed, your app will be available at:
- **Project:** https://betelite-60181385.firebaseapp.com
- **Firebase Studio:** https://studio.firebase.google.com/betelite-60181385

---

## 🛠️ What Gets Deployed

From `firebase.json` configuration:

✅ **static/** - All frontend files (HTML, CSS, JS)
✅ **index.html** - Main entry point
✅ **mobile/** - Mobile app interface  
✅ **landing.html** - Landing page
✅ **docs/** - Documentation site

❌ **Excluded:**
- `backend/` - Runs separately on Node.js/Docker
- `firebase.json` - Configuration file
- `*.md` - Documentation files
- `.git/` - Git repository data
- `node_modules/` - Dependencies

---

## 🔌 Backend Integration

Your Node.js backend (`backend/server.js`) runs separately:

**Development:**
```bash
cd backend
npm install
npm start
```

**Production:**
Use Docker or your hosting provider (Railway, Heroku, Render, etc.)

**Point frontend to backend API:**
Edit calls in `frontend/js/api.js` to point to your backend URL:
- Dev: `http://localhost:3000`
- Prod: `https://your-backend-url.com`

---

## ✅ Deployment Checklist

- [ ] Firebase CLI installed (`firebase --version`)
- [ ] Firebase login completed (`firebase login`)
- [ ] Project ID verified: `betelite-60181385`
- [ ] Backend running (if using API)
- [ ] Frontend API URLs updated
- [ ] Run `firebase deploy`
- [ ] Check https://betelite-60181385.firebaseapp.com

---

## 📊 Monitoring & Logs

View deployment logs:
```bash
firebase open hosting:site
firebase open functions:logs
```

View in Firebase Console:
https://console.firebase.google.com/project/betelite-60181385

---

## 🆘 Troubleshooting

**"firebase: The term 'firebase' is not recognized"**
- Ensure Firebase CLI is installed globally: `npm install -g firebase-tools`
- Restart terminal after installation

**"Permission denied" error**
- Run `firebase login` again
- Ensure your account has access to the project

**"No hosting URL available"**
- Check `firebase.json` is in the root directory
- Run `firebase init hosting` to regenerate if needed

---

**Last Updated:** March 13, 2026  
**Project:** BETELITE - Esports Betting Platform  
**Repository:** https://github.com/OkaforDavis/BETELITE
