// routes/stream.js
const express = require('express');
module.exports = (io) => {
  const r = express.Router();
  r.get('/active', (req, res) => {
    // Return active stream rooms
    res.json({ streams: [] }); // populated via socket
  });
  return r;
};
