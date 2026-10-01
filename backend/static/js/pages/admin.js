import { get, post, del, upload } from '../api.js';
import { store, gameName } from '../store.js';
import { html, ic, money, timeAgo, empty, skeleton, sheet, sheetHead, busy, toast, confirmSheet, statusBadge, scoreLine } from '../ui.js';
import { gameChips } from '../components.js';

const TABS = [['overview', 'Overview'], ['review', 'Review'], ['withdrawals', 'Withdrawals'], ['tournaments', 'Tournaments'], ['users', 'Users'], ['ocr', 'OCR test']];

export default async function admin(view, { query }) {
  if (!store.get().profile?.isAdmin) {
    view.innerHTML = String(html`<div class="page">${empty('lock', 'Admins only', 'You do not have access to this page.')}</div>`);
    return;
  }
  let tab = query.tab || 'overview';
  view.innerHTML = String(html`<div class="page">
    <div class="h1">Admin</div>
    <div class="chips" style="margin-top:14px" role="tablist">${TABS.map(([k, l]) => html`<button class="chip" data-tab="${k}" role="tab">${l}</button>`)}</div>
    <div id="body" style="margin-top:16px"></div></div>`);
  const body = view.querySelector('#body');
  view.querySelectorAll('[data-tab]').forEach((b) => (b.onclick = () => { tab = b.dataset.tab; draw(); }));

  async function draw() {
    view.querySelectorAll('[data-tab]').forEach((b) => b.classList.toggle('active', b.dataset.tab === tab));
    body.innerHTML = String(skeleton(80, 3));
    try { await ({ overview, review, withdrawals, tournaments, users, ocr })[tab](body, draw); }
    catch (e) { body.innerHTML = String(empty('alert', 'Could not load', e.message)); }
  }
  await draw();
}

async function overview(body) {
  const { stats: s } = await get('/admin/stats');
  body.innerHTML = String(html`<div class="grid-2">
    <div class="stat"><div class="v num">${s.users}</div><div class="k">Players</div></div>
    <div class="stat"><div class="v num">${s.activeMatches}</div><div class="k">Active matches</div></div>
    <div class="stat"><div class="v num ${s.needsReview ? 'danger-text' : ''}">${s.needsReview}</div><div class="k">Need review</div></div>
    <div class="stat"><div class="v num ${s.pendingWithdrawals ? 'danger-text' : ''}">${s.pendingWithdrawals}</div><div class="k">Withdrawals pending</div></div>
    <div class="stat"><div class="v num">${money(s.walletTotal)}</div><div class="k">Wallet balances</div></div>
    <div class="stat"><div class="v num">${money(s.inEscrow)}</div><div class="k">In escrow</div></div>
    <div class="stat"><div class="v num brand-text">${money(s.feesToday)}</div><div class="k">1v1 fees, 24h</div></div>
  </div>`);
}

async function review(body, redraw) {
  const { matches } = await get('/admin/matches/review');
  if (!matches.length) { body.innerHTML = String(html`<div class="card flat">${empty('shield', 'Nothing to review', 'Disputed and overdue matches appear here.')}</div>`); return; }
  body.innerHTML = String(html`<div class="stack">${matches.map((m) => html`<div class="card">
    <div class="row between"><span class="eyebrow">${m.kind === 'p2p' ? '1v1 · ' + money(m.stake, m.currency) : m.tournamentName + ' · R' + m.round} · ${gameName(m.game)}</span>${statusBadge(m.status)}</div>
    <div class="versus" style="margin:14px 0">
      <div class="side"><div class="name">${m.homeName}</div><div class="tag">${m.homeTag}</div></div>
      <div class="mid">${scoreLine(m)}</div>
      <div class="side"><div class="name">${m.awayName}</div><div class="tag">${m.awayTag}</div></div></div>
    ${m.disputeReason ? html`<div class="notice warn">${ic('flag')}<div class="small"><b>${m.status === 'disputed' ? 'Dispute' : 'Reason'}:</b> ${m.disputeReason}</div></div>` : ''}
    <div class="small faint" style="margin-top:8px">Created ${timeAgo(m.createdAt)} · ID ${m.id}</div>
    <div class="grid-3" style="margin-top:12px">
      <button class="btn secondary sm" data-ocr="${m.id}">${ic('eye')} AI read</button>
      <button class="btn danger sm" data-void="${m.id}">${ic('xCircle')} Void</button>
      <button class="btn primary sm" data-resolve="${m.id}">${ic('check')} Decide</button></div>
  </div>`)}</div>`);

  body.querySelectorAll('[data-ocr]').forEach((b) => (b.onclick = () => busy(b, async () => {
    const { ocr } = await get(`/admin/matches/${b.dataset.ocr}/ocr`);
    sheet(String(html`${sheetHead('What the AI read')}<pre style="white-space:pre-wrap;font-size:13px;color:var(--text-2)">${JSON.stringify(ocr, null, 2) || 'No screenshot submitted'}</pre>`));
  })));
  body.querySelectorAll('[data-void]').forEach((b) => (b.onclick = async () => {
    const m = matches.find((x) => x.id === b.dataset.void);
    const ok = await confirmSheet({ title: 'Void match?', message: m.kind === 'p2p' ? 'Both stakes are refunded and the match is cancelled.' : 'The result is cleared so the players replay the match.', confirm: 'Void match', danger: true });
    if (ok) busy(b, async () => { await post(`/admin/matches/${m.id}/void`, { reason: 'Voided by admin.' }); toast('Match voided'); redraw(); });
  }));
  body.querySelectorAll('[data-resolve]').forEach((b) => (b.onclick = () => {
    const m = matches.find((x) => x.id === b.dataset.resolve);
    sheet(String(html`${sheetHead('Decide final score', m.homeName + ' vs ' + m.awayName)}
      <form id="rs"><div class="grid-2">
        <label class="field"><span class="label">${m.homeName} (home)</span><input class="input num" name="h" inputmode="numeric" required value="${m.scoreHome ?? ''}"></label>
        <label class="field" style="margin-top:0"><span class="label">${m.awayName} (away)</span><input class="input num" name="a" inputmode="numeric" required value="${m.scoreAway ?? ''}"></label></div>
        <div class="grid-2" style="margin-top:12px">
        <label class="field"><span class="label">Pens home (if any)</span><input class="input num" name="ph" inputmode="numeric" value="${m.pensHome ?? ''}"></label>
        <label class="field" style="margin-top:0"><span class="label">Pens away</span><input class="input num" name="pa" inputmode="numeric" value="${m.pensAway ?? ''}"></label></div>
        <div class="notice warn" style="margin-top:14px">${ic('alert')}<div class="small">This is final and pays out immediately.</div></div>
        <div class="sheet-actions"><button class="btn primary block" type="submit">Confirm & settle</button></div></form>`), {
      onMount(root, close) {
        root.querySelector('#rs').onsubmit = (e) => {
          e.preventDefault();
          const f = new FormData(e.target);
          const n = (k) => (f.get(k) === '' ? null : parseInt(f.get(k), 10));
          busy(e.target.querySelector('[type=submit]'), async () => {
            await post(`/admin/matches/${m.id}/resolve`, { scoreHome: n('h'), scoreAway: n('a'), pensHome: n('ph'), pensAway: n('pa') });
            close(); toast('Match settled'); redraw();
          });
        };
      },
    });
  }));
}

async function withdrawals(body, redraw) {
  const { withdrawals: list } = await get('/admin/withdrawals?status=pending');
  if (!list.length) { body.innerHTML = String(html`<div class="card flat">${empty('bank', 'No pending withdrawals', 'New requests appear here for approval.')}</div>`); return; }
  body.innerHTML = String(html`<div class="stack">${list.map((w) => html`<div class="card">
    <div class="row between"><div class="h2 num">${money(w.amount, w.currency)}</div><span class="small faint">${timeAgo(w.createdAt)}</span></div>
    <div class="small" style="margin-top:8px"><b>${w.accountName}</b> · ${w.bankName} · ${w.accountNumber}</div>
    <div class="small muted">${w.username} (${w.email})</div>
    <div class="grid-2" style="margin-top:12px"><button class="btn danger sm" data-reject="${w.id}">Reject</button><button class="btn primary sm" data-approve="${w.id}">Approve & send</button></div>
  </div>`)}</div>`);
  body.querySelectorAll('[data-approve]').forEach((b) => (b.onclick = async () => {
    const w = list.find((x) => x.id === b.dataset.approve);
    if (await confirmSheet({ title: 'Send ' + money(w.amount, w.currency) + '?', message: `Paystack will transfer the money to ${w.accountName} (${w.bankName}).`, confirm: 'Approve & send' }))
      busy(b, async () => { await post(`/admin/withdrawals/${w.id}/approve`); toast('Transfer started'); redraw(); });
  }));
  body.querySelectorAll('[data-reject]').forEach((b) => (b.onclick = () => {
    sheet(String(html`${sheetHead('Reject withdrawal')}<form id="rj"><label class="field"><span class="label">Reason shown to the player</span>
      <input class="input" name="reason" required placeholder="e.g. Account name doesn't match your profile"></label>
      <div class="sheet-actions"><button class="btn danger block">Reject & refund</button></div></form>`), {
      onMount(root, close) {
        root.querySelector('#rj').onsubmit = (e) => {
          e.preventDefault();
          busy(e.target.querySelector('button'), async () => { await post(`/admin/withdrawals/${b.dataset.reject}/reject`, { reason: new FormData(e.target).get('reason') }); close(); toast('Rejected and refunded'); redraw(); });
        };
      },
    });
  }));
}

async function tournaments(body, redraw) {
  const { tournaments: list } = await get('/tournaments');
  body.innerHTML = String(html`<button class="btn primary block" id="new">${ic('plus')} Create tournament</button>
    <div class="list" style="margin-top:16px">${list.length ? list.map((t) => html`<div class="list-row">
      <div class="grow"><div class="h3">${t.name}</div><div class="small muted">${t.format} · ${t.gameName} · ${t.playerCount}/${t.maxPlayers} · ${t.entryFee ? money(t.entryFee, t.currency) : 'free'} · ${t.status}</div></div>
      ${t.status === 'open' ? html`<button class="btn danger sm" data-cancel="${t.id}">Cancel</button>` : html`<a class="btn secondary sm" href="/tournaments/${t.id}">View</a>`}</div>`) : html`<div class="list-row muted">No tournaments yet</div>`}</div>`);

  body.querySelectorAll('[data-cancel]').forEach((b) => (b.onclick = async () => {
    if (await confirmSheet({ title: 'Cancel tournament?', message: 'Everyone who joined gets their entry fee back.', confirm: 'Cancel tournament', danger: true }))
      busy(b, async () => { await del('/admin/tournaments/' + b.dataset.cancel); toast('Tournament cancelled'); redraw(); });
  }));
  body.querySelector('#new').onclick = () => {
    let game = store.get().games[0].id;
    let format = 'knockout';
    sheet(String(html`${sheetHead('Create tournament')}
      <form id="ct">
        <label class="field"><span class="label">Name</span><input class="input" name="name" required maxlength="60" placeholder="e.g. Friday Night Cup"></label>
        <div class="field"><span class="label">Game</span><div id="g">${gameChips(game)}</div></div>
        <div class="field"><span class="label">Format</span><div class="seg" id="fmt"><button type="button" data-f="knockout" class="active">Knockout</button><button type="button" data-f="league">League</button></div></div>
        <div class="grid-2" style="margin-top:14px">
          <label class="field"><span class="label">Players</span><input class="input num" name="max" inputmode="numeric" value="8" required></label>
          <label class="field" style="margin-top:0"><span class="label">Entry fee (₦, 0 = free)</span><input class="input num" name="fee" inputmode="numeric" value="0"></label></div>
        <div class="grid-2" style="margin-top:14px">
          <label class="field"><span class="label">Hours per round</span><input class="input num" name="hours" inputmode="numeric" value="24"></label>
          <label class="field" style="margin-top:0"><span class="label">Prize seed (free only, ₦)</span><input class="input num" name="seed" inputmode="numeric" value="0"></label></div>
        <label class="field"><span class="label">Prize split % (1st, 2nd, …)</span><input class="input" name="split" value="70"></label>
        <p class="hint" id="split-hint">Knockout default: 70 (winner only). League default: 30, 10, 4.5, 4.5. The rest is the platform's share.</p>
        <div class="sheet-actions"><button class="btn primary block" type="submit">Create</button></div></form>`), {
      onMount(root, close) {
        const f = root.querySelector('#ct');
        const bindG = () => root.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => { game = b.dataset.game; root.querySelector('#g').innerHTML = String(gameChips(game)); bindG(); }));
        bindG();
        root.querySelectorAll('[data-f]').forEach((b) => (b.onclick = () => {
          format = b.dataset.f;
          root.querySelectorAll('[data-f]').forEach((x) => x.classList.toggle('active', x === b));
          f.split.value = format === 'league' ? '30, 10, 4.5, 4.5' : '70';
          f.max.value = format === 'league' ? '16' : '8';
        }));
        f.onsubmit = (e) => {
          e.preventDefault();
          const num = (k) => parseFloat(f[k].value) || 0;
          busy(f.querySelector('[type=submit]'), async () => {
            await post('/admin/tournaments', {
              name: f.name.value, game, format, maxPlayers: num('max'), entryFee: Math.round(num('fee') * 100),
              roundHours: num('hours'), prizePool: Math.round(num('seed') * 100),
              prizeSplit: f.split.value.split(',').map((x) => parseFloat(x)).filter((x) => !isNaN(x)),
            });
            close(); toast('Tournament created'); redraw();
          });
        };
      },
    });
  };
}

async function users(body) {
  body.innerHTML = String(html`<form id="us" class="row"><input class="input grow" name="q" placeholder="Search email, name or ID" aria-label="Search users"><button class="btn secondary">${ic('search')}</button></form><div id="res" style="margin-top:14px"></div>`);
  const res = body.querySelector('#res');
  body.querySelector('#us').onsubmit = async (e) => {
    e.preventDefault();
    const { users: list } = await get('/admin/users?q=' + encodeURIComponent(new FormData(e.target).get('q')));
    res.innerHTML = String(list.length ? html`<div class="list">${list.map((u) => html`<div class="list-row">
      <div class="grow"><div class="h3">${u.username}${u.isAdmin ? ' (admin)' : ''}</div><div class="small muted ellipsis">${u.email}</div><div class="tiny faint">${u.id}</div></div>
      <div style="text-align:right"><b class="num">${money(u.balance, u.currency)}</b><br><button class="link-btn small" data-adj="${u.id}" data-cur="${u.currency}">Adjust</button></div></div>`)}</div>`
      : html`<div class="card flat">${empty('search', 'No users found', 'Try another search.')}</div>`);
    res.querySelectorAll('[data-adj]').forEach((b) => (b.onclick = () => {
      sheet(String(html`${sheetHead('Adjust balance')}<form id="adj">
        <label class="field"><span class="label">Amount (${b.dataset.cur}, negative to deduct)</span><input class="input num" name="amt" required inputmode="decimal"></label>
        <label class="field"><span class="label">Reason (logged)</span><input class="input" name="reason" required minlength="5"></label>
        <div class="sheet-actions"><button class="btn primary block">Apply</button></div></form>`), {
        onMount(root, close) {
          root.querySelector('#adj').onsubmit = (ev) => {
            ev.preventDefault();
            const f = new FormData(ev.target);
            busy(ev.target.querySelector('button'), async () => {
              await post('/admin/balance', { userId: b.dataset.adj, amount: Math.round(parseFloat(f.get('amt')) * 100), reason: f.get('reason') });
              close(); toast('Balance adjusted'); body.querySelector('#us').requestSubmit();
            });
          };
        },
      });
    }));
  };
}

async function ocr(body) {
  let game = store.get().games[0].id;
  body.innerHTML = String(html`<div class="card"><div class="h3">Test the screenshot reader</div>
    <p class="small muted" style="margin-top:4px">Nothing is saved and no match is affected.</p>
    <form id="ot" style="margin-top:14px"><div id="g">${gameChips(game)}</div>
      <div class="grid-2" style="margin-top:14px"><label class="field"><span class="label">Player 1 name</span><input class="input" name="player1"></label>
      <label class="field" style="margin-top:0"><span class="label">Player 2 name</span><input class="input" name="player2"></label></div>
      <label class="field"><span class="label">Screenshot</span><input class="input" type="file" name="image" accept="image/*" required style="padding-top:12px"></label>
      <button class="btn primary block" style="margin-top:14px">${ic('zap')} Analyze</button></form>
    <div id="out" style="margin-top:14px"></div></div>`);
  const bindG = () => body.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => { game = b.dataset.game; body.querySelector('#g').innerHTML = String(gameChips(game)); bindG(); }));
  bindG();
  body.querySelector('#ot').onsubmit = (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    fd.append('game', game);
    busy(e.target.querySelector('button'), async () => {
      const r = await upload('/admin/ocr-test', fd);
      body.querySelector('#out').innerHTML = String(html`<div class="notice ${r.verdict === 'accepted' ? 'brand' : 'danger'}">${ic(r.verdict === 'accepted' ? 'checkCircle' : 'xCircle')}<div><b>${r.verdict}</b></div></div>
        <pre style="white-space:pre-wrap;font-size:13px;color:var(--text-2);margin-top:12px">${JSON.stringify(r.ocr, null, 2)}</pre>`);
    });
  };
}
