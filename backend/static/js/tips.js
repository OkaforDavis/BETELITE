// Tips: free thank-yous to a host or player. The full amount reaches them.
import { get, post } from './api.js';
import { store } from './store.js';
import { html, ic, money, sym, sheet, sheetHead, busy, toast } from './ui.js';
import { needsConsent, consentGate } from './consent.js';
import { refreshProfile } from './app.js';
import { navigate } from './router.js';

// to: user id, name: shown name, context: tournament | match | stream, refId: its id.
export async function tipSheet({ to, name, context, refId, role = 'player' }) {
  const profile = store.get().profile;
  if (needsConsent(profile)) return consentGate(profile, () => refreshProfile());
  const o = await get('/tips/options');
  const cur = o.currency;
  sheet(String(html`${sheetHead(`Tip ${name}`, role === 'host' ? 'Say thanks for hosting' : 'Say thanks for a good game')}
    <form id="tip-form">
      <div class="chips">${o.presets.map((p) => html`<button type="button" class="chip" data-amt="${p}">${money(p, cur)}</button>`)}</div>
      <label class="field"><span class="label">Amount</span>
        <div class="input-money"><span class="cur">${sym(cur)}</span><input class="input num" name="amount" inputmode="numeric" pattern="[0-9]*" aria-label="Tip amount" required></div>
        <span class="hint">Your wallet: ${money(o.balance, cur)} · ${money(o.min, cur)} to ${money(o.max, cur)} per tip</span></label>
      <div class="notice" style="margin-top:12px">${ic('info')}<div class="small"><b>Tips are free.</b> ${name} gets the full amount in their wallet (converted at the wallet rate if they're in another country). Tips can't be refunded.</div></div>
      <div class="sheet-actions"><button class="btn primary block" type="submit">${ic('coins')} Send tip</button></div>
    </form>`), {
    label: 'Send a tip',
    onMount(root, close) {
      const f = root.querySelector('#tip-form');
      const btn = f.querySelector('[type=submit]');
      const label = () => {
        const v = Math.round(Number(f.amount.value) * 100) || 0;
        btn.lastChild.textContent = v ? ` Send ${money(v, cur)} tip` : ' Send tip';
        root.querySelectorAll('[data-amt]').forEach((c) => c.classList.toggle('active', Number(c.dataset.amt) === v));
      };
      root.querySelectorAll('[data-amt]').forEach((c) => (c.onclick = () => { f.amount.value = Number(c.dataset.amt) / 100; label(); }));
      f.amount.oninput = label;
      f.onsubmit = (e) => {
        e.preventDefault();
        const amount = Math.round(Number(f.amount.value) * 100);
        if (!amount || amount < o.min || amount > o.max) return toast(`A tip must be between ${money(o.min, cur)} and ${money(o.max, cur)}.`, 'err');
        if (amount > o.balance) {
          close(); toast('Top up your wallet to send this tip.', 'info');
          return navigate('/wallet?deposit=' + Math.ceil((amount - o.balance) / 100));
        }
        busy(btn, async () => {
          const { tip } = await post('/tips', { to, amount, context, refId });
          close(); refreshProfile();
          toast(`${tip.toName} got your ${money(tip.sent, tip.currency)} tip.`, 'ok', 'Tip sent 🎁');
        });
      };
    },
  });
}

// A small "Tip" button; bind with bindTipButtons.
export const tipButton = ({ to, name, context, refId, role = 'player', label = 'Tip', cls = 'btn secondary sm' }) =>
  html`<button type="button" class="${cls}" data-tip="${to}" data-tip-name="${name}" data-tip-ctx="${context}" data-tip-ref="${refId}" data-tip-role="${role}">${ic('coins')} ${label}</button>`;

export function bindTipButtons(root) {
  root.querySelectorAll('[data-tip]').forEach((b) => (b.onclick = (e) => {
    e.preventDefault(); e.stopPropagation();
    const d = b.dataset;
    tipSheet({ to: d.tip, name: d.tipName, context: d.tipCtx, refId: d.tipRef, role: d.tipRole });
  }));
}
