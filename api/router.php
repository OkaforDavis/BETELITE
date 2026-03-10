<?php
/**
 * API ROUTER
 * ==========
 * CORE_LOCK: API endpoint routing
 */

// Load services
require_once __DIR__ . '/../services/AuthService.php';
require_once __DIR__ . '/../services/WalletService.php';
require_once __DIR__ . '/../services/TournamentService.php';
require_once __DIR__ . '/../services/PaymentService.php';

// Instantiate router
$router = new Router();

// ============================================================
// AUTH ENDPOINTS
// ============================================================

$router->post('/api/auth/register', function($params) {
    global $DB, $Security, $Logger;
    
    $data = json_decode(file_get_contents('php://input'), true);
    
    $authService = new AuthService($DB, $Security, $Logger);
    return $authService->register(
        $data['email'] ?? null,
        $data['username'] ?? null,
        $data['password'] ?? null,
        $data['confirm_password'] ?? null
    );
});

$router->post('/api/auth/login', function($params) {
    global $DB, $Security, $Logger;
    
    $data = json_decode(file_get_contents('php://input'), true);
    
    $authService = new AuthService($DB, $Security, $Logger);
    return $authService->login(
        $data['email'] ?? null,
        $data['password'] ?? null
    );
});

$router->post('/api/auth/logout', function($params) {
    global $DB, $Security, $Logger;
    
    // Get token from header
    $token = $this->getAuthToken();
    $payload = $Security->verifyJWT($token);
    
    if (!$payload) {
        http_response_code(401);
        return ['error' => 'Unauthorized'];
    }
    
    $authService = new AuthService($DB, $Security, $Logger);
    return $authService->logout($payload['user_id'], $params['session_token'] ?? null);
});

// ============================================================
// WALLET ENDPOINTS
// ============================================================

$router->get('/api/wallet', function($params) {
    global $DB, $Logger, $Security;
    
    $token = $this->getAuthToken();
    $payload = $Security->verifyJWT($token);
    
    if (!$payload) {
        http_response_code(401);
        return ['error' => 'Unauthorized'];
    }
    
    $walletService = new WalletService($DB, $Logger);
    return $walletService->getWallet($payload['user_id']);
});

$router->post('/api/wallet/deposit', function($params) {
    global $DB, $Logger, $Security;
    
    $token = $this->getAuthToken();
    $payload = $Security->verifyJWT($token);
    
    if (!$payload) {
        http_response_code(401);
        return ['error' => 'Unauthorized'];
    }
    
    $data = json_decode(file_get_contents('php://input'), true);
    
    $paymentService = new PaymentService($DB, $Logger);
    return $paymentService->initiate($payload['user_id'], $data['amount'] ?? null, 'deposit');
});

// ============================================================
// TOURNAMENT ENDPOINTS
// ============================================================

$router->post('/api/tournaments', function($params) {
    global $DB, $Logger, $Security;
    
    $token = $this->getAuthToken();
    $payload = $Security->verifyJWT($token);
    
    if (!$payload) {
        http_response_code(401);
        return ['error' => 'Unauthorized'];
    }
    
    $data = json_decode(file_get_contents('php://input'), true);
    
    $tournamentService = new TournamentService($DB, $Logger);
    return $tournamentService->create($payload['user_id'], $data);
});

$router->post('/api/tournaments/:tournament_id/register', function($params) {
    global $DB, $Logger, $Security;
    
    $token = $this->getAuthToken();
    $payload = $Security->verifyJWT($token);
    
    if (!$payload) {
        http_response_code(401);
        return ['error' => 'Unauthorized'];
    }
    
    $tournamentService = new TournamentService($DB, $Logger);
    return $tournamentService->registerParticipant($params['tournament_id'], $payload['user_id']);
});

// ============================================================
// HEALTH CHECK
// ============================================================

$router->get('/api/health', function($params) {
    return [
        'status' => 'ok',
        'version' => BETELITE_VERSION,
        'timestamp' => date('Y-m-d H:i:s'),
    ];
});

// ============================================================
// DISPATCH REQUEST
// ============================================================

header('Content-Type: application/json; charset=utf-8');

try {
    $response = $router->dispatch();
    
    if (is_array($response) || is_object($response)) {
        echo json_encode($response, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    } else {
        echo $response;
    }
    
} catch (APIException $e) {
    http_response_code($e->getStatusCode());
    echo json_encode([
        'error' => true,
        'message' => $e->getMessage(),
        'code' => $e->getErrorCode(),
    ]);
} catch (Exception $e) {
    http_response_code(500);
    echo json_encode([
        'error' => true,
        'message' => 'Internal server error',
    ]);
}

// ============================================================
// HELPER: Get authorization token
// ============================================================
function getAuthToken() {
    $header = $_SERVER['HTTP_AUTHORIZATION'] ?? '';
    if (preg_match('/Bearer\s+(.+)/', $header, $matches)) {
        return $matches[1];
    }
    return null;
}
