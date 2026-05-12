const fs = require('fs');

let html = fs.readFileSync('mobile/index.html', 'utf8');

// Remove Gemini API Key Reset button
html = html.replace(
  /<span class="tag tg" onclick="localStorage\.removeItem\('GEMINI_KEY'\);[^>]+>.*Key<\/span>/,
  ''
);

// Replace runAIDetection
const oldFuncRegex = /async function runAIDetection\(\) \{[\s\S]*?btn\.textContent = 'RUN DETECTION';\s*\}\s*\}/;

const newFunc = `async function runAIDetection() {
    if (!aiBase64) {
      toast('Please upload an image first');
      return;
    }
    
    const btn = document.getElementById('btn-detect');
    const resDiv = document.getElementById('ai-result');
    const game = document.getElementById('ai-game-type').value;
    
    btn.disabled = true;
    btn.textContent = 'AI IS ANALYZING...';
    resDiv.style.display = 'block';
    resDiv.textContent = 'Processing via Local Backend AI Engine...';
    
    try {
      // POST to backend API which proxies to FastAPI EasyOCR
      const formData = new FormData();
      formData.append('game', game);
      formData.append('image_b64', aiBase64); // Send raw base64 string
      // In the future (Phase 2), we will also append: formData.append('target_gamertag', S.profile.gamertag);
      
      const res = await fetch(BACKEND_URL + '/api/detect/frame', {
        method: 'POST',
        body: formData
      });
      
      if (!res.ok) {
          const errData = await res.json().catch(() => ({}));
          throw new Error(errData.error || errData.detail || res.statusText);
      }
      
      const data = await res.json();
      
      resDiv.textContent = JSON.stringify(data, null, 2);
    } catch (err) {
      resDiv.textContent = 'Error: ' + err.message + '\\n\\n(Tip: Ensure the backend detection service is running on port 5000)';
    } finally {
      btn.disabled = false;
      btn.textContent = 'RUN DETECTION';
    }
  }`;

html = html.replace(oldFuncRegex, newFunc);

// Check if image upload allows all images
html = html.replace(
  /<input type="file" id="ai-upload" accept="image\/png, image\/jpeg"/g,
  '<input type="file" id="ai-upload" accept="image/*"'
);

fs.writeFileSync('mobile/index.html', html);
console.log('Successfully updated mobile/index.html to use local OCR');
