const fs = require('fs');

let html = fs.readFileSync('mobile/index.html', 'utf8');

// The exact target HTML
const targetHtml = `<label class="fl">SELECT GAME</label>
            <select class="fi" id="ai-game-type" style="margin-bottom: 12px;">
              <option value="football">FIFA / eFootball / DLS</option>
              <option value="fps">COD / PUBG / Free Fire</option>
            </select>`;

const newHtml = `<label class="fl">SELECT GAME</label>
            <select class="fi" id="ai-game-type" style="margin-bottom: 12px;">
              <option value="football">FIFA / eFootball / DLS</option>
              <option value="fps">COD / PUBG / Free Fire</option>
            </select>
            
            <div style="display:flex; gap:8px; margin-bottom:12px;">
              <div style="flex:1;">
                <label class="fl">YOUR ID</label>
                <input class="fi" type="text" id="ai-target-id" placeholder="e.g. KLAUS">
              </div>
              <div style="flex:1;">
                <label class="fl">OPPONENT ID</label>
                <input class="fi" type="text" id="ai-opponent-id" placeholder="e.g. JOSEP">
              </div>
            </div>`;

if (html.includes(targetHtml)) {
  html = html.replace(targetHtml, newHtml);
  console.log("HTML inputs added.");
} else {
  console.log("Could not find HTML inputs target.");
}

// Ensure the JS logic exists and works safely
const targetJs = `const btn = document.getElementById('btn-detect');
      const resDiv = document.getElementById('ai-result');
      const game = document.getElementById('ai-game-type').value;`;

const newJs = `const btn = document.getElementById('btn-detect');
      const resDiv = document.getElementById('ai-result');
      const game = document.getElementById('ai-game-type').value;
      const targetInput = document.getElementById('ai-target-id');
      const oppInput = document.getElementById('ai-opponent-id');
      
      const targetId = targetInput ? targetInput.value.trim() : '';
      const oppId = oppInput ? oppInput.value.trim() : '';`;

if (html.includes(targetJs)) {
  html = html.replace(targetJs, newJs);
  console.log("JS logic updated safely.");
} else {
  console.log("Could not find JS logic target.");
}

// Add the form appends
const targetForm = `const formData = new FormData();
        formData.append('game', game);
        formData.append('image_b64', aiBase64); // Send raw base64 string`;

const newForm = `const formData = new FormData();
        formData.append('game', game);
        formData.append('image_b64', aiBase64);
        formData.append('target_gamertag', targetId);
        formData.append('opponent_gamertag', oppId);`;

if (html.includes(targetForm)) {
  html = html.replace(targetForm, newForm);
  console.log("FormData appends updated.");
} else {
  console.log("Could not find FormData target.");
}

fs.writeFileSync('mobile/index.html', html);
console.log("Done.");
