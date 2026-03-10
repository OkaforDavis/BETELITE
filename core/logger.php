<?php
/**
 * BETELITE LOGGER
 * ===============
 * AUDIT_TRACE: Comprehensive logging for all events
 * ZERO_TRUST: Log everything, trust nothing
 * 
 * Log Levels:
 * - debug: Detailed debug information
 * - info: General informational messages
 * - warning: Warning messages
 * - error: Error messages
 * - critical: Critical/fatal errors
 * - audit: Financial/security audit trail
 */

class Logger {
    
    private $config;
    private $logLevels = [
        'debug' => 1,
        'info' => 2,
        'warning' => 3,
        'error' => 4,
        'critical' => 5,
        'audit' => 99,  // Always log
    ];
    
    public function __construct($config) {
        $this->config = $config;
        
        // Ensure log directories exist
        foreach (['file', 'audit_file', 'error_file'] as $logType) {
            $dir = dirname($this->config[$logType]);
            if (!is_dir($dir)) {
                mkdir($dir, 0750, true);
            }
        }
        
        // Set PHP error logging
        ini_set('log_errors', 1);
        ini_set('error_log', $this->config['error_file']);
    }
    
    /**
     * Format log message with timestamp and context
     */
    private function formatMessage($level, $message, $context = []) {
        $timestamp = date('Y-m-d H:i:s');
        $contextStr = !empty($context) ? ' ' . json_encode($context, JSON_UNESCAPED_SLASHES) : '';
        return "[$timestamp] [$level] $message$contextStr";
    }
    
    /**
     * Write to log file
     * ZERO_TRUST: Ensure file operations are atomic
     */
    private function writeLog($file, $message) {
        try {
            $handle = fopen($file, 'a');
            if ($handle) {
                flock($handle, LOCK_EX);
                fwrite($handle, $message . PHP_EOL);
                flock($handle, LOCK_UN);
                fclose($handle);
            }
        } catch (Exception $e) {
            error_log("Failed to write log file: $file");
        }
    }
    
    /**
     * Check if log level should be recorded
     */
    private function shouldLog($level) {
        $currentLevel = $this->logLevels[$this->config['level']] ?? 2;
        $logLevel = $this->logLevels[$level] ?? 0;
        return $logLevel >= $currentLevel;
    }
    
    /**
     * Log debug message
     */
    public function debug($message, $context = []) {
        if ($this->shouldLog('debug')) {
            $formatted = $this->formatMessage('DEBUG', $message, $context);
            $this->writeLog($this->config['file'], $formatted);
        }
    }
    
    /**
     * Log info message
     */
    public function info($message, $context = []) {
        if ($this->shouldLog('info')) {
            $formatted = $this->formatMessage('INFO', $message, $context);
            $this->writeLog($this->config['file'], $formatted);
        }
    }
    
    /**
     * Log warning message
     */
    public function warning($message, $context = []) {
        if ($this->shouldLog('warning')) {
            $formatted = $this->formatMessage('WARNING', $message, $context);
            $this->writeLog($this->config['file'], $formatted);
        }
    }
    
    /**
     * Log error message
     */
    public function error($message, $context = []) {
        if ($this->shouldLog('error')) {
            $formatted = $this->formatMessage('ERROR', $message, $context);
            $this->writeLog($this->config['file'], $formatted);
            $this->writeLog($this->config['error_file'], $formatted);
        }
    }
    
    /**
     * Log critical error
     */
    public function critical($message, $context = []) {
        $formatted = $this->formatMessage('CRITICAL', $message, $context);
        $this->writeLog($this->config['file'], $formatted);
        $this->writeLog($this->config['error_file'], $formatted);
        error_log($formatted);
    }
    
    /**
     * AUDIT_TRACE: Log security/financial events
     * CORE_LOCK: Critical audit trail
     */
    public function audit($action, $context = []) {
        $context['ip'] = $context['ip'] ?? $_SERVER['REMOTE_ADDR'] ?? 'unknown';
        $context['user_agent'] = $_SERVER['HTTP_USER_AGENT'] ?? 'unknown';
        $formatted = $this->formatMessage('AUDIT', $action, $context);
        $this->writeLog($this->config['audit_file'], $formatted);
    }
    
    /**
     * Rotate logs when they exceed max size
     * FAIL_SOFT: Graceful handling of large logs
     */
    public function rotateIfNeeded() {
        foreach (['file', 'audit_file', 'error_file'] as $logType) {
            $logFile = $this->config[$logType];
            
            if (file_exists($logFile) && filesize($logFile) > $this->config['max_size']) {
                $backup = $logFile . '.' . date('Y-m-d-H-i-s');
                rename($logFile, $backup);
                
                // Delete old backups beyond retention
                $this->cleanOldBackups($logFile);
            }
        }
    }
    
    /**
     * Delete old log backups
     */
    private function cleanOldBackups($pattern) {
        $dir = dirname($pattern);
        $base = basename($pattern);
        $files = glob("$dir/$base.*");
        
        if (is_array($files)) {
            $cutoff = strtotime("-" . $this->config['retention_days'] . " days");
            foreach ($files as $file) {
                if (filemtime($file) < $cutoff) {
                    @unlink($file);
                }
            }
        }
    }
}
