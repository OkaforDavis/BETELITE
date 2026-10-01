// CrestArena service worker. The server stamps the build version below, so
// every deploy installs a new worker and the app shows "New version available".
const VERSION = '__APP_VERSION__';
const CACHE = 'crestarena-' + VERSION;
const SHELL = [
  '/',
  '/css/app.css',
  '/js/app.js', '/js/api.js', '/js/store.js', '/js/ui.js', '/js/icons.js', '/js/router.js',
  '/js/realtime.js', '/js/pwa.js', '/js/auth.js', '/js/consent.js', '/js/components.js', '/js/gameid.js',
  '/js/pages/home.js', '/js/pages/play.js', '/js/pages/match.js', '/js/pages/tournaments.js',
  '/js/pages/tournament.js', '/js/pages/wallet.js', '/js/pages/profile.js', '/js/pages/notifications.js',
  '/js/pages/watch.js', '/js/pages/admin.js',
  '/icons/icon-192.png', '/icons/icon-512.png', '/icons/mark.png', '/icons/badge-96.png', '/icons/wordmark.png', '/icons/apple-touch-icon.png',
  '/manifest.json',
  '/legal/terms.html', '/legal/privacy.html', '/legal/cookies.html', '/legal/responsible-gaming.html',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) =>
      // Cache what we can; one missing file must not block the update.
      Promise.allSettled(SHELL.map((url) => cache.add(new Request(url, { cache: 'reload' }))))
    )
  );
  // Do not skipWaiting automatically: the app asks the user first.
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k.startsWith('crestarena-') && k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') self.skipWaiting();
});

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);

  // Never cache API or WebSocket traffic: balances and results must be live.
  if (url.origin === location.origin && (url.pathname.startsWith('/api/') || url.pathname === '/ws')) return;

  // App pages: network first, fall back to the cached shell when offline.
  if (req.mode === 'navigate') {
    if (url.pathname.startsWith('/legal/')) {
      event.respondWith(fetch(req).catch(() => caches.match(req)));
      return;
    }
    event.respondWith(fetch(req).catch(() => caches.match('/')));
    return;
  }

  // Same-origin static files: cache first (cache is per version).
  if (url.origin === location.origin) {
    event.respondWith(
      caches.match(req, { ignoreSearch: true }).then((hit) =>
        hit || fetch(req).then((res) => {
          if (res.ok) { const copy = res.clone(); caches.open(CACHE).then((c) => c.put(req, copy)); }
          return res;
        })
      )
    );
    return;
  }

  // Google Fonts: cache for offline use.
  if (url.hostname.endsWith('fonts.googleapis.com') || url.hostname.endsWith('fonts.gstatic.com')) {
    event.respondWith(
      caches.open('crestarena-fonts').then((c) =>
        c.match(req).then((hit) => hit || fetch(req).then((res) => { c.put(req, res.clone()); return res; }))
      )
    );
  }
});

// ── Push notifications (Android + iOS 16.4+ home-screen app) ─────────
self.addEventListener('push', (event) => {
  let data = {};
  try { data = event.data ? event.data.json() : {}; } catch { data = { body: event.data?.text() }; }
  const title = data.title || 'CrestArena';
  const options = {
    body: data.body || data.message || '',
    icon: '/icons/icon-192.png',
    badge: '/icons/badge-96.png',
    tag: data.tag || 'crestarena',
    renotify: true,
    vibrate: [120, 60, 120],
    data: { url: data.url || '/' },
  };
  event.waitUntil(
    self.registration.showNotification(title, options).then(() =>
      self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((list) =>
        list.forEach((c) => c.postMessage({ type: 'push', payload: data }))
      )
    )
  );
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const url = event.notification.data?.url || '/';
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((list) => {
      for (const client of list) {
        if (new URL(client.url).origin === location.origin && 'focus' in client) {
          client.postMessage({ type: 'navigate', url });
          return client.focus();
        }
      }
      return self.clients.openWindow(url);
    })
  );
});
