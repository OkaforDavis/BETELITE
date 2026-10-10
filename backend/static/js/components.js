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
  const prize = m.kind === 'p2p' && m.pool > 0 ? money(Math.floor((m.pool * 80) / 100), m.currency) : '';
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

// Tournament formats: label, icon, badge style.
export const FORMATS = {
  knockout: ['Knockout', 'trophy', ''],
  league: ['League', 'list', 'info'],
  swiss: ['Points', 'grid', 'info'],
};
export const formatBadge = (t) => {
  const [label, icon, cls] = FORMATS[t.format] || FORMATS.knockout;
  return html`<span class="badge ${cls}">${ic(icon)}${label}</span>`;
};

// Prize amounts (1st place first) in the currency shown to this player.
// Fixed-prize tournaments are paid in each winner's own currency.
export function tournamentPrizes(t) {
  const cur = store.get().profile?.currency || t.currency;
  if (t.fixedPrizes) {
    const c = t.fixedPrizes[cur] ? cur : Object.keys(t.fixedPrizes)[0];
    return { list: t.fixedPrizes[c] || [], cur: c };
  }
  return { list: (t.prizes || []).filter((p) => p > 0), cur: t.currency };
}

// Membership badge next to a player's name.
export const TIER_NAMES = { pro: 'Pro', pro_plus: 'Pro Plus', elite: 'Elite' };
export const tierBadge = (tier) => (TIER_NAMES[tier] ? html`<span class="badge tier-${tier}">${ic(tier === 'elite' ? 'crown' : 'sparkles')}${TIER_NAMES[tier]}</span>` : '');

export function tournamentCard(t) {
  const pct = Math.round((t.playerCount / t.maxPlayers) * 100);
  const prizes = tournamentPrizes(t);
  const top = prizes.list[0] || 0;
  const early = t.status === 'open' && t.membersOnlyUntil && new Date(t.membersOnlyUntil) > new Date();
  const status = { open: ['Registering', 'brand'], active: ['In progress', 'live info'], finished: ['Finished', ''], cancelled: ['Cancelled', 'danger'] }[t.status] || [t.status, ''];
  return html`
  <a class="card tap" href="/tournaments/${t.id}" style="display:block;color:inherit">
    <div class="row between">
      <span class="row" style="gap:6px">${formatBadge(t)}${early ? html`<span class="badge tier-pro">${ic('sparkles')}Members first</span>` : ''}</span>
      <span class="badge ${status[1]}">${status[0]}</span>
    </div>
    <div class="h3" style="margin-top:12px">${t.name}</div>
    <div class="small muted">${t.gameName}${t.joined ? html` · <span class="brand-text">You're in</span>` : ''}</div>
    <div class="grid-3" style="margin-top:14px">
      <div><div class="tiny faint">ENTRY</div><div class="h3 num">${t.entryFee ? money(t.entryFee, t.currency) : '1 entry'}</div></div>
      <div><div class="tiny faint">1ST PRIZE</div><div class="h3 num brand-text">${top ? money(top, prizes.cur) : '—'}</div></div>
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

// Match screenshot evidence (players + admins). Images need the auth token,
// so they are fetched as blobs and shown via object URLs.
export async function loadEvidence(matchId) {
  const { api } = await import('./api.js');
  const { screenshots } = await api('GET', `/matches/${encodeURIComponent(matchId)}/screenshots`);
  return Promise.all(screenshots.map(async (s) => {
    const res = await api('GET', `/matches/${encodeURIComponent(matchId)}/screenshots/${s.id}`, undefined, { raw: true });
    return { ...s, url: res.ok ? URL.createObjectURL(await res.blob()) : '' };
  }));
}

export function evidenceGallery(list) {
  if (!list.length) return html`<p class="small muted">No screenshots for this match.</p>`;
  return html`<div class="evidence">${list.map((s) => html`
    <button class="evidence-item" data-full="${s.url}" aria-label="Open screenshot">
      ${s.url ? html`<img src="${s.url}" alt="Match screenshot">` : ''}
      <span class="tiny"><b>${s.kind === 'dispute' ? 'Dispute proof' : 'Result'}</b> · ${s.uploader} · ${new Date(s.createdAt).toLocaleString()}</span>
    </button>`)}</div>
    <p class="tiny faint" style="margin-top:8px">Screenshots are deleted 30 days after the match is final.</p>`;
}

// Tap a screenshot to view it full size.
export function bindEvidence(root) {
  root.querySelectorAll('[data-full]').forEach((b) => (b.onclick = async () => {
    const { sheet, sheetHead } = await import('./ui.js');
    sheet(String(html`${sheetHead('Screenshot')}<img src="${b.dataset.full}" alt="Match screenshot" style="width:100%;border-radius:12px">`), { label: 'Screenshot' });
  }));
}

// Payment fees passed on to the player (mirrors services/fees.go).
// rule = { percent, flat, flatFrom, cap } from the API; amounts in minor units.
export function gatewayFee(gross, rule) {
  if (!rule) return 0;
  let fee = Math.ceil((gross * Math.round(rule.percent * 100)) / 10000);
  if (rule.flat && gross >= rule.flatFrom) fee += rule.flat;
  if (rule.cap && fee > rule.cap) fee = rule.cap;
  return fee;
}
export function grossUp(net, rule) {
  if (net <= 0) return 0;
  let gross = net;
  for (let i = 0; i < 64 && gross - gatewayFee(gross, rule) < net; i++) gross += Math.max(net - (gross - gatewayFee(gross, rule)), 1);
  return gross;
}
export function transferFee(amount, cur, bankType) {
  if (cur === 'GHS') return bankType === 'mobile_money' ? 100 : 800;
  return amount <= 500000 ? 1000 : amount <= 5000000 ? 2500 : 5000;
}
