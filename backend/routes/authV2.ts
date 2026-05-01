import express, { Request, Response, Router } from 'express';
import {
  getGoogleAuthURL,
  getGoogleTokens,
  getGoogleUserInfo,
  verifyToken,
  generateJWT,
} from '../middleware/googleOAuth';
import { db } from '../services/firebase';
import bcrypt from 'bcrypt';

const router: Router = express.Router();

/**
 * ═══════════════════════════════════════════════════════════
 * GOOGLE OAUTH ROUTES
 * ═══════════════════════════════════════════════════════════
 */

/**
 * GET /api/auth/google
 * Redirect user to Google OAuth login
 */
router.get('/google', (req: Request, res: Response) => {
  const authURL = getGoogleAuthURL();
  res.redirect(authURL);
});

/**
 * GET /api/auth/google/callback
 * Handle Google OAuth callback
 */
router.get('/google/callback', async (req: Request, res: Response) => {
  try {
    const { code } = req.query;

    if (!code || typeof code !== 'string') {
      return res.status(400).json({ error: 'No authorization code provided' });
    }

    // Exchange code for tokens
    const tokens = await getGoogleTokens(code);

    // Get user info
    const googleUser = await getGoogleUserInfo(tokens.access_token);

    // Check if user exists in database
    let user = await getUserByEmail(googleUser.email);

    if (!user) {
      // Create new user
      user = await createGoogleUser({
        googleId: googleUser.id,
        email: googleUser.email,
        username: googleUser.name.replace(/\s+/g, '_').toLowerCase(),
        displayName: googleUser.name,
        avatarUrl: googleUser.picture,
        emailVerified: googleUser.email_verified,
      });
    }

    // Generate JWT
    const token = generateJWT(user.user_id, user.email, user.username);

    // Redirect to frontend with token
    const redirectURL = `${process.env.FRONTEND_URL || 'http://localhost:3000'}/auth/success?token=${token}&user=${JSON.stringify(
      user
    )}`;

    res.redirect(redirectURL);
  } catch (error) {
    console.error('Google OAuth Callback Error:', error);
    res.status(500).json({ error: 'Authentication failed' });
  }
});

/**
 * ═══════════════════════════════════════════════════════════
 * LOGIN & REGISTER ROUTES
 * ═══════════════════════════════════════════════════════════
 */

/**
 * POST /api/auth/login
 * Traditional email/password login
 */
router.post('/login', async (req: Request, res: Response) => {
  try {
    const { email, password } = req.body;

    if (!email || !password) {
      return res.status(400).json({ error: 'Email and password required' });
    }

    // Find user
    const user = await getUserByEmail(email);
    if (!user) {
      return res.status(401).json({ error: 'Invalid email or password' });
    }

    // Verify password
    const isPasswordValid = await bcrypt.compare(password, user.password_hash);
    if (!isPasswordValid) {
      return res.status(401).json({ error: 'Invalid email or password' });
    }

    // Check if account is active
    if (user.account_status !== 'active' && user.account_status !== 'pending_verification') {
      return res.status(403).json({ error: `Account is ${user.account_status}` });
    }

    // Generate JWT
    const token = generateJWT(user.user_id, user.email, user.username);

    res.json({
      success: true,
      token,
      user: {
        user_id: user.user_id,
        email: user.email,
        username: user.username,
        user_type: user.user_type,
        account_status: user.account_status,
      },
    });
  } catch (error) {
    console.error('Login Error:', error);
    res.status(500).json({ error: 'Login failed' });
  }
});

/**
 * POST /api/auth/register
 * Register new user with email and password
 */
router.post('/register', async (req: Request, res: Response) => {
  try {
    const { email, password, username } = req.body;

    // Validation
    if (!email || !password || !username) {
      return res.status(400).json({ error: 'Email, password, and username required' });
    }

    if (password.length < 8) {
      return res.status(400).json({ error: 'Password must be at least 8 characters' });
    }

    // Check if user already exists
    const existingUser = await getUserByEmail(email);
    if (existingUser) {
      return res.status(409).json({ error: 'Email already registered' });
    }

    // Hash password
    const hashedPassword = await bcrypt.hash(password, 10);

    // Create user
    const user = await createUser({
      email,
      username,
      password_hash: hashedPassword,
      user_type: 'player',
      account_status: 'pending_verification',
      email_verified: false,
    });

    // Generate JWT
    const token = generateJWT(user.user_id, user.email, user.username);

    res.status(201).json({
      success: true,
      token,
      message: 'User registered successfully. Please verify your email.',
      user: {
        user_id: user.user_id,
        email: user.email,
        username: user.username,
      },
    });
  } catch (error) {
    console.error('Register Error:', error);
    res.status(500).json({ error: 'Registration failed' });
  }
});

/**
 * ═══════════════════════════════════════════════════════════
 * PROTECTED ROUTES (REQUIRE LOGIN)
 * ═══════════════════════════════════════════════════════════
 */

/**
 * GET /api/auth/me
 * Get current user info
 */
router.get('/me', verifyToken, async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    const user = await getUserById(req.user.userId);
    if (!user) {
      return res.status(404).json({ error: 'User not found' });
    }

    res.json({
      user: {
        user_id: user.user_id,
        email: user.email,
        username: user.username,
        user_type: user.user_type,
        account_status: user.account_status,
        email_verified: user.email_verified,
      },
    });
  } catch (error) {
    console.error('Get User Error:', error);
    res.status(500).json({ error: 'Failed to get user info' });
  }
});

/**
 * ═══════════════════════════════════════════════════════════
 * DATABASE HELPER FUNCTIONS
 * ═══════════════════════════════════════════════════════════
 */

interface User {
  user_id: string;
  email: string;
  username: string;
  password_hash: string;
  user_type: string;
  account_status: string;
  email_verified: boolean;
  created_at?: Date;
}

async function getUserByEmail(email: string): Promise<User | null> {
  try {
    const snapshot = await db.ref('users').orderByChild('email').equalTo(email).once('value');
    if (snapshot.exists()) {
      const users = snapshot.val();
      const userId = Object.keys(users)[0];
      return { ...users[userId], user_id: userId };
    }
    return null;
  } catch (error) {
    console.error('Get User by Email Error:', error);
    return null;
  }
}

async function getUserById(userId: string): Promise<User | null> {
  try {
    const snapshot = await db.ref(`users/${userId}`).once('value');
    if (snapshot.exists()) {
      return { ...snapshot.val(), user_id: userId };
    }
    return null;
  } catch (error) {
    console.error('Get User by ID Error:', error);
    return null;
  }
}

interface CreateUserInput {
  email: string;
  username: string;
  password_hash: string;
  user_type: string;
  account_status: string;
  email_verified: boolean;
}

async function createUser(data: CreateUserInput): Promise<User> {
  const newUserRef = db.ref('users').push();
  const userId = newUserRef.key as string;

  const user: User = {
    user_id: userId,
    email: data.email,
    username: data.username,
    password_hash: data.password_hash,
    user_type: data.user_type,
    account_status: data.account_status,
    email_verified: data.email_verified,
    created_at: new Date(),
  };

  await newUserRef.set(user);
  return user;
}

interface GoogleUserInput {
  googleId: string;
  email: string;
  username: string;
  displayName: string;
  avatarUrl?: string;
  emailVerified: boolean;
}

async function createGoogleUser(data: GoogleUserInput): Promise<User> {
  const newUserRef = db.ref('users').push();
  const userId = newUserRef.key as string;

  // Generate a random password hash for OAuth users (they won't use it)
  const randomPassword = await bcrypt.hash(Math.random().toString(), 10);

  const user: User = {
    user_id: userId,
    email: data.email,
    username: data.username,
    password_hash: randomPassword,
    user_type: 'player',
    account_status: 'active',
    email_verified: data.emailVerified,
    created_at: new Date(),
  };

  // Store user with Google ID for reference
  await newUserRef.set({
    ...user,
    google_id: data.googleId,
    display_name: data.displayName,
    avatar_url: data.avatarUrl,
  });

  return user;
}

export default router;
