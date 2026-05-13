const fs = require('fs');

let html = fs.readFileSync('mobile/index.html', 'utf8');

// 1. Add fetchGlobalSettings to init() or run it globally
const initRegex = /async function init\(\) \{/;
if (!html.includes('fetchGlobalSettings()')) {
  html = html.replace(initRegex, `async function fetchGlobalSettings() {
    try {
      const res = await fetch(BACKEND_URL + '/api/settings');
      if (res.ok) {
        const data = await res.json();
        if (data.settings && data.settings.paystackKey) {
          PAYMENT_CONFIG.paystackKey = data.settings.paystackKey;
          localStorage.setItem('PAYSTACK_KEY', data.settings.paystackKey);
        }
      }
    } catch(e) {
      console.warn('Could not sync global settings');
    }
  }

  async function init() {
    await fetchGlobalSettings();`);
}

// 2. Update setPaystackKey()
const setKeyRegex = /function setPaystackKey\(\) \{[\s\S]*?closeM\(\);\s*\}\s*\}/;

const newSetKey = `async function setPaystackKey() {
    const k = prompt('Enter Paystack Public Key (pk_live_... or pk_test_...):');
    if (k) {
      localStorage.setItem('PAYSTACK_KEY', k.trim());
      PAYMENT_CONFIG.paystackKey = k.trim();
      
      try {
        await fetch(BACKEND_URL + '/api/settings', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ key: 'paystackKey', value: k.trim() })
        });
        toast('🚀 Paystack Key Synced Globally. Deposits enabled on all devices.');
      } catch (e) {
        toast('⚠️ Saved locally, but failed to sync globally.');
      }
      
      closeM();
    }
  }`;

html = html.replace(setKeyRegex, newSetKey);

fs.writeFileSync('mobile/index.html', html);
console.log('Updated mobile/index.html for Global Sync');
