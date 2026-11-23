// Match chat card: the two players message each other, see if the opponent
// has the app open, and send an "I'm ready" alert (loud notification + sound).
import { get, post } from './api.js';
import { html, ic, busy, toastError, skeleton, timeAgo } from './ui.js';

const cache = new Map();  // matchId → last chat view, so redraws don't flicker
const drafts = new Map(); // matchId → unsent text
const ctxs = new Map();   // matchId → { m, uid, opp }
const QUICK = ["I'm ready, let's play", 'Give me 5 minutes', 'Room is ready, join now', "Can't play now. Later today?", 'GG, well played'];

export const chatSlot = (m) => html`<section class="section" id="chat" data-chat="${m.id}">${cache.has(m.id) ? '' : skeleton(200)}</section>`;

// Fills the #chat slot. opp = { name, tag } (for admins: the away player).
export async function mountChat(view, m, uid, opp) {
  const el = view.querySelector('#chat');
  if (!el) return;
  ctxs.set(m.id, { m, uid, opp });
  if (cache.has(m.id)) render(el, m.id);
  await refresh(m.id);
  // Keep "online" up to date and catch messages missed while reconnecting.
  const timer = setInterval(() => { if (!el.isConnected) clearInterval(timer); else refresh(m.id); }, 30000);
}

async function refresh(id) {
  try {
    const { chat } = await get(`/matches/${id}/chat`);
    chat.pingUntil = Date.now() + (chat.pingWait || 0) * 1000;
    const old = cache.get(id);
    const changed = !old || old.messages.length !== chat.messages.length || old.oppOnline !== chat.oppOnline || old.canSend !== chat.canSend || old.canPing !== chat.canPing;
    cache.set(id, chat);
    const el = slot(id);
    if (el && (changed || !el.querySelector('.chat-card'))) render(el, id);
  } catch (e) {
    const el = slot(id);
    if (el && !cache.has(id)) el.innerHTML = String(html`<p class="small muted">${e.message}</p>`);
  }
}

const slot = (id) => { const el = document.querySelector(`#chat[data-chat="${id}"]`); return el?.isConnected ? el : null; };

function names(m) { return { [m.homeId]: m.homeName, [m.awayId]: m.awayName }; }

function messageRow(msg, uid, who) {
  const mine = msg.userId === uid;
  if (msg.kind === 'ping') {
    return html`<div class="msg-sys">${ic('bellRing')} ${mine ? 'You sent an alert: ready to play' : `${who[msg.userId] || 'Your opponent'} is ready to play`} · ${timeAgo(msg.createdAt)}</div>`;
  }
  return html`<div class="msg ${mine ? 'me' : ''}"><div class="bubble">${msg.body}</div>
    <div class="tiny faint">${mine ? '' : (who[msg.userId] || '') + ' · '}${timeAgo(msg.createdAt)}</div></div>`;
}

function listHTML(id) {
  const chat = cache.get(id); const { m, uid } = ctxs.get(id);
  if (!chat.messages.length) return html`<p class="small muted center" style="padding:18px 8px">No messages yet. Say hi and agree when to play.</p>`;
  const who = names(m);
  return html`${chat.messages.map((msg) => messageRow(msg, uid, who))}`;
}

function render(el, id) {
  const chat = cache.get(id); const c = ctxs.get(id);
  if (!chat || !c) return;
  const { m, uid, opp } = c;
  const player = uid === m.homeId || uid === m.awayId;
  el.innerHTML = String(html`<div class="card chat-card">
    <div class="row between" style="gap:8px"><div class="h2">${ic('chat')} Match chat</div>
      ${player ? html`<span class="small ${chat.oppOnline ? 'brand-text' : 'muted'} row" style="gap:6px"><span class="live-dot ${chat.oppOnline ? 'on' : ''}"></span>${chat.oppOnline ? `${opp.name} is online` : `${opp.name} is offline`}</span>` : html`<span class="badge">Admin view</span>`}</div>

    ${chat.canPing ? html`<button class="btn primary block ping-btn" id="ping" style="margin-top:14px">${ic('bellRing')} I'm ready, alert ${opp.name}</button>
      <p class="hint center">Sends a loud alert with the match sound to their phone.</p>` : ''}

    <div class="chat-list" id="chat-list" aria-live="polite">${listHTML(id)}</div>

    ${chat.canSend ? html`<div class="chips" style="margin-top:10px">${QUICK.map((q) => html`<button class="chip" type="button" data-quick="${q}">${q}</button>`)}</div>
      <form class="chat-form" id="chat-form" autocomplete="off">
        <input class="input" name="text" maxlength="300" placeholder="Message ${opp.name}…" value="${drafts.get(id) || ''}" enterkeyhint="send">
        <button class="btn primary" type="submit" aria-label="Send">${ic('send')}</button>
      </form>`
    : player ? html`<p class="small muted center" style="margin-top:10px">This chat is closed because the match has finished.</p>` : ''}
    ${player ? html`<p class="tiny faint" style="margin-top:10px">${ic('shield')} Stay safe: never share passwords or pay anyone outside CrestArena. Admins can read this chat if there's a dispute.</p>` : ''}
  </div>`);

  scrollDown(el);
  const form = el.querySelector('#chat-form');
  if (form) {
    const input = form.elements.text;
    input.addEventListener('input', () => drafts.set(id, input.value));
    form.onsubmit = (e) => {
      e.preventDefault();
      const text = input.value.trim();
      if (!text) return;
      send(id, text, form.querySelector('[type=submit]'), () => { input.value = ''; drafts.delete(id); input.focus(); });
    };
    el.querySelectorAll('[data-quick]').forEach((b) => b.addEventListener('click', () => send(id, b.dataset.quick, b)));
  }
  const ping = el.querySelector('#ping');
  if (ping) {
    ping.onclick = () => busy(ping, async () => {
      const { message } = await post(`/matches/${id}/ping`);
      add(id, message);
      chat.pingUntil = Date.now() + 60000;
      tickPing(ping, chat, opp);
    });
    tickPing(ping, chat, opp);
  }
}

// Shows "Alert again in 42s" while the cooldown runs.
function tickPing(btn, chat, opp) {
  const left = Math.ceil((chat.pingUntil - Date.now()) / 1000);
  if (left > 0) {
    btn.disabled = true;
    btn.innerHTML = String(html`${ic('check')} Alert sent · again in ${left}s`);
    setTimeout(() => btn.isConnected && tickPing(btn, chat, opp), 1000);
  } else {
    btn.disabled = false;
    btn.innerHTML = String(html`${ic('bellRing')} I'm ready, alert ${opp.name}`);
  }
}

async function send(id, text, btn, done) {
  btn.disabled = true;
  try {
    const { message } = await post(`/matches/${id}/chat`, { text });
    add(id, message);
    done?.();
  } catch (e) {
    toastError(e);
  } finally {
    btn.disabled = false;
  }
}

function scrollDown(el) {
  const list = el.querySelector('#chat-list');
  if (list) list.scrollTop = list.scrollHeight;
}

// Adds a message (from this device or live from the server) without redrawing
// the input, so a half-typed message is kept.
function add(id, msg) {
  const chat = cache.get(id);
  if (!chat || chat.messages.some((x) => x.id === msg.id)) return;
  chat.messages.push(msg);
  const el = slot(id);
  const list = el?.querySelector('#chat-list');
  if (list && ctxs.has(id)) { list.innerHTML = String(listHTML(id)); scrollDown(el); }
}

// Live messages from the server (app.js turns the socket event into this).
document.addEventListener('match_chat', (e) => { const d = e.detail; if (d?.matchId && d.message) add(d.matchId, d.message); });

// Is the chat for this match on screen right now?
export const chatVisible = (id) => !!slot(id) && document.visibilityState === 'visible';
