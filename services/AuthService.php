<?php
/**
 * AUTHENTICATION SERVICE
 * ======================
 * CORE_LOCK: Authentication logic
 * ZERO_TRUST: All credentials validated
 * AUDIT_TRACE: All auth events logged
 */

class AuthService {
    
    private $db;
    private $security;
    private $logger;
    
    public function __construct($db, $security, $logger) {
        $this->db = $db;
        $this->security = $security;
        $this->logger = $logger;
    }
    
    /**
     * Register new user
     * ZERO_TRUST: Validate all inputs
     */
    public function register($email, $username, $password, $confirmPassword) {
        // Validate inputs
        if (!Validator::email($email)) {
            throw new APIException("Invalid email format", 400);
        }
        
        if (!Validator::string($username, 3, 50)) {
            throw new APIException("Username must be 3-50 characters", 400);
        }
        
        if ($password !== $confirmPassword) {
            throw new APIException("Passwords do not match", 400);
        }
        
        if (!Validator::password($password)) {
            throw new APIException("Password does not meet requirements", 400);
        }
        
        // Check if user already exists
        $existing = $this->db->selectOne(
            "SELECT id FROM users WHERE email = ? OR username = ?",
            [$email, $username]
        );
        
        if ($existing) {
            throw new APIException("Email or username already exists", 409);
        }
        
        // Create user
        try {
            $this->db->beginTransaction();
            
            $passwordHash = $this->security->hashPassword($password);
            $verificationToken = $this->security->generateToken();
            
            $userId = $this->db->insert('users', [
                'email' => $email,
                'username' => $username,
                'password_hash' => $passwordHash,
                'verification_token' => $verificationToken,
                'status' => 'pending_verification',
            ]);
            
            // Create wallet
            $this->db->insert('wallets', [
                'user_id' => $userId,
                'available_balance' => 0,
                'escrow_balance' => 0,
            ]);
            
            $this->db->commit();
            
            // Log registration
            $this->logger->info("User registered", [
                'user_id' => $userId,
                'email' => $email,
                'username' => $username,
            ]);
            
            // AUDIT_TRACE
            audit_log('user_registered', $userId, [
                'email' => $email,
                'username' => $username,
            ]);
            
            return [
                'user_id' => $userId,
                'email' => $email,
                'verification_required' => true,
            ];
            
        } catch (Exception $e) {
            $this->db->rollback();
            $this->logger->error("Registration failed: " . $e->getMessage());
            throw new APIException("Registration failed", 500);
        }
    }
    
    /**
     * Login user
     * ZERO_TRUST: Validate credentials carefully
     */
    public function login($email, $password) {
        // Validate inputs
        if (!Validator::email($email) || !Validator::string($password, 1)) {
            throw new APIException("Invalid credentials", 401);
        }
        
        // Get user
        $user = $this->db->selectOne(
            "SELECT id, password_hash, status, verified FROM users WHERE email = ?",
            [$email]
        );
        
        if (!$user) {
            // AUDIT_TRACE: Log failed login attempt
            $this->logger->warning("Login failed - user not found", [
                'email' => $email,
                'ip' => $_SERVER['REMOTE_ADDR'] ?? 'unknown',
            ]);
            throw new APIException("Invalid credentials", 401);
        }
        
        // Check if account is active
        if ($user['status'] !== 'active') {
            throw new APIException("Account is not active", 403);
        }
        
        // Verify password
        if (!$this->security->verifyPassword($password, $user['password_hash'])) {
            // AUDIT_TRACE: Log failed login
            $this->logger->warning("Login failed - wrong password", [
                'user_id' => $user['id'],
                'ip' => $_SERVER['REMOTE_ADDR'] ?? 'unknown',
            ]);
            throw new APIException("Invalid credentials", 401);
        }
        
        // Create JWT token
        $token = $this->security->generateJWT([
            'user_id' => $user['id'],
            'email' => $email,
        ]);
        
        // Create session
        $sessionToken = $this->security->generateToken();
        $this->db->insert('user_sessions', [
            'user_id' => $user['id'],
            'session_token' => $sessionToken,
            'ip_address' => $this->security->getClientIP(),
            'user_agent' => $_SERVER['HTTP_USER_AGENT'] ?? '',
            'expires_at' => date('Y-m-d H:i:s', time() + 86400),
        ]);
        
        // Update last login
        $this->db->update('users', [
            'last_login_at' => date('Y-m-d H:i:s'),
            'last_login_ip' => $this->security->getClientIP(),
        ], 'id = ?', [$user['id']]);
        
        // AUDIT_TRACE
        audit_log('user_login', $user['id'], [
            'email' => $email,
            'ip' => $this->security->getClientIP(),
        ]);
        
        return [
            'user_id' => $user['id'],
            'token' => $token,
            'session_token' => $sessionToken,
            'expires_in' => 86400,
        ];
    }
    
    /**
     * Verify email
     */
    public function verifyEmail($userId, $token) {
        $user = $this->db->selectOne(
            "SELECT id, verification_token FROM users WHERE id = ?",
            [$userId]
        );
        
        if (!$user || $user['verification_token'] !== $token) {
            throw new APIException("Invalid verification token", 400);
        }
        
        $this->db->update('users', [
            'verified' => true,
            'verified_at' => date('Y-m-d H:i:s'),
            'status' => 'active',
            'verification_token' => null,
        ], 'id = ?', [$userId]);
        
        audit_log('email_verified', $userId, []);
        
        return ['verified' => true];
    }
    
    /**
     * Get current user from token
     * ZERO_TRUST: Always verify token
     */
    public function getCurrentUser($token) {
        $payload = $this->security->verifyJWT($token);
        if (!$payload) {
            return null;
        }
        
        $user = $this->db->selectOne(
            "SELECT id, email, username, role, status FROM users WHERE id = ?",
            [$payload['user_id']]
        );
        
        return $user;
    }
    
    /**
     * Logout user
     */
    public function logout($userId, $sessionToken) {
        $this->db->delete('user_sessions', 'user_id = ? AND session_token = ?', [$userId, $sessionToken]);
        audit_log('user_logout', $userId, []);
        return ['logged_out' => true];
    }
    
    /**
     * Request password reset
     */
    public function requestPasswordReset($email) {
        $user = $this->db->selectOne(
            "SELECT id FROM users WHERE email = ?",
            [$email]
        );
        
        if (!$user) {
            // Don't reveal if email exists (security best practice)
            return ['reset_sent' => true];
        }
        
        $token = $this->security->generateToken();
        
        $this->db->insert('password_resets', [
            'user_id' => $user['id'],
            'token' => $token,
            'expires_at' => date('Y-m-d H:i:s', time() + 3600), // 1 hour
        ]);
        
        // TODO: Send email with reset link
        
        audit_log('password_reset_requested', $user['id'], [
            'email' => $email,
        ]);
        
        return ['reset_sent' => true];
    }
    
    /**
     * Reset password
     */
    public function resetPassword($token, $newPassword) {
        if (!Validator::password($newPassword)) {
            throw new APIException("Password does not meet requirements", 400);
        }
        
        $reset = $this->db->selectOne(
            "SELECT user_id FROM password_resets WHERE token = ? AND used = FALSE AND expires_at > NOW()",
            [$token]
        );
        
        if (!$reset) {
            throw new APIException("Invalid or expired reset token", 400);
        }
        
        try {
            $this->db->beginTransaction();
            
            $passwordHash = $this->security->hashPassword($newPassword);
            
            $this->db->update('users', [
                'password_hash' => $passwordHash,
            ], 'id = ?', [$reset['user_id']]);
            
            $this->db->update('password_resets', [
                'used' => true,
                'used_at' => date('Y-m-d H:i:s'),
            ], 'token = ?', [$token]);
            
            $this->db->commit();
            
            audit_log('password_reset', $reset['user_id'], []);
            
            return ['reset' => true];
            
        } catch (Exception $e) {
            $this->db->rollback();
            throw new APIException("Password reset failed", 500);
        }
    }
}
