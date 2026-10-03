import { get, post } from '../api.js';
import { store } from '../store.js';
import { html, ic, money, sym, timeAgo, sheet, sheetHead, busy, toast, empty, skeleton } from '../ui.js';
import { needsConsent, consentGate } from '../consent.js';
import { refreshProfile } from '../app.js';

const TX = {
  deposit: ['Deposit', 'arrowDown', 'brand'],
  withdrawal_hold: ['Withdrawal', 'arrowUp', ''],
  withdrawal_refund: ['Withdrawal returned', 'refresh', 'info'],
  wager_hold: ['Stake locked', 'lock', ''],
  wager_win: ['1v1 winnings', 'trophy', 'brand'],
  wager_refund: ['Stake refunded', 'refresh', 'info'],
  challenge_expired_refund: ['Challenge expired, refunded', 'refresh', 'info'],
  match_timeout_refund: ['Match timed out, refunded', 'refresh', 'info'],
  tournament_entry: ['Tournament entry', 'trophy', ''],
  tournament_refund: ['Tournament refund', 'refresh', 'info'],
  tournament_prize: ['Tournament prize', 'crown', 'brand'],
  referral: ['Invite reward', 'users', 'brand'],
  referral_bonus: ['Welcome bonus', 'sparkles', 'brand'],
  admin_adjustment: ['Balance adjustment', 'settings', ''],
};
const WD_STATUS = { pending: ['Awaiting approval', 'warn'], processing: ['Sending', 'info'], paid: ['Paid', 'brand'], rejected: ['Rejected', 'danger'], failed: ['Failed, refunded', 'danger'] };
const QUICK = { NGN: [1000, 2000, 5000, 10000], GHS: [20, 50, 100, 200] };

export default async function wallet(view, { query }) {
  view.innerHTML = String(html`<div class="page">${skeleton(200)}${skeleton(300)}</div>`);
  let w;

  async function load() {
    const [wal, { transactions }, { withdrawals }] = await Promise.all([get('/wallet'), get('/me/transactions'), get('/wallet/withdrawals')]);
    w = wal;
    const cur = w.currency;
    view.innerHTML = String(html`<div class="page">
      <div class="wallet-card">
        <div class="row between"><span class="eyebrow" style="color:rgba(255,255,255,.7)">Available balance</span>
          <span class="badge brand">${cur === 'GHS' ? 'GHS' : 'NGN'}</span></div>
        <div class="amount">${money(w.balance, cur, { decimals: 2 })}</div>
        ${w.inPlay ? html`<div class="small" style="margin-top:8px;color:rgba(255,255,255,.75)">${ic('lock', '')} ${money(w.inPlay, cur)} locked in active matches</div>` : ''}
        <div class="actions">
          <button class="btn primary" id="dep">${ic('plus')} Deposit</button>
          <button class="btn secondary" id="wd">${ic('arrowUp')} Withdraw</button>
        </div>
      </div>

      ${withdrawals.some((x) => ['pending', 'processing'].includes(x.status)) ? html`<section class="section">
        <div class="section-head"><h2 class="h2">Withdrawals in progress</h2></div>
        <div class="list">${withdrawals.filter((x) => ['pending', 'processing'].includes(x.status)).map((x) => html`
          <div class="list-row"><div class="icon-tile">${ic('bank')}</div><div class="grow"><div class="h3 num">${money(x.amount, x.currency)}</div>
          <div class="small muted">${x.bankName} ${x.account}</div></div><span class="badge ${WD_STATUS[x.status][1]}">${WD_STATUS[x.status][0]}</span></div>`)}</div>
      </section>` : ''}

      <section class="section">
        <div class="section-head"><h2 class="h2">Activity</h2></div>
        ${transactions.length ? html`<div class="list">${transactions.map((t) => {
          const [label, i, tone] = TX[t.type] || [t.type.replace(/_/g, ' '), 'coins', ''];
          return html`<div class="list-row"><div class="icon-tile ${tone}">${ic(i)}</div>
            <div class="grow"><div class="h3">${label}</div><div class="small faint">${timeAgo(t.createdAt)}</div></div>
            <b class="num ${t.amount > 0 ? 'brand-text' : ''}">${t.amount > 0 ? '+' : '−'}${money(Math.abs(t.amount), cur)}</b></div>`;
        })}</div>` : html`<div class="card flat">${empty('wallet', 'No activity yet', 'Deposit to start playing. Your deposits, stakes and winnings appear here.')}</div>`}
      </section>

      ${withdrawals.some((x) => !['pending', 'processing'].includes(x.status)) ? html`<section class="section">
        <div class="section-head"><h2 class="h2">Past withdrawals</h2></div>
        <div class="list">${withdrawals.filter((x) => !['pending', 'processing'].includes(x.status)).map((x) => html`
          <div class="list-row"><div class="grow"><div class="h3 num">${money(x.amount, x.currency)}</div><div class="small faint">${x.bankName} ${x.account} · ${timeAgo(x.createdAt)}</div></div>
          <span class="badge ${WD_STATUS[x.status]?.[1] || ''}">${WD_STATUS[x.status]?.[0] || x.status}</span></div>`)}</div></section>` : ''}

      <div class="notice" style="margin-top:24px">${ic('shield')}<div class="small">Payments are processed securely by Paystack. We never see your card details.
        Set yourself limits and take breaks. <a href="/legal/responsible-gaming.html" target="_blank">Responsible gaming</a></div></div>
    </div>`);

    view.querySelector('#dep').onclick = () => deposit();
    view.querySelector('#wd').onclick = () => withdraw();
  }

  function gate() {
    const p = store.get().profile;
    if (needsConsent(p)) { consentGate(p, () => refreshProfile()); return false; }
    return true;
  }

  function deposit(prefill) {
    if (!gate()) return;
    const cur = w.currency;
    sheet(String(html`${sheetHead('Deposit', 'Card, bank transfer or USSD via Paystack')}
      <form id="dep-form">
        <div class="input-money"><span class="cur">${sym(cur)}</span><input class="input num" name="amount" inputmode="numeric" pattern="[0-9]*" value="${prefill || QUICK[cur][1]}" aria-label="Amount"></div>
        <div class="chips" style="margin-top:10px">${QUICK[cur].map((q) => html`<button type="button" class="chip" data-q="${q}">${money(q * 100, cur)}</button>`)}</div>
        <p class="hint">Min ${money(w.limits.minDeposit, cur)} · Max ${money(w.limits.maxDeposit, cur)}</p>
        <div class="sheet-actions"><button class="btn primary block" type="submit">${ic('shield')} Pay securely</button></div>
      </form>`), {
      label: 'Deposit',
      onMount(root, close) {
        const input = root.querySelector('[name=amount]');
        root.querySelectorAll('[data-q]').forEach((b) => (b.onclick = () => (input.value = b.dataset.q)));
        root.querySelector('#dep-form').onsubmit = (e) => {
          e.preventDefault();
          const amount = (parseInt(input.value.replace(/\D/g, ''), 10) || 0) * 100;
          if (amount < w.limits.minDeposit || amount > w.limits.maxDeposit) return toast(`Enter between ${money(w.limits.minDeposit, cur)} and ${money(w.limits.maxDeposit, cur)}`, 'err');
          busy(e.target.querySelector('[type=submit]'), async () => {
            const d = await post('/wallet/deposit', { amount });
            close();
            await openCheckout(d);
          });
        };
      },
    });
  }

  async function openCheckout(d) {
    try {
      await loadScript('https://js.paystack.co/v2/inline.js');
      const popup = new window.PaystackPop();
      popup.resumeTransaction(d.accessCode, {
        onSuccess: () => waitForCredit(d.reference),
        onCancel: () => toast('Payment cancelled', 'info'),
      });
      waitForCredit(d.reference, true);
    } catch {
      location.href = d.authorizationUrl; // hosted page fallback; returns to /?deposit=ref
    }
  }

  // Poll until Paystack confirms (the webhook may credit first; both are safe).
  let polling = null;
  function waitForCredit(ref, quiet = false) {
    if (polling === ref) return;
    polling = ref;
    let tries = 0;
    const tick = async () => {
      if (polling !== ref) return;
      tries++;
      try {
        const { status } = await post('/wallet/deposit/verify', { reference: ref });
        if (status === 'paid') {
          polling = null;
          toast('Your wallet has been topped up.', 'ok', 'Deposit received');
          refreshProfile(); load();
          return;
        }
        if (status === 'failed') { polling = null; if (!quiet) toast('The payment was not completed.', 'err'); return; }
      } catch { /* keep trying */ }
      if (tries < 60) setTimeout(tick, 5000); else polling = null;
    };
    setTimeout(tick, 3000);
  }

  async function withdraw() {
    if (!gate()) return;
    const cur = w.currency;
    if (w.balance < w.limits.minWithdrawal) return toast(`The minimum withdrawal is ${money(w.limits.minWithdrawal, cur)}.`, 'info');
    sheet(String(html`${sheetHead('Withdraw', 'Paid to your bank after a quick review')}
      <form id="wd-form">
        <label class="field"><span class="label">Amount</span>
          <div class="input-money"><span class="cur">${sym(cur)}</span><input class="input num" name="amount" inputmode="numeric" pattern="[0-9]*" aria-label="Amount"></div>
          <span class="hint">Available ${money(w.balance, cur)} · Min ${money(w.limits.minWithdrawal, cur)} <button type="button" class="link-btn" data-max>Max</button></span></label>
        <label class="field"><span class="label">${cur === 'GHS' ? 'Bank or mobile money' : 'Bank'}</span>
          <select class="select" name="bank" required><option value="">Loading banks…</option></select></label>
        <label class="field"><span class="label">${cur === 'GHS' ? 'Account / phone number' : 'Account number'}</span>
          <input class="input num" name="account" inputmode="numeric" pattern="[0-9]*" maxlength="20" required autocomplete="off"></label>
        <label class="field" id="name-field"><span class="label">Account name</span>
          <input class="input" name="accountName" ${cur === 'NGN' ? 'readonly placeholder="Filled in automatically"' : 'required'}></label>
        <div class="notice" style="margin-top:14px">${ic('info')}<div class="small">The money is held from your balance now. Withdrawals are reviewed and usually paid within 24 hours. The account name must match your own name.</div></div>
        <div class="sheet-actions"><button class="btn primary block" type="submit">Request withdrawal</button></div>
      </form>`), {
      label: 'Withdraw',
      async onMount(root, close) {
        const f = root.querySelector('#wd-form');
        const select = f.bank;
        root.querySelector('[data-max]').onclick = () => (f.amount.value = Math.floor(w.balance / 100));
        let banks = [];
        try {
          ({ banks } = await get('/wallet/banks'));
          select.innerHTML = '<option value="">Choose…</option>' + banks.map((b, i) => `<option value="${i}">${b.name.replace(/</g, '&lt;')}</option>`).join('');
        } catch (e) { select.innerHTML = '<option value="">Could not load banks</option>'; toast(e.message, 'err'); }

        let resolved = '';
        const tryResolve = async () => {
          if (cur !== 'NGN') return;
          const bank = banks[select.value];
          resolved = '';
          f.accountName.value = '';
          if (!bank || !/^\d{10}$/.test(f.account.value)) return;
          f.accountName.value = 'Checking…';
          try {
            const { accountName } = await post('/wallet/resolve', { accountNumber: f.account.value, bankCode: bank.code });
            resolved = accountName;
            f.accountName.value = accountName;
          } catch (e) { f.accountName.value = ''; toast(e.message, 'err'); }
        };
        f.account.oninput = tryResolve;
        select.onchange = tryResolve;

        f.onsubmit = (e) => {
          e.preventDefault();
          const amount = (parseInt(f.amount.value.replace(/\D/g, ''), 10) || 0) * 100;
          const bank = banks[select.value];
          if (!bank) return toast('Choose your bank', 'err');
          if (amount < w.limits.minWithdrawal || amount > w.balance) return toast('Enter an amount within your available balance', 'err');
          if (cur === 'NGN' && !resolved) return toast('Enter a valid 10-digit account number', 'err');
          busy(f.querySelector('[type=submit]'), async () => {
            await post('/wallet/withdraw', {
              amount, bankCode: bank.code, bankName: bank.name, bankType: bank.type,
              accountNumber: f.account.value, accountName: f.accountName.value,
            });
            close();
            toast("Withdrawal requested. We'll notify you when it's sent.");
            refreshProfile(); load();
          });
        };
      },
    });
  }

  await load();
  if (query.deposit) deposit(query.deposit);
}

function loadScript(src) {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) return resolve();
    const s = document.createElement('script');
    s.src = src;
    s.onload = resolve;
    s.onerror = reject;
    document.head.append(s);
  });
}
