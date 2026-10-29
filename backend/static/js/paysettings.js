// Payment currency & home country sheet (opened from Profile).
import { post } from './api.js';
import { store } from './store.js';
import { html, ic, money, sheet, sheetHead, busy, toast, confirmSheet } from './ui.js';
import { refreshProfile } from './app.js';

const NAMES = { NGN: ['Nigeria', 'Naira (₦)'], GHS: ['Ghana', 'Cedis (₵)'] };

export function paySettingsSheet() {
  const { profile, payCurrencies, homeSwitch } = store.get();
  let pref = 'auto';
  try { pref = localStorage.getItem('crestarena.payCurrencyPref') || 'auto'; } catch { /* private mode */ }
  const options = [['auto', 'Automatic (your wallet, or local currency when travelling)'], ...(payCurrencies || []).map((c) => [c, NAMES[c]?.[1] || c])];
  const hs = homeSwitch || {};

  sheet(String(html`${sheetHead('Payment currency & country', 'How you pay, and where your wallet lives')}
    <label class="field" style="margin-top:0"><span class="label">Pay in</span>
      <select class="select" name="pref">${options.map(([v, l]) => html`<option value="${v}" ${v === pref ? 'selected' : ''}>${l}</option>`)}</select></label>
    <p class="hint">Paying in another currency converts at today's rate with a 3% exchange margin. Your wallet, prizes and withdrawals stay in your home currency.</p>

    <div class="card flat" style="margin-top:16px">
      <div class="small muted">Home country (wallet currency)</div>
      <div class="h3" style="margin-top:4px">${NAMES[profile.currency]?.[0] || profile.currency} · ${profile.currency}</div>
      ${hs.to ? (hs.eligible
        ? html`<p class="small" style="margin-top:8px">You've been using CrestArena in ${NAMES[hs.to]?.[0]} for a while. You can move your wallet there: your balance converts at today's rate (less 3%), and prices, prizes and withdrawals switch to ${hs.to}. You can do this once every 90 days.</p>
               <button class="btn primary block" id="move" style="margin-top:10px">${ic('refresh')} Move wallet to ${NAMES[hs.to]?.[0]}</button>`
        : html`<p class="small muted" style="margin-top:8px">Living in ${NAMES[hs.to]?.[0]} now? ${hs.reason || ''}${hs.daysLeft ? ` (${hs.daysLeft} day${hs.daysLeft === 1 ? '' : 's'} left)` : ''}.</p>`)
        : html`<p class="small muted" style="margin-top:8px">Your wallet moves with you if you relocate: after 30 days in another supported country you can switch here.</p>`}
      <a class="small row" href="/support?category=location" style="gap:6px;margin-top:10px">${ic('flag')} Something wrong? Contact an admin</a>
    </div>`), {
    label: 'Payment currency and country',
    onMount(root, close) {
      root.querySelector('[name=pref]').onchange = (e) => {
        try {
          localStorage.setItem('crestarena.payCurrencyPref', e.target.value);
          sessionStorage.removeItem('crestarena.payCurrency'); // a saved choice replaces this visit's choice
        } catch { /* private mode */ }
        toast('Payment currency saved');
      };
      root.querySelector('#move')?.addEventListener('click', async (e) => {
        const ok = await confirmSheet({
          title: `Move your wallet to ${NAMES[hs.to]?.[0]}?`,
          message: `Your ${money(profile.balance, profile.currency)} converts to ${hs.to} at today's rate less a 3% margin. Prices, prizes and withdrawals then use ${hs.to}. You can't move again for 90 days.`,
          confirm: 'Move wallet',
        });
        if (!ok) return;
        busy(e.target, async () => {
          const r = await post('/me/home-currency', { currency: hs.to });
          close();
          toast(`Your wallet is now ${money(r.balance, r.currency)}.`, 'ok', 'Wallet moved');
          refreshProfile();
        });
      });
    },
  });
}
