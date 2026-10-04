// Skeleton screens: grey placeholders in the shape of the content that is
// loading, with a shimmer. Each page has its own so the layout doesn't jump.
import { raw } from './ui.js';

const line = (w = '100%', h = 12, extra = '') => `<span class="sk" style="width:${w};height:${h}px;${extra}"></span>`;
const circle = (s = 44) => `<span class="sk sk-circle" style="width:${s}px;height:${s}px"></span>`;
const pill = (w = 90, h = 24) => `<span class="sk sk-pill" style="width:${w}px;height:${h}px"></span>`;
const box = (h, extra = '') => `<span class="sk" style="width:100%;height:${h}px;border-radius:14px;${extra}"></span>`;
const wrap = (inner) => raw(`<div class="sk-wrap" aria-busy="true" aria-label="Loading">${inner}</div>`);
const page = (inner) => wrap(`<div class="page">${inner}</div>`);
const head = (w = '45%') => `<div class="sk-head">${line(w, 22)}</div>`;

// ── Building blocks ────────────────────────────────────────────
const matchCard = () => `<div class="card">
  <div class="row between">${line('40%', 10)}${pill(86)}</div>
  <div class="sk-versus">
    <div class="sk-side">${circle(56)}${line('70%', 12)}${line('50%', 10)}</div>
    ${line('64px', 36)}
    <div class="sk-side">${circle(56)}${line('70%', 12)}${line('50%', 10)}</div>
  </div>
  ${box(58)}
</div>`;

const listRows = (n = 3, { avatar = true, action = false } = {}) => `<div class="list">${Array.from({ length: n }, () => `
  <div class="list-row">${avatar ? circle(44) : `<span class="sk" style="width:40px;height:40px;border-radius:12px"></span>`}
    <div class="grow sk-stack">${line('55%', 14)}${line('35%', 10)}</div>
    ${action ? `<span class="sk" style="width:76px;height:38px;border-radius:12px"></span>` : line('56px', 14)}
  </div>`).join('')}</div>`;

const tournamentCard = () => `<div class="card">
  <div class="row between">${pill(92)}${pill(96)}</div>
  ${line('60%', 18, 'margin-top:14px')}${line('30%', 10, 'margin-top:8px')}
  <div class="grid-3" style="margin-top:16px">${[0, 1, 2].map(() => `<div class="sk-stack">${line('60%', 8)}${line('80%', 16)}</div>`).join('')}</div>
  ${line('100%', 6, 'margin-top:14px')}
</div>`;

const stats3 = () => `<div class="grid-3">${[0, 1, 2].map(() => `<div class="stat sk-stack">${line('50%', 22)}${line('70%', 8)}</div>`).join('')}</div>`;

// ── Pages ──────────────────────────────────────────────────────
export const sk = {
  home: () => page(`${line('28%', 10)}${line('45%', 26, 'margin-top:10px')}
    <div class="section">${head('35%')}${matchCard()}</div>
    <div class="section">${head('45%')}${tournamentCard()}</div>
    <div class="section">${head('30%')}${listRows(3, { avatar: false })}</div>`),

  play: () => page(`${line('40%', 28)}${line('75%', 12, 'margin-top:10px')}
    ${box(48, 'margin-top:16px')}${box(48, 'margin-top:20px')}
    <div class="sk-chips">${pill(96, 38)}${pill(110, 38)}${pill(100, 38)}</div>
    <div style="margin-top:14px">${listRows(4, { action: true })}</div>`),

  lobby: () => wrap(`<div class="sk-chips" style="margin-top:0">${pill(96, 38)}${pill(110, 38)}${pill(100, 38)}</div>
    <div style="margin-top:14px">${listRows(4, { action: true })}</div>`),

  matchList: () => wrap(`${matchCard()}<div style="margin-top:20px">${listRows(3, { avatar: false })}</div>`),

  match: () => page(`${line('30%', 12)}<div style="margin-top:12px">${matchCard()}</div>
    <div class="section"><div class="card">${line('50%', 20)}${line('90%', 12, 'margin-top:10px')}${box(56, 'margin-top:14px')}${box(180, 'margin-top:14px')}${box(48, 'margin-top:14px')}</div></div>
    <div class="section">${listRows(3, { avatar: false })}</div>`),

  tournaments: () => page(`${line('45%', 28)}${line('80%', 12, 'margin-top:10px')}
    ${box(48, 'margin-top:18px')}<div class="sk-chips">${pill(96, 38)}${pill(90, 38)}${pill(90, 38)}</div>
    <div class="stack" style="margin-top:14px">${tournamentCard()}${tournamentCard()}</div>`),

  tournamentList: () => wrap(`<div class="stack">${tournamentCard()}${tournamentCard()}${tournamentCard()}</div>`),

  tournament: () => page(`${line('30%', 12)}<div class="card pad-lg" style="margin-top:12px">
      <div class="row">${pill(80)}${pill(80)}</div>${line('65%', 28, 'margin-top:14px')}
      <div style="margin-top:18px">${stats3()}</div>${box(48, 'margin-top:18px')}</div>
    ${box(48, 'margin-top:20px')}<div style="margin-top:16px">${listRows(4)}</div>`),

  wallet: () => page(`<div class="wallet-card sk-wallet">${line('40%', 10)}${line('60%', 40, 'margin-top:12px')}
      <div class="actions">${box(48)}${box(48)}</div></div>
    <div class="section">${head('25%')}${listRows(5, { avatar: false })}</div>`),

  profile: () => page(`<div class="row" style="gap:16px">${circle(72)}<div class="grow sk-stack">${line('50%', 20)}${line('70%', 12)}${line('40%', 10)}</div></div>
    <div style="margin-top:20px">${stats3()}</div>
    <div class="section">${head('30%')}${listRows(3, { avatar: false })}</div>
    <div class="section">${head('35%')}${listRows(2, { avatar: false })}</div>`),

  notifications: () => page(`${line('50%', 28)}<div style="margin-top:16px">${listRows(6, { avatar: false })}</div>`),

  admin: () => page(`${line('25%', 28)}<div class="sk-chips">${pill(90, 38)}${pill(80, 38)}${pill(110, 38)}${pill(110, 38)}</div>
    <div class="grid-2" style="margin-top:16px">${Array.from({ length: 6 }, () => `<div class="stat sk-stack">${line('50%', 22)}${line('70%', 8)}</div>`).join('')}</div>`),

  adminRows: () => wrap(listRows(4, { avatar: false, action: true })),

  watch: () => page(`<div class="row between">${line('25%', 28)}${pill(100, 38)}</div><div class="stack" style="margin-top:18px">${listRows(3, { avatar: false })}</div>`),

  tournamentCards: (n = 1) => wrap(`<div class="stack">${Array.from({ length: n }, tournamentCard).join('')}</div>`),
  rows: (n = 3) => wrap(listRows(n, { avatar: false })),

  // The whole app while signing in / waking the server.
  shell: () => raw(`<div class="app sk-wrap" aria-busy="true" aria-label="Loading CrestArena">
    <header class="appbar"><img class="brand" src="/icons/wordmark.png" alt="CrestArena" height="22"><div class="spacer"></div>${circle(36)}<span class="sk sk-pill" style="width:110px;height:38px"></span></header>
    <div class="page">${line('28%', 10)}${line('45%', 26, 'margin-top:10px')}<div class="section">${matchCard()}</div><div class="section">${tournamentCard()}</div>
      <p class="center small muted sk-wake" hidden>Waking up CrestArena… this can take up to a minute the first time.</p></div>
    <nav class="tabbar">${Array.from({ length: 5 }, () => `<div class="tab">${circle(24)}${line('36px', 8)}</div>`).join('')}</nav>
  </div>`),
};
