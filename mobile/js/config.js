// BETELITE Config
const CONFIG = {
    API_BASE: process.env.API_URL || 'http://localhost:3000/api',
    WS_URL: process.env.WS_URL || 'ws://localhost:3000',
    GAMES: {
        EFOOTBALL: 'efootball',
        DLS: 'dls'
    },
    DETECTION_API: 'http://localhost:5000/api/detect',
    MAX_RECONNECT_ATTEMPTS: 5,
    RECONNECT_DELAY: 3000,
    CHAT_MAX_LENGTH: 150,
    MATCH_UPDATE_INTERVAL: 1000,
    HEARTBEAT_INTERVAL: 30000
};

// Expose globally
window.CONFIG = CONFIG;
