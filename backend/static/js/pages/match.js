import { get, post, upload } from '../api.js';
import { store, gameName } from '../store.js';
import { html, ic, money, avatar, statusBadge, scoreLine, timeLeft, sheet, sheetHead, busy, toast, skeleton, empty, confirmSheet } from '../ui.js';
import { refreshProfile, refreshCurrent } from '../app.js';

const STEPS = ['Play', 'Result sent', 'Check', 'Final'];
const stepIndex = { ready: 0, submitted: 1, disputed: 2, review: 2, confirmed: 3, void: 3 };

export default async function matchPage(view, { id }) {
  view.innerHTML = String(html`<div class="page">${skeleton(260)}${skeleton(160)}</div>`);
  let m;

  async function load() {
    try {
      ({ match: m } = await get('/matches/' + encodeURIComponent(id)));
    } catch (e) {
      view.innerHTML = String(html`<div class="page">${empty('alert', 'Match not found', e.message, html`<a class="btn secondary" href="/play">Back to Play</a>`)}</div>`);
      return;
    }
    draw();
  }

  function draw() {
    const uid = store.get().profile.id;
    const mine = uid === m.homeId || uid === m.awayId;
    const opp = uid === m.homeId ? { name: m.awayName, tag: m.awayTag } : { name: m.homeName, tag: m.homeTag };
    const step = stepIndex[m.status] ?? 0;
    const prize = m.kind === 'p2p' ? Math.floor((m.pool * 80) / 100) : 0;

    view.innerHTML = String(html`<div class="page">
      <a href="${m.kind === 'p2p' ? '/play?tab=mine' : '/tournaments/' + m.tournamentId}" class="row small muted" style="gap:4px;margin-bottom:12px">${ic('chevronLeft')}${m.kind === 'p2p' ? 'My matches' : m.tournamentName}</a>

      <div class="card pad-lg">
        <div class="row between">
          <span class="eyebrow">${m.kind === 'p2p' ? '1v1' : 'Round ' + m.round} · ${gameName(m.game)}</span>
          ${statusBadge(m.status)}
        </div>
        <div class="versus" style="margin:22px 0 18px">
          <div class="side ${m.winnerId === m.homeId ? 'winner' : ''}">${avatar(m.homeName, m.homeAvatar, 'lg ' + (m.homeId === uid ? 'me' : ''))}
            <div class="name ellipsis">${m.homeId === uid ? 'You' : m.homeName}</div><div class="tag ellipsis">${m.homeTag}</div></div>
          <div class="mid">${scoreLine(m)}</div>
          <div class="side ${m.winnerId === m.awayId ? 'winner' : ''}">${avatar(m.awayName, m.awayAvatar, 'lg ' + (m.awayId === uid ? 'me' : ''))}
            <div class="name ellipsis">${m.awayId === uid ? 'You' : m.awayName}</div><div class="tag ellipsis">${m.awayTag}</div></div>
        </div>
        ${m.status !== 'void' ? html`<div class="steps" aria-label="Match progress">${STEPS.map((s, i) => html`<div class="step ${i < step ? 'done' : i === step ? (m.status === 'confirmed' ? 'done' : 'now') : ''}">${s}</div>`)}</div>` : ''}
      </div>

      <section class="section" id="action">${mine ? actionArea(m, uid, opp) : spectatorNote(m)}</section>

      <section class="section">
        <div class="list">
          ${m.kind === 'p2p' ? html`
            <div class="list-row"><div class="icon-tile">${ic('coins')}</div><div class="grow muted">Stake (each)</div><b class="num">${money(m.stake, m.currency)}</b></div>
            <div class="list-row"><div class="icon-tile brand">${ic('trophy')}</div><div class="grow muted">Winner gets</div><b class="num brand-text">${money(prize, m.currency)}</b></div>` : ''}
          <div class="list-row"><div class="icon-tile">${ic('hash')}</div><div class="grow muted">Match ID</div>
            <button class="row small" data-copy="${m.id}" style="gap:6px">${m.id.slice(-8).toUpperCase()} ${ic('copy')}</button></div>
          ${m.inGameMatchId ? html`<div class="list-row"><div class="icon-tile">${ic('gamepad')}</div><div class="grow muted">In-game match ID</div><span class="small">${m.inGameMatchId}</span></div>` : ''}
          <div class="list-row"><div class="icon-tile">${ic('clock')}</div><div class="grow muted">Created</div><span class="small">${new Date(m.createdAt).toLocaleString()}</span></div>
        </div>
      </section>
    </div>`);

    view.querySelector('[data-copy]')?.addEventListener('click', (e) => {
      navigator.clipboard?.writeText(e.currentTarget.dataset.copy);
      toast('Match ID copied');
    });
    bindActions(uid);
  }

  function actionArea(m, uid, opp) {
    switch (m.status) {
      case 'ready': return uploadArea(m, opp);
      case 'submitted':
        if (m.submittedBy === uid) {
          return html`<div class="notice warn">${ic('clock')}<div><b>Waiting for ${opp.name} to check the result</b>
            <div class="small">If they don't dispute, it becomes final in <span class="num" data-countdown="${m.disputeDeadline}">${timeLeft(m.disputeDeadline)}</span> and the money is paid automatically.</div></div></div>`;
        }
        return html`<div class="card highlight">
          <div class="h2">Is this result correct?</div>
          <p class="muted small" style="margin-top:6px">${opp.name} uploaded the result above. If it's right, confirm it. If it's wrong, dispute it and an admin will check.</p>
          <div class="notice warn" style="margin-top:14px">${ic('clock')}<div class="small">Becomes final automatically in <b class="num" data-countdown="${m.disputeDeadline}">${timeLeft(m.disputeDeadline)}</b></div></div>
          <div class="grid-2" style="margin-top:16px">
            <button class="btn danger" id="dispute">${ic('flag')} Dispute</button>
            <button class="btn primary" id="confirm">${ic('check')} Confirm</button>
          </div></div>`;
      case 'disputed':
        return html`<div class="notice danger">${ic('flag')}<div><b>Result disputed</b><div class="small">An admin is reviewing the screenshot. ${m.disputeReason ? html`Reason: “${m.disputeReason}”. ` : ''}You'll get a notification with the decision.</div></div></div>`;
      case 'review':
        return html`<div class="notice">${ic('shield')}<div><b>Under review</b><div class="small">${m.disputeReason || 'An admin is checking this match.'} You'll be notified of the decision.</div></div></div>`;
      case 'confirmed': {
        const won = m.winnerId === uid;
        const draw = !m.winnerId;
        const prize = m.kind === 'p2p' ? Math.floor((m.pool * 80) / 100) : 0;
        return html`<div class="notice ${won ? 'brand' : ''}">${ic(won ? 'trophy' : draw ? 'scale' : 'checkCircle')}<div>
          <b>${won ? 'You won!' : draw ? 'Draw' : 'Match finished'}</b>
          <div class="small">${won && prize ? html`${money(prize, m.currency)} was added to your wallet.` : draw && m.kind === 'p2p' ? 'Both stakes were refunded.' : won ? 'You advance in the tournament.' : 'Better luck next time. Ready for a rematch?'}</div></div></div>
          ${m.kind === 'p2p' ? html`<a class="btn secondary block" href="/play" style="margin-top:12px">${ic('swords')} Play again</a>` : ''}`;
      }
      case 'void':
        return html`<div class="notice">${ic('xCircle')}<div><b>Match cancelled</b><div class="small">${m.disputeReason || 'No result was submitted in time.'} Stakes were refunded.</div></div></div>`;
    }
    return '';
  }

  function uploadArea(m, opp) {
    const myTag = store.get().profile.gameProfiles.find((g) => g.game === m.game)?.gamertag;
    return html`<div class="card">
      <div class="row between"><div class="h2">Upload the result</div>
        ${m.playDeadline ? html`<span class="badge warn">${ic('clock')}<span class="num" data-countdown="${m.playDeadline}">${timeLeft(m.playDeadline)}</span></span>` : ''}</div>
      <p class="muted small" style="margin-top:6px">After the final whistle, take a screenshot of the <b>full-time result screen</b> and upload it here. Either player can upload.</p>

      <div class="notice brand" style="margin-top:14px">${ic('users')}<div class="small">The screenshot must show both names:
        <b>${myTag || 'your name'}</b> and <b>${opp.tag || opp.name}</b></div></div>

      <div class="grid-2" style="margin-top:14px">
        <ul class="checklist"><li class="ok">${ic('checkCircle')}<span>Full-time / final result screen</span></li>
          <li class="ok">${ic('checkCircle')}<span>Player vs player (H2H, friendly, online)</span></li>
          <li class="ok">${ic('checkCircle')}<span>Whole screen, not cropped</span></li></ul>
        <ul class="checklist"><li class="no">${ic('xCircle')}<span>Scenarios, events or vs AI</span></li>
          <li class="no">${ic('xCircle')}<span>Pause menu (Resume / Quit)</span></li>
          <li class="no">${ic('xCircle')}<span>Screens without usernames</span></li></ul>
      </div>

      <label class="dropzone" style="margin-top:16px" id="drop">
        <input type="file" accept="image/png,image/jpeg,image/webp" id="file" class="sr-only">
        <span id="drop-inner">${ic('upload')}<b>Choose screenshot</b><span class="small faint">PNG or JPG, up to 8 MB</span></span>
      </label>
      <div id="upload-error"></div>
      <button class="btn primary block" id="submit" style="margin-top:14px" disabled>${ic('zap')} Verify & submit result</button>
      <p class="hint center">Our AI reads the score and checks both names. Usually takes 5–15 seconds.</p>
    </div>`;
  }

  function spectatorNote(m) {
    return m.status === 'confirmed' ? '' : html`<div class="notice">${ic('eye')}<div class="small">You're viewing this match. Only the two players can submit or confirm the result.</div></div>`;
  }

  function bindActions() {
    const file = view.querySelector('#file');
    if (file) {
      let chosen = null;
      const submit = view.querySelector('#submit');
      const drop = view.querySelector('#drop');
      const pick = (f) => {
        if (!f) return;
        if (!/^image\/(png|jpeg|webp)$/.test(f.type)) return toast('Please choose a PNG or JPG screenshot', 'err');
        if (f.size > 8 * 1024 * 1024) return toast('That image is larger than 8 MB', 'err');
        chosen = f;
        const url = URL.createObjectURL(f);
        view.querySelector('#drop-inner').innerHTML = `<img src="${url}" alt="Selected screenshot"><span class="small faint">Tap to choose a different image</span>`;
        view.querySelector('#upload-error').innerHTML = '';
        submit.disabled = false;
      };
      file.onchange = () => pick(file.files[0]);
      ['dragover', 'dragenter'].forEach((ev) => drop.addEventListener(ev, (e) => { e.preventDefault(); drop.classList.add('drag'); }));
      ['dragleave', 'drop'].forEach((ev) => drop.addEventListener(ev, (e) => { e.preventDefault(); drop.classList.remove('drag'); }));
      drop.addEventListener('drop', (e) => pick(e.dataTransfer.files[0]));
      submit.onclick = () => {
        if (!chosen) return;
        submit.innerHTML = String(html`Reading the score…`);
        // Google can be slow when busy; tell the player we are still working.
        const slow = setTimeout(() => { submit.innerHTML = String(html`Still reading… retrying, please wait`); }, 15000);
        busy(submit, async () => {
          try {
            const fd = new FormData();
            fd.append('image', await shrinkScreenshot(chosen), 'result.jpg');
            const res = await upload(`/matches/${m.id}/result`, fd);
            m = res.match;
            toast('Result submitted. Your opponent has 15 minutes to check it.', 'ok', 'Score verified');
            refreshCurrent();
            draw();
          } catch (e) {
            view.querySelector('#upload-error').innerHTML = String(html`<div class="notice danger" style="margin-top:14px">${ic('alert')}<div><b>Screenshot not accepted</b><div class="small">${e.message}</div></div></div>`);
            submit.innerHTML = String(html`${ic('zap')} Try again`);
          } finally {
            clearTimeout(slow);
          }
        });
      };
    }

    view.querySelector('#confirm')?.addEventListener('click', async (e) => {
      const btn = e.currentTarget;
      const ok = await confirmSheet({ title: 'Confirm this result?', message: 'This makes the result final straight away and settles the match. You can’t undo it.', confirm: 'Yes, it’s correct' });
      if (!ok) return;
      busy(btn, async () => {
        ({ match: m } = await post(`/matches/${m.id}/confirm`));
        toast('Result confirmed');
        refreshProfile(); refreshCurrent(); draw();
      });
    });

    view.querySelector('#dispute')?.addEventListener('click', () => {
      sheet(String(html`${sheetHead('Dispute result', 'An admin will check both sides')}
        <form id="dsp"><label class="field"><span class="label">What's wrong?</span>
          <textarea class="textarea" name="reason" maxlength="500" required placeholder="e.g. The real score was 2–3. The screenshot is from a different match."></textarea></label>
          <div class="notice warn" style="margin-top:12px">${ic('alert')}<div class="small">False disputes slow everyone down and may lead to account restrictions.</div></div>
          <div class="sheet-actions"><button class="btn danger block" type="submit">Send dispute</button></div></form>`), {
        label: 'Dispute',
        onMount(root, close) {
          root.querySelector('#dsp').onsubmit = (e) => {
            e.preventDefault();
            busy(e.target.querySelector('[type=submit]'), async () => {
              ({ match: m } = await post(`/matches/${m.id}/dispute`, { reason: new FormData(e.target).get('reason') }));
              close();
              toast('Dispute sent. An admin will review it.', 'info');
              refreshCurrent(); draw();
            });
          };
        },
      });
    });
  }

  const onUpdate = (e) => { if (e.detail.id === id) { m = e.detail; if (!view.querySelector('#file')?.files?.length) draw(); } };
  document.addEventListener('match_update', onUpdate);
  await load();
  return () => document.removeEventListener('match_update', onUpdate);
}

// Phone screenshots are often 2–5 MB PNGs. Re-encode large ones as a JPEG
// no wider than 1920px: still sharp enough to read names and scores, but much
// faster to upload on mobile data and quicker for the AI to process.
export async function shrinkScreenshot(file) {
  const MAX = 1920;
  if (file.size < 1024 * 1024 && file.type === 'image/jpeg') return file;
  const bitmap = await createImageBitmap(file).catch(() => null);
  if (!bitmap) return file;
  const scale = Math.min(1, MAX / Math.max(bitmap.width, bitmap.height));
  if (scale === 1 && file.size < 1024 * 1024) { bitmap.close?.(); return file; }
  const canvas = Object.assign(document.createElement('canvas'), {
    width: Math.round(bitmap.width * scale), height: Math.round(bitmap.height * scale),
  });
  canvas.getContext('2d').drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close?.();
  const blob = await new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.9));
  return blob && blob.size < file.size ? blob : file;
}
