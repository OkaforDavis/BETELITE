import re

with open("mobile/index.html", "r", encoding="utf-8") as f:
    content = f.read()

# Replace HTML (Regex matches regardless of spaces)
html_pattern = re.compile(r'(<label class="fl">SELECT GAME</label>\s*<select class="fi" id="ai-game-type"[^>]*>\s*<option[^>]*>.*?</option>\s*<option[^>]*>.*?</option>\s*</select>)', re.DOTALL)

html_replacement = r'''\1
            
            <div style="display:flex; gap:8px; margin-bottom:12px;">
              <div style="flex:1;">
                <label class="fl">YOUR ID</label>
                <input class="fi" type="text" id="ai-target-id" placeholder="e.g. KLAUS" value="KLAUS">
              </div>
              <div style="flex:1;">
                <label class="fl">OPPONENT ID</label>
                <input class="fi" type="text" id="ai-opponent-id" placeholder="e.g. JOSEP" value="JOSEP GUARDIOLA">
              </div>
            </div>'''

if html_pattern.search(content):
    content = html_pattern.sub(html_replacement, content)
    print("HTML Replaced!")
else:
    print("HTML Pattern Not Found!")

# Replace JS logic
js_pattern = re.compile(r'const targetId = document\.getElementById\(\'ai-target-id\'\)\.value\.trim\(\);\s*const oppId = document\.getElementById\(\'ai-opponent-id\'\)\.value\.trim\(\);', re.DOTALL)

js_replacement = r'''const tEl = document.getElementById('ai-target-id');
        const oEl = document.getElementById('ai-opponent-id');
        const targetId = tEl ? tEl.value.trim() : 'KLAUS';
        const oppId = oEl ? oEl.value.trim() : 'JOSEP GUARDIOLA';'''

if js_pattern.search(content):
    content = js_pattern.sub(js_replacement, content)
    print("JS Replaced!")
else:
    print("JS Pattern Not Found!")

with open("mobile/index.html", "w", encoding="utf-8") as f:
    f.write(content)
