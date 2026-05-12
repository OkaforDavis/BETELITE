const fs = require('fs');
let content = fs.readFileSync('mobile/index.html', 'utf8');

content = content.replace(
  /onclick="toast\('\$\{a\.n\} — coming to BETELITE soon!'\)"/g,
  `onclick="navTo('tourneys')"`
);

// We must also make sure setTimeout is called in buildAds and buildTicker
content = content.replace(
  /function buildAds\(\) \{\s*const all = \[\.\.\.ADS, \.\.\.ADS\];\s*document\.getElementById\('ad-tr'\)\.innerHTML = all\.map\(a =>\s*`<div class="ad-c" onclick="navTo\('tourneys'\)">\s*<div class="ad-gl" style="background:linear-gradient\(135deg,\$\{a\.c\},transparent\)"><\/div>\s*<div class="ad-ico">\$\{a\.i\}<\/div>\s*<div class="ad-tx"><div class="ad-nm">\$\{a\.n\}<\/div><div class="ad-ds">\$\{a\.d\}<\/div><\/div>\s*<div class="ad-bl">\$\{a\.b\}<\/div>\s*<\/div>`\)\.join\(''\);\s*\}/,
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
}`
);

content = content.replace(
  /function buildTicker\(\) \{\s*const all = \[\.\.\.TICKS, \.\.\.TICKS\];\s*document\.getElementById\('tk-sc'\)\.innerHTML = all\.map\(t =>\s*`<div class="tk-i">● \$\{t\}<\/div>`\)\.join\(''\);\s*\}/,
  `function buildTicker() {
  const all = [...TICKS, ...TICKS];
  document.getElementById('tk-sc').innerHTML = all.map(t =>
    \`<div class="tk-i">● \$\{t\}</div>\`).join('');
  setTimeout(updateDynamicPrices, 50);
}`
);

fs.writeFileSync('mobile/index.html', content);
