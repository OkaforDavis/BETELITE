# SECURITY DOCUMENTATION

## SECURITY ARCHITECTURE

BETELITE implements **defense-in-depth** security with multiple layers:

### Layer 1: Network Security
- HTTPS/TLS 1.3 enforced
- HSTS headers
- DDoS protection (via CDN)
- Firewall rules
- Rate limiting

### Layer 2: Application Security
- Input validation (ZERO_TRUST)
- SQL injection prevention (prepared statements)
- XSS prevention (HTML encoding)
- CSRF protection (tokens)
- Session hardening

### Layer 3: Data Security
- Password hashing (Argon2id)
- Encryption at rest (TBD)
- Encryption in transit (TLS)
- Database backups
- Audit logging

### Layer 4: Access Control
- Role-based access control
- JWT authentication
- Session management
- IP whitelisting (admin panel)
- API key validation

---

## VULNERABILITY PREVENTION

### SQL Injection

**Vulnerable:**
```php
$result = mysqli_query("SELECT * FROM users WHERE email = '$email'");
```

**Secure:**
```php
$stmt = $pdo->prepare("SELECT * FROM users WHERE email = ?");
$result = $stmt->execute([$email]);
```

**BETELITE Approach:**
- All database queries use prepared statements
- User input NEVER concatenated into SQL
- Database layer enforces this in code review

### Cross-Site Scripting (XSS)

**Vulnerable:**
```php
echo "<h1>" . $_GET['title'] . "</h1>";
```

**Secure:**
```php
echo "<h1>" . htmlspecialchars($_GET['title'], ENT_QUOTES | ENT_HTML5, 'UTF-8') . "</h1>";
```

**BETELITE Approach:**
- All user input escaped before output
- Content Security Policy (CSP) headers
- No inline scripts allowed
- Template auto-escaping

### Cross-Site Request Forgery (CSRF)

**Vulnerable:**
```html
<form action="/transfer" method="POST">
    <input type="text" name="amount">
</form>
```

**Secure:**
```html
<form action="/transfer" method="POST">
    <input type="hidden" name="csrf_token" value="<?php echo $Security->generateCSRFToken(); ?>">
    <input type="text" name="amount">
</form>
```

**BETELITE Approach:**
- CSRF token generated on page load
- Token validated on form submission
- Token bound to session
- SameSite cookie attribute set

### Password Security

**Policy:**
- Minimum 8 characters
- 1 uppercase letter
- 1 lowercase letter
- 1 digit
- 1 special character

**Hashing:**
- Algorithm: Argon2id (PHP's PASSWORD_ARGON2ID)
- Memory: 19.4MB
- Time cost: 4
- Verification: password_verify() with timing attack protection

### Session Security

**Configuration:**
```php
session_set_cookie_params([
    'lifetime' => 3600,           // 1 hour
    'path' => '/',
    'domain' => '.betelite.example.com',
    'secure' => true,             // HTTPS only
    'httponly' => true,           // No JavaScript access
    'samesite' => 'Strict',       // Prevent CSRF
]);
```

**Regeneration:**
```php
// After login
session_regenerate_id(true);

// After privilege change
session_regenerate_id(true);
```

---

## AUTHENTICATION & AUTHORIZATION

### Authentication Flow

```
1. User submits email + password
2. System looks up user by email
3. System verifies password with password_verify()
4. If valid:
   a. Session created
   b. JWT token generated
   c. Session token created
   d. Last login time updated
5. Token sent to client
6. Client includes token in Authorization header
```

### Authorization

**Role-Based Access Control:**
```php
public function requireRole($role) {
    if ($this->user['role'] !== $role) {
        throw new APIException("Forbidden", 403);
    }
}
```

**Permission Model:**
- `player`: Can register for tournaments, place bets
- `organizer`: Can create tournaments, manage matches
- `moderator`: Can resolve disputes, ban players
- `admin`: Full system access

### Token Security

**JWT Token:**
```
Header: {"alg":"HS256","typ":"JWT"}
Payload: {"user_id":123,"email":"user@example.com","iat":1234567890,"exp":1234654290}
Signature: HMAC-SHA256(header.payload, JWT_SECRET)
```

**Security:**
- Signed with secret key
- Expiration time enforced (24 hours)
- Signature verified on every request
- Claims validated before use

---

## FINANCIAL SECURITY (CORE_LOCK)

### Transaction Atomicity

```php
$DB->beginTransaction();
try {
    // Debit wallet
    $DB->update('wallets', ['balance' => $newBalance], ...);
    
    // Create audit log
    $DB->insert('wallet_transactions', ...);
    
    // Commit all or nothing
    $DB->commit();
} catch (Exception $e) {
    $DB->rollback();
    throw $e;
}
```

### Escrow Logic

**Tournament Entry:**
1. User requests registration with entry fee
2. Funds locked in escrow (not available)
3. Tournament confirmed, funds remain locked
4. On tournament end:
   - If lost: Funds released to organizer
   - If won: Winnings credited, fees transferred

### Double-Spending Prevention

```php
// Select with row lock
$wallet = $DB->selectOne(
    "SELECT id, balance FROM wallets WHERE user_id = ? FOR UPDATE",
    [$userId]
);

// Check balance
if ($wallet['balance'] < $amount) {
    throw new Exception("Insufficient balance");
}

// Debit and commit atomically
```

### Audit Trail

**Every transaction records:**
- User ID
- Transaction type
- Amount
- Balance before/after
- Timestamp
- IP address
- User agent

---

## RATE LIMITING & THROTTLING

### Global Rate Limits

```
Authentication: 5 attempts/minute/IP
API: 100 requests/minute/user
File Upload: 10 uploads/hour/user
```

### Implementation

```php
class RateLimiter {
    public function checkLimit($identifier, $limit, $window) {
        $key = "rate_limit:" . $identifier;
        $count = $redis->get($key) ?? 0;
        
        if ($count >= $limit) {
            throw new APIException("Rate limited", 429);
        }
        
        $redis->incr($key);
        $redis->expire($key, $window);
    }
}
```

### Bypass Protection

- Admin panel: IP whitelist
- API: User-based limits
- Web: Device fingerprint + rate limit

---

## SECURITY HEADERS

### HTTP Security Headers

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'
```

### Content Security Policy

```
default-src 'self'              # Only from same origin
script-src 'self'               # Only internal scripts
style-src 'self' 'unsafe-inline' # CSS needed for styling
img-src 'self' data: https:      # Images from self and HTTPS
font-src 'self'                 # Fonts from self
```

---

## LOGGING & MONITORING (AUDIT_TRACE)

### What Gets Logged

**Authentication Events:**
- Login attempts (success/failure)
- Password changes
- Email verification
- Account lock/unlock

**Financial Events:**
- Deposits
- Withdrawals
- Bets placed/settled
- Prize distributions
- Manual adjustments

**Security Events:**
- Failed login attempts
- Suspicious activity detected
- Rate limit hits
- Account locks
- Permission changes

**System Events:**
- Application startup
- Errors/exceptions
- Database operations (financial only)
- Payment provider calls

### Log Retention

```
Active logs: /var/www/betelite/logs/
├── betelite.log      (all events)
├── audit.log         (financial/security - IMMUTABLE)
└── errors.log        (errors only)

Rotation: Daily, keep 90 days
```

### Log Analysis

```bash
# Failed login attempts
grep "login failed" logs/audit.log

# Suspicious IP
grep "192.168.1.100" logs/audit.log

# Payment failures
grep "payment.*failed" logs/audit.log

# Large withdrawals
grep "withdrawal.*amount.*[0-9][0-9][0-9][0-9][0-9][0-9]" logs/audit.log
```

---

## INCIDENT RESPONSE

### Detection

1. **Automated Alerts:**
   - Failed login attempts (>5 in 10 min)
   - Rate limit hits (suspicious)
   - Payment failures (multiple)
   - Database errors (recurring)

2. **Manual Review:**
   - Check audit logs daily
   - Review security events
   - Monitor error rates

### Response Process

```
1. Detect incident
   └─ Alert admin immediately

2. Contain
   └─ Freeze affected accounts
   └─ Review transaction logs
   └─ Check for unauthorized access

3. Investigate
   └─ Analyze audit logs
   └─ Review payment records
   └─ Check system logs

4. Remediate
   └─ Reset passwords if needed
   └─ Reverse fraudulent transactions
   └─ Update security rules

5. Recover
   └─ Restore from backup if needed
   └─ Verify data integrity
   └─ Resume normal operations

6. Learn
   └─ Document incident
   └─ Update security policies
   └─ Implement preventive measures
```

---

## COMPLIANCE

### Data Protection

**User Data:**
- Only collect what's necessary
- Secure storage (hashed passwords)
- Regular backups
- 30-day deletion retention after account closure

**Financial Data:**
- PCI-DSS compliance (via payment gateway)
- Never store card numbers
- Encrypted transmission
- Regular audits

### Regulations

- **GDPR**: Right to access, deletion, data portability
- **CCPA**: Transparency, opt-out, deletion
- **AML/KYC**: Disabled by default, available if needed

---

## SECURITY TESTING

### Regular Audits

- **Weekly**: Log analysis, error monitoring
- **Monthly**: Security review, patch updates
- **Quarterly**: Penetration testing, code review
- **Annually**: Full security audit, compliance check

### Test Coverage

- SQL injection tests
- XSS payload testing
- CSRF attack simulation
- Brute force protection
- Session hijacking attempts
- Payment fraud scenarios

---

## INCIDENT HISTORY

None yet. System is new.

---

**Last Updated**: 2026-01-13
**Next Review**: 2026-02-13
**Security Level**: PRODUCTION
