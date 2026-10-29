import { get, post } from '../api.js';
import { store } from '../store.js';
import { html, ic, avatar, money, skeleton, empty, busy, toast, confirmSheet } from '../ui.js';
import { matchRow, hasGameProfile, formatBadge, tournamentPrizes, tierBadge } from '../components.js';
import { gameIdSheet } from '../gameid.js';
import { needsConsent, consentGate } from '../consent.js';
import { navigate } from '../router.js';
import { refreshProfile, refreshCurrent } from '../app.js';

const ordinal = (n) => n + ({ 1: 'st', 2: 'nd', 3: 'rd' }[n] || 'th');

export default async function tournamentPage(view, { id }) {
  let tab = 'overview';
  let data;
  // The router already shows this page's skeleton.

  async function load() {
    try { data = await get('/tournaments/' + encodeURIComponent(id)); }
    catch (e) { view.innerHTML = String(html`<div class="page">${empty('alert', 'Tournament not found', e.message)}</div>`); return; }
    draw();
  }

  function draw() {
    const { tournament: t, standings, matches } = data;
    const uid = store.get().profile.id;
    const myMatch = matches.find((m) => (m.homeId === uid || m.awayId === uid) && ['ready', 'submitted', 'disputed', 'review'].includes(m.status));
    const pct = Math.round((t.playerCount / t.maxPlayers) * 100);
    const second = t.format === 'knockout' ? 'bracket' : 'table';
    const prizes = tournamentPrizes(t);
    const membersUntil = t.membersOnlyUntil && new Date(t.membersOnlyUntil) > new Date() ? new Date(t.membersOnlyUntil) : null;
    const roundLabel = t.totalRounds ? `Round ${t.currentRound} of ${t.totalRounds}` : `Round ${t.currentRound}`;

    view.innerHTML = String(html`<div class="page">
      <a href="/tournaments" class="row small muted" style="gap:4px;margin-bottom:12px">${ic('chevronLeft')}Tournaments</a>
      <div class="card pad-lg">
        <div class="row" style="gap:8px">
          ${formatBadge(t)}
          <span class="badge">${t.gameName}</span>
          ${t.status === 'active' ? html`<span class="badge live info">${roundLabel}</span>` : ''}
          ${t.status === 'finished' ? html`<span class="badge">Finished</span>` : ''}
        </div>
        <div class="h1" style="margin-top:12px">${t.name}</div>
        <div class="grid-3" style="margin-top:18px">
          <div class="stat"><div class="v num">${t.entryFee ? money(t.entryFee, t.currency) : '1 entry'}</div><div class="k">Entry</div></div>
          <div class="stat"><div class="v num brand-text">${money(prizes.list[0] || 0, prizes.cur)}</div><div class="k">1st prize</div></div>
          <div class="stat"><div class="v num">${t.playerCount}/${t.maxPlayers}</div><div class="k">Players</div></div>
        </div>
        ${t.status === 'open' ? html`
          <div class="progress" style="margin-top:16px"><span style="width:${pct}%"></span></div>
          <div class="small muted" style="margin-top:8px">${t.maxPlayers - t.playerCount} spots left${t.freeSeats != null ? html` · free spots ${Math.max(t.freeSeats - t.freeTaken, 0)} of ${t.freeSeats}` : ''} · starts automatically when full</div>
          ${membersUntil && !t.joined ? html`<div class="notice brand" style="margin-top:12px">${ic('sparkles')}<div class="small"><b>${t.earlyTier === 'elite' ? 'Elite' : 'Members'}' early sign-up</b> until ${membersUntil.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}. Then it opens to everyone.</div></div>` : ''}
          <div style="margin-top:16px">${t.joined
            ? html`<div class="notice brand">${ic('checkCircle')}<div><b>You're registered</b><div class="small">We'll notify you when it starts.</div></div></div>
                <button class="btn ghost block" id="leave" style="margin-top:8px">Leave tournament${t.entryFee ? ' (refund)' : ''}</button>`
            : html`<button class="btn primary block" id="join">${ic('trophy')} Join${t.entryFee ? ' for ' + money(t.entryFee, t.currency) : ' (uses 1 entry)'}</button>
                ${t.entryFee ? '' : html`<a class="small muted row" href="/membership" style="gap:4px;margin-top:10px;justify-content:center">${ic('sparkles')}Entries and plans</a>`}`}</div>` : ''}
      </div>

      ${myMatch ? html`<a class="notice brand" href="/match/${myMatch.id}" style="margin-top:16px;color:inherit">${ic('gamepad')}<div class="grow"><b>Your Round ${myMatch.round} match is ready</b>
        <div class="small">vs ${myMatch.homeId === uid ? myMatch.awayName : myMatch.homeName} · tap to play & upload the result</div></div>${ic('chevronRight')}</a>` : ''}

      <div class="seg" style="margin-top:20px" role="tablist">
        <button role="tab" data-tab="overview">Overview</button>
        <button role="tab" data-tab="${second}">${second === 'table' ? 'Table' : 'Bracket'}</button>
        <button role="tab" data-tab="matches">Matches</button>
      </div>
      <div id="tab-body" style="margin-top:16px"></div>
    </div>`);

    view.querySelectorAll('[data-tab]').forEach((b) => (b.onclick = () => { tab = b.dataset.tab; drawTab(); }));
    view.querySelector('#join')?.addEventListener('click', (e) => join(t, e.currentTarget));
    view.querySelector('#leave')?.addEventListener('click', async (e) => {
      if (!(await confirmSheet({ title: 'Leave tournament?', message: t.entryFee ? 'Your entry fee will be refunded to your wallet.' : 'You can join again while spots are open.', confirm: 'Leave', danger: true }))) return;
      busy(e.currentTarget, async () => { await post(`/tournaments/${t.id}/leave`); toast('You left the tournament'); refreshProfile(); load(); });
    });
    drawTab();

    function drawTab() {
      view.querySelectorAll('[data-tab]').forEach((b) => b.classList.toggle('active', b.dataset.tab === tab));
      const el = view.querySelector('#tab-body');
      if (tab === 'overview') el.innerHTML = String(overview(t));
      else if (tab === 'table') el.innerHTML = String(table(t, standings, uid));
      else if (tab === 'bracket') el.innerHTML = String(bracket(t, matches, uid));
      else el.innerHTML = String(matches.length ? html`<div class="list">${matches.map(matchRow)}</div>`
        : html`<div class="card flat">${empty('list', 'No matches yet', 'Fixtures are created automatically when the tournament is full.')}</div>`);
    }
  }

  async function join(t, btn) {
    const profile = store.get().profile;
    if (needsConsent(profile)) return consentGate(profile, () => refreshProfile());
    if (!hasGameProfile(t.game)) return gameIdSheet(t.game, () => join(t, btn));
    if (t.entryFee && profile.balance < t.entryFee) {
      toast('Top up your wallet to pay the entry fee.', 'info');
      return navigate('/wallet?deposit=' + Math.ceil((t.entryFee - profile.balance) / 100));
    }
    if (t.entryFee && !(await confirmSheet({ title: `Join ${t.name}?`, message: `${money(t.entryFee, t.currency)} will be taken from your wallet. Refundable until the tournament starts.`, confirm: 'Pay & join' }))) return;
    busy(btn, async () => {
      let started;
      try {
        ({ started } = await post(`/tournaments/${t.id}/join`));
      } catch (e) {
        // Out of entries, or members-only early sign-up: offer the plans.
        if (e.status === 403 && /entries|Members/.test(e.message)) {
          if (await confirmSheet({ title: 'Membership needed', message: e.message, confirm: 'See plans' })) navigate('/membership');
          return;
        }
        throw e;
      }
      toast(started ? 'The tournament is full and has started. Check your first match!' : "You're in! We'll notify you when it starts.");
      refreshProfile(); refreshCurrent(); load();
    });
  }

  await load();
}

function overview(t) {
  const prizes = tournamentPrizes(t);
  const fixed = !!t.fixedPrizes;
  const places = prizes.list.map((p, i) => [i + 1, p, t.prizeSplit?.[i]]).filter(([, p]) => p > 0);
  const other = fixed && Object.keys(t.fixedPrizes).find((c) => c !== prizes.cur);
  const rules = {
    league: html`<li class="ok">${ic('check')}<span>Everyone plays everyone once. Win = 3 pts, draw = 1.</span></li>
                 <li class="ok">${ic('check')}<span>Ties are split by goal difference, then goals scored.</span></li>`,
    swiss: html`<li class="ok">${ic('check')}<span>${t.totalRounds} rounds. Each round you play one match against a player on similar points, never the same opponent twice.</span></li>
                <li class="ok">${ic('check')}<span>Win = 3 pts, draw = 1. Nobody is knocked out; the top of the table wins.</span></li>
                <li class="ok">${ic('check')}<span>Ties are split by goal difference, then goals scored.</span></li>`,
    knockout: html`<li class="ok">${ic('check')}<span>Lose once and you're out. Draws go to penalties.</span></li>
                   <li class="ok">${ic('check')}<span>Upload the screen that shows the penalty shoot-out if there is one.</span></li>`,
  };
  return html`
    <div class="card"><div class="h3">Prizes</div>
      ${places.length ? html`<div class="list" style="margin-top:12px">${places.map(([pos, p, pct]) => html`
        <div class="list-row"><div class="icon-tile ${pos === 1 ? 'brand' : ''}">${ic(pos === 1 ? 'crown' : 'trophy')}</div>
        <div class="grow">${ordinal(pos)} place</div><div style="text-align:right"><b class="num">${money(p, prizes.cur)}</b>
        ${other ? html`<div class="tiny faint">${money(t.fixedPrizes[other][pos - 1], other)}</div>` : pct ? html`<div class="tiny faint">${pct}% of pool</div>` : ''}</div></div>`)}</div>`
      : html`<p class="muted small" style="margin-top:8px">No cash prizes for this tournament.</p>`}
      <p class="tiny faint" style="margin-top:10px">${fixed
        ? 'Fixed prizes, the same for every player. Winners are paid in their own currency.'
        : `Prize amounts shown for a full tournament${t.entryFee ? ', as a share of total entry fees' : ''}.`}</p>
    </div>
    <div class="card" style="margin-top:12px"><div class="h3">How it works</div>
      <ul class="checklist" style="margin-top:12px">
        ${rules[t.format] || rules.knockout}
        <li class="ok">${ic('check')}<span>Each round has ${t.roundHours} hours. Play your match and upload the full-time screen.</span></li>
        <li class="ok">${ic('check')}<span>Your opponent has 15 minutes to dispute. Disputes are decided by an admin.</span></li>
        <li class="ok">${ic('check')}<span>Prizes are paid to your wallet automatically when the tournament ends.</span></li>
      </ul></div>`;
}

function table(t, rows, uid) {
  if (!rows?.length) return html`<div class="card flat">${empty('list', 'No players yet', 'The table appears once players join.')}</div>`;
  const paid = tournamentPrizes(t).list.filter((p) => p > 0).length;
  return html`<div class="card" style="padding:6px 4px;overflow-x:auto"><table class="table">
    <thead><tr><th>#</th><th>Player</th><th>P</th><th>W</th><th>D</th><th>L</th><th>GD</th><th>Pts</th></tr></thead>
    <tbody>${rows.map((r, i) => html`<tr class="${r.userId === uid ? 'me' : ''} ${i < paid ? 'prize' : ''}">
      <td class="pos">${i + 1}</td><td><div class="row" style="gap:8px">${avatar(r.username, r.avatarUrl, "sm")}<div style="min-width:0"><div class="row" style="gap:6px"><span class="ellipsis" style="max-width:120px">${r.userId === uid ? "You" : r.username}</span>${tierBadge(r.tier)}</div><div class="tiny faint">${r.gamertag}</div></div></div></td>
      <td>${r.played}</td><td>${r.wins}</td><td>${r.draws}</td><td>${r.losses}</td><td>${r.goalsFor - r.goalsAgainst}</td><td class="pts">${r.points}</td></tr>`)}</tbody>
  </table></div><p class="tiny faint" style="margin-top:8px">Green positions win prizes.</p>`;
}

function bracket(t, matches, uid) {
  if (!matches.length) return html`<div class="card flat">${empty('trophy', 'Bracket not drawn yet', 'Players are paired randomly when the tournament fills up.')}</div>`;
  const rounds = [...new Set(matches.map((m) => m.round))].sort((a, b) => a - b);
  const name = (id, n) => (id === uid ? 'You' : id ? n : 'Bye');
  const roundTitle = (r) => {
    const n = matches.filter((m) => m.round === r).length;
    return n === 1 ? 'Final' : n === 2 ? 'Semi-finals' : n <= 4 ? 'Quarter-finals' : 'Round ' + r;
  };
  return html`<div class="bracket">${rounds.map((r) => html`<div class="round"><div class="round-title">${roundTitle(r)}</div>
    ${matches.filter((m) => m.round === r).map((m) => html`<a class="tie" href="/match/${m.id}" style="color:inherit">
      <div class="${m.winnerId && m.winnerId === m.homeId ? 'won' : ''}"><span class="ellipsis">${name(m.homeId, m.homeName)}</span><b class="num">${m.scoreHome ?? ''}</b></div>
      <div class="${m.winnerId && m.winnerId === m.awayId ? 'won' : ''}"><span class="ellipsis">${name(m.awayId, m.awayName)}</span><b class="num">${m.scoreAway ?? ''}</b></div>
    </a>`)}</div>`)}</div>`;
}
