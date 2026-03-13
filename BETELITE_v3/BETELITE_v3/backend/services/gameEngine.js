const { v4: uuid } = require('uuid');
const Anthropic = require('@anthropic-ai/sdk');

const GAME_CONFIGS = {
  efootball: {
    label: 'eFootball', icon: '⚽', duration: 90, etExtra: 3,
    statKeys: ['shots','possession','fouls'],
    endTrigger: 'time', // ends at 90+ET
  },
  dream: {
    label: 'Dream League Soccer', icon: '🏟️', duration: 90, etExtra: 3,
    statKeys: ['shots','possession','corners'],
    endTrigger: 'time',
  },
  fifa: {
    label: 'FIFA Mobile', icon: '🏆', duration: 90, etExtra: 3,
    statKeys: ['shots','possession','fouls'],
    endTrigger: 'time',
  },
  cod: {
    label: 'COD Mobile', icon: '🎯', duration: 10, // 10 rounds
    statKeys: ['kills','deaths','assists'],
    endTrigger: 'score', // ends when team reaches 6 kills/points
    winScore: 6,
  },
  freefire: {
    label: 'Free Fire', icon: '🔥', duration: 30, // 30 min match
    statKeys: ['kills','damage','survived'],
    endTrigger: 'time',
  },
};

const MATCH_STATUS = {
  SCHEDULED: 'scheduled',
  LIVE:      'live',
  ADDED_TIME:'added_time', // 90+1, 90+2, etc.
  UPDATING:  'updating',   // "Updating scores..." state
  FINISHED:  'finished',
};

class GameEngine {
  constructor(io, db) {
    this.io       = io;
    this.db       = db;
    this.matches  = new Map();  // matchId → match object
    this.throttles= new Map();  // key → timestamp
    this.client   = process.env.ANTHROPIC_API_KEY
      ? new Anthropic({ apiKey: process.env.ANTHROPIC_API_KEY })
      : null;
  }

  async init() {
    console.log('[ENGINE] Initializing game engine...');
    // Load any persisted matches from Firebase
    try {
      if (this.db) {
        const snap = await this.db.ref('live_matches').once('value');
        const data = snap.val();
        if (data) {
          Object.entries(data).forEach(([id, m]) => {
            if (m.status !== MATCH_STATUS.FINISHED) this.matches.set(id, m);
          });
          console.log(`[ENGINE] Restored ${this.matches.size} live matches`);
        }
      }
    } catch (e) { console.log('[ENGINE] Firebase not connected, using in-memory'); }
    // Seed demo matches
    this._seedMatches();
  }

  _seedMatches() {
    const games = Object.entries(GAME_CONFIGS);
    games.forEach(([gameType, cfg]) => {
      const id = uuid();
      const match = this._createMatch(id, gameType, cfg);
      this.matches.set(id, match);
    });
    console.log(`[ENGINE] Seeded ${this.matches.size} demo matches`);
  }

  _createMatch(id, gameType, cfg, overrides = {}) {
    const teams = this._getTeams(gameType);
    const startMin = 15 + Math.floor(Math.random() * 60);
    return {
      id,
      gameType,
      icon: cfg.icon,
      label: cfg.label,
      home: teams[0],
      away: teams[1],
      scoreHome: Math.floor(Math.random() * 3),
      scoreAway: Math.floor(Math.random() * 3),
      minute: startMin,
      addedTime: 0,       // extra time minutes shown as 90+N
      status: MATCH_STATUS.LIVE,
      endTrigger: cfg.endTrigger,
      maxDuration: cfg.duration,
      etExtra: cfg.etExtra || 0,
      winScore: cfg.winScore || null,
      stats: { home: this._randStats(cfg), away: this._randStats(cfg) },
      cheatFlags: [],
      verifiedBy: 'engine', // 'engine' | 'ai_screenshot' | 'ai_stream' | 'player'
      lastUpdate: Date.now(),
      createdAt: Date.now(),
      ...overrides,
    };
  }

  _randStats(cfg) {
    const s = {};
    (cfg.statKeys || []).forEach(k => {
      if (k === 'possession') s[k] = 40 + Math.floor(Math.random() * 20);
      else s[k] = Math.floor(Math.random() * 12);
    });
    return s;
  }

  _getTeams(gameType) {
    const pools = {
      efootball: ['FC Barcelona','Real Madrid','Man City','Liverpool','PSG','Bayern','Juventus','Chelsea','Arsenal','Dortmund','Inter','AC Milan'],
      dream:     ['Lagos FC','Accra Giants','Nairobi Kings','Cairo United','Cape Town FC','Tunis Elite','Kumasi FC','Ibadan Utd','Kano Stars','Enugu FC','Abuja FC','Dakar FC'],
      fifa:      ['Al Nassr','Al Hilal','Flamengo','Boca Juniors','Santos FC','Atletico','Sevilla','Porto FC','Benfica','Ajax','Celtic','Rangers'],
      cod:       ['Ghost Squad','Reaper Team','Alpha Force','Dark Ops','Shadow Co.','Iron Fist','Storm Unit','Wolf Pack','Phantom GG','Viper Squad'],
      freefire:  ['Blaze Squad','Phoenix FF','Inferno GG','Nova Wolves','Delta Strike','Omega Kings','Venom FF','Apex Fire','Thunder GG','Storm Riders'],
    };
    const pool = pools[gameType] || pools.efootball;
    const shuffled = [...pool].sort(() => Math.random() - 0.5);
    return [shuffled[0], shuffled[1]];
  }

  // ══════════════════════════════════════════
  //  TICK — called every 30 seconds by cron
  // ══════════════════════════════════════════
  tickAllMatches(io) {
    this.matches.forEach((match, id) => {
      if (match.status === MATCH_STATUS.FINISHED) return;
      if (match.status === MATCH_STATUS.UPDATING) return;

      const cfg = GAME_CONFIGS[match.gameType];

      // Increment game time
      match.minute += 0.5; // 30s real = 0.5 game minutes (compressed)

      // Random stat increments
      this._tickStats(match, cfg);

      // Random scoring events
      if (Math.random() < 0.04) {
        if (Math.random() < 0.5) match.scoreHome++;
        else match.scoreAway++;
        match.verifiedBy = 'engine';
        io.emit('goal_scored', { matchId: id, home: match.scoreHome, away: match.scoreAway, label: match.label });
      }

      // Cheat detection
      this._detectCheats(match);

      // ── END CONDITIONS ──
      const ended = this._checkEndCondition(match, cfg);
      if (ended) {
        this._endMatch(match, io);
      } else if (match.minute >= match.maxDuration && match.status === MATCH_STATUS.LIVE) {
        // Enter added time phase
        match.status = MATCH_STATUS.ADDED_TIME;
        match.addedTime = 0;
        io.to(`match:${id}`).emit('added_time_start', { matchId: id, etExtra: match.etExtra });
      } else if (match.status === MATCH_STATUS.ADDED_TIME) {
        match.addedTime += 0.5;
        if (match.addedTime >= match.etExtra) {
          // Game ENDS here - go to "updating" briefly then finish
          match.status = MATCH_STATUS.UPDATING;
          io.to(`match:${id}`).emit('score_updating', {
            matchId: id,
            message: 'Updating scores...',
            expectedIn: 5000, // 5 seconds
          });
          // After 5 seconds, finalize
          setTimeout(() => this._endMatch(match, io), 5000);
        }
      }

      match.lastUpdate = Date.now();
      io.to(`match:${id}`).emit('match_tick', this._publicMatch(match));

      // Persist to Firebase
      this._persist(match);
    });
  }

  _tickStats(match, cfg) {
    if (!cfg) return;
    cfg.statKeys?.forEach(k => {
      if (k === 'possession') return; // handled separately
      if (Math.random() < 0.15) match.stats.home[k] = (match.stats.home[k] || 0) + 1;
      if (Math.random() < 0.15) match.stats.away[k] = (match.stats.away[k] || 0) + 1;
    });
    if (cfg.statKeys?.includes('possession')) {
      match.stats.home.possession = 35 + Math.floor(Math.random() * 30);
      match.stats.away.possession = 100 - match.stats.home.possession;
    }
  }

  _checkEndCondition(match, cfg) {
    if (cfg.endTrigger === 'score' && cfg.winScore) {
      return match.scoreHome >= cfg.winScore || match.scoreAway >= cfg.winScore;
    }
    return false;
  }

  _detectCheats(match) {
    const flags = [];
    // Impossible scores
    if (match.scoreHome > 15 || match.scoreAway > 15) flags.push('impossible_score');
    // Score went backwards (shouldn't happen normally)
    if (match.scoreHome < 0 || match.scoreAway < 0) flags.push('negative_score');
    // Kills too high for COD round
    if (match.gameType === 'cod') {
      const hk = match.stats?.home?.kills || 0;
      const ak = match.stats?.away?.kills || 0;
      if (hk > 50 || ak > 50) flags.push('abnormal_kill_count');
    }
    if (flags.length) {
      match.cheatFlags = [...new Set([...match.cheatFlags, ...flags])];
      if (this.io) this.io.to(`match:${match.id}`).emit('cheat_detected', { matchId: match.id, flags });
    }
  }

  _endMatch(match, io) {
    match.status = MATCH_STATUS.FINISHED;
    match.finishedAt = Date.now();
    // Determine winner
    match.result = {
      winner: match.scoreHome > match.scoreAway ? 'home'
            : match.scoreAway > match.scoreHome ? 'away' : 'draw',
      homeScore: match.scoreHome,
      awayScore: match.scoreAway,
      finalMinute: `${match.maxDuration}+${Math.ceil(match.addedTime)}`,
    };

    console.log(`[ENGINE] Match finished: ${match.home} ${match.scoreHome}-${match.scoreAway} ${match.away}`);

    if (io) {
      io.to(`match:${match.id}`).emit('match_finished', this._publicMatch(match));
      io.emit('match_result', {
        matchId: match.id,
        home: match.home, away: match.away,
        score: `${match.scoreHome}-${match.scoreAway}`,
        winner: match.result.winner,
        label: match.label,
      });
    }

    // Settle bets via Firebase
    this._settleBets(match);

    // Persist final result
    this._persist(match);

    // Replace with a new scheduled match after 30s
    setTimeout(() => this._scheduleNewMatch(match.gameType), 30000);
  }

  _scheduleNewMatch(gameType) {
    const cfg = GAME_CONFIGS[gameType];
    if (!cfg) return;
    const id = uuid();
    const match = this._createMatch(id, gameType, cfg, { minute: 0, status: MATCH_STATUS.LIVE });
    this.matches.set(id, match);
    if (this.io) this.io.emit('new_match', this._publicMatch(match));
    console.log(`[ENGINE] New match scheduled: ${match.home} vs ${match.away}`);
  }

  // ══════════════════════════════════════════
  //  PLAYER SUBMITS SCORE
  // ══════════════════════════════════════════
  async submitScore({ matchId, playerId, scoreHome, scoreAway, gameTime, method }) {
    const match = this.matches.get(matchId);
    if (!match) throw new Error('Match not found');
    if (match.status === MATCH_STATUS.FINISHED) throw new Error('Match already finished');

    // Validate submission
    const prevHome = match.scoreHome;
    const prevAway = match.scoreAway;
    const diff = Math.abs((scoreHome - prevHome) + (scoreAway - prevAway));
    if (diff > 5) {
      match.cheatFlags.push('suspicious_submission');
      if (this.io) this.io.to(`match:${matchId}`).emit('cheat_detected', { matchId, flags: ['suspicious_submission'] });
    }

    match.scoreHome = scoreHome;
    match.scoreAway = scoreAway;
    match.verifiedBy = method || 'player';
    match.lastUpdate = Date.now();

    // Check if this ends the game
    const cfg = GAME_CONFIGS[match.gameType];
    if (method === 'game_end' || this._checkEndCondition(match, cfg)) {
      match.status = MATCH_STATUS.UPDATING;
      if (this.io) this.io.to(`match:${matchId}`).emit('score_updating', {
        matchId, message: 'Updating scores...', expectedIn: 3000,
      });
      setTimeout(() => this._endMatch(match, this.io), 3000);
    }

    this._persist(match);
    return this._publicMatch(match);
  }

  // ══════════════════════════════════════════
  //  AI SCREENSHOT ANALYSIS
  // ══════════════════════════════════════════
  async analyzeScreenshot(matchId, imageBase64, mimeType = 'image/jpeg') {
    if (!this.client) {
      return { error: 'Anthropic API key not configured', demo: true, ...this._mockAI() };
    }

    try {
      const response = await this.client.messages.create({
        model: 'claude-opus-4-5',
        max_tokens: 500,
        messages: [{
          role: 'user',
          content: [
            {
              type: 'image',
              source: { type: 'base64', media_type: mimeType, data: imageBase64 },
            },
            {
              type: 'text',
              text: `You are BETELITE's AI score detection engine for mobile esports.
Analyze this screenshot and extract ALL of the following:

1. game: exact game name (eFootball / COD Mobile / Free Fire / FIFA Mobile / Dream League Soccer / other)
2. team1: home team or player name
3. team2: away team or player name  
4. score1: home score (number)
5. score2: away score (number)
6. gameTime: current time/round shown (e.g. "67'", "Round 5", "25:30")
7. gameEnded: true if this shows final result / game over screen
8. finalResult: true if match is definitively over
9. cheatDetected: true if you see any suspicious editing, impossible scores, or anomalies
10. cheatReason: explain any suspicious findings
11. confidence: 0.0-1.0 how confident you are
12. notes: any other relevant info

Respond with ONLY valid JSON. No markdown. No explanation.`,
            },
          ],
        }],
      });

      const raw = response.content[0].text.replace(/```json|```/g, '').trim();
      const parsed = JSON.parse(raw);
      return { ...parsed, source: 'anthropic_vision', matchId };
    } catch (e) {
      console.error('[AI] Screenshot analysis error:', e.message);
      return { error: e.message, demo: true, ...this._mockAI() };
    }
  }

  // ══════════════════════════════════════════
  //  AI STREAM FRAME ANALYSIS
  // ══════════════════════════════════════════
  async analyzeFrame(matchId, frameBase64) {
    const match = this.matches.get(matchId);
    if (!this.client || !match) return null;

    try {
      const response = await this.client.messages.create({
        model: 'claude-opus-4-5',
        max_tokens: 200,
        messages: [{
          role: 'user',
          content: [
            {
              type: 'image',
              source: { type: 'base64', media_type: 'image/jpeg', data: frameBase64 },
            },
            {
              type: 'text',
              text: `Quick score extraction from game stream. 
Current known score: ${match.scoreHome}-${match.scoreAway} at ${match.minute}'.
Extract: {"score1":N,"score2":N,"time":"","gameEnded":bool,"scoreChanged":bool}
JSON only.`,
            },
          ],
        }],
      });

      const parsed = JSON.parse(response.content[0].text.replace(/```json|```/g, '').trim());

      if (parsed.scoreChanged) {
        match.scoreHome = parsed.score1 ?? match.scoreHome;
        match.scoreAway = parsed.score2 ?? match.scoreAway;
        match.verifiedBy = 'ai_stream';
        match.lastUpdate = Date.now();
        this._persist(match);
      }

      return { ...parsed, matchId, ...this._publicMatch(match) };
    } catch (e) {
      return null;
    }
  }

  async applyAIScore({ matchId, score1, score2, gameEnded, cheatDetected, cheatReason }) {
    return this.submitScore({
      matchId, scoreHome: score1, scoreAway: score2,
      method: gameEnded ? 'game_end' : 'ai_screenshot',
    });
  }

  _mockAI() {
    return {
      game: 'eFootball', team1: 'FC Barcelona', team2: 'Real Madrid',
      score1: 2, score2: 1, gameTime: "67'", gameEnded: false,
      cheatDetected: false, confidence: 0.85, notes: 'Demo mode',
    };
  }

  // ══════════════════════════════════════════
  //  BET SETTLEMENT
  // ══════════════════════════════════════════
  async _settleBets(match) {
    if (!this.db) return;
    try {
      const snap = await this.db.ref(`bets`).orderByChild('matchId').equalTo(match.id).once('value');
      const bets = snap.val();
      if (!bets) return;

      const updates = {};
      Object.entries(bets).forEach(([betId, bet]) => {
        if (bet.status !== 'pending' && bet.status !== 'live') return;
        let won = false;
        if (bet.pick === 'home' && match.result.winner === 'home') won = true;
        if (bet.pick === 'away' && match.result.winner === 'away') won = true;
        if (bet.pick === 'draw' && match.result.winner === 'draw') won = true;

        updates[`bets/${betId}/status`] = won ? 'won' : 'lost';
        if (won) {
          const payout = Math.round(bet.amount * bet.odds);
          updates[`users/${bet.userId}/wallet`] = (this.db.ServerValue?.increment || 0) + payout;
          this.io?.to(`user:${bet.userId}`).emit('bet_won', { betId, payout, match: `${match.home} vs ${match.away}` });
        } else {
          this.io?.to(`user:${bet.userId}`).emit('bet_lost', { betId });
        }
      });

      await this.db.ref().update(updates);
    } catch (e) {
      console.error('[ENGINE] Bet settlement error:', e.message);
    }
  }

  async _persist(match) {
    if (!this.db) return;
    try {
      await this.db.ref(`live_matches/${match.id}`).set(this._publicMatch(match));
    } catch (e) { /* silent */ }
  }

  // ── Getters
  getMatch(id) { return this._publicMatch(this.matches.get(id)); }
  getAllMatches() { return [...this.matches.values()].map(m => this._publicMatch(m)); }
  getActiveMatchCount() { return [...this.matches.values()].filter(m => m.status !== MATCH_STATUS.FINISHED).length; }

  _publicMatch(m) {
    if (!m) return null;
    return {
      id: m.id, gameType: m.gameType, icon: m.icon, label: m.label,
      home: m.home, away: m.away,
      scoreHome: m.scoreHome, scoreAway: m.scoreAway,
      minute: Math.min(Math.round(m.minute), m.maxDuration),
      addedTime: m.addedTime || 0,
      displayTime: this._displayTime(m),
      status: m.status, result: m.result || null,
      stats: m.stats, cheatFlags: m.cheatFlags,
      verifiedBy: m.verifiedBy, lastUpdate: m.lastUpdate,
    };
  }

  _displayTime(m) {
    if (m.status === MATCH_STATUS.UPDATING) return 'Updating scores...';
    if (m.status === MATCH_STATUS.FINISHED) return `FT ${m.maxDuration}+${Math.ceil(m.addedTime||0)}'`;
    if (m.status === MATCH_STATUS.ADDED_TIME) return `${m.maxDuration}+${Math.ceil(m.addedTime)}'`;
    return `${Math.min(Math.round(m.minute), m.maxDuration)}'`;
  }

  isThrottled(key, ms) { return this.throttles.has(key) && Date.now() - this.throttles.get(key) < ms; }
  setThrottle(key) { this.throttles.set(key, Date.now()); }
}

module.exports = GameEngine;
