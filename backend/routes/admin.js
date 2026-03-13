const admin = require('firebase-admin');

module.exports = (io, engine) => {
  const router = require('express').Router();
  const db = admin.database();
  
  // Middleware: Check if user is admin
  const requireAdmin = async (req, res, next) => {
    const userId = req.headers['x-user-id'];
    const token = req.headers['authorization']?.replace('Bearer ', '');
    
    if (!userId || !token) return res.status(401).json({ ok: false, error: 'Unauthorized' });
    
    try {
      const userSnap = await db.ref(`users/${userId}`).get();
      const user = userSnap.val();
      
      if (!user || user.role !== 'admin') {
        return res.status(403).json({ ok: false, error: 'Admin access required' });
      }
      
      req.admin = true;
      req.userId = userId;
      next();
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  };
  
  // GET admin config
  router.get('/config', async (req, res) => {
    try {
      const snap = await db.ref('admin/config').get();
      const config = snap.val() || {
        paymentMethods: {},
        featureFlags: {}
      };
      res.json({ ok: true, config });
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  });
  
  // SET payment keys (admin only)
  router.post('/config/payment', requireAdmin, async (req, res) => {
    const { paystack_enabled, flutterwave_enabled } = req.body;
    
    try {
      await db.ref('admin/config/paymentMethods').set({
        paystack: paystack_enabled ? true : false,
        flutterwave: flutterwave_enabled ? true : false,
        updatedAt: new Date().toISOString(),
        updatedBy: req.userId
      });
      
      // Broadcast to all connected clients
      io.emit('admin:config-updated', {
        paymentMethods: {
          paystack: paystack_enabled ? true : false,
          flutterwave: flutterwave_enabled ? true : false
        }
      });
      
      res.json({ ok: true, message: 'Payment config updated' });
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  });
  
  // SET feature flags (admin only)
  router.post('/config/features', requireAdmin, async (req, res) => {
    const { flags } = req.body;
    
    try {
      await db.ref('admin/config/featureFlags').set({
        ...flags,
        updatedAt: new Date().toISOString(),
        updatedBy: req.userId
      });
      
      io.emit('admin:features-updated', flags);
      
      res.json({ ok: true, message: 'Features updated' });
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  });
  
  // GET all users (admin only)
  router.get('/users', requireAdmin, async (req, res) => {
    try {
      const snap = await db.ref('users').get();
      const users = snap.val() || {};
      
      const userList = Object.keys(users).map(uid => ({
        id: uid,
        email: users[uid].email,
        username: users[uid].username,
        role: users[uid].role,
        wallet: users[uid].wallet,
        createdAt: users[uid].createdAt
      }));
      
      res.json({ ok: true, users: userList });
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  });
  
  // UPDATE user role (admin only)
  router.post('/users/:userId/role', requireAdmin, async (req, res) => {
    const { userId } = req.params;
    const { role } = req.body;
    
    if (!['player', 'admin', 'moderator'].includes(role)) {
      return res.status(400).json({ ok: false, error: 'Invalid role' });
    }
    
    try {
      await db.ref(`users/${userId}/role`).set(role);
      
      res.json({ ok: true, message: `User role updated to ${role}` });
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  });
  
  // GET analytics (admin only)
  router.get('/analytics', requireAdmin, async (req, res) => {
    try {
      const usersSnap = await db.ref('users').get();
      const users = usersSnap.val() || {};
      
      const totalUsers = Object.keys(users).length;
      const totalWalletNGN = Object.values(users).reduce((sum, u) => sum + (u.wallet?.NGN || 0), 0);
      const totalWalletGHS = Object.values(users).reduce((sum, u) => sum + (u.wallet?.GHS || 0), 0);
      const totalBets = Object.values(users).reduce((sum, u) => sum + (u.stats?.totalBets || 0), 0);
      
      res.json({ 
        ok: true, 
        analytics: {
          totalUsers,
          totalWalletNGN,
          totalWalletGHS,
          totalBets,
          timestamp: new Date().toISOString()
        } 
      });
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  });
  
  // UPDATE user wallet (admin only)
  router.post('/users/:userId/wallet', requireAdmin, async (req, res) => {
    const { userId } = req.params;
    const { currency, amount, action } = req.body; // action: 'set' or 'add'
    
    try {
      const userSnap = await db.ref(`users/${userId}`).get();
      const user = userSnap.val();
      
      if (!user) return res.status(404).json({ ok: false, error: 'User not found' });
      
      const current = user.wallet?.[currency] || 0;
      const newAmount = action === 'add' ? current + amount : amount;
      
      await db.ref(`users/${userId}/wallet/${currency}`).set(newAmount);
      
      res.json({ ok: true, newAmount });
    } catch (e) {
      res.status(500).json({ ok: false, error: e.message });
    }
  });
  
  return router;
};
