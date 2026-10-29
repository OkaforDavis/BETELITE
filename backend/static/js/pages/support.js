import { get, post } from '../api.js';
import { html, ic, timeAgo, empty, busy, toast } from '../ui.js';

const STATUS = { open: ['Waiting for reply', 'warn'], answered: ['Answered', 'brand'], closed: ['Resolved', ''] };

// Help & complaints: players message the admins and read their replies.
export default async function supportPage(view, { query }) {
  const [{ categories }, mine] = await Promise.all([get('/support/categories'), get('/support')]);
  let tickets = mine.tickets;

  function draw() {
    view.innerHTML = String(html`<div class="page">
      <div class="h1">Help & complaints</div>
      <div class="muted small" style="margin-top:4px">Tell us about a problem with a match, tournament, payment or player. An admin replies here and you get a notification.</div>

      <form class="card" id="new" style="margin-top:18px">
        <label class="field" style="margin-top:0"><span class="label">What is it about?</span>
          <select class="select" name="category" required>
            <option value="">Choose…</option>
            ${Object.entries(categories).map(([k, l]) => html`<option value="${k}" ${query.category === k ? 'selected' : ''}>${l}</option>`)}
          </select></label>
        <label class="field"><span class="label">Subject</span><input class="input" name="subject" maxlength="100" required placeholder="e.g. My opponent uploaded the wrong score"></label>
        <label class="field"><span class="label">Details</span><textarea class="input" name="message" rows="5" maxlength="2000" required placeholder="What happened, when, and what you'd like us to do."></textarea></label>
        <label class="field"><span class="label">Match, tournament or withdrawal ID <span class="faint">(optional)</span></span><input class="input" name="refId" maxlength="64" value="${query.ref || ''}"></label>
        <button class="btn primary block" style="margin-top:16px">${ic('send')} Send to admin</button>
      </form>

      <div class="section-head" style="margin-top:24px"><h2 class="h2">Your messages</h2></div>
      ${tickets.length ? html`<div class="stack">${tickets.map((t) => {
        const [label, cls] = STATUS[t.status] || [t.status, ''];
        return html`<div class="card">
          <div class="row between"><span class="badge ${cls}">${label}</span><span class="small faint">${timeAgo(t.createdAt)}</span></div>
          <div class="h3" style="margin-top:10px">${t.subject}</div>
          <div class="small muted">${categories[t.category] || t.category}${t.refId ? ' · ' + t.refId : ''}</div>
          <p class="small" style="margin-top:8px;white-space:pre-wrap">${t.message}</p>
          ${t.adminReply ? html`<div class="notice brand" style="margin-top:10px">${ic('shield')}<div class="small"><b>CrestArena support:</b> <span style="white-space:pre-wrap">${t.adminReply}</span></div></div>` : ''}
        </div>`;
      })}</div>` : html`<div class="card flat">${empty('checkCircle', 'No messages yet', 'Anything you send appears here with our reply.')}</div>`}
    </div>`);

    view.querySelector('#new').onsubmit = (e) => {
      e.preventDefault();
      const f = e.target;
      busy(f.querySelector('button'), async () => {
        await post('/support', { category: f.category.value, subject: f.subject.value, message: f.message.value, refId: f.refId.value });
        toast("Sent. We'll reply here and notify you.", 'ok');
        tickets = (await get('/support')).tickets;
        draw();
      });
    };
  }
  draw();
}
