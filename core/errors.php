<?php
/**
 * BETELITE ERROR HANDLER
 * ======================
 * FAIL_SOFT: Graceful error handling
 * Never expose sensitive information in error messages
 */

class ErrorHandler {
    
    private $logger;
    private $inProduction;
    
    public function __construct($logger, $inProduction = true) {
        $this->logger = $logger;
        $this->inProduction = $inProduction;
    }
    
    /**
     * Handle exceptions
     * FAIL_SOFT: Log internally, show generic message to user
     */
    public function handleException($exception) {
        $errorCode = $exception->getCode() ?: 500;
        
        // Log exception with full details
        $this->logger->error('Exception occurred: ' . $exception->getMessage(), [
            'file' => $exception->getFile(),
            'line' => $exception->getLine(),
            'code' => $errorCode,
            'trace' => $exception->getTraceAsString(),
        ]);
        
        // Send error response
        http_response_code($errorCode);
        
        if ($this->inProduction) {
            // ZERO_TRUST: Never expose internal details in production
            $response = [
                'error' => true,
                'message' => 'An error occurred. Please try again later.',
                'code' => $errorCode,
            ];
        } else {
            // Development: Show details
            $response = [
                'error' => true,
                'message' => $exception->getMessage(),
                'code' => $errorCode,
                'file' => $exception->getFile(),
                'line' => $exception->getLine(),
            ];
        }
        
        header('Content-Type: application/json');
        echo json_encode($response);
        exit;
    }
    
    /**
     * Handle PHP errors
     * FAIL_SOFT: Convert to exceptions
     */
    public function handleError($errno, $errstr, $errfile, $errline) {
        // ZERO_TRUST: Never suppress errors
        if (!(error_reporting() & $errno)) {
            return false;
        }
        
        $this->logger->error("PHP Error: $errstr", [
            'file' => $errfile,
            'line' => $errline,
            'code' => $errno,
        ]);
        
        // Convert to exception for consistent handling
        throw new ErrorException($errstr, 0, $errno, $errfile, $errline);
    }
    
    /**
     * Handle shutdown - catch fatal errors
     * FAIL_SOFT: Final chance to log
     */
    public function handleShutdown() {
        $error = error_get_last();
        if ($error && in_array($error['type'], [E_ERROR, E_CORE_ERROR, E_COMPILE_ERROR, E_PARSE])) {
            $this->logger->critical('Fatal error occurred', [
                'message' => $error['message'],
                'file' => $error['file'],
                'line' => $error['line'],
                'type' => $error['type'],
            ]);
            
            http_response_code(500);
            
            if (!$this->inProduction) {
                echo "<pre>";
                echo "Fatal Error: " . $error['message'] . "\n";
                echo "File: " . $error['file'] . "\n";
                echo "Line: " . $error['line'] . "\n";
                echo "</pre>";
            }
        }
    }
}

/**
 * Custom exception for API responses
 */
class APIException extends Exception {
    
    private $statusCode;
    private $errorCode;
    
    public function __construct($message, $statusCode = 400, $errorCode = null) {
        parent::__construct($message, $statusCode);
        $this->statusCode = $statusCode;
        $this->errorCode = $errorCode;
    }
    
    public function getStatusCode() {
        return $this->statusCode;
    }
    
    public function getErrorCode() {
        return $this->errorCode;
    }
}
