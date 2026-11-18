// Cup sponsors: a brand's name, link, pictures and one short video on a host cup.
import { api, upload, del, API_BASE } from './api.js';
import { html, ic, sheet, sheetHead, busy, toast, confirmSheet, empty } from './ui.js';

const MAX_IMAGES = 4;
const MAX_VIDEO = 5 * 1024 * 1024;
const MAX_SECONDS = 30;
const mediaUrl = (id) => `${API_BASE}/api/sponsor-media/${id}`;
const domain = (link) => { try { return new URL(link).hostname.replace(/^www\./, ''); } catch { return ''; } };

// The sponsor card on a cup page. Players see approved sponsors; the host
// also sees pending/rejected ones and the buttons to manage them.
export function sponsorCard(sponsor, { isHost, open }) {
  if (!sponsor && !(isHost && open)) return '';
  if (!sponsor) {
    return html`<div class="card row" style="margin-top:12px;gap:12px"><div class="icon-tile">${ic('sparkles')}</div>
      <div class="grow"><div class="h3">Got a sponsor?</div><div class="small muted">Show a brand's pictures and a short video on your cup.</div></div>
      <button class="btn secondary sm" data-sponsor-edit>Add sponsor</button></div>`;
  }
  const live = sponsor.status === 'approved';
  const pics = sponsor.media.filter((m) => m.kind === 'image');
  const vid = sponsor.media.find((m) => m.kind === 'video');
  return html`<div class="card sponsor" style="margin-top:12px">
    <div class="row between" style="gap:8px"><span class="eyebrow">Sponsored by</span>
      ${isHost ? html`<span class="badge ${live ? 'brand' : sponsor.status === 'rejected' ? 'danger' : 'warn'}">${live ? 'Showing' : sponsor.status === 'rejected' ? 'Not approved' : 'Waiting for approval'}</span>` : ''}</div>
    <div class="h3" style="margin-top:6px">${sponsor.name}</div>
    ${sponsor.link ? html`<a class="small row" href="${sponsor.link}" target="_blank" rel="noopener nofollow sponsored" style="gap:4px;margin-top:2px">${domain(sponsor.link)} ${ic('external')}</a>` : ''}
    ${live ? html`
      ${vid ? html`<video class="sponsor-video" src="${mediaUrl(vid.id)}" muted autoplay loop playsinline controls preload="metadata" style="margin-top:12px"></video>` : ''}
      ${pics.length ? html`<div class="sponsor-pics" style="margin-top:12px">${pics.map((p) => html`<img src="${mediaUrl(p.id)}" alt="${sponsor.name}" loading="lazy">`)}</div>` : ''}`
    : isHost ? html`<p class="small muted" style="margin-top:8px">${sponsor.status === 'rejected'
        ? html`<b>Reason:</b> ${sponsor.reason}. Change it and send it again.`
        : `${pics.length} picture${pics.length === 1 ? '' : 's'}${vid ? ' and a video' : ''}. An admin checks it first, usually within a few hours. Players see it once approved.`}</p>` : ''}
    ${isHost ? html`<div class="row" style="gap:8px;margin-top:12px">${open ? html`<button class="btn secondary sm" data-sponsor-edit>Change</button>` : ''}
      <button class="btn ghost sm" data-sponsor-remove>Remove</button></div>` : ''}
    <p class="tiny faint" style="margin-top:10px">${live ? 'Shown until a day after the cup ends, then deleted.' : 'Sponsors show while the cup runs and for a day after it ends, then they are deleted.'}</p>
  </div>`;
}

export function bindSponsor(root, tid, onChange) {
  root.querySelector('[data-sponsor-edit]')?.addEventListener('click', () => sponsorSheet(tid, onChange));
  root.querySelector('[data-sponsor-remove]')?.addEventListener('click', async (e) => {
    if (!(await confirmSheet({ title: 'Remove the sponsor?', message: 'Its pictures and video are deleted.', confirm: 'Remove', danger: true }))) return;
    busy(e.currentTarget, async () => { await del(`/tournaments/${tid}/sponsor`); toast('Sponsor removed'); onChange(); });
  });
}

// Shrinks a picture on the phone before upload (max 1280px JPEG).
async function shrink(file) {
  const bmp = await createImageBitmap(file).catch(() => null);
  if (!bmp) throw new Error(`"${file.name}" isn't a picture we can read. Use JPG or PNG.`);
  const scale = Math.min(1, 1280 / Math.max(bmp.width, bmp.height));
  const c = document.createElement('canvas');
  c.width = Math.round(bmp.width * scale); c.height = Math.round(bmp.height * scale);
  c.getContext('2d').drawImage(bmp, 0, 0, c.width, c.height);
  return new Promise((ok) => c.toBlob((b) => ok(b), 'image/jpeg', 0.75));
}

const videoSeconds = (file) => new Promise((ok) => {
  const v = document.createElement('video');
  v.preload = 'metadata';
  v.onloadedmetadata = () => { URL.revokeObjectURL(v.src); ok(v.duration); };
  v.onerror = () => ok(NaN);
  v.src = URL.createObjectURL(file);
});

function sponsorSheet(tid, onChange) {
  sheet(String(html`${sheetHead('Add a sponsor', 'Shown on your cup after an admin approves it')}
    <form id="sp">
      <label class="field" style="margin-top:0"><span class="label">Sponsor name</span><input class="input" name="name" required minlength="2" maxlength="40" placeholder="e.g. Mama Put Kitchen"></label>
      <label class="field"><span class="label">Website or social link (optional)</span><input class="input" name="link" inputmode="url" maxlength="200" placeholder="instagram.com/yourbrand" autocapitalize="off" spellcheck="false"></label>
      <label class="field"><span class="label">Pictures (up to ${MAX_IMAGES})</span><input class="input" name="images" type="file" accept="image/*" multiple>
        <span class="hint">Logos, flyers or products. We shrink them so they load fast.</span></label>
      <label class="field"><span class="label">Short video (optional)</span><input class="input" name="video" type="file" accept="video/mp4,video/webm,video/quicktime">
        <span class="hint">MP4, up to ${MAX_SECONDS} seconds and 5 MB. Low quality is fine. It plays muted.</span></label>
      <ul class="checklist" style="margin-top:12px">
        <li class="no">${ic('xCircle')}<span>No betting, gambling, loans, adult content, alcohol, tobacco or politics</span></li>
        <li class="ok">${ic('check')}<span>You must have the right to use these pictures and video</span></li>
      </ul>
      <div class="sheet-actions"><button class="btn primary block" type="submit">Send for approval</button></div>
    </form>`), {
    label: 'Add a sponsor',
    onMount(root, close) {
      const f = root.querySelector('#sp');
      f.onsubmit = (e) => {
        e.preventDefault();
        busy(f.querySelector('[type=submit]'), async () => {
          const pics = [...f.images.files];
          const video = f.video.files[0];
          if (!pics.length && !video) throw new Error('Add at least one picture or a short video.');
          if (pics.length > MAX_IMAGES) throw new Error(`You can add up to ${MAX_IMAGES} pictures.`);
          if (video) {
            if (video.size > MAX_VIDEO) throw new Error('The video is too large. Keep it under 5 MB (a short, low-quality clip).');
            const secs = await videoSeconds(video);
            if (secs > MAX_SECONDS + 0.5) throw new Error(`The video is ${Math.round(secs)} seconds. Keep it to ${MAX_SECONDS} seconds or less.`);
          }
          const fd = new FormData();
          fd.append('name', f.name.value.trim());
          fd.append('link', f.link.value.trim());
          for (const p of pics) fd.append('images', await shrink(p), 'picture.jpg');
          if (video) fd.append('video', video, video.name);
          await upload(`/tournaments/${tid}/sponsor`, fd);
          close(); toast('Sent. We will tell you when it is approved.', 'ok', 'Sponsor added');
          onChange();
        });
      };
    },
  });
}

// ── Admin: review sponsors ───────────────────────────────────
export async function adminSponsors(body, redraw) {
  const { sponsors } = await api('GET', '/admin/sponsors');
  if (!sponsors.length) { body.innerHTML = String(html`<div class="card flat">${empty('sparkles', 'No sponsors', 'Sponsors that hosts add to their cups appear here for approval.')}</div>`); return; }
  body.innerHTML = String(html`<div class="list">${sponsors.map((s) => html`<div class="card" style="margin-bottom:12px">
    <div class="row between" style="gap:8px"><div class="h3">${s.name}</div>
      <span class="badge ${s.status === 'approved' ? 'brand' : s.status === 'rejected' ? 'danger' : 'warn'}">${s.status}</span></div>
    <div class="small muted">${s.cup} · host ${s.host}${s.link ? html` · <a href="${s.link}" target="_blank" rel="noopener nofollow">${domain(s.link)}</a>` : ''}</div>
    <div class="sponsor-pics" style="margin-top:10px" data-media="${s.media.map((m) => `${m.kind}:${m.id}`).join(',')}"></div>
    <div class="row" style="gap:8px;margin-top:12px">
      ${s.status !== 'approved' ? html`<button class="btn primary sm" data-approve="${s.tournamentId}">Approve</button>` : ''}
      ${s.status !== 'rejected' ? html`<button class="btn ghost sm" data-reject="${s.tournamentId}">${s.status === 'approved' ? 'Take down' : 'Reject'}</button>` : ''}
      <a class="btn ghost sm" href="/tournaments/${s.tournamentId}">Open cup</a></div>
  </div>`)}</div>`);

  // Admin previews need the login token, so load them as blobs.
  body.querySelectorAll('[data-media]').forEach(async (el) => {
    for (const item of el.dataset.media.split(',').filter(Boolean)) {
      const [kind, id] = item.split(':');
      const res = await api('GET', `/admin/sponsors/media/${id}`, undefined, { raw: true, quiet: true }).catch(() => null);
      if (!res?.ok) continue;
      const url = URL.createObjectURL(await res.blob());
      const node = document.createElement(kind === 'video' ? 'video' : 'img');
      node.src = url;
      if (kind === 'video') { node.controls = true; node.muted = true; node.playsInline = true; node.className = 'sponsor-video'; }
      el.appendChild(node);
    }
  });
  body.querySelectorAll('[data-approve]').forEach((b) => (b.onclick = () => busy(b, async () => {
    await api('POST', `/admin/sponsors/${b.dataset.approve}/review`, { approve: true });
    toast('Approved. The host has been told.'); redraw();
  })));
  body.querySelectorAll('[data-reject]').forEach((b) => (b.onclick = () => {
    sheet(String(html`${sheetHead('Reason for the host')}<form id="rj"><textarea class="input" name="reason" rows="3" required minlength="3" placeholder="e.g. Betting adverts aren't allowed"></textarea>
      <div class="sheet-actions"><button class="btn danger block" type="submit">Reject / take down</button></div></form>`), {
      label: 'Reject sponsor',
      onMount(root, close) {
        root.querySelector('#rj').onsubmit = (e) => {
          e.preventDefault();
          busy(e.target.querySelector('[type=submit]'), async () => {
            await api('POST', `/admin/sponsors/${b.dataset.reject}/review`, { approve: false, reason: e.target.reason.value.trim() });
            close(); toast('Done. The host has been told.'); redraw();
          });
        };
      },
    });
  }));
}
