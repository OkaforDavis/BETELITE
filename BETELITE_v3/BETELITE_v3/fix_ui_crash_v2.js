const fs = require('fs');

let html = fs.readFileSync('mobile/index.html', 'utf8');

// 1. Fix the HTML
const htmlTarget = `<label class="fl">SELECT GAME</label>
            <select class="fi" id="ai-game-type" style="margin-bottom: 12px;">
              <option value="football">FIFA / eFootball / DLS</option>
              <option value="fps">COD / PUBG / Free Fire</option>
            </select>`;

const htmlReplacement = `<label class="fl">SELECT GAME</label>
            <select class="fi" id="ai-game-type" style="margin-bottom: 12px;">
              <option value="football">FIFA / eFootball / DLS</option>
              <option value="fps">COD / PUBG / Free Fire</option>
            </select>
            
            <div style="display:flex; gap:8px; margin-bottom:12px;">
              <div style="flex:1;">
                <label class="fl">YOUR ID</label>
                <input class="fi" type="text" id="ai-target-id" placeholder="e.g. KLAUS" value="KLAUS">
              </div>
              <div style="flex:1;">
                <label class="fl">OPPONENT ID</label>
                <input class="fi" type="text" id="ai-opponent-id" placeholder="e.g. JOSEP" value="JOSEP GUARDIOLA">
              </div>
            </div>`;

if (html.includes(htmlTarget)) {
  html = html.replace(htmlTarget, htmlReplacement);
  console.log("HTML replaced successfully.");
} else {
  console.log("HTML target NOT found.");
}

// 2. Fix the JS
const jsTarget = `const game = document.getElementById('ai-game-type').value;
        const targetId = document.getElementById('ai-target-id').value.trim();
        const oppId = document.getElementById('ai-opponent-id').value.trim();`;

const jsReplacement = `const game = document.getElementById('ai-game-type').value;
      const targetInput = document.getElementById('ai-target-id');
      const oppInput = document.getElementById('ai-opponent-id');
      
      const targetId = targetInput ? targetInput.value.trim() : '';
      const oppId = oppInput ? oppInput.value.trim() : '';`;

if (html.includes(jsTarget)) {
  html = html.replace(jsTarget, jsReplacement);
  console.log("JS replaced successfully.");
} else {
  console.log("JS target NOT found.");
}

fs.writeFileSync('mobile/index.html', html);
