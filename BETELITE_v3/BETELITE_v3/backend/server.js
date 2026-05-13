require('dotenv').config();
const express    = require('express');
const http       = require('http');
const { Server } = require('socket.io');
const cors       = require('cors');
const helmet     = require('helmet');
const rateLimit  = require('express-rate-limit');
const path       = require('path');
const cron       = require('node-cron');

const app    = express();
const server = http.createServer(app);
const io     = new Server(server, {
  cors: { origin: '*', methods: ['GET','POST'] },
  pingTimeout: 60000,
  pingInterval: 25000,
});

// ── Routes
const matchRoutes      = require('./routes/matches');
const tournamentRoutes = require('./routes/tournaments');
const betRoutes        = require('./routes/bets');
const detectRoutes     = require('./routes/detect');
const streamRoutes     = require('./routes/stream');
const footballRoutes   = require('./routes/football');

// ── Payments & Auth (try to load if available)
let paymentsRoutes = null;
let authRoutes = null;
try {
  // These may not be compiled yet, will fallback gracefully
  paymentsRoutes = require('./routes/payments');
} catch (e) {
  console.log('[INFO] Payment routes not available yet');
}

// ── Firebase
const { db } = require('./services/firebase');

// ── Game Engine
const GameEngine = require('./services/gameEngine');
const engine     = new GameEngine(io, db);

// ── Middleware
app.use(helmet({ contentSecurityPolicy: false }));
app.use(cors({ origin: '*' }));
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true }));
app.use('/uploads', express.static(path.join(__dirname, 'uploads')));

// Rate limiting
const limiter = rateLimit({ windowMs: 60000, max: 200 });
app.use('/api/', limiter);

// ── API Routes
const settingsRoutes = require('./routes/settings');
app.use('/api/settings',    settingsRoutes);
app.use('/api/matches',     matchRoutes(io, engine));
app.use('/api/tournaments', tournamentRoutes(io, engine));
app.use('/api/bets',        betRoutes(io, engine));
app.use('/api/detect',      detectRoutes(io, engine));
app.use('/api/stream',      streamRoutes(io));
app.use('/api/football',    footballRoutes(io, engine));

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

// ── Serve mobile frontend
app.use('/mobile', express.static(path.join(__dirname, '../mobile')));
app.get('/', (req, res) => res.redirect('/mobile'));

// ══════════════════════════════════════════
//  SOCKET.IO — Real-time hub
// ══════════════════════════════════════════
const connectedUsers = new Map();  // socketId → { userId, room }
const streamRooms    = new Map();  // roomId → { host, viewers }

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
  socket.on('stream_start', ({ matchId, username }) => {
    const roomId = `stream:${matchId}`;
    streamRooms.set(roomId, { hostSocket: socket.id, username, viewers: 0, matchId });
    socket.join(roomId);
    socket.emit('stream_ready', { roomId, matchId });
    io.emit('stream_live', { matchId, username, roomId });
    console.log(`[STREAM] ${username} started stream for match ${matchId}`);
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

// Every 30s: tick all active matches (increment game time)
cron.schedule('*/30 * * * * *', () => {
  engine.tickAllMatches(io);
});

// Every 15s: fetch real football scores from API
cron.schedule('*/15 * * * * *', async () => {
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
