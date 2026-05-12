const fs = require('fs');
let code = fs.readFileSync('mobile/index.html', 'utf8');

// 1. Replace CCY_DATA object
code = code.replace(
  /const CCY_DATA = \{\s*NGN: \{ sym: '₦', rate: 1, name: 'Nigerian Naira', maxBet: 1000000 \},\s*GHS: \{ sym: '₵', rate: 0\.037, name: 'Ghanaian Cedi', maxBet: 10000 \},\s*\};/g,
  `const CCY_DATA = {
  NGN: { sym: '₦', ngnValue: 1, name: 'Nigerian Naira', maxBet: 1000000 },
  GHS: { sym: '₵', ngnValue: 123, name: 'Ghanaian Cedi', maxBet: 10000 },
};`
);

// 2. Replace fmt and fmtAbbr
code = code.replace(
  /function fmt\(n\)\s*\{\s*return getSym\(\)\s*\+\s*\(ccy === 'NGN' \? n : Math\.round\(n \* CCY_DATA\.GHS\.rate\)\)\.toLocaleString\(\);\s*\}/g,
  `function fmt(n)   { return getSym() + (ccy === 'NGN' ? n : Math.round(n / CCY_DATA[ccy].ngnValue)).toLocaleString(); }`
);

code = code.replace(
  /const v = ccy === 'NGN' \? n : Math\.round\(n \* CCY_DATA\.GHS\.rate\);/g,
  `const v = ccy === 'NGN' ? n : Math.round(n / CCY_DATA[ccy].ngnValue);`
);

// 3. Replace doDeposit
code = code.replace(
  /checkoutAmt = Math\.round\(amt \* CCY_DATA\[ccy\]\.rate\);/g,
  `checkoutAmt = Math.round(amt * CCY_DATA[ccy].ngnValue);`
);

code = code.replace(
  /const n = ccy === 'NGN' \? amt : Math\.round\(amt \/ CCY_DATA\.GHS\.rate\);/g,
  `const n = ccy === 'NGN' ? amt : Math.round(amt * CCY_DATA[ccy].ngnValue);`
);

fs.writeFileSync('mobile/index.html', code);
console.log('Currency logic updated to exact 123 rate.');
