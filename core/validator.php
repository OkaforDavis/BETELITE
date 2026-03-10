<?php
/**
 * BETELITE VALIDATOR
 * ==================
 * ZERO_TRUST: All input validation
 * Input validation for common data types
 */

class Validator {
    
    private static $errors = [];
    
    /**
     * Validate email format
     */
    public static function email($email) {
        return filter_var($email, FILTER_VALIDATE_EMAIL) !== false;
    }
    
    /**
     * Validate URL format
     */
    public static function url($url) {
        return filter_var($url, FILTER_VALIDATE_URL) !== false;
    }
    
    /**
     * Validate integer
     */
    public static function integer($value, $min = null, $max = null) {
        $filtered = filter_var($value, FILTER_VALIDATE_INT);
        if ($filtered === false) {
            return false;
        }
        
        if ($min !== null && $filtered < $min) {
            return false;
        }
        
        if ($max !== null && $filtered > $max) {
            return false;
        }
        
        return true;
    }
    
    /**
     * Validate float
     */
    public static function float($value, $min = null, $max = null) {
        $filtered = filter_var($value, FILTER_VALIDATE_FLOAT);
        if ($filtered === false) {
            return false;
        }
        
        if ($min !== null && $filtered < $min) {
            return false;
        }
        
        if ($max !== null && $filtered > $max) {
            return false;
        }
        
        return true;
    }
    
    /**
     * Validate string length
     */
    public static function string($value, $minLength = 1, $maxLength = null) {
        if (!is_string($value)) {
            return false;
        }
        
        $length = strlen($value);
        
        if ($length < $minLength) {
            return false;
        }
        
        if ($maxLength !== null && $length > $maxLength) {
            return false;
        }
        
        return true;
    }
    
    /**
     * Validate password strength
     */
    public static function password($password) {
        // Minimum 8 characters
        if (strlen($password) < 8) {
            return false;
        }
        
        // At least one uppercase letter
        if (!preg_match('/[A-Z]/', $password)) {
            return false;
        }
        
        // At least one lowercase letter
        if (!preg_match('/[a-z]/', $password)) {
            return false;
        }
        
        // At least one digit
        if (!preg_match('/[0-9]/', $password)) {
            return false;
        }
        
        // At least one special character
        if (!preg_match('/[!@#$%^&*()_+\-=\[\]{};:\'",.<>?\/\\|`~]/', $password)) {
            return false;
        }
        
        return true;
    }
    
    /**
     * Validate IP address
     */
    public static function ipAddress($ip) {
        return filter_var($ip, FILTER_VALIDATE_IP) !== false;
    }
    
    /**
     * Validate array has required keys
     */
    public static function requiredKeys($array, $keys) {
        if (!is_array($array)) {
            return false;
        }
        
        foreach ($keys as $key) {
            if (!isset($array[$key]) || $array[$key] === '') {
                return false;
            }
        }
        
        return true;
    }
    
    /**
     * Validate value is in allowed list
     */
    public static function enum($value, $allowed) {
        return in_array($value, $allowed, true);
    }
    
    /**
     * Validate date format (YYYY-MM-DD)
     */
    public static function date($date) {
        $d = \DateTime::createFromFormat('Y-m-d', $date);
        return $d && $d->format('Y-m-d') === $date;
    }
    
    /**
     * Validate datetime format (YYYY-MM-DD HH:MM:SS)
     */
    public static function datetime($datetime) {
        $d = \DateTime::createFromFormat('Y-m-d H:i:s', $datetime);
        return $d && $d->format('Y-m-d H:i:s') === $datetime;
    }
    
    /**
     * Validate phone number (basic international)
     */
    public static function phone($phone) {
        // Remove non-numeric characters except +
        $cleaned = preg_replace('/[^0-9+]/', '', $phone);
        
        // Check length 10-15 digits
        $digitsOnly = preg_replace('/[^0-9]/', '', $cleaned);
        return strlen($digitsOnly) >= 10 && strlen($digitsOnly) <= 15;
    }
    
    /**
     * Get validation errors
     */
    public static function getErrors() {
        return self::$errors;
    }
    
    /**
     * Clear validation errors
     */
    public static function clearErrors() {
        self::$errors = [];
    }
}
