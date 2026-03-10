<?php
/**
 * BETELITE BOOTSTRAP
 * ==================
 * CORE_LOCK: Critical initialization sequence
 * ZERO_TRUST: All services initialized securely
 * 
 * Initialization order is CRITICAL - do not reorder
 */

// 1. Load configuration
$CONFIG = require_once __DIR__ . '/../config.php';

// 2. Set timezone
date_default_timezone_set($CONFIG['app']['timezone']);

// 3. Initialize logging first
require_once __DIR__ . '/logger.php';
$Logger = new Logger($CONFIG['logging']);

// 4. Load core services
require_once __DIR__ . '/security.php';
require_once __DIR__ . '/validator.php';
require_once __DIR__ . '/errors.php';
require_once __DIR__ . '/router.php';

// 5. Initialize database connection
require_once __DIR__ . '/../core/database.php';
try {
    $DB = new Database($CONFIG['database'], $Logger);
} catch (Exception $e) {
    $Logger->critical("Database connection failed: " . $e->getMessage());
    // In development mode, allow graceful degradation
    if ($CONFIG['app']['env'] !== 'development') {
        http_response_code(503);
        exit("Service Unavailable");
    }
    // Create a dummy DB object for dev mode
    $DB = null;
    $Logger->warning("Database unavailable - running in degraded mode");
}

// 6. Start secure session
session_set_cookie_params([
    'lifetime' => $CONFIG['session']['timeout'],
    'path' => '/',
    'domain' => parse_url($CONFIG['app']['url'], PHP_URL_HOST),
    'secure' => $CONFIG['session']['secure_cookie'],
    'httponly' => $CONFIG['session']['httponly'],
    'samesite' => $CONFIG['session']['samesite'],
]);

session_name($CONFIG['session']['name']);
session_start();

// 7. Initialize security layer
$Security = new Security($CONFIG['security'], $Logger);

// 8. Set error handlers
set_exception_handler([new ErrorHandler($Logger), 'handleException']);
set_error_handler([new ErrorHandler($Logger), 'handleError']);
register_shutdown_function([new ErrorHandler($Logger), 'handleShutdown']);

// 9. AUDIT_TRACE: Log startup
$Logger->info("[BOOTSTRAP] BETELITE v" . BETELITE_VERSION . " initialized", [
    'environment' => $CONFIG['app']['env'],
    'database' => $CONFIG['database']['host'],
    'timestamp' => date('Y-m-d H:i:s'),
]);

// 10. Make global variables available
global $CONFIG, $DB, $Logger, $Security;

/**
 * Helper function to access configuration
 */
function config($key, $default = null) {
    global $CONFIG;
    $keys = explode('.', $key);
    $value = $CONFIG;
    
    foreach ($keys as $k) {
        if (is_array($value) && isset($value[$k])) {
            $value = $value[$k];
        } else {
            return $default;
        }
    }
    
    return $value;
}

/**
 * Helper function to access database
 */
function db() {
    global $DB;
    return $DB;
}

/**
 * Helper function to access logger
 */
function log_info($message, $context = []) {
    global $Logger;
    $Logger->info($message, $context);
}

/**
 * Helper function for audit logging
 * AUDIT_TRACE: All financial/security events
 */
function audit_log($action, $user_id, $details) {
    global $Logger;
    $Logger->audit($action, [
        'user_id' => $user_id,
        'action' => $action,
        'timestamp' => time(),
        'ip' => $_SERVER['REMOTE_ADDR'] ?? 'unknown',
        'details' => $details,
    ]);
}

// Bootstrap complete
define('BETELITE_BOOTSTRAPPED', true);
