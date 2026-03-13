// routes/matches.js
const express = require('express');
module.exports = (io, engine) => {
  const r = express.Router();
  r.get('/',        (req, res) => res.json({ matches: engine.getAllMatches() }));
  r.get('/:id',     (req, res) => {
    const m = engine.getMatch(req.params.id);
    if (!m) return res.status(404).json({ error: 'Not found' });
    res.json(m);
  });
  r.post('/:id/submit', async (req, res) => {
    try {
      const result = await engine.submitScore({ matchId: req.params.id, ...req.body });
      res.json({ ok: true, match: result });
    } catch (e) { res.status(400).json({ ok: false, error: e.message }); }
  });
  return r;
};
