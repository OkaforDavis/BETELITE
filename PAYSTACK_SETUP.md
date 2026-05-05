# Paystack Payment Integration Guide

## Overview

BETELITE now supports multi-currency payments through Paystack with automatic currency detection based on user location.

**Supported Currencies & Countries:**
- 🇬🇭 Ghana - GHS (Ghanaian Cedis)
- 🇳🇬 Nigeria - NGN (Nigerian Naira)
- 🇰🇪 Kenya - KES (Kenyan Shilling)
- 🇹🇿 Tanzania - TZS (Tanzanian Shilling)
- 🇺🇬 Uganda - UGX (Ugandan Shilling)
- 🇿🇦 South Africa - ZAR (South African Rand)
- 🇪🇬 Egypt - EGP (Egyptian Pound)
- 🇲🇦 Morocco - MAD (Moroccan Dirham)
- And more...

---

## Setup Instructions

### Step 1: Get Paystack API Keys

1. Go to [Paystack Dashboard](https://dashboard.paystack.com)
2. Create an account or log in
3. Navigate to **Settings → API Keys**
4. You'll find:
   - **Public Key** - For frontend/client-side
   - **Secret Key** - For backend/server-side (keep this secret!)

### Step 2: Add Environment Variables

Create a `.env` file in the `backend/` directory with:

```bash
# Paystack Configuration
PAYSTACK_PUBLIC_KEY=pk_test_fc5dc9f01286c9d16f2abd1cb64df1155b8e6e1c
PAYSTACK_SECRET_KEY=sk_test_YOUR_SECRET_KEY_HERE

# Geolocation API (optional, for better accuracy)
# Using ip-api.com which is free
GEOLOCATION_API=ip-api.com
```

### Step 3: Install Dependencies

```bash
cd backend
npm install axios
```

### Step 4: Update Backend Routes

Add the payment router to your `server.ts` or `server.js`:

```typescript
import paymentsRouter from './routes/paymentsV2';

// ... other imports and setup ...

app.use('/api/payments', paymentsRouter);
```

---

## How It Works

### 1. Automatic Currency Detection

When a user accesses your platform:
1. Their IP address is detected
2. Geolocation API determines their country
3. Currency is automatically set based on their location
4. Currency symbol is displayed (₦ for NGN, ₵ for GHS, etc.)

**Example:**
```
User IP → 102.89.x.x → Nigeria → NGN (Nigerian Naira) → ₦
User IP → 197.255.x.x → Ghana → GHS (Ghanaian Cedis) → ₵
```

### 2. Real-Time Currency Conversion

When users from different countries participate in a tournament:
- Tournament entry fee is automatically converted to user's local currency
- Uses real-time exchange rates from exchangerate-api.com
- User always sees amounts in their own currency

**Example:**
```
Tournament Entry: $10 USD
Nigerian User sees: ₦5,500 (approx)
Ghanaian User sees: ₵60 (approx)
```

### 3. Payment Processing

**Flow:**
1. User clicks "Pay" button
2. System detects their currency (automatic)
3. Amount is shown in user's currency
4. User is redirected to Paystack payment page
5. User completes payment in their currency
6. System verifies payment with Paystack
7. User is added to tournament/bet settled
8. Funds added to user's wallet

---

## API Endpoints

### Initialize Payment
```
POST /api/payments/initialize

Body:
{
  "amount": 100,
  "description": "Tournament entry fee",
  "tournamentId": "tournament_123",
  "currency": "NGN"  // Optional, auto-detected if omitted
}

Response:
{
  "success": true,
  "transaction": {
    "id": "txn_123",
    "reference": "paystack_ref_123",
    "accessCode": "abc123",
    "authorizationUrl": "https://checkout.paystack.com/abc123",
    "amount": "₦5,500.00",
    "currency": "NGN"
  }
}
```

### Verify Payment
```
POST /api/payments/verify/:reference

Response:
{
  "success": true,
  "message": "Payment verified successfully",
  "transaction": {
    "reference": "paystack_ref_123",
    "amount": "₦5,500.00",
    "status": "success",
    "timestamp": "2026-05-05T10:30:00Z"
  }
}
```

### Get User's Currency
```
GET /api/payments/currencies

Response:
{
  "success": true,
  "currencies": {...},
  "userCurrency": {
    "code": "NGN",
    "symbol": "₦",
    "countryCode": "NG",
    "countryName": "Nigeria"
  }
}
```

### Convert Currency
```
POST /api/payments/convert

Body:
{
  "amount": 100,
  "fromCurrency": "USD",
  "toCurrency": "NGN"
}

Response:
{
  "success": true,
  "conversion": {
    "amount": 100,
    "fromCurrency": "USD",
    "toCurrency": "NGN",
    "convertedAmount": 55000,
    "formattedAmount": "$100.00",
    "formattedConverted": "₦55,000.00"
  }
}
```

### Get Supported Countries
```
GET /api/payments/countries

Response:
{
  "success": true,
  "count": 15,
  "countries": [
    {
      "country": "Nigeria",
      "code": "NG",
      "currency": "NGN",
      "symbol": "₦"
    },
    ...
  ],
  "userLocation": {
    "code": "NG",
    "name": "Nigeria",
    "currency": "NGN",
    "currencySymbol": "₦"
  }
}
```

### Get Payment Transactions
```
GET /api/payments/transactions?limit=20&offset=0

Response:
{
  "success": true,
  "count": 5,
  "transactions": [
    {
      "id": "txn_123",
      "user_id": "user_456",
      "amount": 5500,
      "currency": "NGN",
      "status": "completed",
      "created_at": "2026-05-05T10:30:00Z",
      "tournament_id": "tournament_789"
    },
    ...
  ]
}
```

---

## Frontend Implementation

### 1. Display Currency Symbol

```html
<div class="price">
  <span id="currency-symbol">₦</span>
  <span id="amount">5,500</span>
</div>

<script>
  fetch('/api/payments/currencies')
    .then(res => res.json())
    .then(data => {
      document.getElementById('currency-symbol').textContent = data.userCurrency.symbol;
    });
</script>
```

### 2. Initialize Payment

```javascript
async function initiatePayment(tournamentId) {
  try {
    const response = await fetch('/api/payments/initialize', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${userToken}`
      },
      body: JSON.stringify({
        amount: 100,
        description: 'Tournament entry',
        tournamentId: tournamentId
      })
    });

    const data = await response.json();
    
    if (data.success) {
      // Redirect to Paystack
      window.location.href = data.transaction.authorizationUrl;
    }
  } catch (error) {
    console.error('Payment error:', error);
  }
}
```

### 3. Verify Payment

```javascript
async function verifyPayment(reference) {
  try {
    const response = await fetch(`/api/payments/verify/${reference}`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${userToken}`
      }
    });

    const data = await response.json();
    
    if (data.success) {
      console.log('Payment successful!');
      // Redirect to success page
      window.location.href = '/payment-success';
    }
  } catch (error) {
    console.error('Verification error:', error);
  }
}

// When user returns from Paystack
const urlParams = new URLSearchParams(window.location.search);
const reference = urlParams.get('reference');
if (reference) {
  verifyPayment(reference);
}
```

---

## Currency Conversion in Tournaments

When a tournament has users from multiple countries:

```typescript
// Get tournament entry fee in USD
const entryFeeUSD = 10;

// Get Nigerian user's currency and convert
const nigerianRate = await convertCurrency(entryFeeUSD, 'USD', 'NGN');
// Result: ~5,500 NGN

// Get Ghanaian user's currency and convert
const ghanaianRate = await convertCurrency(entryFeeUSD, 'USD', 'GHS');
// Result: ~60 GHS
```

---

## Testing

### Test Keys (Provided by Paystack)

**Public Key:** `pk_test_fc5dc9f01286c9d16f2abd1cb64df1155b8e6e1c`

**Test Card Numbers:**
- Visa: `4111 1111 1111 1111`
- Mastercard: `5555 5555 5555 4444`
- Expiry: Any future date
- CVV: Any 3 digits

### Testing Flow

1. Add test keys to `.env`
2. Start backend: `npm run dev`
3. Visit your app
4. Initiate a test payment
5. Use test card numbers
6. Verify the transaction

---

## Security Notes

### DO NOT:
- ❌ Expose `PAYSTACK_SECRET_KEY` in frontend code
- ❌ Store keys in version control
- ❌ Display secret key in client responses
- ❌ Trust client-side validation alone

### DO:
- ✅ Use environment variables for keys
- ✅ Validate payments on server-side
- ✅ Verify all transactions with Paystack
- ✅ Implement rate limiting on payment endpoints
- ✅ Log all payment activities
- ✅ Use HTTPS in production

---

## Troubleshooting

### "Currency not supported"
- Make sure the user's country is in the supported list
- Check geolocation detection is working
- Try using a different IP address

### "Payment verification failed"
- Ensure `PAYSTACK_SECRET_KEY` is correct
- Check that reference code is valid
- Verify payment status with Paystack dashboard

### "Geolocation detection not working"
- May be blocked by corporate firewalls
- Use IP-based detection as fallback
- Implement manual country selection as backup

### Real Exchange Rates Not Updating
- exchangerate-api.com may be rate limited
- Implement caching of rates
- Consider paid API for production

---

## Support

For issues:
1. Check Paystack documentation: https://paystack.com/docs
2. Check IP-API documentation: https://ip-api.com
3. Check exchangerate-api: https://exchangerate-api.com

---

## Migration from Test to Live

When ready for production:

1. Get Live Keys from Paystack Dashboard
2. Update `.env` with live keys:
   ```bash
   PAYSTACK_PUBLIC_KEY=pk_live_YOUR_LIVE_KEY
   PAYSTACK_SECRET_KEY=sk_live_YOUR_SECRET_KEY
   ```
3. Use live card numbers to test
4. Monitor transactions in Paystack dashboard
5. Ensure SSL/HTTPS is enabled

---

Last Updated: May 5, 2026
