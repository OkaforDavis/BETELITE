const express = require('express');
const router = express.Router();

// In-memory global settings (will reset on server restart unless we use a DB)
let globalSettings = {
  paystackKey: process.env.PAYSTACK_PUBLIC_KEY || null,
  paystackKeyGH: process.env.PAYSTACK_PUBLIC_KEY_GH || null,
};

// GET is public — frontend needs to read keys on startup
router.get('/', (req, res) => {
  res.json({ success: true, settings: globalSettings });
});

// POST requires admin auth — handled by verifyFirebaseToken + requireAdmin in server.js
// We add a basic check here: only allow known setting keys
router.post('/', (req, res) => {
  try {
    const { key, value } = req.body;
    if (!key) return res.status(400).json({ error: 'Missing key' });

    const ALLOWED_KEYS = ['paystackKey', 'paystackKeyGH'];
    if (!ALLOWED_KEYS.includes(key)) {
      return res.status(400).json({ error: 'Invalid setting key' });
    }

    globalSettings[key] = value;
    return res.json({ success: true, settings: globalSettings });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save settings' });
  }
});

module.exports = router;
