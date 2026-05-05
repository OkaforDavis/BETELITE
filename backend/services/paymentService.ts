import axios from 'axios';

/**
 * Paystack Payment Service with Multi-Currency Support
 * Handles payments, currency detection, and exchange rate conversion
 */

const PAYSTACK_API_URL = 'https://api.paystack.co';
const PAYSTACK_PUBLIC_KEY = process.env.PAYSTACK_PUBLIC_KEY || '';
const PAYSTACK_SECRET_KEY = process.env.PAYSTACK_SECRET_KEY || '';

// Currency mapping by country code
const COUNTRY_CURRENCY_MAP: Record<string, string> = {
  GH: 'GHS', // Ghana - Ghanaian Cedis
  NG: 'NGN', // Nigeria - Nigerian Naira
  KE: 'KES', // Kenya - Kenyan Shilling
  TZ: 'TZS', // Tanzania - Tanzanian Shilling
  UG: 'UGX', // Uganda - Ugandan Shilling
  ZA: 'ZAR', // South Africa - South African Rand
  EG: 'EGP', // Egypt - Egyptian Pound
  MA: 'MAD', // Morocco - Moroccan Dirham
  CM: 'XAF', // Cameroon - Central African Franc
  SN: 'XOF', // Senegal - West African Franc
  CI: 'XOF', // Côte d'Ivoire - West African Franc
  US: 'USD', // USA - US Dollar
  GB: 'GBP', // UK - British Pound
  CA: 'CAD', // Canada - Canadian Dollar
  AU: 'AUD', // Australia - Australian Dollar
  JP: 'JPY', // Japan - Japanese Yen
  IN: 'INR', // India - Indian Rupee
  BR: 'BRL', // Brazil - Brazilian Real
};

// Paystack supported currencies
const PAYSTACK_CURRENCIES = ['GHS', 'NGN', 'KES', 'TZS', 'UGX', 'ZAR', 'EGP', 'MAD', 'XAF', 'XOF', 'USD', 'GBP'];

// Currency symbols
const CURRENCY_SYMBOLS: Record<string, string> = {
  GHS: '₵',
  NGN: '₦',
  KES: 'KSh',
  TZS: 'TSh',
  UGX: 'USh',
  ZAR: 'R',
  EGP: 'E£',
  MAD: 'د.م.',
  XAF: 'FCFA',
  XOF: 'CFA',
  USD: '$',
  GBP: '£',
};

/**
 * Interface for currency info
 */
export interface CurrencyInfo {
  code: string;
  symbol: string;
  countryCode: string;
  countryName: string;
}

/**
 * Interface for exchange rates
 */
export interface ExchangeRate {
  from: string;
  to: string;
  rate: number;
  timestamp: Date;
}

/**
 * Get currency based on country code (IP geolocation)
 */
export const getCurrencyByCountry = (countryCode: string): CurrencyInfo => {
  const currencyCode = COUNTRY_CURRENCY_MAP[countryCode.toUpperCase()] || 'USD';

  return {
    code: currencyCode,
    symbol: CURRENCY_SYMBOLS[currencyCode] || currencyCode,
    countryCode: countryCode.toUpperCase(),
    countryName: getCountryName(countryCode),
  };
};

/**
 * Get country name from country code
 */
const getCountryName = (countryCode: string): string => {
  const countryNames: Record<string, string> = {
    GH: 'Ghana',
    NG: 'Nigeria',
    KE: 'Kenya',
    TZ: 'Tanzania',
    UG: 'Uganda',
    ZA: 'South Africa',
    EG: 'Egypt',
    MA: 'Morocco',
    CM: 'Cameroon',
    SN: 'Senegal',
    CI: 'Côte d\'Ivoire',
    US: 'United States',
    GB: 'United Kingdom',
    CA: 'Canada',
    AU: 'Australia',
    JP: 'Japan',
    IN: 'India',
    BR: 'Brazil',
  };

  return countryNames[countryCode.toUpperCase()] || countryCode;
};

/**
 * Get real-time exchange rates
 * Uses fixer.io or exchangerate-api
 */
export const getExchangeRates = async (baseCurrency: string, targetCurrencies: string[]): Promise<Record<string, number>> => {
  try {
    // Using exchangerate-api (free tier available)
    const response = await axios.get(`https://api.exchangerate-api.com/v4/latest/${baseCurrency}`);

    const rates: Record<string, number> = {};
    targetCurrencies.forEach((currency) => {
      rates[currency] = response.data.rates[currency] || 1;
    });

    return rates;
  } catch (error) {
    console.error('Exchange Rate API Error:', error);

    // Fallback: return 1:1 rate if API fails
    const fallbackRates: Record<string, number> = {};
    targetCurrencies.forEach((currency) => {
      fallbackRates[currency] = 1;
    });

    return fallbackRates;
  }
};

/**
 * Convert amount from one currency to another
 */
export const convertCurrency = async (
  amount: number,
  fromCurrency: string,
  toCurrency: string
): Promise<number> => {
  if (fromCurrency === toCurrency) {
    return amount;
  }

  const rates = await getExchangeRates(fromCurrency, [toCurrency]);
  return amount * rates[toCurrency];
};

/**
 * Initialize Paystack transaction
 */
export const initializePaystackTransaction = async (
  email: string,
  amount: number,
  currency: string,
  metadata: any = {}
) => {
  try {
    // Paystack requires amounts in lowest currency unit (e.g., kobo for NGN)
    const amountInLowestUnit = Math.round(amount * 100);

    const response = await axios.post(
      `${PAYSTACK_API_URL}/transaction/initialize`,
      {
        email,
        amount: amountInLowestUnit,
        currency,
        metadata,
      },
      {
        headers: {
          Authorization: `Bearer ${PAYSTACK_SECRET_KEY}`,
          'Content-Type': 'application/json',
        },
      }
    );

    return response.data;
  } catch (error) {
    console.error('Paystack Initialize Error:', error);
    throw error;
  }
};

/**
 * Verify Paystack transaction
 */
export const verifyPaystackTransaction = async (reference: string) => {
  try {
    const response = await axios.get(`${PAYSTACK_API_URL}/transaction/verify/${reference}`, {
      headers: {
        Authorization: `Bearer ${PAYSTACK_SECRET_KEY}`,
      },
    });

    return response.data;
  } catch (error) {
    console.error('Paystack Verification Error:', error);
    throw error;
  }
};

/**
 * Create payment plan (for recurring payments)
 */
export const createPaymentPlan = async (
  name: string,
  description: string,
  amount: number,
  interval: 'monthly' | 'quarterly' | 'half-annually' | 'annually',
  currency: string
) => {
  try {
    const amountInLowestUnit = Math.round(amount * 100);

    const response = await axios.post(
      `${PAYSTACK_API_URL}/plan`,
      {
        name,
        description,
        amount: amountInLowestUnit,
        interval,
        currency,
      },
      {
        headers: {
          Authorization: `Bearer ${PAYSTACK_SECRET_KEY}`,
          'Content-Type': 'application/json',
        },
      }
    );

    return response.data;
  } catch (error) {
    console.error('Create Payment Plan Error:', error);
    throw error;
  }
};

/**
 * Get supported currencies
 */
export const getSupportedCurrencies = (): Record<string, CurrencyInfo> => {
  const currencies: Record<string, CurrencyInfo> = {};

  PAYSTACK_CURRENCIES.forEach((currency) => {
    const countryCode = Object.keys(COUNTRY_CURRENCY_MAP).find(
      (key) => COUNTRY_CURRENCY_MAP[key] === currency
    );

    if (countryCode) {
      currencies[currency] = {
        code: currency,
        symbol: CURRENCY_SYMBOLS[currency] || currency,
        countryCode,
        countryName: getCountryName(countryCode),
      };
    }
  });

  return currencies;
};

/**
 * Format amount for display
 */
export const formatCurrency = (amount: number, currency: string): string => {
  const symbol = CURRENCY_SYMBOLS[currency] || currency;
  return `${symbol}${amount.toFixed(2)}`;
};

/**
 * Check if currency is supported by Paystack
 */
export const isCurrencySupported = (currency: string): boolean => {
  return PAYSTACK_CURRENCIES.includes(currency);
};

/**
 * Get all available countries and currencies
 */
export const getCountryCurrencyList = (): Array<{ country: string; code: string; currency: string; symbol: string }> => {
  return Object.entries(COUNTRY_CURRENCY_MAP).map(([countryCode, currencyCode]) => ({
    country: getCountryName(countryCode),
    code: countryCode,
    currency: currencyCode,
    symbol: CURRENCY_SYMBOLS[currencyCode] || currencyCode,
  }));
};

export default {
  getCurrencyByCountry,
  getExchangeRates,
  convertCurrency,
  initializePaystackTransaction,
  verifyPaystackTransaction,
  createPaymentPlan,
  getSupportedCurrencies,
  formatCurrency,
  isCurrencySupported,
  getCountryCurrencyList,
};
