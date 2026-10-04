import { get } from '../api.js';
import { html, skeleton, empty } from '../ui.js';
import { sk } from '../skeletons.js';
import { tournamentCard, gameChips } from '../components.js';

export default async function tournaments(view) {
  let filter = '';
  let tab = 'open';
  view.innerHTML = String(html`<div class="page">
    <div class="h1">Tournaments</div>
    <div class="muted small" style="margin-top:4px">Knockouts: winner takes 70% of entry fees. Leagues: top 4 get paid.</div>
    <div class="seg" style="margin-top:18px" role="tablist">
      <button role="tab" data-tab="open">Open</button><button role="tab" data-tab="active">In progress</button><button role="tab" data-tab="mine">Mine</button>
    </div>
    <div id="filters" style="margin-top:14px"></div>
    <div id="list" style="margin-top:14px">${sk.tournamentList()}</div>
  </div>`);

  const { tournaments: all } = await get('/tournaments');

  function draw() {
    view.querySelectorAll('[data-tab]').forEach((b) => b.classList.toggle('active', b.dataset.tab === tab));
    view.querySelector('#filters').innerHTML = String(gameChips(filter, { all: true }));
    view.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => { filter = b.dataset.game; draw(); }));
    const list = all.filter((t) => (!filter || t.game === filter) &&
      (tab === 'mine' ? t.joined : tab === 'open' ? t.status === 'open' : t.status === 'active'));
    view.querySelector('#list').innerHTML = String(list.length
      ? html`<div class="stack">${list.map(tournamentCard)}</div>`
      : html`<div class="card flat">${empty('trophy', tab === 'mine' ? "You haven't joined any yet" : 'Nothing here right now',
          tab === 'open' ? 'New tournaments are announced with a notification. Check back soon.' : 'Join an open tournament to compete for prizes.')}</div>`);
  }
  view.querySelectorAll('[data-tab]').forEach((b) => (b.onclick = () => { tab = b.dataset.tab; draw(); }));
  draw();
}
