// Reusable UI blocks shared by several pages.
import { html, ic, money, avatar, statusBadge, scoreLine, timeLeft } from './ui.js';
import { store, gameName } from './store.js';

const me = () => store.get().profile?.id;

// What the signed-in player should do next for a match, in plain words.
export function nextStep(m) {
  const uid = me();
  switch (m.status) {
    case 'ready':
      return { text: 'Play the match, then upload the final result screen', cta: 'Upload result', tone: 'brand' };
    case 'submitted':
      return m.submittedBy === uid
        ? { text: 'Waiting for your opponent to confirm', cta: 'View', tone: 'warn' }
        : { text: 'Check the result: confirm or dispute', cta: 'Review result', tone: 'warn' };
    case 'disputed':
      return { text: 'An admin is reviewing this match', cta: 'View', tone: 'danger' };
    case 'review':
      return { text: 'An admin is reviewing this match', cta: 'View', tone: 'info' };
    default:
      return null;
  }
}

export function matchHero(m) {
  const uid = me();
  const step = nextStep(m);
  const prize = m.kind === 'p2p' ? money(Math.floor((m.pool * 80) / 100), m.currency) : '';
  const deadline = m.status === 'submitted' ? m.disputeDeadline : m.status === 'ready' ? m.playDeadline : null;
  return html`
  <a class="card highlight tap" href="/match/${m.id}" style="display:block;color:inherit">
    <div class="row between">
      <span class="eyebrow">${m.kind === 'p2p' ? '1v1 match' : m.tournamentName + ' · Round ' + m.round} · ${gameName(m.game)}</span>
      ${statusBadge(m.status)}
    </div>
    <div class="versus" style="margin:18px 0 14px">
      <div class="side ${m.winnerId && m.winnerId === m.homeId ? 'winner' : ''}">${avatar(m.homeName, m.homeAvatar, m.homeId === uid ? 'me' : '')}
        <div class="name ellipsis">${m.homeId === uid ? 'You' : m.homeName}</div><div class="tag ellipsis">${m.homeTag}</div></div>
      <div class="mid">${scoreLine(m)}</div>
      <div class="side ${m.winnerId && m.winnerId === m.awayId ? 'winner' : ''}">${avatar(m.awayName, m.awayAvatar, m.awayId === uid ? 'me' : '')}
        <div class="name ellipsis">${m.awayId === uid ? 'You' : m.awayName}</div><div class="tag ellipsis">${m.awayTag}</div></div>
    </div>
    ${step ? html`<div class="notice ${step.tone}">${ic(step.tone === 'brand' ? 'gamepad' : 'clock')}
      <div class="grow"><b>${step.text}</b>${deadline ? html`<div class="small">Time left: <span class="num" data-countdown="${deadline}">${timeLeft(deadline)}</span></div>` : ''}</div></div>` : ''}
    ${prize ? html`<div class="row between small" style="margin-top:12px"><span class="muted">Stake ${money(m.stake, m.currency)} each</span><span>Prize <b class="brand-text">${prize}</b></span></div>` : ''}
  </a>`;
}

export function matchRow(m) {
  const uid = me();
  const name = (id, n) => (id === uid ? 'You' : n);
  const hs = m.scoreHome ?? '', as = m.scoreAway ?? '';
  return html`<a class="match-row" href="/match/${m.id}" style="color:inherit">
    <div class="teams">
      <div><span class="ellipsis ${m.winnerId === m.homeId ? 'brand-text' : ''}">${name(m.homeId, m.homeName)}</span><b>${hs}</b></div>
      <div><span class="ellipsis ${m.winnerId === m.awayId ? 'brand-text' : ''}">${name(m.awayId, m.awayName)}</span><b>${as}</b></div>
    </div>
    <div style="text-align:right">${statusBadge(m.status)}<div class="tiny faint" style="margin-top:6px">${gameName(m.game)}</div></div>
  </a>`;
}

export function tournamentCard(t) {
  const pct = Math.round((t.playerCount / t.maxPlayers) * 100);
  const top = t.prizes?.[0] || 0;
  const status = { open: ['Registering', 'brand'], active: ['In progress', 'live info'], finished: ['Finished', ''], cancelled: ['Cancelled', 'danger'] }[t.status] || [t.status, ''];
  return html`
  <a class="card tap" href="/tournaments/${t.id}" style="display:block;color:inherit">
    <div class="row between">
      <span class="badge ${t.format === 'league' ? 'info' : ''}">${ic(t.format === 'league' ? 'list' : 'trophy')}${t.format === 'league' ? 'League' : 'Knockout'}</span>
      <span class="badge ${status[1]}">${status[0]}</span>
    </div>
    <div class="h3" style="margin-top:12px">${t.name}</div>
    <div class="small muted">${t.gameName}${t.joined ? html` · <span class="brand-text">You're in</span>` : ''}</div>
    <div class="grid-3" style="margin-top:14px">
      <div><div class="tiny faint">ENTRY</div><div class="h3 num">${t.entryFee ? money(t.entryFee, t.currency) : 'Free'}</div></div>
      <div><div class="tiny faint">1ST PRIZE</div><div class="h3 num brand-text">${top ? money(top, t.currency) : '—'}</div></div>
      <div><div class="tiny faint">PLAYERS</div><div class="h3 num">${t.playerCount}/${t.maxPlayers}</div></div>
    </div>
    <div class="progress" style="margin-top:12px" aria-label="${pct}% full"><span style="width:${pct}%"></span></div>
  </a>`;
}

export function gameChips(selected, { all = false } = {}) {
  const games = store.get().games;
  return html`<div class="chips" role="radiogroup" aria-label="Game">
    ${all ? html`<button class="chip ${!selected ? 'active' : ''}" data-game="" role="radio" aria-checked="${!selected}">All games</button>` : ''}
    ${games.map((g) => html`<button class="chip ${selected === g.id ? 'active' : ''}" data-game="${g.id}" role="radio" aria-checked="${selected === g.id}">${ic('gamepad')}${g.short}</button>`)}
  </div>`;
}

export function hasGameProfile(game) {
  return store.get().profile?.gameProfiles?.some((g) => g.game === game);
}
