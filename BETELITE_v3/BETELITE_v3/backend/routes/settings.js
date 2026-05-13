const express = require('express');
const router = express.Router();

// In-memory global settings (will reset on server restart unless we use a DB)
// Eventually, this should be pushed to Firebase or a local JSON file.
let globalSettings = {
  paystackKey: process.env.PAYSTACK_PUBLIC_KEY || null,
};

router.get('/', (req, res) => {
  res.json({ success: true, settings: globalSettings });
});

router.post('/', (req, res) => {
  try {
    const { key, value } = req.body;
    if (!key) return res.status(400).json({ error: 'Missing key' });

    // Validate the key being set
    if (key === 'paystackKey') {
      globalSettings.paystackKey = value;
      return res.json({ success: true, settings: globalSettings });
    }

    res.status(400).json({ error: 'Invalid setting key' });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save settings' });
  }
});

module.exports = router;
