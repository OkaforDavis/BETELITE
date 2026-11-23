// WebSocket client: authenticates with the Firebase token, reconnects with
// backoff and fans events out to listeners.
import { API_BASE } from './api.js';

const listeners = new Map();
let ws = null;
let tokenFn = null;
let retry = 0;
let rooms = new Set();
let pingTimer = null;

export function on(event, fn) {
  if (!listeners.has(event)) listeners.set(event, new Set());
  listeners.get(event).add(fn);
  return () => listeners.get(event)?.delete(fn);
}
function emit(event, data) {
  listeners.get(event)?.forEach((fn) => { try { fn(data); } catch (e) { console.error(e); } });
}

export function send(event, data) {
  if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ event, data }));
}
export function joinRoom(room) { rooms.add(room); send('join_room', { room }); }
export function leaveRoom(room) { rooms.delete(room); send('leave_room', { room }); }

export function connect(getToken) {
  tokenFn = getToken;
  open();
}
export function disconnect() {
  tokenFn = null;
  clearInterval(pingTimer);
  ws?.close();
  ws = null;
}

let reconnectTimer = null;
let hiddenAt = 0;

function open() {
  if (!tokenFn) return;
  clearTimeout(reconnectTimer);
  if (ws && ws.readyState <= 1) return; // already connecting or connected
  const base = API_BASE || location.origin;
  const url = base.replace(/^http/, 'ws') + '/ws';
  const sock = new WebSocket(url);
  ws = sock;
  sock.onopen = async () => {
    retry = 0;
    emit('status', 'online');
    const token = await tokenFn();
    if (token) send('identify', { token });
    rooms.forEach((room) => send('join_room', { room }));
    clearInterval(pingTimer);
    pingTimer = setInterval(() => send('ping', {}), 25000);
  };
  sock.onmessage = (e) => {
    // The server may batch several JSON messages separated by newlines.
    for (const line of String(e.data).split('\n')) {
      if (!line.trim()) continue;
      try {
        const msg = JSON.parse(line);
        emit(msg.event, msg.data);
      } catch { /* ignore malformed */ }
    }
  };
  sock.onclose = () => {
    if (ws !== sock) return; // an older socket closing late
    clearInterval(pingTimer);
    if (!tokenFn) return;
    // Phones cut the connection when the app goes to the background. That's
    // normal: reconnect quietly when it comes back instead of retrying (and
    // showing "Reconnecting…") while nobody is looking.
    if (document.visibilityState === 'hidden') return;
    emit('status', 'offline');
    const delay = Math.min(15000, 500 * 2 ** retry++);
    reconnectTimer = setTimeout(open, delay);
  };
  sock.onerror = () => sock.close();
}

// Back in the foreground: reconnect at once and tell the app to refresh what
// may have changed while it was away (results, chat, balance).
document.addEventListener('visibilitychange', () => {
  if (!tokenFn) return;
  if (document.visibilityState === 'hidden') { hiddenAt = Date.now(); return; }
  retry = 0;
  const wasDown = !ws || ws.readyState > 1;
  if (wasDown) open();
  if (hiddenAt && Date.now() - hiddenAt > 3000) emit('resume', { away: Date.now() - hiddenAt, reconnected: wasDown });
  hiddenAt = 0;
});
window.addEventListener('online', () => { if (tokenFn) { retry = 0; open(); } });
