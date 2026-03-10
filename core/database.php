<?php
/**
 * BETELITE DATABASE CONNECTION
 * =============================
 * CORE_LOCK: Database interface
 * PDO-based, prepared statements only
 * ZERO_TRUST: No raw SQL ever
 */

class Database {
    
    private $pdo;
    private $config;
    private $logger;
    private $transactionActive = false;
    
    public function __construct($config, $logger = null) {
        $this->config = $config;
        $this->logger = $logger;
        
        try {
            $dsn = "mysql:host=" . $config['host'] . ":" . $config['port'] . 
                   ";dbname=" . $config['name'] . 
                   ";charset=" . $config['charset'];
            
            $this->pdo = new PDO($dsn, $config['user'], $config['password'], [
                PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
                PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
                PDO::ATTR_TIMEOUT => $config['connection_timeout'],
            ]);
            
            // Enable strict mode
            if ($config['strict_mode']) {
                $this->pdo->query("SET SESSION sql_mode='STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'");
            }
            
            if ($this->logger) {
                $this->logger->debug("Database connection established", [
                    'host' => $config['host'],
                    'database' => $config['name'],
                ]);
            }
        } catch (PDOException $e) {
            if ($this->logger) {
                $this->logger->critical("Database connection failed: " . $e->getMessage());
            }
            throw $e;
        }
    }
    
    /**
     * Execute SELECT query
     * ZERO_TRUST: Always use prepared statements
     */
    public function select($query, $params = []) {
        try {
            $stmt = $this->pdo->prepare($query);
            $stmt->execute($params);
            return $stmt->fetchAll();
        } catch (PDOException $e) {
            $this->logQueryError($query, $params, $e);
            throw new Exception("Database query failed");
        }
    }
    
    /**
     * Execute SELECT query, return first row
     */
    public function selectOne($query, $params = []) {
        try {
            $stmt = $this->pdo->prepare($query);
            $stmt->execute($params);
            return $stmt->fetch();
        } catch (PDOException $e) {
            $this->logQueryError($query, $params, $e);
            throw new Exception("Database query failed");
        }
    }
    
    /**
     * Execute INSERT query
     */
    public function insert($table, $data) {
        try {
            $columns = array_keys($data);
            $placeholders = array_fill(0, count($columns), '?');
            
            $query = "INSERT INTO " . $table . " (" . implode(', ', $columns) . ") VALUES (" . implode(', ', $placeholders) . ")";
            $stmt = $this->pdo->prepare($query);
            $stmt->execute(array_values($data));
            
            return $this->pdo->lastInsertId();
        } catch (PDOException $e) {
            $this->logQueryError($query, array_values($data), $e);
            throw new Exception("Insert failed");
        }
    }
    
    /**
     * Execute UPDATE query
     */
    public function update($table, $data, $where, $whereParams = []) {
        try {
            $sets = [];
            $params = [];
            
            foreach ($data as $column => $value) {
                $sets[] = "$column = ?";
                $params[] = $value;
            }
            
            // Merge where params
            $params = array_merge($params, $whereParams);
            
            $query = "UPDATE " . $table . " SET " . implode(', ', $sets) . " WHERE " . $where;
            $stmt = $this->pdo->prepare($query);
            $stmt->execute($params);
            
            return $stmt->rowCount();
        } catch (PDOException $e) {
            $this->logQueryError($query, $params, $e);
            throw new Exception("Update failed");
        }
    }
    
    /**
     * Execute DELETE query
     */
    public function delete($table, $where, $params = []) {
        try {
            $query = "DELETE FROM " . $table . " WHERE " . $where;
            $stmt = $this->pdo->prepare($query);
            $stmt->execute($params);
            
            return $stmt->rowCount();
        } catch (PDOException $e) {
            $this->logQueryError($query, $params, $e);
            throw new Exception("Delete failed");
        }
    }
    
    /**
     * Execute raw prepared query
     * CORE_LOCK: Use only when necessary
     */
    public function execute($query, $params = []) {
        try {
            $stmt = $this->pdo->prepare($query);
            $stmt->execute($params);
            return $stmt;
        } catch (PDOException $e) {
            $this->logQueryError($query, $params, $e);
            throw new Exception("Query execution failed");
        }
    }
    
    /**
     * Start transaction
     * CORE_LOCK: Critical for financial operations
     */
    public function beginTransaction() {
        if (!$this->transactionActive) {
            $this->pdo->beginTransaction();
            $this->transactionActive = true;
            if ($this->logger) {
                $this->logger->debug("Transaction started");
            }
        }
    }
    
    /**
     * Commit transaction
     * ROLLBACK_READY: Always ready to roll back
     */
    public function commit() {
        if ($this->transactionActive) {
            $this->pdo->commit();
            $this->transactionActive = false;
            if ($this->logger) {
                $this->logger->debug("Transaction committed");
            }
        }
    }
    
    /**
     * Rollback transaction
     * FAIL_SOFT: Graceful rollback
     */
    public function rollback() {
        if ($this->transactionActive) {
            $this->pdo->rollBack();
            $this->transactionActive = false;
            if ($this->logger) {
                $this->logger->warning("Transaction rolled back");
            }
        }
    }
    
    /**
     * Check if transaction is active
     */
    public function inTransaction() {
        return $this->transactionActive;
    }
    
    /**
     * Log query errors - but never expose SQL
     * AUDIT_TRACE: Log for debugging
     */
    private function logQueryError($query, $params, $exception) {
        if ($this->logger) {
            $this->logger->error("Database error", [
                'error' => $exception->getMessage(),
                'code' => $exception->getCode(),
            ]);
        }
    }
    
    /**
     * Get raw PDO connection (use sparingly)
     */
    public function getPDO() {
        return $this->pdo;
    }
    
    /**
     * Close connection
     */
    public function close() {
        $this->pdo = null;
    }
    
    /**
     * Destructor - ensure transaction is closed
     */
    public function __destruct() {
        if ($this->transactionActive) {
            $this->rollback();
        }
    }
}
