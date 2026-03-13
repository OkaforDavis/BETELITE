const express = require('express');
const { db } = require('../services/firebase');
const bcrypt = require('bcrypt');
const jwt = require('jsonwebtoken');
const nodemailer = require('nodemailer');

const JWT_SECRET = process.env.JWT_SECRET || 'betelite-secret-key-change-in-production';
const JWT_EXPIRE = 7 * 24 * 60 * 60; // 7 days

// Setup email service (Gmail SMTP - configure in .env)
const emailService = nodemailer.createTransport({
  service: 'gmail',
  auth: {
    user: process.env.GMAIL_USER || 'your-email@gmail.com',
    pass: process.env.GMAIL_PASSWORD || 'your-app-password',
  },
});

module.exports = (io, engine) => {
  const r = express.Router();

  // ══════════════════════════════════════
  // REGISTER
  // ══════════════════════════════════════
  r.post('/register', async (req, res) => {
    try {
      const { email, password, username } = req.body;

      // Validation
      if (!email || !password || !username) {
        return res.status(400).json({ error: 'Missing email, password, or username' });
      }

      if (password.length < 6) {
        return res.status(400).json({ error: 'Password must be at least 6 characters' });
      }

      // Check if user exists
      const snapshot = await db.ref('users').orderByChild('email').equalTo(email).once('value');
      if (snapshot.exists()) {
        return res.status(409).json({ error: 'Email already registered' });
      }

      // Hash password
      const hashedPassword = await bcrypt.hash(password, 10);

      // Create user in Firebase
      const newUserRef = db.ref('users').push();
      const userId = newUserRef.key;

      await newUserRef.set({
        userId,
        email,
        username,
        password: hashedPassword,
        createdAt: Date.now(),
        emailVerified: false,
        role: 'player', // 'player' or 'admin'
        profilePicture: null,
        wallet: { NGN: 0, GHS: 0 },
        stats: { totalBets: 0, wins: 0, losses: 0, earnings: 0 },
        gameHistory: [],
        achievements: [],
        referralCode: generateRefCode(),
        referredBy: null,
        paymentKeys: { paystackKey: null, flutterwaveKey: null },
      });

      // Generate JWT token
      const token = jwt.sign({ userId, email, role: 'player' }, JWT_SECRET, { expiresIn: JWT_EXPIRE });

      res.status(201).json({
        ok: true,
        userId,
        email,
        token,
        message: 'Account created successfully',
      });
    } catch (e) {
      console.error('[AUTH REGISTER]', e.message);
      res.status(500).json({ ok: false, error: e.message });
    }
  });

  // ══════════════════════════════════════
  // LOGIN
  // ══════════════════════════════════════
  r.post('/login', async (req, res) => {
    try {
      const { email, password } = req.body;

      if (!email || !password) {
        return res.status(400).json({ error: 'Email and password required' });
      }

      // Find user by email
      const snapshot = await db.ref('users').orderByChild('email').equalTo(email).once('value');
      if (!snapshot.exists()) {
        return res.status(401).json({ error: 'Invalid email or password' });
      }

      const users = snapshot.val();
      const userData = Object.values(users)[0];
      const userId = Object.keys(users)[0];

      // Verify password
      const passwordMatch = await bcrypt.compare(password, userData.password);
      if (!passwordMatch) {
        return res.status(401).json({ error: 'Invalid email or password' });
      }

      // Generate JWT token
      const token = jwt.sign(
        { userId, email, role: userData.role || 'player' },
        JWT_SECRET,
        { expiresIn: JWT_EXPIRE }
      );

      res.json({
        ok: true,
        userId,
        email: userData.email,
        username: userData.username,
        token,
        role: userData.role || 'player',
        profilePicture: userData.profilePicture,
      });
    } catch (e) {
      console.error('[AUTH LOGIN]', e.message);
      res.status(500).json({ ok: false, error: e.message });
    }
  });

  // ══════════════════════════════════════
  // FORGOT PASSWORD
  // ══════════════════════════════════════
  r.post('/forgot-password', async (req, res) => {
    try {
      const { email } = req.body;

      if (!email) {
        return res.status(400).json({ error: 'Email required' });
      }

      // Find user
      const snapshot = await db.ref('users').orderByChild('email').equalTo(email).once('value');
      if (!snapshot.exists()) {
        return res.status(404).json({ error: 'Email not found' });
      }

      const users = snapshot.val();
      const userId = Object.keys(users)[0];
      const userData = Object.values(users)[0];

      // Generate reset token (valid for 1 hour)
      const resetToken = jwt.sign({ userId, email }, JWT_SECRET, { expiresIn: 3600 });

      // Store reset token in Firebase
      await db.ref(`users/${userId}`).update({
        passwordResetToken: resetToken,
        passwordResetExpires: Date.now() + 3600000, // 1 hour
      });

      // Send email with reset link
      const resetLink = `${process.env.FRONTEND_URL || 'https://betelite-60181385.firebaseapp.com'}/reset-password?token=${resetToken}`;

      await emailService.sendMail({
        from: process.env.GMAIL_USER,
        to: email,
        subject: 'BETELITE - Password Reset',
        html: `
          <h2>Password Reset Request</h2>
          <p>Click the link below to reset your password:</p>
          <a href="${resetLink}" style="padding:10px 20px; background:#F5C518; color:#000; text-decoration:none; border-radius:5px;">
            Reset Password
          </a>
          <p>This link expires in 1 hour.</p>
          <p>If you didn't request this, ignore this email.</p>
        `,
      });

      res.json({
        ok: true,
        message: 'Password reset link sent to email',
      });
    } catch (e) {
      console.error('[AUTH FORGOT]', e.message);
      res.status(500).json({ ok: false, error: e.message });
    }
  });

  // ══════════════════════════════════════
  // RESET PASSWORD
  // ══════════════════════════════════════
  r.post('/reset-password', async (req, res) => {
    try {
      const { token, newPassword } = req.body;

      if (!token || !newPassword) {
        return res.status(400).json({ error: 'Token and new password required' });
      }

      // Verify token
      let decoded;
      try {
        decoded = jwt.verify(token, JWT_SECRET);
      } catch (e) {
        return res.status(400).json({ error: 'Invalid or expired token' });
      }

      // Get user and verify token matches
      const snapshot = await db.ref(`users`).orderByChild('email').equalTo(decoded.email).once('value');
      if (!snapshot.exists()) {
        return res.status(404).json({ error: 'User not found' });
      }

      const users = snapshot.val();
      const userId = Object.keys(users)[0];
      const userData = Object.values(users)[0];

      if (userData.passwordResetToken !== token || userData.passwordResetExpires < Date.now()) {
        return res.status(400).json({ error: 'Token expired or invalid' });
      }

      // Hash new password
      const hashedPassword = await bcrypt.hash(newPassword, 10);

      // Update password and clear reset token
      await db.ref(`users/${userId}`).update({
        password: hashedPassword,
        passwordResetToken: null,
        passwordResetExpires: null,
      });

      res.json({
        ok: true,
        message: 'Password reset successfully',
      });
    } catch (e) {
      console.error('[AUTH RESET]', e.message);
      res.status(500).json({ ok: false, error: e.message });
    }
  });

  // ══════════════════════════════════════
  // GET USER PROFILE
  // ══════════════════════════════════════
  r.get('/profile/:userId', async (req, res) => {
    try {
      const { userId } = req.params;

      const snapshot = await db.ref(`users/${userId}`).once('value');
      if (!snapshot.exists()) {
        return res.status(404).json({ error: 'User not found' });
      }

      const userData = snapshot.val();

      // Don't send password hash
      delete userData.password;
      delete userData.passwordResetToken;

      res.json({
        ok: true,
        user: userData,
      });
    } catch (e) {
      console.error('[AUTH PROFILE]', e.message);
      res.status(500).json({ ok: false, error: e.message });
    }
  });

  // ══════════════════════════════════════
  // UPDATE PROFILE
  // ══════════════════════════════════════
  r.post('/profile/:userId', async (req, res) => {
    try {
      const { userId } = req.params;
      const { username, profilePicture } = req.body;

      const updates = {};
      if (username) updates.username = username;
      if (profilePicture) updates.profilePicture = profilePicture;

      await db.ref(`users/${userId}`).update(updates);

      res.json({
        ok: true,
        message: 'Profile updated',
      });
    } catch (e) {
      console.error('[AUTH PROFILE UPDATE]', e.message);
      res.status(500).json({ ok: false, error: e.message });
    }
  });

  // ══════════════════════════════════════
  // VERIFY TOKEN
  // ══════════════════════════════════════
  r.get('/verify/:token', (req, res) => {
    try {
      const { token } = req.params;
      const decoded = jwt.verify(token, JWT_SECRET);
      res.json({ ok: true, user: decoded });
    } catch (e) {
      res.status(401).json({ ok: false, error: 'Invalid token' });
    }
  });

  // ══════════════════════════════════════
  // GOOGLE OAUTH CALLBACK
  // ══════════════════════════════════════
  r.post('/google-oauth', async (req, res) => {
    try {
      const { idToken, googleUser } = req.body;
      const { email, name, picture } = googleUser;

      // Check if user exists
      const snapshot = await db.ref('users').orderByChild('email').equalTo(email).once('value');

      let userId;
      if (snapshot.exists()) {
        // Update existing user
        const users = snapshot.val();
        userId = Object.keys(users)[0];
        await db.ref(`users/${userId}`).update({
          profilePicture: picture,
          lastLogin: Date.now(),
        });
      } else {
        // Create new user
        const newUserRef = db.ref('users').push();
        userId = newUserRef.key;

        await newUserRef.set({
          userId,
          email,
          username: name,
          password: null, // OAuth users have no password
          createdAt: Date.now(),
          emailVerified: true,
          role: 'player',
          profilePicture: picture,
          wallet: { NGN: 0, GHS: 0 },
          stats: { totalBets: 0, wins: 0, losses: 0, earnings: 0 },
          gameHistory: [],
          achievements: [],
          referralCode: generateRefCode(),
          referredBy: null,
          authMethod: 'google',
        });
      }

      // Generate JWT token
      const token = jwt.sign({ userId, email, role: 'player' }, JWT_SECRET, { expiresIn: JWT_EXPIRE });

      res.json({
        ok: true,
        userId,
        email,
        username: name,
        token,
        profilePicture: picture,
        message: 'Google login successful',
      });
    } catch (e) {
      console.error('[AUTH GOOGLE]', e.message);
      res.status(500).json({ ok: false, error: e.message });
    }
  });

  return r;
};

// Helper: Generate referral code
function generateRefCode() {
  return 'BET' + Math.random().toString(36).substring(2, 8).toUpperCase();
}
