const jwt = require('jsonwebtoken');
const prisma = require('../prismaClient');

const JWT_SECRET = process.env.JWT_SECRET || 'betelite-secret-key-change-in-production';

const authenticateUser = async (req, res, next) => {
  try {
    const authHeader = req.headers.authorization;
    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      return res.status(401).json({ error: 'Unauthorized: Missing or invalid token' });
    }

    const token = authHeader.split(' ')[1];
    const decoded = jwt.verify(token, JWT_SECRET);

    // Verify user exists in SQL
    const user = await prisma.user.findUnique({
      where: { email: decoded.email }
    });

    if (!user) {
      return res.status(401).json({ error: 'Unauthorized: User not found in database' });
    }

    if (user.accountStatus !== 'active' && user.accountStatus !== 'pending_verification') {
      return res.status(403).json({ error: `Forbidden: Account is ${user.accountStatus}` });
    }

    req.user = user;
    next();
  } catch (error) {
    if (error.name === 'TokenExpiredError') {
      return res.status(401).json({ error: 'Unauthorized: Token expired' });
    }
    return res.status(401).json({ error: 'Unauthorized: Invalid token' });
  }
};

module.exports = { authenticateUser, JWT_SECRET };
