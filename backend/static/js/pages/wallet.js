import { get, post, quietPost, quietGet } from '../api.js';
import { store } from '../store.js';
import { html, ic, money, sym, timeAgo, sheet, sheetHead, busy, toast, empty, skeleton } from '../ui.js';
import { needsConsent, consentGate } from '../consent.js';
import { grossUp, transferFee, payCurrency, openCheckout } from '../components.js';
import { refreshProfile } from '../app.js';

const TX = {
  deposit: ['Deposit', 'arrowDown', 'brand'],
  withdrawal_hold: ['Withdrawal', 'arrowUp', ''],
  withdrawal_refund: ['Withdrawal returned', 'refresh', 'info'],
  wager_hold: ['Stake locked', 'lock', ''],
  wager_win: ['1v1 winnings', 'trophy', 'brand'],
  wager_refund: ['Stake refunded', 'refresh', 'info'],
  challenge_expired_refund: ['Challenge expired, refunded', 'refresh', 'info'],
  bounty_hold: ['Bounty posted', 'zap', ''],
  bounty_fee: ['Bounty hosting fee', 'coins', ''],
  bounty_win: ['Bounty won', 'trophy', 'brand'],
  bounty_return: ['Bounty returned', 'refresh', 'info'],
  bounty_refund: ['Bounty refunded', 'refresh', 'info'],
  cup_host_hold: ['Cup prize pool', 'trophy', ''],
  cup_host_fee: ['Cup hosting fee', 'coins', ''],
  cup_host_refund: ['Cup refunded', 'refresh', 'info'],
  match_timeout_refund: ['Match timed out, refunded', 'refresh', 'info'],
  tournament_entry: ['Tournament entry', 'trophy', ''],
  tournament_refund: ['Tournament refund', 'refresh', 'info'],
  tournament_prize: ['Tournament prize', 'crown', 'brand'],
  referral: ['Invite reward', 'users', 'brand'],
  referral_bonus: ['Welcome bonus', 'sparkles', 'brand'],
  membership: ['Membership', 'sparkles', ''],
  membership_cashback: ['Membership cashback', 'coins', 'brand'],
  membership_upgrade: ['Upgrade to Elite', 'crown', ''],
  currency_switch: ['Wallet moved to new country', 'refresh', 'info'],
  tip_sent: ['Tip sent', 'coins', ''],
  tip_received: ['Tip received', 'coins', 'brand'],
  admin_adjustment: ['Balance adjustment', 'settings', ''],
};
const WD_STATUS = { pending: ['Awaiting approval', 'warn'], processing: ['Sending', 'info'], paid: ['Paid', 'brand'], rejected: ['Rejected', 'danger'], failed: ['Failed, refunded', 'danger'], reversed: ['Bank returned it, refunded', 'danger'] };
const QUICK = { NGN: [1000, 2000, 5000, 10000], GHS: [20, 50, 100, 200] };

export default async function wallet(view, { query }) {
  // The router already shows this page's skeleton.
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
        })}</div>` : html`<div class="card flat">${empty('wallet', 'No activity yet', 'Your deposits, prizes and withdrawals appear here.')}</div>`}
      </section>

      ${withdrawals.some((x) => !['pending', 'processing'].includes(x.status)) ? html`<section class="section">
        <div class="section-head"><h2 class="h2">Past withdrawals</h2></div>
        <div class="list">${withdrawals.filter((x) => !['pending', 'processing'].includes(x.status)).map((x) => html`
          <div class="list-row"><div class="grow"><div class="h3 num">${money(x.amount, x.currency)}</div><div class="small faint">${x.bankName} ${x.account} · ${timeAgo(x.createdAt)}</div></div>
          <span class="badge ${WD_STATUS[x.status]?.[1] || ''}">${WD_STATUS[x.status]?.[0] || x.status}</span></div>`)}</div></section>` : ''}

      <div class="notice" style="margin-top:24px">${ic('shield')}<div class="small">Payments are processed securely by Paystack and Flutterwave. We never see your card details.
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
    const payCurs = (w.payCurrencies || []).filter((c) => ['NGN', 'GHS'].includes(c));
    let payCur = payCurrency() || cur;
    if (!payCurs.includes(payCur)) payCur = cur;
    const CUR_NAME = { NGN: 'Naira (₦)', GHS: 'Cedis (₵)' };
    sheet(String(html`${sheetHead('Deposit', 'Card, bank transfer, USSD or mobile money')}
      <form id="dep-form">
        ${payCurs.length > 1 ? html`<span class="label">Pay in</span><div class="seg" id="paycur" style="margin-bottom:14px">${payCurs.map((c) => html`<button type="button" data-pc="${c}" class="${c === payCur ? 'active' : ''}">${CUR_NAME[c] || c}</button>`)}</div>` : ''}
        <div class="input-money"><span class="cur">${sym(cur)}</span><input class="input num" name="amount" inputmode="numeric" pattern="[0-9]*" value="${prefill || QUICK[cur][1]}" aria-label="Amount"></div>
        <div class="chips" style="margin-top:10px">${QUICK[cur].map((q) => html`<button type="button" class="chip" data-q="${q}">${money(q * 100, cur)}</button>`)}</div>
        <p class="hint">Min ${money(w.limits.minDeposit, cur)} · Max ${money(w.limits.maxDeposit, cur)}</p>
        <div class="card flat" id="dep-sum" style="margin-top:12px"></div>
        <div class="sheet-actions"><button class="btn primary block" type="submit">${ic('shield')} Pay securely</button></div>
      </form>`), {
      label: 'Deposit',
      onMount(root, close) {
        const input = root.querySelector('[name=amount]');
        const sum = root.querySelector('#dep-sum');
        // The exchange rate (when paying in another currency) and the payment
        // provider's fee are added on top, so the full amount reaches the wallet.
        let seq = 0;
        let timer = null;
        const summary = (amount, fee, total, pc, q) => html`<div class="row between small"><span class="muted">Added to wallet</span><b class="num">${money(amount, cur)}</b></div>
          ${q ? html`<div class="row between small" style="margin-top:6px"><span class="muted">Converted (incl. 3% exchange margin)</span><span class="num">${money(q.payAmount, pc)}</span></div>` : ''}
          <div class="row between small" style="margin-top:6px"><span class="muted">Payment fee</span><span class="num">${money(fee, pc)}</span></div>
          <div class="divider" style="margin:10px 0"></div>
          <div class="row between"><span>You pay</span><b class="h3 num">${money(total, pc)}</b></div>`;
        const update = () => {
          clearTimeout(timer);
          const amount = (parseInt(input.value.replace(/\D/g, ''), 10) || 0) * 100;
          if (!amount) { sum.innerHTML = ''; return; }
          if (payCur === cur) {
            const total = grossUp(amount, w.feeRule);
            sum.innerHTML = String(summary(amount, total - amount, total, cur));
            return;
          }
          sum.innerHTML = '<div class="small muted">Working out today’s rate…</div>';
          const mine = ++seq;
          timer = setTimeout(async () => {
            try {
              const q = await quietGet(`/wallet/quote?amount=${amount}&payCurrency=${payCur}`);
              if (mine === seq) sum.innerHTML = String(summary(amount, q.fee, q.charged, payCur, q));
            } catch (e) {
              if (mine === seq) sum.innerHTML = String(html`<div class="small danger-text">${e.message}</div>`);
            }
          }, 350);
        };
        root.querySelectorAll('[data-pc]').forEach((b) => (b.onclick = () => {
          payCur = b.dataset.pc;
          root.querySelectorAll('[data-pc]').forEach((x) => x.classList.toggle('active', x === b));
          update();
        }));
        input.oninput = update;
        root.querySelectorAll('[data-q]').forEach((b) => (b.onclick = () => { input.value = b.dataset.q; update(); }));
        update();
        root.querySelector('#dep-form').onsubmit = (e) => {
          e.preventDefault();
          const amount = (parseInt(input.value.replace(/\D/g, ''), 10) || 0) * 100;
          if (amount < w.limits.minDeposit || amount > w.limits.maxDeposit) return toast(`Enter between ${money(w.limits.minDeposit, cur)} and ${money(w.limits.maxDeposit, cur)}`, 'err');
          busy(e.target.querySelector('[type=submit]'), async () => {
            const d = await post('/wallet/deposit', { amount, payCurrency: payCur });
            close();
            if (d.provider === 'paystack') waitForCredit(d.reference, true);
            await openCheckout(d, (ref) => waitForCredit(ref));
          });
        };
      },
    });
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
        const { status } = await quietPost('/wallet/deposit/verify', { reference: ref });
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
    sheet(String(html`${sheetHead('Withdraw', cur === 'GHS' ? 'To your bank or mobile money' : 'To your bank account')}
      <form id="wd-form">
        <label class="field"><span class="label">Amount</span>
          <div class="input-money"><span class="cur">${sym(cur)}</span><input class="input num" name="amount" inputmode="numeric" pattern="[0-9]*" aria-label="Amount"></div>
          <span class="hint">Available ${money(w.balance, cur)} · Min ${money(w.limits.minWithdrawal, cur)} <button type="button" class="link-btn" data-max>Max</button></span></label>
        <div class="field"><label class="label" for="bank-search">${cur === 'GHS' ? 'Bank or mobile money' : 'Bank'}</label>
          <div class="search-input">${ic('search')}<input class="input" id="bank-search" type="search" placeholder="Loading banks…" autocomplete="off" autocapitalize="off" spellcheck="false" role="combobox" aria-expanded="false" aria-controls="bank-list"></div>
          <div class="pick-list" id="bank-list" role="listbox" hidden></div></div>
        <label class="field"><span class="label">${cur === 'GHS' ? 'Account / phone number' : 'Account number'}</span>
          <input class="input num" name="account" inputmode="numeric" pattern="[0-9]*" maxlength="20" required autocomplete="off"></label>
        <label class="field" id="name-field"><span class="label">Account name</span>
          <input class="input" name="accountName" ${cur === 'NGN' ? 'readonly placeholder="Filled in automatically"' : 'required'}></label>
        <div class="card flat" id="wd-sum" style="margin-top:14px" hidden></div>
        <div class="notice" style="margin-top:14px">${ic('info')}<div class="small">The money is held from your balance now. Most withdrawals arrive within minutes; larger ones are checked first and paid within 24 hours. The account name must match your own name.</div></div>
        <div class="sheet-actions"><button class="btn primary block" type="submit">Request withdrawal</button></div>
      </form>`), {
      label: 'Withdraw',
      async onMount(root, close) {
        const f = root.querySelector('#wd-form');
        root.querySelector('[data-max]').onclick = () => { f.amount.value = Math.floor(w.balance / 100); showFee(); };
        let banks = [];
        let bankIdx = -1;
        let payoutProvider = w.payoutProvider || 'paystack';
        const search = root.querySelector('#bank-search');
        const listEl = root.querySelector('#bank-list');
        try {
          ({ banks, provider: payoutProvider } = await get('/wallet/banks'));
          search.placeholder = cur === 'GHS' ? 'Search, e.g. MTN or GCB' : 'Search, e.g. OPay or GTBank';
        } catch (e) { search.placeholder = 'Could not load banks'; toast(e.message, 'err'); }
        const showList = (open) => { listEl.hidden = !open; search.setAttribute('aria-expanded', String(open)); };
        const drawList = () => {
          const found = searchBanks(banks, search.value, cur);
          listEl.innerHTML = found.length
            ? found.map(({ b, i }) => `<button type="button" role="option" data-bi="${i}">${b.name.replace(/</g, '&lt;')}${b.type === 'mobile_money' ? ' <span class="tiny faint">mobile money</span>' : ''}</button>`).join('')
            : '<div class="small muted" style="padding:12px">No bank matches that. Check the spelling.</div>';
          listEl.querySelectorAll('[data-bi]').forEach((b) => (b.onclick = () => {
            bankIdx = +b.dataset.bi;
            search.value = banks[bankIdx].name;
            showList(false);
            tryResolve(); showFee();
            f.account.focus();
          }));
          showList(true);
        };
        search.onfocus = () => { if (banks.length) { if (bankIdx >= 0) search.select(); drawList(); } };
        search.oninput = () => { bankIdx = -1; drawList(); tryResolve(); };
        search.onkeydown = (e) => {
          if (e.key === 'Enter') { e.preventDefault(); listEl.querySelector('[data-bi]')?.click(); }
          if (e.key === 'Escape' && !listEl.hidden) { e.stopPropagation(); showList(false); }
        };
        root.addEventListener('click', (e) => { if (!e.target.closest('#bank-list, #bank-search')) showList(false); });

        let resolved = '';
        const tryResolve = async () => {
          if (cur !== 'NGN') return;
          const bank = banks[bankIdx];
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
        const wdSum = root.querySelector('#wd-sum');
        // Paystack's transfer fee is taken from the amount sent.
        function showFee() {
          const amount = (parseInt(f.amount.value.replace(/\D/g, ''), 10) || 0) * 100;
          const bank = banks[bankIdx];
          wdSum.hidden = !amount;
          if (!amount) return;
          const fee = transferFee(amount, cur, bank?.type, payoutProvider);
          wdSum.innerHTML = String(html`<div class="row between small"><span class="muted">Transfer fee</span><span class="num">${money(fee, cur)}</span></div>
            <div class="row between" style="margin-top:6px"><span>You receive</span><b class="h3 num">${money(Math.max(amount - fee, 0), cur)}</b></div>`);
        }
        f.amount.oninput = showFee;
        f.account.oninput = tryResolve;

        f.onsubmit = (e) => {
          e.preventDefault();
          const amount = (parseInt(f.amount.value.replace(/\D/g, ''), 10) || 0) * 100;
          const bank = banks[bankIdx];
          if (!bank) { search.focus(); return toast('Choose your bank from the list', 'err'); }
          if (amount < w.limits.minWithdrawal) return toast(`The minimum withdrawal is ${money(w.limits.minWithdrawal, cur)}.`, 'err');
          if (amount > w.balance) return toast(`You can withdraw up to ${money(w.balance, cur)}.`, 'err');
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

// Bank search: matches every typed word, common short names (GTB, UBA, FBN,
// momo), and lists the banks most players use first before anything is typed.
const POPULAR = {
  NGN: ['opay', 'palmpay', 'moniepoint', 'kuda', 'access', 'guaranty', 'zenith', 'first bank', 'united bank for africa'],
  GHS: ['mtn', 'telecel', 'vodafone', 'airteltigo', 'gcb', 'ecobank'],
};
const ALIASES = { gtb: 'guaranty', gtbank: 'guaranty', uba: 'united bank for africa', fbn: 'first bank', firstbank: 'first bank', momo: 'mobile money', voda: 'telecel', fcmb: 'first city' };
const norm = (t) => t.toLowerCase().replace(/[^a-z0-9 ]/g, ' ').replace(/\s+/g, ' ').trim();
export function searchBanks(banks, query, cur) {
  const all = banks.map((b, i) => ({ b, i, n: norm(b.name + (b.type === 'mobile_money' ? ' mobile money' : '')) }));
  const pop = POPULAR[cur] || [];
  const rank = (x) => { const k = pop.findIndex((p) => x.n.includes(p)); return k < 0 ? 99 : k; };
  const q = norm(query || '');
  if (!q) return all.sort((a, b) => rank(a) - rank(b) || a.n.localeCompare(b.n));
  const terms = q.split(' ').map((t) => ALIASES[t] || t);
  const compact = q.replace(/ /g, '');
  const starts = (x) => (x.n.startsWith(terms[0]) ? 0 : 1);
  return all
    .filter((x) => terms.every((t) => x.n.includes(t)) || x.n.replace(/ /g, '').includes(compact))
    .sort((a, b) => starts(a) - starts(b) || rank(a) - rank(b) || a.n.localeCompare(b.n))
    .slice(0, 50);
}
