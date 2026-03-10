# BETELITE ARCHITECTURE DOCUMENTATION

## EXECUTIVE SUMMARY

BETELITE is a **production-grade esports platform** with:
- **Version-gated features** (1.0 → 4.0)
- **Zero-trust security** throughout
- **Pluggable integrations** (SAFE_SWAP)
- **Immutable audit trails** (AUDIT_TRACE)
- **Atomic financial operations** (CORE_LOCK)
- **Graceful degradation** (FAIL_SOFT)
- **100% rollback-ready** (ROLLBACK_READY)

---

## SYSTEM ARCHITECTURE

### LAYERS

```
┌─────────────────────────────────────────┐
│      CLIENT LAYER (HTML/CSS/JS)         │
├─────────────────────────────────────────┤
│      API LAYER (REST/JSON)              │
├─────────────────────────────────────────┤
│      SERVICE LAYER (Business Logic)     │
├─────────────────────────────────────────┤
│      DATABASE LAYER (MySQL 8+)          │
├─────────────────────────────────────────┤
│      SECURITY/LOGGING (ZERO_TRUST)      │
└─────────────────────────────────────────┘
```

### CORE PRINCIPLES

| Principle | Implementation | Benefit |
|-----------|----------------|---------|
| **ZERO_TRUST** | All input validated, no assumptions | Security |
| **CORE_LOCK** | Critical logic immutable | Reliability |
| **SAFE_SWAP** | Pluggable integrations | Flexibility |
| **FAIL_SOFT** | Graceful error handling | Resilience |
| **AUDIT_TRACE** | Immutable event logs | Compliance |
| **ROLLBACK_READY** | Transactional operations | Data integrity |
| **VERSION_GATE** | Feature versioning | Scalability |

---

## TECHNOLOGY STACK

| Layer | Technology | Version |
|-------|-----------|---------|
| Backend | PHP | 8.2+ |
| Database | MySQL | 8.0+ |
| Frontend | HTML5/CSS3/Vanilla JS | ES6+ |
| Payments | Paystack/Flutterwave | Latest |
| Streaming | Cloudflare/Mux | Latest |
| Web Server | Apache/Nginx | Any |

---

## PROJECT STRUCTURE

```
betelite/
├── core/                    # CORE_LOCK: Bootstrap and foundation
│   ├── bootstrap.php        # Application initialization
│   ├── security.php         # ZERO_TRUST: Security layer
│   ├── logger.php           # AUDIT_TRACE: Logging
│   ├── validator.php        # Input validation
│   ├── database.php         # PDO wrapper (prepared statements only)
│   ├── router.php           # REST router
│   └── errors.php           # FAIL_SOFT: Error handling
│
├── api/                     # REST API endpoints
│   ├── router.php           # API routing
│   └── [endpoints]          # Organized by feature
│
├── services/                # Business logic
│   ├── AuthService.php      # User authentication
│   ├── WalletService.php    # CORE_LOCK: Wallet management
│   ├── TournamentService.php
│   ├── BettingService.php
│   ├── PaymentService.php   # SAFE_SWAP: Payment processing
│   └── StreamingService.php # SAFE_SWAP: Streaming
│
├── admin/                   # Admin panel
│   ├── index.php
│   └── [admin features]
│
├── user/                    # User dashboard
│   ├── dashboard.php
│   └── [user features]
│
├── auth/                    # Authentication pages
│   ├── login.php
│   ├── register.php
│   └── reset.php
│
├── assets/                  # Static files
│   ├── css/
│   ├── js/
│   └── images/
│
├── database/                # Database files
│   ├── schema.sql           # CORE_LOCK: Database structure
│   ├── migrations.sql
│   └── seeds.sql
│
├── tests/                   # Testing suite
│   ├── unit/
│   ├── integration/
│   ├── security/
│   └── penetration/
│
├── logs/                    # Application logs
│   ├── betelite.log
│   ├── audit.log           # AUDIT_TRACE
│   └── errors.log
│
├── uploads/                 # User uploads (secure)
│
├── config.php               # Configuration loader
├── version.php              # VERSION_GATE: Feature versioning
├── .env                     # Environment variables
├── .env.example
├── .htaccess                # Apache rules
└── index.php                # Main entry point
```

---

## FEATURE ROADMAP (VERSION_GATE)

### Version 1.0 (Current)
- ✅ User authentication (login/register)
- ✅ Tournament creation & management
- ✅ Match scheduling
- ✅ Streaming integration
- ✅ Basic reporting

### Version 1.5 (Q2 2026)
- 🔄 Wallet system
- 🔄 Deposit/withdrawal
- 🔄 Escrow logic
- 🔄 Prize payouts

### Version 2.0 (Q3 2026)
- ⏳ Betting engine
- ⏳ Odds calculation
- ⏳ Bet settlement
- ⏳ Live betting

### Version 3.0 (Q4 2026)
- ⏳ Mobile API
- ⏳ Push notifications
- ⏳ Offline support

### Version 4.0 (Q1 2027)
- ⏳ AI Anti-cheat
- ⏳ Advanced analytics
- ⏳ Sponsorship system

---

## SECURITY ARCHITECTURE

### AUTHENTICATION & AUTHORIZATION

```
User Login
    ↓
JWT Token Generated
    ↓
Token Verified on Each Request
    ↓
User ID Extracted from Token
    ↓
Database Record Verified
    ↓
Session Created with IP/User-Agent
    ↓
Rate Limiting Applied
    ↓
Request Processed
```

### INPUT VALIDATION (ZERO_TRUST)

1. **Type Validation** - Is it the right type?
2. **Length Validation** - Is it the right length?
3. **Format Validation** - Does it match the pattern?
4. **Boundary Validation** - Is it within limits?
5. **Business Logic Validation** - Does it make sense?

### SQL INJECTION PREVENTION

```php
// ❌ NEVER: Raw SQL
$result = mysqli_query("SELECT * FROM users WHERE email = '$email'");

// ✅ ALWAYS: Prepared Statements
$stmt = $pdo->prepare("SELECT * FROM users WHERE email = ?");
$stmt->execute([$email]);
```

### XSS PREVENTION

```php
// ❌ NEVER: Output unescaped
echo $_GET['name'];

// ✅ ALWAYS: HTML encode
echo htmlspecialchars($_GET['name'], ENT_QUOTES | ENT_HTML5, 'UTF-8');
```

### CSRF PROTECTION

```php
// Token generated on page load
$token = $Security->generateCSRFToken();

// Token validated on form submission
$Security->validateCSRFToken($_POST['csrf_token']);
```

---

## DATABASE DESIGN

### KEY PRINCIPLES

1. **Strict Mode Enabled** - No loose type conversions
2. **Prepared Statements Only** - No SQL injection possible
3. **Transactions for Financial Ops** - All-or-nothing
4. **Audit Trail** - Immutable event log
5. **Referential Integrity** - Foreign keys enforced

### CRITICAL TABLES (CORE_LOCK)

| Table | Purpose | Transaction |
|-------|---------|-------------|
| `users` | User accounts | No |
| `wallets` | User funds | Yes |
| `wallet_transactions` | Movement log | Yes |
| `tournaments` | Tournament data | No |
| `matches` | Match results | Yes |
| `bets` | Betting data | Yes |
| `payments` | Payment records | Yes |
| `audit_logs` | Immutable audit trail | Yes |

---

## FINANCIAL OPERATIONS (CORE_LOCK)

### DEPOSIT FLOW

```
1. Payment initiated
2. User redirected to provider
3. Payment processed by provider
4. Webhook received
5. Payment verified with provider
6. Wallet credited atomically
7. Transaction recorded
8. Audit logged
```

### WITHDRAWAL FLOW

```
1. User requests withdrawal
2. Wallet balance checked (locked)
3. Limits verified
4. Fee calculated
5. Funds deducted atomically
6. Withdrawal initiated
7. Provider processes
8. Wallet updated on confirmation
9. Audit logged
```

### TOURNAMENT ENTRY FLOW

```
1. User registers for tournament
2. Entry fee calculated
3. Funds locked (escrow)
4. Tournament confirms entry
5. Match time scheduled
6. On match completion:
   - Locked funds released if lost
   - Winnings credited if won
7. All atomic, all audited
```

---

## PAYMENT INTEGRATION (SAFE_SWAP)

### PLUGGABLE ARCHITECTURE

```php
// All payment providers implement same interface
interface PaymentHandler {
    public function initiate($paymentId, $userId, $amount);
    public function verify($reference);
    public function refund($reference);
}

// Easy to swap providers
'payment' => [
    'provider' => 'paystack', // or 'flutterwave'
    'paystack' => [...],
    'flutterwave' => [...],
]
```

### SUPPORTED PROVIDERS

- **Paystack** - Africa's leading payment gateway
- **Flutterwave** - Pan-African payments
- **Crypto** - Future implementation

---

## STREAMING INTEGRATION (SAFE_SWAP)

### PROVIDERS

- **Cloudflare Stream** - Global CDN, low latency
- **Mux** - Developer-friendly streaming
- **LiveKit** - Self-hostable WebRTC

### IMPLEMENTATION

```php
// Provider abstraction
'streaming' => [
    'provider' => 'cloudflare',
    'cloudflare' => [
        'account_id' => '...',
        'auth_token' => '...',
    ],
]

// Easy provider swap
'streaming' => [
    'provider' => 'mux',
    'mux' => [
        'access_token' => '...',
    ],
]
```

---

## TESTING STRATEGY

### TEST COVERAGE

| Type | Location | Purpose |
|------|----------|---------|
| **Unit Tests** | `tests/unit/` | Function logic |
| **Integration Tests** | `tests/integration/` | Component interaction |
| **Security Tests** | `tests/security/` | Auth, injection, etc. |
| **Penetration Tests** | `tests/penetration/` | Real attack scenarios |

### CRITICAL TEST CASES

1. **Authentication**
   - Valid login
   - Invalid credentials
   - Brute force protection
   - Session hijacking attempts

2. **Financial Operations**
   - Insufficient balance
   - Duplicate transactions
   - Concurrent operations
   - Rollback on failure

3. **Payment Processing**
   - Provider integration
   - Webhook handling
   - Refund processing
   - Fraud detection

4. **Security**
   - SQL injection attempts
   - XSS payloads
   - CSRF attacks
   - Rate limiting bypass

---

## DEPLOYMENT GUIDE

### REQUIREMENTS

- **PHP 8.2+** with PDO MySQL extension
- **MySQL 8.0+** with InnoDB
- **Apache/Nginx** with URL rewriting
- **HTTPS certificate** (required in production)
- **4GB RAM minimum** for production
- **SSD storage** recommended

### INSTALLATION STEPS

```bash
# 1. Clone/Download code
git clone https://github.com/betelite/betelite.git
cd betelite

# 2. Create environment
cp .env.example .env
# Edit .env with your values

# 3. Set permissions
chmod 750 logs uploads
chmod 640 config.php version.php

# 4. Create database
mysql < database/schema.sql

# 5. Seed initial data
mysql < database/seeds.sql

# 6. Configure web server
# Apache: Enable mod_rewrite, .htaccess
# Nginx: See deployment/nginx.conf

# 7. Test installation
curl https://yourdomain.com/api/health
```

### PRODUCTION CHECKLIST

- [ ] Environment set to `production`
- [ ] Debug mode disabled
- [ ] HTTPS enabled
- [ ] Database backed up
- [ ] Logs rotated daily
- [ ] Rate limiting configured
- [ ] Payment gateway tested
- [ ] Email configured
- [ ] Admin account secured
- [ ] .env file protected
- [ ] Regular security audits scheduled

---

## MONITORING & MAINTENANCE

### LOG FILES

```
logs/
├── betelite.log      # All application events
├── audit.log         # Financial/security events (AUDIT_TRACE)
└── errors.log        # Error stack traces
```

### MONITORING METRICS

- **Authentication**: Login attempts, failures
- **Transactions**: Deposits, withdrawals, bets
- **Errors**: Application errors, database errors
- **Security**: Rate limit hits, suspicious activity
- **Performance**: Query times, response times

### BACKUP STRATEGY

```bash
# Daily database backup
mysqldump -u user -p betelite_db > backups/betelite_$(date +%Y%m%d).sql

# Weekly full backup
tar -czf backups/betelite_$(date +%Y%m%d).tar.gz .

# Monthly offsite backup
aws s3 sync backups/ s3://betelite-backups/
```

---

## TROUBLESHOOTING

### Common Issues

1. **"Database connection failed"**
   - Check .env DB credentials
   - Verify MySQL is running
   - Check user permissions

2. **"CSRF token mismatch"**
   - Clear browser cookies
   - Check session timeout
   - Verify time sync

3. **"Payment verification failed"**
   - Check API keys in .env
   - Verify provider API is accessible
   - Check payment reference

4. **"File upload failed"**
   - Check `uploads/` permissions (chmod 750)
   - Verify file type allowed
   - Check file size limit

---

## SUPPORT & CONTACT

- **Documentation**: See `/docs`
- **Issues**: GitHub Issues
- **Security**: security@betelite.com
- **Support**: support@betelite.com

---

**Version**: 1.0.0
**Last Updated**: 2026-01-13
**Status**: Production Ready
