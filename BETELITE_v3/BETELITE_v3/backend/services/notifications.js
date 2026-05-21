const { db } = require('./firebase');
const { v4: uuid } = require('uuid');

/**
 * Creates a notification for a specific user in Firebase Realtime Database
 * @param {string} userId - Firebase User UID
 * @param {string} type - 'deposit', 'withdrawal', 'wager', 'tournament', 'system'
 * @param {string} title - Short title for the notification
 * @param {string} message - Full message body
 * @param {object} metadata - Extra data (e.g. amount, matchId, tournamentId)
 */
async function sendNotification(userId, type, title, message, metadata = {}) {
  try {
    if (!db) {
      console.warn('[NOTIFICATIONS] Firebase DB not initialized. Skipping notification.');
      return;
    }
    
    if (!userId) return;

    const notifId = uuid();
    const notification = {
      id: notifId,
      type,
      title,
      message,
      metadata,
      read: false,
      timestamp: Date.now()
    };

    // Save to user's notifications node
    await db.ref(`users/${userId}/notifications/${notifId}`).set(notification);
    
    // Also emit a socket event if the user is connected (handled in server.js if needed, but Firebase .on() is better)
    return notification;
  } catch (error) {
    console.error('[NOTIFICATIONS] Error sending notification:', error);
  }
}

module.exports = { sendNotification };
