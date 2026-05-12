const fs = require('fs');

const file = 'mobile/index.html';
let content = fs.readFileSync(file, 'utf8');

// 1. Title & Branding replacements
content = content.replace(/<title>BETELITE<\/title>/gi, '<title>CrestArena</title>');
content = content.replace(/BETELITE is launching soon/g, 'CrestArena is launching soon');
content = content.replace(/Upcoming on BETELITE/g, 'Upcoming on CrestArena');

// 2. Logo replacements (Auth screen)
content = content.replace(
  /<svg class="auth-logo-svg"[^>]*>[\s\S]*?<\/svg>/m,
  '<img src="logo.png" alt="CrestArena" class="auth-logo-svg" style="border-radius:12px;">'
);
content = content.replace(
  /<div class="auth-logo-wm">BET<em>ELITE<\/em><\/div>/m,
  '<div class="auth-logo-wm" style="letter-spacing:1px; font-size:28px;">Crest<em>Arena</em></div>'
);

// 3. Logo replacements (Header)
content = content.replace(
  /<svg class="logo-svg"[^>]*>[\s\S]*?<\/svg>/m,
  '<img src="logo.png" alt="CrestArena" class="logo-svg" style="border-radius:6px; background:#000; padding:2px;">'
);
content = content.replace(
  /<span class="logo-wm">BET<em>ELITE<\/em><\/span>/m,
  '<span class="logo-wm" style="letter-spacing:1px; font-size:20px;">Crest<em>Arena</em></span>'
);

// 4. Structural HTML Layout: Move #ads and #ticker inside #pg-live
// First, extract #ads and #ticker
const adsMatch = content.match(/<!-- ADS -->[\s\S]*?<div id="ads">[\s\S]*?<\/div>[\s]*<\/div>/);
const tickerMatch = content.match(/<!-- TICKER -->[\s\S]*?<div id="ticker">[\s\S]*?<\/div>[\s]*<\/div>/);

if (adsMatch && tickerMatch) {
  content = content.replace(adsMatch[0], '');
  content = content.replace(tickerMatch[0], '');
  
  // Insert right after <div class="pg on" id="pg-live">
  content = content.replace(
    /(<div class="pg on" id="pg-live">)/,
    `$1\n  ${adsMatch[0]}\n  ${tickerMatch[0]}`
  );
}

// 5. CSS Upgrades
// Tablet sizing
content = content.replace(
  /#hd \{[\s\S]*?\}/,
  `#hd {
  flex-shrink: 0; z-index: 20;
  background: rgba(13, 15, 28, 0.7);
  backdrop-filter: blur(15px);
  -webkit-backdrop-filter: blur(15px);
  border-bottom: 1px solid var(--border);
  padding-top: env(safe-area-inset-top, 0px);
  max-width: 430px; margin: 0 auto; width: 100%;
}`
);

content = content.replace(
  /#nav \{[\s\S]*?min-height: calc\(var\(--nav-h\) \+ max\(env\(safe-area-inset-bottom, 0px\), 6px\)\);[\s\S]*?\}/,
  `#nav {
  flex-shrink: 0; display: flex;
  background: rgba(13, 15, 28, 0.7);
  backdrop-filter: blur(15px);
  -webkit-backdrop-filter: blur(15px);
  border-top: 1px solid var(--border);
  z-index: 20;
  padding-bottom: max(env(safe-area-inset-bottom, 0px), 6px);
  min-height: calc(var(--nav-h) + max(env(safe-area-inset-bottom, 0px), 6px));
  position: relative; width: 100%; max-width: 430px; margin: 0 auto;
}`
);

// Wallet pill upgrade
content = content.replace(
  /\.w-pill \{[\s\S]*?\}/,
  `.w-pill {
  display: flex; align-items: center; gap: 4px;
  background: linear-gradient(135deg, rgba(245,197,24,0.15), rgba(212,167,0,0.05));
  border: 1px solid rgba(245,197,24,0.3); border-radius: 20px; padding: 5px 10px;
  font-family: var(--fd); font-weight: 700; font-size: 13px;
  color: var(--gold); cursor: pointer; white-space: nowrap;
  box-shadow: 0 2px 10px rgba(245,197,24,0.1);
}`
);

// Promo cards animations
content = content.replace(
  /\.promo-card:hover \{ border-color: var\(--gold2\); transform: translateY\(-1px\); \}/,
  `.promo-card:hover { border-color: var(--gold2); transform: scale(0.98); }\n.promo-card:active { transform: scale(0.95); }`
);

// Add missing nav IDs (just replacing class="nb" in the nav block if needed)
content = content.replace(/class="nb" onclick="navTo\('bets'\)"/g, 'class="nb" id="nb-bets" onclick="navTo(\'bets\')"');
content = content.replace(/class="nb" onclick="navTo\('watch'\)"/g, 'class="nb" id="nb-watch" onclick="navTo(\'watch\')"');

fs.writeFileSync(file, content);
console.log('Refactoring complete.');
