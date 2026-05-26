// routes/bets.js
const express = require('express');
const { v4: uuid } = require('uuid');
const { db } = require('../services/firebase');
const bets = new Map();

module.exports = (io, engine) => {
  const r = express.Router();

  r.post('/place', async (req, res) => {
    const { userId, matchId, pick, odds, amount, currency } = req.body;
    if (!userId || !matchId || !pick || !odds || !amount)
      return res.status(400).json({ error: 'Missing fields' });

    const betAmount = parseFloat(amount);
    if (betAmount <= 0) return res.status(400).json({ error: 'Invalid amount' });

    // Validate and deduct balance atomically
    if (db) {
      const balRef = db.ref(`users/${userId}/wallet/balance`);
      const result = await balRef.transaction((currentBalance) => {
        const bal = currentBalance || 0;
        if (bal < betAmount) return; // Abort if insufficient
        return bal - betAmount;
      });
      if (!result.committed) {
        return res.status(400).json({ error: 'Insufficient balance' });
      }
    }

    const bet = {
      id: uuid(), userId, matchId, pick,
      odds: parseFloat(odds), amount: betAmount,
      currency: currency || 'NGN', status: 'live',
      potentialWin: Math.round(betAmount * parseFloat(odds)),
      placedAt: Date.now()
    };
    bets.set(bet.id, bet);

    // Persist bet to Firebase for settlement
    if (db) {
      db.ref(`bets/${bet.id}`).set(bet).catch(e => console.error('[BET] Persist error:', e.message));
    }

    res.json({ ok: true, bet });
  });

  r.get('/user/:userId', (req, res) => {
    const userBets = [...bets.values()].filter(b => b.userId === req.params.userId);
    res.json({ bets: userBets });
  });

  return r;
};
