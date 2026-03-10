// WebSocket Service
class SocketService {
    constructor() {
        this.socket = null;
        this.reconnectAttempts = 0;
        this.listeners = {};
        this.messageQueue = [];
    }

    connect(userId) {
        return new Promise((resolve, reject) => {
            try {
                this.socket = new WebSocket(`${CONFIG.WS_URL}?token=${localStorage.getItem('auth_token')}`);

                this.socket.onopen = () => {
                    console.log('WebSocket connected');
                    this.reconnectAttempts = 0;
                    this.flushQueue();
                    this.startHeartbeat();
                    resolve();
                };

                this.socket.onmessage = (event) => {
                    const message = JSON.parse(event.data);
                    this.emit(message.type, message.data);
                };

                this.socket.onerror = (error) => {
                    console.error('WebSocket error:', error);
                    reject(error);
                };

                this.socket.onclose = () => {
                    console.log('WebSocket disconnected');
                    this.attemptReconnect();
                };
            } catch (error) {
                reject(error);
            }
        });
    }

    attemptReconnect() {
        if (this.reconnectAttempts < CONFIG.MAX_RECONNECT_ATTEMPTS) {
            this.reconnectAttempts++;
            setTimeout(() => {
                console.log(`Reconnecting (${this.reconnectAttempts}/${CONFIG.MAX_RECONNECT_ATTEMPTS})...`);
                this.connect(localStorage.getItem('user_id'));
            }, CONFIG.RECONNECT_DELAY);
        }
    }

    emit(type, data) {
        if (!this.listeners[type]) return;
        this.listeners[type].forEach(callback => callback(data));
    }

    on(type, callback) {
        if (!this.listeners[type]) {
            this.listeners[type] = [];
        }
        this.listeners[type].push(callback);
    }

    off(type, callback) {
        if (!this.listeners[type]) return;
        this.listeners[type] = this.listeners[type].filter(cb => cb !== callback);
    }

    send(type, data) {
        const message = JSON.stringify({ type, data });

        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            this.socket.send(message);
        } else {
            this.messageQueue.push(message);
        }
    }

    flushQueue() {
        while (this.messageQueue.length > 0 && this.socket.readyState === WebSocket.OPEN) {
            this.socket.send(this.messageQueue.shift());
        }
    }

    startHeartbeat() {
        setInterval(() => {
            if (this.socket && this.socket.readyState === WebSocket.OPEN) {
                this.send('ping', {});
            }
        }, CONFIG.HEARTBEAT_INTERVAL);
    }

    joinMatch(matchId) {
        this.send('join_match', { matchId });
    }

    leaveMatch(matchId) {
        this.send('leave_match', { matchId });
    }

    sendChat(matchId, message) {
        this.send('chat_message', { matchId, message });
    }

    placeBet(matchId, prediction, amount) {
        this.send('place_bet', { matchId, prediction, amount });
    }

    disconnect() {
        if (this.socket) {
            this.socket.close();
        }
    }
}

const socket = new SocketService();
window.socket = socket;
