require('dotenv').config();
const express    = require('express');
const http       = require('http');
const { Server } = require('socket.io');
const cors       = require('cors');
const helmet     = require('helmet');
const rateLimit  = require('express-rate-limit');
const path       = require('path');
const cron       = require('node-cron');
const { db, admin } = require('./services/firebase');
const webPush = require('web-push');

// ── Web Push Setup (require real keys in production)
const VAPID_PUBLIC = process.env.VAPID_PUBLIC_KEY;
const VAPID_PRIVATE = process.env.VAPID_PRIVATE_KEY;
if (VAPID_PUBLIC && VAPID_PRIVATE) {
  webPush.setVapidDetails('mailto:admin@crestarena.com', VAPID_PUBLIC, VAPID_PRIVATE);
  console.log('[VAPID] ✅ Web Push configured');
} else {
  console.warn('[VAPID] ⚠️ VAPID keys missing — Web Push disabled. Set VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY in .env');
}

// ── CORS — restrict to your domain in production
const ALLOWED_ORIGINS = process.env.ALLOWED_ORIGINS
  ? process.env.ALLOWED_ORIGINS.split(',')
  : ['*'];

const app    = express();
const server = http.createServer(app);
const io     = new Server(server, {
  cors: { origin: ALLOWED_ORIGINS, methods: ['GET','POST'] },
  pingTimeout: 60000,
  pingInterval: 25000,
});

// ── Auth middleware: verifies Firebase ID token
const ADMIN_EMAIL = process.env.ADMIN_EMAIL || 'okafordavis8@gmail.com';

async function verifyFirebaseToken(req, res, next) {
  const authHeader = req.headers.authorization;
  if (!authHeader || !authHeader.startsWith('Bearer ')) {
    return res.status(401).json({ error: 'Missing or invalid authorization header' });
  }
  const idToken = authHeader.split('Bearer ')[1];
  try {
    if (!admin) return next(); // Skip if admin SDK not initialized
    const decoded = await admin.auth().verifyIdToken(idToken);
    req.uid = decoded.uid;
    req.email = decoded.email;
    next();
  } catch (e) {
    return res.status(401).json({ error: 'Invalid or expired token' });
  }
}

function requireAdmin(req, res, next) {
  if (req.email !== ADMIN_EMAIL) {
    return res.status(403).json({ error: 'Admin access required' });
  }
  next();
}

// ── Routes
const matchRoutes      = require('./routes/matches');
const tournamentRoutes = require('./routes/tournaments');
const betRoutes        = require('./routes/bets');
const detectRoutes     = require('./routes/detect');
const footballRoutes   = require('./routes/football');

// ── Payments (try to load if available)
let paymentsRoutes = null;
try {
  paymentsRoutes = require('./routes/payments');
} catch (e) {
  console.log('[INFO] Payment routes not available yet');
}

// ── Game Engine
const GameEngine = require('./services/gameEngine');
const engine     = new GameEngine(io, db, admin);

// ── Middleware
app.use(helmet({ contentSecurityPolicy: false }));
app.use(cors({ origin: ALLOWED_ORIGINS }));
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true }));
app.use('/uploads', express.static(path.join(__dirname, 'uploads')));

// Rate limiting
const limiter = rateLimit({ windowMs: 60000, max: 200 });
app.use('/api/', limiter);

// ── API Routes
const settingsRoutes = require('./routes/settings');
const lobbyRoutes = require('./routes/lobby');

app.use('/api/settings',    settingsRoutes);

// ── Web Push Subscription Route (auth-protected: only the user themselves)
app.post('/api/v1/notifications/subscribe', verifyFirebaseToken, async (req, res) => {
  const uid = req.uid; // from verified token, not from body
  const { subscription } = req.body;
  if (!subscription) return res.status(400).json({ error: 'Missing subscription data' });
  try {
    if (db) await db.ref(`users/${uid}/push_subscription`).set(subscription);
    res.json({ ok: true });
  } catch(e) {
    res.status(500).json({ error: e.message });
  }
});

// Helper to send push
async function sendPushNotification(uid, payload) {
  try {
    if (!db) return;
    const snap = await db.ref(`users/${uid}/push_subscription`).once('value');
    const sub = snap.val();
    if (sub) {
      await webPush.sendNotification(sub, JSON.stringify(payload));
    }
  } catch(e) {
    console.error('[PUSH] Failed:', e.message);
  }
}
app.locals.sendPushNotification = sendPushNotification;

// ── Action-Trigger Referral System (auth-protected)
app.post('/api/v1/referrals/process', verifyFirebaseToken, async (req, res) => {
  const uid = req.uid; // from verified token
  const { action } = req.body;
  if (!action) return res.status(400).json({ error: 'Missing action' });
  
  if (!db) return res.json({ ok: false, error: 'DB not connected' });
  
  try {
    const userSnap = await db.ref(`users/${uid}`).once('value');
    const user = userSnap.val();
    if (!user || !user.pending_referral) return res.json({ ok: true, msg: 'No pending referral' });

    const referrerId = user.pending_referral;
    const rewardAmount = 5;

    // Atomic transaction to credit referrer
    const referrerRef = db.ref(`users/${referrerId}/wallet/balance`);
    await referrerRef.transaction((currentBalance) => {
      return (currentBalance || 0) + rewardAmount;
    });

    // Remove pending referral status
    await db.ref(`users/${uid}/pending_referral`).remove();

    // Fire realtime event and push notification
    if (io) {
      io.to(referrerId).emit('referral_completed', {
        title: 'REFERRAL BONUS',
        message: `You earned ₵${rewardAmount} from a referral!`,
        amount: rewardAmount
      });
    }

    sendPushNotification(referrerId, {
      title: 'Referral Bonus Unlocked! 💸',
      message: `Your referral just made a deposit. ₵${rewardAmount} has been credited to your wallet!`
    });

    res.json({ ok: true, rewarded: true, referrerId });
  } catch(e) {
    console.error('[REFERRAL] Error:', e.message);
    res.status(500).json({ error: e.message });
  }
});

app.use('/api/matches',     matchRoutes(io, engine));
app.use('/api/tournaments', tournamentRoutes(io, engine));
app.use('/api/bets',        betRoutes(io, engine));
app.use('/api/detect',      detectRoutes(io, engine));
app.use('/api/football',    footballRoutes(io, engine));
// Stream data served by /api/streams endpoint above
app.use('/api/lobby',       lobbyRoutes(io, engine, db));

// Payment routes (if available)
if (paymentsRoutes) {
  app.use('/api/payments', paymentsRoutes);
}

// ── Health check
app.get('/health', (req, res) => res.json({
  status: 'ok',
  version: '2.0.0',
  time: new Date().toISOString(),
  activeMatches: engine.getActiveMatchCount(),
}));

// ── Geo-detection proxy (avoids browser CORS issues with ipapi.co)
const axios = require('axios');
app.get('/api/geo', async (req, res) => {
  try {
    let ip = req.headers['x-forwarded-for'] || req.socket.remoteAddress || '';
    if (ip.includes('::1') || ip.includes('127.0.0.1')) ip = ''; // avoid reserved IP error

    const url = ip ? `https://ipapi.co/${ip}/json/` : 'https://ipapi.co/json/';
    const resp = await axios.get(url, { timeout: 3000 });
    
    if (resp.data.error || !resp.data.country_code) {
      throw new Error(resp.data.reason || 'Missing country code');
    }
    res.json({ country_code: resp.data.country_code, country_name: resp.data.country_name });
  } catch (e) {
    res.json({ country_code: 'NG', country_name: 'Nigeria' }); // default
  }
});

// ── Live matches shortcut (for Live Board initial load)
app.get('/api/matches/live', (req, res) => {
  try {
    const matches = engine.getAllMatches ? engine.getAllMatches() : [];
    res.json({ ok: true, matches });
  } catch (e) {
    res.json({ ok: true, matches: [] });
  }
});

// ── Active + ended streams (for Watch tab)
app.get('/api/streams', (req, res) => {
  const streams = [];
  // Live streams
  streamRooms.forEach((room, roomId) => {
    const match = engine.getMatch(room.matchId);
    streams.push({
      roomId,
      matchId: room.matchId,
      streamer: room.username || 'Unknown',
      streamerAvatar: room.avatar || null,
      viewers: room.viewers || 0,
      game: match?.game || room.game || 'CrestArena',
      label: match?.label || 'Live Stream',
      home: match?.home || room.username || 'Player',
      away: match?.away || 'Waiting...',
      scoreHome: match?.scoreHome || 0,
      scoreAway: match?.scoreAway || 0,
      status: 'live',
    });
  });
  // Ended streams (keep for 24h)
  const now = Date.now();
  const DAY_MS = 24 * 60 * 60 * 1000;
  endedStreams.forEach((room, roomId) => {
    if (now - room.endedAt > DAY_MS) {
      endedStreams.delete(roomId);
      return;
    }
    streams.push({
      roomId,
      matchId: room.matchId,
      streamer: room.streamer || 'Unknown',
      streamerAvatar: room.streamerAvatar || null,
      viewers: room.totalViewers || 0,
      game: room.game || 'CrestArena',
      label: room.label || 'Stream',
      home: room.home || 'Player',
      away: room.away || 'Opponent',
      scoreHome: room.scoreHome || 0,
      scoreAway: room.scoreAway || 0,
      status: 'ended',
      endedAt: room.endedAt,
    });
  });
  res.json({ ok: true, streams });
});

// ── Admin: Clear stuck P2P matches (auth + admin only)
app.get('/api/admin/clear_p2p', verifyFirebaseToken, requireAdmin, (req, res) => {
  let count = 0;
  engine.matches.forEach((m, id) => {
    if (m.isP2P) {
      engine.matches.delete(id);
      if (engine.db) engine.db.ref(`live_matches/${id}`).remove();
      count++;
    }
  });
  res.json({ ok: true, message: `Cleared ${count} stuck P2P matches.` });
});

// ── Serve mobile frontend
app.use('/mobile', express.static(path.join(__dirname, '../mobile')));
app.get('/', (req, res) => res.redirect('/mobile'));


// ══════════════════════════════════════════
//  SOCKET.IO — Real-time hub
// ══════════════════════════════════════════
const connectedUsers = new Map();  // socketId → { userId, room }
const streamRooms    = new Map();  // roomId → { host, viewers }
const endedStreams   = new Map();  // roomId → { ...streamData, endedAt, totalViewers }

io.on('connection', (socket) => {
  console.log(`[SOCKET] Connected: ${socket.id}`);

  // ── User auth/identify
  socket.on('identify', ({ userId, username }) => {
    connectedUsers.set(socket.id, { userId, username, rooms: [] });
    socket.emit('identified', { socketId: socket.id });
    io.emit('online_count', connectedUsers.size);
  });

  // ── Join match room (live updates for that match)
  socket.on('join_match', ({ matchId }) => {
    socket.join(`match:${matchId}`);
    const match = engine.getMatch(matchId);
    if (match) socket.emit('match_state', match);
  });

  socket.on('leave_match', ({ matchId }) => {
    socket.leave(`match:${matchId}`);
  });

  // ── Player submits score (mobile game ended)
  socket.on('submit_score', async (data) => {
    /*  data: { matchId, playerId, scoreHome, scoreAway,
                gameTime, screenshot_b64?, method }
        method: 'manual' | 'ai_screenshot' | 'screen_capture' */
    try {
      const result = await engine.submitScore(data);
      // Broadcast to everyone watching this match
      io.to(`match:${data.matchId}`).emit('score_update', result);
      // If match ended, broadcast to all
      if (result.status === 'finished') {
        io.emit('match_finished', result);
        io.to(`match:${data.matchId}`).emit('match_finished', result);
      }
      socket.emit('score_submitted', { ok: true, result });
    } catch (err) {
      socket.emit('score_submitted', { ok: false, error: err.message });
    }
  });

  // ── AI score detection result comes back
  socket.on('ai_score_detected', async (data) => {
    const result = await engine.applyAIScore(data);
    io.to(`match:${data.matchId}`).emit('score_update', result);
    socket.emit('ai_applied', result);
  });

  // ══ STREAMING (WebRTC signaling) ══

  // Host starts stream
  socket.on('stream_start', ({ matchId, username, mode, avatar, game }) => {
    const roomId = `stream:${matchId}`;
    streamRooms.set(roomId, { hostSocket: socket.id, username, avatar: avatar || null, game: game || '', viewers: 0, matchId });
    socket.join(roomId);
    socket.emit('stream_ready', { roomId, matchId });
    io.emit('stream_live', { matchId, username, roomId });
    console.log(`[STREAM] ${username} started stream for match ${matchId}`);
  });

  socket.on('stream_end', ({ roomId }) => {
    const endedRoom = streamRooms.get(roomId);
    if (endedRoom) {
       const match = engine.getMatch(endedRoom.matchId);
       endedStreams.set(roomId, {
         matchId: endedRoom.matchId,
         streamer: endedRoom.username || 'Unknown',
         streamerAvatar: endedRoom.avatar || null,
         totalViewers: endedRoom.viewers || 0,
         game: match?.game || endedRoom.game || 'CrestArena',
         label: match?.label || 'Stream',
         home: match?.home || endedRoom.username || 'Player',
         away: match?.away || 'Opponent',
         scoreHome: match?.scoreHome || 0,
         scoreAway: match?.scoreAway || 0,
         endedAt: Date.now()
       });
       streamRooms.delete(roomId);
       io.emit('stream_ended', { roomId });
       console.log(`[STREAM] Stream ${roomId} ended.`);
    }
  });

  // Viewer joins stream
  socket.on('stream_join', ({ roomId }) => {
    const room = streamRooms.get(roomId);
    if (!room) { socket.emit('stream_error', 'Stream not found'); return; }
    socket.join(roomId);
    room.viewers++;
    socket.emit('stream_joined', { roomId, hostSocket: room.hostSocket });
    // Tell host about new viewer (for WebRTC offer/answer)
    io.to(room.hostSocket).emit('viewer_joined', { viewerSocket: socket.id });
    io.to(roomId).emit('viewer_count', room.viewers);
  });

  // WebRTC signaling relay
  socket.on('webrtc_offer',     ({ to, offer })       => io.to(to).emit('webrtc_offer',     { from: socket.id, offer }));
  socket.on('webrtc_answer',    ({ to, answer })       => io.to(to).emit('webrtc_answer',    { from: socket.id, answer }));
  socket.on('webrtc_ice',       ({ to, candidate })    => io.to(to).emit('webrtc_ice',       { from: socket.id, candidate }));

  // AI analyses stream frame
  socket.on('stream_frame', async ({ matchId, frameBase64 }) => {
    // Throttle: only process 1 frame every 5 seconds per match
    const key = `frame:${matchId}`;
    if (engine.isThrottled(key, 5000)) return;
    engine.setThrottle(key);
    try {
      const detected = await engine.analyzeFrame(matchId, frameBase64);
      if (detected && detected.scoreChanged) {
        io.to(`match:${matchId}`).emit('score_update', detected);
        if (detected.gameEnded) io.emit('match_finished', detected);
      }
    } catch (e) { /* silent */ }
  });

  // ── Tournament room
  socket.on('join_tournament', ({ tournamentId }) => {
    socket.join(`tournament:${tournamentId}`);
  });

  // ── Chat in match room
  socket.on('chat_message', ({ matchId, username, message }) => {
    const msg = { username, message, time: Date.now() };
    io.to(`match:${matchId}`).emit('chat_message', msg);
  });

  // ── Disconnect
  socket.on('disconnect', () => {
    connectedUsers.delete(socket.id);
    // Clean up stream rooms where this socket was host
    streamRooms.forEach((room, roomId) => {
      if (room.hostSocket === socket.id) {
        io.to(roomId).emit('stream_ended', { roomId });
        // Save to ended streams before deleting
        const endedRoom = streamRooms.get(roomId);
        if (endedRoom) {
          const match = engine.getMatch(endedRoom.matchId);
          endedStreams.set(roomId, {
            matchId: endedRoom.matchId,
            streamer: endedRoom.username || 'Unknown',
            streamerAvatar: endedRoom.avatar || null,
            totalViewers: endedRoom.viewers || 0,
            game: match?.game || endedRoom.game || 'CrestArena',
            label: match?.label || 'Stream',
            home: match?.home || endedRoom.username || 'Player',
            away: match?.away || 'Opponent',
            scoreHome: match?.scoreHome || 0,
            scoreAway: match?.scoreAway || 0,
            endedAt: Date.now(),
          });
        }
        streamRooms.delete(roomId);
      }
    });
    io.emit('online_count', connectedUsers.size);
    console.log(`[SOCKET] Disconnected: ${socket.id}`);
  });
});

// ══════════════════════════════════════════
//  CRON JOBS
// ══════════════════════════════════════════

// Every 30s: tick all active matches (increment game time) + cleanup throttles
cron.schedule('*/30 * * * * *', () => {
  engine.tickAllMatches(io);
  engine.cleanupThrottles();
});

// Every 2 minutes: fetch real football scores from API (free tier = 100 calls/day)
cron.schedule('*/2 * * * *', async () => {
  try {
    const { syncFootballScores } = require('./services/footballAPI');
    const updates = await syncFootballScores();
    if (updates && updates.length) {
      updates.forEach(u => io.to(`match:${u.matchId}`).emit('score_update', u));
    }
  } catch (e) { /* ignore */ }
});

// ══════════════════════════════════════════
//  START
// ══════════════════════════════════════════
const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`
╔═══════════════════════════════════════╗
║   BETELITE Backend v2.0               ║
║   Port: ${PORT}                          ║
║   Mobile: http://localhost:${PORT}/mobile ║
║   API:    http://localhost:${PORT}/api    ║
╚═══════════════════════════════════════╝`);
  engine.init();
});

module.exports = { app, io };
