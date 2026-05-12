const fs = require('fs');
let html = fs.readFileSync('mobile/index.html', 'utf8');

// Restore the deleted HTML
if (!html.includes('<div id="toast"></div>')) {
  html = html.replace(
    'const FIREBASE_CONFIG = {',
    `<!-- TOAST -->
<div id="toast"></div>

<script>
const isProd = window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1' && window.location.protocol !== 'file:';
const BACKEND_URL = isProd ? 'https://betelite-backend.onrender.com' : 'http://localhost:3000';

// ══════════════════════════════════════════════════════
//  FIREBASE CONFIG
//  Replace with your actual Firebase project config
// ══════════════════════════════════════════════════════
const FIREBASE_CONFIG = {`
  );
}

fs.writeFileSync('mobile/index.html', html);
console.log('Restored toast and script tags, fixed BACKEND_URL.');
