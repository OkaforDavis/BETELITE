// routes/bets.js
const express = require('express');
const prisma = require('../prismaClient');
const { authenticateUser } = require('../middleware/auth');

module.exports = (io, engine) => {
  const r = express.Router();

  // Apply auth middleware to all bet routes
  r.use(authenticateUser);

  r.post('/place', async (req, res) => {
    try {
      const { matchId, pick, odds, amount, currency } = req.body;
      const user = req.user; // from authenticateUser middleware

      if (!matchId || !pick || !odds || !amount) {
        return res.status(400).json({ error: 'Missing fields' });
      }

      // Check wallet balance
      const wallet = await prisma.wallet.findUnique({ where: { userId: user.id } });
      if (!wallet || wallet.balanceAvailable < amount) {
        return res.status(400).json({ error: 'Insufficient balance' });
      }

      // Deduct balance and create bet in transaction
      const bet = await prisma.$transaction(async (tx) => {
        // Deduct from wallet
        await tx.wallet.update({
          where: { userId: user.id },
          data: { balanceAvailable: { decrement: amount } }
        });

        // Record transaction
        await tx.walletTransaction.create({
          data: {
            walletId: wallet.walletId,
            transactionType: 'bet_placed',
            amount: amount,
            balanceBefore: wallet.balanceAvailable,
            balanceAfter: wallet.balanceAvailable - amount,
            transactionStatus: 'completed'
          }
        });

        // Place bet
        return await tx.bet.create({
          data: {
            userId: user.id,
            matchId: BigInt(matchId),
            betType: 'match_winner', // Assuming standard for now
            betAmount: amount,
            odds: odds,
            potentialPayout: Math.round(amount * odds),
            betStatus: 'pending'
          }
        });
      });

      // Serialization for BigInt
      const serializedBet = { ...bet, betId: bet.betId.toString(), userId: bet.userId.toString(), matchId: bet.matchId.toString() };

      res.json({ ok: true, bet: serializedBet });
    } catch (e) {
      console.error('[BETS PLACE]', e.message);
      res.status(500).json({ ok: false, error: 'Failed to place bet' });
    }
  });

  r.get('/user/:userId', async (req, res) => {
    try {
      const { userId } = req.params;
      
      // Basic check to ensure users only fetch their own bets unless admin (omitted for simplicity, but req.user.id can be checked)
      if (req.user.id.toString() !== userId) {
         return res.status(403).json({ error: 'Forbidden: Can only view your own bets' });
      }

      const userBets = await prisma.bet.findMany({
        where: { userId: BigInt(userId) },
        orderBy: { placedAt: 'desc' }
      });

      // Convert BigInt to strings
      const serializedBets = userBets.map(b => ({
        ...b,
        betId: b.betId.toString(),
        userId: b.userId.toString(),
        matchId: b.matchId.toString(),
      }));

      res.json({ bets: serializedBets });
    } catch (e) {
      console.error('[BETS GET]', e.message);
      res.status(500).json({ ok: false, error: 'Failed to get bets' });
    }
  });

  return r;
};
