// Service worker, "new version" prompt, install prompt and Web Push.
import { get, post } from './api.js';
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
    const v = await get('/version');
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
window.addEventListener('beforeinstallprompt', (e) => {
  e.preventDefault();
  deferredInstall = e;
  document.dispatchEvent(new Event('installable'));
});
window.addEventListener('appinstalled', () => { deferredInstall = null; toast('CrestArena installed on your device'); });

export const canInstall = () => !isStandalone() && (!!deferredInstall || isIOS);

export async function promptInstall() {
  if (deferredInstall) {
    deferredInstall.prompt();
    await deferredInstall.userChoice.catch(() => {});
    deferredInstall = null;
    return;
  }
  if (isIOS) {
    sheet(String(html`${sheetHead('Install on iPhone', 'Get match alerts and full-screen play')}
      <ol class="list" style="list-style:none">
        <li class="list-row"><div class="icon-tile">${ic('share')}</div><div>Tap the <b>Share</b> button in Safari's toolbar</div></li>
        <li class="list-row"><div class="icon-tile">${ic('plus')}</div><div>Scroll down and tap <b>Add to Home Screen</b></div></li>
        <li class="list-row"><div class="icon-tile brand">${ic('checkCircle')}</div><div>Tap <b>Add</b>, then open CrestArena from your Home Screen</div></li>
      </ol>
      <p class="hint" style="margin-top:12px">Must be opened in Safari. iPhone notifications need iOS 16.4 or newer and the app added to your Home Screen.</p>
      <div class="sheet-actions"><button class="btn secondary block" data-close>Got it</button></div>`), { label: 'Install' });
  }
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
    const { publicKey } = await get('/notifications/vapid-public-key');
    if (!publicKey) return false;
    let sub = await r.pushManager.getSubscription();
    if (!sub) sub = await r.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: urlBase64ToUint8Array(publicKey) });
    await post('/notifications/subscribe', sub.toJSON());
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
