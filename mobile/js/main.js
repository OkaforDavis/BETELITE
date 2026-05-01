// Main Application Initialization
class App {
    async init() {
        console.log('Initializing BETELITE Mobile App...');
        console.log('Environment:', { DEMO_MODE: CONFIG.DEMO_MODE, IS_GITHUB_PAGES: CONFIG.IS_GITHUB_PAGES });

        // On GitHub Pages, run in demo mode
        if (CONFIG.DEMO_MODE) {
            this.initDemoMode();
            return;
        }

        // Firebase onAuthStateChanged in index.html will handle auth state and show App or Auth screen.
        // We just wait for it.
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

    initDemoMode() {
        console.log('🎮 Running in DEMO MODE (GitHub Pages)');

        // Set demo authentication
        localStorage.setItem('auth_token', 'demo-token-' + Date.now());
        localStorage.setItem('user_id', 'demo-user-001');

        // Update UI with demo user
        document.getElementById('username').textContent = 'Demo Player';
        document.getElementById('userBalance').textContent = '₦50,000.00';
        document.getElementById('walletBalance').textContent = '₦50,000.00';
        document.getElementById('totalWinnings').textContent = '₦5,200.00';

        // Load demo data
        this.loadDemoData();

        console.log('✓ BETELITE Demo Mode Ready!');
        console.log('Tip: Join tournaments, place bets, and spectate matches with demo data');
    }

    loadDemoData() {
        // Demo tournaments
        const demoTournaments = [
            {
                id: 'tournament-1',
                name: 'Elite Championship',
                game: 'efootball',
                status: 'live',
                teams: {
                    team1: { name: 'Team Alpha', logo: '🟦', score: 2 },
                    team2: { name: 'Dragons FC', logo: '🐉', score: 1 }
                },
                minBet: 500,
                maxBet: 50000,
                odds: { team1: 1.85, team2: 2.10, draw: 3.50 },
                prize: '₦100,000',
                entrants: 342,
                timeRemaining: '18:45',
                predictions: []
            },
            {
                id: 'tournament-2',
                name: 'Quick Fire League',
                game: 'dls',
                status: 'live',
                teams: {
                    team1: { name: 'Thunder United', logo: '⚡', score: 3 },
                    team2: { name: 'Phoenix Rising', logo: '🔥', score: 2 }
                },
                minBet: 300,
                maxBet: 30000,
                odds: { team1: 1.95, team2: 2.00, draw: 3.25 },
                prize: '₦75,000',
                entrants: 256,
                timeRemaining: '12:30',
                predictions: []
            },
            {
                id: 'tournament-3',
                name: 'Casual Play Cup',
                game: 'efootball',
                status: 'upcoming',
                teams: {
                    team1: { name: 'Legends FC', logo: '👑', score: 0 },
                    team2: { name: 'City Stars', logo: '⭐', score: 0 }
                },
                minBet: 200,
                maxBet: 20000,
                odds: { team1: 1.90, team2: 2.05, draw: 3.40 },
                prize: '₦50,000',
                entrants: 189,
                timeRemaining: '05:15',
                predictions: []
            }
        ];

        // Demo bets
        const demoBets = [
            {
                id: 'bet-1',
                tournament: 'Elite Championship',
                team: 'Team Alpha',
                amount: 5000,
                odds: 1.85,
                potential: 9250,
                status: 'live'
            },
            {
                id: 'bet-2',
                tournament: 'Quick Fire League',
                team: 'Thunder United',
                amount: 2000,
                odds: 1.95,
                potential: 3900,
                status: 'live'
            }
        ];

        // Save to sessionStorage
        sessionStorage.setItem('demoTournaments', JSON.stringify(demoTournaments));
        sessionStorage.setItem('demoBets', JSON.stringify(demoBets));
        sessionStorage.setItem('demoMode', 'true');
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
