import { get, post } from '../api.js';
import { store, gameName } from '../store.js';
import { html, ic, money, sym, avatar, timeAgo, sheet, sheetHead, busy, toast, empty, skeleton, confirmSheet } from '../ui.js';
import { matchHero, matchRow, gameChips, hasGameProfile } from '../components.js';
import { gameIdSheet } from '../gameid.js';
import { needsConsent, consentGate } from '../consent.js';
import { navigate } from '../router.js';
import { refreshProfile, refreshCurrent } from '../app.js';
import * as rt from '../realtime.js';

const WIN_PCT = 80;
const QUICK = { NGN: [500, 1000, 2000, 5000], GHS: [10, 20, 50, 100] };

export default async function play(view, { query }) {
  let tab = query.tab === 'mine' ? 'mine' : 'open';
  let filter = '';

  view.innerHTML = String(html`<div class="page">
    <div class="row between">
      <div><div class="h1">Play 1v1</div><div class="muted small" style="margin-top:4px">Stake, play, upload the result. Winner takes ${WIN_PCT}%.</div></div>
    </div>
    <button class="btn primary block" style="margin-top:16px" id="new">${ic('plus')} Post a challenge</button>
    <div class="seg" style="margin-top:20px" role="tablist">
      <button role="tab" data-tab="open">Open challenges</button>
      <button role="tab" data-tab="mine">My matches</button>
    </div>
    <div id="body" style="margin-top:16px"></div>
  </div>`);

  const body = view.querySelector('#body');
  view.querySelector('#new').onclick = () => createChallenge(() => { tab = 'open'; draw(); });
  view.querySelectorAll('[data-tab]').forEach((b) => (b.onclick = () => { tab = b.dataset.tab; draw(); }));

  async function draw() {
    view.querySelectorAll('[data-tab]').forEach((b) => b.classList.toggle('active', b.dataset.tab === tab));
    body.innerHTML = String(skeleton(84, 3));
    if (tab === 'open') await drawOpen(); else await drawMine();
  }

  async function drawOpen() {
    const { challenges } = await get('/lobby');
    const uid = store.get().profile.id;
    const list = challenges.filter((c) => !filter || c.game === filter);
    body.innerHTML = String(html`${gameChips(filter, { all: true })}
      <div style="margin-top:14px">${list.length ? html`<div class="list">${list.map((c) => html`
        <div class="list-row">
          ${avatar(c.creatorName, '', c.creatorId === uid ? 'me' : '')}
          <div class="grow">
            <div class="h3 ellipsis">${c.creatorId === uid ? 'Your challenge' : c.creatorName}</div>
            <div class="small muted ellipsis">${c.gameName} · ${c.creatorTag || '—'} · ${timeAgo(c.createdAt)}</div>
          </div>
          <div style="text-align:right">
            <div class="h3 num">${money(c.amount, c.currency)}</div>
            <div class="tiny brand-text">Win ${money(c.prize, c.currency)}</div>
          </div>
          ${c.creatorId === uid
            ? html`<button class="btn danger sm" data-cancel="${c.id}" aria-label="Cancel challenge">${ic('x')}</button>`
            : html`<button class="btn primary sm" data-accept="${c.id}">Accept</button>`}
        </div>`)}</div>`
      : html`<div class="card flat">${empty('swords', 'No open challenges', filter ? 'No challenges for this game yet. Post one and others can accept it.' : 'Be the first: post a challenge and wait for an opponent.')}</div>`}</div>`);

    body.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => { filter = b.dataset.game; drawOpen(); }));
    body.querySelectorAll('[data-accept]').forEach((b) => (b.onclick = () => accept(challenges.find((c) => c.id === b.dataset.accept), b)));
    body.querySelectorAll('[data-cancel]').forEach((b) => (b.onclick = async () => {
      if (!(await confirmSheet({ title: 'Cancel challenge?', message: 'Your stake goes straight back to your wallet.', confirm: 'Cancel challenge', danger: true }))) return;
      busy(b, async () => { await post('/lobby/delete', { challengeId: b.dataset.cancel }); toast('Challenge cancelled. Stake refunded.'); refreshProfile(); drawOpen(); });
    }));
  }

  async function drawMine() {
    const { matches } = await get('/matches/mine');
    const active = matches.filter((m) => ['ready', 'submitted', 'disputed', 'review'].includes(m.status));
    const done = matches.filter((m) => !active.includes(m));
    body.innerHTML = String(html`
      ${active.length ? html`<div class="stack">${active.map(matchHero)}</div>` : ''}
      <div class="section-head" style="margin-top:${active.length ? 24 : 0}px"><h2 class="h2">History</h2></div>
      ${done.length ? html`<div class="list">${done.map(matchRow)}</div>`
        : html`<div class="card flat">${empty('clock', 'No matches yet', 'Your finished matches and results will show here.')}</div>`}`);
  }

  async function accept(c, btn) {
    const profile = store.get().profile;
    if (needsConsent(profile)) return consentGate(profile, () => refreshProfile());
    if (!hasGameProfile(c.game)) return gameIdSheet(c.game, () => accept(c, btn));
    if (profile.balance < c.amount) {
      toast(`You need ${money(c.amount, c.currency)} to accept. Top up your wallet first.`, 'info');
      return navigate('/wallet?deposit=' + Math.ceil((c.amount - profile.balance) / 100));
    }
    const ok = await confirmSheet({
      title: `Accept ${money(c.amount, c.currency)} challenge?`,
      message: `You and ${c.creatorName} each stake ${money(c.amount, c.currency)}. The winner gets ${money(c.prize, c.currency)}. Play ${gameName(c.game)} within 3 hours and upload the full-time screen showing both usernames.`,
      confirm: 'Accept & lock stake',
    });
    if (!ok) return;
    busy(btn, async () => {
      const { matchId } = await post('/lobby/accept', { challengeId: c.id });
      toast("Challenge accepted. It's game time!", 'ok');
      refreshProfile();
      refreshCurrent();
      navigate('/match/' + matchId);
    });
  }

  const offA = rt.on('lobby_new_challenge', () => tab === 'open' && drawOpen());
  const offB = rt.on('lobby_challenge_removed', () => tab === 'open' && drawOpen());
  await draw();
  return () => { offA(); offB(); };
}

function createChallenge(onDone) {
  const profile = store.get().profile;
  if (needsConsent(profile)) return consentGate(profile, () => refreshProfile());
  const cur = profile.currency;
  const games = store.get().games;
  let game = profile.gameProfiles[0]?.game || games[0].id;
  let amount = QUICK[cur][1];

  sheet(String(html`${sheetHead('Post a challenge', 'Your stake is held safely until the match is settled')}
    <form id="cc">
      <span class="label">Game</span>
      <div id="games">${gameChips(game)}</div>
      <div class="field" style="margin-top:18px"><span class="label">Your stake</span>
        <div class="input-money"><span class="cur">${sym(cur)}</span><input class="input num" name="amount" inputmode="numeric" pattern="[0-9]*" value="${amount}" aria-label="Stake amount"></div>
        <div class="chips" style="margin-top:10px">${QUICK[cur].map((q) => html`<button type="button" class="chip" data-q="${q}">${money(q * 100, cur)}</button>`)}</div>
      </div>
      <div class="card flat" style="margin-top:18px" id="summary"></div>
      <ul class="checklist" style="margin-top:16px">
        <li class="ok">${ic('check')}<span>Play a <b>player-vs-player</b> match (not vs AI or a scenario)</span></li>
        <li class="ok">${ic('check')}<span>Upload the <b>full-time screen</b> showing both usernames within 3 hours</span></li>
        <li class="ok">${ic('check')}<span>Your opponent has 15 minutes to dispute, then you're paid automatically</span></li>
      </ul>
      <div class="sheet-actions"><button class="btn primary block" type="submit">Post challenge</button></div>
    </form>`), {
    label: 'Post a challenge',
    onMount(root, close) {
      const input = root.querySelector('[name=amount]');
      const summary = root.querySelector('#summary');
      const update = () => {
        amount = Math.max(0, parseInt(input.value.replace(/\D/g, ''), 10) || 0);
        const stake = amount * 100;
        summary.innerHTML = String(html`
          <div class="row between small"><span class="muted">You stake</span><b class="num">${money(stake, cur)}</b></div>
          <div class="row between small" style="margin-top:6px"><span class="muted">Opponent stakes</span><b class="num">${money(stake, cur)}</b></div>
          <div class="divider" style="margin:10px 0"></div>
          <div class="row between"><span>Winner gets</span><b class="h3 num brand-text">${money(Math.floor(stake * 2 * WIN_PCT / 100), cur)}</b></div>
          <div class="tiny faint" style="margin-top:6px">Draw = both stakes refunded. Platform fee ${100 - WIN_PCT}% of the pot.</div>`);
      };
      const bindGames = () => root.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => {
        game = b.dataset.game;
        root.querySelector('#games').innerHTML = String(gameChips(game));
        bindGames();
      }));
      bindGames();
      input.oninput = update;
      root.querySelectorAll('[data-q]').forEach((b) => (b.onclick = () => { input.value = b.dataset.q; update(); }));
      update();
      root.querySelector('#cc').onsubmit = (e) => {
        e.preventDefault();
        if (!hasGameProfile(game)) { close(); return gameIdSheet(game, () => createChallenge(onDone)); }
        if (profile.balance < amount * 100) {
          close();
          toast('Not enough balance for this stake. Top up first.', 'info');
          return navigate('/wallet?deposit=' + amount);
        }
        busy(e.target.querySelector('[type=submit]'), async () => {
          await post('/lobby/create', { game, amount: amount * 100 });
          close();
          toast('Challenge posted. We’ll notify you when someone accepts.');
          refreshProfile();
          onDone();
        });
      };
    },
  });
}
