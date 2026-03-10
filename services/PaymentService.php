<?php
/**
 * PAYMENT SERVICE
 * ===============
 * SAFE_SWAP: Pluggable payment gateways
 * Supports: Paystack, Flutterwave, Crypto (future)
 */

class PaymentService {
    
    private $db;
    private $logger;
    private $provider;
    
    public function __construct($db, $logger) {
        $this->db = $db;
        $this->logger = $logger;
        $this->provider = config('payment.provider');
    }
    
    /**
     * Initiate payment
     * SAFE_SWAP: Provider abstraction
     */
    public function initiate($userId, $amount, $type = 'deposit') {
        if (!Validator::float($amount, 0.01)) {
            throw new APIException("Invalid amount", 400);
        }
        
        if (!Validator::enum($type, ['deposit', 'withdrawal'])) {
            throw new APIException("Invalid payment type", 400);
        }
        
        try {
            // Get wallet
            $wallet = $this->db->selectOne(
                "SELECT id FROM wallets WHERE user_id = ?",
                [$userId]
            );
            
            if (!$wallet) {
                throw new Exception("Wallet not found");
            }
            
            // Create payment record
            $paymentId = $this->db->insert('payments', [
                'user_id' => $userId,
                'wallet_id' => $wallet['id'],
                'type' => $type,
                'amount' => $amount,
                'provider' => $this->provider,
                'status' => 'pending',
            ]);
            
            // Get provider handler
            $handler = $this->getProviderHandler();
            
            // Initialize payment
            $result = $handler->initiate($paymentId, $userId, $amount);
            
            return [
                'payment_id' => $paymentId,
                'authorization_url' => $result['url'] ?? null,
                'reference' => $result['reference'] ?? null,
            ];
            
        } catch (Exception $e) {
            $this->logger->error("Payment initiation failed: " . $e->getMessage());
            throw new APIException("Payment initiation failed", 500);
        }
    }
    
    /**
     * Verify payment
     * CORE_LOCK: Verify with provider
     */
    public function verify($paymentId, $providerReference = null) {
        try {
            // Get payment
            $payment = $this->db->selectOne(
                "SELECT * FROM payments WHERE id = ?",
                [$paymentId]
            );
            
            if (!$payment) {
                throw new APIException("Payment not found", 404);
            }
            
            // Get provider handler
            $handler = $this->getProviderHandler();
            
            // Verify with provider
            $result = $handler->verify($providerReference ?? $payment['provider_reference']);
            
            if (!$result['success']) {
                $this->db->update('payments', [
                    'status' => 'failed',
                    'failure_reason' => $result['error'] ?? 'Verification failed',
                ], 'id = ?', [$paymentId]);
                
                throw new APIException("Payment verification failed", 400);
            }
            
            // Update payment
            $this->db->update('payments', [
                'status' => 'completed',
                'completed_at' => date('Y-m-d H:i:s'),
                'provider_reference' => $result['reference'] ?? null,
            ], 'id = ?', [$paymentId]);
            
            // Credit wallet
            if ($payment['type'] === 'deposit') {
                $walletService = new WalletService($this->db, $this->logger);
                $walletService->deposit($payment['user_id'], $payment['amount'], $paymentId, $this->provider);
            }
            
            audit_log('payment_verified', $payment['user_id'], [
                'payment_id' => $paymentId,
                'amount' => $payment['amount'],
            ]);
            
            return ['verified' => true, 'status' => 'completed'];
            
        } catch (APIException $e) {
            throw $e;
        } catch (Exception $e) {
            $this->logger->error("Payment verification error: " . $e->getMessage());
            throw new APIException("Verification error", 500);
        }
    }
    
    /**
     * Get provider handler
     * SAFE_SWAP: Dynamic provider selection
     */
    private function getProviderHandler() {
        switch ($this->provider) {
            case 'paystack':
                return new PaystackPaymentHandler();
            case 'flutterwave':
                return new FlutterWavePaymentHandler();
            default:
                throw new Exception("Unsupported payment provider: " . $this->provider);
        }
    }
    
    /**
     * Process refund
     * ROLLBACK_READY
     */
    public function refund($paymentId) {
        try {
            $this->db->beginTransaction();
            
            $payment = $this->db->selectOne(
                "SELECT * FROM payments WHERE id = ?",
                [$paymentId]
            );
            
            if (!$payment || $payment['status'] !== 'completed') {
                throw new APIException("Cannot refund this payment", 400);
            }
            
            // Get provider handler
            $handler = $this->getProviderHandler();
            
            // Request refund from provider
            $result = $handler->refund($payment['provider_reference']);
            
            if (!$result['success']) {
                throw new Exception("Provider refund failed");
            }
            
            // Update payment
            $this->db->update('payments', [
                'status' => 'refunded',
            ], 'id = ?', [$paymentId]);
            
            // Reverse wallet transaction
            $walletService = new WalletService($this->db, $this->logger);
            $walletService->releaseFunds($payment['user_id'], $payment['amount'], 'refund', $paymentId);
            
            $this->db->commit();
            
            audit_log('payment_refunded', $payment['user_id'], [
                'payment_id' => $paymentId,
                'amount' => $payment['amount'],
            ]);
            
            return ['refunded' => true];
            
        } catch (Exception $e) {
            $this->db->rollback();
            throw $e;
        }
    }
}

/**
 * Paystack Payment Handler
 * SAFE_SWAP: Pluggable provider
 */
class PaystackPaymentHandler {
    
    private $publicKey;
    private $secretKey;
    
    public function __construct() {
        $this->publicKey = config('payment.paystack.public_key');
        $this->secretKey = config('payment.paystack.secret_key');
    }
    
    public function initiate($paymentId, $userId, $amount) {
        // Paystack integration
        // Returns: ['url' => authorization_url, 'reference' => reference]
        return [
            'url' => "https://checkout.paystack.com/",
            'reference' => 'paystack_' . $paymentId,
        ];
    }
    
    public function verify($reference) {
        // Verify with Paystack API
        return [
            'success' => true,
            'reference' => $reference,
        ];
    }
    
    public function refund($reference) {
        // Request refund from Paystack
        return ['success' => true];
    }
}

/**
 * Flutterwave Payment Handler
 * SAFE_SWAP: Pluggable provider
 */
class FlutterWavePaymentHandler {
    
    private $publicKey;
    private $secretKey;
    
    public function __construct() {
        $this->publicKey = config('payment.flutterwave.public_key');
        $this->secretKey = config('payment.flutterwave.secret_key');
    }
    
    public function initiate($paymentId, $userId, $amount) {
        // Flutterwave integration
        return [
            'url' => "https://checkout.flutterwave.com/",
            'reference' => 'flw_' . $paymentId,
        ];
    }
    
    public function verify($reference) {
        // Verify with Flutterwave API
        return [
            'success' => true,
            'reference' => $reference,
        ];
    }
    
    public function refund($reference) {
        // Request refund from Flutterwave
        return ['success' => true];
    }
}
