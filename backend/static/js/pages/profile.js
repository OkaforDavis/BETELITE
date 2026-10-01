import { api, get, put, del } from '../api.js';
import { store } from '../store.js';
import { html, ic, avatar, sheet, sheetHead, busy, toast, toastError } from '../ui.js';
import { gameIdSheet } from '../gameid.js';
import { APP_VERSION, canInstall, promptInstall, pushState, enablePush, disablePush, isIOS } from '../pwa.js';
import { signOut, sendVerification, reloadVerification } from '../auth.js';
import { refreshProfile } from '../app.js';

export default async function profilePage(view, { query }) {
  const [{ matches }, ref] = await Promise.all([
    get('/matches/mine').catch(() => ({ matches: [] })),
    get('/referrals').catch(() => ({ code: '' })),
  ]);

  function draw() {
    const p = store.get().profile;
    const games = store.get().games;
    const done = matches.filter((m) => m.status === 'confirmed');
    const wins = done.filter((m) => m.winnerId === p.id).length;
    const ps = pushState();
    const pushOn = ps === 'granted' && p.pushEnabled;

    view.innerHTML = String(html`<div class="page">
      <div class="row" style="gap:16px">
        <button class="avatar-edit" id="photo" aria-label="Change profile photo">${avatar(p.username, p.avatarUrl, 'lg me')}<span class="cam">${ic('camera')}</span></button>
        <div class="grow"><div class="h2 ellipsis">${p.username}</div><div class="small muted ellipsis">${p.email}</div>
          <div class="tiny faint" style="margin-top:2px">Member since ${new Date(p.createdAt).toLocaleDateString(undefined, { month: 'short', year: 'numeric' })}</div></div>
        <button class="btn secondary sm" id="edit">Edit</button>
      </div>

      ${!p.emailVerified && p.email ? html`<div class="card highlight" style="margin-top:20px">
        <div class="row" style="align-items:flex-start"><div class="icon-tile warn">${ic("alert")}</div>
          <div class="grow"><div class="h3">Verify your email</div>
          <div class="small muted" style="margin-top:2px">We'll send a link to <b>${p.email}</b>. Verifying protects your account and is needed for withdrawals${p.adminPending ? " and to unlock the admin dashboard" : ""}.</div></div></div>
        <div class="grid-2" style="margin-top:14px"><button class="btn secondary sm" id="verify-send">Send link</button><button class="btn primary sm" id="verify-check">I've verified</button></div>
      </div>` : ""}

      <div class="grid-3" style="margin-top:20px">
        <div class="stat"><div class="v num">${done.length}</div><div class="k">Played</div></div>
        <div class="stat"><div class="v num brand-text">${wins}</div><div class="k">Wins</div></div>
        <div class="stat"><div class="v num">${done.length ? Math.round((wins / done.length) * 100) : 0}%</div><div class="k">Win rate</div></div>
      </div>

      <section class="section" id="games">
        <div class="section-head"><h2 class="h2">Game IDs</h2></div>
        <p class="small muted" style="margin:-4px 0 12px">Your in-game names are how we verify match screenshots. They lock after your first match.</p>
        <div class="list">${games.map((g) => {
          const gp = p.gameProfiles.find((x) => x.game === g.id);
          return html`<button class="list-row" data-game="${g.id}" ${gp?.locked ? 'aria-disabled="true"' : ''}>
            <div class="icon-tile ${gp ? 'brand' : ''}">${ic('gamepad')}</div>
            <div class="grow"><div class="h3">${g.name}</div><div class="small ${gp ? '' : 'faint'}">${gp ? gp.gamertag + (gp.inGameId ? ' · ID ' + gp.inGameId : '') : 'Not added'}</div></div>
            ${gp?.locked ? ic('lock', 'chev') : html`<span class="small brand-text">${gp ? 'Edit' : 'Add'}</span>`}</button>`;
        })}</div>
      </section>

      <section class="section">
        <div class="section-head"><h2 class="h2">Notifications</h2></div>
        <div class="list">
          <div class="list-row"><div class="icon-tile">${ic('bell')}</div>
            <div class="grow"><div class="h3">Match alerts</div><div class="small muted">${
              ps === 'install-first' ? 'Install the app to your Home Screen first (iPhone)' :
              ps === 'denied' ? 'Blocked in your browser settings' :
              ps === 'unsupported' ? 'Not supported on this browser' : 'Challenges, results, payouts and new tournaments'}</div></div>
            <label class="switch"><input type="checkbox" id="push" ${pushOn ? 'checked' : ''} ${ps === 'unsupported' || ps === 'denied' ? 'disabled' : ''} aria-label="Match alerts"><span></span></label>
          </div>
          <div class="list-row"><div class="icon-tile">${ic('file')}</div>
            <div class="grow"><div class="h3">Email updates</div><div class="small muted">Receipts and important account emails</div></div>
            <label class="switch"><input type="checkbox" id="email" ${p.emailEnabled ? 'checked' : ''} aria-label="Email updates"><span></span></label>
          </div>
        </div>
      </section>

      ${ref.code ? html`<section class="section">
        <div class="section-head"><h2 class="h2">Invite friends</h2><span class="small faint">${ref.referrals || 0} joined</span></div>
        <div class="card row between"><div><div class="tiny faint">YOUR CODE</div><div class="h2 num" style="letter-spacing:.12em">${ref.code}</div></div>
          <button class="btn outline sm" id="share">${ic('share')} Share</button></div>
      </section>` : ''}

      <section class="section">
        <div class="section-head"><h2 class="h2">App</h2></div>
        <div class="list">
          ${canInstall() ? html`<button class="list-row" id="install"><div class="icon-tile brand">${ic('phone')}</div>
            <div class="grow"><div class="h3">Install CrestArena</div><div class="small muted">${isIOS ? 'Add to your iPhone Home Screen' : 'Full-screen app with notifications'}</div></div>${ic('chevronRight', 'chev')}</button>` : ''}
          ${p.isAdmin ? html`<a class="list-row" href="/admin" style="color:inherit"><div class="icon-tile warn">${ic('shield')}</div><div class="grow h3">Admin dashboard</div>${ic('chevronRight', 'chev')}</a>` : ''}
          <a class="list-row" href="/watch" style="color:inherit"><div class="icon-tile">${ic('radio')}</div><div class="grow h3">Live streams</div>${ic('chevronRight', 'chev')}</a>
          <button class="list-row" id="update"><div class="icon-tile">${ic('refresh')}</div>
            <div class="grow"><div class="h3">Check for updates</div><div class="small muted">Version ${APP_VERSION}</div></div></button>
        </div>
      </section>

      <section class="section">
        <div class="section-head"><h2 class="h2">Privacy & legal</h2></div>
        <div class="list">
          ${[['Terms of Service', 'terms'], ['Privacy Policy', 'privacy'], ['Cookie Policy', 'cookies'], ['Responsible Gaming', 'responsible-gaming']].map(([t, f]) =>
            html`<a class="list-row" href="/legal/${f}.html" target="_blank" style="color:inherit"><div class="icon-tile">${ic('file')}</div><div class="grow h3">${t}</div>${ic('external', 'chev')}</a>`)}
          <button class="list-row" id="export"><div class="icon-tile">${ic('download')}</div>
            <div class="grow"><div class="h3">Download my data</div><div class="small muted">Everything we store about you, as a file</div></div></button>
          <button class="list-row" id="delete"><div class="icon-tile danger">${ic('trash')}</div>
            <div class="grow"><div class="h3 danger-text">Delete account</div><div class="small muted">Permanently remove your personal data</div></div></button>
        </div>
      </section>

      <button class="btn secondary block" id="logout" style="margin-top:24px">${ic('logout')} Sign out</button>
      <p class="center tiny faint" style="margin-top:16px">CrestArena ${APP_VERSION} · 18+ · Play responsibly</p>
    </div>`);

    view.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => {
      const gp = p.gameProfiles.find((x) => x.game === b.dataset.game);
      if (gp?.locked) return toast('This name is locked because you have played with it. Contact support to change it.', 'info');
      gameIdSheet(b.dataset.game, draw);
    }));

    view.querySelector('#verify-send')?.addEventListener('click', (e) => busy(e.currentTarget, async () => {
      await sendVerification();
      toast('Check your inbox (and spam folder) for the link, then come back and tap "I’ve verified".', 'ok', 'Link sent');
    }));
    view.querySelector('#verify-check')?.addEventListener('click', (e) => busy(e.currentTarget, async () => {
      if (!(await reloadVerification())) return toast('Not verified yet. Open the link in the email first.', 'info');
      await refreshProfile();
      toast(store.get().profile.isAdmin ? 'Email verified. Admin dashboard unlocked.' : 'Email verified.');
      draw();
    }));
    view.querySelector('#edit').onclick = () => editProfile(draw);
    view.querySelector('#photo').onclick = () => photoSheet(draw);
    view.querySelector('#push').onchange = async (e) => {
      const on = e.target.checked;
      if (on) {
        const ok = await enablePush();
        if (ok) await api('POST', '/user/settings/update', { pushEnabled: true }).catch(() => {});
        e.target.checked = ok;
      } else {
        await disablePush();
        await api('POST', '/user/settings/update', { pushEnabled: false }).catch(() => {});
        toast('Match alerts turned off', 'info');
      }
      refreshProfile();
    };
    view.querySelector('#email').onchange = (e) => api('POST', '/user/settings/update', { emailEnabled: e.target.checked }).then(refreshProfile).catch(toastError);
    view.querySelector('#share')?.addEventListener('click', () => {
      const text = `Join me on CrestArena and play FC Mobile, eFootball & DLS for prizes. Use my code ${ref.code}: ${location.origin}`;
      if (navigator.share) navigator.share({ title: 'CrestArena', text }).catch(() => {});
      else { navigator.clipboard?.writeText(text); toast('Invite copied to clipboard'); }
    });
    view.querySelector('#install')?.addEventListener('click', promptInstall);
    view.querySelector('#update').onclick = async () => {
      const reg = await navigator.serviceWorker?.getRegistration();
      await reg?.update().catch(() => {});
      const { version } = await get('/version').catch(() => ({}));
      if (version && version !== APP_VERSION) location.reload();
      else toast("You're on the latest version");
    };
    view.querySelector('#export').onclick = (e) => busy(e.currentTarget, async () => {
      const res = await api('GET', '/me/export', undefined, { raw: true });
      if (!res.ok) throw new Error('Could not prepare your data. Please try again.');
      const blob = await res.blob();
      const a = Object.assign(document.createElement('a'), { href: URL.createObjectURL(blob), download: 'crestarena-my-data.json' });
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 2000);
    });
    view.querySelector('#delete').onclick = () => deleteAccount();
    view.querySelector('#logout').onclick = () => signOut();
  }

  draw();
  if (query.section === 'games') view.querySelector('#games')?.scrollIntoView({ behavior: 'smooth' });
}

function editProfile(onDone) {
  const p = store.get().profile;
  sheet(String(html`${sheetHead('Edit profile')}
    <form id="ep"><label class="field"><span class="label">Display name</span>
      <input class="input" name="username" maxlength="24" minlength="3" required value="${p.username}"></label>
      <p class="hint">This is what other players see. Your in-game names are set under Game IDs.</p>
      <div class="sheet-actions"><button class="btn primary block" type="submit">Save</button></div></form>`), {
    label: 'Edit profile',
    onMount(root, close) {
      root.querySelector('#ep').onsubmit = (e) => {
        e.preventDefault();
        busy(e.target.querySelector('[type=submit]'), async () => {
          const { profile } = await put('/me', { username: new FormData(e.target).get('username') });
          store.set({ profile });
          close(); toast('Profile updated'); onDone();
        });
      };
    },
  });
}

function deleteAccount() {
  sheet(String(html`${sheetHead('Delete account')}
    <div class="notice danger">${ic('alert')}<div class="small"><b>This can't be undone.</b> Your profile, game IDs and notifications are deleted and you are signed out.
      We keep anonymised payment records only as long as the law requires.</div></div>
    <ul class="checklist" style="margin-top:14px">
      <li class="ok">${ic('check')}<span>Withdraw your balance first</span></li>
      <li class="ok">${ic('check')}<span>Finish or cancel open matches, challenges and withdrawals</span></li>
    </ul>
    <form id="del" style="margin-top:16px"><label class="field"><span class="label">Type DELETE to confirm</span>
      <input class="input" name="confirm" autocomplete="off" autocapitalize="characters"></label>
      <div class="sheet-actions"><button class="btn danger block" type="submit">Delete my account</button></div></form>`), {
    label: 'Delete account',
    onMount(root) {
      root.querySelector('#del').onsubmit = (e) => {
        e.preventDefault();
        if (new FormData(e.target).get('confirm') !== 'DELETE') return toast('Type DELETE to confirm', 'err');
        busy(e.target.querySelector('[type=submit]'), async () => {
          await del('/me');
          toast('Your account has been deleted.', 'info');
          setTimeout(() => signOut(), 1200);
        });
      };
    },
  });
}

// Crop to a centred square and shrink on the phone before uploading, so the
// upload is small and fast on mobile data. The server re-processes it anyway.
async function squareJpeg(file, size = 512) {
  const bitmap = await createImageBitmap(file).catch(() => null);
  if (!bitmap) throw new Error("We couldn't open that image. Try a JPG or PNG photo.");
  const side = Math.min(bitmap.width, bitmap.height);
  const canvas = Object.assign(document.createElement('canvas'), { width: size, height: size });
  canvas.getContext('2d').drawImage(bitmap, (bitmap.width - side) / 2, (bitmap.height - side) / 2, side, side, 0, 0, size, size);
  bitmap.close?.();
  return new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.9));
}

function photoSheet(onDone) {
  const p = store.get().profile;
  sheet(String(html`${sheetHead('Profile photo', 'Shown to other players on matches and challenges')}
    <div class="center" style="margin:6px 0 18px" id="preview">${avatar(p.username, p.avatarUrl, 'xl me')}</div>
    <input type="file" id="photo-file" accept="image/jpeg,image/png,image/webp" class="sr-only">
    <p class="hint center">Use a clear photo of yourself or your gaming logo. No offensive images; they'll be removed.</p>
    <div class="sheet-actions">
      <button class="btn primary block" id="pick">${ic('camera')} Choose photo</button>
      <button class="btn primary block" id="save" hidden>${ic('check')} Save photo</button>
      ${p.avatarUrl ? html`<button class="btn danger block" id="remove">${ic('trash')} Remove photo</button>` : ''}
      <button class="btn ghost block" data-close>Cancel</button>
    </div>`), {
    label: 'Profile photo',
    onMount(root, close) {
      const input = root.querySelector('#photo-file');
      const pick = root.querySelector('#pick');
      const save = root.querySelector('#save');
      let blob = null;
      pick.onclick = () => input.click();
      input.onchange = async () => {
        const file = input.files[0];
        if (!file) return;
        if (file.size > 20 * 1024 * 1024) return toast('That photo is too large', 'err');
        try {
          blob = await squareJpeg(file);
        } catch (e) { return toastError(e); }
        const url = URL.createObjectURL(blob);
        root.querySelector('#preview').innerHTML = String(html`<div class="avatar xl me"><img src="${url}" alt="New photo"></div>`);
        pick.innerHTML = String(html`${ic('image')} Choose a different photo`);
        pick.classList.replace('primary', 'secondary');
        save.hidden = false;
      };
      save.onclick = () => busy(save, async () => {
        const fd = new FormData();
        fd.append('image', blob, 'avatar.jpg');
        const { avatarUrl } = await api('PUT', '/me/avatar', fd, { form: true });
        store.set({ profile: { ...store.get().profile, avatarUrl } });
        close(); toast('Profile photo updated'); onDone();
      });
      root.querySelector('#remove')?.addEventListener('click', (e) => busy(e.currentTarget, async () => {
        await del('/me/avatar');
        store.set({ profile: { ...store.get().profile, avatarUrl: '' } });
        close(); toast('Profile photo removed'); onDone();
      }));
    },
  });
}
