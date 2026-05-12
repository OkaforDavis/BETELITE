const fs = require('fs');

let html = fs.readFileSync('mobile/index.html', 'utf8');

// Replace the old BACKEND_URL logic
const oldRegex = /const isProd = window\.location\.hostname !== 'localhost'[\s\S]*?const BACKEND_URL = isProd \? 'https:\/\/betelite-backend\.onrender\.com' : 'http:\/\/localhost:3000';/;

const newLogic = `const isLocal = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1' || window.location.hostname.startsWith('192.168.') || window.location.hostname.startsWith('10.') || window.location.protocol === 'file:';
const BACKEND_URL = isLocal ? (window.location.protocol === 'file:' ? 'http://localhost:3000' : window.location.origin.replace(/:\\d+$/, ':3000')) : 'https://betelite-backend.onrender.com';`;

if (html.match(oldRegex)) {
  html = html.replace(oldRegex, newLogic);
  fs.writeFileSync('mobile/index.html', html);
  console.log('Fixed BACKEND_URL logic for local network testing');
} else {
  console.log('Could not find the target BACKEND_URL logic to replace');
}
