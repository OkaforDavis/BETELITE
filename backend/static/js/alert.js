// CrestArena match sounds, made in the browser (no audio files to download).
//  - matchSound(): "gling-zzzz" — two bright bell strikes and a short buzz,
//    played twice. Used for "I'm ready" alerts, so players know at once that
//    it's from a match.
//  - chatSound(): a soft two-note pop for new chat messages.
// Phones only allow sound after the player has tapped the page once, so the
// audio is unlocked on the first tap.
import { navigate } from './router.js';
import { html, ic } from './ui.js';

let ctx = null;
function audio() {
  if (!ctx) {
    const AC = window.AudioContext || window.webkitAudioContext;
    if (!AC) return null;
    try { ctx = new AC(); } catch { return null; }
  }
  if (ctx.state === 'suspended') ctx.resume().catch(() => {});
  return ctx;
}
['pointerdown', 'keydown', 'touchstart'].forEach((ev) =>
  window.addEventListener(ev, function unlock() {
    const a = audio();
    if (a) {
      // A silent blip unlocks audio on iOS.
      const o = a.createOscillator(); const g = a.createGain(); g.gain.value = 0;
      o.connect(g).connect(a.destination); o.start(); o.stop(a.currentTime + 0.01);
    }
    window.removeEventListener(ev, unlock, true);
  }, { capture: true, passive: true }));

const muted = () => { try { return localStorage.getItem('crestarena.sound') === 'off'; } catch { return false; } };
export function soundOn() { return !muted(); }
export function setSound(on) { try { localStorage.setItem('crestarena.sound', on ? 'on' : 'off'); } catch { /* private mode */ } }

// One bell strike: a few sine partials that fade out like metal.
function bell(a, out, t, freq, vol) {
  [[1, 1], [2.76, 0.45], [5.4, 0.22], [8.93, 0.08]].forEach(([mult, amp]) => {
    const o = a.createOscillator(); const g = a.createGain();
    o.type = 'sine'; o.frequency.value = freq * mult;
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(vol * amp, t + 0.005);
    g.gain.exponentialRampToValueAtTime(0.0001, t + 0.9 / Math.sqrt(mult));
    o.connect(g).connect(out); o.start(t); o.stop(t + 1);
  });
}

// The "zzzz": a buzzy low tone with a fast flutter, rising slightly.
function buzz(a, out, t, len, vol) {
  const g = a.createGain();
  g.gain.setValueAtTime(0.0001, t);
  g.gain.exponentialRampToValueAtTime(vol, t + 0.03);
  g.gain.setValueAtTime(vol, t + len * 0.6);
  g.gain.exponentialRampToValueAtTime(0.0001, t + len);
  const flutter = a.createGain(); flutter.gain.value = 0.5;
  const lfo = a.createOscillator(); const lfoAmp = a.createGain();
  lfo.frequency.value = 32; lfoAmp.gain.value = 0.5;
  lfo.connect(lfoAmp).connect(flutter.gain);
  const band = a.createBiquadFilter(); band.type = 'bandpass'; band.frequency.value = 1100; band.Q.value = 0.8;
  [110, 165.5].forEach((f) => {
    const o = a.createOscillator(); o.type = 'sawtooth';
    o.frequency.setValueAtTime(f, t); o.frequency.linearRampToValueAtTime(f * 1.12, t + len);
    o.connect(band); o.start(t); o.stop(t + len + 0.05);
  });
  band.connect(flutter).connect(g).connect(out);
  lfo.start(t); lfo.stop(t + len + 0.05);
}

export function matchSound() {
  if (muted()) return;
  const a = audio(); if (!a) return;
  const out = a.createGain(); out.gain.value = 0.9; out.connect(a.destination);
  const t0 = a.currentTime + 0.02;
  for (const t of [t0, t0 + 1.15]) {
    bell(a, out, t, 1318.5, 0.32);          // E6  "gl-"
    bell(a, out, t + 0.11, 1975.5, 0.3);    // B6  "-ing"
    buzz(a, out, t + 0.3, 0.55, 0.22);      // "zzzz"
  }
  try { navigator.vibrate?.([300, 120, 300, 120, 700]); } catch { /* not allowed */ }
}

export function chatSound() {
  if (muted()) return;
  const a = audio(); if (!a) return;
  const out = a.createGain(); out.gain.value = 0.5; out.connect(a.destination);
  const t = a.currentTime + 0.01;
  bell(a, out, t, 880, 0.25);
  bell(a, out, t + 0.09, 1318.5, 0.22);
  try { navigator.vibrate?.(60); } catch { /* not allowed */ }
}

// Big banner at the top for "I'm ready" alerts (and chat messages when the
// chat isn't on screen), with a button to the match.
export function matchBanner({ title, message, url, chat = false }) {
  document.querySelector('.match-alert')?.remove();
  const el = document.createElement('div');
  el.className = 'match-alert' + (chat ? ' chat' : '');
  el.setAttribute('role', chat ? 'status' : 'alert');
  el.innerHTML = String(html`<div class="match-alert-icon">${ic(chat ? 'chat' : 'bellRing')}</div>
    <div class="grow"><b>${title}</b><div class="small">${message}</div></div>
    ${url ? html`<button class="btn primary sm" data-go>${chat ? 'Reply' : 'Open'}</button>` : ''}
    <button class="icon-btn" data-close aria-label="Close">${ic('x')}</button>`);
  document.body.append(el);
  const close = () => { el.classList.add('out'); setTimeout(() => el.remove(), 260); };
  el.querySelector('[data-go]')?.addEventListener('click', () => {
    close();
    if (location.pathname === url) document.querySelector(chat ? '#chat' : '#room, #action')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    else navigate(url + (chat ? '?chat=1' : ''));
  });
  el.querySelector('[data-close]').addEventListener('click', close);
  setTimeout(close, chat ? 7000 : 15000);
}
