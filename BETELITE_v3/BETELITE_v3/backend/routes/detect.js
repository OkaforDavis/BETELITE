const express = require('express');
const router = express.Router();
const axios = require('axios');
const multer = require('multer');
const FormData = require('form-data');

const upload = multer({ storage: multer.memoryStorage(), limits: { fileSize: 10 * 1024 * 1024 } });

const DETECT_SERVICE_URL = process.env.DETECTION_SERVICE_URL || 'http://127.0.0.1:5000';

module.exports = function(io, engine) {

  // ── Main frame analysis endpoint
  router.post('/frame', upload.single('image'), async (req, res) => {
    try {
      const gameType         = req.body.game || 'football';
      const targetGamertag   = req.body.target_gamertag || '';
      const opponentGamertag = req.body.opponent_gamertag || '';
      const matchId          = req.body.match_id || null;

      let detectionResult = null;

      // ── Try Python OCR service first (local or remote)
      try {
        const form = new FormData();
        form.append('game', gameType);
        form.append('target_gamertag', targetGamertag);
        form.append('opponent_gamertag', opponentGamertag);

        if (req.file) {
          form.append('file', req.file.buffer, {
            filename: req.file.originalname || 'upload.png',
            contentType: req.file.mimetype || 'image/png',
          });
        } else if (req.body.image_b64) {
          form.append('image_b64', req.body.image_b64);
        } else {
          return res.status(400).json({ error: 'No image provided' });
        }

        const pyRes = await axios.post(`${DETECT_SERVICE_URL}/api/detect/frame`, form, {
          headers: form.getHeaders(),
          timeout: 15000,
        });
        detectionResult = pyRes.data;
        console.log(`[AI] OCR detection complete for match ${matchId || 'none'}`);

      } catch (pyErr) {
        // ── Fallback: Anthropic Claude vision (works on any server)
        if (engine.client && (req.file || req.body.image_b64)) {
          console.log('[AI] Python service unavailable, falling back to Anthropic vision...');
          try {
            const imageData = req.file
              ? req.file.buffer.toString('base64')
              : req.body.image_b64.replace(/^data:image\/\w+;base64,/, '');
            const mediaType = req.file ? (req.file.mimetype || 'image/png') : 'image/png';

            detectionResult = await engine.analyzeScreenshot(matchId || 'temp', imageData, mediaType);
          } catch (aiErr) {
            console.error('[AI] Anthropic fallback failed:', aiErr.message);
            return res.status(503).json({
              error: 'Detection service unavailable. Start the Python service: cd detection_service && python app.py',
              python_error: pyErr.message,
            });
          }
        } else {
          return res.status(503).json({
            error: 'Detection service offline. Start: cd detection_service && python app.py',
            detail: pyErr.message,
          });
        }
      }

      // ── If a matchId was provided, inject the score into the game engine
      if (matchId && detectionResult && !detectionResult.error) {
        try {
          const updatedMatch = await engine.applyAIScore({
            matchId,
            score1: detectionResult.score1 ?? detectionResult.score_home ?? null,
            score2: detectionResult.score2 ?? detectionResult.score_away ?? null,
            gameEnded: detectionResult.game_ended || detectionResult.gameEnded || false,
            cheatDetected: detectionResult.cheat_detected || detectionResult.cheatDetected || false,
            cheatReason: detectionResult.cheat_reason || '',
          });

          if (updatedMatch) {
            // Broadcast to ALL clients in the match room AND globally
            io.to(`match:${matchId}`).emit('score_update', updatedMatch);
            io.emit('score_update', updatedMatch);  // global so live board everywhere updates
            console.log(`[AI] Score injected into match ${matchId}: ${updatedMatch.scoreHome}-${updatedMatch.scoreAway}`);
          }
        } catch (injectErr) {
          console.warn('[AI] Score injection failed:', injectErr.message);
        }
      }

      res.json({ ...detectionResult, _matchId: matchId });

    } catch (error) {
      console.error('[AI Proxy Error]:', error.message);
      if (error.response) {
        res.status(error.response.status).json(error.response.data);
      } else {
        res.status(500).json({ error: 'Detection failed: ' + error.message });
      }
    }
  });

  return router;
};
