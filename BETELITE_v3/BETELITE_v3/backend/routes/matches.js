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

  r.post('/locked-room', (req, res) => {
    const { hostName, gameType, roomId } = req.body;
    if (!roomId || !hostName) return res.status(400).json({ error: 'Missing room data' });
    
    // Create an ad-hoc match for the live board
    const roomMatch = {
      id: roomId,
      game: gameType || 'CrestArena',
      label: 'Private Stream',
      home: hostName,
      away: 'Waiting...',
      scoreHome: 0,
      scoreAway: 0,
      status: 'live',
      minute: 0,
      isLockedRoom: true,
      verifiedBy: 'host'
    };
    
    // Inject it into the live matches map
    engine.liveMatches.set(roomId, roomMatch);
    
    // Broadcast to everyone so the Live Board updates
    io.emit('score_update', roomMatch);
    
    res.json({ ok: true, match: roomMatch });
  });
  return r;
};
