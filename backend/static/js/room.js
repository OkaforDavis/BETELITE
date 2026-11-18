// "Start the match" card: how to meet in the game, and the room code one
// player posts for the other (or that CrestArena makes up, for DLS).
import { get, post } from './api.js';
import { gameInfo } from './store.js';
import { html, ic, busy, toast, copyText, skeleton } from './ui.js';

const cache = new Map(); // matchId → last room, so redraws don't flicker
const domain = (link) => { try { return new URL(link).hostname.replace(/^www\./, ''); } catch { return ''; } };

export const roomSlot = (m) => html`<section class="section" id="room" data-room="${m.id}">${cache.has(m.id) ? '' : skeleton(160)}</section>`;

// Fills the #room slot. opp = { name, tag }.
export async function mountRoom(view, m, uid, opp) {
  const el = view.querySelector('#room');
  if (!el) return;
  if (cache.has(m.id)) render(el, m, uid, opp, cache.get(m.id));
  try {
    const { room } = await get(`/matches/${m.id}/room`);
    cache.set(m.id, room);
    if (el.isConnected) render(el, m, uid, opp, room);
  } catch (e) {
    if (!cache.has(m.id)) el.innerHTML = String(html`<p class="small muted">${e.message}</p>`);
  }
}

function render(el, m, uid, opp, room) {
  const g = gameInfo(m.game);
  const spec = g.room || {};
  const shared = room.mode === 'shared';
  const iHost = room.hostId === uid;
  const undecided = !shared && !room.hostId;
  const steps = shared || iHost || undecided ? spec.hostSteps : spec.joinSteps;
  const who = shared ? 'Both of you type the same code'
    : iHost ? 'You host this match'
    : room.hostId ? `${opp.name} hosts this match`
    : 'Whoever has a Custom Room Card hosts';

  el.innerHTML = String(html`<div class="card" id="room-card">
    <div class="row between" style="gap:8px"><div class="h2">Start the match</div><span class="badge">${g.short || m.game}</span></div>
    <p class="small muted" style="margin-top:4px">${ic(shared ? 'users' : iHost ? 'crown' : 'gamepad')} ${who}</p>
    ${steps?.length ? html`<ol class="howto">${steps.map((s) => html`<li>${s}</li>`)}</ol>` : ''}

    ${room.code ? html`<div class="room-box" style="margin-top:14px">
        <div class="tiny faint">${spec.codeLabel || 'Room code'}</div>
        <div class="row between" style="gap:8px"><span class="room-code" id="room-code">${room.code}</span>
          <button class="btn secondary sm" data-copy-room="${room.code}">${ic('copy')} Copy</button></div>
        ${room.password ? html`<div class="row between" style="gap:8px;margin-top:10px"><span class="small">Password <b class="room-pass">${room.password}</b></span>
          <button class="btn ghost sm" data-copy-room="${room.password}">${ic('copy')} Copy</button></div>` : ''}
        ${room.link ? html`<a class="btn primary block" style="margin-top:12px" href="${room.link}" target="_blank" rel="noopener nofollow">${ic('external')} Open the invite link</a>
          <p class="tiny faint center" style="margin-top:6px">Opens ${domain(room.link)}. Only tap links from your opponent you trust.</p>` : ''}
      </div>`
    : !shared && !room.canPost ? html`<div class="notice" style="margin-top:14px">${ic('clock')}<div class="small"><b>Waiting for ${opp.name} to create the room.</b> You'll get a notification when the ${spec.codeLabel || 'room code'} is posted.</div></div>` : ''}

    ${room.canPost ? html`<form id="room-form" style="margin-top:14px" ${room.code ? 'hidden' : ''}>
        <label class="field" style="margin-top:0"><span class="label">${spec.codeLabel || 'Room code'}</span>
          <input class="input" name="code" required minlength="3" maxlength="24" value="${room.code || ''}" autocomplete="off" autocapitalize="characters" spellcheck="false" placeholder="Exactly as the game shows it"></label>
        ${spec.password ? html`<label class="field"><span class="label">Room password <span class="faint">(if you set one)</span></span>
          <input class="input" name="password" maxlength="24" value="${room.password || ''}" autocomplete="off" spellcheck="false"></label>` : ''}
        ${spec.link ? html`<label class="field"><span class="label">Invite link <span class="faint">(optional)</span></span>
          <input class="input" name="link" inputmode="url" maxlength="300" value="${room.link || ''}" autocomplete="off" spellcheck="false" placeholder="https://…"></label>` : ''}
        <button class="btn primary block" type="submit" style="margin-top:14px">${room.code ? 'Save changes' : `Post the ${spec.codeLabel || 'room code'}`}</button>
      </form>
      ${room.code ? html`<div class="row" style="gap:8px;margin-top:12px"><button class="btn ghost sm" id="room-edit">${ic('refresh')} Change</button>
        ${spec.host === 'either' ? html`<button class="btn ghost sm" id="room-clear">Stop hosting</button>` : ''}</div>` : ''}` : ''}

    ${spec.warning ? html`<div class="notice warn" style="margin-top:14px">${ic('alert')}<div class="small">${spec.warning}</div></div>` : ''}
    <div class="list" style="margin-top:14px">
      <div class="list-row"><div class="icon-tile">${ic('user')}</div><div class="grow"><div class="small muted">Opponent in ${g.short || 'the game'}</div><b>${opp.tag || opp.name}</b></div>
        ${opp.tag ? html`<button class="btn ghost sm" data-copy-room="${opp.tag}">${ic('copy')}</button>` : ''}</div>
      ${room.oppGameId ? html`<div class="list-row"><div class="icon-tile">${ic('hash')}</div><div class="grow"><div class="small muted">Their in-game ID</div><b>${room.oppGameId}</b></div>
        <button class="btn ghost sm" data-copy-room="${room.oppGameId}">${ic('copy')}</button></div>` : ''}
    </div>
  </div>`);

  el.querySelectorAll('[data-copy-room]').forEach((b) => (b.onclick = () => copyText(b.dataset.copyRoom, 'Copied')));
  const form = el.querySelector('#room-form');
  el.querySelector('#room-edit')?.addEventListener('click', () => { form.hidden = !form.hidden; });
  const save = (btn, body, msg) => busy(btn, async () => {
    const { room: next } = await post(`/matches/${m.id}/room`, body);
    cache.set(m.id, next);
    toast(msg);
    render(el, m, uid, opp, next);
  });
  if (form) {
    form.onsubmit = (e) => {
      e.preventDefault();
      const f = new FormData(form);
      save(form.querySelector('[type=submit]'), { code: f.get('code') || '', password: f.get('password') || '', link: f.get('link') || '' },
        `Posted. ${opp.name} has been told.`);
    };
  }
  el.querySelector('#room-clear')?.addEventListener('click', (e) => save(e.currentTarget, { code: '' }, 'You are no longer hosting'));
}
