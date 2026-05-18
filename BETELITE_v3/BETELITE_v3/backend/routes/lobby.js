const express = require('express');
const { v4: uuid } = require('uuid');

// In-memory lobby challenges
// Map of challengeId -> challenge details
const lobbyChallenges = new Map();

module.exports = (io, engine, db) => {
  const router = express.Router();

  // Get all active open challenges
  router.get('/', (req, res) => {
    try {
      const challenges = Array.from(lobbyChallenges.values());
      res.json({ ok: true, challenges });
    } catch (e) {
      res.status(500).json({ error: e.message });
    }
  });

  // Create a new challenge
  router.post('/create', async (req, res) => {
    try {
      const { uid, username, game, amount, platform } = req.body;
      if (!uid || !amount || !game) return res.status(400).json({ error: 'Missing fields' });

      // Check creator balance before allowing challenge post
      if (db) {
        const snap = await db.ref(`users/${uid}`).once('value');
        const user = snap.val() || {};
        if ((user.balance || 0) < Number(amount)) {
          return res.status(400).json({ error: 'Insufficient balance to post this challenge' });
        }
      }

      const challengeId = `ch_${uuid().substring(0, 8)}`;
      const challenge = {
        id: challengeId,
        creatorId: uid,
        creatorName: username || 'Unknown',
        game,
        amount: Number(amount),
        platform: platform || 'mobile',
        createdAt: Date.now()
      };

      lobbyChallenges.set(challengeId, challenge);
      
      // Broadcast to lobby
      io.emit('lobby_new_challenge', challenge);

      res.json({ ok: true, challenge });
    } catch (e) {
      res.status(500).json({ error: e.message });
    }
  });

  // Accept a challenge
  router.post('/accept', async (req, res) => {
    try {
      const { challengeId, uid, username } = req.body;
      const challenge = lobbyChallenges.get(challengeId);
      
      if (!challenge) return res.status(404).json({ error: 'Challenge not found or already accepted' });
      if (challenge.creatorId === uid) return res.status(400).json({ error: 'Cannot accept your own challenge' });

      // Atomically deduct from both wallets into escrow
      if (db) {
        const creatorRef = db.ref(`users/${challenge.creatorId}`);
        const acceptorRef = db.ref(`users/${uid}`);

        const [cSnap, aSnap] = await Promise.all([
          creatorRef.once('value'),
          acceptorRef.once('value'),
        ]);
        const cUser = cSnap.val() || {};
        const aUser = aSnap.val() || {};

        // Re-validate both balances at accept time
        if ((cUser.balance || 0) < challenge.amount) {
          lobbyChallenges.delete(challengeId);
          io.emit('lobby_challenge_removed', { id: challengeId });
          return res.status(400).json({ error: 'Challenger no longer has sufficient funds' });
        }
        if ((aUser.balance || 0) < challenge.amount) {
          return res.status(400).json({ error: 'Insufficient balance to accept this challenge' });
        }

        // Atomic multi-path update: deduct both + record escrow
        const escrowId = `esc_${challengeId}`;
        await db.ref().update({
          [`users/${challenge.creatorId}/balance`]: (cUser.balance || 0) - challenge.amount,
          [`users/${uid}/balance`]:                 (aUser.balance || 0) - challenge.amount,
          [`escrow/${escrowId}`]: {
            challengeId,
            creatorId:  challenge.creatorId,
            acceptorId: uid,
            amount:     challenge.amount,
            pool:       challenge.amount * 2,
            status:     'held',
            heldAt:     Date.now(),
          },
        });
        console.log(`[ESCROW] ₦${challenge.amount * 2} held for challenge ${challengeId}`);

        // Log transaction for audit trail
        await db.ref('transactions').push().set({
          type:         'escrow_hold',
          amount:       challenge.amount * 2,
          participants: [challenge.creatorId, uid],
          escrowId,
          challengeId,
          timestamp:    Date.now(),
        });
      }

      // Create Match in GameEngine
      const match = engine.createP2PMatch({
        gameType: challenge.game,
        home: challenge.creatorName,
        away: username || 'Opponent',
        homeId: challenge.creatorId,
        awayId: uid,
        wagerPool: challenge.amount * 2,
        wagerAmount: challenge.amount,
        status: 'live' // auto start
      });

      // Remove from lobby
      lobbyChallenges.delete(challengeId);
      
      // Broadcast match created so both clients can redirect
      io.emit('lobby_challenge_accepted', {
        challengeId,
        matchId: match.id,
        creatorId: challenge.creatorId,
        acceptorId: uid
      });

      res.json({ ok: true, matchId: match.id });
    } catch (e) {
      res.status(500).json({ error: e.message });
    }
  });

  return router;
};
