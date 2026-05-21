// routes/bets.js
const express = require('express');
const { v4: uuid } = require('uuid');
const bets = new Map();

module.exports = (io, engine) => {
  const r = express.Router();

  r.post('/place', (req, res) => {
    const { userId, matchId, pick, odds, amount, currency } = req.body;
    if (!userId||!matchId||!pick||!odds||!amount)
      return res.status(400).json({ error: 'Missing fields' });

    const bet = { id:uuid(), userId, matchId, pick,
      odds:parseFloat(odds), amount:parseFloat(amount),
      currency: currency||'NGN', status:'live',
      potentialWin: Math.round(amount*odds), placedAt: Date.now() };
    bets.set(bet.id, bet);
    res.json({ ok:true, bet });
  });

  r.get('/user/:userId', (req, res) => {
    const userBets = [...bets.values()].filter(b=>b.userId===req.params.userId);
    res.json({ bets: userBets });
  });

  return r;
};
