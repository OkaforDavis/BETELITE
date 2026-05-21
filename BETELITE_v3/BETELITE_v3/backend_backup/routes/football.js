// routes/football.js
const express = require('express');
const { getLiveFootball } = require('../services/footballAPI');
module.exports = (io, engine) => {
  const r = express.Router();
  r.get('/live', async (req, res) => {
    const matches = await getLiveFootball();
    res.json({ matches });
  });
  return r;
};
