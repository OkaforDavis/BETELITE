const express = require('express');
const router = express.Router();
const axios = require('axios');
const multer = require('multer');
const FormData = require('form-data');

const upload = multer({ storage: multer.memoryStorage() });

module.exports = function(io, engine) {
  // Proxy endpoint to Python FastAPI Detection Service
  router.post('/frame', upload.single('image'), async (req, res) => {
    try {
      const gameType = req.body.game || 'football';
      const targetGamertag = req.body.target_gamertag || '';
      const opponentGamertag = req.body.opponent_gamertag || '';
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

      // Call local FastAPI service on port 5000
      const pyRes = await axios.post('http://127.0.0.1:5000/api/detect/frame', form, {
        headers: form.getHeaders(),
      });

      res.json(pyRes.data);
    } catch (error) {
      console.error('[AI Proxy Error]:', error.message);
      if (error.response) {
        res.status(error.response.status).json(error.response.data);
      } else {
        res.status(500).json({ error: 'Failed to contact AI Detection Service' });
      }
    }
  });

  return router;
};
