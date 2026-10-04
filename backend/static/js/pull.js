// Pull down from the top of a page to refresh it. Installed apps (PWA) have
// no browser refresh button, so this is the way to reload data.
import { icon } from './icons.js';

const THRESHOLD = 70;

export function installPullToRefresh(onRefresh) {
  if (!('ontouchstart' in window) || document.querySelector('.ptr')) return;
  const ind = document.createElement('div');
  ind.className = 'ptr';
  ind.setAttribute('aria-hidden', 'true');
  ind.innerHTML = icon('arrowDown');
  document.body.append(ind);

  let startY = null;
  let dy = 0;
  let busy = false;

  const reset = () => {
    ind.style.transition = 'transform 0.25s, opacity 0.25s';
    ind.style.transform = 'translateY(-80px)';
    ind.style.opacity = '0';
    ind.classList.remove('ready', 'loading');
  };

  document.addEventListener('touchstart', (e) => {
    if (busy || window.scrollY > 0 || document.querySelector('.sheet-backdrop') || e.touches.length !== 1) { startY = null; return; }
    startY = e.touches[0].clientY;
    dy = 0;
  }, { passive: true });

  document.addEventListener('touchmove', (e) => {
    if (startY === null) return;
    dy = e.touches[0].clientY - startY;
    if (dy <= 0 || window.scrollY > 0) { dy = 0; return; }
    const pull = Math.min(dy * 0.5, THRESHOLD + 20);
    ind.style.transition = 'none';
    ind.style.transform = `translateY(${pull - 40}px)`;
    ind.style.opacity = String(Math.min(1, pull / THRESHOLD));
    ind.classList.toggle('ready', pull >= THRESHOLD);
  }, { passive: true });

  document.addEventListener('touchend', async () => {
    if (startY === null) return;
    startY = null;
    if (dy * 0.5 < THRESHOLD) { reset(); return; }
    busy = true;
    ind.classList.remove('ready');
    ind.classList.add('loading');
    ind.innerHTML = icon('refresh');
    try { await onRefresh(); } catch { /* the page shows its own error */ }
    busy = false;
    ind.innerHTML = icon('arrowDown');
    reset();
  });
}
