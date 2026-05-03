const express = require('express');
const { v4: uuid } = require('uuid');
const axios = require('axios');

// In-memory store
const tournaments = new Map();
const fixtures    = new Map(); // fixtureId -> fixture object

// ── Seed tournaments
function seedTournaments() {
  const configs = [
    { game:'efootball', name:'eFootball Champions Cup', mode:'5v5', icon:'⚽' },
    { game:'freefire',  name:'Free Fire Battle Royale', mode:'Squad', icon:'🔥' },
    { game:'cod',       name:'COD Mobile Warzone Pro',  mode:'5v5', icon:'🎯' },
    { game:'dream',     name:'Dream League Afrika Cup', mode:'1v1', icon:'🏟️' },
    { game:'fifa',      name:'FIFA Mobile Super Cup',   mode:'1v1', icon:'🏆' },
  ];
  const tiersNGN = [1500, 5000, 10000, 15000, 25000];
  configs.forEach((c, i) => {
    const id = uuid();
    const tier = tiersNGN[i % tiersNGN.length];
    tournaments.set(id, {
      id, ...c,
      entryFeeNGN: tier,
      maxPlayers: 8,
      players: [],
      status: 'open',   // open | active | finished
      prizePool: 0,
      matches: [],       // completed match results
      createdAt: Date.now(),
    });
  });
}
seedTournaments();

const PRIZE_STRUCTURE = [0.30, 0.20, 0.133, 0.067, 0.047];

module.exports = (io, engine) => {
  const r = express.Router();

  // ── List all tournaments
  r.get('/', (req, res) => {
    res.json({ tournaments: [...tournaments.values()].map(formatT) });
  });

  // ── Get single tournament + fixtures for logged-in user
  r.get('/:id', (req, res) => {
    const t = tournaments.get(req.params.id);
    if (!t) return res.status(404).json({ error: 'Not found' });
    const userId = req.query.userId;
    const myFixtures = userId
      ? [...fixtures.values()].filter(f => f.tournamentId === t.id && (f.homeId === userId || f.awayId === userId))
      : [];
    res.json({ tournament: formatT(t), myFixtures });
  });

  // ── Join tournament
  r.post('/:id/join', (req, res) => {
    const t = tournaments.get(req.params.id);
    if (!t) return res.status(404).json({ error: 'Not found' });
    if (t.status !== 'open') return res.status(400).json({ error: 'Tournament not open' });
    const { userId, username } = req.body;
    if (t.players.find(p => p.userId === userId))
      return res.status(400).json({ error: 'Already joined' });

    t.players.push({ userId, username, joinedAt: Date.now(), wins: 0, losses: 0, goals: 0, points: 0 });
    t.prizePool += t.entryFeeNGN;

    if (t.players.length >= t.maxPlayers) {
      t.status = 'active';
      generateFixtures(t);
      io.emit('tournament_started', { id: t.id, name: t.name });
    }

    io.emit('tournament_update', formatT(t));
    res.json({ ok: true, tournament: formatT(t), myFixtures: getMyFixtures(t.id, userId) });
  });

  // ── Get my fixtures
  r.get('/:id/fixtures', (req, res) => {
    const t = tournaments.get(req.params.id);
    if (!t) return res.status(404).json({ error: 'Not found' });
    const userId = req.query.userId;
    res.json({ fixtures: getMyFixtures(t.id, userId) });
  });

  // ── Submit match result with AI screenshot
  r.post('/:id/fixtures/:fixtureId/submit', async (req, res) => {
    const t = tournaments.get(req.params.id);
    const fixture = fixtures.get(req.params.fixtureId);

    if (!t || !fixture) return res.status(404).json({ error: 'Not found' });
    if (fixture.status === 'completed') return res.status(400).json({ error: 'Already submitted' });

    const { userId, scoreHome, scoreAway, image_b64 } = req.body;

    // Verify user is in this fixture
    if (fixture.homeId !== userId && fixture.awayId !== userId) {
      return res.status(403).json({ error: 'Not your match' });
    }

    let finalHome = parseInt(scoreHome) || 0;
    let finalAway = parseInt(scoreAway) || 0;
    let aiVerified = false;
    let aiResult = null;

    // AI verification if screenshot provided
    if (image_b64) {
      try {
        const aiRes = await axios.post('http://127.0.0.1:5000/api/detect/frame', {
          game: t.game,
          image_b64
        }, { timeout: 15000 });

        aiResult = aiRes.data;
        if (aiResult.detected) {
          finalHome = aiResult.scoreHome;
          finalAway = aiResult.scoreAway;
          aiVerified = true;
        }
      } catch (e) {
        console.log('[AI] Detection failed, using manual scores:', e.message);
      }
    }

    // Update fixture
    fixture.status = 'completed';
    fixture.scoreHome = finalHome;
    fixture.scoreAway = finalAway;
    fixture.aiVerified = aiVerified;
    fixture.submittedAt = Date.now();
    fixture.submittedBy = userId;

    // Update player stats
    const homePlayer = t.players.find(p => p.userId === fixture.homeId);
    const awayPlayer = t.players.find(p => p.userId === fixture.awayId);

    if (homePlayer && awayPlayer) {
      homePlayer.goals += finalHome;
      awayPlayer.goals += finalAway;
      if (finalHome > finalAway) {
        homePlayer.wins++; homePlayer.points += 3;
        awayPlayer.losses++;
      } else if (finalAway > finalHome) {
        awayPlayer.wins++; awayPlayer.points += 3;
        homePlayer.losses++;
      } else {
        homePlayer.points += 1;
        awayPlayer.points += 1;
      }
    }

    t.matches.push({
      fixtureId: fixture.id,
      homeId: fixture.homeId, homeName: fixture.homeName,
      awayId: fixture.awayId, awayName: fixture.awayName,
      scoreHome: finalHome, scoreAway: finalAway,
      aiVerified, completedAt: Date.now()
    });

    io.to(`tournament:${t.id}`).emit('match_result', {
      tournament: formatT(t),
      fixture,
      aiResult
    });

    res.json({ ok: true, fixture, aiVerified, finalScore: `${finalHome}-${finalAway}`, aiResult });
  });

  // ── Leaderboard
  r.get('/:id/leaderboard', (req, res) => {
    const t = tournaments.get(req.params.id);
    if (!t) return res.status(404).json({ error: 'Not found' });
    const sorted = [...t.players].sort((a, b) => b.points - a.points || b.goals - a.goals);
    res.json({ leaderboard: sorted, prizes: buildPrizes(t) });
  });

  return r;
};

function generateFixtures(t) {
  // Round-robin: each player plays every other player once
  const players = t.players;
  for (let i = 0; i < players.length; i++) {
    for (let j = i + 1; j < players.length; j++) {
      const id = uuid();
      fixtures.set(id, {
        id,
        tournamentId: t.id,
        homeId: players[i].userId,
        homeName: players[i].username,
        awayId: players[j].userId,
        awayName: players[j].username,
        status: 'pending',  // pending | completed
        scoreHome: null,
        scoreAway: null,
        aiVerified: false,
        createdAt: Date.now(),
      });
    }
  }
}

function getMyFixtures(tournamentId, userId) {
  if (!userId) return [];
  return [...fixtures.values()].filter(
    f => f.tournamentId === tournamentId && (f.homeId === userId || f.awayId === userId)
  );
}

function formatT(t) {
  const sorted = [...t.players].sort((a, b) => b.points - a.points || b.goals - a.goals);
  return {
    ...t,
    playerCount: t.players.length,
    slotsLeft: t.maxPlayers - t.players.length,
    leaderboard: sorted,
    prizes: buildPrizes(t),
  };
}

function buildPrizes(t) {
  return PRIZE_STRUCTURE.map((pct, i) => ({
    position: i + 1, pct,
    amount: Math.floor(t.prizePool * pct),
  }));
}
