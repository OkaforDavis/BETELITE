// Service worker, "new version" prompt, install prompt and Web Push.
import { quietGet, quietPost, post } from './api.js';
import { html, ic, sheet, sheetHead, toast, toastError } from './ui.js';

export const APP_VERSION = window.APP_VERSION || 'dev';
export const isIOS = /iphone|ipad|ipod/i.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
export const isStandalone = () => matchMedia('(display-mode: standalone)').matches || navigator.standalone === true;

let reg = null;
let deferredInstall = null;
let updateShown = false;

// ── Updates ──────────────────────────────────────────────────
export async function registerSW() {
  if (!('serviceWorker' in navigator)) return;
  try {
    reg = await navigator.serviceWorker.register('/sw.js', { scope: '/', updateViaCache: 'none' });
  } catch (e) {
    console.warn('SW registration failed', e);
    return;
  }
  if (reg.waiting && navigator.serviceWorker.controller) showUpdate();
  reg.addEventListener('updatefound', () => {
    const sw = reg.installing;
    sw?.addEventListener('statechange', () => {
      if (sw.state === 'installed' && navigator.serviceWorker.controller) showUpdate();
    });
  });
  let reloading = false;
  navigator.serviceWorker.addEventListener('controllerchange', () => {
    if (reloading) return;
    reloading = true;
    location.reload();
  });
  navigator.serviceWorker.addEventListener('message', (e) => {
    if (e.data?.type === 'navigate' && e.data.url) {
      import('./router.js').then((r) => r.navigate(e.data.url));
    }
  });

  const check = () => { reg?.update().catch(() => {}); checkVersion(); };
  setInterval(check, 15 * 60 * 1000);
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') check(); });
  checkVersion();
}

async function checkVersion() {
  try {
    const v = await quietGet('/version');
    if (v.version && v.version !== APP_VERSION && APP_VERSION !== 'dev') {
      reg?.update().catch(() => {});
      showUpdate(v.force);
    }
  } catch { /* offline */ }
}

function applyUpdate() {
  if (reg?.waiting) reg.waiting.postMessage({ type: 'SKIP_WAITING' });
  else location.reload();
}

function showUpdate(force = false) {
  if (force) {
    sheet(String(html`<div class="center" style="padding:8px 0">
      <div class="icon-tile brand" style="width:64px;height:64px;margin:0 auto 16px">${ic('sparkles')}</div>
      <div class="h2">Update required</div>
      <p class="muted" style="margin-top:8px">A new version of CrestArena is required to keep playing. It only takes a second.</p>
      <div class="sheet-actions"><button class="btn primary block" data-update>${ic('refresh')} Update now</button></div></div>`),
    { dismissable: false, label: 'Update required', onMount: (root) => { root.querySelector('[data-update]').onclick = applyUpdate; } });
    return;
  }
  if (updateShown) return;
  updateShown = true;
  const el = document.createElement('div');
  el.className = 'banner';
  el.setAttribute('role', 'alert');
  el.innerHTML = String(html`<div class="icon-tile brand">${ic('sparkles')}</div>
    <div class="grow"><b>New version available</b><span>Update for the latest features and fixes.</span></div>
    <button class="btn primary sm" data-update>Update</button>
    <button class="icon-btn" data-later aria-label="Later">${ic('x')}</button>`);
  el.querySelector('[data-update]').onclick = applyUpdate;
  el.querySelector('[data-later]').onclick = () => el.remove();
  document.body.append(el);
}

// ── Install ──────────────────────────────────────────────────
// Chrome fires this a few seconds after the page loads, usually after the
// home page has already drawn. Pages listen for 'installable' to redraw, and
// a small banner offers the install the first time it becomes possible.
window.addEventListener('beforeinstallprompt', (e) => {
  e.preventDefault();
  deferredInstall = e;
  document.dispatchEvent(new Event('installable'));
  showInstallBanner();
});
window.addEventListener('appinstalled', () => {
  deferredInstall = null;
  try { localStorage.setItem(INSTALLED, '1'); } catch { /* ignore */ }
  document.querySelector('.banner.install')?.remove();
  document.dispatchEvent(new Event('installable'));
  toast('CrestArena installed on your device');
});

const INSTALLED = 'crestarena.installed';
const installedHere = () => { try { return localStorage.getItem(INSTALLED) === '1'; } catch { return false; } };
// In-app browsers (Instagram, TikTok, Facebook…) can't install web apps.
const inAppBrowser = () => /FBAN|FBAV|FB_IAB|Instagram|musical_ly|TikTok|BytedanceWebview|Snapchat|Line\//i.test(navigator.userAgent);

// The install option stays available in any browser tab (not inside the app):
// if Chrome's own prompt isn't ready, we show the steps to install by hand.
// Home hides it once this device has installed the app; Profile always offers it.
export const canInstall = () => !isStandalone() && !installedHere();
export const canInstallAgain = () => !isStandalone();

// onInstallable runs fn whenever the install state changes; returns an unsubscribe.
export function onInstallable(fn) {
  document.addEventListener('installable', fn);
  return () => document.removeEventListener('installable', fn);
}

const INSTALL_SNOOZE = 'crestarena.install-snooze';
function showInstallBanner() {
  if (isStandalone() || document.querySelector('.banner.install')) return;
  try {
    if (+localStorage.getItem(INSTALL_SNOOZE) > Date.now()) return;
  } catch { /* storage unavailable */ }
  const el = document.createElement('div');
  el.className = 'banner install';
  el.setAttribute('role', 'dialog');
  el.innerHTML = String(html`<div class="icon-tile brand">${ic('phone')}</div>
    <div class="grow"><b>Install CrestArena</b><span>Full screen, faster, with match alerts.</span></div>
    <button class="btn primary sm" data-install>Install</button>
    <button class="icon-btn" data-later aria-label="Not now">${ic('x')}</button>`);
  const snooze = () => {
    el.remove();
    try { localStorage.setItem(INSTALL_SNOOZE, String(Date.now() + 7 * 864e5)); } catch { /* ignore */ }
  };
  el.querySelector('[data-install]').onclick = async () => { el.remove(); await promptInstall(); };
  el.querySelector('[data-later]').onclick = snooze;
  document.body.append(el);
}

export async function promptInstall() {
  if (deferredInstall && !inAppBrowser()) {
    // Chrome allows each install prompt to be used only once, so never keep it.
    const ev = deferredInstall;
    deferredInstall = null;
    try {
      await ev.prompt();
      const choice = await ev.userChoice;
      document.dispatchEvent(new Event('installable'));
      // Accepted: Chrome installs it. Cancelled: the option stays; next tap shows the manual steps.
      if (choice) return;
    } catch { /* the prompt failed: fall through to the manual steps */ }
  }
  manualInstall();
}

// Step-by-step install for when the browser's own prompt isn't available.
function manualInstall() {
  const ua = navigator.userAgent;
  const row = (icon, text) => html`<li class="list-row"><div class="icon-tile">${ic(icon)}</div><div>${text}</div></li>`;
  let title = 'Install CrestArena';
  let steps;
  let note = 'Already installed? Open CrestArena from your home screen or app list.';
  if (inAppBrowser()) {
    title = 'Open in your browser first';
    steps = [
      row('external', html`Tap the <b>⋮</b> or <b>…</b> menu at the top of this screen`),
      row('external', html`Choose <b>Open in browser</b> (or Open in Chrome / Safari)`),
      row('phone', html`Then tap <b>Install the app</b> again there`),
    ];
    note = "Apps like Instagram, TikTok and Facebook open links in their own viewer, which can't install apps.";
  } else if (isIOS) {
    title = 'Install on iPhone';
    steps = [
      row('share', html`Tap the <b>Share</b> button (square with an arrow) in the toolbar`),
      row('plus', html`Scroll down and tap <b>Add to Home Screen</b>`),
      row('checkCircle', html`Tap <b>Add</b>, then open CrestArena from your Home Screen`),
    ];
    note = 'Use Safari if you don’t see Add to Home Screen. iPhone notifications need iOS 16.4 or newer and the app on your Home Screen.';
  } else if (/SamsungBrowser/i.test(ua)) {
    steps = [
      row('list', html`Tap the <b>☰</b> menu at the bottom right`),
      row('plus', html`Tap <b>Add page to</b> → <b>Home screen</b>`),
      row('checkCircle', html`Tap <b>Add</b>, then open CrestArena from your home screen`),
    ];
  } else if (/Android/i.test(ua)) {
    steps = [
      row('list', html`Tap the <b>⋮</b> menu at the top right of Chrome`),
      row('plus', html`Tap <b>Install app</b> (or <b>Add to Home screen</b> → <b>Install</b>)`),
      row('checkCircle', html`Open CrestArena from your home screen`),
    ];
    note += ' If it fails, close all CrestArena tabs, reopen crestarena.com in Chrome and try again.';
  } else {
    steps = [
      row('download', html`Click the <b>install</b> icon at the right of the address bar`),
      row('list', html`Or open the browser menu → <b>Save and share</b> / <b>Apps</b> → <b>Install CrestArena</b>`),
      row('checkCircle', html`CrestArena opens in its own window`),
    ];
  }
  sheet(String(html`${sheetHead(title, 'Full screen, faster, with match alerts')}
    <ol class="list" style="list-style:none">${steps}</ol>
    <p class="hint" style="margin-top:12px">${note}</p>
    <div class="sheet-actions">${inAppBrowser() ? html`<button class="btn primary block" data-copy-site>${ic('copy')} Copy the link</button>` : ''}
      <button class="btn secondary block" data-close>Got it</button></div>`), {
    label: 'Install',
    onMount(root, close) {
      root.querySelector('[data-close]').onclick = close;
      root.querySelector('[data-copy-site]')?.addEventListener('click', () => {
        navigator.clipboard?.writeText(location.origin).then(() => toast('Link copied. Paste it in Chrome or Safari.'), () => {});
      });
    },
  });
}

// ── Push notifications ───────────────────────────────────────
export function pushState() {
  if (!('Notification' in window) || !('serviceWorker' in navigator) || !('PushManager' in window)) {
    return isIOS && !isStandalone() ? 'install-first' : 'unsupported';
  }
  if (isIOS && !isStandalone()) return 'install-first';
  return Notification.permission; // granted | denied | default
}

function urlBase64ToUint8Array(b64) {
  const pad = '='.repeat((4 - (b64.length % 4)) % 4);
  const raw = atob((b64 + pad).replace(/-/g, '+').replace(/_/g, '/'));
  return Uint8Array.from([...raw].map((c) => c.charCodeAt(0)));
}

// Must be called from a tap (iOS requires a user gesture).
export async function enablePush() {
  const state = pushState();
  if (state === 'install-first') { promptInstall(); return false; }
  if (state === 'unsupported') { toast('Notifications are not supported on this browser', 'info'); return false; }
  const perm = await Notification.requestPermission();
  if (perm !== 'granted') {
    toast('Notifications are blocked. Allow them in your browser settings to get match alerts.', 'info');
    return false;
  }
  return syncPushSubscription(true);
}

export async function syncPushSubscription(announce = false) {
  if (pushState() !== 'granted') return false;
  try {
    const r = reg || (await navigator.serviceWorker.ready);
    const { publicKey } = await quietGet('/notifications/vapid-public-key');
    if (!publicKey) return false;
    let sub = await r.pushManager.getSubscription();
    if (!sub) sub = await r.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: urlBase64ToUint8Array(publicKey) });
    await quietPost('/notifications/subscribe', sub.toJSON());
    if (announce) toast("Match alerts are on. We'll notify you when it's your move.");
    return true;
  } catch (e) {
    if (announce) toastError(e);
    return false;
  }
}

export async function disablePush() {
  try {
    const r = reg || (await navigator.serviceWorker.ready);
    const sub = await r.pushManager.getSubscription();
    if (sub) {
      await post('/notifications/unsubscribe', { endpoint: sub.endpoint });
      await sub.unsubscribe();
    }
  } catch { /* ignore */ }
}

export function setBadge(n) {
  if ('setAppBadge' in navigator) {
    (n > 0 ? navigator.setAppBadge(n) : navigator.clearAppBadge()).catch(() => {});
  }
}
