const fs = require('fs');
const html = fs.readFileSync('mobile/index.html', 'utf8');
const liveStart = html.indexOf('<div class="pg on" id="pg-live">');
const tournStart = html.indexOf('<!-- TOURNAMENTS -->');
console.log(html.substring(liveStart, tournStart));
