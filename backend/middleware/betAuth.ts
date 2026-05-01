import { Request, Response, NextFunction } from 'express';
import { db } from '../services/firebase';
import { verifyToken } from './googleOAuth';

/**
 * Middleware to enforce login requirement for bet access
 * Checks if user is authenticated before allowing access to betting endpoints
 */
export const requireBetAccess = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  try {
    // Verify JWT token first
    const token = req.headers.authorization?.split(' ')[1];

    if (!token) {
      res.status(401).json({
        error: 'Authentication required',
        message: 'You must be logged in to access betting features',
        requireLogin: true,
      });
      return;
    }

    // Use the verifyToken middleware
    verifyToken(req, res, async () => {
      if (!req.user) {
        res.status(401).json({ error: 'Invalid authentication' });
        return;
      }

      try {
        // Verify user exists and account is active
        const user = await getUserById(req.user.userId);

        if (!user) {
          res.status(404).json({ error: 'User not found' });
          return;
        }

        // Check account status
        if (user.account_status === 'banned') {
          res.status(403).json({
            error: 'Account banned',
            message: 'Your account has been banned and cannot place bets',
          });
          return;
        }

        if (user.account_status === 'suspended') {
          res.status(403).json({
            error: 'Account suspended',
            message: 'Your account is currently suspended. Please contact support.',
          });
          return;
        }

        // Check if email is verified
        if (!user.email_verified && user.account_status === 'pending_verification') {
          res.status(403).json({
            error: 'Email not verified',
            message: 'Please verify your email before placing bets',
            requireEmailVerification: true,
          });
          return;
        }

        // Check if user has KYC verified (for higher bet amounts)
        req.user.kycVerified = user.kyc_verified || false;

        // User is authorized
        next();
      } catch (error) {
        console.error('Bet Access Check Error:', error);
        res.status(500).json({ error: 'Authentication verification failed' });
      }
    });
  } catch (error) {
    console.error('Bet Access Middleware Error:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
};

/**
 * Middleware to check if user has KYC verification for high-value bets
 */
export const requireKYCForHighBets = (maxBetWithoutKYC: number = 1000) => {
  return async (req: Request, res: Response, next: NextFunction): Promise<void> => {
    try {
      if (!req.user) {
        res.status(401).json({ error: 'Not authenticated' });
        return;
      }

      const { bet_amount } = req.body;

      if (!bet_amount) {
        res.status(400).json({ error: 'Bet amount required' });
        return;
      }

      // Check if bet amount exceeds limit
      if (bet_amount > maxBetWithoutKYC && !req.user.kycVerified) {
        res.status(403).json({
          error: 'KYC verification required',
          message: `Bets over ${maxBetWithoutKYC} require KYC verification`,
          maxBetWithoutKYC,
          requireKYC: true,
        });
        return;
      }

      next();
    } catch (error) {
      console.error('KYC Check Error:', error);
      res.status(500).json({ error: 'KYC verification check failed' });
    }
  };
};

/**
 * Middleware to log all betting activity for audit trail
 */
export const logBetActivity = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  try {
    if (!req.user) {
      next();
      return;
    }

    const activityData = {
      user_id: req.user.userId,
      action: req.method,
      endpoint: req.path,
      bet_data: req.body,
      ip_address: req.ip,
      user_agent: req.get('user-agent'),
      timestamp: new Date(),
    };

    // Save to audit log asynchronously (don't block request)
    await saveBetActivityLog(activityData);

    next();
  } catch (error) {
    console.error('Activity Log Error:', error);
    // Don't fail the request if logging fails
    next();
  }
};

/**
 * Extend Express Request type with user and kyc properties
 */
declare global {
  namespace Express {
    interface Request {
      user?: {
        userId: string;
        email: string;
        username: string;
        iat: number;
        kycVerified?: boolean;
      };
    }
  }
}

/**
 * ═══════════════════════════════════════════════════════════
 * DATABASE HELPER FUNCTIONS
 * ═══════════════════════════════════════════════════════════
 */

interface User {
  user_id: string;
  email: string;
  username: string;
  account_status: string;
  email_verified: boolean;
  kyc_verified: boolean;
}

async function getUserById(userId: string): Promise<User | null> {
  try {
    const snapshot = await db.ref(`users/${userId}`).once('value');
    if (snapshot.exists()) {
      return { user_id: userId, ...snapshot.val() };
    }
    return null;
  } catch (error) {
    console.error('Get User Error:', error);
    return null;
  }
}

async function saveBetActivityLog(activity: any): Promise<void> {
  try {
    const newLogRef = db.ref('bet_activity_logs').push();
    await newLogRef.set(activity);
  } catch (error) {
    console.error('Save Activity Log Error:', error);
  }
}

export default {
  requireBetAccess,
  requireKYCForHighBets,
  logBetActivity,
};
