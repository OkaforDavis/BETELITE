const fs = require('fs');
let content = fs.readFileSync('mobile/index.html', 'utf8');

// Fix the marquee tag from COMING to LIVE
content = content.replace(
  /<div id="ticker"><div class="tk-tag">COMING<\/div>/g,
  '<div id="ticker"><div class="tk-tag" style="background:var(--red); color:#fff;">LIVE</div>'
);

// Fix the "No Live Matches Yet" empty state text which was hidden under an already replaced empty state
content = content.replace(
  /<div class="es-title">No Live Matches Yet<\/div>\s*<div class="es-sub">CrestArena is launching soon\.<br>Live matches, scores and betting<br>will appear here once we go live\.<\/div>\s*<div class="es-badge">🚀 Be the first to play<\/div>/g,
  ''
);

fs.writeFileSync('mobile/index.html', content);
