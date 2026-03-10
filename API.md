# BETELITE API DOCUMENTATION

## BASE URL

```
https://api.betelite.example.com/api
```

All responses are JSON. Always include `Authorization: Bearer {TOKEN}` header for authenticated endpoints.

---

## AUTHENTICATION ENDPOINTS

### Register User

**POST** `/api/auth/register`

Create new user account.

**Request Body:**
```json
{
  "email": "user@example.com",
  "username": "playerone",
  "password": "SecureP@ss123",
  "confirm_password": "SecureP@ss123"
}
```

**Response (201):**
```json
{
  "user_id": 12345,
  "email": "user@example.com",
  "verification_required": true
}
```

**Error Responses:**
- `400` - Invalid input (email format, password strength)
- `409` - Email or username already exists

---

### Login

**POST** `/api/auth/login`

Authenticate user and receive JWT token.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecureP@ss123"
}
```

**Response (200):**
```json
{
  "user_id": 12345,
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "session_token": "abc123def456",
  "expires_in": 86400
}
```

**Headers in Response:**
```
Set-Cookie: betelite_session=abc123; HttpOnly; Secure; SameSite=Strict
```

---

### Logout

**POST** `/api/auth/logout`

Invalidate current session.

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Response (200):**
```json
{
  "logged_out": true
}
```

---

### Verify Email

**POST** `/api/auth/verify-email`

Verify email address with token sent to email.

**Request Body:**
```json
{
  "user_id": 12345,
  "token": "verification_token_from_email"
}
```

**Response (200):**
```json
{
  "verified": true
}
```

---

### Request Password Reset

**POST** `/api/auth/forgot-password`

Request password reset link.

**Request Body:**
```json
{
  "email": "user@example.com"
}
```

**Response (200):**
```json
{
  "reset_sent": true
}
```

**Note**: Returns success regardless of email existence (security best practice).

---

### Reset Password

**POST** `/api/auth/reset-password`

Reset password using reset token.

**Request Body:**
```json
{
  "token": "reset_token_from_email",
  "password": "NewSecureP@ss456",
  "confirm_password": "NewSecureP@ss456"
}
```

**Response (200):**
```json
{
  "reset": true
}
```

---

## WALLET ENDPOINTS

### Get Wallet

**GET** `/api/wallet`

Get current user's wallet balance and details.

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Response (200):**
```json
{
  "id": 5678,
  "user_id": 12345,
  "available_balance": 50000.00,
  "escrow_balance": 10000.00,
  "total_balance": 60000.00,
  "frozen": false,
  "last_transaction_at": "2026-01-13T10:30:00Z"
}
```

---

### Get Wallet Transactions

**GET** `/api/wallet/transactions?limit=50&offset=0`

Get transaction history.

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Query Parameters:**
- `limit` (int, 1-100): Number of transactions
- `offset` (int): Pagination offset
- `type` (string): Filter by type (deposit, withdrawal, bet, prize)
- `status` (string): Filter by status (pending, completed, failed)
- `start_date` (string, YYYY-MM-DD): From date
- `end_date` (string, YYYY-MM-DD): To date

**Response (200):**
```json
{
  "transactions": [
    {
      "id": 999,
      "type": "deposit",
      "amount": 10000.00,
      "balance_after": 50000.00,
      "status": "completed",
      "created_at": "2026-01-13T10:30:00Z"
    }
  ],
  "total": 150,
  "limit": 50,
  "offset": 0
}
```

---

### Initiate Deposit

**POST** `/api/wallet/deposit`

Start deposit process (redirects to payment gateway).

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Request Body:**
```json
{
  "amount": 10000.00
}
```

**Response (200):**
```json
{
  "payment_id": 111,
  "authorization_url": "https://checkout.paystack.com/...",
  "reference": "paystack_111"
}
```

---

### Initiate Withdrawal

**POST** `/api/wallet/withdraw`

Request withdrawal to bank account or mobile money.

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Request Body:**
```json
{
  "amount": 5000.00,
  "method": "bank_transfer",
  "bank_code": "058",
  "account_number": "1234567890"
}
```

**Response (200):**
```json
{
  "balance": 45000.00,
  "transaction_id": 1000,
  "status": "pending"
}
```

**Errors:**
- `400` - Amount outside limits
- `403` - Wallet is frozen

---

## TOURNAMENT ENDPOINTS

### Create Tournament

**POST** `/api/tournaments`

Create new tournament (organizers only).

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Request Body:**
```json
{
  "game_id": 1,
  "name": "FIFA 24 Championship 2026",
  "description": "Professional FIFA tournament",
  "type": "solo",
  "format": "single_elimination",
  "max_participants": 64,
  "entry_fee": 5000.00,
  "prize_pool": 500000.00,
  "registration_starts": "2026-01-20T00:00:00Z",
  "registration_ends": "2026-02-10T00:00:00Z",
  "starts_at": "2026-02-15T14:00:00Z"
}
```

**Response (201):**
```json
{
  "tournament_id": 777,
  "slug": "fifa-24-championship-2026"
}
```

---

### Get Tournament

**GET** `/api/tournaments/:tournament_id`

Get tournament details.

**Response (200):**
```json
{
  "id": 777,
  "name": "FIFA 24 Championship 2026",
  "game_id": 1,
  "type": "solo",
  "status": "open",
  "max_participants": 64,
  "current_participants": 32,
  "entry_fee": 5000.00,
  "prize_pool": 500000.00,
  "registration_starts": "2026-01-20T00:00:00Z",
  "registration_ends": "2026-02-10T00:00:00Z",
  "starts_at": "2026-02-15T14:00:00Z"
}
```

---

### List Tournaments

**GET** `/api/tournaments?game_id=1&status=open&limit=20`

List tournaments with filters.

**Query Parameters:**
- `game_id` (int): Filter by game
- `status` (string): draft, open, live, completed
- `organizer_id` (int): Filter by organizer
- `limit` (int): Results per page
- `offset` (int): Pagination offset

**Response (200):**
```json
{
  "tournaments": [
    {
      "id": 777,
      "name": "FIFA 24 Championship 2026",
      "game_id": 1,
      "status": "open",
      "current_participants": 32,
      "max_participants": 64
    }
  ],
  "total": 150,
  "limit": 20
}
```

---

### Register for Tournament

**POST** `/api/tournaments/:tournament_id/register`

Register user for tournament.

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Response (200):**
```json
{
  "registered": true,
  "participant_id": 999
}
```

**Errors:**
- `400` - Tournament not open
- `409` - Already registered
- `403` - Insufficient balance (if entry fee)

---

### Withdraw from Tournament

**POST** `/api/tournaments/:tournament_id/withdraw`

Withdraw from tournament (before start).

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Response (200):**
```json
{
  "withdrawn": true,
  "refund_amount": 5000.00
}
```

---

## MATCH ENDPOINTS

### Get Match

**GET** `/api/tournaments/:tournament_id/matches/:match_id`

Get match details including stream.

**Response (200):**
```json
{
  "id": 555,
  "tournament_id": 777,
  "round": 1,
  "player1_id": 111,
  "player2_id": 222,
  "status": "live",
  "score_p1": 2,
  "score_p2": 1,
  "scheduled_at": "2026-02-15T14:00:00Z",
  "started_at": "2026-02-15T14:05:00Z",
  "stream_url": "https://stream.example.com/match555",
  "stream_provider": "cloudflare"
}
```

---

### Update Match Result

**POST** `/api/tournaments/:tournament_id/matches/:match_id/result`

Update match result (referee/admin only).

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Request Body:**
```json
{
  "winner_id": 111,
  "score_p1": 3,
  "score_p2": 1
}
```

**Response (200):**
```json
{
  "updated": true,
  "winner_id": 111
}
```

---

## BETTING ENDPOINTS (VERSION 2.0+)

### Get Match Odds

**GET** `/api/tournaments/:tournament_id/matches/:match_id/odds`

Get current odds for match betting.

**Response (200):**
```json
{
  "match_id": 555,
  "odds": [
    {
      "participant_id": 111,
      "name": "Player One",
      "odds": 1.85,
      "total_staked": 100000.00
    },
    {
      "participant_id": 222,
      "name": "Player Two",
      "odds": 2.10,
      "total_staked": 80000.00
    }
  ]
}
```

---

### Place Bet

**POST** `/api/tournaments/:tournament_id/matches/:match_id/bet`

Place bet on match.

**Headers:**
```
Authorization: Bearer {TOKEN}
```

**Request Body:**
```json
{
  "prediction": 111,
  "stake_amount": 1000.00,
  "bet_type": "winner"
}
```

**Response (200):**
```json
{
  "bet_id": 666,
  "stake_amount": 1000.00,
  "odds": 1.85,
  "potential_payout": 1850.00,
  "status": "pending"
}
```

---

## STREAMING ENDPOINTS

### Get Stream URL

**GET** `/api/tournaments/:tournament_id/matches/:match_id/stream`

Get live stream URL for match.

**Response (200):**
```json
{
  "stream_id": "stream_123",
  "url": "https://stream.example.com/live/stream_123",
  "status": "live",
  "viewer_count": 5420
}
```

---

## GAMES ENDPOINT

### List Games

**GET** `/api/games`

Get list of supported games.

**Response (200):**
```json
{
  "games": [
    {
      "id": 1,
      "name": "FIFA 24",
      "slug": "fifa24",
      "category": "sports",
      "enabled": true,
      "max_players": 2
    },
    {
      "id": 2,
      "name": "Valorant",
      "slug": "valorant",
      "category": "fps",
      "enabled": true,
      "max_players": 5
    }
  ]
}
```

---

## ERROR RESPONSES

All errors follow this format:

```json
{
  "error": true,
  "message": "Description of error",
  "code": "ERROR_CODE"
}
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request (invalid input) |
| 401 | Unauthorized (missing/invalid token) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Not Found |
| 409 | Conflict (duplicate entry) |
| 429 | Rate Limited |
| 500 | Server Error |

### Common Error Codes

- `INVALID_INPUT` - Input validation failed
- `UNAUTHORIZED` - Token missing or invalid
- `FORBIDDEN` - Permission denied
- `NOT_FOUND` - Resource not found
- `DUPLICATE_ENTRY` - Already exists
- `INSUFFICIENT_BALANCE` - Not enough funds
- `RATE_LIMITED` - Too many requests
- `SERVER_ERROR` - Internal error

---

## RATE LIMITING

All endpoints are rate limited:

- **Authentication**: 5 requests per minute per IP
- **API**: 100 requests per minute per user
- **File Upload**: 10 requests per hour per user

**Response Header:**
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705079400
```

---

## PAGINATION

List endpoints support pagination:

**Query Parameters:**
- `limit` (1-100): Items per page (default: 20)
- `offset` (0+): Starting position (default: 0)

**Response:**
```json
{
  "items": [...],
  "total": 500,
  "limit": 20,
  "offset": 0
}
```

---

## DATES & TIMES

All timestamps are ISO 8601 format in UTC:

```
2026-01-13T10:30:45Z
```

---

**API Version**: 1.0.0
**Last Updated**: 2026-01-13
**Status**: Production Ready
