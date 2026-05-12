const fs = require('fs');
const file = 'mobile/index.html';
let content = fs.readFileSync(file, 'utf8');

// 1. Empty State
content = content.replace(
  /<div class="empty-state">[\s\S]*?<\/div>/m,
  `<div class="empty-state" style="padding:20px; min-height:auto; border:1px solid rgba(0,212,106,0.2); background:rgba(0,212,106,0.05); border-radius:12px; margin-bottom:12px;">
    <div style="font-size:24px; margin-bottom:8px;">🟢</div>
    <div class="es-title" style="font-size:16px;">Live Matchmaking Active</div>
    <div class="es-sub" style="font-size:12px;">Join tournaments or challenge players to start earning.</div>
  </div>`
);

// 2. Promo Grid Titles & Toasts
content = content.replace(/Upcoming on CrestArena/g, 'Active Leagues');
content = content.replace(/SOON/g, 'LIVE');
content = content.replace(/Q2 2025/g, 'HOT');
content = content.replace(/BETA/g, 'NEW');
content = content.replace(/onclick="toast\('eFootball tournaments launching soon!'\)"/g, `onclick="navTo('tourneys')"`);
content = content.replace(/onclick="toast\('COD Mobile tournaments launching soon!'\)"/g, `onclick="navTo('tourneys')"`);
content = content.replace(/onclick="toast\('Free Fire tournaments launching soon!'\)"/g, `onclick="navTo('tourneys')"`);
content = content.replace(/onclick="toast\('Dream League tournaments launching soon!'\)"/g, `onclick="navTo('tourneys')"`);
content = content.replace(/onclick="toast\('FIFA Mobile — coming soon!'\)"/g, `onclick="navTo('tourneys')"`);
content = content.replace(/onclick="toast\('NBA 2K Mobile — coming soon!'\)"/g, `onclick="navTo('tourneys')"`);

// 3. Ticker Data & Language
content = content.replace(
  /const TICKS = \[[\s\S]*?\];/m,
  `const TICKS = [
  '🚀 CrestArena is <span class="tk-h">LIVE</span> — matchmaking is active',
  '🏆 eFootball 5v5 — Entry from <span class="tk-w dyn-p" data-base="1500">₦1,500</span> · Prize pool up to <span class="dyn-p" data-base="22500">₦22,500</span>',
  '🎯 COD Mobile Pro League — Prize pool up to <span class="tk-w dyn-p" data-base="750000">₦750,000</span>',
  '🔥 Free Fire Battle Royale — <span class="tk-h">15 players</span> · Winner takes 30%',
  '🌍 Global players welcome — Auto-converted payouts',
  '💰 Max bet per match: <span class="tk-h dyn-p" data-base="1000000">₦1,000,000</span>',
  '🛡️ Anti-cheat AI monitors every match in real-time',
  '📺 Live streaming with AI score detection active',
];`
);

// 4. Ads Data
content = content.replace(
  /const ADS = \[[\s\S]*?\];/m,
  `const ADS = [
  { i:'🎯', n:'COD Mobile',     d:'5v5 Pro · Win <span class="dyn-p" data-base="1000000">₦1M</span>', b:'LIVE', c:'#FF3B3B' },
  { i:'🔥', n:'Free Fire',      d:'Battle Royale · <span class="dyn-p" data-base="250000">₦250K</span>', b:'HOT', c:'#FF7A00' },
  { i:'⚽', n:'eFootball',      d:'5v5 Championships', b:'LIVE', c:'#00D46A' },
  { i:'🏟️', n:'Dream League',   d:'Afrika Cup · <span class="dyn-p" data-base="150000">₦150K</span>', b:'HOT', c:'#00AFFF' },
  { i:'🏆', n:'FIFA Mobile',    d:'Ultimate Cup Series', b:'NEW', c:'#F5C518' },
  { i:'🏀', n:'NBA 2K Mobile',  d:'Basketball Leagues', b:'LIVE', c:'#FF6B00' },
  { i:'⚔️', n:'Mobile Legends', d:'MOBA 5v5 League', b:'HOT', c:'#9B59B6' },
  { i:'🎲', n:'Chess Blitz',    d:'Mind Sports <span class="dyn-p" data-base="500000">₦500K</span>', b:'NEW', c:'#00D46A' },
];`
);

// Update ADS template string to avoid literal SOON toast
content = content.replace(
  /onclick="toast\('\$\{a\.n\} — coming to BETELITE soon!'\)"/g,
  `onclick="navTo('tourneys')"`
);
content = content.replace(
  /onclick="toast\('\$\{a\.n\} — coming to CrestArena soon!'\)"/g,
  `onclick="navTo('tourneys')"`
);

// Add updateDynamicPrices() call at the end of buildTicker and buildAds
content = content.replace(
  /function buildAds\(\) \{[\s\S]*?\}\n/m,
  `function buildAds() {
  const all = [...ADS, ...ADS];
  document.getElementById('ad-tr').innerHTML = all.map(a =>
    \`<div class="ad-c" onclick="navTo('tourneys')">
      <div class="ad-gl" style="background:linear-gradient(135deg,\$\{a.c\},transparent)"></div>
      <div class="ad-ico">\$\{a.i\}</div>
      <div class="ad-tx"><div class="ad-nm">\$\{a.n\}</div><div class="ad-ds">\$\{a.d\}</div></div>
      <div class="ad-bl">\$\{a.b\}</div>
    </div>\`).join('');
  setTimeout(updateDynamicPrices, 50);
}\n`
);

content = content.replace(
  /function buildTicker\(\) \{[\s\S]*?\}\n/m,
  `function buildTicker() {
  const all = [...TICKS, ...TICKS];
  document.getElementById('tk-sc').innerHTML = all.map(t =>
    \`<div class="tk-i">● \$\{t\}</div>\`).join('');
  setTimeout(updateDynamicPrices, 50);
}\n`
);

// "Launching soon" header text
content = content.replace(
  /<span class="tag to">🔜 Launching Soon<\/span>/g,
  `<span class="tag tg">🟢 System Online</span>`
);
content = content.replace(
  /<div id="conn-bar" class="demo">⚡ Platform launching soon — stay tuned!<\/div>/g,
  `<div id="conn-bar" class="connected">⚡ Real-time Matchmaking Connected</div>`
);


fs.writeFileSync(file, content);
console.log('Site made LIVE.');
