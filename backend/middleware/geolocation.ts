import { Request, Response, NextFunction } from 'express';
import axios from 'axios';
import { getCurrencyByCountry, isCurrencySupported } from '../services/paymentService';

/**
 * Geolocation and Currency Detection Middleware
 * Automatically detects user's location and preferred currency
 */

export interface GeoLocationInfo {
  ip: string;
  countryCode: string;
  countryName: string;
  city: string;
  region: string;
  latitude: number;
  longitude: number;
  timezone: string;
  isp: string;
}

/**
 * Get geolocation from IP address
 * Uses ip-api.com or ipapi.co
 */
export const getGeolocationFromIP = async (ipAddress: string): Promise<GeoLocationInfo | null> => {
  try {
    // Using ip-api.com (free tier)
    const response = await axios.get(`http://ip-api.com/json/${ipAddress}?fields=status,country,countryCode,city,regionName,lat,lon,timezone,isp`, {
      timeout: 5000,
    });

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
    console.error('Geolocation API Error:', error);
    return null;
  }
};

/**
 * Get client IP address from request
 */
export const getClientIP = (req: Request): string => {
  return (
    (req.headers['x-forwarded-for'] as string)?.split(',')[0].trim() ||
    (req.headers['x-client-ip'] as string) ||
    (req.headers['cf-connecting-ip'] as string) ||
    req.socket.remoteAddress ||
    'unknown'
  );
};

/**
 * Middleware to detect user location and set currency
 */
export const detectUserCurrency = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  try {
    // Get client IP
    const clientIP = getClientIP(req);

    // Try to get geolocation
    const geoLocation = await getGeolocationFromIP(clientIP);

    // Set geolocation info on request
    if (geoLocation) {
      req.geoLocation = geoLocation;

      // Get currency for country
      const currency = getCurrencyByCountry(geoLocation.countryCode);

      // Set currency on request
      req.userCurrency = currency.code;
      req.currencySymbol = currency.symbol;
      req.countryCode = geoLocation.countryCode;
      req.countryName = geoLocation.countryName;

      console.log(`[GeoLocation] User from ${geoLocation.countryName} (${geoLocation.countryCode}) - Currency: ${currency.code}`);
    } else {
      // Default to USD if geolocation fails
      req.userCurrency = 'USD';
      req.currencySymbol = '$';
      req.countryCode = 'US';
      req.countryName = 'Unknown';
    }

    next();
  } catch (error) {
    console.error('Currency Detection Error:', error);
    // Continue with default USD if detection fails
    req.userCurrency = 'USD';
    req.currencySymbol = '$';
    req.countryCode = 'US';
    req.countryName = 'Unknown';
    next();
  }
};

/**
 * Middleware to require supported currency
 */
export const requireSupportedCurrency = (req: Request, res: Response, next: NextFunction): void => {
  if (!req.userCurrency || !isCurrencySupported(req.userCurrency)) {
    res.status(400).json({
      error: 'Currency not supported',
      message: `Currency ${req.userCurrency} is not supported for payments`,
      userCurrency: req.userCurrency,
    });
    return;
  }

  next();
};

/**
 * Extend Express Request type with geolocation and currency properties
 */
declare global {
  namespace Express {
    interface Request {
      geoLocation?: GeoLocationInfo;
      userCurrency?: string;
      currencySymbol?: string;
      countryCode?: string;
      countryName?: string;
    }
  }
}

export default {
  getGeolocationFromIP,
  getClientIP,
  detectUserCurrency,
  requireSupportedCurrency,
};
