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
        const snap = await db.ref(`users/${uid}/wallet/balance`).once('value');
        const balance = snap.val() || 0;
        if (balance < Number(amount)) {
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

  // Delete a challenge (only by creator)
  router.post('/delete', (req, res) => {
    try {
      const { challengeId, uid } = req.body;
      const challenge = lobbyChallenges.get(challengeId);
      if (!challenge) return res.status(404).json({ ok: false, error: 'Wager not found' });
      if (challenge.creatorId !== uid) return res.status(403).json({ ok: false, error: 'You can only delete your own wagers' });
      
      lobbyChallenges.delete(challengeId);
      io.emit('lobby_challenge_removed', { id: challengeId });
      console.log(`[LOBBY] Wager ${challengeId} deleted by ${uid}`);
      res.json({ ok: true });
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  });
  // Accept a challenge
  router.post('/accept', async (req, res) => {
    try {
      const { challengeId, uid, username } = req.body;
      const challenge = lobbyChallenges.get(challengeId);
      
      if (!challenge) return res.status(404).json({ error: 'Challenge not found or already accepted' });
      if (challenge.creatorId === uid) return res.status(400).json({ error: 'Cannot accept your own challenge' });

      // Atomically deduct from both wallets into escrow using transactions
      if (db) {
        // Validate and deduct creator balance atomically
        const creatorBalRef = db.ref(`users/${challenge.creatorId}/wallet/balance`);
        const creatorResult = await creatorBalRef.transaction((currentBalance) => {
          const bal = currentBalance || 0;
          if (bal < challenge.amount) return; // Abort transaction (returns undefined)
          return bal - challenge.amount;
        });
        if (!creatorResult.committed) {
          lobbyChallenges.delete(challengeId);
          io.emit('lobby_challenge_removed', { id: challengeId });
          return res.status(400).json({ error: 'Challenger no longer has sufficient funds' });
        }

        // Validate and deduct acceptor balance atomically
        const acceptorBalRef = db.ref(`users/${uid}/wallet/balance`);
        const acceptorResult = await acceptorBalRef.transaction((currentBalance) => {
          const bal = currentBalance || 0;
          if (bal < challenge.amount) return; // Abort
          return bal - challenge.amount;
        });
        if (!acceptorResult.committed) {
          // Refund creator since we already deducted
          await creatorBalRef.transaction((currentBalance) => {
            return (currentBalance || 0) + challenge.amount;
          });
          return res.status(400).json({ error: 'Insufficient balance to accept this challenge' });
        }

        // Record escrow
        const escrowId = `esc_${challengeId}`;
        await db.ref(`escrow/${escrowId}`).set({
          challengeId,
          creatorId:  challenge.creatorId,
          acceptorId: uid,
          amount:     challenge.amount,
          pool:       challenge.amount * 2,
          status:     'held',
          heldAt:     Date.now(),
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
