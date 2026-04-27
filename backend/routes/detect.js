const express = require('express');
const multer  = require('multer');
const path    = require('path');
const fs      = require('fs');

const upload = multer({
  dest: 'uploads/',
  limits: { fileSize: 10 * 1024 * 1024 }, // 10MB
  fileFilter: (req, file, cb) => {
    const ok = /^image\/(jpeg|jpg|png|webp|gif)$/.test(file.mimetype);
    cb(null, ok);
  },
});

module.exports = (io, engine) => {
  const r = express.Router();

  // POST /api/detect/screenshot
  r.post('/screenshot', upload.single('screenshot'), async (req, res) => {
    try {
      let imageBase64, mimeType;

      if (req.file) {
        // File upload
        const buf = fs.readFileSync(req.file.path);
        imageBase64 = buf.toString('base64');
        mimeType = req.file.mimetype;
        fs.unlinkSync(req.file.path); // clean up
      } else if (req.body.imageBase64) {
        // Base64 in body
        imageBase64 = req.body.imageBase64.replace(/^data:image\/\w+;base64,/, '');
        mimeType = req.body.mimeType || 'image/jpeg';
      } else {
        return res.status(400).json({ error: 'No image provided' });
      }

      const matchId = req.body.matchId || null;
      const result  = await engine.analyzeScreenshot(matchId, imageBase64, mimeType);

      // Auto-apply if confident and matchId provided
      if (matchId && result.confidence >= 0.75 && !result.cheatDetected && !result.error) {
        try {
          const updated = await engine.applyAIScore({ matchId, ...result });
          io.to(`match:${matchId}`).emit('score_update', updated);
        } catch (e) { /* match may not exist */ }
      }

      if (result.cheatDetected) {
        io.emit('cheat_alert', { matchId, result });
      }

      res.json({ ok: true, result });
    } catch (e) {
      console.error('[DETECT]', e.message);
      res.status(500).json({ ok: false, error: e.message });
    }
  });

  // POST /api/detect/frame  (stream frame base64)
  r.post('/frame', async (req, res) => {
    const { matchId, frameBase64 } = req.body;
    if (!matchId || !frameBase64) return res.status(400).json({ error: 'Missing data' });
    const result = await engine.analyzeFrame(matchId, frameBase64);
    res.json({ ok: true, result });
  });

  // POST /api/detect/event (from Python AI service)
  r.post('/event', async (req, res) => {
    const { type, matchId, team, confidence } = req.body;
    if (!matchId || !type) return res.status(400).json({ error: 'Missing data' });
    
    // Simulate updating the score based on the event type
    if (type === 'goal_detected') {
      try {
        const updated = await engine.applyAIScore({ 
          matchId, 
          confidence, 
          team,
          detectedGoal: true
        });
        io.to(`match:${matchId}`).emit('score_update', updated);
      } catch (e) {
        console.error('[DETECT EVENT]', e.message);
      }
    }
    
    res.json({ ok: true });
  });

  return r;
};
