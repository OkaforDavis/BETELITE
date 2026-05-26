from sqlalchemy import create_engine, Column, Integer, String, Float, Boolean, ForeignKey, Enum
from sqlalchemy.orm import declarative_base, sessionmaker, relationship
import enum

SQLALCHEMY_DATABASE_URL = "sqlite:///./crestleague.db"
# For MySQL later: "mysql+pymysql://user:password@localhost/crestleague"

engine = create_engine(SQLALCHEMY_DATABASE_URL, connect_args={"check_same_thread": False})
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()

class WagerStatus(enum.Enum):
    PENDING = "PENDING"
    ACTIVE = "ACTIVE"
    RESOLVED = "RESOLVED"
    DISPUTED = "DISPUTED"

class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True, index=True)
    username = Column(String, unique=True, index=True)
    wallet_balance = Column(Float, default=0.0)
    current_league_id = Column(Integer, ForeignKey("leagues.id"), nullable=True)

    standings = relationship("LeagueStanding", back_populates="user")

class League(Base):
    __tablename__ = "leagues"
    id = Column(Integer, primary_key=True, index=True)
    name = Column(String, index=True) # e.g. "Division 5"
    tier_level = Column(Integer) # Lower is better, e.g. 1 is highest
    entry_fee = Column(Float, default=0.0)

class LeagueStanding(Base):
    __tablename__ = "league_standings"
    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(Integer, ForeignKey("users.id"))
    league_id = Column(Integer, ForeignKey("leagues.id"))
    season = Column(Integer)
    points = Column(Integer, default=0)
    wins = Column(Integer, default=0)
    losses = Column(Integer, default=0)
    goals_diff = Column(Integer, default=0)

    user = relationship("User", back_populates="standings")

class Wager(Base):
    __tablename__ = "wagers"
    id = Column(Integer, primary_key=True, index=True)
    player1_id = Column(Integer, ForeignKey("users.id"))
    player2_id = Column(Integer, ForeignKey("users.id"))
    amount = Column(Float)
    status = Column(Enum(WagerStatus), default=WagerStatus.PENDING)
    winner_id = Column(Integer, ForeignKey("users.id"), nullable=True)

class MarketplaceSlot(Base):
    __tablename__ = "marketplace_slots"
    id = Column(Integer, primary_key=True, index=True)
    seller_id = Column(Integer, ForeignKey("users.id"))
    league_id = Column(Integer, ForeignKey("leagues.id"))
    price = Column(Float)
    status = Column(String, default="OPEN") # OPEN, SOLD

Base.metadata.create_all(bind=engine)
