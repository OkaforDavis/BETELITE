# BETELITE Paystack Payment Integration Quick Start

## What Was Implemented

### ✅ Complete Paystack Payment System
- Multi-currency support (15+ currencies)
- Automatic currency detection by user location
- Real-time currency conversion
- Payment verification and wallet integration
- Tournament entry with automatic currency conversion

---

## Quick Setup (5 Minutes)

### 1. Add Your Paystack Keys to `.env`

```bash
# In BETELITE_v3/BETELITE_v3/backend/.env

PAYSTACK_PUBLIC_KEY=pk_test_fc5dc9f01286c9d16f2abd1cb64df1155b8e6e1c
PAYSTACK_SECRET_KEY=sk_test_YOUR_SECRET_KEY_HERE
```

**How to get these:**
1. Go to [Paystack Dashboard](https://dashboard.paystack.com)
2. Click Settings → API Keys
3. Copy your Test keys and add to `.env`

### 2. Start the Backend

```bash
cd BETELITE_v3/BETELITE_v3/backend
npm install
npm run dev
```

### 3. Test Payment Flow

```
User visits → Currency auto-detected → User sees local currency symbol
User pays → Redirected to Paystack → Payment verified → Wallet credited
```

---

## How It Works

### Automatic Currency Detection
```
User IP: 102.89.x.x (Nigeria)
  ↓
Geolocation API identifies Nigeria
  ↓
Currency set to NGN (Nigerian Naira)
  ↓
User sees: ₦5,500 instead of $10
```

### Tournament Entry with Conversion
```
Tournament Entry: $10 USD

Nigerian user → ₦5,500 (at current rate)
Ghanaian user → ₵60 (at current rate)  
Kenyan user → KSh1,200 (at current rate)
```

### Payment Processing
```
1. User clicks "Enter Tournament"
2. System detects user's currency → Shows amount in NGN/GHS/etc
3. User clicks "Pay with Paystack"
4. Redirected to Paystack checkout (in their currency)
5. User completes payment
6. System verifies with Paystack
7. User added to tournament
8. Funds added to wallet
```

---

## Supported Currencies

| Country | Code | Symbol |
|---------|------|--------|
| 🇳🇬 Nigeria | NGN | ₦ |
| 🇬🇭 Ghana | GHS | ₵ |
| 🇰🇪 Kenya | KES | KSh |
| 🇹🇿 Tanzania | TZS | TSh |
| 🇺🇬 Uganda | UGX | USh |
| 🇿🇦 South Africa | ZAR | R |
| 🇪🇬 Egypt | EGP | E£ |
| 🇲🇦 Morocco | MAD | د.م. |
| 🇨🇲 Cameroon | XAF | FCFA |
| 🇸🇳 Senegal | XOF | CFA |
| 🇺🇸 USA | USD | $ |
| 🇬🇧 UK | GBP | £ |

---

## API Endpoints

### Initialize Payment
```bash
POST /api/payments/initialize
Authorization: Bearer YOUR_JWT_TOKEN

Body:
{
  "amount": 100,
  "description": "Tournament entry",
  "tournamentId": "tournament_123"
}

Response:
{
  "success": true,
  "transaction": {
    "authorizationUrl": "https://checkout.paystack.com/abc123",
    "amount": "₦5,500.00",
    "currency": "NGN"
  }
}
```

### Verify Payment
```bash
POST /api/payments/verify/:reference
Authorization: Bearer YOUR_JWT_TOKEN

Response:
{
  "success": true,
  "message": "Payment verified successfully",
  "transaction": {
    "reference": "paystack_ref_123",
    "amount": "₦5,500.00",
    "status": "success"
  }
}
```

### Get User's Currency
```bash
GET /api/payments/currencies

Response:
{
  "success": true,
  "userCurrency": {
    "code": "NGN",
    "symbol": "₦",
    "countryCode": "NG",
    "countryName": "Nigeria"
  }
}
```

### Convert Currency
```bash
POST /api/payments/convert

Body:
{
  "amount": 100,
  "fromCurrency": "USD",
  "toCurrency": "NGN"
}

Response:
{
  "conversion": {
    "convertedAmount": 55000,
    "formattedAmount": "$100.00",
    "formattedConverted": "₦55,000.00"
  }
}
```

### Get Payment History
```bash
GET /api/payments/transactions?limit=20
Authorization: Bearer YOUR_JWT_TOKEN

Response:
{
  "success": true,
  "transactions": [
    {
      "id": "txn_123",
      "amount": 5500,
      "currency": "NGN",
      "status": "completed",
      "tournament_id": "tournament_789"
    }
  ]
}
```

---

## Frontend Integration Example

### Display Currency
```html
<div id="price">
  <span id="currency-symbol">₦</span>
  <span id="amount">5,500</span>
</div>

<script>
  fetch('/api/payments/currencies')
    .then(res => res.json())
    .then(data => {
      document.getElementById('currency-symbol').textContent = data.userCurrency.symbol;
      document.getElementById('amount').textContent = formatAmount(5500, data.userCurrency.code);
    });
</script>
```

### Process Payment
```javascript
async function payWithPaystack(tournamentId) {
  const response = await fetch('/api/payments/initialize', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${userToken}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      amount: 100,
      tournamentId: tournamentId
    })
  });
  
  const data = await response.json();
  
  if (data.success) {
    // Redirect to Paystack
    window.location.href = data.transaction.authorizationUrl;
  }
}
```

### Verify After Payment
```javascript
// After user returns from Paystack
const reference = new URLSearchParams(window.location.search).get('reference');

if (reference) {
  fetch(`/api/payments/verify/${reference}`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${userToken}`
    }
  })
  .then(res => res.json())
  .then(data => {
    if (data.success) {
      alert('Payment successful! You are now in the tournament.');
      window.location.href = '/tournaments';
    }
  });
}
```

---

## Files Created

| File | Purpose |
|------|---------|
| `backend/services/paymentService.ts` | Paystack API integration, currency detection, conversion |
| `backend/middleware/geolocation.ts` | IP-based geolocation detection |
| `backend/routes/paymentsV2.ts` | Payment API endpoints |
| `PAYSTACK_SETUP.md` | Complete setup and API documentation |

---

## Test Card Numbers (for Testing)

- **Visa**: `4111 1111 1111 1111`
- **Mastercard**: `5555 5555 5555 4444`
- **Expiry**: Any future date
- **CVV**: Any 3 digits

Use these to test the full payment flow in test mode.

---

## Features

✅ Automatic currency detection by location
✅ Real-time exchange rate conversion
✅ Paystack payment processing
✅ Payment verification
✅ Wallet crediting on payment
✅ Tournament participation on payment
✅ Multi-currency tournament support
✅ Complete payment history
✅ Currency conversion API
✅ IP-based geolocation

---

## Configuration Reference

```bash
# .env file location:
BETELITE_v3/BETELITE_v3/backend/.env

# Required variables:
PAYSTACK_PUBLIC_KEY=pk_test_xxx
PAYSTACK_SECRET_KEY=sk_test_xxx

# Optional (auto-detected):
GEOLOCATION_API=ip-api.com
EXCHANGE_RATE_API=exchangerate-api.com
```

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "Currency not supported" | Add Paystack secret key to .env |
| "Payment verification failed" | Check that reference code is correct |
| "Geolocation not working" | Use fallback to USD, or test with different IP |
| "Exchange rates not updating" | Check exchangerate-api.com is accessible |

---

## Next Steps

1. ✅ Add Paystack keys to `.env`
2. ✅ Start backend server
3. ✅ Test with frontend
4. ✅ Add payment button to tournament entry
5. ✅ Monitor transactions in Paystack dashboard
6. ✅ When ready: switch to Live keys for production

---

## Support

For detailed documentation, see [PAYSTACK_SETUP.md](PAYSTACK_SETUP.md)

For Paystack help: https://paystack.com/docs
For API integration: https://api-docs.paystack.co

Last Updated: May 5, 2026
