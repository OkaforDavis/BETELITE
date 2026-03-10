<?php
/**
 * BETELITE SECURITY LAYER
 * =======================
 * ZERO_TRUST: Never trust client input
 * CORE_LOCK: Critical security functions
 * 
 * Security features:
 * - CSRF token generation and validation
 * - Input sanitization
 * - SQL injection prevention (prepared statements)
 * - XSS prevention
 * - Session security
 * - JWT token handling
 */

class Security {
    
    private $config;
    private $logger;
    private $tokenCache = [];
    
    public function __construct($config, $logger = null) {
        $this->config = $config;
        $this->logger = $logger;
    }
    
    /**
     * Generate CSRF token
     * CORE_LOCK: Critical for form submissions
     */
    public function generateCSRFToken() {
        if (!isset($_SESSION['csrf_token'])) {
            $_SESSION['csrf_token'] = bin2hex(random_bytes($this->config['csrf_token_length']));
        }
        return $_SESSION['csrf_token'];
    }
    
    /**
     * Validate CSRF token
     * ZERO_TRUST: Always verify
     */
    public function validateCSRFToken($token) {
        if (!isset($_SESSION['csrf_token'])) {
            return false;
        }
        
        $valid = hash_equals($_SESSION['csrf_token'], $token ?? '');
        
        if (!$valid && $this->logger) {
            $this->logger->warning('CSRF token validation failed', [
                'ip' => $_SERVER['REMOTE_ADDR'] ?? 'unknown',
                'session_id' => session_id(),
            ]);
        }
        
        return $valid;
    }
    
    /**
     * Hash password with Argon2id
     * CORE_LOCK: Password storage
     */
    public function hashPassword($password) {
        if (strlen($password) < 8) {
            throw new Exception("Password must be at least 8 characters");
        }
        
        return password_hash($password, $this->config['password_algo'], $this->config['password_options']);
    }
    
    /**
     * Verify password
     * ZERO_TRUST: Always use proper comparison
     */
    public function verifyPassword($password, $hash) {
        return password_verify($password, $hash);
    }
    
    /**
     * Sanitize string input
     * ZERO_TRUST: Remove all dangerous characters
     */
    public function sanitizeString($input, $maxLength = null) {
        // Remove null bytes
        $input = str_replace("\0", '', $input);
        
        // HTML entity encode
        $input = htmlspecialchars($input, ENT_QUOTES | ENT_HTML5, 'UTF-8');
        
        // Trim whitespace
        $input = trim($input);
        
        // Enforce max length
        if ($maxLength !== null) {
            $input = substr($input, 0, $maxLength);
        }
        
        return $input;
    }
    
    /**
     * Sanitize email
     * ZERO_TRUST: Validate and normalize
     */
    public function sanitizeEmail($email) {
        $email = trim(strtolower($email));
        
        if (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
            throw new Exception("Invalid email format");
        }
        
        return $email;
    }
    
    /**
     * Sanitize integer
     * ZERO_TRUST: Strict type checking
     */
    public function sanitizeInt($input) {
        $value = filter_var($input, FILTER_VALIDATE_INT);
        if ($value === false) {
            throw new Exception("Invalid integer value");
        }
        return $value;
    }
    
    /**
     * Sanitize float
     * ZERO_TRUST: Strict type checking
     */
    public function sanitizeFloat($input) {
        $value = filter_var($input, FILTER_VALIDATE_FLOAT);
        if ($value === false) {
            throw new Exception("Invalid float value");
        }
        return $value;
    }
    
    /**
     * Validate phone number (basic)
     * ZERO_TRUST: Strict validation
     */
    public function sanitizePhone($phone) {
        // Remove non-numeric characters
        $phone = preg_replace('/[^0-9+]/', '', $phone);
        
        // Check length (10-15 digits)
        if (strlen($phone) < 10 || strlen($phone) > 15) {
            throw new Exception("Invalid phone number");
        }
        
        return $phone;
    }
    
    /**
     * Generate JWT token
     * CORE_LOCK: Authentication token
     */
    public function generateJWT($payload) {
        if (empty($this->config['jwt_secret'])) {
            throw new Exception("JWT secret not configured");
        }
        
        $header = json_encode(['alg' => $this->config['jwt_algorithm'], 'typ' => 'JWT']);
        $payload['iat'] = time();
        $payload['exp'] = time() + $this->config['jwt_expiry'];
        
        $payload = json_encode($payload);
        
        $headerEncoded = rtrim(strtr(base64_encode($header), '+/', '-_'), '=');
        $payloadEncoded = rtrim(strtr(base64_encode($payload), '+/', '-_'), '=');
        
        $signature = hash_hmac('sha256', $headerEncoded . '.' . $payloadEncoded, $this->config['jwt_secret'], true);
        $signatureEncoded = rtrim(strtr(base64_encode($signature), '+/', '-_'), '=');
        
        return $headerEncoded . '.' . $payloadEncoded . '.' . $signatureEncoded;
    }
    
    /**
     * Verify JWT token
     * ZERO_TRUST: Always verify signature and expiry
     */
    public function verifyJWT($token) {
        if (empty($this->config['jwt_secret'])) {
            throw new Exception("JWT secret not configured");
        }
        
        $parts = explode('.', $token);
        if (count($parts) !== 3) {
            return null;
        }
        
        list($headerEncoded, $payloadEncoded, $signatureEncoded) = $parts;
        
        // Verify signature
        $signature = hash_hmac('sha256', $headerEncoded . '.' . $payloadEncoded, $this->config['jwt_secret'], true);
        $signatureExpected = rtrim(strtr(base64_encode($signature), '+/', '-_'), '=');
        
        if (!hash_equals($signatureExpected, $signatureEncoded)) {
            return null;
        }
        
        // Decode payload
        $payload = json_decode(base64_decode(strtr($payloadEncoded, '-_', '+/')), true);
        
        if (!$payload) {
            return null;
        }
        
        // Check expiry
        if (isset($payload['exp']) && $payload['exp'] < time()) {
            return null;
        }
        
        return $payload;
    }
    
    /**
     * Get client IP address
     * ZERO_TRUST: Consider proxies
     */
    public function getClientIP() {
        // Check for proxy headers (be careful - can be spoofed)
        if (!empty($_SERVER['HTTP_CLIENT_IP'])) {
            $ip = $_SERVER['HTTP_CLIENT_IP'];
        } elseif (!empty($_SERVER['HTTP_X_FORWARDED_FOR'])) {
            // Use first IP if multiple
            $ips = explode(',', $_SERVER['HTTP_X_FORWARDED_FOR']);
            $ip = trim($ips[0]);
        } else {
            $ip = $_SERVER['REMOTE_ADDR'] ?? 'unknown';
        }
        
        // Validate IP format
        if (filter_var($ip, FILTER_VALIDATE_IP)) {
            return $ip;
        }
        
        return 'unknown';
    }
    
    /**
     * Generate secure random token
     */
    public function generateToken($length = 32) {
        return bin2hex(random_bytes($length));
    }
    
    /**
     * Set secure headers
     * CORE_LOCK: HTTP security headers
     */
    public function setSecurityHeaders() {
        // Prevent clickjacking
        header('X-Frame-Options: DENY');
        
        // Prevent MIME sniffing
        header('X-Content-Type-Options: nosniff');
        
        // Enable XSS protection
        header('X-XSS-Protection: 1; mode=block');
        
        // Referrer policy
        header('Referrer-Policy: strict-origin-when-cross-origin');
        
        // Content Security Policy (restrictive)
        header("Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:;");
        
        // Strict Transport Security (HTTPS only)
        header('Strict-Transport-Security: max-age=31536000; includeSubDomains');
        
        // Cache control for sensitive content
        header('Cache-Control: no-store, no-cache, must-revalidate, max-age=0');
        header('Pragma: no-cache');
    }
    
    /**
     * Check if request method is allowed
     */
    public function isMethodAllowed($allowed = ['GET', 'POST'], $method = null) {
        $method = $method ?? $_SERVER['REQUEST_METHOD'];
        return in_array($method, $allowed);
    }
    
    /**
     * Verify request is HTTPS (in production)
     * ZERO_TRUST: Always enforce HTTPS in production
     */
    public function verifyHTTPS() {
        if ($_SERVER['REQUEST_SCHEME'] !== 'https' && PHP_SAPI !== 'cli') {
            throw new Exception("HTTPS required");
        }
    }
}
