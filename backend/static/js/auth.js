// Firebase is used for sign-in only. Money and profile live in the Go API.
import { html, ic, $, busy, toast } from './ui.js';
import { pendingInvite, setPendingInvite } from './invite.js';

// The server stamps FIREBASE_AUTH_DOMAIN into index.html. Serving the sign-in
// pages from our own domain keeps Google sign-in working in browsers that block
// third-party storage (Safari, Chrome, Firefox, installed apps).
const stampedAuthDomain = document.querySelector('meta[name=crest-auth-domain]')?.content || '';
const authDomain = stampedAuthDomain && !stampedAuthDomain.startsWith('__') ? stampedAuthDomain : 'betelite-crest.firebaseapp.com';

// Google blocks sign-in inside in-app browsers (WhatsApp, Instagram, Facebook, TikTok…).
const inAppBrowser = /FBAN|FBAV|FB_IAB|Instagram|WhatsApp|Snapchat|TikTok|musical_ly|Line\/|Twitter|LinkedInApp|; wv\)/i.test(navigator.userAgent);
const isAndroid = /android/i.test(navigator.userAgent);
const isStandalone = matchMedia('(display-mode: standalone)').matches || navigator.standalone === true;

const firebaseConfig = {
  apiKey: 'AIzaSyCBMcP7ikJCYVkNjGksvglVkVp0ZTvsz0w',
  authDomain,
  projectId: 'betelite-crest',
  storageBucket: 'betelite-crest.firebasestorage.app',
  messagingSenderId: '889436668560',
  appId: '1:889436668560:web:f8b88a780bd83530170e15',
};

let auth = null;

export function initAuth(onUser) {
  firebase.initializeApp(firebaseConfig);
  auth = firebase.auth();
  auth.getRedirectResult().catch((e) => e?.code && toast(friendly(e), 'err'));
  auth.onAuthStateChanged(onUser);
}

export const currentUser = () => auth?.currentUser || null;
export const getToken = async () => (auth?.currentUser ? auth.currentUser.getIdToken() : null);
export const signOut = () => {
  if (sessionStorage.getItem('crestarena.dev')) { sessionStorage.removeItem('crestarena.dev'); location.href = '/'; return; }
  return auth.signOut();
};

function friendly(e) {
  const map = {
    'auth/invalid-credential': 'Wrong email or password.',
    'auth/wrong-password': 'Wrong email or password.',
    'auth/user-not-found': 'No account found with that email.',
    'auth/email-already-in-use': 'An account with this email already exists. Sign in instead.',
    'auth/weak-password': 'Use a stronger password (at least 8 characters).',
    'auth/invalid-email': 'Enter a valid email address.',
    'auth/too-many-requests': 'Too many attempts. Please wait a minute and try again.',
    'auth/network-request-failed': "You're offline. Check your connection.",
    'auth/popup-closed-by-user': 'Google sign-in was cancelled.',
    'auth/cancelled-popup-request': 'Google sign-in was cancelled.',
    'auth/unauthorized-domain': "Google sign-in isn't enabled for this web address yet. Use email for now.",
    'auth/internal-error': 'Google sign-in failed. Try again, or open CrestArena in Chrome or Safari.',
    'auth/web-storage-unsupported': 'Your browser is blocking sign-in. Turn off private mode or open CrestArena in Chrome or Safari.',
    'auth/account-exists-with-different-credential': 'This email already has an account. Sign in with your email and password.',
  };
  return map[e?.code] || e?.message || 'Something went wrong. Please try again.';
}

export function renderAuth(root, mode = pendingInvite() ? 'signup' : 'signin') {
  const signup = mode === 'signup';
  const invite = pendingInvite();
  root.innerHTML = String(html`
  <main class="auth">
    <img class="wordmark" src="/icons/wordmark.png" alt="CrestArena" height="34">
    <p class="tagline">Play. Prove it. Get paid.</p>
    ${invite ? html`<div class="notice brand" style="margin:-12px 0 18px">${ic('users')}<div class="small"><b>You've been invited!</b>
      Create your account and invite code <b>${invite}</b> is applied automatically.</div></div>` : ''}

    <div class="seg" role="tablist" style="margin-bottom:20px">
      <button role="tab" class="${signup ? '' : 'active'}" data-mode="signin">Sign in</button>
      <button role="tab" class="${signup ? 'active' : ''}" data-mode="signup">Create account</button>
    </div>

    <form id="auth-form" novalidate>
      ${signup ? html`<label class="field"><span class="label">Display name</span>
        <input class="input" name="name" autocomplete="nickname" maxlength="24" placeholder="What players will see" required></label>` : ''}
      <label class="field"><span class="label">Email</span>
        <input class="input" name="email" type="email" autocomplete="email" inputmode="email" placeholder="you@example.com" required></label>
      <label class="field"><span class="label">Password</span>
        <input class="input" name="password" type="password" autocomplete="${signup ? 'new-password' : 'current-password'}" placeholder="${signup ? 'At least 8 characters' : 'Your password'}" required></label>
      ${signup ? html`<label class="field"><span class="label">Invite code <span class="faint">(optional)</span></span>
        <input class="input" name="invite" maxlength="12" autocapitalize="characters" autocomplete="off" value="${invite}" placeholder="e.g. UT802R6Q"></label>` : ''}
      <div class="form-error" id="auth-error" role="alert" hidden></div>
      <button class="btn primary block" style="margin-top:20px" type="submit">${signup ? 'Create account' : 'Sign in'}</button>
      ${signup ? '' : html`<button class="btn ghost block" type="button" data-forgot style="margin-top:6px">Forgot password?</button>`}
    </form>

    <div class="or">or</div>
    <button class="btn secondary block oauth" data-google>${ic('google')} Continue with Google</button>
    ${inAppBrowser ? html`<div class="notice warn" style="margin-top:12px">${ic('alert')}<div class="small">
      <b>Google sign-in doesn't work inside this app.</b> Open CrestArena in ${isAndroid ? 'Chrome' : 'Safari'} to use Google, or sign in with email above.
      <div class="row" style="gap:8px;margin-top:10px">
        ${isAndroid ? html`<a class="btn primary sm" href="intent://${location.host}${location.pathname}#Intent;scheme=https;package=com.android.chrome;end">Open in Chrome</a>` : ''}
        <button class="btn secondary sm" type="button" data-copy-link>${ic('copy')} Copy link</button></div></div></div>` : ''}

    <p class="legal-foot">By continuing you agree to our <a href="/legal/terms.html" target="_blank">Terms</a> and
      <a href="/legal/privacy.html" target="_blank">Privacy Policy</a>. 18+ only. Play responsibly.</p>
    <p class="center" style="margin-top:14px"><a class="made-by" href="https://webcrest.app" target="_blank" rel="noopener">Designed by <span class="wc"><span class="wc-w">W</span><span class="wc-rest">ebCrest</span></span></a></p>
  </main>`);

  root.querySelectorAll('[data-mode]').forEach((b) => (b.onclick = () => renderAuth(root, b.dataset.mode)));
  const err = $('#auth-error', root);
  const showErr = (e) => { err.textContent = friendly(e); err.hidden = false; };

  $('#auth-form', root).onsubmit = (e) => {
    e.preventDefault();
    err.hidden = true;
    const f = new FormData(e.target);
    const email = String(f.get('email')).trim();
    const password = String(f.get('password'));
    busy(e.target.querySelector('[type=submit]'), async () => {
      try {
        if (signup) {
          setPendingInvite(String(f.get('invite') || ''));
          const name = String(f.get('name') || '').trim();
          if (name.length < 3) throw { message: 'Display name must be at least 3 characters.' };
          if (password.length < 8) throw { code: 'auth/weak-password' };
          const cred = await auth.createUserWithEmailAndPassword(email, password);
          await cred.user.updateProfile({ displayName: name });
          await cred.user.getIdToken(true);
          cred.user.sendEmailVerification().catch(() => {});
        } else {
          await auth.signInWithEmailAndPassword(email, password);
        }
      } catch (e2) { showErr(e2); }
    });
  };

  root.querySelector('[data-forgot]')?.addEventListener('click', async () => {
    const email = String(new FormData($('#auth-form', root)).get('email')).trim();
    if (!email) { showErr({ message: 'Enter your email above, then tap "Forgot password?"' }); return; }
    try {
      await auth.sendPasswordResetEmail(email);
      toast('Check your inbox for a password reset link.', 'ok', 'Email sent');
    } catch (e) { showErr(e); }
  });

  root.querySelector('[data-google]').onclick = async (e) => {
    const typed = root.querySelector('[name=invite]')?.value;
    if (typed) setPendingInvite(typed);
    const provider = new firebase.auth.GoogleAuthProvider();
    provider.setCustomParameters({ prompt: 'select_account' });
    await busy(e.currentTarget, async () => {
      try {
        // Installed apps can't open popups reliably; redirect when the sign-in pages are on our domain.
        if (isStandalone && authDomain === location.host) return await auth.signInWithRedirect(provider);
        await auth.signInWithPopup(provider);
      } catch (e2) {
        if (['auth/popup-blocked', 'auth/operation-not-supported-in-this-environment', 'auth/web-storage-unsupported'].includes(e2?.code)) {
          await auth.signInWithRedirect(provider);
        } else if (e2?.code !== 'auth/popup-closed-by-user' && e2?.code !== 'auth/cancelled-popup-request') showErr(e2);
      }
    });
  };
  root.querySelector('[data-copy-link]')?.addEventListener('click', async () => {
    try { await navigator.clipboard.writeText(location.href); toast('Link copied. Paste it in your browser.'); }
    catch { toast(location.href, 'info'); }
  });
}

// Email verification (needed for admin access; recommended for withdrawals).
export async function sendVerification() {
  if (!auth?.currentUser) throw new Error('Please sign in again.');
  await auth.currentUser.sendEmailVerification();
}

// Re-reads the account after the user clicked the link and refreshes the
// ID token so the server sees email_verified = true.
export async function reloadVerification() {
  if (!auth?.currentUser) return false;
  await auth.currentUser.reload();
  await auth.currentUser.getIdToken(true);
  return auth.currentUser.emailVerified;
}
