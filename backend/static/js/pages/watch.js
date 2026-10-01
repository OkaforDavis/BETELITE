import { get, post, del } from '../api.js';
import { store, gameName } from '../store.js';
import { html, ic, timeAgo, empty, skeleton, sheet, sheetHead, busy, toast, toastError, esc } from '../ui.js';
import { gameChips } from '../components.js';
import { navigate } from '../router.js';
import * as rt from '../realtime.js';

const LIVEKIT_SRC = 'https://cdn.jsdelivr.net/npm/livekit-client@2/dist/livekit-client.umd.min.js';
const canShareScreen = !!navigator.mediaDevices?.getDisplayMedia && !/android|iphone|ipad/i.test(navigator.userAgent);

export default async function watch(view, { id }) {
  return id ? room(view, id) : list(view);
}

async function list(view) {
  view.innerHTML = String(html`<div class="page">
    <div class="row between"><div><div class="h1">Live</div><div class="muted small" style="margin-top:4px">Watch players compete in real time.</div></div>
      <button class="btn primary sm" id="golive">${ic('radio')} Go live</button></div>
    <div id="streams" style="margin-top:18px">${skeleton(90, 3)}</div></div>`);
  view.querySelector('#golive').onclick = goLive;

  const draw = async () => {
    const { streams } = await get('/streams');
    const el = view.querySelector('#streams');
    if (!el) return;
    el.innerHTML = String(streams.length ? html`<div class="stack">${streams.map((s) => html`
      <a class="card tap row" href="/watch/${s.id}" style="color:inherit">
        <div class="icon-tile danger">${ic('radio')}</div>
        <div class="grow"><div class="h3 ellipsis">${s.title}</div><div class="small muted ellipsis">${s.hostName}${s.game ? ' · ' + gameName(s.game) : ''} · started ${timeAgo(s.startedAt)}</div></div>
        <span class="badge danger live">Live</span></a>`)}</div>`
      : html`<div class="card flat">${empty('tv', 'Nobody is live right now', 'Start a stream and your match will show up here for others to watch.')}</div>`);
  };
  await draw();
  const offs = [rt.on('stream_start', draw), rt.on('stream_end', draw)];
  return () => offs.forEach((f) => f());
}

function goLive() {
  let game = store.get().profile.gameProfiles[0]?.game || '';
  sheet(String(html`${sheetHead('Go live', 'Stream your match to the CrestArena community')}
    <form id="gl"><label class="field"><span class="label">Title</span><input class="input" name="title" maxlength="60" placeholder="e.g. Weekend Cup semi-final" required></label>
      <div class="field"><span class="label">Game</span><div id="g">${gameChips(game)}</div></div>
      <div class="field"><span class="label">Source</span><div class="seg" id="src">
        <button type="button" data-src="camera" class="active">${ic('camera')} Camera</button>
        <button type="button" data-src="screen" ${canShareScreen ? '' : 'disabled'}>${ic('monitor')} Screen</button></div>
        ${canShareScreen ? '' : html`<p class="hint">Phone browsers can't share the screen. Use your camera, or go live from a computer to share your screen.</p>`}</div>
      <div class="sheet-actions"><button class="btn primary block" type="submit">${ic('radio')} Start streaming</button></div></form>`), {
    label: 'Go live',
    onMount(root, close) {
      let source = 'camera';
      const bindG = () => root.querySelectorAll('[data-game]').forEach((b) => (b.onclick = () => { game = b.dataset.game; root.querySelector('#g').innerHTML = String(gameChips(game)); bindG(); }));
      bindG();
      root.querySelectorAll('[data-src]').forEach((b) => (b.onclick = () => { source = b.dataset.src; root.querySelectorAll('[data-src]').forEach((x) => x.classList.toggle('active', x === b)); }));
      root.querySelector('#gl').onsubmit = (e) => {
        e.preventDefault();
        busy(e.target.querySelector('[type=submit]'), async () => {
          const { stream } = await post('/streams', { title: new FormData(e.target).get('title'), game });
          close();
          sessionStorage.setItem('crestarena.stream-source', source);
          navigate('/watch/' + stream.id);
        });
      };
    },
  });
}

async function room(view, id) {
  view.innerHTML = String(html`<div class="page">
    <a href="/watch" class="row small muted" style="gap:4px;margin-bottom:12px">${ic('chevronLeft')}Live</a>
    <div class="video-wrap"><video id="video" playsinline autoplay muted></video></div>
    <div class="row between" style="margin-top:12px"><div id="meta" class="grow"></div>
      <button class="btn secondary sm" id="unmute" hidden>${ic('zap')} Tap for sound</button>
      <button class="btn danger sm" id="end" hidden>End stream</button></div>
    <div class="card" style="margin-top:16px;padding:0"><div class="chat" id="chat" aria-live="polite"></div>
      <form id="say" class="row" style="padding:10px;border-top:1px solid var(--line)">
        <input class="input grow" name="m" maxlength="300" placeholder="Say something nice…" autocomplete="off" aria-label="Chat message">
        <button class="icon-btn" aria-label="Send">${ic('send')}</button></form></div>
  </div>`);

  const video = view.querySelector('#video');
  const chat = view.querySelector('#chat');
  let lk = null;

  try {
    await loadScript(LIVEKIT_SRC);
    const { token, isHost, url } = await post(`/streams/${id}/token`);
    const { streams } = await get('/streams');
    const s = streams.find((x) => x.id === id);
    view.querySelector('#meta').innerHTML = String(html`<div class="h3 ellipsis">${s?.title || 'Live'}</div><div class="small muted">${s?.hostName || ''} <span class="badge danger live" style="margin-left:6px">Live</span></div>`);

    const L = window.LivekitClient;
    lk = new L.Room({ adaptiveStream: true, dynacast: true });
    lk.on(L.RoomEvent.TrackSubscribed, (track) => {
      if (track.kind === 'video') track.attach(video);
      if (track.kind === 'audio') { track.attach(video); view.querySelector('#unmute').hidden = false; }
    });
    lk.on(L.RoomEvent.Disconnected, () => toast('The stream has ended', 'info'));
    await lk.connect(url, token);

    if (isHost) {
      const source = sessionStorage.getItem('crestarena.stream-source') || 'camera';
      if (source === 'screen') await lk.localParticipant.setScreenShareEnabled(true, { audio: true });
      else await lk.localParticipant.setCameraEnabled(true);
      await lk.localParticipant.setMicrophoneEnabled(true);
      const pub = [...lk.localParticipant.videoTrackPublications.values()][0];
      pub?.track?.attach(video);
      const end = view.querySelector('#end');
      end.hidden = false;
      end.onclick = async () => { await del('/streams/' + id).catch(() => {}); lk.disconnect(); navigate('/watch'); };
    }
  } catch (e) {
    toastError(e);
  }

  view.querySelector('#unmute').onclick = (e) => { video.muted = false; video.play(); e.currentTarget.hidden = true; };

  const roomName = 'stream:' + id;
  rt.joinRoom(roomName);
  const off = rt.on('chat_message', (d) => {
    if (d.room !== roomName) return;
    const el = document.createElement('div');
    el.className = 'msg';
    el.innerHTML = `<b>${esc(d.sender)}</b>${esc(d.message)}`;
    chat.append(el);
    chat.scrollTop = chat.scrollHeight;
  });
  view.querySelector('#say').onsubmit = (e) => {
    e.preventDefault();
    const input = e.target.m;
    if (!input.value.trim()) return;
    rt.send('chat_message', { room: roomName, message: input.value });
    input.value = '';
  };

  return () => { off(); rt.leaveRoom(roomName); lk?.disconnect(); };
}

function loadScript(src) {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) return resolve();
    const s = Object.assign(document.createElement('script'), { src, onload: resolve, onerror: () => reject(new Error('Could not load the video player')) });
    document.head.append(s);
  });
}
