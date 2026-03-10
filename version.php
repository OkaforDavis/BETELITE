<?php
/**
 * BETELITE VERSION MANAGEMENT
 * ============================
 * VERSION_GATE: Controls feature availability by version
 * CORE_LOCK: Do not edit without documentation
 * 
 * Semantic Versioning: MAJOR.MINOR.PATCH
 * Features are activated based on current version
 */

define('BETELITE_VERSION', '1.0.0');
define('BETELITE_BUILD', '20260113');

/**
 * Feature Gates - Controls which features are available
 * VERSION_GATE: All feature access goes through this
 */
class VersionGate {
    
    private static $VERSION = '1.0.0';
    
    // Feature availability matrix
    private static $FEATURES = [
        '1.0.0' => [
            'tournaments' => true,
            'streaming' => true,
            'user_system' => true,
            'wallet' => false,           // VERSION 1.5+
            'betting' => false,          // VERSION 2.0+
            'live_betting' => false,     // VERSION 2.0+
            'mobile_api' => false,       // VERSION 3.0+
            'ai_anticheat' => false,     // VERSION 4.0+
            'kyc' => false,              // Disabled by default
            'mfa' => false,              // Disabled by default
        ],
        '1.5.0' => [
            'tournaments' => true,
            'streaming' => true,
            'user_system' => true,
            'wallet' => true,
            'betting' => false,
            'live_betting' => false,
            'mobile_api' => false,
            'ai_anticheat' => false,
            'kyc' => false,
            'mfa' => false,
        ],
        '2.0.0' => [
            'tournaments' => true,
            'streaming' => true,
            'user_system' => true,
            'wallet' => true,
            'betting' => true,
            'live_betting' => true,
            'mobile_api' => false,
            'ai_anticheat' => false,
            'kyc' => false,
            'mfa' => false,
        ],
    ];
    
    /**
     * Check if feature is enabled
     * ZERO_TRUST: Always verify, never assume
     */
    public static function isEnabled($feature) {
        $features = self::$FEATURES[self::$VERSION] ?? [];
        return $features[$feature] ?? false;
    }
    
    /**
     * Get current version
     */
    public static function getVersion() {
        return self::$VERSION;
    }
    
    /**
     * Require feature or fail
     */
    public static function require($feature) {
        if (!self::isEnabled($feature)) {
            throw new Exception("Feature '$feature' not available in version " . self::$VERSION);
        }
    }
    
    /**
     * Get all enabled features
     */
    public static function getEnabledFeatures() {
        return self::$FEATURES[self::$VERSION] ?? [];
    }
}

// AUDIT_TRACE: Log version info
if (php_sapi_name() !== 'cli') {
    error_log("[VERSION] BETELITE v" . BETELITE_VERSION . " Build:" . BETELITE_BUILD);
}
