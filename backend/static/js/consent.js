// Legal gates: cookie notice (essential storage only) and Terms + 18+ consent.
import { post, put } from './api.js';
import { html, ic, sheet, busy, toast } from './ui.js';

const COOKIE_KEY = 'crestarena.cookie-notice.v1';

// We only use strictly necessary storage (sign-in session, app cache,
// preferences). No advertising or analytics cookies, so this is a notice with
// a link to the policy rather than an opt-in for tracking.
export function cookieNotice(standalone = false) {
  try { if (localStorage.getItem(COOKIE_KEY)) return; } catch { return; }
  const el = document.createElement('div');
  if (document.querySelector('.banner.cookie')) return;
  el.className = 'banner cookie' + (standalone ? ' standalone' : '');
  el.setAttribute('role', 'region');
  el.setAttribute('aria-label', 'Cookie notice');
  el.innerHTML = String(html`<div class="icon-tile">${ic('shield')}</div>
    <div class="grow"><b>Only essential cookies</b>
    <span>We use storage only to keep you signed in and the app working. No ads or tracking. <a href="/legal/cookies.html" target="_blank">Cookie Policy</a></span></div>
    <button class="btn secondary sm" data-ok>OK</button>`);
  el.querySelector('[data-ok]').onclick = () => {
    try { localStorage.setItem(COOKIE_KEY, new Date().toISOString()); } catch { /* private mode */ }
    el.remove();
  };
  document.body.append(el);
}

// Shown until the user has accepted the current Terms and confirmed 18+.
export function needsConsent(profile) {
  return !profile.termsCurrent || !profile.ageVerified;
}

export function consentGate(profile, onDone) {
  const max = new Date();
  max.setFullYear(max.getFullYear() - 18);
  const maxStr = max.toISOString().slice(0, 10);
  const isUpdate = !!profile.termsVersion && !profile.termsCurrent && profile.ageVerified;

  sheet(String(html`
    <div class="center" style="margin-bottom:18px">
      <img src="/icons/mark.png" alt="" width="56" height="56" style="margin:0 auto 10px">
      <div class="h2">${isUpdate ? 'We updated our Terms' : 'Welcome to CrestArena'}</div>
      <p class="muted small" style="margin-top:6px">${isUpdate
        ? 'Please review and accept the updated Terms and Privacy Policy to keep playing.'
        : 'A few quick details before you play for real prizes.'}</p>
    </div>
    <form id="consent-form" novalidate>
      ${profile.balance === 0 && !isUpdate ? html`
      <div class="field"><span class="label">Where do you play from?</span>
        <div class="seg" id="country">
          <button type="button" data-cur="NGN" class="${profile.currency !== 'GHS' ? 'active' : ''}">Nigeria · ₦ NGN</button>
          <button type="button" data-cur="GHS" class="${profile.currency === 'GHS' ? 'active' : ''}">Ghana · ₵ GHS</button>
        </div>
        <p class="hint">Your wallet currency. It can't be changed after your first deposit.</p></div>` : ''}
      <label class="field"><span class="label">Date of birth</span>
        <input class="input" type="date" name="birthDate" max="${maxStr}" min="1900-01-01" required>
        <span class="hint">You must be 18 or older. We use this only to confirm your age.</span></label>
      <div class="field stack-sm" style="margin-top:18px">
        <label class="check"><input type="checkbox" name="accept" required>
          <span>I have read and agree to the <a href="/legal/terms.html" target="_blank">Terms of Service</a>,
          <a href="/legal/privacy.html" target="_blank">Privacy Policy</a> and
          <a href="/legal/responsible-gaming.html" target="_blank">Responsible Gaming Policy</a>.</span></label>
        <label class="check"><input type="checkbox" name="adult" required>
          <span>I confirm I am 18+ and that playing skill games for money is legal where I live.</span></label>
        <label class="check"><input type="checkbox" name="marketing">
          <span>Send me news about tournaments and promotions (optional).</span></label>
      </div>
      <div class="form-error" id="consent-error" role="alert" hidden></div>
      <div class="sheet-actions"><button class="btn primary block" type="submit">Continue</button></div>
    </form>`), {
    dismissable: false,
    label: 'Terms and age confirmation',
    onMount(root, close) {
      let currency = profile.currency || 'NGN';
      root.querySelectorAll('[data-cur]').forEach((b) => (b.onclick = () => {
        currency = b.dataset.cur;
        root.querySelectorAll('[data-cur]').forEach((x) => x.classList.toggle('active', x === b));
      }));
      const err = root.querySelector('#consent-error');
      root.querySelector('#consent-form').onsubmit = (e) => {
        e.preventDefault();
        const f = new FormData(e.target);
        err.hidden = true;
        const fail = (m) => { err.textContent = m; err.hidden = false; };
        if (!f.get('birthDate')) return fail('Enter your date of birth.');
        if (!f.get('accept') || !f.get('adult')) return fail('Please tick both boxes to continue.');
        busy(e.target.querySelector('[type=submit]'), async () => {
          try {
            if (root.querySelector('#country') && currency !== profile.currency) await put('/me', { currency });
            await post('/me/consent', { birthDate: f.get('birthDate'), accept: true, marketing: !!f.get('marketing') });
            close();
            onDone();
          } catch (e2) { fail(e2.message); }
        });
      };
    },
  });
}

export function requireConsentToast() {
  toast('Accept the Terms and confirm your age to play for money.', 'info');
}
