<?php
/**
 * BETELITE MAIN ENTRY POINT
 * =========================
 * CORE_LOCK: Application bootstrap
 * All requests route through here
 */

// Prevent direct access to files
if (basename(__FILE__) !== 'index.php') {
    http_response_code(403);
    exit('Access Denied');
}

try {
    // 1. Bootstrap application
    require_once __DIR__ . '/core/bootstrap.php';
    
    // 2. Ensure HTTPS in production
    if (config('app.env') === 'production' && php_sapi_name() !== 'cli') {
        $Security->verifyHTTPS();
    }
    
    // 3. Set security headers
    $Security->setSecurityHeaders();
    
    // 4. Determine request type (API or Web)
    $path = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
    $isAPI = strpos($path, '/api/') === 0;
    
    if ($isAPI) {
        // API Request
        header('Content-Type: application/json; charset=utf-8');
        
        // ZERO_TRUST: Validate request method
        $method = $_SERVER['REQUEST_METHOD'];
        if (!in_array($method, ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'])) {
            http_response_code(405);
            echo json_encode(['error' => 'Method not allowed']);
            exit;
        }
        
        // Handle CORS preflight
        if ($method === 'OPTIONS') {
            http_response_code(200);
            exit;
        }
        
        // Route API request
        require_once __DIR__ . '/api/router.php';
        
    } else {
        // Web Request
        // Route to web pages
        require_once __DIR__ . '/web/router.php';
    }
    
} catch (Exception $e) {
    // Error handled by exception handler
    http_response_code(500);
    if (php_sapi_name() !== 'cli') {
        if (config('app.env') === 'production') {
            echo json_encode(['error' => 'An error occurred']);
        } else {
            echo json_encode(['error' => $e->getMessage()]);
        }
    }
}
