<?php
/**
 * BETELITE TEST FRAMEWORK
 * =======================
 * tests/
 */

// Base test class
abstract class BaseTest {
    
    protected $db;
    protected $security;
    protected $logger;
    protected $testUserId;
    protected $testToken;
    
    public function setUp() {
        // Connect to test database
        $config = [
            'host' => 'localhost',
            'port' => 3306,
            'name' => 'betelite_test',
            'user' => 'betelite_test',
            'password' => 'test_password',
            'charset' => 'utf8mb4',
            'strict_mode' => true,
        ];
        
        $this->db = new Database($config);
        
        // Setup logger
        $logConfig = [
            'file' => __DIR__ . '/test.log',
            'audit_file' => __DIR__ . '/test-audit.log',
            'error_file' => __DIR__ . '/test-error.log',
            'level' => 'debug',
            'max_size' => 10485760,
            'retention_days' => 7,
        ];
        
        $this->logger = new Logger($logConfig);
        
        // Setup security
        $securityConfig = [
            'csrf_token_length' => 32,
            'jwt_secret' => 'test_secret_key',
            'jwt_algorithm' => 'HS256',
            'jwt_expiry' => 86400,
            'password_algo' => PASSWORD_ARGON2ID,
            'password_options' => [
                'memory_cost' => 19456,
                'time_cost' => 4,
                'threads' => 1,
            ],
        ];
        
        $this->security = new Security($securityConfig, $this->logger);
        
        // Clear test data
        $this->tearDown();
        
        // Create test user
        $this->createTestUser();
    }
    
    public function tearDown() {
        // Clear test data
        $this->db->execute("TRUNCATE wallet_transactions");
        $this->db->execute("TRUNCATE wallets");
        $this->db->execute("TRUNCATE user_sessions");
        $this->db->execute("TRUNCATE password_resets");
        $this->db->execute("TRUNCATE tournament_participants");
        $this->db->execute("TRUNCATE matches");
        $this->db->execute("TRUNCATE tournaments");
        $this->db->execute("TRUNCATE teams");
        $this->db->execute("TRUNCATE users");
        $this->db->execute("TRUNCATE audit_logs");
    }
    
    protected function createTestUser() {
        $this->testUserId = $this->db->insert('users', [
            'username' => 'testuser',
            'email' => 'test@example.com',
            'password_hash' => $this->security->hashPassword('TestPass123!'),
            'status' => 'active',
            'verified' => true,
        ]);
        
        // Create wallet
        $this->db->insert('wallets', [
            'user_id' => $this->testUserId,
            'available_balance' => 100000,
            'escrow_balance' => 0,
        ]);
        
        // Generate token
        $this->testToken = $this->security->generateJWT([
            'user_id' => $this->testUserId,
            'email' => 'test@example.com',
        ]);
    }
    
    protected function assert($condition, $message) {
        if (!$condition) {
            throw new Exception("Assertion failed: $message");
        }
        echo "✓ $message\n";
    }
    
    protected function assertEquals($expected, $actual, $message) {
        if ($expected !== $actual) {
            throw new Exception("Assertion failed: $message (expected: $expected, got: $actual)");
        }
        echo "✓ $message\n";
    }
    
    protected function assertGreaterThan($threshold, $value, $message) {
        if ($value <= $threshold) {
            throw new Exception("Assertion failed: $message ($value <= $threshold)");
        }
        echo "✓ $message\n";
    }
    
    protected function assertThrows($callback, $exceptionType, $message) {
        try {
            $callback();
            throw new Exception("Assertion failed: $message (no exception thrown)");
        } catch (Exception $e) {
            if (get_class($e) !== $exceptionType && !is_subclass_of($e, $exceptionType)) {
                throw new Exception("Assertion failed: $message (wrong exception: " . get_class($e) . ")");
            }
            echo "✓ $message\n";
        }
    }
}

// ============================================================
// UNIT TESTS
// ============================================================

class AuthServiceTest extends BaseTest {
    
    public function testUserRegistration() {
        $authService = new AuthService($this->db, $this->security, $this->logger);
        
        $result = $authService->register(
            'newuser@example.com',
            'newuser',
            'NewPass123!',
            'NewPass123!'
        );
        
        $this->assert(isset($result['user_id']), "User registered successfully");
        $this->assert($result['verification_required'] === true, "Verification required");
    }
    
    public function testWeakPassword() {
        $authService = new AuthService($this->db, $this->security, $this->logger);
        
        $this->assertThrows(
            function() use ($authService) {
                $authService->register(
                    'user@example.com',
                    'username',
                    'weak',
                    'weak'
                );
            },
            'APIException',
            'Weak password rejected'
        );
    }
    
    public function testLogin() {
        $authService = new AuthService($this->db, $this->security, $this->logger);
        
        $result = $authService->login('test@example.com', 'TestPass123!');
        
        $this->assert(isset($result['token']), "Token generated");
        $this->assert($result['user_id'] === $this->testUserId, "Correct user returned");
    }
    
    public function testInvalidPassword() {
        $authService = new AuthService($this->db, $this->security, $this->logger);
        
        $this->assertThrows(
            function() use ($authService) {
                $authService->login('test@example.com', 'WrongPassword');
            },
            'APIException',
            'Invalid password rejected'
        );
    }
}

// ============================================================
// SECURITY TESTS
// ============================================================

class SecurityTest extends BaseTest {
    
    public function testSQLInjectionPrevention() {
        $malicious = "' OR '1'='1";
        
        // Should not throw, but return empty
        $result = $this->db->selectOne(
            "SELECT id FROM users WHERE email = ?",
            [$malicious]
        );
        
        $this->assert($result === null, "SQL injection prevented");
    }
    
    public function testXSSPrevention() {
        $xssPayload = "<script>alert('xss')</script>";
        
        $sanitized = $this->security->sanitizeString($xssPayload);
        
        $this->assert(strpos($sanitized, '<script>') === false, "XSS payload sanitized");
        $this->assert(strpos($sanitized, '&lt;script&gt;') !== false, "HTML encoded");
    }
    
    public function testCSRFTokenValidation() {
        $_SESSION['csrf_token'] = null;
        $token = $this->security->generateCSRFToken();
        
        $this->assert($this->security->validateCSRFToken($token), "Valid token accepted");
        $this->assert(!$this->security->validateCSRFToken('invalid_token'), "Invalid token rejected");
    }
    
    public function testPasswordHashing() {
        $password = "SecurePass123!";
        
        $hash1 = $this->security->hashPassword($password);
        $hash2 = $this->security->hashPassword($password);
        
        // Hashes should be different (salt)
        $this->assert($hash1 !== $hash2, "Salting works");
        
        // Both should verify
        $this->assert($this->security->verifyPassword($password, $hash1), "Hash 1 verifies");
        $this->assert($this->security->verifyPassword($password, $hash2), "Hash 2 verifies");
    }
}

// ============================================================
// WALLET TESTS
// ============================================================

class WalletServiceTest extends BaseTest {
    
    public function testDeposit() {
        $walletService = new WalletService($this->db, $this->logger);
        
        $result = $walletService->deposit($this->testUserId, 10000, 'payment_123', 'paystack');
        
        $this->assert($result['balance'] > 0, "Deposit successful");
        
        // Verify transaction logged
        $transaction = $this->db->selectOne(
            "SELECT * FROM wallet_transactions WHERE type = 'deposit' AND user_id = ?",
            [$this->testUserId]
        );
        
        $this->assert($transaction !== null, "Transaction logged");
    }
    
    public function testWithdrawalLimits() {
        $walletService = new WalletService($this->db, $this->logger);
        
        // Amount below minimum
        $this->assertThrows(
            function() use ($walletService) {
                $walletService->withdraw($this->testUserId, 100, 'bank_transfer');
            },
            'APIException',
            'Amount below minimum rejected'
        );
    }
    
    public function testInsufficientBalance() {
        $walletService = new WalletService($this->db, $this->logger);
        
        $this->assertThrows(
            function() use ($walletService) {
                // Try to withdraw more than available
                $walletService->withdraw($this->testUserId, 1000000, 'bank_transfer');
            },
            'APIException',
            'Insufficient balance rejected'
        );
    }
}

// ============================================================
// FINANCIAL OPERATION TESTS
// ============================================================

class FinancialTest extends BaseTest {
    
    public function testAtomicTransaction() {
        try {
            $this->db->beginTransaction();
            
            // Debit wallet
            $wallet = $this->db->selectOne(
                "SELECT * FROM wallets WHERE user_id = ? FOR UPDATE",
                [$this->testUserId]
            );
            
            $newBalance = $wallet['available_balance'] - 5000;
            $this->db->update('wallets', [
                'available_balance' => $newBalance,
            ], 'user_id = ?', [$this->testUserId]);
            
            // Simulate error mid-transaction
            throw new Exception("Simulated error");
            
            // This won't execute
            $this->db->commit();
            
        } catch (Exception $e) {
            $this->db->rollback();
        }
        
        // Balance should be unchanged
        $wallet = $this->db->selectOne(
            "SELECT available_balance FROM wallets WHERE user_id = ?",
            [$this->testUserId]
        );
        
        $this->assertEquals(100000, $wallet['available_balance'], "Rollback successful");
    }
}

// ============================================================
// RUN ALL TESTS
// ============================================================

function runAllTests() {
    $tests = [
        new AuthServiceTest(),
        new SecurityTest(),
        new WalletServiceTest(),
        new FinancialTest(),
    ];
    
    $passed = 0;
    $failed = 0;
    
    foreach ($tests as $test) {
        $testName = get_class($test);
        echo "\n=== Running $testName ===\n";
        
        // Get test methods
        $reflection = new ReflectionClass($test);
        $methods = $reflection->getMethods(ReflectionMethod::IS_PUBLIC);
        
        foreach ($methods as $method) {
            if (strpos($method->getName(), 'test') === 0) {
                try {
                    $test->setUp();
                    $test->{$method->getName()}();
                    $passed++;
                } catch (Exception $e) {
                    echo "✗ {$method->getName()}: {$e->getMessage()}\n";
                    $failed++;
                } finally {
                    $test->tearDown();
                }
            }
        }
    }
    
    echo "\n=== TEST SUMMARY ===\n";
    echo "Passed: $passed\n";
    echo "Failed: $failed\n";
    echo "Total: " . ($passed + $failed) . "\n";
    
    return $failed === 0;
}

// Run tests if executed directly
if (basename($_SERVER['PHP_SELF']) === basename(__FILE__)) {
    require_once __DIR__ . '/../core/bootstrap.php';
    
    $success = runAllTests();
    exit($success ? 0 : 1);
}
