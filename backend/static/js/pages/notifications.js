import { get, post } from '../api.js';
import { html, ic, timeAgo, empty, skeleton } from '../ui.js';
import { navigate } from '../router.js';
import { refreshUnread } from '../app.js';
import { pushState, enablePush } from '../pwa.js';

const ICONS = {
  result_submitted: ['clock', 'warn'], result_disputed: ['flag', 'danger'], result_confirmed: ['trophy', 'brand'],
  challenge_accepted: ['swords', 'brand'], challenge_expired: ['refresh', 'info'], match_void: ['xCircle', ''],
  deposit: ['arrowDown', 'brand'], withdrawal: ['bank', 'info'],
  tournament_started: ['trophy', 'brand'], tournament_round: ['trophy', 'info'], tournament_finished: ['crown', 'brand'],
  tournament_cancelled: ['xCircle', 'danger'], admin_review: ['shield', 'warn'],
};

export default async function notificationsPage(view) {
  view.innerHTML = String(html`<div class="page">${skeleton(64, 6)}</div>`);

  async function draw() {
    const { notifications, unread } = await get('/notifications');
    const ps = pushState();
    view.innerHTML = String(html`<div class="page">
      <div class="row between"><div class="h1">Notifications</div>
        ${unread ? html`<button class="btn ghost sm" id="all">${ic('check')} Mark all read</button>` : ''}</div>
      ${ps === 'default' || ps === 'install-first' ? html`<button class="notice brand" id="enable" style="margin-top:16px;width:100%;text-align:left">${ic('bell')}
        <div class="grow"><b>Get alerts on your phone</b><div class="small">Know the moment a challenge is accepted or a result needs checking.</div></div>${ic('chevronRight')}</button>` : ''}
      <div style="margin-top:16px">${notifications.length ? html`<div class="list">${notifications.map((n) => {
        const [i, tone] = ICONS[n.type] || ['bell', ''];
        return html`<button class="list-row" data-id="${n.id}" data-url="${n.metadata?.url || ''}" style="align-items:flex-start">
          <div class="icon-tile ${tone}">${ic(i)}</div>
          <div class="grow"><div class="row between" style="gap:8px"><div class="h3">${n.title}</div>${n.read ? '' : html`<span class="badge brand" style="height:18px;padding:0 6px">New</span>`}</div>
            <div class="small muted" style="margin-top:2px">${n.message}</div><div class="tiny faint" style="margin-top:6px">${timeAgo(n.createdAt)}</div></div>
        </button>`;
      })}</div>` : html`<div class="card flat">${empty('bell', "You're all caught up", 'Match updates, payouts and new tournaments will appear here.')}</div>`}</div>
    </div>`);

    view.querySelector('#all')?.addEventListener('click', async () => { await post('/notifications/read', {}); refreshUnread(); draw(); });
    view.querySelector('#enable')?.addEventListener('click', async () => { if (await enablePush()) draw(); });
    view.querySelectorAll('[data-id]').forEach((b) => (b.onclick = async () => {
      post('/notifications/read', { ids: [b.dataset.id] }).then(refreshUnread).catch(() => {});
      if (b.dataset.url) navigate(b.dataset.url);
      else draw();
    }));
  }
  await draw();
}
