// Main Application Initialization
class App {
    async init() {
        console.log('Initializing BETELITE Mobile App...');

        // Check auth
        const token = localStorage.getItem('auth_token');
        if (!token) {
            window.location.href = '/auth';
            return;
        }

        try {
            // Connect WebSocket
            await socket.connect(localStorage.getItem('user_id'));
            console.log('✓ WebSocket connected');

            // Initialize game modules
            await eFootball.init();
            await dls.init();
            console.log('✓ Game modules initialized');

            // Load user profile
            const userResult = await api.getUser();
            if (!userResult.error) {
                this.updateUserInfo(userResult);
                console.log('✓ User profile loaded');
            }

            // Register service worker for PWA
            if ('serviceWorker' in navigator) {
                try {
                    await navigator.serviceWorker.register('/sw.js');
                    console.log('✓ Service Worker registered');
                } catch (error) {
                    console.warn('Service Worker registration failed:', error);
                }
            }

            // Request notification permission
            if ('Notification' in window && Notification.permission === 'default') {
                Notification.requestPermission();
            }

            // Set up real-time listeners
            socket.on('chat_message', (data) => {
                ui.addChatMessage(data.username, data.message, data.userId === localStorage.getItem('user_id'));
            });

            socket.on('bet_placed', (data) => {
                console.log('Bet placed:', data);
                this.showNotification('Bet Placed', `₦${data.amount} on ${data.prediction}`);
            });

            socket.on('match_ended', (data) => {
                console.log('Match ended:', data);
                this.showNotification('Match Ended', `Final score: ${data.score.team1} - ${data.score.team2}`);
            });

            console.log('✓ BETELITE Mobile App Ready!');
        } catch (error) {
            console.error('Initialization error:', error);
            this.showError('Failed to initialize app');
        }
    }

    updateUserInfo(user) {
        document.getElementById('username').textContent = user.username;
        document.getElementById('userBalance').textContent = `₦${user.wallet.balance.toFixed(2)}`;
        document.getElementById('walletBalance').textContent = `₦${user.wallet.balance.toFixed(2)}`;
        document.getElementById('totalWinnings').textContent = `₦${user.wallet.totalWinnings.toFixed(2)}`;
    }

    showNotification(title, message) {
        if ('Notification' in window && Notification.permission === 'granted') {
            new Notification(title, {
                body: message,
                tag: 'betelite-' + Date.now(),
                badge: '⚽'
            });
        }
    }

    showError(message) {
        console.error('Error:', message);
        alert(`Error: ${message}`);
    }
}

// Initialize app when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        const app = new App();
        app.init();
    });
} else {
    const app = new App();
    app.init();
}
