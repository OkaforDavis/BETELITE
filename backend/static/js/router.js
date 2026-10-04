// Path-based router (deep links like /match/abc work from push notifications).
// Shows the page's skeleton immediately, then the real content; a failed page
// shows an error with "Try again" instead of hanging.
import * as progress from './progress.js';

const routes = [];
let cleanup = null;
let onChange = () => {};
let renderSeq = 0;

export function route(pattern, loader, { tab, skeleton } = {}) {
  const keys = [];
  const re = new RegExp('^' + pattern.replace(/:(\w+)/g, (_, k) => { keys.push(k); return '([^/]+)'; }) + '/?$');
  routes.push({ re, keys, loader, tab, skeleton });
}

export function onRouteChange(fn) { onChange = fn; }

export function navigate(path, { replace = false } = {}) {
  if (path === location.pathname + location.search && !replace) return render();
  history[replace ? 'replaceState' : 'pushState']({}, '', path);
  return render();
}

export async function render() {
  const path = location.pathname;
  const view = document.getElementById('view');
  const r = routes.find((x) => x.re.test(path));
  if (!r) return navigate('/', { replace: true });

  const seq = ++renderSeq;
  const m = path.match(r.re);
  const params = Object.fromEntries(r.keys.map((k, i) => [k, decodeURIComponent(m[i + 1])]));
  params.query = Object.fromEntries(new URLSearchParams(location.search));
  if (typeof cleanup === 'function') cleanup();
  cleanup = null;
  onChange(r.tab);
  window.scrollTo(0, 0);
  if (r.skeleton) view.innerHTML = String(r.skeleton());

  progress.start();
  try {
    const mod = await r.loader();
    if (seq !== renderSeq) return; // the user already moved on
    const c = await mod.default(view, params);
    if (seq === renderSeq) cleanup = c;
    else if (typeof c === 'function') c();
  } catch (e) {
    if (seq !== renderSeq) return;
    console.error(e);
    const { html, ic } = await import('./ui.js');
    view.innerHTML = String(html`<div class="page"><div class="page-error">
      <div class="icon-tile danger">${ic(e?.status === 0 ? 'radio' : 'alert')}</div>
      <div class="h2">${e?.status === 0 ? "You're offline" : "This page didn't load"}</div>
      <p class="muted" style="margin:8px 0 20px">${e?.message || 'Something went wrong. Please try again.'}</p>
      <button class="btn primary" data-retry>${ic('refresh')} Try again</button></div></div>`);
    view.querySelector('[data-retry]').onclick = () => render();
  } finally {
    progress.done();
  }
}

export function startRouter() {
  window.addEventListener('popstate', render);
  document.addEventListener('click', (e) => {
    const a = e.target.closest('a[href]');
    if (!a || a.target === '_blank' || a.hasAttribute('download') || e.metaKey || e.ctrlKey) return;
    const url = new URL(a.href, location.href);
    if (url.origin !== location.origin || url.pathname.startsWith('/legal/') || url.pathname.startsWith('/api/')) return;
    e.preventDefault();
    navigate(url.pathname + url.search);
  });
  return render();
}
