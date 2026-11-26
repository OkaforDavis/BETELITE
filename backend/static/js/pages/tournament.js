import { get, post, api, upload } from '../api.js';
import { store, gameInfo, resultScreen, can } from '../store.js';
import { html, ic, avatar, money, skeleton, empty, busy, toast, confirmSheet, shareLink, sheet, sheetHead, copyText, timeLeft } from '../ui.js';
import { matchRow, hasGameProfile, formatBadge, tournamentPrizes, tierBadge } from '../components.js';
import { gameIdSheet } from '../gameid.js';
import { needsConsent, consentGate } from '../consent.js';
import { navigate } from '../router.js';
import { tipButton, bindTipButtons } from '../tips.js';
import { sponsorCard, bindSponsor } from '../sponsors.js';
import { refreshProfile, refreshCurrent } from '../app.js';
import { settingsBox } from '../roomsettings.js';
import { shrinkScreenshot } from './match.js';

const when = (iso) => new Date(iso).toLocaleString(undefined, { weekday: 'short', day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit' });

const ordinal = (n) => n + ({ 1: 'st', 2: 'nd', 3: 'rd' }[n] || 'th');

export default async function tournamentPage(view, { id }) {
  let tab = 'overview';
  let data;
  // The router already shows this page's skeleton.

  async function load() {
    try { data = await get('/tournaments/' + encodeURIComponent(id)); }
    catch (e) {
      view.innerHTML = String(html`<div class="page">${empty('alert', e.status === 404 ? 'Tournament not found' : "Couldn't load this tournament", e.message,
        html`<div class="row" style="gap:8px;justify-content:center"><a class="btn secondary" href="/tournaments">${ic('chevronLeft')} Tournaments</a>${e.status === 404 ? '' : html`<button class="btn primary" data-retry>${ic('refresh')} Try again</button>`}</div>`)}</div>`);
      view.querySelector('[data-retry]')?.addEventListener('click', load);
      return;
    }
    draw();
  }

  function draw() {
    const { tournament: t, standings, matches, sponsor } = data;
    const uid = store.get().profile.id;
    const myMatch = matches.find((m) => (m.homeId === uid || m.awayId === uid) && ['ready', 'submitted', 'disputed', 'review'].includes(m.status));
    const pct = Math.round((t.playerCount / t.maxPlayers) * 100);
    const ffa = t.format === 'ffa';
    const second = t.format === 'knockout' ? 'bracket' : 'table';
    const startsAt = ffa && t.startsAt ? new Date(t.startsAt) : null;
    const prizes = tournamentPrizes(t);
    const membersUntil = t.membersOnlyUntil && new Date(t.membersOnlyUntil) > new Date() ? new Date(t.membersOnlyUntil) : null;
    const roundLabel = t.totalRounds ? `Round ${t.currentRound} of ${t.totalRounds}` : `Round ${t.currentRound}`;
    const isHost = !!t.hostId && t.hostId === uid;
    const canEdit = (isHost || can('cups')) && ['open', 'active'].includes(t.status);
    const fillBy = t.fillDeadline && t.status === 'open' ? new Date(t.fillDeadline) : null;

    view.innerHTML = String(html`<div class="page">
      <a href="/tournaments" class="row small muted" style="gap:4px;margin-bottom:12px">${ic('chevronLeft')}Tournaments</a>
      <div class="card pad-lg">
        ${t.coverUrl ? html`<div class="cup-cover"><img src="${t.coverUrl}" alt="${t.name}"></div>` : ''}
        <div class="row" style="gap:8px">
          ${formatBadge(t)}
          <span class="badge">${t.gameName}</span>
          ${t.hostId ? html`<span class="badge info">${ic('users')}Hosted by ${isHost ? 'you' : t.hostName}</span>` : ''}
          ${t.membersOnly ? html`<span class="badge tier-pro">${ic('sparkles')}Members only</span>` : ''}
          ${t.status === 'active' ? html`<span class="badge live info">${ffa ? (t.room ? 'Room open' : 'Starting') : roundLabel}</span>` : ''}
          ${t.status === 'finished' ? html`<span class="badge">Finished</span>` : ''}
        </div>
        <div class="row between" style="margin-top:12px;gap:8px;align-items:flex-start"><div class="h1">${t.name}</div>
          ${canEdit ? html`<button class="btn ghost sm" id="edit-cup">${ic('edit')} Edit</button>` : ''}</div>
        ${t.description ? html`<p class="small muted" style="margin-top:8px;white-space:pre-line">${t.description}</p>` : ''}
        ${startsAt && t.status !== 'finished' ? html`<div class="notice ${startsAt > new Date() ? '' : 'warn'}" style="margin-top:12px">${ic('clock')}<div class="small"><b>${when(t.startsAt)}</b>${startsAt > new Date()
          ? html` · starts in <b class="num" data-countdown="${t.startsAt}">${timeLeft(t.startsAt)}</b>` : t.room ? ' · the room is open' : ' · waiting for the host to post the room'}</div></div>` : ''}
        ${t.settings ? html`<div style="margin-top:12px">${settingsBox(t.game, t.settings, { title: ffa ? 'Room settings' : 'Match settings' })}</div>` : ''}
        <div class="grid-3" style="margin-top:18px">
          <div class="stat"><div class="v num">${t.entryFee ? money(t.entryFee, t.currency) : t.membersOnly ? 'Members' : t.hostId ? 'Free' : '1 entry'}</div><div class="k">Entry</div></div>
          <div class="stat"><div class="v num brand-text">${money(prizes.list[0] || 0, prizes.cur)}</div><div class="k">1st prize</div></div>
          <div class="stat"><div class="v num">${t.playerCount}/${t.maxPlayers}</div><div class="k">Players</div></div>
        </div>
        ${t.status === 'open' ? html`
          <div class="progress" style="margin-top:16px"><span style="width:${pct}%"></span></div>
          <div class="small muted" style="margin-top:8px">${t.maxPlayers - t.playerCount} spots left${t.freeSeats != null ? html` · free spots ${Math.max(t.freeSeats - t.freeTaken, 0)} of ${t.freeSeats}` : ''} · ${ffa ? 'one room, everyone plays at once' : 'starts automatically when full'}${fillBy ? html` · fills by ${fillBy.toLocaleDateString(undefined, { day: 'numeric', month: 'short' })} or it's cancelled` : ''}</div>
          ${isHost ? html`<div class="notice" style="margin-top:12px">${ic('info')}<div class="small"><b>You're hosting.</b> Prize pool ${money(t.hostPool, t.currency)}${t.hostFee ? ` + ${money(t.hostFee, t.currency)} fee` : ' · no fee (Elite)'}. ${t.coverUrl ? '' : 'Tap Edit to add a cover picture. '}Share the link to fill it up. New players who join and get their first membership earn you a commission.</div></div>` : ''}
          ${membersUntil && !t.joined ? html`<div class="notice brand" style="margin-top:12px">${ic('sparkles')}<div class="small"><b>${t.earlyTier === 'elite' ? 'Elite' : 'Members'}' early sign-up</b> until ${membersUntil.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}. Then it opens to everyone.</div></div>` : ''}
          <div style="margin-top:16px">${t.joined
            ? html`<div class="notice brand">${ic('checkCircle')}<div><b>You're registered</b><div class="small">${ffa ? "At start time the host posts the Room ID here and you'll get a loud alert." : "We'll notify you when it starts."}</div></div></div>
                <button class="btn ghost block" id="leave" style="margin-top:8px">Leave tournament${t.entryFee ? ' (refund)' : ''}</button>`
            : html`<button class="btn primary block" id="join">${ic('trophy')} Join${t.entryFee ? ' for ' + money(t.entryFee, t.currency) : t.membersOnly ? ' (members)' : t.hostId ? ' free' : ' (uses 1 entry)'}</button>
                ${t.entryFee || t.hostId ? '' : html`<a class="small muted row" href="/membership" style="gap:4px;margin-top:10px;justify-content:center">${ic('sparkles')}Entries and plans</a>`}`}
            <button class="btn secondary block" id="share" style="margin-top:8px">${ic('share')} ${isHost ? 'Share your cup' : 'Share with friends'}</button>
            ${isHost ? html`<button class="btn ghost block" id="cancel-cup" style="margin-top:8px">Cancel my cup (full refund)</button>` : ''}</div>` : ''}
      </div>

      ${ffa ? ffaSection(t) : ''}

      ${sponsorCard(sponsor, { isHost: isHost || can('cups'), open: ['open', 'active'].includes(t.status) })}

      ${t.hostId && !isHost ? html`<div class="card row" style="margin-top:12px;gap:12px"><div class="icon-tile">${ic('coins')}</div>
        <div class="grow"><div class="h3">Enjoying ${t.hostName}'s cup?</div><div class="small muted">Send a free tip. The host gets all of it.</div></div>
        ${tipButton({ to: t.hostId, name: t.hostName, context: 'tournament', refId: t.id, role: 'host', label: 'Tip host' })}</div>` : ''}

      ${myMatch ? html`<a class="notice brand" href="/match/${myMatch.id}" style="margin-top:16px;color:inherit">${ic('gamepad')}<div class="grow"><b>Your Round ${myMatch.round} match is ready</b>
        <div class="small">vs ${myMatch.homeId === uid ? myMatch.awayName : myMatch.homeName} · tap to play & upload the result</div></div>${ic('chevronRight')}</a>` : ''}

      <div class="seg" style="margin-top:20px" role="tablist">
        <button role="tab" data-tab="overview">Overview</button>
        <button role="tab" data-tab="${second}">${ffa ? 'Players' : second === 'table' ? 'Table' : 'Bracket'}</button>
        ${ffa ? '' : html`<button role="tab" data-tab="matches">Matches</button>`}
      </div>
      <div id="tab-body" style="margin-top:16px"></div>
    </div>`);

    view.querySelectorAll('[data-tab]').forEach((b) => (b.onclick = () => { tab = b.dataset.tab; drawTab(); }));
    view.querySelector('#join')?.addEventListener('click', (e) => join(t, e.currentTarget));
    view.querySelector('#share')?.addEventListener('click', () => {
      const top = prizes.list[0];
      shareLink({
        title: t.name,
        text: `Join "${t.name}" on CrestArena: ${t.gameName}, ${t.maxPlayers} players${top ? `, ${money(top, prizes.cur)} for 1st` : ''}. ${t.membersOnly ? 'For members.' : 'Free to join.'}`,
        url: `${location.origin}/tournaments/${t.id}`,
      });
    });
    view.querySelector('#edit-cup')?.addEventListener('click', () => editCup(t, load));
    view.querySelector('#cancel-cup')?.addEventListener('click', async (e) => {
      if (!(await confirmSheet({ title: 'Cancel your cup?', message: 'Everyone who joined is told, and the prize pool and hosting fee go back to your wallet.', confirm: 'Cancel cup', danger: true }))) return;
      busy(e.currentTarget, async () => { await post(`/tournaments/${t.id}/cancel`); toast('Cup cancelled and refunded'); refreshProfile(); navigate('/tournaments'); });
    });
    view.querySelector('#leave')?.addEventListener('click', async (e) => {
      if (!(await confirmSheet({ title: 'Leave tournament?', message: t.entryFee ? 'Your entry fee will be refunded to your wallet.' : 'You can join again while spots are open.', confirm: 'Leave', danger: true }))) return;
      busy(e.currentTarget, async () => { await post(`/tournaments/${t.id}/leave`); toast('You left the tournament'); refreshProfile(); load(); });
    });
    if (ffa) bindFFA(view, t, standings, load);
    bindTipButtons(view);
    bindSponsor(view, t.id, load);
    drawTab();

    function drawTab() {
      view.querySelectorAll('[data-tab]').forEach((b) => b.classList.toggle('active', b.dataset.tab === tab));
      const el = view.querySelector('#tab-body');
      if (tab === 'overview') el.innerHTML = String(overview(t));
      else if (tab === 'table') el.innerHTML = String(ffa ? ffaTable(t, standings, uid) : table(t, standings, uid));
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

// Edit a cup after posting. Prizes, entry, game and format can't change; round
// length, size and members-only only until the cup starts.
const SIZES = { knockout: [4, 8, 16, 32, 64], swiss: [8, 16, 32], league: [4, 5, 6, 7, 8] };
function editCup(t, reload) {
  const open = t.status === 'open';
  const hostCup = !!t.hostId;
  const sizes = hostCup && !t.entryFee ? (SIZES[t.format] || []).filter((n) => n > t.playerCount || n === t.maxPlayers) : [];
  sheet(String(html`${sheetHead('Edit cup', 'Prizes stay the same. Players who joined get a notification.')}
    <div class="field"><span class="label">Cover picture <span class="faint">(optional, wide pictures look best)</span></span>
      <div class="cup-cover cover-edit" id="cover-preview">${t.coverUrl ? html`<img src="${t.coverUrl}" alt="Cover">` : html`<span class="small">${ic('image')} No picture yet</span>`}</div>
      <input type="file" id="cover-file" accept="image/jpeg,image/png,image/webp" class="sr-only">
      <div class="row" style="gap:8px;margin-top:8px"><button type="button" class="btn secondary sm" id="cover-pick">${ic('image')} ${t.coverUrl ? 'Change picture' : 'Add picture'}</button>
        <button type="button" class="btn ghost sm" id="cover-remove" ${t.coverUrl ? '' : 'hidden'}>${ic('trash')} Remove</button></div>
      <span class="hint">Your cup logo, a game poster or a sponsor banner. No offensive pictures: they'll be removed.</span></div>
    <form id="edit-form">
      <label class="field"><span class="label">Name</span>
        <input class="input" name="name" required minlength="3" maxlength="40" value="${t.name}"></label>
      <label class="field"><span class="label">Description and rules <span class="faint">(optional)</span></span>
        <textarea class="input" name="description" rows="4" maxlength="600" placeholder="E.g. No legendary players. 6-minute halves. Join our WhatsApp group for updates.">${t.description || ''}</textarea></label>
      ${open ? html`
        <label class="field"><span class="label">Time per round</span>
          ${hostCup ? html`<select class="input" name="roundHours">${[12, 24, 48].map((h) => html`<option value="${h}" ${h === t.roundHours ? 'selected' : ''}>${h} hours</option>`)}</select>`
            : html`<input class="input num" name="roundHours" type="number" min="1" max="168" value="${t.roundHours}">`}</label>
        ${sizes.length > 1 ? html`<label class="field"><span class="label">Players</span>
          <select class="input" name="maxPlayers">${sizes.map((n) => html`<option value="${n}" ${n === t.maxPlayers ? 'selected' : ''}>${n} players</option>`)}</select>
          ${t.playerCount ? html`<span class="hint">${t.playerCount} joined already</span>` : ''}</label>` : ''}
        ${hostCup ? html`<label class="check" style="margin-top:14px"><input type="checkbox" name="membersOnly" ${t.membersOnly ? 'checked' : ''}><span>Members only (players with a paid plan)</span></label>` : ''}`
      : html`<p class="small muted" style="margin-top:12px">The cup has started, so only the name and description can change.</p>`}
      <div class="notice" style="margin-top:14px">${ic('lock')}<div class="small">The prize${t.entryFee ? ', entry fee' : ''}, game and format can't be changed after posting. To change those, cancel and post a new cup.</div></div>
      <div class="sheet-actions"><button class="btn primary block" type="submit">Save changes</button></div>
    </form>`), {
    label: 'Edit cup',
    onMount(root, close) {
      bindCover(root, t, reload);
      const f = root.querySelector('#edit-form');
      f.onsubmit = (e) => {
        e.preventDefault();
        const d = new FormData(f);
        busy(f.querySelector('[type=submit]'), async () => {
          const { changed } = await post(`/tournaments/${t.id}/edit`, {
            name: d.get('name'), description: d.get('description') || '',
            roundHours: Number(d.get('roundHours') || t.roundHours),
            maxPlayers: Number(d.get('maxPlayers') || t.maxPlayers),
            membersOnly: open && hostCup ? d.get('membersOnly') === 'on' : t.membersOnly,
          });
          close();
          toast(changed ? 'Cup updated' : 'Nothing changed');
          if (changed) reload();
        });
      };
    },
  });
}

// Shrinks a picture in the browser before upload (the server crops it to 16:9).
async function coverJpeg(file, maxSide = 1600) {
  const bitmap = await createImageBitmap(file).catch(() => null);
  if (!bitmap) throw new Error("We couldn't open that picture. Try a JPG or PNG.");
  const scale = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height));
  const canvas = Object.assign(document.createElement('canvas'), { width: Math.round(bitmap.width * scale), height: Math.round(bitmap.height * scale) });
  canvas.getContext('2d').drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close?.();
  return new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.9));
}

// The cover picture saves as soon as it's chosen.
function bindCover(root, t, reload) {
  const input = root.querySelector('#cover-file');
  const pick = root.querySelector('#cover-pick');
  const remove = root.querySelector('#cover-remove');
  const preview = root.querySelector('#cover-preview');
  pick.onclick = () => input.click();
  input.onchange = () => {
    const file = input.files[0];
    input.value = '';
    if (!file) return;
    if (file.size > 25 * 1024 * 1024) return toast('That picture is too large', 'err');
    busy(pick, async () => {
      const blob = await coverJpeg(file);
      const fd = new FormData();
      fd.append('image', blob, 'cover.jpg');
      const { coverUrl } = await api('PUT', `/tournaments/${t.id}/cover`, fd, { form: true });
      t.coverUrl = coverUrl;
      preview.innerHTML = String(html`<img src="${coverUrl}" alt="Cover">`);
      remove.hidden = false;
      toast('Cover picture saved');
      reload();
    });
  };
  remove.onclick = () => busy(remove, async () => {
    await api('DELETE', `/tournaments/${t.id}/cover`);
    t.coverUrl = '';
    preview.innerHTML = String(html`<span class="small">${ic('image')} No picture yet</span>`);
    remove.hidden = true;
    toast('Cover picture removed');
    reload();
  });
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
    ffa: html`<li class="ok">${ic('check')}<span>Everyone plays at the same time in <b>one game room</b> (up to ${t.maxPlayers} players).</span></li>
              <li class="ok">${ic('check')}<span>At start time the host posts the <b>Room ID and password</b> here. You get a loud alert: join straight away.</span></li>
              <li class="ok">${ic('check')}<span>${t.settings ? `First to ${t.settings.scoreLimit} wins. If time runs out, the highest score wins.` : 'The highest score wins.'} A tie for 1st is settled by a 1v1 tiebreak.</span></li>
              <li class="ok">${ic('check')}<span>The host uploads the final scoreboard, which every player can see. Problems? Report them from the cup page within 24 hours.</span></li>`,
    knockout: gameInfo(t.game).football === false
      ? html`<li class="ok">${ic('check')}<span>Lose once and you're out. Every match needs a winner: play a deciding round if it's level.</span></li>`
      : html`<li class="ok">${ic('check')}<span>Lose once and you're out. Draws go to penalties.</span></li>
                   <li class="ok">${ic('check')}<span>Upload the screen that shows the penalty shoot-out if there is one.</span></li>`,
  };
  return html`
    <div class="card"><div class="h3">Prizes</div>
      ${places.length ? html`<div class="list" style="margin-top:12px">${places.map(([pos, p, pct]) => html`
        <div class="list-row"><div class="icon-tile ${pos === 1 ? 'brand' : ''}">${ic(pos === 1 ? 'crown' : 'trophy')}</div>
        <div class="grow">${ordinal(pos)} place</div><div style="text-align:right"><b class="num">${money(p, prizes.cur)}</b>
        ${other ? html`<div class="tiny faint">${money(t.fixedPrizes[other][pos - 1], other)}</div>` : pct ? html`<div class="tiny faint">${pct}% of pool</div>` : ''}</div></div>`)}</div>`
      : html`<p class="muted small" style="margin-top:8px">No cash prizes for this tournament.</p>`}
      <p class="tiny faint" style="margin-top:10px">${t.hostId
        ? `Prize pool put up by the host (${t.hostName}) and held by CrestArena until the cup ends. Winners in the other currency get the same value at the market rate.`
        : fixed
        ? 'Fixed prizes, the same for every player. Winners are paid in their own currency.'
        : `Prize amounts shown for a full tournament${t.entryFee ? ', as a share of total entry fees' : ''}.`}</p>
    </div>
    <div class="card" style="margin-top:12px"><div class="h3">How it works</div>
      <ul class="checklist" style="margin-top:12px">
        ${rules[t.format] || rules.knockout}
        ${t.format === 'ffa' ? '' : html`<li class="ok">${ic('check')}<span>Each round has ${t.roundHours} hours. Play your match and upload the ${resultScreen(t.game)}.</span></li>
        <li class="ok">${ic('check')}<span>Your opponent has 15 minutes to dispute. Disputes are decided by an admin.</span></li>`}
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
    const n = matches.filter((m) => m.round === r && !m.thirdPlace).length;
    return n === 1 ? 'Final' : n === 2 ? 'Semi-finals' : n <= 4 ? 'Quarter-finals' : 'Round ' + r;
  };
  // The 3rd-place match sits under the final, with its own label.
  return html`<div class="bracket">${rounds.map((r) => html`<div class="round"><div class="round-title">${roundTitle(r)}</div>
    ${matches.filter((m) => m.round === r).sort((a, b) => a.thirdPlace - b.thirdPlace).map((m) => html`${m.thirdPlace ? html`<div class="round-title" style="margin-top:12px">3rd place</div>` : ''}<a class="tie" href="/match/${m.id}" style="color:inherit">
      <div class="${m.winnerId && m.winnerId === m.homeId ? 'won' : ''}"><span class="ellipsis">${name(m.homeId, m.homeName)}</span><b class="num">${m.scoreHome ?? ''}</b></div>
      <div class="${m.winnerId && m.winnerId === m.awayId ? 'won' : ''}"><span class="ellipsis">${name(m.awayId, m.awayName)}</span><b class="num">${m.scoreAway ?? ''}</b></div>
    </a>`)}</div>`)}</div>`;
}

// ── Free-for-All cups: one game room for everyone ─────────────────────
const JOIN_STEPS = ['Open the game → Multiplayer → the ≡ menu (top right) → Private.', 'Tap the home icon, search the Room ID and enter the password.', 'Wait in the room. The host starts when everyone is in.'];
const HOST_STEPS = ['Open the game → Multiplayer → the ≡ menu (top right) → Private.', 'Set the mode, score limit, time limit and map exactly as in Room settings, and set a password.', 'Post the Room ID and password below: every player gets a loud alert. Don’t leave the room (the ID changes).'];

function ffaSection(t) {
  const live = ['open', 'active'].includes(t.status);
  const room = t.room;
  return html`
    ${room && live ? html`<div class="card highlight" style="margin-top:12px">
      <div class="row between" style="gap:8px"><div class="h2 row" style="gap:8px">${ic('gamepad')} Join the room</div>${room.postedAt ? html`<span class="tiny faint">posted ${new Date(room.postedAt).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })}</span>` : ''}</div>
      <div class="room-box" style="margin-top:12px">
        <div class="tiny faint">Room ID</div>
        <div class="row between" style="gap:8px"><span class="room-code">${room.code}</span><button class="btn secondary sm" data-copy-room="${room.code}">${ic('copy')} Copy</button></div>
        ${room.password ? html`<div class="row between" style="gap:8px;margin-top:10px"><span class="small">Password <b class="room-pass">${room.password}</b></span><button class="btn ghost sm" data-copy-room="${room.password}">${ic('copy')} Copy</button></div>` : ''}
        ${room.note ? html`<p class="small" style="margin-top:10px">${ic('info')} ${room.note}</p>` : ''}
      </div>
      ${t.canManage ? '' : html`<ol class="howto">${JOIN_STEPS.map((s) => html`<li>${s}</li>`)}</ol>`}
    </div>` : ''}

    ${t.canManage && live ? html`<div class="card" style="margin-top:12px">
      <div class="h2 row" style="gap:8px">${ic('crown')} You run this room</div>
      ${room ? '' : html`<ol class="howto">${HOST_STEPS.map((s) => html`<li>${s}</li>`)}</ol>`}
      ${room ? html`<button class="btn secondary block" id="room-edit" style="margin-top:12px">${ic('edit')} Change the Room ID</button>` : ''}
      <form id="ffa-room" style="margin-top:12px" ${room ? 'hidden' : ''}>
        <div class="grid-2"><label class="field" style="margin-top:0"><span class="label">Room ID</span><input class="input" name="code" required minlength="3" maxlength="24" autocomplete="off" spellcheck="false" value="${room?.code || ''}" placeholder="e.g. 24827"></label>
          <label class="field" style="margin-top:0"><span class="label">Password</span><input class="input" name="password" maxlength="24" autocomplete="off" spellcheck="false" value="${room?.password || ''}"></label></div>
        <label class="field"><span class="label">Note for players <span class="faint">(optional)</span></span><input class="input" name="note" maxlength="120" value="${room?.note || ''}" placeholder="e.g. Map: Crash. Starting in 5 minutes."></label>
        ${t.status === 'open' && t.playerCount < 2 ? html`<p class="hint">At least 2 players must join first.</p>` : t.status === 'open' ? html`<p class="hint">Posting the room closes sign-up (${t.playerCount} of ${t.maxPlayers} joined) and alerts every player.</p>` : ''}
        <button class="btn primary block" type="submit" style="margin-top:12px">${ic('bellRing')} ${room ? 'Update room & alert players' : 'Post room & alert players'}</button>
      </form>
      ${t.status === 'active' ? html`<button class="btn primary block" id="ffa-result" style="margin-top:10px">${ic('trophy')} Enter the result</button>
        <p class="hint center">After the game: upload the final scoreboard and each player's score. The winner is paid straight away.</p>` : ''}
    </div>` : ''}

    ${!room && t.joined && live && !t.canManage ? html`<div class="notice" style="margin-top:12px">${ic('bell')}<div class="small"><b>The Room ID will appear here.</b> Keep notifications on: you'll get a loud alert the moment the host posts it.</div></div>` : ''}
    ${t.status === 'finished' && t.hasResult && (t.joined || t.canManage) ? html`<button class="btn secondary block" id="ffa-board" style="margin-top:12px">${ic('image')} See the final scoreboard</button>` : ''}
    ${t.joined && t.status !== 'open' ? html`<a class="row small muted" href="/support?category=tournament&ref=${t.id}" style="gap:6px;justify-content:center;margin-top:12px">${ic('flag')} Report a problem with this cup</a>` : ''}`;
}

function ffaTable(t, rows, uid) {
  if (!rows?.length) return html`<div class="card flat">${empty('users', 'No players yet', 'Players appear here as they join.')}</div>`;
  const label = t.settings?.mode ? (gameInfo(t.game).settings?.modes.find((m) => m.id === t.settings.mode)?.scoreLabel || 'score') : 'score';
  const done = t.status === 'finished';
  const list = done ? [...rows].sort((a, b) => (a.position ?? 99) - (b.position ?? 99)) : rows;
  return html`<div class="list">${list.map((r, i) => html`<div class="list-row ${r.userId === uid ? 'me' : ''}">
    <div class="pos num" style="width:22px;text-align:center;font-weight:700">${done ? (r.position === 1 ? ic('crown') : r.position) : i + 1}</div>
    ${avatar(r.username, r.avatarUrl, 'sm')}
    <div class="grow" style="min-width:0"><div class="row" style="gap:6px"><span class="ellipsis">${r.userId === uid ? 'You' : r.username}</span>${tierBadge(r.tier)}</div><div class="tiny faint ellipsis">${r.gamertag}</div></div>
    ${r.score != null ? html`<b class="num ${r.position === 1 ? 'brand-text' : ''}">${r.score} <span class="tiny faint">${label}</span></b>` : ''}</div>`)}</div>
    ${done ? '' : html`<p class="tiny faint" style="margin-top:8px">Scores appear when the host enters the result.</p>`}`;
}

function bindFFA(view, t, standings, reload) {
  view.querySelectorAll('[data-copy-room]').forEach((b) => b.addEventListener('click', () => copyText(b.dataset.copyRoom, 'Copied')));
  const form = view.querySelector('#ffa-room');
  view.querySelector('#room-edit')?.addEventListener('click', (e) => { form.hidden = false; e.currentTarget.hidden = true; });
  if (form) form.onsubmit = (e) => {
    e.preventDefault();
    const d = new FormData(form);
    busy(form.querySelector('[type=submit]'), async () => {
      await post(`/tournaments/${t.id}/room`, { code: d.get('code'), password: d.get('password'), note: d.get('note') });
      toast('Room posted. Every player got an alert.', 'ok');
      reload();
    });
  };
  view.querySelector('#ffa-result')?.addEventListener('click', () => resultSheet(t, standings, reload));
  view.querySelector('#ffa-board')?.addEventListener('click', (e) => busy(e.currentTarget, async () => {
    const res = await api('GET', `/tournaments/${t.id}/result-image`, undefined, { raw: true });
    if (!res.ok) throw new Error("Couldn't load the scoreboard");
    const url = URL.createObjectURL(await res.blob());
    sheet(String(html`${sheetHead('Final scoreboard', t.name)}<img src="${url}" alt="Final scoreboard" style="width:100%;border-radius:12px;margin-top:8px">`), { label: 'Scoreboard' });
  }));
}

function resultSheet(t, players, reload) {
  const spec = gameInfo(t.game).settings;
  const label = spec?.modes.find((m) => m.id === t.settings?.mode)?.scoreLabel || 'score';
  const limit = t.settings?.scoreLimit || 999;
  sheet(String(html`${sheetHead('Enter the result', 'Players can see the scoreboard you upload')}
    <form id="ffa-res">
      <label class="dropzone" style="margin-top:4px"><input type="file" accept="image/png,image/jpeg,image/webp" name="image" class="sr-only" required>
        <span id="res-drop">${ic('upload')}<b>Final scoreboard screenshot</b><span class="small faint">Showing every player's name and ${label}</span></span></label>
      <div class="field"><span class="label">Each player's ${label}</span>
        <div class="list">${players.map((p) => html`<div class="list-row">${avatar(p.username, p.avatarUrl, 'sm')}
          <div class="grow" style="min-width:0"><div class="ellipsis">${p.username}</div><div class="tiny faint ellipsis">${p.gamertag}</div></div>
          <input class="input num" style="width:84px;text-align:center" type="number" inputmode="numeric" min="0" max="${limit}" value="0" data-score="${p.userId}" aria-label="${p.username} ${label}"></div>`)}</div></div>
      <label class="field"><span class="label">Winner</span><select class="select" name="winner">${players.map((p) => html`<option value="${p.userId}">${p.username}</option>`)}</select>
        <span class="hint">Picked automatically from the highest score. Only change it after a tiebreak.</span></label>
      <div class="notice warn" style="margin-top:12px">${ic('alert')}<div class="small">This pays the prize straight away and can't be undone. Check the scores against the screenshot.</div></div>
      <div class="sheet-actions"><button class="btn primary block" type="submit">${ic('trophy')} Submit & pay the winner</button></div>
    </form>`), {
    label: 'Enter the result',
    onMount(root, close) {
      const f = root.querySelector('#ffa-res');
      const inputs = [...root.querySelectorAll('[data-score]')];
      const auto = () => { const top = inputs.reduce((a, b) => (+b.value > +a.value ? b : a)); f.winner.value = top.dataset.score; };
      inputs.forEach((i) => i.addEventListener('input', auto));
      f.image.onchange = () => { const file = f.image.files[0]; if (file) root.querySelector('#res-drop').innerHTML = `<img src="${URL.createObjectURL(file)}" alt="Scoreboard">`; };
      f.onsubmit = (e) => {
        e.preventDefault();
        const scores = Object.fromEntries(inputs.map((i) => [i.dataset.score, Math.max(0, parseInt(i.value, 10) || 0)]));
        const winner = players.find((p) => p.userId === f.winner.value);
        busy(f.querySelector('[type=submit]'), async () => {
          if (!(await confirmSheet({ title: `${winner.username} wins?`, message: `${winner.username} gets the prize with ${scores[winner.userId]} ${label}. This can't be undone.`, confirm: 'Yes, pay the winner' }))) return;
          const fd = new FormData();
          fd.append('image', await shrinkScreenshot(f.image.files[0]), 'scoreboard.jpg');
          fd.append('result', JSON.stringify({ scores, winner: winner.userId }));
          await upload(`/tournaments/${t.id}/result`, fd);
          close();
          toast(`${winner.username} wins! The prize has been paid.`, 'ok');
          reload();
        });
      };
    },
  });
}
