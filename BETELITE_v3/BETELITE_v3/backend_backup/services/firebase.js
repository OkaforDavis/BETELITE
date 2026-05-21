const admin = require('firebase-admin');

let db = null;

try {
  const projectId  = process.env.FIREBASE_PROJECT_ID;
  const dbUrl      = process.env.FIREBASE_DATABASE_URL;
  const saJson     = process.env.FIREBASE_SERVICE_ACCOUNT_JSON;

  const hasValidProject = projectId && !projectId.includes('REPLACE');
  const hasServiceAccount = saJson && saJson.trim().length > 2;

  if (hasValidProject && hasServiceAccount) {
    // Full admin mode — use service account for privileged DB access
    const serviceAccount = JSON.parse(saJson);
    if (!admin.apps.length) {
      admin.initializeApp({
        credential: admin.credential.cert(serviceAccount),
        databaseURL: dbUrl || `https://${projectId}-default-rtdb.firebaseio.com`,
      });
    }
    db = admin.database();
    console.log('[FIREBASE] ✅ Admin SDK connected with service account');
  } else if (hasValidProject) {
    // Project ID set but no service account — skip Admin SDK, run in-memory
    // The frontend Firebase JS SDK handles auth directly, backend uses in-memory state
    console.log('[FIREBASE] ℹ️  No service account — backend running in-memory mode.');
    console.log('[FIREBASE]    To enable server-side DB writes, add FIREBASE_SERVICE_ACCOUNT_JSON to .env');
  } else {
    console.log('[FIREBASE] No credentials — running in-memory only');
  }
} catch (e) {
  console.error('[FIREBASE] Init error:', e.message);
}

module.exports = { db, admin };
