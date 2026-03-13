const admin = require('firebase-admin');

let db = null;

try {
  if (process.env.FIREBASE_PROJECT_ID) {
    const serviceAccount = process.env.FIREBASE_SERVICE_ACCOUNT_JSON
      ? JSON.parse(process.env.FIREBASE_SERVICE_ACCOUNT_JSON)
      : null;

    const config = serviceAccount
      ? { credential: admin.credential.cert(serviceAccount) }
      : { credential: admin.credential.applicationDefault(),
          databaseURL: `https://${process.env.FIREBASE_PROJECT_ID}-default-rtdb.firebaseio.com` };

    if (!admin.apps.length) admin.initializeApp({
      ...config,
      databaseURL: process.env.FIREBASE_DATABASE_URL ||
        `https://${process.env.FIREBASE_PROJECT_ID}-default-rtdb.firebaseio.com`,
    });

    db = admin.database();
    console.log('[FIREBASE] Connected to Realtime Database');
  } else {
    console.log('[FIREBASE] No credentials — running in-memory only');
  }
} catch (e) {
  console.error('[FIREBASE] Init error:', e.message);
}

module.exports = { db, admin };
