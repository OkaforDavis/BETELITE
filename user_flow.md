# BETELITE User Flow Diagram

## Complete User Journey

```mermaid
flowchart TD
    Start([User Visits BETELITE]) --> Login{Existing User?}
    
    Login -->|No| SignUp["📝 Sign Up<br>Email/Password via Firebase"]
    SignUp --> KYC["🪪 KYC Verification<br>Basic Identity Check"]
    KYC --> WalletSetup["💳 Setup Payment Method<br>Link Paystack Account"]
    WalletSetup --> Dashboard
    
    Login -->|Yes| SignIn["🔑 Sign In<br>Firebase Authentication"]
    SignIn --> Dashboard["📊 Dashboard<br>View Balance & Upcoming Matches"]
    
    Dashboard --> Browse{What Next?}
    
    Browse -->|View Matches| MatchBrowse["🎮 Browse Matches<br>Filter by Sport/League/Status"]
    MatchBrowse --> MatchDetail["📋 Match Details<br>Teams, Odds, Bet Types"]
    MatchDetail --> Decision1{Interested?}
    
    Decision1 -->|Not Now| Browse
    Decision1 -->|Place Bet| CheckBalance{Sufficient<br>Balance?}
    
    CheckBalance -->|No| Deposit["💰 Deposit Funds<br>Paystack Payment Gateway"]
    Deposit --> DepositConfirm["✅ Funds Added to Wallet"]
    DepositConfirm --> PlaceBet
    
    CheckBalance -->|Yes| PlaceBet["🎯 Place Bet<br>Select Odds & Amount<br>Confirm Stake"]
    PlaceBet --> BetConfirm["✅ Bet Confirmed<br>Get Bet Slip & Odds"]
    BetConfirm --> WatchOption{Watch Match?}
    
    WatchOption -->|Yes| WatchMatch["🎥 Live Stream Match<br>LiveKit Video Feed<br>Real-Time Commentary"]
    WatchOption -->|No| WaitResult["⏳ Wait for Result"]
    
    WatchMatch --> MatchEnds["🏁 Match Ends<br>Score Detection Triggered"]
    MatchEnds --> Detection["🤖 AI Detection<br>Gemini Vision API<br>Extracts Final Score"]
    Detection --> Verification{Manual Review<br>Needed?}
    
    Verification -->|Yes| AdminReview["👨‍⚖️ Admin Reviews<br>Resolves Disputes"]
    AdminReview --> Result
    Verification -->|No| Result["📊 Result Confirmed<br>System Calculates Outcome"]
    
    WaitResult --> Result
    
    Result --> BetResult{Bet Won?}
    
    BetResult -->|Won| Win["🎉 You Won!<br>Winnings Added to Wallet<br>Show Payout Amount"]
    BetResult -->|Lost| Loss["❌ Bet Lost<br>Stake Forfeited"]
    BetResult -->|Push/Void| Push["🔄 Bet Voided<br>Stake Returned"]
    
    Win --> PostMatch{What Next?}
    Loss --> PostMatch
    Push --> PostMatch
    
    PostMatch -->|Place Another Bet| Browse
    PostMatch -->|Refer Friend| Referral["👥 Referral Program<br>Share Unique Link<br>Earn Commissions"]
    Referral --> Browse
    PostMatch -->|Withdraw| Withdraw["💸 Withdraw Winnings<br>Paystack Payout"]
    Withdraw --> WithdrawConfirm["✅ Withdrawal Processed<br>Funds Sent to Account"]
    WithdrawConfirm --> Browse
    PostMatch -->|View History| History["📈 Bet History<br>Past Bets & Results"]
    History --> Browse
    PostMatch -->|Settings| Settings["⚙️ Account Settings<br>Profile, Preferences<br>Responsible Gaming"]
    Settings --> Browse
    PostMatch -->|Logout| End([Session End])
    
    style Start fill:#28a745,stroke:#1e7e34,color:#fff,stroke-width:3px
    style End fill:#dc3545,stroke:#ab2c2c,color:#fff,stroke-width:3px
    style PlaceBet fill:#ffc107,stroke:#e0a800,color:#000,stroke-width:2px
    style WatchMatch fill:#0dcaf0,stroke:#0ba0d8,color:#000,stroke-width:2px
    style Detection fill:#9966ff,stroke:#7744dd,color:#fff,stroke-width:2px
    style Result fill:#17a2b8,stroke:#0f6674,color:#fff,stroke-width:2px
    style Win fill:#28a745,stroke:#1e7e34,color:#fff,stroke-width:2px
    style Loss fill:#dc3545,stroke:#ab2c2c,color:#fff,stroke-width:2px
```

## User Flow Summary

### Phase 1: Onboarding
- User registration via Firebase
- KYC verification
- Payment method setup (Paystack)

### Phase 2: Match Discovery
- Browse available tournaments and matches
- View match details (teams, odds, bet types)
- Compare different bet options

### Phase 3: Betting
- Check wallet balance
- Deposit funds if needed (Paystack)
- Place bet with selected odds
- Get bet confirmation and slip

### Phase 4: Live Experience
- Optional: Watch live stream (LiveKit)
- View real-time match updates
- Monitor bet status

### Phase 5: Results & Settlement
- AI Detection (Gemini Vision) extracts final score
- System verifies result (admin review if disputed)
- Calculate bet outcome (Win/Loss/Push)

### Phase 6: Post-Match Actions
- View winnings/losses
- Withdraw funds (Paystack payout)
- View bet history
- Invite friends (referral program)
- Manage account settings

---

## Key Features Highlighted

🔑 **Authentication**: Firebase secure login  
💳 **Payments**: Paystack for deposits/withdrawals  
🤖 **AI Detection**: Gemini Vision API for automated score detection  
🎥 **Live Streaming**: LiveKit integration for match viewing  
👥 **Referrals**: Earn commissions by inviting friends  
⚙️ **Settings**: Account management & responsible gaming options  
📊 **History**: Complete bet tracking and analytics  
