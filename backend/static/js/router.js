// Path-based router (deep links like /match/abc work from push notifications).
const routes = [];
let cleanup = null;
let onChange = () => {};

export function route(pattern, loader, { tab } = {}) {
  const keys = [];
  const re = new RegExp('^' + pattern.replace(/:(\w+)/g, (_, k) => { keys.push(k); return '([^/]+)'; }) + '/?$');
  routes.push({ re, keys, loader, tab });
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
  for (const r of routes) {
    const m = path.match(r.re);
    if (!m) continue;
    const params = Object.fromEntries(r.keys.map((k, i) => [k, decodeURIComponent(m[i + 1])]));
    params.query = Object.fromEntries(new URLSearchParams(location.search));
    if (typeof cleanup === 'function') cleanup();
    cleanup = null;
    onChange(r.tab);
    window.scrollTo(0, 0);
    const mod = await r.loader();
    cleanup = await mod.default(view, params);
    return;
  }
  navigate('/', { replace: true });
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
