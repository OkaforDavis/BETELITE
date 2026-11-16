import { get, post } from '../api.js';
import { store } from '../store.js';
import { html, ic, skeleton, empty, money, sym, sheet, sheetHead, busy, toast } from '../ui.js';
import { sk } from '../skeletons.js';
import { tournamentCard, gameChips, hasGameProfile } from '../components.js';
import { gameIdSheet } from '../gameid.js';
import { needsConsent, consentGate } from '../consent.js';
import { navigate } from '../router.js';
import { refreshProfile } from '../app.js';

export default async function tournaments(view) {
  let filter = '';
  let tab = 'open';
  const hosting = store.get().hostCupsEnabled;
  view.innerHTML = String(html`<div class="page">
    <div class="h1">Tournaments</div>
    <div class="muted small" style="margin-top:4px">Fixed prizes, no entry fees. CrestArena tournaments use one of your monthly entries; player-hosted cups are free and unlimited.</div>
    ${hosting ? html`<button class="btn primary block" style="margin-top:16px" id="host">${ic('trophy')} Host a cup</button>` : ''}
    <a class="notice brand" href="/membership" style="margin-top:14px;color:inherit">${ic('sparkles')}<div class="grow small"><b>Want more tournaments?</b> Members get up to 6 entries a month, early sign-up${hosting ? ', and Elite hosts a cup a month with no fee' : ''}.</div>${ic('chevronRight')}</a>
    <div class="seg" style="margin-top:18px" role="tablist">
      <button role="tab" data-tab="open">Open</button><button role="tab" data-tab="active">In progress</button><button role="tab" data-tab="mine">Mine</button>
    </div>
    <div id="filters" style="margin-top:14px"></div>
    <div id="list" style="margin-top:14px">${sk.tournamentList()}</div>
  </div>`);

  view.querySelector('#host')?.addEventListener('click', () => hostCupSheet());
  let { tournaments: all } = await get('/tournaments');

  function draw() {
    view.querySelectorAll('[data-tab]').forEach((b) => b.classList.toggle('active', b.dataset.tab === tab));
    view.querySelector('#filters').innerHTML = String(gameChips(filter, { all: true }));
    view.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => { filter = b.dataset.game; draw(); }));
    const uid = store.get().profile?.id;
    const list = all.filter((t) => (!filter || t.game === filter) &&
      (tab === 'mine' ? (t.joined || t.hostId === uid) : tab === 'open' ? t.status === 'open' : t.status === 'active'));
    view.querySelector('#list').innerHTML = String(list.length
      ? html`<div class="stack">${list.map(tournamentCard)}</div>`
      : html`<div class="card flat">${empty('trophy', tab === 'mine' ? "You haven't joined any yet" : 'Nothing here right now',
          tab === 'open' ? 'New tournaments are announced with a notification. Check back soon.' : 'Join an open tournament to compete for prizes.')}</div>`);
  }
  view.querySelectorAll('[data-tab]').forEach((b) => (b.onclick = () => { tab = b.dataset.tab; draw(); }));
  draw();

  async function reload() {
    ({ tournaments: all } = await get('/tournaments'));
    draw();
  }

  // Host a cup: I fund the prize pool (+ hosting fee); players join free.
  async function hostCupSheet() {
    const profile = store.get().profile;
    if (needsConsent(profile)) return consentGate(profile, () => refreshProfile());
    const o = await get('/tournaments/host-options');
    const cur = o.currency;
    const games = store.get().games;
    let game = profile.gameProfiles[0]?.game || games[0].id;
    let format = 'knockout';
    const SIZES = { knockout: [4, 8, 16, 32, 64], swiss: [8, 16, 32], league: [4, 6, 8] };
    const SPLITS = {
      knockout: [['Winner takes all', [100]], ['70 / 30', [70, 30]]],
      swiss: [['50 / 30 / 20', [50, 30, 20]], ['60 / 40', [60, 40]], ['Winner takes all', [100]]],
      league: [['50 / 30 / 20', [50, 30, 20]], ['60 / 40', [60, 40]], ['Winner takes all', [100]]],
    };
    let split = 0;
    const HINTS = {
      knockout: 'Lose once and you are out. Draws go to penalties.',
      swiss: 'Everyone plays every round against players on similar points. Top of the table wins.',
      league: 'Everyone plays everyone once. Best for small groups.',
    };
    sheet(String(html`${sheetHead('Host a cup', 'You put up the prize pool. Players join free.')}
      <form id="hc">
        <label class="field"><span class="label">Cup name</span><input class="input" name="name" required minlength="3" maxlength="40" placeholder="e.g. Saturday Showdown"></label>
        <div class="field"><span class="label">Game</span><div id="g">${gameChips(game)}</div></div>
        <div class="field"><span class="label">Format</span><div class="seg" id="fmt">
          <button type="button" data-f="knockout" class="active">Knockout</button><button type="button" data-f="swiss">Points</button><button type="button" data-f="league">League</button></div>
          <p class="hint" id="fmt-hint"></p></div>
        <div class="grid-2" style="margin-top:14px">
          <label class="field"><span class="label">Players</span><select class="select" name="max" id="max"></select></label>
          <label class="field" style="margin-top:0"><span class="label">Hours per round</span><select class="select" name="hours"><option>12</option><option selected>24</option><option>48</option></select></label></div>
        <div class="field" style="margin-top:14px"><span class="label">Prize pool</span>
          <div class="input-money"><span class="cur">${sym(cur)}</span><input class="input num" name="pool" inputmode="numeric" pattern="[0-9]*" value="${o.minPool / 100}" aria-label="Prize pool"></div>
          <span class="hint">Between ${money(o.minPool, cur)} and ${money(o.maxPool, cur)}</span></div>
        <div class="field"><span class="label">Prize split</span><div class="chips" id="splits"></div></div>
        <div class="card flat" style="margin-top:14px" id="summary"></div>
        <ul class="checklist" style="margin-top:14px">
          <li class="ok">${ic('check')}<span>Players join <b>free</b>. It starts automatically when full.</span></li>
          <li class="ok">${ic('check')}<span>Prizes are paid to the winners' wallets automatically when it ends.</span></li>
          <li class="ok">${ic('check')}<span>Not full in ${o.fillDays} days? Everything, fee included, comes back to you. You can also cancel before it starts.</span></li>
          <li class="ok">${ic('check')}<span>You can play in your own cup.</span></li>
        </ul>
        <div class="sheet-actions"><button class="btn primary block" type="submit" id="go">Host cup</button></div>
      </form>`), {
      label: 'Host a cup',
      onMount(root, close) {
        const f = root.querySelector('#hc');
        const maxSel = root.querySelector('#max');
        const fillSizes = () => { maxSel.innerHTML = SIZES[format].map((n) => `<option ${n === SIZES[format][1] ? 'selected' : ''}>${n}</option>`).join(''); };
        const drawSplits = () => {
          root.querySelector('#splits').innerHTML = String(html`${SPLITS[format].map(([label], i) => html`<button type="button" class="chip ${i === split ? 'active' : ''}" data-s="${i}">${label}</button>`)}`);
          root.querySelectorAll('[data-s]').forEach((b) => (b.onclick = () => { split = +b.dataset.s; drawSplits(); update(); }));
        };
        const update = () => {
          const pool = Math.max(0, parseInt(f.pool.value.replace(/\D/g, ''), 10) || 0) * 100;
          const fee = o.freeCup ? 0 : Math.ceil(pool * o.feePercent / 100);
          const pct = SPLITS[format][split][1];
          const prizes = pct.map((p) => Math.floor(pool * p / 100));
          prizes[0] += pool - prizes.reduce((a, b) => a + b, 0);
          root.querySelector('#fmt-hint').textContent = HINTS[format];
          root.querySelector('#go').textContent = `Host cup · ${money(pool + fee, cur)}`;
          root.querySelector('#summary').innerHTML = String(html`
            ${prizes.map((p, i) => html`<div class="row between small"><span class="muted">${['1st', '2nd', '3rd'][i]} place</span><b class="num">${money(p, cur)}</b></div>`)}
            <div class="row between small" style="margin-top:6px"><span class="muted">Hosting fee (${o.feePercent}%)</span><b class="num">${o.freeCup ? html`<span class="brand-text">Free (Elite)</span>` : money(fee, cur)}</b></div>
            <div class="divider" style="margin:10px 0"></div>
            <div class="row between"><span>Taken from your wallet now</span><b class="h3 num">${money(pool + fee, cur)}</b></div>
            <div class="tiny faint" style="margin-top:6px">Your balance: ${money(o.balance, cur)}. Winners from the other country are paid the same value in their currency.</div>`);
        };
        const bindG = () => root.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => { game = b.dataset.game; root.querySelector('#g').innerHTML = String(gameChips(game)); bindG(); }));
        bindG();
        root.querySelectorAll('[data-f]').forEach((b) => (b.onclick = () => {
          format = b.dataset.f; split = 0;
          root.querySelectorAll('[data-f]').forEach((x) => x.classList.toggle('active', x === b));
          fillSizes(); drawSplits(); update();
        }));
        f.pool.oninput = update;
        fillSizes(); drawSplits(); update();
        f.onsubmit = (e) => {
          e.preventDefault();
          if (!hasGameProfile(game)) { close(); return gameIdSheet(game, () => hostCupSheet()); }
          const pool = Math.max(0, parseInt(f.pool.value.replace(/\D/g, ''), 10) || 0) * 100;
          const total = pool + (o.freeCup ? 0 : Math.ceil(pool * o.feePercent / 100));
          if (total > o.balance) {
            close();
            toast(`You need ${money(total, cur)} in your wallet to host this cup.`, 'info');
            return navigate('/wallet?deposit=' + Math.ceil((total - o.balance) / 100));
          }
          busy(root.querySelector('#go'), async () => {
            const { tournamentId } = await post('/tournaments/host', { name: f.name.value, game, format, maxPlayers: +maxSel.value, pool, split: SPLITS[format][split][1], roundHours: +f.hours.value });
            close();
            toast('Your cup is live. Share it so it fills up!', 'ok');
            refreshProfile();
            navigate('/tournaments/' + tournamentId);
          });
        };
      },
    });
  }
}
