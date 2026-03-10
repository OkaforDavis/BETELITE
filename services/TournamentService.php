<?php
/**
 * TOURNAMENT SERVICE
 * ==================
 * CORE_LOCK: Tournament management
 * VERSION_GATE: Features based on version
 */

class TournamentService {
    
    private $db;
    private $logger;
    
    public function __construct($db, $logger) {
        $this->db = $db;
        $this->logger = $logger;
    }
    
    /**
     * Create tournament
     * CORE_LOCK: Critical operation
     */
    public function create($organizerId, $data) {
        // Validate inputs
        if (!Validator::string($data['name'], 3, 255)) {
            throw new APIException("Invalid tournament name", 400);
        }
        
        if (!Validator::integer($data['max_participants'], 2, 1000)) {
            throw new APIException("Invalid max participants", 400);
        }
        
        if (!Validator::enum($data['type'], ['solo', 'team'])) {
            throw new APIException("Invalid tournament type", 400);
        }
        
        try {
            $this->db->beginTransaction();
            
            $tournamentId = $this->db->insert('tournaments', [
                'organizer_id' => $organizerId,
                'game_id' => $data['game_id'],
                'name' => $data['name'],
                'description' => $data['description'] ?? null,
                'type' => $data['type'],
                'format' => $data['format'] ?? 'single_elimination',
                'max_participants' => $data['max_participants'],
                'entry_fee' => $data['entry_fee'] ?? 0,
                'prize_pool' => $data['prize_pool'] ?? 0,
                'registration_starts' => $data['registration_starts'],
                'registration_ends' => $data['registration_ends'],
                'starts_at' => $data['starts_at'],
                'status' => 'draft',
            ]);
            
            $this->db->commit();
            
            audit_log('tournament_created', $organizerId, [
                'tournament_id' => $tournamentId,
                'name' => $data['name'],
            ]);
            
            return ['tournament_id' => $tournamentId];
            
        } catch (Exception $e) {
            $this->db->rollback();
            throw new APIException("Failed to create tournament", 500);
        }
    }
    
    /**
     * Register participant
     * CORE_LOCK: Entry management
     */
    public function registerParticipant($tournamentId, $userId) {
        try {
            $this->db->beginTransaction();
            
            // Get tournament
            $tournament = $this->db->selectOne(
                "SELECT * FROM tournaments WHERE id = ? FOR UPDATE",
                [$tournamentId]
            );
            
            if (!$tournament) {
                throw new APIException("Tournament not found", 404);
            }
            
            // Check status
            if ($tournament['status'] !== 'open') {
                throw new APIException("Tournament registration is closed", 400);
            }
            
            // Check capacity
            if ($tournament['current_participants'] >= $tournament['max_participants']) {
                throw new APIException("Tournament is full", 400);
            }
            
            // Check if already registered
            $existing = $this->db->selectOne(
                "SELECT id FROM tournament_participants WHERE tournament_id = ? AND user_id = ?",
                [$tournamentId, $userId]
            );
            
            if ($existing) {
                throw new APIException("Already registered", 409);
            }
            
            // If entry fee, lock funds
            if ($tournament['entry_fee'] > 0) {
                $walletService = new WalletService($this->db, $this->logger);
                $walletService->lockFunds($userId, $tournament['entry_fee'], 'tournament', $tournamentId);
            }
            
            // Register participant
            $participantId = $this->db->insert('tournament_participants', [
                'tournament_id' => $tournamentId,
                'user_id' => $userId,
                'status' => 'registered',
            ]);
            
            // Update tournament participant count
            $this->db->update('tournaments', [
                'current_participants' => $tournament['current_participants'] + 1,
            ], 'id = ?', [$tournamentId]);
            
            $this->db->commit();
            
            audit_log('tournament_registered', $userId, [
                'tournament_id' => $tournamentId,
                'participant_id' => $participantId,
            ]);
            
            return ['registered' => true, 'participant_id' => $participantId];
            
        } catch (APIException $e) {
            $this->db->rollback();
            throw $e;
        } catch (Exception $e) {
            $this->db->rollback();
            throw new APIException("Registration failed", 500);
        }
    }
    
    /**
     * Start tournament
     * CORE_LOCK: Generate bracket
     */
    public function start($tournamentId) {
        try {
            $this->db->beginTransaction();
            
            $tournament = $this->db->selectOne(
                "SELECT * FROM tournaments WHERE id = ?",
                [$tournamentId]
            );
            
            if (!$tournament) {
                throw new APIException("Tournament not found", 404);
            }
            
            // Generate bracket based on format
            $this->generateBracket($tournamentId, $tournament);
            
            // Update status
            $this->db->update('tournaments', [
                'status' => 'live',
                'started_at' => date('Y-m-d H:i:s'),
            ], 'id = ?', [$tournamentId]);
            
            $this->db->commit();
            
            return ['started' => true];
            
        } catch (Exception $e) {
            $this->db->rollback();
            throw $e;
        }
    }
    
    /**
     * Generate bracket
     * FUTURE_HOOK: Advanced bracket logic
     */
    private function generateBracket($tournamentId, $tournament) {
        // Get all participants
        $participants = $this->db->select(
            "SELECT id FROM tournament_participants WHERE tournament_id = ? AND status = 'registered' ORDER BY RAND()",
            [$tournamentId]
        );
        
        // For single elimination, create first round matches
        $round = 1;
        $matchNumber = 1;
        
        for ($i = 0; $i < count($participants); $i += 2) {
            $p1 = $participants[$i]['id'];
            $p2 = isset($participants[$i + 1]) ? $participants[$i + 1]['id'] : null;
            
            $this->db->insert('matches', [
                'tournament_id' => $tournamentId,
                'round' => $round,
                'match_number' => $matchNumber,
                'player1_id' => $p1,
                'player2_id' => $p2,
                'status' => 'pending',
            ]);
            
            $matchNumber++;
        }
    }
    
    /**
     * Get tournament details
     */
    public function getTournament($tournamentId) {
        $tournament = $this->db->selectOne(
            "SELECT * FROM tournaments WHERE id = ?",
            [$tournamentId]
        );
        
        if (!$tournament) {
            throw new APIException("Tournament not found", 404);
        }
        
        // Get participants count
        $participants = $this->db->select(
            "SELECT COUNT(*) as count FROM tournament_participants WHERE tournament_id = ?",
            [$tournamentId]
        );
        
        return array_merge($tournament, $participants[0] ?? []);
    }
}
