// UI helpers: safe HTML templating, toasts, sheets, money/time formatting.
import { icon } from './icons.js';
import { API_BASE } from './api.js';

const ESC = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
export const esc = (v) => String(v ?? '').replace(/[&<>"']/g, (c) => ESC[c]);

// html`...${value}...` escapes every interpolation unless wrapped in raw().
class Raw { constructor(s) { this.s = s; } toString() { return this.s; } }
export const raw = (s) => new Raw(s);
export function html(strings, ...values) {
  let out = '';
  strings.forEach((s, i) => {
    out += s;
    if (i < values.length) {
      const v = values[i];
      if (v instanceof Raw) out += v.s;
      else if (Array.isArray(v)) out += v.map((x) => (x instanceof Raw ? x.s : esc(x))).join('');
      else if (v === false || v == null) out += '';
      else out += esc(v);
    }
  });
  return raw(out);
}
export const ic = (name, cls) => raw(icon(name, cls));

export const $ = (sel, root = document) => root.querySelector(sel);
export const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

// ── Money & time ──────────────────────────────────────────────
const SYM = { NGN: '₦', GHS: '₵' };
export const sym = (cur) => SYM[cur] || '₦';
export function money(minor, cur = 'NGN', { decimals = 'auto' } = {}) {
  const v = (Number(minor) || 0) / 100;
  const frac = decimals === 'auto' ? (Number.isInteger(v) ? 0 : 2) : decimals;
  return sym(cur) + v.toLocaleString('en-NG', { minimumFractionDigits: frac, maximumFractionDigits: frac });
}
export function timeAgo(iso) {
  const s = Math.round((Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return 'just now';
  if (s < 3600) return Math.floor(s / 60) + 'm ago';
  if (s < 86400) return Math.floor(s / 3600) + 'h ago';
  if (s < 604800) return Math.floor(s / 86400) + 'd ago';
  return new Date(iso).toLocaleDateString();
}
export function timeLeft(iso) {
  const ms = new Date(iso).getTime() - Date.now();
  if (ms <= 0) return 'now';
  const m = Math.floor(ms / 60000), h = Math.floor(m / 60), d = Math.floor(h / 24);
  if (d > 0) return `${d}d ${h % 24}h`;
  if (h > 0) return `${h}h ${m % 60}m`;
  const s = Math.floor((ms % 60000) / 1000);
  return `${m}:${String(s).padStart(2, '0')}`;
}
export const initials = (name = '?') =>
  name.trim().split(/\s+/).slice(0, 2).map((w) => w[0]).join('').toUpperCase() || '?';
export function avatar(name, url, cls = '') {
  if (url && url.startsWith('/api/')) url = API_BASE + url;
  // Initials sit underneath the photo, so a photo that fails to load falls back cleanly.
  return html`<div class="avatar ${cls}"><span>${initials(name)}</span>${url
    ? html`<img src="${url}" alt="" loading="lazy" onerror="this.remove()">`
    : ''}</div>`;
}

// Live countdowns: any [data-countdown=iso] element updates every second.
setInterval(() => {
  document.querySelectorAll('[data-countdown]').forEach((el) => {
    el.textContent = timeLeft(el.dataset.countdown);
  });
}, 1000);

// ── Toasts ────────────────────────────────────────────────────
export function toast(message, type = 'ok', title = '') {
  let box = $('.toasts');
  if (!box) { box = document.createElement('div'); box.className = 'toasts'; box.setAttribute('role', 'status'); document.body.append(box); }
  const el = document.createElement('div');
  el.className = 'toast ' + type;
  const ico = type === 'err' ? 'alert' : type === 'info' ? 'info' : 'checkCircle';
  el.innerHTML = String(html`${ic(ico)}<div>${title ? html`<b>${title}</b>` : ''}${message}</div>`);
  box.append(el);
  setTimeout(() => { el.classList.add('out'); setTimeout(() => el.remove(), 260); }, type === 'err' ? 5500 : 3500);
}
export const toastError = (e) => toast(e?.message || String(e), 'err');

// ── Sheets (bottom modals) ────────────────────────────────────
export function sheet(content, { onMount, onClose, dismissable = true, label = "Dialog" } = {}) {
  const back = document.createElement('div');
  back.className = 'sheet-backdrop';
  back.innerHTML = `<div class="sheet" role="dialog" aria-modal="true" aria-label="${esc(label)}">${content}</div>`;
  let closed = false;
  const close = () => {
    if (closed) return;
    closed = true;
    back.remove();
    document.removeEventListener("keydown", onKey);
    onClose?.();
  };
  const onKey = (e) => { if (e.key === 'Escape' && dismissable) close(); };
  if (dismissable) back.addEventListener('click', (e) => { if (e.target === back) close(); });
  back.addEventListener('click', (e) => { if (e.target.closest('[data-close]')) close(); });
  document.addEventListener('keydown', onKey);
  document.body.append(back);
  const root = back.firstElementChild;
  onMount?.(root, close);
  root.querySelector('input,select,textarea')?.focus({ preventScroll: true });
  return close;
}
export const sheetHead = (title, sub = '') => html`
  <div class="sheet-head"><div><div class="h2">${title}</div>${sub ? html`<div class="muted small" style="margin-top:4px">${sub}</div>` : ''}</div>
  <button class="icon-btn" data-close aria-label="Close">${ic('x')}</button></div>`;

export function confirmSheet({ title, message, confirm = "Confirm", danger = false }) {
  return new Promise((resolve) => {
    let yes = false;
    const close = sheet(String(html`${sheetHead(title)}<p class="muted">${message}</p>
      <div class="sheet-actions"><button class="btn ${danger ? "danger" : "primary"} block" data-yes>${confirm}</button>
      <button class="btn ghost block" data-close>Cancel</button></div>`), {
      onMount(root) { root.querySelector("[data-yes]").onclick = () => { yes = true; close(); }; },
      onClose: () => resolve(yes),
    });
  });
}

// Run an async action with a loading state on its button.
export async function busy(btn, fn) {
  btn?.classList.add('loading');
  btn && (btn.disabled = true);
  try { return await fn(); }
  catch (e) { toastError(e); return undefined; }
  finally { btn?.classList.remove('loading'); btn && (btn.disabled = false); }
}

// ── Shared fragments ──────────────────────────────────────────
export const empty = (iconName, title, text, action = '') => html`
  <div class="empty"><div class="icon-tile">${ic(iconName)}</div><div class="h3">${title}</div>
  <p class="small">${text}</p>${action ? html`<div style="margin-top:16px">${action}</div>` : ''}</div>`;
export const skeleton = (h = 120, n = 1) =>
  raw(Array.from({ length: n }, () => `<div class="skeleton" style="height:${h}px;margin-bottom:12px"></div>`).join(''));

const STATUS = {
  ready: ['Play now', 'brand', 'gamepad'],
  submitted: ['Result sent', 'warn', 'clock'],
  disputed: ['Disputed', 'danger', 'flag'],
  review: ['Under review', 'info', 'shield'],
  confirmed: ['Final', '', 'checkCircle'],
  void: ['Cancelled', '', 'xCircle'],
};
export function statusBadge(status) {
  const [label, cls, ico] = STATUS[status] || [status, '', 'info'];
  return html`<span class="badge ${cls}">${ic(ico)}${label}</span>`;
}

export function scoreLine(m) {
  if (m.scoreHome == null) return html`<span class="vs">VS</span>`;
  return html`<div class="score">${m.scoreHome}<span class="sep">–</span>${m.scoreAway}</div>
    ${m.pensHome != null ? html`<div class="pens">Pens ${m.pensHome}–${m.pensAway}</div>` : ''}`;
}
