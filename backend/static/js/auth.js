// Firebase is used for sign-in only. Money and profile live in the Go API.
import { html, ic, $, busy, toast } from './ui.js';

const firebaseConfig = {
  apiKey: 'AIzaSyCBMcP7ikJCYVkNjGksvglVkVp0ZTvsz0w',
  authDomain: 'betelite-crest.firebaseapp.com',
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
  };
  return map[e?.code] || e?.message || 'Something went wrong. Please try again.';
}

export function renderAuth(root, mode = 'signin') {
  const signup = mode === 'signup';
  root.innerHTML = String(html`
  <main class="auth">
    <img class="wordmark" src="/icons/wordmark.png" alt="CrestArena" height="34">
    <p class="tagline">Play. Prove it. Get paid.</p>

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
      <div class="form-error" id="auth-error" role="alert" hidden></div>
      <button class="btn primary block" style="margin-top:20px" type="submit">${signup ? 'Create account' : 'Sign in'}</button>
      ${signup ? '' : html`<button class="btn ghost block" type="button" data-forgot style="margin-top:6px">Forgot password?</button>`}
    </form>

    <div class="or">or</div>
    <button class="btn secondary block oauth" data-google>${ic('google')} Continue with Google</button>

    <p class="legal-foot">By continuing you agree to our <a href="/legal/terms.html" target="_blank">Terms</a> and
      <a href="/legal/privacy.html" target="_blank">Privacy Policy</a>. 18+ only. Play responsibly.</p>
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
    const provider = new firebase.auth.GoogleAuthProvider();
    await busy(e.currentTarget, async () => {
      try {
        await auth.signInWithPopup(provider);
      } catch (e2) {
        if (e2?.code === 'auth/popup-blocked' || e2?.code === 'auth/operation-not-supported-in-this-environment') {
          await auth.signInWithRedirect(provider);
        } else showErr(e2);
      }
    });
  };
}
