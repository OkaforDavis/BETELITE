const fs = require('fs');
let content = fs.readFileSync('mobile/index.html', 'utf8');

// 1. Fix the orphaned </div> bug
content = content.replace(
  /  <\/div>\s*<\/div>\s*<!-- Promo grid — upcoming games -->/m,
  `  </div>\n\n      <!-- Promo grid — upcoming games -->`
);

// 2. Add Paystack SDK
content = content.replace(
  /<!-- Socket\.io -->\n<script src="\/socket\.io\/socket\.io\.js" onerror="window\._noSocket=true"><\/script>/,
  `<!-- Socket.io -->\n<script src="/socket.io/socket.io.js" onerror="window._noSocket=true"></script>\n<!-- Paystack SDK -->\n<script src="https://js.paystack.co/v1/inline.js"></script>`
);

// 3. Update PAYMENT_CONFIG
content = content.replace(
  /const PAYMENT_CONFIG = \{\s*paystackKey:\s*window\.PAYSTACK_PUBLIC_KEY\s*\|\|\s*null,\s*flutterwaveKey:\s*window\.FLUTTERWAVE_PUBLIC_KEY\s*\|\|\s*null,\s*\};/,
  `const PAYMENT_CONFIG = {\n  paystackKey:    localStorage.getItem('PAYSTACK_KEY') || window.PAYSTACK_PUBLIC_KEY || null,\n  flutterwaveKey: window.FLUTTERWAVE_PUBLIC_KEY || null,\n};`
);

// 4. Update doDeposit
content = content.replace(
  /function doDeposit\(amt\) \{\s*if \(!hasPayment\(\)\) \{ toast\('Payment not yet available'\); return; \}\s*const n = ccy === 'NGN' \? amt : Math\.round\(amt \/ CCY_DATA\.GHS\.rate\);\s*S\.wallet \+= n;\s*saveProfile\(\{ wallet: S\.wallet \}\);\s*updateWallet\(\); closeM\(\);\s*toast\(`✅ \$\{getSym\(\)\}\$\{amt\.toLocaleString\(\)\} deposited!`\);\s*\}/,
  `function doDeposit(amt) {
  if (!hasPayment()) { toast('Payment not yet available'); return; }
  
  const ref = 'CA_' + Math.floor((Math.random() * 1000000000) + 1);
  
  const handler = PaystackPop.setup({
    key: PAYMENT_CONFIG.paystackKey,
    email: S.user?.email || 'guest@crestarena.com',
    amount: amt * 100, // Paystack works in kobo/pesewas
    currency: ccy,     // Dynamically charges exactly in NGN or GHS
    ref: ref,
    callback: function(response) {
      toast(\`✅ Payment successful! (Ref: \${response.reference})\`);
      // Update normalized NGN base wallet
      const n = ccy === 'NGN' ? amt : Math.round(amt / CCY_DATA.GHS.rate);
      S.wallet += n;
      saveProfile({ wallet: S.wallet });
      updateWallet(); 
      closeM();
    },
    onClose: function() {
      toast('Payment window closed.');
    }
  });
  handler.openIframe();
}`
);

// 5. Add Admin Setting UI
content = content.replace(
  /<div class="menu-card" style="margin-bottom:10px">\s*<div class="menu-card-title">ACCOUNT<\/div>/,
  `<div class="menu-card" style="margin-bottom:10px">
        <div class="menu-card-title">ACCOUNT</div>
        \${S.user?.email === ADMIN_EMAIL ? \`
        <div class="setting-row">
          <div><div class="setting-label">Paystack Live Key (Admin)</div><div class="setting-sub">\${PAYMENT_CONFIG.paystackKey ? 'Key is configured ✅' : 'No key configured ❌'}</div></div>
          <button style="font-size:12px;color:var(--gold);font-weight:700;cursor:pointer" onclick="setPaystackKey()">Set Key</button>
        </div>\` : ''}`
);

// 6. Add setPaystackKey function
content = content.replace(
  /function toggleSetting\(key, el\) \{/,
  `function setPaystackKey() {
  const k = prompt('Enter Paystack Public Key (pk_live_... or pk_test_...):');
  if (k) {
    localStorage.setItem('PAYSTACK_KEY', k.trim());
    PAYMENT_CONFIG.paystackKey = k.trim();
    toast('✅ Paystack Key Saved. Deposits enabled.');
    closeM();
  }
}

function toggleSetting(key, el) {`
);

fs.writeFileSync('mobile/index.html', content);
console.log('Paystack setup complete and UI bug fixed.');
