// CrestArena PWA entry point.
import { get, post, quietGet, setTokenProvider, currentToken } from './api.js';
import { initAuth, getToken, renderAuth } from './auth.js';
import { store } from './store.js';
import { html, ic, money, toast, $ } from './ui.js';
import { route, startRouter, onRouteChange, navigate } from './router.js';
import * as rt from './realtime.js';
import { registerSW, syncPushSubscription, setBadge } from './pwa.js';
import { cookieNotice, needsConsent, consentGate } from './consent.js';
import { captureInvite, applyPendingInvite } from './invite.js';
import { sk } from './skeletons.js';
import { render as rerender } from './router.js';
import { installPullToRefresh } from './pull.js';

captureInvite();

setTokenProvider(getToken);

route('/', () => import('./pages/home.js'), { tab: 'home', skeleton: sk.home });
route('/play', () => import('./pages/play.js'), { tab: 'play', skeleton: sk.play });
route('/match/:id', () => import('./pages/match.js'), { tab: 'play', skeleton: sk.match });
route('/tournaments', () => import('./pages/tournaments.js'), { tab: 'tournaments', skeleton: sk.tournaments });
route('/tournaments/:id', () => import('./pages/tournament.js'), { tab: 'tournaments', skeleton: sk.tournament });
route('/wallet', () => import('./pages/wallet.js'), { tab: 'wallet', skeleton: sk.wallet });
route('/profile', () => import('./pages/profile.js'), { tab: 'profile', skeleton: sk.profile });
route('/notifications', () => import('./pages/notifications.js'), { skeleton: sk.notifications });
route('/watch', () => import('./pages/watch.js'), { tab: 'home', skeleton: sk.watch });
route('/watch/:id', () => import('./pages/watch.js'), { tab: 'home', skeleton: sk.watch });
route('/admin', () => import('./pages/admin.js'), { skeleton: sk.admin });
route('/join/:code', () => import('./pages/home.js'), { tab: 'home', skeleton: sk.home });

const root = document.getElementById('root');
let shellMounted = false;

// ── Data refreshers (used by pages and realtime events) ─────────────
export async function refreshProfile() {
  const { profile } = await get('/me');
  store.set({ profile });
  return profile;
}
export async function refreshCurrent() {
  const { match } = await quietGet('/matches/current').catch(() => ({ match: null }));
  store.set({ current: match });
  return match;
}
export async function refreshUnread() {
  const { unread } = await quietGet('/notifications').catch(() => ({ unread: 0 }));
  store.set({ unread });
  setBadge(unread);
}
export const refreshAll = () => Promise.all([refreshProfile(), refreshCurrent(), refreshUnread()]);

// ── Shell ───────────────────────────────────────────────────────────
const TABS = [
  ['home', '/', 'home', 'Home'],
  ['play', '/play', 'swords', 'Play'],
  ['tournaments', '/tournaments', 'trophy', 'Compete'],
  ['wallet', '/wallet', 'wallet', 'Wallet'],
  ['profile', '/profile', 'user', 'Profile'],
];

function mountShell() {
  if (shellMounted) return;
  shellMounted = true;
  root.innerHTML = String(html`
    <div class="app">
      <header class="appbar">
        <a href="/" aria-label="CrestArena home"><img class="brand" src="/icons/wordmark.png" alt="CrestArena" height="22"></a>
        <div class="spacer"></div>
        <a class="icon-btn" href="/notifications" aria-label="Notifications" id="bell">${ic('bell')}<span class="count" hidden></span></a>
        <a class="balance-pill num" href="/wallet" aria-label="Wallet balance"><span id="balance">—</span><span class="plus">${ic('plus')}</span></a>
      </header>
      <main id="view" tabindex="-1"></main>
      <nav class="tabbar" aria-label="Main">
        ${TABS.map(([key, href, ico, label]) => html`<a class="tab" href="${href}" data-tab="${key}">${ic(ico)}<span>${label}</span>${key === 'play' ? html`<i class="dot" hidden></i>` : ''}</a>`)}
      </nav>
    </div>`);

  onRouteChange((tab) => {
    document.querySelectorAll('.tab').forEach((t) => {
      const on = t.dataset.tab === tab;
      t.classList.toggle('active', on);
      if (on) t.setAttribute('aria-current', 'page'); else t.removeAttribute('aria-current');
    });
  });

  store.subscribe(({ profile, unread, current }) => {
    if (!$("#balance")) return;
    if (profile) $("#balance").textContent = money(profile.balance, profile.currency);
    const badge = $('#bell .count');
    badge.hidden = !unread;
    badge.textContent = unread > 9 ? '9+' : unread;
    const uid = profile?.id;
    const actionNeeded = current && (current.status === 'ready' || (current.status === 'submitted' && current.submittedBy !== uid));
    $('.tab[data-tab=play] .dot').hidden = !actionNeeded;
  });
}

// ── Realtime ────────────────────────────────────────────────────────
function wireRealtime() {
  rt.connect(currentToken);
  rt.on('status', (s) => {
    let bar = $('.connection');
    if (s === 'offline' && !bar) {
      bar = document.createElement('div');
      bar.className = 'connection';
      bar.textContent = 'Reconnecting…';
      document.body.append(bar);
    } else if (s === 'online') bar?.remove();
  });
  rt.on('notification', (n) => {
    toast(n.message, 'info', n.title);
    refreshUnread();
    if (/deposit|withdrawal|result_confirmed|tournament_finished|challenge_expired|match_void/.test(n.type)) refreshProfile();
    if (/result|challenge|match|tournament/.test(n.type)) refreshCurrent();
  });
  rt.on('match_update', (m) => {
    const me = store.get().profile?.id;
    if (m.homeId === me || m.awayId === me) refreshCurrent();
    document.dispatchEvent(new CustomEvent('match_update', { detail: m }));
  });
  navigator.serviceWorker?.addEventListener('message', (e) => {
    if (e.data?.type === 'push') refreshUnread();
  });
}

// Paystack returns here after the hosted checkout (?deposit=ref).
async function handleDepositReturn() {
  const ref = new URLSearchParams(location.search).get('deposit');
  if (!ref) return;
  history.replaceState({}, '', '/wallet');
  try {
    const { status } = await post('/wallet/deposit/verify', { reference: ref });
    if (status === 'paid') toast('Your wallet has been topped up.', 'ok', 'Deposit received');
    else if (status === 'pending') toast("We're confirming your payment. Your balance updates automatically.", 'info');
    else toast('The payment was not completed.', 'err');
    refreshProfile();
  } catch (e) { toast(e.message, 'err'); }
}

// ── Boot ────────────────────────────────────────────────────────────
async function onUser(user) {
  if (!user) {
    shellMounted = false;
    rt.disconnect();
    store.set({ user: null, profile: null, current: null, unread: 0 });
    renderAuth(root);
    cookieNotice(true);
    return;
  }
  store.set({ user });
  // Skeleton of the whole app while we sign in. On the free hosting plan the
  // server may be asleep, so after a few seconds explain the wait.
  root.innerHTML = String(sk.shell());
  const wake = setTimeout(() => { const w = root.querySelector('.sk-wake'); if (w) w.hidden = false; }, 6000);
  try {
    const [{ games }] = await Promise.all([get('/games'), refreshProfile()]);
    store.set({ games });
  } catch (e) {
    root.innerHTML = String(html`<div class="auth center"><div class="h2">Can't reach CrestArena</div>
      <p class="muted" style="margin:8px 0 20px">${e.message}</p><button class="btn primary" onclick="location.reload()">Try again</button></div>`);
    return;
  }
  clearTimeout(wake);
  mountShell();
  wireRealtime();
  installPullToRefresh(async () => { await Promise.all([refreshProfile(), refreshCurrent(), refreshUnread()]); await rerender(); });
  await startRouter();
  refreshCurrent();
  refreshUnread();
  handleDepositReturn();
  syncPushSubscription();
  applyPendingInvite();
  if (new URLSearchParams(location.search).get('open') === 'current' && store.get().current) navigate('/match/' + store.get().current.id);

  const profile = store.get().profile;
  if (needsConsent(profile)) consentGate(profile, () => { refreshProfile(); cookieNotice(); });
  else cookieNotice();
}

registerSW();

// Local development only: ?dev=alice signs in as a test user against a server
// running without Firebase (the server rejects dev tokens in production).
const isLocal = ['localhost', '127.0.0.1'].includes(location.hostname);
const devUser = isLocal && (new URLSearchParams(location.search).get('dev') || sessionStorage.getItem('crestarena.dev'));
if (devUser) {
  sessionStorage.setItem('crestarena.dev', devUser);
  setTokenProvider(async () => 'dev:' + devUser);
  onUser({ uid: 'dev-' + devUser });
} else {
  initAuth(onUser);
}
