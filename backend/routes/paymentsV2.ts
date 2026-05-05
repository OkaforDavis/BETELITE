import express, { Request, Response, Router } from 'express';
import {
  initializePaystackTransaction,
  verifyPaystackTransaction,
  convertCurrency,
  formatCurrency,
  getCountryCurrencyList,
  getSupportedCurrencies,
} from '../services/paymentService';
import { verifyToken } from '../middleware/googleOAuth';
import { detectUserCurrency, requireSupportedCurrency } from '../middleware/geolocation';
import { db } from '../services/firebase';

const router: Router = express.Router();

// Apply geolocation detection to all payment routes
router.use(detectUserCurrency);

/**
 * ═══════════════════════════════════════════════════════════
 * PAYMENT INITIALIZATION ROUTES
 * ═══════════════════════════════════════════════════════════
 */

/**
 * POST /api/payments/initialize
 * Initialize a payment transaction
 */
router.post('/initialize', verifyToken, requireSupportedCurrency, async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    const { amount, description, tournamentId, betId, metadata = {} } = req.body;

    if (!amount || amount <= 0) {
      return res.status(400).json({ error: 'Invalid amount' });
    }

    // Add user and transaction info to metadata
    const enrichedMetadata = {
      ...metadata,
      userId: req.user.userId,
      userEmail: req.user.email,
      tournamentId: tournamentId || null,
      betId: betId || null,
      countryCode: req.countryCode,
      countryName: req.countryName,
      currency: req.userCurrency,
    };

    // Initialize Paystack transaction
    const paystackResponse = await initializePaystackTransaction(
      req.user.email,
      amount,
      req.userCurrency!,
      enrichedMetadata
    );

    if (!paystackResponse.status) {
      return res.status(400).json({ error: paystackResponse.message });
    }

    // Save transaction record to database
    const transactionRef = db.ref('payments').push();
    const transactionId = transactionRef.key;

    await transactionRef.set({
      transaction_id: transactionId,
      user_id: req.user.userId,
      email: req.user.email,
      amount,
      currency: req.userCurrency,
      status: 'pending',
      paystack_reference: paystackResponse.data.reference,
      paystack_access_code: paystackResponse.data.access_code,
      tournament_id: tournamentId || null,
      bet_id: betId || null,
      country_code: req.countryCode,
      metadata: enrichedMetadata,
      created_at: new Date(),
      updated_at: new Date(),
    });

    res.json({
      success: true,
      transaction: {
        id: transactionId,
        reference: paystackResponse.data.reference,
        accessCode: paystackResponse.data.access_code,
        authorizationUrl: paystackResponse.data.authorization_url,
        amount: formatCurrency(amount, req.userCurrency!),
        currency: req.userCurrency,
        currencySymbol: req.currencySymbol,
      },
    });
  } catch (error) {
    console.error('Initialize Payment Error:', error);
    res.status(500).json({ error: `Payment initialization failed: ${error}` });
  }
});

/**
 * POST /api/payments/verify/:reference
 * Verify a completed payment
 */
router.post('/verify/:reference', verifyToken, async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    const { reference } = req.params;

    // Verify with Paystack
    const paystackResponse = await verifyPaystackTransaction(reference);

    if (!paystackResponse.status) {
      return res.status(400).json({ error: 'Payment verification failed' });
    }

    const paymentData = paystackResponse.data;

    // Find and update transaction in database
    const transactionsRef = db.ref('payments').orderByChild('paystack_reference').equalTo(reference);
    const snapshot = await transactionsRef.once('value');

    if (!snapshot.exists()) {
      return res.status(404).json({ error: 'Transaction not found' });
    }

    const transactions = snapshot.val();
    const transactionKey = Object.keys(transactions)[0];
    const transaction = transactions[transactionKey];

    // Verify payment status
    if (paymentData.status !== 'success') {
      await db.ref(`payments/${transactionKey}`).update({
        status: 'failed',
        paystack_status: paymentData.status,
        updated_at: new Date(),
      });

      return res.status(400).json({ error: 'Payment was not successful' });
    }

    // Verify user owns this transaction
    if (transaction.user_id !== req.user.userId) {
      return res.status(403).json({ error: 'Not authorized' });
    }

    // Update transaction to successful
    await db.ref(`payments/${transactionKey}`).update({
      status: 'completed',
      paystack_status: paymentData.status,
      amount_paid: paymentData.amount / 100, // Convert from kobo
      paid_at: new Date(),
      updated_at: new Date(),
    });

    // If tournament payment, add user to tournament
    if (transaction.tournament_id) {
      await addUserToTournament(req.user.userId, transaction.tournament_id);
    }

    // If bet payment, process bet
    if (transaction.bet_id) {
      await processBetPayment(req.user.userId, transaction.bet_id);
    }

    // Add funds to user wallet
    await addToUserWallet(req.user.userId, paymentData.amount / 100, transaction.currency);

    res.json({
      success: true,
      message: 'Payment verified successfully',
      transaction: {
        reference: paymentData.reference,
        amount: formatCurrency(paymentData.amount / 100, transaction.currency),
        status: paymentData.status,
        timestamp: paymentData.paid_at,
      },
    });
  } catch (error) {
    console.error('Verify Payment Error:', error);
    res.status(500).json({ error: `Payment verification failed: ${error}` });
  }
});

/**
 * ═══════════════════════════════════════════════════════════
 * CURRENCY & CONVERSION ROUTES
 * ═══════════════════════════════════════════════════════════
 */

/**
 * GET /api/payments/currencies
 * Get list of supported currencies
 */
router.get('/currencies', async (req: Request, res: Response) => {
  try {
    const currencies = getSupportedCurrencies();

    res.json({
      success: true,
      currencies,
      userCurrency: {
        code: req.userCurrency,
        symbol: req.currencySymbol,
        countryCode: req.countryCode,
        countryName: req.countryName,
      },
    });
  } catch (error) {
    console.error('Get Currencies Error:', error);
    res.status(500).json({ error: 'Failed to get currencies' });
  }
});

/**
 * GET /api/payments/countries
 * Get list of supported countries and currencies
 */
router.get('/countries', async (req: Request, res: Response) => {
  try {
    const countries = getCountryCurrencyList();

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
    console.error('Get Countries Error:', error);
    res.status(500).json({ error: 'Failed to get countries' });
  }
});

/**
 * POST /api/payments/convert
 * Convert amount between currencies
 */
router.post('/convert', async (req: Request, res: Response) => {
  try {
    const { amount, fromCurrency, toCurrency } = req.body;

    if (!amount || !fromCurrency || !toCurrency) {
      return res.status(400).json({ error: 'Amount, fromCurrency, and toCurrency required' });
    }

    // Convert currency
    const convertedAmount = await convertCurrency(amount, fromCurrency, toCurrency);

    res.json({
      success: true,
      conversion: {
        amount,
        fromCurrency,
        toCurrency,
        convertedAmount: parseFloat(convertedAmount.toFixed(2)),
        formattedAmount: formatCurrency(amount, fromCurrency),
        formattedConverted: formatCurrency(convertedAmount, toCurrency),
      },
    });
  } catch (error) {
    console.error('Currency Conversion Error:', error);
    res.status(500).json({ error: `Conversion failed: ${error}` });
  }
});

/**
 * GET /api/payments/transactions
 * Get user's payment transactions
 */
router.get('/transactions', verifyToken, async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    const { limit = 20, offset = 0 } = req.query;

    const transactionsRef = db.ref('payments').orderByChild('user_id').equalTo(req.user.userId);
    const snapshot = await transactionsRef.once('value');

    if (!snapshot.exists()) {
      return res.json({ success: true, transactions: [] });
    }

    const allTransactions = snapshot.val();
    const transactions = Object.entries(allTransactions)
      .map(([id, data]: any) => ({
        id,
        ...data,
      }))
      .reverse()
      .slice(parseInt(offset as string), parseInt(limit as string) + parseInt(offset as string));

    res.json({
      success: true,
      count: transactions.length,
      transactions,
    });
  } catch (error) {
    console.error('Get Transactions Error:', error);
    res.status(500).json({ error: 'Failed to get transactions' });
  }
});

/**
 * ═══════════════════════════════════════════════════════════
 * DATABASE HELPER FUNCTIONS
 * ═══════════════════════════════════════════════════════════
 */

async function addUserToTournament(userId: string, tournamentId: string): Promise<void> {
  try {
    const newParticipantRef = db.ref('tournament_participants').push();

    await newParticipantRef.set({
      user_id: userId,
      tournament_id: tournamentId,
      joined_at: new Date(),
      status: 'active',
    });

    console.log(`User ${userId} added to tournament ${tournamentId}`);
  } catch (error) {
    console.error('Add User to Tournament Error:', error);
  }
}

async function processBetPayment(userId: string, betId: string): Promise<void> {
  try {
    // Mark bet as paid
    await db.ref(`bets/${betId}`).update({
      payment_status: 'completed',
      paid_at: new Date(),
    });

    console.log(`Bet ${betId} payment processed for user ${userId}`);
  } catch (error) {
    console.error('Process Bet Payment Error:', error);
  }
}

async function addToUserWallet(userId: string, amount: number, currency: string): Promise<void> {
  try {
    const walletRef = db.ref(`user_wallets/${userId}`);
    const snapshot = await walletRef.once('value');

    if (snapshot.exists()) {
      const wallet = snapshot.val();
      await walletRef.update({
        balance: wallet.balance + amount,
        total_received: (wallet.total_received || 0) + amount,
        updated_at: new Date(),
      });
    } else {
      await walletRef.set({
        user_id: userId,
        balance: amount,
        currency,
        total_received: amount,
        total_spent: 0,
        created_at: new Date(),
        updated_at: new Date(),
      });
    }

    console.log(`Added ${formatCurrency(amount, currency)} to wallet for user ${userId}`);
  } catch (error) {
    console.error('Add to Wallet Error:', error);
  }
}

export default router;
