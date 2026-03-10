<?php
/**
 * BETELITE CONFIGURATION LOADER
 * =============================
 * CORE_LOCK: Critical configuration bootstrap
 * ZERO_TRUST: Load and validate all config
 */

// Error reporting - Disabled in production
if (getenv('APP_ENV') === 'production') {
    error_reporting(0);
    ini_set('display_errors', 0);
} else {
    error_reporting(E_ALL);
    ini_set('display_errors', 1);
}

// Load version control
require_once __DIR__ . '/version.php';

/**
 * Load environment variables from .env file
 * ZERO_TRUST: Validate all values
 */
function loadEnvFile($filePath) {
    if (!file_exists($filePath)) {
        throw new Exception(".env file not found at: $filePath");
    }
    
    $lines = file($filePath, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
    foreach ($lines as $line) {
        // Skip comments and empty lines
        if (strpos(trim($line), '#') === 0) continue;
        if (empty(trim($line))) continue;
        
        // Parse KEY=VALUE
        if (strpos($line, '=') !== false) {
            list($key, $value) = explode('=', $line, 2);
            $key = trim($key);
            $value = trim($value);
            
            // Remove quotes if present
            if ((strpos($value, '"') === 0 && substr($value, -1) === '"') ||
                (strpos($value, "'") === 0 && substr($value, -1) === "'")) {
                $value = substr($value, 1, -1);
            }
            
            // ZERO_TRUST: Validate sensitive keys
            if (in_array($key, ['DB_PASSWORD', 'JWT_SECRET', 'PAYSTACK_SECRET_KEY'])) {
                if (strlen($value) < 8) {
                    throw new Exception("$key must be at least 8 characters");
                }
            }
            
            putenv("$key=$value");
            $_ENV[$key] = $value;
        }
    }
}

// Load configuration
$envFile = __DIR__ . '/.env';
if (file_exists($envFile)) {
    loadEnvFile($envFile);
} else {
    // Fallback to .env.example in development
    if (getenv('APP_ENV') !== 'production') {
        error_log("WARNING: .env file not found. Using defaults.");
    }
}

/**
 * Global Configuration Array
 * CORE_LOCK: Do not modify at runtime
 */
$CONFIG = [
    // Application
    'app' => [
        'name' => getenv('APP_NAME') ?: 'BETELITE',
        'version' => BETELITE_VERSION,
        'env' => getenv('APP_ENV') ?: 'production',
        'debug' => getenv('APP_DEBUG') === 'true',
        'url' => getenv('APP_URL') ?: 'https://betelite.example.com',
        'timezone' => 'UTC',
    ],
    
    // Database Configuration
    'database' => [
        'host' => getenv('DB_HOST') ?: 'localhost',
        'port' => (int)(getenv('DB_PORT') ?: 3306),
        'name' => getenv('DB_NAME') ?: 'betelite_db',
        'user' => getenv('DB_USER') ?: 'betelite_user',
        'password' => getenv('DB_PASSWORD') ?: '',
        'charset' => getenv('DB_CHARSET') ?: 'utf8mb4',
        'strict_mode' => getenv('DB_STRICT_MODE') === 'true',
        'connection_timeout' => 10,
        'max_connections' => 50,
    ],
    
    // Session Configuration
    'session' => [
        'timeout' => (int)(getenv('SESSION_TIMEOUT') ?: 3600),
        'secure_cookie' => getenv('SESSION_SECURE_COOKIE') === 'true',
        'httponly' => getenv('SESSION_HTTPONLY') === 'true',
        'samesite' => getenv('SESSION_SAMESITE') ?: 'Strict',
        'name' => 'betelite_session',
    ],
    
    // Security
    'security' => [
        'csrf_enabled' => true,
        'csrf_token_length' => (int)(getenv('CSRF_TOKEN_LENGTH') ?: 32),
        'jwt_secret' => getenv('JWT_SECRET') ?: '',
        'jwt_algorithm' => getenv('JWT_ALGORITHM') ?: 'HS256',
        'jwt_expiry' => (int)(getenv('JWT_EXPIRY') ?: 86400),
        'password_algo' => PASSWORD_ARGON2ID,
        'password_options' => [
            'memory_cost' => 19456,  // 19MB
            'time_cost' => 4,
            'threads' => 1,
        ],
    ],
    
    // Rate Limiting
    'rate_limit' => [
        'enabled' => getenv('RATE_LIMIT_ENABLED') === 'true',
        'requests' => (int)(getenv('RATE_LIMIT_REQUESTS') ?: 100),
        'window' => (int)(getenv('RATE_LIMIT_WINDOW') ?: 3600),
        'ip_throttle_enabled' => getenv('IP_THROTTLE_ENABLED') === 'true',
        'ip_throttle_attempts' => (int)(getenv('IP_THROTTLE_ATTEMPTS') ?: 5),
        'ip_throttle_lockout' => (int)(getenv('IP_THROTTLE_LOCKOUT') ?: 900),
    ],
    
    // Logging - AUDIT_TRACE
    'logging' => [
        'level' => getenv('LOG_LEVEL') ?: 'info',
        'file' => __DIR__ . '/' . (getenv('LOG_FILE') ?: 'logs/betelite.log'),
        'audit_file' => __DIR__ . '/' . (getenv('AUDIT_LOG_FILE') ?: 'logs/audit.log'),
        'error_file' => __DIR__ . '/' . (getenv('ERROR_LOG_FILE') ?: 'logs/errors.log'),
        'max_size' => 10485760,  // 10MB
        'retention_days' => 90,
    ],
    
    // Payment Gateway - SAFE_SWAP
    'payment' => [
        'provider' => getenv('PAYMENT_PROVIDER') ?: 'paystack',
        'paystack' => [
            'public_key' => getenv('PAYSTACK_PUBLIC_KEY') ?: '',
            'secret_key' => getenv('PAYSTACK_SECRET_KEY') ?: '',
            'verify_url' => getenv('PAYSTACK_VERIFY_URL') ?: 'https://api.paystack.co/transaction/verify',
        ],
        'flutterwave' => [
            'public_key' => getenv('FLUTTERWAVE_PUBLIC_KEY') ?: '',
            'secret_key' => getenv('FLUTTERWAVE_SECRET_KEY') ?: '',
            'encryption_key' => getenv('FLUTTERWAVE_ENCRYPTION_KEY') ?: '',
        ],
    ],
    
    // Wallet & Escrow - CORE_LOCK
    'wallet' => [
        'enabled' => VersionGate::isEnabled('wallet'),
        'min_withdrawal' => (int)(getenv('MIN_WITHDRAWAL') ?: 1000),
        'max_withdrawal' => (int)(getenv('MAX_WITHDRAWAL') ?: 1000000),
        'withdrawal_fee_percent' => (float)(getenv('WITHDRAWAL_FEE_PERCENT') ?: 0.5),
        'precision' => (int)(getenv('WALLET_PRECISION') ?: 2),
        'escrow_enabled' => getenv('ESCROW_ENABLED') === 'true',
    ],
    
    // Streaming - SAFE_SWAP
    'streaming' => [
        'provider' => getenv('STREAMING_PROVIDER') ?: 'cloudflare',
        'cloudflare' => [
            'account_id' => getenv('CLOUDFLARE_ACCOUNT_ID') ?: '',
            'auth_token' => getenv('CLOUDFLARE_AUTH_TOKEN') ?: '',
            'api_url' => getenv('CLOUDFLARE_STREAM_API_URL') ?: 'https://api.cloudflare.com/client/v4/accounts/{account_id}/stream',
        ],
        'mux' => [
            'access_token' => getenv('MUX_ACCESS_TOKEN') ?: '',
            'api_url' => getenv('MUX_API_URL') ?: 'https://api.mux.com',
        ],
    ],
    
    // Betting Engine
    'betting' => [
        'enabled' => VersionGate::isEnabled('betting'),
        'min_bet_amount' => (int)(getenv('MIN_BET_AMOUNT') ?: 100),
        'max_bet_amount' => (int)(getenv('MAX_BET_AMOUNT') ?: 100000),
        'max_bet_ratio' => (float)(getenv('MAX_BET_RATIO') ?: 0.5),
        'precision' => (int)(getenv('ODDS_PRECISION') ?: 2),
    ],
    
    // Games Supported
    'games' => [
        'fifa' => true,
        'pes' => true,
        'cod' => true,
        'pubg' => true,
        'valorant' => true,
        'fortnite' => true,
        'cs2' => true,
        'dota2' => true,
        'lol' => true,
    ],
];

/**
 * Ensure database configuration is valid
 * ZERO_TRUST: Always validate before use
 */
if (empty($CONFIG['database']['password']) && $CONFIG['app']['env'] === 'production') {
    throw new Exception("Database password not configured in production");
}

return $CONFIG;
