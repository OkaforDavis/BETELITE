<?php
/**
 * BETELITE MAIN ENTRY POINT
 * =========================
 */

// Get request path
$requestUri = $_SERVER['REQUEST_URI'] ?? '/';
$requestPath = parse_url($requestUri, PHP_URL_PATH);

// Serve mobile app static files
if (strpos($requestPath, '/mobile') === 0) {
    $filePath = __DIR__ . $requestPath;
    
    // If it's a directory, serve index.html
    if (is_dir($filePath)) {
        $filePath = rtrim($filePath, '/') . '/index.html';
    }
    
    if (is_file($filePath)) {
        // Determine content type
        $ext = pathinfo($filePath, PATHINFO_EXTENSION);
        $mimeTypes = [
            'html' => 'text/html',
            'css' => 'text/css',
            'js' => 'application/javascript',
            'json' => 'application/json',
            'png' => 'image/png',
            'jpg' => 'image/jpeg',
            'gif' => 'image/gif',
            'svg' => 'image/svg+xml',
            'ico' => 'image/x-icon'
        ];
        
        header('Content-Type: ' . ($mimeTypes[$ext] ?? 'application/octet-stream'));
        header('Cache-Control: public, max-age=3600');
        readfile($filePath);
        exit;
    }
}

// Serve landing page
if ($requestPath === '/' || $requestPath === '/index.php' || $requestPath === '/index.html') {
    header('Content-Type: text/html');
    readfile(__DIR__ . '/landing.html');
    exit;
}

// If trying to access API, bootstrap application
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
