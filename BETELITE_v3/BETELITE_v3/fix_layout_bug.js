const fs = require('fs');
let html = fs.readFileSync('mobile/index.html', 'utf8');

const startIdx = html.indexOf('    <!-- LIVE / HOME -->');
const endIdx = html.indexOf('    <!-- TOURNAMENTS -->');

const replacement = `    <!-- LIVE / HOME -->
    <div class="pg on" id="pg-live">
      <!-- ADS -->
      <div id="ads"><div class="ad-tr" id="ad-tr"></div></div>
      <!-- TICKER -->
      <div id="ticker"><div class="tk-tag" style="background:var(--red); color:#fff;">LIVE</div><div class="tk-sc" id="tk-sc"></div></div>
      <div class="sh">
        <div class="st"><div class="dl"></div>Live Matches</div>
        <span class="tag tg">🟢 System Online</span>
      </div>

      <!-- EMPTY STATE — no live games yet -->
      <div class="empty-state" style="padding:20px; min-height:auto; border:1px solid rgba(0,212,106,0.2); background:rgba(0,212,106,0.05); border-radius:12px; margin-bottom:12px;">
        <div style="font-size:24px; margin-bottom:8px;">🟢</div>
        <div class="es-title" style="font-size:16px;">Live Matchmaking Active</div>
        <div class="es-sub" style="font-size:12px;">Join tournaments or challenge players to start earning.</div>
      </div>

      <!-- Promo grid — upcoming games -->
      <div class="sh" style="margin-top:4px">
        <div class="st" style="font-size:13px">Active Leagues</div>
      </div>
      <div class="promo-grid">
        <div class="promo-card" style="border-top:2px solid #00D46A" onclick="navTo('tourneys')">
          <div class="promo-card-glow" style="background:linear-gradient(135deg,#00D46A,transparent)"></div>
          <div class="promo-ico">⚽</div>
          <div class="promo-nm">eFootball</div>
          <div class="promo-ds">5v5 tournaments · <span class="dyn-p" data-base="500000">₦500K</span> prize</div>
          <div class="promo-badge">LIVE</div>
        </div>
        <div class="promo-card" style="border-top:2px solid #FF3B3B" onclick="navTo('tourneys')">
          <div class="promo-card-glow" style="background:linear-gradient(135deg,#FF3B3B,transparent)"></div>
          <div class="promo-ico">🎯</div>
          <div class="promo-nm">COD Mobile</div>
          <div class="promo-ds">5v5 Pro League · <span class="dyn-p" data-base="1000000">₦1M</span> prize</div>
          <div class="promo-badge">LIVE</div>
        </div>
        <div class="promo-card" style="border-top:2px solid #FF7A00" onclick="navTo('tourneys')">
          <div class="promo-card-glow" style="background:linear-gradient(135deg,#FF7A00,transparent)"></div>
          <div class="promo-ico">🔥</div>
          <div class="promo-nm">Free Fire</div>
          <div class="promo-ds">Battle Royale · <span class="dyn-p" data-base="250000">₦250K</span> prize</div>
          <div class="promo-badge">LIVE</div>
        </div>
        <div class="promo-card" style="border-top:2px solid #00AFFF" onclick="navTo('tourneys')">
          <div class="promo-card-glow" style="background:linear-gradient(135deg,#00AFFF,transparent)"></div>
          <div class="promo-ico">🏟️</div>
          <div class="promo-nm">Dream League</div>
          <div class="promo-ds">Afrika Cup · <span class="dyn-p" data-base="150000">₦150K</span> prize</div>
          <div class="promo-badge">LIVE</div>
        </div>
      </div>

      <div class="coming-section">
        <div class="coming-label">ALSO COMING</div>
        <div class="promo-grid">
          <div class="promo-card" onclick="navTo('tourneys')">
            <div class="promo-ico">🏆</div>
            <div class="promo-nm">FIFA Mobile</div>
            <div class="promo-ds">Super Cup Series</div>
            <div class="promo-badge">HOT</div>
          </div>
          <div class="promo-card" onclick="navTo('tourneys')">
            <div class="promo-ico">🏀</div>
            <div class="promo-nm">NBA 2K</div>
            <div class="promo-ds">Basketball League</div>
            <div class="promo-badge">NEW</div>
          </div>
        </div>
      </div>
    </div>\n\n`;

html = html.substring(0, startIdx) + replacement + html.substring(endIdx);
fs.writeFileSync('mobile/index.html', html);
console.log('Fixed pg-live HTML structure!');
