html_target = """            <label class="fl">SELECT GAME</label>
            <select class="fi" id="ai-game-type" style="margin-bottom: 12px;">
              <option value="football">FIFA / eFootball / DLS</option>
              <option value="fps">COD / PUBG / Free Fire</option>
            </select>"""

html_replacement = """            <label class="fl">SELECT GAME</label>
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
            </div>"""

js_target = """        const targetId = document.getElementById('ai-target-id').value.trim();
        const oppId = document.getElementById('ai-opponent-id').value.trim();"""

js_replacement = """        const tEl = document.getElementById('ai-target-id');
        const oEl = document.getElementById('ai-opponent-id');
        const targetId = tEl ? tEl.value.trim() : 'KLAUS';
        const oppId = oEl ? oEl.value.trim() : 'JOSEP GUARDIOLA';"""

with open("mobile/index.html", "r", encoding="utf-8") as f:
    content = f.read()

changed = False
if html_target in content:
    content = content.replace(html_target, html_replacement)
    print("HTML Replaced")
    changed = True
else:
    print("HTML not found")
    
if js_target in content:
    content = content.replace(js_target, js_replacement)
    print("JS Replaced")
    changed = True
else:
    print("JS not found")
    
if changed:
    with open("mobile/index.html", "w", encoding="utf-8") as f:
        f.write(content)
    print("File saved")
