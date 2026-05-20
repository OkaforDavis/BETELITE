const express = require('express');
const router = express.Router();

// In-memory global settings (will reset on server restart unless we use a DB)
let globalSettings = {
  paystackKey: process.env.PAYSTACK_PUBLIC_KEY || null,
  paystackKeyGH: process.env.PAYSTACK_PUBLIC_KEY_GH || null,
};

router.get('/', (req, res) => {
  res.json({ success: true, settings: globalSettings });
});

router.post('/', (req, res) => {
  try {
    const { key, value } = req.body;
    if (!key) return res.status(400).json({ error: 'Missing key' });

    if (key === 'paystackKey') {
      globalSettings.paystackKey = value;
      return res.json({ success: true, settings: globalSettings });
    }
    if (key === 'paystackKeyGH') {
      globalSettings.paystackKeyGH = value;
      return res.json({ success: true, settings: globalSettings });
    }

    res.status(400).json({ error: 'Invalid setting key' });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save settings' });
  }
});

module.exports = router;
