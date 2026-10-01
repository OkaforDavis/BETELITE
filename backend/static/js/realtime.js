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

function open() {
  if (!tokenFn) return;
  const base = API_BASE || location.origin;
  const url = base.replace(/^http/, 'ws') + '/ws';
  ws = new WebSocket(url);
  ws.onopen = async () => {
    retry = 0;
    emit('status', 'online');
    const token = await tokenFn();
    if (token) send('identify', { token });
    rooms.forEach((room) => send('join_room', { room }));
    clearInterval(pingTimer);
    pingTimer = setInterval(() => send('ping', {}), 25000);
  };
  ws.onmessage = (e) => {
    // The server may batch several JSON messages separated by newlines.
    for (const line of String(e.data).split('\n')) {
      if (!line.trim()) continue;
      try {
        const msg = JSON.parse(line);
        emit(msg.event, msg.data);
      } catch { /* ignore malformed */ }
    }
  };
  ws.onclose = () => {
    clearInterval(pingTimer);
    if (!tokenFn) return;
    emit('status', 'offline');
    const delay = Math.min(30000, 1000 * 2 ** retry++);
    setTimeout(open, delay);
  };
  ws.onerror = () => ws.close();
}

// Reconnect immediately when the app comes back to the foreground.
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible' && tokenFn && (!ws || ws.readyState > 1)) { retry = 0; open(); }
});
