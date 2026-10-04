import { get } from '../api.js';
import { store } from '../store.js';
import { html, ic, skeleton, empty, sectionError } from '../ui.js';
import { sk } from '../skeletons.js';
import { matchHero, matchRow, tournamentCard } from '../components.js';
import { canInstall, promptInstall, pushState, enablePush } from '../pwa.js';
import { refreshCurrent } from '../app.js';

export default async function home(view) {
  const draw = async () => {
    const { profile, current } = store.get();
    const firstName = (profile?.username || 'Player').split(' ')[0];
    const setup = [];
    if (!profile.gameProfiles.length) setup.push(['gamepad', 'Add your game IDs', 'So we can verify your results', '/profile#games']);
    const ps = pushState();
    if (ps === 'default' || ps === 'install-first') setup.push(['bell', 'Turn on match alerts', "Know instantly when it's your move", 'push']);
    if (canInstall()) setup.push(['phone', 'Install the app', 'Faster, full-screen, works offline', 'install']);

    view.innerHTML = String(html`<div class="page">
      <div class="eyebrow">Welcome back</div>
      <div class="h1" style="margin-top:6px">${firstName}</div>

      <section class="section" aria-labelledby="now">
        <div class="section-head"><h2 class="h2" id="now">Your match</h2>${current ? html`<a href="/play">All matches</a>` : ''}</div>
        <div id="now-body">${nowBody(current)}</div>
      </section>

      ${setup.length ? html`<section class="section">
        <div class="section-head"><h2 class="h2">Get ready to play</h2><span class="small faint">${setup.length} left</span></div>
        <div class="list">${setup.map(([i, t, s, act]) => html`
          <button class="list-row" data-act="${act}"><div class="icon-tile brand">${ic(i)}</div>
          <div class="grow"><div class="h3">${t}</div><div class="small muted">${s}</div></div>${ic('chevronRight', 'chev')}</button>`)}</div>
      </section>` : ''}

      <section class="section" aria-labelledby="tourn">
        <div class="section-head"><h2 class="h2" id="tourn">Open tournaments</h2><a href="/tournaments">See all</a></div>
        <div id="tournaments">${sk.tournamentCards(1)}</div>
      </section>

      <section class="section" aria-labelledby="live">
        <div class="section-head"><h2 class="h2" id="live">Live now</h2><a href="/watch">${ic('radio', '')}</a></div>
        <div id="live-list">${sk.rows(3)}</div>
      </section>

      <p class="center tiny faint" style="margin-top:32px">18+ only · Play responsibly · <a href="/legal/responsible-gaming.html" target="_blank">Get help</a></p>
    </div>`);

    view.querySelectorAll('[data-act]').forEach((b) => (b.onclick = async () => {
      const act = b.dataset.act;
      if (act === 'push') { if (await enablePush()) draw(); }
      else if (act === 'install') promptInstall();
      else (await import("../router.js")).navigate("/profile?section=games");
    }));

    loadTournaments();
    loadLive();
  };

  const loadTournaments = () => {
    const el = view.querySelector('#tournaments');
    if (el) el.innerHTML = String(sk.tournamentCards(1));
    get('/tournaments').then(({ tournaments }) => {
      const open = tournaments.filter((t) => t.status === 'open').slice(0, 3);
      const box = view.querySelector('#tournaments');
      if (!box) return;
      box.innerHTML = String(open.length
        ? html`<div class="stack">${open.map(tournamentCard)}</div>`
        : html`<div class="card flat">${empty('trophy', 'No open tournaments', 'New tournaments are posted every week. Turn on alerts so you don’t miss one.')}</div>`);
    }).catch((e) => sectionError(view.querySelector('#tournaments'), e.message, loadTournaments));
  };

  const loadLive = () => {
    const el = view.querySelector('#live-list');
    if (el) el.innerHTML = String(sk.rows(3));
    get('/matches').then(({ matches }) => {
      const box = view.querySelector('#live-list');
      if (!box) return;
      box.innerHTML = String(matches.length
        ? html`<div class="list">${matches.slice(0, 6).map(matchRow)}</div>`
        : html`<div class="card flat">${empty('radio', 'Quiet right now', 'Matches appear here as soon as players start.')}</div>`);
    }).catch((e) => sectionError(view.querySelector('#live-list'), e.message, loadLive));
  };

  await draw();
  let last = key(store.get().current);
  const unsub = store.subscribe(({ current }) => {
    const body = view.querySelector("#now-body");
    if (body && key(current) !== last) { last = key(current); body.innerHTML = String(nowBody(current)); }
  });
  refreshCurrent();
  return () => unsub();
}

const key = (m) => (m ? m.id + m.status : '');

function nowBody(current) {
  if (current) return matchHero(current);
  return html`<div class="grid-2">
    <a class="card tap" href="/play" style="color:inherit">
      <div class="icon-tile brand">${ic('swords')}</div>
      <div class="h3" style="margin-top:12px">Play 1v1</div>
      <div class="small muted">Challenge a player. Winner takes 80% of the pot.</div></a>
    <a class="card tap" href="/tournaments" style="color:inherit">
      <div class="icon-tile info">${ic('trophy')}</div>
      <div class="h3" style="margin-top:12px">Tournaments</div>
      <div class="small muted">Knockouts and leagues with big prize pools.</div></a>
  </div>`;
}
