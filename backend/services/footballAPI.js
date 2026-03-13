const axios = require('axios');

// API-Football (api-football.com) — free tier: 100 calls/day
const BASE = 'https://v3.football.api-sports.io';
const KEY  = process.env.API_FOOTBALL_KEY || '';

// Cache to avoid hitting rate limits
let cache = { data: null, ts: 0 };
const CACHE_TTL = 60000; // 1 minute

async function getLiveFootball() {
  if (!KEY) return getMockFootball();
  if (Date.now() - cache.ts < CACHE_TTL && cache.data) return cache.data;

  try {
    const res = await axios.get(`${BASE}/fixtures`, {
      headers: { 'x-apisports-key': KEY },
      params: { live: 'all' },
      timeout: 8000,
    });
    const fixtures = res.data?.response || [];
    const mapped = fixtures.slice(0, 10).map(f => ({
      id:         `ff_${f.fixture.id}`,
      source:     'api_football',
      gameType:   'football_real',
      icon:       '⚽',
      label:      f.league.name,
      home:       f.teams.home.name,
      away:       f.teams.away.name,
      scoreHome:  f.goals.home ?? 0,
      scoreAway:  f.goals.away ?? 0,
      minute:     f.fixture.status.elapsed ?? 0,
      displayTime:`${f.fixture.status.elapsed ?? 0}'`,
      status:     f.fixture.status.short === 'FT' ? 'finished' : 'live',
      venue:      f.fixture.venue?.name,
      league:     f.league.name,
    }));
    cache = { data: mapped, ts: Date.now() };
    return mapped;
  } catch (e) {
    console.error('[FOOTBALL_API] Error:', e.message);
    return getMockFootball();
  }
}

async function syncFootballScores() {
  // Returns array of score updates to broadcast
  const matches = await getLiveFootball();
  return matches.map(m => ({ ...m, matchId: m.id }));
}

function getMockFootball() {
  return [
    { id:'mock_1', source:'mock', gameType:'football_real', icon:'⚽', label:'Premier League',
      home:'Man City', away:'Arsenal', scoreHome:2, scoreAway:1, minute:67,
      displayTime:"67'", status:'live' },
    { id:'mock_2', source:'mock', gameType:'football_real', icon:'⚽', label:'La Liga',
      home:'Barcelona', away:'Real Madrid', scoreHome:1, scoreAway:1, minute:55,
      displayTime:"55'", status:'live' },
  ];
}

module.exports = { getLiveFootball, syncFootballScores };
