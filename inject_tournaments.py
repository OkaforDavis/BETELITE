import re

INJECT = """
// ===== TOURNAMENTS - Live Data =====
let _activeTournamentId = null;
let _fixtureImages = {};

async function loadTournaments() {
  var listEl = document.getElementById('tournament-list');
  if (!listEl) return;
  listEl.innerHTML = '<div class="empty-state"><div class="es-icon">⏳</div><div class="es-title">Loading...</div></div>';
  try {
    var res = await fetch('/api/tournaments');
    var data = await res.json();
    renderTournamentList(data.tournaments);
    if (_activeTournamentId) loadMyFixtures(_activeTournamentId);
  } catch(e) {
    listEl.innerHTML = '<div class="empty-state"><div class="es-icon">❌</div><div class="es-title">Backend offline</div><div class="es-sub">Start the Node.js server</div></div>';
  }
}

function renderTournamentList(list) {
  var listEl = document.getElementById('tournament-list');
  if (!list || !list.length) { listEl.innerHTML = '<div class="empty-state"><div class="es-title">No tournaments</div></div>'; return; }
  var html = '';
  list.forEach(function(t) {
    var isJoined = S.user && t.players.find(function(p){ return p.userId === S.user.uid; });
    var sc = t.status === 'open' ? 'var(--green)' : t.status === 'active' ? 'var(--gold)' : 'var(--text3)';
    var ep = Math.round(t.entryFeeNGN * (ccy === 'GHS' ? 0.037 : 1));
    var pp = Math.round(t.prizePool * (ccy === 'GHS' ? 0.037 : 1));
    var p1 = t.prizes[0] ? getSym() + Math.round(t.prizes[0].amount * (ccy === 'GHS' ? 0.037 : 1)).toLocaleString() : '--';
    var joinBtn = '';
    if (isJoined) {
      joinBtn = '<button class="trn-notify-btn" style="background:var(--gold);color:#000;" onclick="viewMyMatches(' + "'" + t.id + "'" + ')">📋 VIEW MY MATCHES</button>';
    } else if (t.status === 'open') {
      joinBtn = '<button class="trn-notify-btn" onclick="joinTournamentById(' + "'" + t.id + "'" + ',' + "'" + t.name.replace(/'/g,'') + "'" + ',this)">⚔️ JOIN TOURNAMENT</button>';
    } else {
      joinBtn = '<button class="trn-notify-btn" disabled style="opacity:.5;">🔒 ' + t.status.toUpperCase() + '</button>';
    }
    html += '<div class="trn-preview" style="border-top:2px solid ' + sc + ';">' +
      '<div class="trn-soon-badge" style="background:' + sc + ';color:#000;">' + t.status.toUpperCase() + ' · ' + t.playerCount + '/' + t.maxPlayers + '</div>' +
      '<div class="trn-icon">' + t.icon + '</div>' +
      '<div class="trn-name">' + t.name + '</div>' +
      '<div class="trn-meta">' + t.mode + ' · Entry: ' + getSym() + ep.toLocaleString() + ' · Prize: ' + getSym() + pp.toLocaleString() + '</div>' +
      '<div class="trn-prize"><div><div class="trn-prize-val">' + p1 + '</div><div class="trn-prize-lbl">1st place (30%)</div></div><span class="tag tg">Top 5 win</span></div>' +
      joinBtn + '</div>';
  });
  listEl.innerHTML = html;
}

async function joinTournamentById(id, name, btn) {
  if (!S.user) { toast('Please login first'); return; }
  btn.disabled = true; btn.textContent = 'Joining...';
  try {
    var res = await fetch('/api/tournaments/' + id + '/join', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ userId: S.user.uid, username: S.profile.displayName })
    });
    var data = await res.json();
    if (!data.ok) { toast('Failed: ' + (data.error || 'Unknown')); btn.disabled = false; btn.textContent = '⚔️ JOIN TOURNAMENT'; return; }
    toast('✅ Joined ' + name + '!');
    _activeTournamentId = id;
    loadTournaments();
  } catch(e) { toast('Error: ' + e.message); btn.disabled = false; btn.textContent = '⚔️ JOIN TOURNAMENT'; }
}

async function viewMyMatches(id) {
  _activeTournamentId = id;
  document.getElementById('my-fixtures-section').style.display = 'block';
  await loadMyFixtures(id);
  document.getElementById('my-fixtures-section').scrollIntoView({ behavior: 'smooth' });
}

async function loadMyFixtures(id) {
  if (!S.user) return;
  try {
    var res = await fetch('/api/tournaments/' + id + '/fixtures?userId=' + encodeURIComponent(S.user.uid));
    var data = await res.json();
    renderMyFixtures(id, data.fixtures);
  } catch(e) { console.error('[Fixtures]', e); }
}

function renderMyFixtures(tournamentId, fixList) {
  var el = document.getElementById('my-fixtures-list');
  if (!fixList || !fixList.length) {
    el.innerHTML = '<div style="text-align:center;padding:12px;font-size:12px;color:var(--text3)">Waiting for more players to join and generate fixtures...</div>';
    return;
  }
  var html = '';
  fixList.forEach(function(f) {
    var done = f.status === 'completed';
    var isHome = S.user && f.homeId === S.user.uid;
    var my = isHome ? f.homeName : f.awayName;
    var opp = isHome ? f.awayName : f.homeName;
    var body = '';
    if (done) {
      body = '<div style="font-size:11px;color:var(--text3);">' + (f.aiVerified ? '🤖 AI Verified' : '✍️ Manual') + ' · ' + f.scoreHome + '-' + f.scoreAway + '</div>';
    } else {
      body = '<div style="font-size:11px;color:var(--text2);margin-bottom:8px;">Play the match then upload scoreboard screenshot</div>' +
        '<div class="pfp-upload" style="width:100%;height:80px;margin-bottom:8px;border-radius:8px;" onclick="document.getElementById(' + "'fix-img-" + f.id + "'" + ').click()">' +
        '<span id="fix-icon-' + f.id + '" style="font-size:18px;">📸 Upload Screenshot</span>' +
        '<img id="fix-prev-' + f.id + '" src="" style="display:none;width:100%;height:100%;object-fit:contain;border-radius:6px;"></div>' +
        '<input type="file" id="fix-img-' + f.id + '" accept="image/*" style="display:none" onchange="previewFixture(this,' + "'" + f.id + "'" + ')">' +
        '<button class="cta" id="fix-btn-' + f.id + '" onclick="submitFixture(' + "'" + tournamentId + "','" + f.id + "'" + ')" style="background:var(--gold);color:#000;padding:10px;">✅ MARK PLAYED &amp; SUBMIT</button>';
    }
    html += '<div class="menu-card" style="margin-bottom:10px;border:1px solid ' + (done ? 'var(--green)' : 'var(--gold)') + ';">' +
      '<div style="padding:12px;">' +
      '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px;">' +
      '<span style="font-weight:900;font-size:13px;color:var(--text);">' + my + ' <span style="color:var(--text3)">vs</span> ' + opp + '</span>' +
      '<span class="tag ' + (done ? 'tg' : 'to') + '">' + (done ? '✅ ' + f.scoreHome + '-' + f.scoreAway : '⏳ PENDING') + '</span></div>' +
      body + '</div></div>';
  });
  el.innerHTML = html;
}

function previewFixture(input, fid) {
  if (!input.files || !input.files[0]) return;
  var r = new FileReader();
  r.onload = function(e) {
    _fixtureImages[fid] = e.target.result;
    document.getElementById('fix-prev-' + fid).src = e.target.result;
    document.getElementById('fix-prev-' + fid).style.display = 'block';
    document.getElementById('fix-icon-' + fid).style.display = 'none';
  };
  r.readAsDataURL(input.files[0]);
}

async function submitFixture(tournamentId, fixtureId) {
  if (!S.user) { toast('Please login'); return; }
  var btn = document.getElementById('fix-btn-' + fixtureId);
  btn.disabled = true; btn.textContent = '🤖 AI Detecting...';
  try {
    var res = await fetch('/api/tournaments/' + tournamentId + '/fixtures/' + fixtureId + '/submit', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ userId: S.user.uid, image_b64: _fixtureImages[fixtureId] || null })
    });
    var data = await res.json();
    if (!data.ok) { toast('Failed: ' + (data.error || 'Error')); btn.disabled = false; btn.textContent = '✅ MARK PLAYED & SUBMIT'; return; }
    toast('Result: ' + data.finalScore + ' ' + (data.aiVerified ? '🤖 AI Verified' : '✍️ Manual'));
    loadMyFixtures(tournamentId);
    loadTournaments();
  } catch(e) { toast('Error: ' + e.message); btn.disabled = false; btn.textContent = '✅ MARK PLAYED & SUBMIT'; }
}

// ===== END TOURNAMENTS =====

"""

HTML_PATH = r'c:\\Users\\PC\\BETELITE\\BETELITE_v3\\BETELITE_v3\\mobile\\index.html'

with open(HTML_PATH, 'r', encoding='utf-8') as f:
    content = f.read()

# Find the TOAST section using a pattern robust to special unicode chars
pattern = re.compile(r'(// [^\n]+\n//  TOAST\n// [^\n]+\nlet _tt;)')
m = pattern.search(content)
if m:
    insert_pos = m.start()
    content = content[:insert_pos] + INJECT + content[insert_pos:]
    with open(HTML_PATH, 'w', encoding='utf-8') as f:
        f.write(content)
    print('SUCCESS: Tournament JS injected at line', content[:insert_pos].count('\\n'))
else:
    print('MARKER NOT FOUND')
