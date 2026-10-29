import { get, post, quietPost } from '../api.js';
import { store } from '../store.js';
import { html, ic, money, busy, toast, confirmSheet } from '../ui.js';
import { tierBadge, grossUp, payCurrency, openCheckout } from '../components.js';
import { needsConsent, consentGate } from '../consent.js';
import { refreshProfile } from '../app.js';

const fmtDate = (iso) => new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' });

export default async function membership(view) {
  let data;

  async function load() {
    data = await get('/membership');
    draw();
  }

  function draw() {
    const { tiers, usage, currency, balance, renewWindowDays } = data;
    const plan = usage.plan;
    const daysLeft = plan ? Math.ceil((new Date(plan.endsAt) - new Date()) / 864e5) : 0;
    const canRenew = plan && !usage.renewedTo && daysLeft <= renewWindowDays;

    view.innerHTML = String(html`<div class="page">
      <div class="h1">Membership</div>
      <div class="muted small" style="margin-top:4px">Everyone plays the same tournaments for the same prizes. Members get more entries each month, a badge and early sign-up.</div>

      <div class="card pad-lg" style="margin-top:18px">
        <div class="row between">
          <div><div class="tiny faint">YOUR PLAN</div>
            <div class="row" style="gap:8px;margin-top:4px"><div class="h2">${tiers.find((t) => t.id === usage.tier)?.name || 'Free'}</div>${tierBadge(usage.tier)}</div></div>
          <div style="text-align:right"><div class="tiny faint">ENTRIES LEFT</div><div class="h2 num brand-text">${usage.left}/${usage.allowed}</div></div>
        </div>
        <div class="usage-dots" aria-label="${usage.used} of ${usage.allowed} entries used">${Array.from({ length: usage.allowed }, (_, i) => html`<span class="${i < usage.used ? 'used' : ''}"></span>`)}</div>
        <div class="small muted" style="margin-top:10px">${plan
          ? html`${usage.renewedTo ? html`Renewed until <b>${fmtDate(usage.renewedTo)}</b>` : html`Ends <b>${fmtDate(plan.endsAt)}</b> (${daysLeft} day${daysLeft === 1 ? '' : 's'} left)`} · entries reset when the plan renews`
          : html`Your free entry resets on <b>${fmtDate(usage.resetsAt)}</b>`}</div>
      </div>

      <div class="section-head" style="margin-top:24px"><h2 class="h2">Plans</h2></div>
      <div class="plans">${tiers.map((t) => {
        const current = t.id === usage.tier;
        let action = '';
        if (t.price > 0) {
          if (current && canRenew) action = html`<button class="btn primary block" data-buy="${t.id}">Renew for ${money(t.price, currency)}</button>`;
          else if (current) action = html`<button class="btn secondary block" disabled>Current plan</button>`;
          else if (!plan) action = html`<button class="btn ${t.id === 'pro' ? 'primary' : 'secondary'} block" data-buy="${t.id}">Get ${t.name}</button>`;
          else action = html`<button class="btn ghost block" disabled>Available after ${fmtDate(usage.renewedTo || plan.endsAt)}</button>`;
        }
        return html`<div class="card plan tier-${t.id} ${current ? 'current' : ''}">
          <div class="row between"><div class="plan-name">${t.name}</div>${current ? html`<span class="badge brand">${ic('check')}Your plan</span>` : ''}</div>
          <div class="plan-price" style="margin-top:10px">${t.price ? money(t.price, currency) : 'Free'}${t.price ? html`<small>/ 30 days</small>` : ''}</div>
          <div class="small muted" style="margin-top:6px"><span class="plan-entries">${t.entries} tournament${t.entries === 1 ? '' : 's'}</span> ${t.price ? 'per plan' : 'per month'}</div>
          <ul class="checklist" style="margin-top:12px">${t.perks.map((p) => html`<li class="ok">${ic('check')}<span>${p}</span></li>`)}</ul>
          ${action ? html`<div style="margin-top:14px">${action}</div>` : ''}
        </div>`;
      })}</div>

      <div class="notice" style="margin-top:20px">${ic('info')}<div class="small">Prizes are fixed before each tournament and are the same for every player, whatever their plan. There are no entry fees: your plan only sets how many tournaments you can enter.
        Plans don't renew automatically. You can renew in the last ${renewWindowDays} days, and change plan after the current one ends.</div></div>
    </div>`);

    view.querySelectorAll('[data-buy]').forEach((b) => (b.onclick = () => buy(tiers.find((t) => t.id === b.dataset.buy), b, balance, currency)));
  }

  async function buy(tier, btn, balance, currency) {
    const profile = store.get().profile;
    if (needsConsent(profile)) return consentGate(profile, () => refreshProfile());
    const fromWallet = balance >= tier.price;
    const shortfall = Math.max(tier.price - balance, data.minDeposit || 0);
    // Pay in this visit's currency (e.g. naira for a Ghana wallet); the server quotes
    // the exchange (incl. 3% margin) and the provider's fee, which the player covers.
    const pc = payCurrency() || currency;
    let charge = grossUp(shortfall, data.feeRule);
    let payCur = currency;
    if (!fromWallet && pc !== currency) {
      try {
        const q = await get('/wallet/quote?amount=' + shortfall + '&payCurrency=' + pc);
        charge = q.charged;
        payCur = pc;
      } catch { /* fall back to the wallet currency */ }
    }
    const ok = await confirmSheet({
      title: `Get ${tier.name}?`,
      message: fromWallet
        ? `${money(tier.price, currency)} will be paid from your wallet. You get ${tier.entries} tournament entries for 30 days.`
        : `You'll pay ${money(charge, payCur)} by card, bank or mobile money${payCur !== currency ? ` (converted from ${money(shortfall, currency)} at today's rate, incl. fees)` : ` (${money(shortfall, currency)} + ${money(charge - shortfall, currency)} payment fee)`}${balance > 0 ? `; your wallet covers ${money(balance, currency)}` : ''}. You get ${tier.entries} tournament entries for 30 days.`,
      confirm: fromWallet ? `Pay ${money(tier.price, currency)}` : `Pay ${money(charge, payCur)}`,
    });
    if (!ok) return;
    busy(btn, async () => {
      const r = await post('/membership/buy', { tier: tier.id, payCurrency: payCur });
      if (r.membership) {
        toast(`You can now enter ${tier.entries} tournaments.`, 'ok', `${tier.name} is active`);
        refreshProfile();
        load();
        return;
      }
      await openCheckout(r.checkout, (ref) => waitForPlan(ref));
    });
  }

  // The plan starts as soon as Paystack confirms the payment.
  function waitForPlan(ref) {
    let tries = 0;
    const tick = async () => {
      tries++;
      try {
        const { status } = await quietPost('/wallet/deposit/verify', { reference: ref });
        if (status === 'paid') {
          toast('Your membership is active.', 'ok', 'Payment received');
          refreshProfile();
          load();
          return;
        }
        if (status === 'failed') return toast('The payment was not completed.', 'err');
      } catch { /* keep trying */ }
      if (tries < 40) setTimeout(tick, 4000);
    };
    setTimeout(tick, 2000);
  }

  await load();
}

