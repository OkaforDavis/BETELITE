// First-time guided tours: a ring and a pulsing pointer on one part of the
// screen, a short note, and Back / Next. Each page's tour shows once per
// account on this device; "Show app tips again" in Profile brings them back.
import { store } from './store.js';
import { esc } from './ui.js';

// el: CSS selector (first visible match). No el = a centred note.
// Steps whose element isn't on screen are skipped.
const TOURS = [
  {
    id: 'home', path: /^\/$/, steps: [
      { title: 'Welcome to CrestArena', text: 'Play real people in FC Mobile, eFootball, DLS, CODM and Free Fire. Here is a quick look around. It takes 30 seconds.' },
      { el: '.tab[data-tab=profile]', title: 'First: add your game IDs', text: 'Open Profile and add your in-game name for each game you play. We use it to check your result screenshots.' },
      { el: '.tab[data-tab=play]', title: 'Play 1v1', text: 'Post a challenge or accept one. You play in the game, then upload the final result screen.' },
      { el: '.tab[data-tab=tournaments]', title: 'Compete in cups', text: 'Join cups, or host your own and invite friends.' },
      { el: '#now', title: 'Your match', text: 'Your next match shows here, so you always know what to do next.' },
      { el: '#bell', title: 'Notifications', text: 'Room codes, results and cup rounds arrive here. Turn on alerts so you never miss your turn.' },
      { el: '.balance-pill', title: 'Your wallet', text: 'Your balance. Tap it to add money, see tips you got, or cash out.' },
    ],
  },
  {
    id: 'play', path: /^\/play$/, steps: [
      { el: '#new', title: 'Post a challenge', text: 'Pick your game and post. Any player can accept it, and you both get a match page.' },
      { el: '.seg[role=tablist]', title: 'Open challenges and your matches', text: 'Open shows challenges you can accept. My matches shows the ones you are in.' },
      { el: '[data-accept]', title: 'Accept a challenge', text: 'Tap Accept to play this person. You have 3 hours to play and upload the result.' },
    ],
  },
  {
    id: 'match', path: /^\/match\/[^/]+$/, steps: [
      { el: '.versus', title: 'Your match', text: 'You and your opponent, with the score once it is in.' },
      { el: '.steps', title: 'Where you are', text: 'Play → Result sent → Check → Final. The bar shows the step you are on.' },
      { el: '#room-card', title: 'Meet in the game', text: 'Follow the steps for your game. The host posts the room code here, and the other player copies it and joins.' },
      { el: '#drop', title: 'Upload the result', text: 'After the match, screenshot the final result screen showing both names, and upload it here. Our AI reads the score.' },
      { el: '[data-tip]', title: 'Tips', text: 'Liked the game? Send your opponent a tip. It is free: they get the full amount.' },
    ],
  },
  {
    id: 'tournaments', path: /^\/tournaments$/, steps: [
      { el: '#host', title: 'Host a cup', text: 'Make your own cup, invite friends and add a sponsor. Hosts can earn tips and a commission.' },
      { el: '.seg[role=tablist]', title: 'Find a cup', text: 'Open cups take players now. In progress are being played. Mine are the cups you joined or host.' },
      { el: '#list .card', title: 'Open a cup', text: 'Tap a cup to see the prize, rules and players, and to join.' },
    ],
  },
  {
    id: 'tournament', path: /^\/tournaments\/[^/]+$/, steps: [
      { el: '#join', title: 'Join the cup', text: 'Join here. When the cup starts, your matches appear under Matches and you get a notification.' },
      { el: '.seg[role=tablist]', title: 'Bracket, table and matches', text: 'Follow the cup: see who plays who and the results.' },
      { el: '#share', title: 'Bring friends', text: 'Share the cup link so your friends can join.' },
      { el: '[data-tip]', title: 'Tip the host', text: 'Enjoying the cup? Tip the host. It is free: they get the full amount.' },
    ],
  },
  {
    id: 'wallet', path: /^\/wallet$/, steps: [
      { el: '.wallet-card .amount', title: 'Your balance', text: 'Money you can use or cash out. Prizes and tips land here.' },
      { el: '#dep', title: 'Add money', text: 'Pay by card, bank transfer or mobile money. Use it for memberships, hosting cups and tips.' },
      { el: '#wd', title: 'Cash out', text: 'Send money to your bank or mobile money. The minimum is ₦500 or GH₵5.' },
      { el: '.section .list', title: 'Activity', text: 'Every deposit, prize, tip and cash-out is listed here.' },
    ],
  },
  {
    id: 'profile', path: /^\/profile$/, steps: [
      { el: '#games', title: 'Game IDs', text: 'Add your in-game name for each game, exactly as the game shows it. It locks after your first match.' },
      { el: '#share', title: 'Invite friends', text: 'Share your invite link. Friends who join with it are linked to you.' },
      { el: '#tours-reset', title: 'See these tips again', text: 'Tap here any time to replay these guides.' },
    ],
  },
];

const key = () => `crestarena.tours.${store.get().profile?.id || 'anon'}`;
function seen() {
  try { return new Set(JSON.parse(localStorage.getItem(key()) || '[]')); } catch { return new Set(); }
}
function markSeen(id) {
  try { const s = seen(); s.add(id); localStorage.setItem(key(), JSON.stringify([...s])); } catch { /* private mode */ }
}
export function resetTours() {
  try { localStorage.removeItem(key()); } catch { /* ignore */ }
}

const visible = (el) => { const r = el.getBoundingClientRect(); return r.width > 0 && r.height > 0; };
const find = (sel) => [...document.querySelectorAll(sel)].find(visible);
const busy = () => document.querySelector('.sheet-backdrop, .banner.cookie, .tour');

let active = null; // { end() }
let pending = 0;

// Called by the router after a page has drawn.
export function pageReady(path) {
  active?.end();
  const seq = ++pending;
  if (!store.get().profile) return;
  const tour = TOURS.find((t) => t.path.test(path));
  if (!tour || seen().has(tour.id)) return;
  // Wait for the page's data (and any open sheet or cookie notice) to settle.
  let tries = 0;
  const tick = () => {
    if (seq !== pending || location.pathname !== path) return;
    // A sheet or notice is open (e.g. the Terms on signup): wait for it to close.
    if (busy()) return setTimeout(tick, 600);
    const steps = tour.steps.filter((s) => !s.el || find(s.el));
    if (steps.some((s) => s.el)) return start(tour, steps);
    if (++tries < 20) setTimeout(tick, 400);
  };
  setTimeout(tick, 700);
}

function start(tour, steps) {
  let i = 0;
  const root = document.createElement('div');
  root.className = 'tour';
  root.innerHTML = `<div class="tour-ring" aria-hidden="true"></div><div class="tour-dot" aria-hidden="true"></div>
    <div class="tour-tip" role="dialog" aria-modal="true" aria-labelledby="tour-title" tabindex="-1">
      <div class="tour-arrow" aria-hidden="true"></div>
      <div class="tiny faint tour-count"></div>
      <div class="h3" id="tour-title"></div>
      <p class="small tour-text"></p>
      <div class="tour-actions"><button type="button" class="link-btn" data-t="skip">Skip</button>
        <span class="spacer"></span>
        <button type="button" class="btn ghost sm" data-t="back">Back</button>
        <button type="button" class="btn primary sm" data-t="next">Next</button></div>
    </div>`;
  document.body.append(root);
  const ring = root.querySelector('.tour-ring');
  const dot = root.querySelector('.tour-dot');
  const tip = root.querySelector('.tour-tip');
  const arrow = root.querySelector('.tour-arrow');

  const place = () => {
    const s = steps[i];
    const el = s.el && find(s.el);
    const vw = window.innerWidth, vh = window.innerHeight, pad = 8, gap = 14;
    tip.style.maxWidth = Math.min(340, vw - 32) + 'px';
    const tw = tip.offsetWidth, th = tip.offsetHeight;
    if (!el) {
      root.classList.add('centred');
      ring.style.cssText = `left:${vw / 2}px;top:${vh / 2}px;width:0;height:0`;
      tip.style.left = Math.max(16, (vw - tw) / 2) + 'px';
      tip.style.top = Math.max(16, (vh - th) / 2) + 'px';
      return;
    }
    root.classList.remove('centred');
    const r = el.getBoundingClientRect();
    ring.style.cssText = `left:${r.left - pad}px;top:${r.top - pad}px;width:${r.width + pad * 2}px;height:${r.height + pad * 2}px`;
    const cx = r.left + r.width / 2;
    // The pulsing pointer sits on the ring's top-right corner so it never hides text.
    dot.style.left = Math.min(r.right + pad - 2, vw - 10) + 'px';
    dot.style.top = Math.max(r.top - pad + 2, 10) + 'px';
    const below = r.bottom + pad + gap + th < vh - 8 || r.top - pad - gap - th < 8;
    const top = below ? r.bottom + pad + gap : r.top - pad - gap - th;
    const left = Math.min(Math.max(16, cx - tw / 2), vw - tw - 16);
    tip.style.top = Math.max(8, top) + 'px';
    tip.style.left = left + 'px';
    tip.dataset.side = below ? 'below' : 'above';
    arrow.style.left = Math.min(Math.max(18, cx - left), tw - 18) + 'px';
  };

  const show = () => {
    const s = steps[i];
    root.querySelector('.tour-count').textContent = `${i + 1} of ${steps.length}`;
    root.querySelector('#tour-title').textContent = s.title;
    root.querySelector('.tour-text').innerHTML = esc(s.text);
    root.querySelector('[data-t=back]').hidden = i === 0;
    root.querySelector('[data-t=next]').textContent = i === steps.length - 1 ? 'Done' : 'Next';
    const el = s.el && find(s.el);
    if (el) {
      const r = el.getBoundingClientRect();
      const fixed = el.closest('.tabbar, .appbar');
      if (!fixed && (r.top < 80 || r.bottom > window.innerHeight - 120)) {
        el.scrollIntoView({ block: 'center', behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
        setTimeout(place, 350);
      }
    }
    place();
    tip.focus({ preventScroll: true });
  };

  const end = (done = true) => {
    if (!root.isConnected) return;
    if (done) markSeen(tour.id);
    root.remove();
    window.removeEventListener('resize', place);
    window.removeEventListener('scroll', place, true);
    document.removeEventListener('keydown', onKey, true);
    active = null;
  };
  const next = () => (i < steps.length - 1 ? (i++, show()) : end());
  const back = () => { if (i > 0) { i--; show(); } };
  const onKey = (e) => {
    if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); end(); }
    else if (e.key === 'ArrowRight') { e.preventDefault(); next(); }
    else if (e.key === 'ArrowLeft') { e.preventDefault(); back(); }
  };
  root.addEventListener('click', (e) => {
    const t = e.target.closest('[data-t]')?.dataset.t;
    if (t === 'next') next();
    else if (t === 'back') back();
    else if (t === 'skip') end();
  });
  window.addEventListener('resize', place);
  window.addEventListener('scroll', place, true);
  document.addEventListener('keydown', onKey, true);
  active = { end: () => end(false) };
  show();
}

// Replays the tour for the page the player is on now.
export function replayHere() {
  resetTours();
  pageReady(location.pathname);
}
