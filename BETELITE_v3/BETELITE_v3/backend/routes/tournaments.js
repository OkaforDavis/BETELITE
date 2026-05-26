const express = require('express');
const { v4: uuid } = require('uuid');
const axios = require('axios');
const { sendNotification } = require('../services/notifications');
const { db } = require('../services/firebase');

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
  r.post('/:id/join', async (req, res) => {
    const t = tournaments.get(req.params.id);
    if (!t) return res.status(404).json({ error: 'Not found' });
    if (t.status !== 'open') return res.status(400).json({ error: 'Tournament not open' });
    const { userId, username } = req.body;
    if (t.players.find(p => p.userId === userId))
      return res.status(400).json({ error: 'Already joined' });

    // Deduct entry fee from wallet atomically
    if (db && t.entryFeeNGN > 0) {
      const balRef = db.ref(`users/${userId}/wallet/balance`);
      const result = await balRef.transaction((currentBalance) => {
        const bal = currentBalance || 0;
        if (bal < t.entryFeeNGN) return; // Abort if insufficient
        return bal - t.entryFeeNGN;
      });
      if (!result.committed) {
        return res.status(400).json({ error: 'Insufficient balance for entry fee' });
      }
    }

    t.players.push({ userId, username, joinedAt: Date.now(), wins: 0, losses: 0, goals: 0, points: 0 });
    t.prizePool += t.entryFeeNGN;

    if (t.players.length >= t.maxPlayers) {
      t.status = 'active';
      generateFixtures(t);
      io.emit('tournament_started', { id: t.id, name: t.name });
    }

    io.emit('tournament_update', formatT(t));

    // Send notification (single call, not duplicated)
    try {
      await sendNotification(userId, 'tournament', 'Tournament Joined', `You successfully registered for the ${t.name} tournament. Get ready!`, { tournamentId: t.id });
    } catch (err) {
      console.error('[NOTIF] Error:', err.message);
    }

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
      round: fixture.round,
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

    // Check if round is complete and auto-advance bracket
    await checkProgression(t, io, engine);

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

// ── Bracket generator: single-elimination rounds
function generateFixtures(t) {
  // Round 1: pair players randomly
  const players = [...t.players].sort(() => Math.random() - 0.5);
  t.currentRound = 1;
  t.totalRounds  = Math.ceil(Math.log2(players.length));

  for (let i = 0; i < players.length - 1; i += 2) {
    const id = uuid();
    fixtures.set(id, {
      id,
      tournamentId: t.id,
      round:        1,
      homeId:       players[i].userId,
      homeName:     players[i].username,
      awayId:       players[i + 1].userId,
      awayName:     players[i + 1].username,
      status:       'pending',
      scoreHome:    null,
      scoreAway:    null,
      aiVerified:   false,
      createdAt:    Date.now(),
    });
  }

  // Bye for odd player count
  if (players.length % 2 !== 0) {
    const bye = players[players.length - 1];
    t.byePlayers = t.byePlayers || [];
    t.byePlayers.push({ userId: bye.userId, username: bye.username, round: 1 });
  }
}

// ── After each result, check if the round is complete and advance
async function checkProgression(t, io, engine) {
  const roundFixtures = [...fixtures.values()].filter(
    f => f.tournamentId === t.id && f.round === t.currentRound
  );
  const allDone = roundFixtures.every(f => f.status === 'completed');
  if (!allDone) return;

  // Collect winners of this round
  const winners = roundFixtures.map(f => {
    // If draw, home player advances (deterministic tiebreak)
    const winnerId = f.scoreHome >= f.scoreAway ? f.homeId : f.awayId;
    const winName  = f.scoreHome >= f.scoreAway ? f.homeName : f.awayName;
    return { userId: winnerId, username: winName };
  });

  // Add any bye players from this round
  if (t.byePlayers) {
    t.byePlayers.filter(b => b.round === t.currentRound).forEach(b => winners.push(b));
  }

  // Final: only 1 winner left → distribute prizes
  if (winners.length === 1) {
    t.status    = 'finished';
    t.winnerId  = winners[0].userId;
    t.winnerName= winners[0].username;
    await distributePrizes(t, io);
    io.emit('tournament_finished', { id: t.id, name: t.name, winner: winners[0] });

    try {
      const prize = t.payouts && t.payouts.length > 0 ? t.payouts[0].amount : t.prizePool;
      await sendNotification(t.winnerId, 'tournament', 'Tournament Champion! 🏆', `You won the ${t.name} tournament! Prize money of ₦${prize} has been deposited.`, { tournamentId: t.id, prize });
    } catch (err) {
      console.error('[NOTIF] Error:', err.message);
    }

    return;
  }

  // Advance: create next round fixtures
  t.currentRound++;
  const shuffled = winners.sort(() => Math.random() - 0.5);
  for (let i = 0; i < shuffled.length - 1; i += 2) {
    const id = uuid();
    fixtures.set(id, {
      id,
      tournamentId: t.id,
      round:        t.currentRound,
      homeId:       shuffled[i].userId,
      homeName:     shuffled[i].username,
      awayId:       shuffled[i + 1].userId,
      awayName:     shuffled[i + 1].username,
      status:       'pending',
      scoreHome:    null,
      scoreAway:    null,
      aiVerified:   false,
      createdAt:    Date.now(),
    });
  }
  if (shuffled.length % 2 !== 0) {
    t.byePlayers = t.byePlayers || [];
    t.byePlayers.push({ userId: shuffled[shuffled.length - 1].userId, username: shuffled[shuffled.length - 1].username, round: t.currentRound });
  }

  io.emit('tournament_round_advanced', {
    id: t.id, name: t.name, round: t.currentRound,
    fixtures: [...fixtures.values()].filter(f => f.tournamentId === t.id && f.round === t.currentRound),
  });
  io.emit('tournament_update', formatT(t));

  for (const w of winners) {
    try {
      await sendNotification(w.userId, 'tournament', 'Match Won!', `You won your match and advanced to the next round of ${t.name}!`, { tournamentId: t.id });
    } catch (err) {
      console.error('[NOTIF] Error:', err.message);
    }
  }
}

// ── Distribute prizes to top finishers
async function distributePrizes(t, io) {
  const PRIZE_CUTS = [0.50, 0.25, 0.15]; // 1st, 2nd, 3rd
  const PLATFORM_FEE = 0.10;
  const netPool = Math.round(t.prizePool * (1 - PLATFORM_FEE));

  // Build final standings from round results
  const standings = buildStandings(t);
  const payouts   = [];

  for (let i = 0; i < Math.min(standings.length, PRIZE_CUTS.length); i++) {
    const player = standings[i];
    const amount = Math.round(netPool * PRIZE_CUTS[i]);
    payouts.push({ position: i + 1, userId: player.userId, username: player.username, amount });
    
    // Actually credit the wallet atomically
    if (db) {
      try {
        const balRef = db.ref(`users/${player.userId}/wallet/balance`);
        await balRef.transaction((currentBalance) => {
          return (currentBalance || 0) + amount;
        });

        // Log the transaction
        await db.ref('transactions').push().set({
          type: 'tournament_prize',
          userId: player.userId,
          tournamentId: t.id,
          position: i + 1,
          amount,
          timestamp: Date.now(),
        });
      } catch (e) {
        console.error(`[TOURNAMENT] Failed to credit ${player.userId}:`, e.message);
      }
    }

    // Send Notification
    sendNotification(player.userId, 'tournament', `Tournament Finished - #${i + 1}`, `You finished #${i + 1} in ${t.name}! Prize money of ₦${amount} has been deposited.`, { tournamentId: t.id, prize: amount });
  }

  io.emit('tournament_prizes', { id: t.id, payouts, netPool });
  t.payouts = payouts;
  t.finalStandings = standings;
  console.log(`[TOURNAMENT] Prize distribution for ${t.name}:`, payouts);
}

function buildStandings(t) {
  // Winner of the final is 1st; collect round finishes in reverse
  const roundMax = t.currentRound || 1;
  const standings = [];
  const seen = new Set();

  for (let r = roundMax; r >= 1; r--) {
    const roundF = [...fixtures.values()].filter(f => f.tournamentId === t.id && f.round === r && f.status === 'completed');
    roundF.forEach(f => {
      const winnerId = f.scoreHome >= f.scoreAway ? f.homeId : f.awayId;
      const loserId  = f.scoreHome >= f.scoreAway ? f.awayId : f.homeId;
      const winName  = f.scoreHome >= f.scoreAway ? f.homeName : f.awayName;
      const loseName = f.scoreHome >= f.scoreAway ? f.awayName : f.homeName;
      if (!seen.has(winnerId)) { standings.push({ userId: winnerId, username: winName }); seen.add(winnerId); }
      if (!seen.has(loserId))  { standings.push({ userId: loserId,  username: loseName }); seen.add(loserId); }
    });
  }
  return standings;
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
