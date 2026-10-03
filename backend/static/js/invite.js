// Invite links: /join/CODE (or ?ref=CODE). The code is remembered on this
// device until the visitor signs up, then applied automatically.
import { get, post } from './api.js';
import { money, toast } from './ui.js';

const KEY = 'crestarena.invite';
const VALID = /^[A-Z0-9]{6,12}$/;

export function captureInvite() {
  const m = location.pathname.match(/^\/join\/([A-Za-z0-9]+)\/?$/);
  const code = (m?.[1] || new URLSearchParams(location.search).get('ref') || '').toUpperCase();
  if (VALID.test(code)) {
    try { localStorage.setItem(KEY, code); } catch { /* private mode */ }
  }
  if (m) history.replaceState({}, '', '/');
}

export function pendingInvite() {
  try { return localStorage.getItem(KEY) || ''; } catch { return ''; }
}

export function setPendingInvite(code) {
  try {
    code = (code || '').trim().toUpperCase();
    if (VALID.test(code)) localStorage.setItem(KEY, code);
  } catch { /* ignore */ }
}

function clearInvite() {
  try { localStorage.removeItem(KEY); } catch { /* ignore */ }
}

export function inviteLink(code) {
  return `${location.origin}/join/${code}`;
}

// After sign-in: apply a remembered code if this account can still use one.
export async function applyPendingInvite() {
  const code = pendingInvite();
  if (!code) return;
  try {
    const info = await get('/referrals');
    if (!info.canClaim || info.code === code) { clearInvite(); return; }
    const { referredBy } = await post('/referrals/claim', { code });
    clearInvite();
    toast(`Play your first paid match to get your ${money(info.friendReward, info.currency)} welcome bonus.`, 'ok', `You joined with ${referredBy}'s invite`);
  } catch (e) {
    clearInvite();
    if (e.status === 404) toast("That invite link isn't valid any more, but you're all set to play.", 'info');
  }
}
