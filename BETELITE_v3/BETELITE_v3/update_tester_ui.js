const fs = require('fs');

let html = fs.readFileSync('mobile/index.html', 'utf8');

// 1. Add input fields to the UI
const selectGameRegex = /<div class="setting-label">SELECT GAME<\/div>[\s\S]*?<\/select>/;

const newInputs = `<div class="setting-label">SELECT GAME</div>
          <select id="ai-game-type" style="width: 100%; padding: 12px; background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; color: var(--text); font-family: 'Space Grotesk', sans-serif; outline: none; margin-bottom: 12px;">
            <option value="football">FIFA / eFootball / DLS</option>
            <option value="fps">COD Mobile / PUBG / Free Fire</option>
          </select>

          <div style="display:flex; gap:8px; margin-bottom:12px;">
            <div style="flex:1;">
              <div class="setting-label">YOUR ID (e.g. KLAUS)</div>
              <input type="text" id="ai-target-id" placeholder="Your Gamertag" style="width: 100%; padding: 12px; background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; color: var(--text); font-family: 'Space Grotesk', sans-serif; outline: none;">
            </div>
            <div style="flex:1;">
              <div class="setting-label">OPPONENT ID</div>
              <input type="text" id="ai-opponent-id" placeholder="Opponent Gamertag" style="width: 100%; padding: 12px; background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; color: var(--text); font-family: 'Space Grotesk', sans-serif; outline: none;">
            </div>
          </div>`;

html = html.replace(selectGameRegex, newInputs);

// 2. Update runAIDetection logic
const runAIDetectionRegex = /const game = document.getElementById\('ai-game-type'\).value;[\s\S]*?\/\/ In the future \(Phase 2\), we will also append: formData.append\('target_gamertag', S.profile.gamertag\);/;

const newLogic = `const game = document.getElementById('ai-game-type').value;
      const targetId = document.getElementById('ai-target-id').value.trim();
      const oppId = document.getElementById('ai-opponent-id').value.trim();
      
      btn.disabled = true;
      btn.textContent = 'AI IS ANALYZING...';
      resDiv.style.display = 'block';
      resDiv.textContent = 'Processing via Spatial Detection Engine...';
      
      try {
        const formData = new FormData();
        formData.append('game', game);
        formData.append('image_b64', aiBase64);
        formData.append('target_gamertag', targetId);
        formData.append('opponent_gamertag', oppId);`;

html = html.replace(runAIDetectionRegex, newLogic);

fs.writeFileSync('mobile/index.html', html);
console.log('Updated AI Tester UI for DLS 1v1 instances');
