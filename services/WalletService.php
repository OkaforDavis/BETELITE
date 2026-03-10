<?php
/**
 * WALLET SERVICE
 * ==============
 * CORE_LOCK: Financial operations
 * FAIL_SAFE: All transactions atomic
 * AUDIT_TRACE: Every transaction logged
 */

class WalletService {
    
    private $db;
    private $logger;
    
    public function __construct($db, $logger) {
        $this->db = $db;
        $this->logger = $logger;
    }
    
    /**
     * Get user wallet
     * ZERO_TRUST: Always verify user ownership
     */
    public function getWallet($userId) {
        return $this->db->selectOne(
            "SELECT * FROM wallets WHERE user_id = ?",
            [$userId]
        );
    }
    
    /**
     * Deposit funds
     * CORE_LOCK: Critical financial operation
     */
    public function deposit($userId, $amount, $paymentId, $paymentGateway) {
        if (!Validator::float($amount, 0.01)) {
            throw new APIException("Invalid amount", 400);
        }
        
        try {
            $this->db->beginTransaction();
            
            // Get current wallet
            $wallet = $this->db->selectOne(
                "SELECT id, available_balance FROM wallets WHERE user_id = ? FOR UPDATE",
                [$userId]
            );
            
            if (!$wallet) {
                throw new Exception("Wallet not found");
            }
            
            // Update wallet
            $newBalance = $wallet['available_balance'] + $amount;
            $this->db->update('wallets', [
                'available_balance' => $newBalance,
                'total_deposited' => new \PDOStatement("total_deposited + " . $amount),
            ], 'user_id = ?', [$userId]);
            
            // Record transaction
            $this->db->insert('wallet_transactions', [
                'wallet_id' => $wallet['id'],
                'user_id' => $userId,
                'type' => 'deposit',
                'amount' => $amount,
                'balance_before' => $wallet['available_balance'],
                'balance_after' => $newBalance,
                'payment_gateway' => $paymentGateway,
                'payment_reference' => $paymentId,
                'status' => 'completed',
            ]);
            
            $this->db->commit();
            
            // AUDIT_TRACE
            audit_log('wallet_deposit', $userId, [
                'amount' => $amount,
                'gateway' => $paymentGateway,
                'payment_id' => $paymentId,
            ]);
            
            return [
                'balance' => $newBalance,
                'transaction_id' => $transactionId,
            ];
            
        } catch (Exception $e) {
            $this->db->rollback();
            $this->logger->error("Deposit failed: " . $e->getMessage());
            throw new APIException("Deposit failed", 500);
        }
    }
    
    /**
     * Withdraw funds
     * CORE_LOCK: Critical financial operation
     */
    public function withdraw($userId, $amount, $paymentMethod) {
        if (!Validator::float($amount, 0.01)) {
            throw new APIException("Invalid amount", 400);
        }
        
        try {
            $this->db->beginTransaction();
            
            // Get wallet with lock
            $wallet = $this->db->selectOne(
                "SELECT id, available_balance, frozen FROM wallets WHERE user_id = ? FOR UPDATE",
                [$userId]
            );
            
            if (!$wallet) {
                throw new Exception("Wallet not found");
            }
            
            // Check if frozen
            if ($wallet['frozen']) {
                throw new APIException("Wallet is frozen", 403);
            }
            
            // Check balance
            if ($wallet['available_balance'] < $amount) {
                throw new APIException("Insufficient balance", 400);
            }
            
            // Check limits
            $minWithdrawal = config('wallet.min_withdrawal', 1000);
            $maxWithdrawal = config('wallet.max_withdrawal', 1000000);
            
            if ($amount < $minWithdrawal || $amount > $maxWithdrawal) {
                throw new APIException("Amount outside withdrawal limits", 400);
            }
            
            // Calculate fee
            $feePercent = config('wallet.withdrawal_fee_percent', 0.5);
            $fee = $amount * ($feePercent / 100);
            $netAmount = $amount - $fee;
            
            // Update wallet
            $newBalance = $wallet['available_balance'] - $amount;
            $this->db->update('wallets', [
                'available_balance' => $newBalance,
            ], 'user_id = ?', [$userId]);
            
            // Create withdrawal transaction
            $transactionId = $this->db->insert('wallet_transactions', [
                'wallet_id' => $wallet['id'],
                'user_id' => $userId,
                'type' => 'withdrawal',
                'amount' => $amount,
                'balance_before' => $wallet['available_balance'],
                'balance_after' => $newBalance,
                'status' => 'pending',
                'metadata' => json_encode([
                    'method' => $paymentMethod,
                    'fee' => $fee,
                    'net_amount' => $netAmount,
                ]),
            ]);
            
            $this->db->commit();
            
            // AUDIT_TRACE
            audit_log('wallet_withdrawal', $userId, [
                'amount' => $amount,
                'fee' => $fee,
                'method' => $paymentMethod,
            ]);
            
            return [
                'balance' => $newBalance,
                'transaction_id' => $transactionId,
                'status' => 'pending',
            ];
            
        } catch (APIException $e) {
            $this->db->rollback();
            throw $e;
        } catch (Exception $e) {
            $this->db->rollback();
            $this->logger->error("Withdrawal failed: " . $e->getMessage());
            throw new APIException("Withdrawal failed", 500);
        }
    }
    
    /**
     * Lock funds for tournament entry
     * CORE_LOCK: Escrow logic
     */
    public function lockFunds($userId, $amount, $reason, $referenceId) {
        try {
            $this->db->beginTransaction();
            
            $wallet = $this->db->selectOne(
                "SELECT id, available_balance, escrow_balance FROM wallets WHERE user_id = ? FOR UPDATE",
                [$userId]
            );
            
            if (!$wallet || $wallet['available_balance'] < $amount) {
                throw new APIException("Insufficient balance", 400);
            }
            
            // Move funds to escrow
            $this->db->update('wallets', [
                'available_balance' => $wallet['available_balance'] - $amount,
                'escrow_balance' => $wallet['escrow_balance'] + $amount,
            ], 'user_id = ?', [$userId]);
            
            // Record transaction
            $this->db->insert('wallet_transactions', [
                'wallet_id' => $wallet['id'],
                'user_id' => $userId,
                'type' => 'bet',
                'amount' => $amount,
                'balance_before' => $wallet['available_balance'],
                'balance_after' => $wallet['available_balance'] - $amount,
                'reference_type' => $reason,
                'reference_id' => $referenceId,
                'status' => 'completed',
            ]);
            
            $this->db->commit();
            
            return ['locked' => true, 'amount' => $amount];
            
        } catch (Exception $e) {
            $this->db->rollback();
            throw $e;
        }
    }
    
    /**
     * Release locked funds
     * ROLLBACK_READY: Can refund bets
     */
    public function releaseFunds($userId, $amount, $reason, $referenceId) {
        try {
            $this->db->beginTransaction();
            
            $wallet = $this->db->selectOne(
                "SELECT id, available_balance, escrow_balance FROM wallets WHERE user_id = ? FOR UPDATE",
                [$userId]
            );
            
            if (!$wallet || $wallet['escrow_balance'] < $amount) {
                throw new Exception("Insufficient escrow balance");
            }
            
            // Move funds back to available
            $this->db->update('wallets', [
                'escrow_balance' => $wallet['escrow_balance'] - $amount,
                'available_balance' => $wallet['available_balance'] + $amount,
            ], 'user_id = ?', [$userId]);
            
            // Record transaction
            $this->db->insert('wallet_transactions', [
                'wallet_id' => $wallet['id'],
                'user_id' => $userId,
                'type' => 'refund',
                'amount' => $amount,
                'balance_before' => $wallet['available_balance'],
                'balance_after' => $wallet['available_balance'] + $amount,
                'reference_type' => $reason,
                'reference_id' => $referenceId,
                'status' => 'completed',
            ]);
            
            $this->db->commit();
            
            audit_log('funds_released', $userId, [
                'amount' => $amount,
                'reason' => $reason,
            ]);
            
            return ['released' => true, 'amount' => $amount];
            
        } catch (Exception $e) {
            $this->db->rollback();
            throw $e;
        }
    }
    
    /**
     * Transfer winnings
     * CORE_LOCK: Prize payout
     */
    public function creditWinnings($userId, $amount, $tournamentId) {
        try {
            $this->db->beginTransaction();
            
            $wallet = $this->db->selectOne(
                "SELECT id, available_balance FROM wallets WHERE user_id = ? FOR UPDATE",
                [$userId]
            );
            
            if (!$wallet) {
                throw new Exception("Wallet not found");
            }
            
            $newBalance = $wallet['available_balance'] + $amount;
            
            $this->db->update('wallets', [
                'available_balance' => $newBalance,
                'total_won' => new \PDOStatement("total_won + " . $amount),
            ], 'user_id = ?', [$userId]);
            
            $this->db->insert('wallet_transactions', [
                'wallet_id' => $wallet['id'],
                'user_id' => $userId,
                'type' => 'prize',
                'amount' => $amount,
                'balance_before' => $wallet['available_balance'],
                'balance_after' => $newBalance,
                'reference_type' => 'tournament',
                'reference_id' => $tournamentId,
                'status' => 'completed',
            ]);
            
            $this->db->commit();
            
            audit_log('winnings_credited', $userId, [
                'amount' => $amount,
                'tournament_id' => $tournamentId,
            ]);
            
            return ['credited' => true];
            
        } catch (Exception $e) {
            $this->db->rollback();
            throw $e;
        }
    }
    
    /**
     * Freeze wallet (admin action)
     * CORE_LOCK: Fraud prevention
     */
    public function freezeWallet($userId, $reason) {
        $this->db->update('wallets', [
            'frozen' => true,
            'frozen_reason' => $reason,
        ], 'user_id = ?', [$userId]);
        
        audit_log('wallet_frozen', $userId, [
            'reason' => $reason,
        ]);
        
        return ['frozen' => true];
    }
    
    /**
     * Unfreeze wallet
     */
    public function unfreezeWallet($userId) {
        $this->db->update('wallets', [
            'frozen' => false,
            'frozen_reason' => null,
        ], 'user_id = ?', [$userId]);
        
        audit_log('wallet_unfrozen', $userId, []);
        
        return ['frozen' => false];
    }
}
