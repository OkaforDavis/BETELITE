// UI Controller
class UIController {
    constructor() {
        this.currentTab = 'live';
        this.currentGame = CONFIG.GAMES.EFOOTBALL;
        this.setupEventListeners();
    }

    setupEventListeners() {
        // Tab navigation
        document.querySelectorAll('.nav-btn').forEach(btn => {
            btn.addEventListener('click', (e) => this.switchTab(e.target.dataset.tab));
        });

        // Game selector
        document.querySelectorAll('.game-btn').forEach(btn => {
            btn.addEventListener('click', (e) => this.switchGame(e.target.dataset.game));
        });

        // Game option selector in modal
        document.querySelectorAll('.game-option').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const game = e.currentTarget.dataset.game;
                this.selectGame(game);
            });
        });

        // Chat
        document.getElementById('chatSendBtn').addEventListener('click', () => this.sendChat());
        document.getElementById('chatInput').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') this.sendChat();
        });

        // Account buttons
        document.getElementById('depositBtn').addEventListener('click', () => this.showModal('deposit'));
        document.getElementById('withdrawBtn').addEventListener('click', () => this.showModal('withdraw'));
        document.getElementById('logoutBtn').addEventListener('click', () => this.logout());
        document.getElementById('profileBtn').addEventListener('click', () => this.switchTab('account'));

        // Modal close
        document.querySelector('.modal-close').addEventListener('click', () => this.closeModal());
    }

    switchTab(tabName) {
        // Hide all tabs
        document.querySelectorAll('.tab-content').forEach(tab => {
            tab.classList.remove('active');
        });

        // Remove active from nav buttons
        document.querySelectorAll('.nav-btn').forEach(btn => {
            btn.classList.remove('active');
        });

        // Show selected tab
        document.getElementById(`${tabName}-tab`).classList.add('active');
        document.querySelector(`.nav-btn[data-tab="${tabName}"]`).classList.add('active');

        this.currentTab = tabName;

        // Load content if needed
        if (tabName === 'tournaments') {
            this.loadTournaments();
        } else if (tabName === 'bets') {
            this.loadBets();
        } else if (tabName === 'spectate') {
            this.loadSpectators();
        }
    }

    switchGame(gameId) {
        this.currentGame = gameId;

        // Update UI
        document.querySelectorAll('.game-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.game === gameId);
        });

        this.loadTournaments();
    }

    selectGame(gameId) {
        this.currentGame = gameId;
        this.closeModal();
        this.switchTab('tournaments');
    }

    async loadTournaments() {
        const listEl = document.getElementById('tournamentsList');
        listEl.innerHTML = '<div class="spinner"></div>';

        const gameModule = this.currentGame === CONFIG.GAMES.EFOOTBALL ? eFootball : dls;
        const result = await gameModule.getTournaments();

        if (result.error) {
            listEl.innerHTML = `<div class="empty-state">Error: ${result.error}</div>`;
            return;
        }

        if (!result.tournaments || result.tournaments.length === 0) {
            listEl.innerHTML = '<div class="empty-state">No tournaments available</div>';
            return;
        }

        listEl.innerHTML = result.tournaments.map(t => `
            <div class="tournament-item" onclick="ui.joinTournament('${t.id}')">
                <div class="match-header">
                    <span class="match-status">${t.status}</span>
                    <span>${t.players}/${t.maxPlayers} players</span>
                </div>
                <div class="match-teams">
                    <div class="team-info">
                        <div class="team-name">${t.team1}</div>
                    </div>
                    <div class="vs">VS</div>
                    <div class="team-info">
                        <div class="team-name">${t.team2}</div>
                    </div>
                </div>
                <div class="match-meta">
                    <span>Entry: ₦${t.entryFee}</span>
                    <span>Prize: ₦${t.prizePool}</span>
                </div>
            </div>
        `).join('');
    }

    async loadBets() {
        const poolsEl = document.getElementById('bettingPools');
        poolsEl.innerHTML = '<div class="spinner"></div>';

        const result = await api.getActiveBets();

        if (result.error) {
            poolsEl.innerHTML = `<div class="empty-state">Error: ${result.error}</div>`;
            return;
        }

        if (!result.bets || result.bets.length === 0) {
            poolsEl.innerHTML = '<div class="empty-state">No active betting pools</div>';
            return;
        }

        poolsEl.innerHTML = result.bets.map(b => `
            <div class="bet-item">
                <div class="match-teams">
                    <div class="team-info">
                        <div class="team-name">${b.team1}</div>
                    </div>
                    <div class="vs">VS</div>
                    <div class="team-info">
                        <div class="team-name">${b.team2}</div>
                    </div>
                </div>
                <div class="match-meta">
                    <span>Pool: ₦${b.totalPool}</span>
                    <span>Odds: ${b.odds}</span>
                </div>
                <button class="btn btn-primary" onclick="ui.openBettingModal('${b.id}')">
                    Place Bet
                </button>
            </div>
        `).join('');
    }

    async loadSpectators() {
        const listEl = document.getElementById('spectatorList');
        listEl.innerHTML = '<div class="spinner"></div>';

        // Load live matches available to spectate
        const gameModule = this.currentGame === CONFIG.GAMES.EFOOTBALL ? eFootball : dls;
        const result = await gameModule.getTournaments();

        if (result.error) {
            listEl.innerHTML = `<div class="empty-state">Error: ${result.error}</div>`;
            return;
        }

        const liveMatches = (result.tournaments || []).filter(t => t.status === 'live');

        if (liveMatches.length === 0) {
            listEl.innerHTML = '<div class="empty-state">No live matches to watch</div>';
            return;
        }

        listEl.innerHTML = liveMatches.map(m => `
            <div class="spectator-item" onclick="ui.startWatching('${m.id}')">
                <div class="match-teams">
                    <div class="team-info">
                        <div class="team-name">${m.team1}</div>
                        <div class="team-score">${m.score?.team1 || 0}</div>
                    </div>
                    <div class="vs">VS</div>
                    <div class="team-info">
                        <div class="team-name">${m.team2}</div>
                        <div class="team-score">${m.score?.team2 || 0}</div>
                    </div>
                </div>
                <div class="match-meta">
                    <span class="spectators">👁️ ${m.spectators || 0} watching</span>
                    <span>${m.timeElapsed}'</span>
                </div>
            </div>
        `).join('');
    }

    async joinTournament(tournamentId) {
        const tournament = await api.getMatchDetails(tournamentId);
        if (tournament.error) {
            alert('Error joining tournament');
            return;
        }

        const gameModule = this.currentGame === CONFIG.GAMES.EFOOTBALL ? eFootball : dls;
        await gameModule.startMatch(tournament);

        socket.joinMatch(tournamentId);
        this.showLiveViewer();
    }

    async startWatching(matchId) {
        socket.joinMatch(matchId);
        this.showLiveViewer();
    }

    showLiveViewer() {
        document.getElementById('liveGameViewer').classList.remove('hidden');
        document.getElementById('live-tab').classList.add('active');
        document.querySelectorAll('.nav-btn').forEach(btn => btn.classList.remove('active'));
        document.querySelector('.nav-btn[data-tab="live"]').classList.add('active');
    }

    sendChat() {
        const input = document.getElementById('chatInput');
        const message = input.value.trim();

        if (!message || message.length > CONFIG.CHAT_MAX_LENGTH) {
            return;
        }

        const gameModule = this.currentGame === CONFIG.GAMES.EFOOTBALL ? eFootball : dls;
        socket.sendChat(gameModule.currentMatch?.id, message);

        input.value = '';
        input.focus();
    }

    addChatMessage(username, message, isOwn = false) {
        const messagesEl = document.getElementById('chatMessages');
        const msgEl = document.createElement('div');
        msgEl.className = `chat-message ${isOwn ? 'own' : ''}`;
        msgEl.innerHTML = `
            ${!isOwn ? `<div class="message-user">${username}</div>` : ''}
            <div class="message-content">${this.escapeHtml(message)}</div>
        `;
        messagesEl.appendChild(msgEl);
        messagesEl.scrollTop = messagesEl.scrollHeight;
    }

    openBettingModal(betId) {
        // Implementation for betting
        alert('Betting feature coming soon');
    }

    showModal(type) {
        alert(`${type} feature coming soon`);
    }

    closeModal() {
        document.getElementById('gameModal').classList.add('hidden');
    }

    logout() {
        if (confirm('Are you sure you want to logout?')) {
            api.logout();
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

const ui = new UIController();
window.ui = ui;
