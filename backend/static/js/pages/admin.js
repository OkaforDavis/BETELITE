import { get, post, del, upload } from '../api.js';
import { store, gameName } from '../store.js';
import { html, ic, avatar, money, timeAgo, empty, skeleton, sheet, sheetHead, busy, toast, confirmSheet, statusBadge, scoreLine } from '../ui.js';
import { sk } from '../skeletons.js';
import { gameChips, loadEvidence, evidenceGallery, bindEvidence } from '../components.js';

const TABS = [['overview', 'Overview'], ['review', 'Review'], ['withdrawals', 'Withdrawals'], ['tournaments', 'Tournaments'], ['support', 'Support'], ['users', 'Users'], ['ocr', 'OCR test']];

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
    body.innerHTML = String(sk.adminRows());
    try { await ({ overview, review, withdrawals, tournaments, support, users, ocr })[tab](body, draw); }
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
      <button class="btn secondary sm" data-ocr="${m.id}">${ic('image')} Evidence</button>
      <button class="btn danger sm" data-void="${m.id}">${ic('xCircle')} Void</button>
      <button class="btn primary sm" data-resolve="${m.id}">${ic('check')} Decide</button></div>
  </div>`)}</div>`);

  body.querySelectorAll('[data-ocr]').forEach((b) => (b.onclick = () => busy(b, async () => {
    const [{ ocr }, shots] = await Promise.all([get(`/admin/matches/${b.dataset.ocr}/ocr`), loadEvidence(b.dataset.ocr)]);
    sheet(String(html`${sheetHead('Evidence')}${evidenceGallery(shots)}
      <div class="h3" style="margin-top:18px">What the AI read</div>
      <pre style="white-space:pre-wrap;font-size:13px;color:var(--text-2)">${ocr ? JSON.stringify(ocr, null, 2) : 'No AI reading for this match.'}</pre>`),
    { label: 'Evidence', onMount: (root) => bindEvidence(root) });
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
    <div class="row between"><div><div class="h2 num">${money(w.payout ?? w.amount, w.currency)}</div>${w.fee ? html`<div class="tiny faint">${money(w.amount, w.currency)} minus ${money(w.fee, w.currency)} transfer fee</div>` : ""}</div><span class="small faint">${timeAgo(w.createdAt)}</span></div>
    <div class="small" style="margin-top:8px"><b>${w.accountName}</b> · ${w.bankName} · ${w.accountNumber}</div>
    <div class="small muted">${w.username} (${w.email})</div>
    <div class="grid-3" style="margin-top:12px"><button class="btn danger sm" data-reject="${w.id}">Reject</button><button class="btn secondary sm" data-manual="${w.id}">Mark paid</button><button class="btn primary sm" data-approve="${w.id}">Paystack</button></div>
    <p class="tiny faint" style="margin-top:8px">"Paystack" sends the money automatically. "Mark paid" is for when you sent the bank transfer yourself.</p>
  </div>`)}</div>`);
  body.querySelectorAll('[data-manual]').forEach((b) => (b.onclick = () => {
    const w = list.find((x) => x.id === b.dataset.manual);
    sheet(String(html`${sheetHead('Mark as paid', money(w.payout ?? w.amount, w.currency) + ' to ' + w.accountName)}
      <div class="notice warn">${ic('alert')}<div class="small">Only do this <b>after</b> you have sent <b>${money(w.payout ?? w.amount, w.currency)}</b> to
        <b>${w.accountName}</b>, ${w.bankName} ${w.accountNumber}. The player is told the money has been sent.</div></div>
      <form id="mp" style="margin-top:14px"><label class="field"><span class="label">Bank transfer reference</span>
        <input class="input" name="note" required minlength="3" placeholder="e.g. session ID from your bank app"></label>
        <div class="sheet-actions"><button class="btn primary block">Confirm paid</button></div></form>`), {
      onMount(root, close) {
        root.querySelector('#mp').onsubmit = (e) => {
          e.preventDefault();
          busy(e.target.querySelector('button'), async () => {
            await post(`/admin/withdrawals/${w.id}/mark-paid`, { note: new FormData(e.target).get('note') });
            close(); toast('Marked as paid. The player has been notified.'); redraw();
          });
        };
      },
    });
  }));
  body.querySelectorAll('[data-approve]').forEach((b) => (b.onclick = async () => {
    const w = list.find((x) => x.id === b.dataset.approve);
    if (await confirmSheet({ title: 'Send ' + money(w.payout ?? w.amount, w.currency) + '?', message: `Paystack will transfer the money to ${w.accountName} (${w.bankName}).`, confirm: 'Approve & send' }))
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
      <div class="grow"><div class="h3">${t.name}</div><div class="small muted">${t.format} · ${t.gameName} · ${t.playerCount}/${t.maxPlayers} · ${t.entryFee ? money(t.entryFee, t.currency) : t.fixedPrizes ? '1st ' + money(t.fixedPrizes.GHS?.[0] || 0, 'GHS') : 'free'} · ${t.status}</div></div>
      ${t.status === 'open' ? html`<button class="btn danger sm" data-cancel="${t.id}">Cancel</button>` : html`<a class="btn secondary sm" href="/tournaments/${t.id}">View</a>`}</div>`) : html`<div class="list-row muted">No tournaments yet</div>`}</div>`);

  body.querySelectorAll('[data-cancel]').forEach((b) => (b.onclick = async () => {
    if (await confirmSheet({ title: 'Cancel tournament?', message: 'Everyone who joined gets their entry back.', confirm: 'Cancel tournament', danger: true }))
      busy(b, async () => { await del('/admin/tournaments/' + b.dataset.cancel); toast('Tournament cancelled'); redraw(); });
  }));
  body.querySelector('#new').onclick = () => {
    let game = store.get().games[0].id;
    let format = 'knockout';
    const staking = store.get().stakingEnabled;
    // Suggested fixed prizes (major units) at about ₵1 = ₦115. Each paid seat is
    // worth about ₦1,800 (₵15); these keep prizes near half of that with 75% paid seats.
    const PRESETS = {
      knockout: { 8: [[50], [5800]], 16: [[100], [11500]], 32: [[150], [17300]], 64: [[300], [34500]] },
      swiss: { 8: [[30, 12, 8], [3500, 1400, 900]], 16: [[60, 25, 15], [7000, 2900, 1700]], 32: [[100, 50, 30], [11500, 5800, 3500]], 64: [[150, 90, 60], [17300, 10400, 6900]] },
      league: { 8: [[30, 12, 8], [3500, 1400, 900]], 16: [[60, 25, 15], [7000, 2900, 1700]] },
    };
    const NAMES = { 'swiss-32': 'Top Player', 'swiss-64': 'End-of-Month Platter', 'knockout-64': 'End-of-Month Platter' };
    sheet(String(html`${sheetHead('Create tournament', 'Fixed prizes in cedis and naira, paid by CrestArena')}
      <form id="ct">
        <label class="field"><span class="label">Name</span><input class="input" name="name" required maxlength="60" placeholder="e.g. Friday Night Cup"></label>
        <div class="field"><span class="label">Game</span><div id="g">${gameChips(game)}</div></div>
        <div class="field"><span class="label">Format</span><div class="seg" id="fmt">
          <button type="button" data-f="knockout" class="active">Knockout</button><button type="button" data-f="swiss">Points</button><button type="button" data-f="league">League</button></div>
          <p class="hint" id="fmt-hint"></p></div>
        <div class="grid-2" style="margin-top:14px">
          <label class="field"><span class="label">Players</span><select class="select" name="max"><option>4</option><option selected>8</option><option>16</option><option>32</option><option>64</option></select></label>
          <label class="field" style="margin-top:0"><span class="label">Hours per round</span><input class="input num" name="hours" inputmode="numeric" value="24"></label></div>
        <div class="grid-2" style="margin-top:14px">
          <label class="field"><span class="label">Prizes ₵ (1st, 2nd, …)</span><input class="input" name="ghs" required></label>
          <label class="field" style="margin-top:0"><span class="label">Prizes ₦ (1st, 2nd, …)</span><input class="input" name="ngn" required></label></div>
        <div class="grid-2" style="margin-top:14px">
          <label class="field"><span class="label">Early sign-up (hours, 0 = none)</span><input class="input num" name="early" inputmode="numeric" value="12"></label>
          <label class="field" style="margin-top:0"><span class="label">Early sign-up for</span><select class="select" name="earlyTier"><option value="pro">All members</option><option value="elite">Elite only</option></select></label></div>
        <label class="field"><span class="label">Free-plan seats (max)</span><input class="input num" name="freeSeats" inputmode="numeric"></label>
        <p class="hint">Default 25% of seats. Free players still get in, but most seats go to members so the prizes stay affordable.</p>
        ${staking ? html`<label class="field"><span class="label">Entry fee (₦, 0 = prize tournament)</span><input class="input num" name="fee" inputmode="numeric" value="0"></label>` : ''}
        <p class="hint">Every player uses one monthly entry. Prizes are the same for everyone and don't depend on how many members paid.</p>
        <div class="sheet-actions"><button class="btn primary block" type="submit">Create</button></div></form>`), {
      onMount(root, close) {
        const f = root.querySelector('#ct');
        const hint = root.querySelector('#fmt-hint');
        const HINTS = {
          knockout: 'Lose once and you are out. Pays 1st (and optionally 2nd).',
          swiss: 'Everyone plays every round (8 players: 3 rounds, 16: 4, 32: 5, 64: 6) against players on similar points. Top 3 by points win.',
          league: 'Everyone plays everyone once. Best for 8 players or fewer (up to 20).',
        };
        const preset = () => {
          const p = PRESETS[format]?.[f.max.value];
          if (p) { f.ghs.value = p[0].join(', '); f.ngn.value = p[1].join(', '); }
          hint.textContent = HINTS[format];
          f.freeSeats.value = Math.max(1, Math.floor(+f.max.value / 4));
          const suggested = NAMES[format + '-' + f.max.value];
          if (suggested && !f.name.value) f.name.value = suggested;
          if (+f.max.value === 64) f.earlyTier.value = 'elite';
        };
        const bindG = () => root.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => { game = b.dataset.game; root.querySelector('#g').innerHTML = String(gameChips(game)); bindG(); }));
        bindG();
        root.querySelectorAll('[data-f]').forEach((b) => (b.onclick = () => {
          format = b.dataset.f;
          root.querySelectorAll('[data-f]').forEach((x) => x.classList.toggle('active', x === b));
          if (format === 'league' && +f.max.value > 16) f.max.value = '8';
          preset();
        }));
        f.max.onchange = preset;
        preset();
        const list = (s) => s.split(',').map((x) => Math.round(parseFloat(x) * 100)).filter((x) => x > 0);
        f.onsubmit = (e) => {
          e.preventDefault();
          const num = (k) => parseFloat(f[k]?.value) || 0;
          const ghs = list(f.ghs.value);
          const ngn = list(f.ngn.value);
          if (ghs.length !== ngn.length) return toast('Give the same number of prizes in cedis and naira', 'err');
          busy(f.querySelector('[type=submit]'), async () => {
            await post('/admin/tournaments', {
              name: f.name.value, game, format, maxPlayers: num('max'), roundHours: num('hours'), earlyHours: num('early'),
              earlyTier: f.earlyTier.value, freeSeats: num('freeSeats'),
              entryFee: Math.round(num('fee') * 100), prizes: { GHS: ghs, NGN: ngn },
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
      ${avatar(u.username, u.avatarUrl, "sm")}
      <div class="grow"><div class="h3">${u.username}${u.isAdmin ? ' (admin)' : ''}</div><div class="small muted ellipsis">${u.email}</div><div class="tiny faint">${u.id}</div></div>
      <div style="text-align:right"><b class="num">${money(u.balance, u.currency)}</b><br><button class="link-btn small" data-adj="${u.id}" data-cur="${u.currency}">Adjust</button>${u.avatarUrl ? html`<br><button class="link-btn small danger-text" data-unphoto="${u.id}">Remove photo</button>` : ""}</div></div>`)}</div>`
      : html`<div class="card flat">${empty('search', 'No users found', 'Try another search.')}</div>`);
    res.querySelectorAll("[data-unphoto]").forEach((b) => (b.onclick = async () => {
      if (await confirmSheet({ title: "Remove this photo?", message: "The player is notified and can upload a different one.", confirm: "Remove photo", danger: true }))
        busy(b, async () => { await del("/admin/users/" + b.dataset.unphoto + "/avatar"); toast("Photo removed"); body.querySelector("#us").requestSubmit(); });
    }));
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

// Complaints from players: read, reply, close.
async function support(body, redraw) {
  let status = 'open';
  const draw = async () => {
    const { tickets, categories } = await get('/admin/support?status=' + status);
    body.innerHTML = String(html`<div class="seg" role="tablist">${[['open', 'Open'], ['answered', 'Answered'], ['closed', 'Closed']].map(([k, l]) => html`<button data-st="${k}" class="${k === status ? 'active' : ''}">${l}</button>`)}</div>
      <div class="stack" style="margin-top:14px">${tickets.length ? tickets.map((t) => html`<div class="card">
        <div class="row between"><span class="badge">${categories[t.category] || t.category}</span><span class="small faint">${timeAgo(t.createdAt)}</span></div>
        <div class="h3" style="margin-top:10px">${t.subject}</div>
        <div class="small muted">${t.username} · ${t.email}${t.refId ? ' · ref ' + t.refId : ''}</div>
        <p style="margin-top:10px;white-space:pre-wrap">${t.message}</p>
        ${t.adminReply ? html`<div class="notice" style="margin-top:10px">${ic('send')}<div class="small"><b>Your reply:</b> ${t.adminReply}</div></div>` : ''}
        <div class="row" style="gap:8px;margin-top:12px"><button class="btn primary sm" data-reply="${t.id}">${ic('send')} Reply</button></div>
      </div>`) : html`<div class="card flat">${empty('checkCircle', 'Nothing here', 'No ' + status + ' messages.')}</div>`}</div>`);
    body.querySelectorAll('[data-st]').forEach((b) => (b.onclick = () => { status = b.dataset.st; draw(); }));
    body.querySelectorAll('[data-reply]').forEach((b) => (b.onclick = () => reply(tickets.find((t) => t.id === b.dataset.reply))));
  };
  function reply(t) {
    sheet(String(html`${sheetHead('Reply to ' + t.username, t.subject)}
      <form id="rp"><textarea class="input" name="reply" rows="5" required placeholder="Your reply (the player gets a notification)"></textarea>
        <label class="row" style="gap:8px;margin-top:12px"><input type="checkbox" name="close" checked> Mark as resolved</label>
        <div class="sheet-actions"><button class="btn primary block">Send reply</button></div></form>`), {
      onMount(root, close) {
        root.querySelector('#rp').onsubmit = (e) => {
          e.preventDefault();
          const f = e.target;
          busy(f.querySelector('button'), async () => {
            await post('/admin/support/' + t.id + '/reply', { reply: f.reply.value, close: f.close.checked });
            close(); toast('Reply sent'); draw();
          });
        };
      },
    });
  }
  await draw();
}
