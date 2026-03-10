// eFootball Game Module
class eFootballGame {
    constructor() {
        this.name = 'eFootball';
        this.gameId = CONFIG.GAMES.EFOOTBALL;
        this.currentMatch = null;
        this.initialized = false;
    }

    async init() {
        console.log('Initializing eFootball');
        socket.on('match_update', (data) => this.handleMatchUpdate(data));
        socket.on('goal_detected', (data) => this.handleGoalDetected(data));
        socket.on('offside_detected', (data) => this.handleOffside(data));
        this.initialized = true;
    }

    async getTournaments() {
        return api.getTournaments(this.gameId);
    }

    async startMatch(tournament) {
        // eFootball specific match initialization
        this.currentMatch = {
            id: tournament.id,
            game: this.gameId,
            status: 'starting',
            team1: tournament.team1,
            team2: tournament.team2,
            score: { team1: 0, team2: 0 },
            startedAt: new Date(),
            goals: [],
            offsides: [],
            detectionStats: {
                goalsDetected: 0,
                offsidesDetected: 0,
                possessionTeam1: 50,
                possessionTeam2: 50
            }
        };

        // Emit to detection service
        this.sendForDetection();
        
        return this.currentMatch;
    }

    async sendForDetection() {
        if (!this.currentMatch) return;

        try {
            const response = await fetch(CONFIG.DETECTION_API, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    matchId: this.currentMatch.id,
                    game: this.gameId,
                    matchData: this.currentMatch
                })
            });

            if (response.ok) {
                const detectionStream = await response.json();
                console.log('Detection started for eFootball match');
            }
        } catch (error) {
            console.error('Detection service error:', error);
        }
    }

    handleMatchUpdate(data) {
        if (data.game !== this.gameId || data.matchId !== this.currentMatch?.id) return;

        this.currentMatch.score = data.score;
        this.currentMatch.status = data.status;
        this.currentMatch.detectionStats = data.detectionStats || this.currentMatch.detectionStats;

        this.updateUI();
    }

    handleGoalDetected(data) {
        if (data.game !== this.gameId || data.matchId !== this.currentMatch?.id) return;

        const goal = {
            timestamp: new Date(),
            team: data.team,
            player: data.player || 'Auto-detected',
            confidence: data.confidence
        };

        this.currentMatch.goals.push(goal);
        this.currentMatch.detectionStats.goalsDetected++;
        
        console.log(`GOAL! ${data.team} - Confidence: ${data.confidence}%`);
        this.notifyGoal(goal);
        this.updateUI();
    }

    handleOffside(data) {
        if (data.game !== this.gameId) return;

        const offside = {
            timestamp: new Date(),
            team: data.team,
            confidence: data.confidence
        };

        this.currentMatch.offsides.push(offside);
        this.currentMatch.detectionStats.offsidesDetected++;
        
        console.log(`Offside detected - ${data.team} - Confidence: ${data.confidence}%`);
        this.updateUI();
    }

    notifyGoal(goal) {
        if ('Notification' in window && Notification.permission === 'granted') {
            new Notification('GOAL!', {
                body: `${goal.team} just scored!`,
                tag: 'goal-' + Date.now(),
                badge: '⚽'
            });
        }
    }

    updateUI() {
        if (!this.currentMatch) return;

        document.getElementById('matchScore').textContent = 
            `${this.currentMatch.score.team1} - ${this.currentMatch.score.team2}`;
        document.getElementById('goalsDetected').textContent = 
            this.currentMatch.detectionStats.goalsDetected;
        document.getElementById('offsidesDetected').textContent = 
            this.currentMatch.detectionStats.offsidesDetected;
    }
}

const eFootball = new eFootballGame();
window.eFootball = eFootball;
