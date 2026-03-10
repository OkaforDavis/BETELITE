// API Service
class APIService {
    constructor() {
        this.baseUrl = CONFIG.API_BASE;
        this.token = localStorage.getItem('auth_token');
    }

    async request(endpoint, options = {}) {
        const url = `${this.baseUrl}${endpoint}`;
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers
        };

        if (this.token) {
            headers['Authorization'] = `Bearer ${this.token}`;
        }

        try {
            const response = await fetch(url, {
                ...options,
                headers
            });

            if (response.status === 401) {
                this.logout();
                return { error: 'Unauthorized' };
            }

            const data = await response.json();
            
            if (!response.ok) {
                return { error: data.message || 'Request failed' };
            }

            return data;
        } catch (error) {
            console.error('API Error:', error);
            return { error: 'Network error' };
        }
    }

    async getTournaments(game) {
        // In demo mode, return demo tournaments
        if (CONFIG.DEMO_MODE || sessionStorage.getItem('demoMode')) {
            const demoTournaments = JSON.parse(sessionStorage.getItem('demoTournaments') || '[]');
            return { tournaments: demoTournaments.filter(t => !game || t.game === game) };
        }
        return this.request(`/tournaments?game=${game}`);
    }

    async getActiveBets() {
        // In demo mode, return demo bets
        if (CONFIG.DEMO_MODE || sessionStorage.getItem('demoMode')) {
            const demoBets = JSON.parse(sessionStorage.getItem('demoBets') || '[]');
            return { bets: demoBets };
        }
        return this.request('/bets/active');
    }

    async placeBet(tournamentId, amount, prediction) {
        return this.request('/bets', {
            method: 'POST',
            body: JSON.stringify({ tournamentId, amount, prediction })
        });
    }

    async getUser() {
        return this.request('/user/profile');
    }

    async updateWallet(action, amount) {
        return this.request('/wallet/update', {
            method: 'POST',
            body: JSON.stringify({ action, amount })
        });
    }

    async getMatchDetails(matchId) {
        return this.request(`/matches/${matchId}`);
    }

    async getSpectators(matchId) {
        return this.request(`/matches/${matchId}/spectators`);
    }

    setToken(token) {
        this.token = token;
        localStorage.setItem('auth_token', token);
    }

    logout() {
        this.token = null;
        localStorage.removeItem('auth_token');
        window.location.href = '/auth';
    }
}

const api = new APIService();
window.api = api;
