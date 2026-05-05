const express = require('express');
const axios = require('axios');
const router = express.Router();

const PAYSTACK_API_URL = 'https://api.paystack.co';
const PAYSTACK_SECRET_KEY = process.env.PAYSTACK_SECRET_KEY || '';

// Currency mapping by country
const COUNTRY_CURRENCY = {
  GH: 'GHS', NG: 'NGN', KE: 'KES', TZ: 'TZS', UG: 'UGX', ZA: 'ZAR',
  EG: 'EGP', MA: 'MAD', CM: 'XAF', SN: 'XOF', CI: 'XOF',
  US: 'USD', GB: 'GBP', CA: 'CAD', AU: 'AUD', JP: 'JPY', IN: 'INR', BR: 'BRL',
};

const CURRENCY_SYMBOLS = {
  GHS: '₵', NGN: '₦', KES: 'KSh', TZS: 'TSh', UGX: 'USh', ZAR: 'R',
  EGP: 'E£', MAD: 'د.م.', XAF: 'FCFA', XOF: 'CFA', USD: '$', GBP: '£',
};

const COUNTRY_NAMES = {
  GH: 'Ghana', NG: 'Nigeria', KE: 'Kenya', TZ: 'Tanzania', UG: 'Uganda',
  ZA: 'South Africa', EG: 'Egypt', MA: 'Morocco', CM: 'Cameroon', SN: 'Senegal',
  CI: 'Côte d\'Ivoire', US: 'United States', GB: 'United Kingdom',
};

/**
 * Get client IP from request
 */
function getClientIP(req) {
  return (
    (req.headers['x-forwarded-for'] || '').split(',')[0].trim() ||
    req.headers['x-client-ip'] ||
    req.headers['cf-connecting-ip'] ||
    req.socket.remoteAddress ||
    'unknown'
  );
}

/**
 * Get geolocation from IP
 */
async function getGeolocationFromIP(ipAddress) {
  try {
    const response = await axios.get(
      `http://ip-api.com/json/${ipAddress}?fields=status,country,countryCode,city,regionName,lat,lon,timezone,isp`,
      { timeout: 5000 }
    );

    if (response.data.status === 'success') {
      return {
        ip: ipAddress,
        countryCode: response.data.countryCode,
        countryName: response.data.country,
        city: response.data.city,
        region: response.data.regionName,
        latitude: response.data.lat,
        longitude: response.data.lon,
        timezone: response.data.timezone,
        isp: response.data.isp,
      };
    }
    return null;
  } catch (error) {
    console.error('[GEO] Error:', error.message);
    return null;
  }
}

/**
 * Middleware: Detect user currency from geolocation
 */
const detectCurrency = async (req, res, next) => {
  try {
    const clientIP = getClientIP(req);
    const geoLocation = await getGeolocationFromIP(clientIP);

    if (geoLocation) {
      const currencyCode = COUNTRY_CURRENCY[geoLocation.countryCode] || 'USD';
      req.geoLocation = geoLocation;
      req.userCurrency = currencyCode;
      req.currencySymbol = CURRENCY_SYMBOLS[currencyCode] || currencyCode;
      req.countryCode = geoLocation.countryCode;
      req.countryName = geoLocation.countryName;
      console.log(`[CURRENCY] ${geoLocation.countryName} → ${currencyCode}`);
    } else {
      req.userCurrency = 'USD';
      req.currencySymbol = '$';
      req.countryCode = 'US';
      req.countryName = 'Unknown';
    }
    next();
  } catch (error) {
    console.error('[CURRENCY] Error:', error);
    req.userCurrency = 'USD';
    req.currencySymbol = '$';
    req.countryCode = 'US';
    req.countryName = 'Unknown';
    next();
  }
};

router.use(detectCurrency);

/**
 * POST /api/payments/initialize
 */
router.post('/initialize', async (req, res) => {
  try {
    const { amount, description, email = 'test@betelite.com', tournamentId, betId } = req.body;

    if (!amount || amount <= 0) {
      return res.status(400).json({ error: 'Invalid amount' });
    }

    const amountInLowestUnit = Math.round(amount * 100);

    const response = await axios.post(
      `${PAYSTACK_API_URL}/transaction/initialize`,
      {
        email,
        amount: amountInLowestUnit,
        currency: req.userCurrency,
        metadata: {
          tournamentId,
          betId,
          countryCode: req.countryCode,
          countryName: req.countryName,
        },
      },
      {
        headers: {
          Authorization: `Bearer ${PAYSTACK_SECRET_KEY}`,
          'Content-Type': 'application/json',
        },
      }
    );

    if (!response.data.status) {
      return res.status(400).json({ error: response.data.message });
    }

    res.json({
      success: true,
      transaction: {
        reference: response.data.data.reference,
        accessCode: response.data.data.access_code,
        authorizationUrl: response.data.data.authorization_url,
        amount: `${req.currencySymbol}${amount.toFixed(2)}`,
        currency: req.userCurrency,
        currencySymbol: req.currencySymbol,
      },
    });
  } catch (error) {
    console.error('[PAYMENT] Initialize Error:', error.message);
    res.status(500).json({ error: 'Payment initialization failed' });
  }
});

/**
 * POST /api/payments/verify/:reference
 */
router.post('/verify/:reference', async (req, res) => {
  try {
    const { reference } = req.params;

    const response = await axios.get(
      `${PAYSTACK_API_URL}/transaction/verify/${reference}`,
      {
        headers: {
          Authorization: `Bearer ${PAYSTACK_SECRET_KEY}`,
        },
      }
    );

    if (!response.data.status) {
      return res.status(400).json({ error: 'Payment verification failed' });
    }

    const paymentData = response.data.data;

    res.json({
      success: true,
      message: 'Payment verified successfully',
      transaction: {
        reference: paymentData.reference,
        amount: `${req.currencySymbol}${(paymentData.amount / 100).toFixed(2)}`,
        status: paymentData.status,
        timestamp: paymentData.paid_at,
      },
    });
  } catch (error) {
    console.error('[PAYMENT] Verify Error:', error.message);
    res.status(500).json({ error: 'Payment verification failed' });
  }
});

/**
 * GET /api/payments/currencies
 */
router.get('/currencies', async (req, res) => {
  try {
    res.json({
      success: true,
      userCurrency: {
        code: req.userCurrency,
        symbol: req.currencySymbol,
        countryCode: req.countryCode,
        countryName: req.countryName,
      },
      supportedCurrencies: Object.keys(COUNTRY_CURRENCY)
        .reduce((acc, code) => {
          const currency = COUNTRY_CURRENCY[code];
          acc[currency] = {
            code: currency,
            symbol: CURRENCY_SYMBOLS[currency],
          };
          return acc;
        }, {}),
    });
  } catch (error) {
    res.status(500).json({ error: 'Failed to get currencies' });
  }
});

/**
 * GET /api/payments/countries
 */
router.get('/countries', async (req, res) => {
  try {
    const countries = Object.entries(COUNTRY_CURRENCY).map(([code, currency]) => ({
      country: COUNTRY_NAMES[code] || code,
      code: code,
      currency: currency,
      symbol: CURRENCY_SYMBOLS[currency] || currency,
    }));

    res.json({
      success: true,
      count: countries.length,
      countries,
      userLocation: {
        code: req.countryCode,
        name: req.countryName,
        currency: req.userCurrency,
        currencySymbol: req.currencySymbol,
      },
    });
  } catch (error) {
    res.status(500).json({ error: 'Failed to get countries' });
  }
});

/**
 * POST /api/payments/convert
 */
router.post('/convert', async (req, res) => {
  try {
    const { amount, fromCurrency, toCurrency } = req.body;

    if (!amount || !fromCurrency || !toCurrency) {
      return res.status(400).json({ error: 'Missing required fields' });
    }

    const response = await axios.get(
      `https://api.exchangerate-api.com/v4/latest/${fromCurrency}`
    );

    const rate = response.data.rates[toCurrency] || 1;
    const convertedAmount = amount * rate;

    res.json({
      success: true,
      conversion: {
        amount,
        fromCurrency,
        toCurrency,
        convertedAmount: parseFloat(convertedAmount.toFixed(2)),
        rate: parseFloat(rate.toFixed(4)),
        formattedAmount: `${CURRENCY_SYMBOLS[fromCurrency] || fromCurrency}${amount.toFixed(2)}`,
        formattedConverted: `${CURRENCY_SYMBOLS[toCurrency] || toCurrency}${convertedAmount.toFixed(2)}`,
      },
    });
  } catch (error) {
    console.error('[CONVERT] Error:', error.message);
    res.status(500).json({ error: 'Currency conversion failed' });
  }
});

module.exports = router;
