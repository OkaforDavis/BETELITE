from fastapi import FastAPI, Depends, HTTPException
from sqlalchemy.orm import Session
import database as db
from pydantic import BaseModel
from typing import List, Optional

app = FastAPI(title="CrestArena League & Wager Backend")

# Dependency
def get_db():
    database = db.SessionLocal()
    try:
        yield database
    finally:
        database.close()

# --- Pydantic Schemas ---
class UserCreate(BaseModel):
    username: str

class WagerCreate(BaseModel):
    player1_id: int
    player2_id: int
    amount: float

class WagerResolve(BaseModel):
    winner_id: int

class MarketplaceList(BaseModel):
    seller_id: int
    league_id: int
    price: float

# --- Routes ---

@app.post("/api/v1/users")
def create_user(user: UserCreate, session: Session = Depends(get_db)):
    db_user = db.User(username=user.username, wallet_balance=0.0)
    session.add(db_user)
    session.commit()
    session.refresh(db_user)
    return db_user

@app.get("/api/v1/users/{user_id}")
def get_user(user_id: int, session: Session = Depends(get_db)):
    user = session.query(db.User).filter(db.User.id == user_id).first()
    if not user:
        raise HTTPException(status_code=404, detail="User not found")
    return user

@app.post("/api/v1/wagers")
def create_wager(wager: WagerCreate, session: Session = Depends(get_db)):
    # Check balances
    p1 = session.query(db.User).filter(db.User.id == wager.player1_id).first()
    p2 = session.query(db.User).filter(db.User.id == wager.player2_id).first()
    
    if not p1 or not p2:
        raise HTTPException(status_code=404, detail="Players not found")
    
    if p1.wallet_balance < wager.amount or p2.wallet_balance < wager.amount:
        raise HTTPException(status_code=400, detail="Insufficient funds")

    # Deduct funds into escrow
    p1.wallet_balance -= wager.amount
    p2.wallet_balance -= wager.amount

    db_wager = db.Wager(
        player1_id=wager.player1_id,
        player2_id=wager.player2_id,
        amount=wager.amount,
        status=db.WagerStatus.ACTIVE
    )
    session.add(db_wager)
    session.commit()
    session.refresh(db_wager)
    return db_wager

@app.post("/api/v1/wagers/{wager_id}/resolve")
def resolve_wager(wager_id: int, resolve: WagerResolve, session: Session = Depends(get_db)):
    wager = session.query(db.Wager).filter(db.Wager.id == wager_id).first()
    if not wager or wager.status != db.WagerStatus.ACTIVE:
        raise HTTPException(status_code=400, detail="Wager not active")

    if resolve.winner_id not in [wager.player1_id, wager.player2_id]:
        raise HTTPException(status_code=400, detail="Invalid winner")

    winner = session.query(db.User).filter(db.User.id == resolve.winner_id).first()
    
    # Total pot
    total_pot = wager.amount * 2
    # Platform keeps 50% as requested by user
    platform_fee = total_pot * 0.5
    winner_payout = total_pot - platform_fee

    winner.wallet_balance += winner_payout

    wager.status = db.WagerStatus.RESOLVED
    wager.winner_id = resolve.winner_id

    session.commit()
    return {"message": "Wager resolved", "winner_payout": winner_payout, "platform_fee": platform_fee}

# League Logic Simulation
@app.post("/api/v1/leagues/season/end")
def end_season(league_id: int, session: Session = Depends(get_db)):
    """
    Cron job endpoint: Ends the season for a league, promotes top 3, relegates bottom 3.
    """
    standings = session.query(db.LeagueStanding).filter(db.LeagueStanding.league_id == league_id).order_by(db.LeagueStanding.points.desc()).all()
    
    if len(standings) < 6:
        return {"message": "Not enough players to process promotion/relegation"}

    promoted = standings[:3]
    relegated = standings[-3:]
    
    # In a real app, we update their user.current_league_id here based on tier_level of leagues.
    
    return {
        "message": "Season ended",
        "promoted_user_ids": [s.user_id for s in promoted],
        "relegated_user_ids": [s.user_id for s in relegated]
    }

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
