// Thin progress bar at the top of the screen while pages or data load.
let active = 0;
let bar = null;
let hideTimer = null;

function el() {
  if (!bar) {
    bar = document.createElement('div');
    bar.className = 'topbar';
    bar.setAttribute('aria-hidden', 'true');
    document.body.append(bar);
  }
  return bar;
}

export function start() {
  active++;
  clearTimeout(hideTimer);
  const b = el();
  if (!b.classList.contains('on')) {
    b.classList.remove('done');
    void b.offsetWidth; // restart the transition
    b.classList.add('on');
  }
}

export function done() {
  active = Math.max(0, active - 1);
  if (active > 0) return;
  hideTimer = setTimeout(() => {
    const b = el();
    b.classList.remove('on');
    b.classList.add('done');
  }, 120);
}

// Wrap a promise so the bar shows while it runs.
export async function track(promise) {
  start();
  try { return await promise; } finally { done(); }
}
