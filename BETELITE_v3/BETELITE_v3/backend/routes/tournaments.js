const express = require('express');
const { v4: uuid } = require('uuid');

// In-memory tournament store (replace with Firebase in production)
const tournaments = new Map();

// Seed some tournaments
function seedTournaments() {
  const configs = [
    { game:'efootball', name:'eFootball Champions Cup', mode:'5v5', icon:'⚽' },
    { game:'freefire',  name:'Free Fire Battle Royale', mode:'Squad', icon:'🔥' },
    { game:'cod',       name:'COD Mobile Warzone Pro',  mode:'5v5', icon:'🎯' },
    { game:'dream',     name:'Dream League Afrika Cup', mode:'1v1', icon:'🏟️' },
    { game:'fifa',      name:'FIFA Mobile Super Cup',   mode:'1v1', icon:'🏆' },
  ];
  const tiersNGN = [1500, 5000, 10000, 15000, 25000, 50000];
  configs.forEach((c, i) => {
    const id = uuid();
    const tier = tiersNGN[i % tiersNGN.length];
    tournaments.set(id, {
      id, ...c,
      entryFeeNGN: tier,
      maxPlayers: 15,
      players: [],
      status: 'open',
      prizePool: tier * 15,
      createdAt: Date.now(),
    });
  });
}
seedTournaments();

const PRIZE_STRUCTURE = [0.30, 0.20, 0.133, 0.067, 0.047];

module.exports = (io, engine) => {
  const r = express.Router();

  r.get('/', (req, res) => {
    res.json({ tournaments: [...tournaments.values()].map(formatT) });
  });

  r.post('/:id/join', (req, res) => {
    const t = tournaments.get(req.params.id);
    if (!t) return res.status(404).json({ error: 'Not found' });
    if (t.status === 'full') return res.status(400).json({ error: 'Tournament full' });
    const { userId, username, entryTier } = req.body;
    if (t.players.find(p => p.userId === userId))
      return res.status(400).json({ error: 'Already joined' });

    const fee = entryTier || t.entryFeeNGN;
    t.players.push({ userId, username, joinedAt: Date.now(), entryFee: fee });
    t.prizePool += fee;

    if (t.players.length >= t.maxPlayers) {
      t.status = 'full';
      io.emit('tournament_full', { id: t.id, name: t.name });
    }

    io.emit('tournament_update', formatT(t));
    res.json({ ok: true, tournament: formatT(t) });
  });

  r.get('/:id/bracket', (req, res) => {
    const t = tournaments.get(req.params.id);
    if (!t) return res.status(404).json({ error: 'Not found' });
    res.json({ bracket: buildBracket(t), prizes: buildPrizes(t) });
  });

  return r;
};

function formatT(t) {
  return {
    ...t,
    playerCount: t.players.length,
    slotsLeft: t.maxPlayers - t.players.length,
    prizes: buildPrizes(t),
  };
}

function buildPrizes(t) {
  return PRIZE_STRUCTURE.map((pct, i) => ({
    position: i + 1,
    pct,
    amount: Math.floor(t.prizePool * pct),
  }));
}

function buildBracket(t) {
  return t.players.map((p, i) => ({ position: i+1, ...p, wins: 0, losses: 0 }));
}
