"""
BETELITE AI Detection Service - FastAPI + LLM Vision
Detects game scores from uploaded screenshots using LLM vision APIs
(OpenAI, Gemini, Groq) inspired by receipt-ocr approach.
"""
import os
os.environ['PYTHONIOENCODING'] = 'utf-8'
os.environ['PYTHONUTF8'] = '1'

import base64
import json
import logging
from typing import Optional, List

from fastapi import FastAPI, UploadFile, Form, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from openai import OpenAI

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="BETELITE AI Detection")

# Enable CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# ─── Pydantic models for structured LLM output ───────────────────────

class PlayerScore(BaseModel):
    gamertag: str
    score: int
    side: Optional[str] = None  # LEFT / RIGHT for football

class GameScoreResult(BaseModel):
    detected: bool
    game_type: Optional[str] = None
    target_player: Optional[PlayerScore] = None
    opponent: Optional[PlayerScore] = None
    players: Optional[List[PlayerScore]] = None
    notes: str = ""
    raw_text: Optional[List[str]] = None

# ─── LLM Client Setup ────────────────────────────────────────────────

def get_llm_client() -> tuple[OpenAI, str]:
    """
    Returns (client, model_name) based on available env vars.
    Priority: GEMINI_API_KEY > OPENAI_API_KEY > GROQ_API_KEY
    Uses OpenAI-compatible API for all providers (like receipt-ocr does).
    """
    gemini_key = os.getenv("GEMINI_API_KEY", "")
    openai_key = os.getenv("OPENAI_API_KEY", "")
    groq_key = os.getenv("GROQ_API_KEY", "")

    if gemini_key:
        return OpenAI(
            api_key=gemini_key,
            base_url="https://generativelanguage.googleapis.com/v1beta/openai/"
        ), os.getenv("GEMINI_MODEL", "gemini-2.0-flash")
    elif openai_key:
        return OpenAI(api_key=openai_key), os.getenv("OPENAI_MODEL", "gpt-4o")
    elif groq_key:
        return OpenAI(
            api_key=groq_key,
            base_url="https://api.groq.com/openai/v1"
        ), os.getenv("GROQ_MODEL", "llama-3.2-90b-vision-preview")
    else:
        raise RuntimeError(
            "No LLM API key found. Set GEMINI_API_KEY, OPENAI_API_KEY, or GROQ_API_KEY."
        )

# ─── Game Score Extraction ────────────────────────────────────────────

GAME_SCORE_PROMPT = """You are a gaming score detection AI. Analyze this screenshot from a competitive mobile/console game match and extract the scores.

Game type: {game_type}
Target player gamertag: {target_gamertag}
Opponent gamertag: {opponent_gamertag}

Instructions:
1. Identify the scoreboard or result screen in the screenshot
2. Find the scores for each player/team
3. If gamertags are provided, match them to the correct scores using fuzzy matching
4. For football games (FIFA, eFootball, DLS, Dream League), identify LEFT and RIGHT sides
5. For FPS games (COD Mobile, PUBG, Free Fire), find kill counts or match scores

Return your response as VALID JSON ONLY (no markdown, no backticks, no explanation) with this exact structure:
{{
    "detected": true or false,
    "game_type": "detected game type",
    "target_player": {{
        "gamertag": "player name from screenshot",
        "score": <number>,
        "side": "LEFT" or "RIGHT" (for football games only)
    }},
    "opponent": {{
        "gamertag": "opponent name from screenshot",
        "score": <number>,
        "side": "LEFT" or "RIGHT" (for football games only)
    }},
    "notes": "brief description of what you detected"
}}

If you cannot detect scores, return:
{{
    "detected": false,
    "notes": "reason why detection failed"
}}
"""


def encode_image_to_base64(image_bytes: bytes) -> str:
    """Encode image bytes to base64 data URI."""
    b64 = base64.b64encode(image_bytes).decode("utf-8")
    return f"data:image/jpeg;base64,{b64}"


def extract_game_score(
    image_bytes: bytes,
    game_type: str = "",
    target_gamertag: str = "",
    opponent_gamertag: str = ""
) -> GameScoreResult:
    """
    Send screenshot to LLM vision API and extract game scores.
    Uses the receipt-ocr pattern of OpenAI-compatible API with vision.
    """
    try:
        client, model = get_llm_client()
    except RuntimeError as e:
        logger.error(str(e))
        return GameScoreResult(detected=False, notes=str(e))

    prompt = GAME_SCORE_PROMPT.format(
        game_type=game_type or "auto-detect",
        target_gamertag=target_gamertag or "unknown",
        opponent_gamertag=opponent_gamertag or "unknown"
    )

    image_data_uri = encode_image_to_base64(image_bytes)

    try:
        response = client.chat.completions.create(
            model=model,
            messages=[
                {
                    "role": "user",
                    "content": [
                        {"type": "text", "text": prompt},
                        {
                            "type": "image_url",
                            "image_url": {"url": image_data_uri}
                        }
                    ]
                }
            ],
            max_tokens=1024,
            temperature=0.1,  # Low temp for consistent structured output
        )

        raw_response = response.choices[0].message.content.strip()
        logger.info(f"LLM raw response: {raw_response}")

        # Clean response — strip markdown code fences if present
        cleaned = raw_response
        if cleaned.startswith("```"):
            # Remove ```json ... ``` wrapper
            lines = cleaned.split("\n")
            lines = [l for l in lines if not l.strip().startswith("```")]
            cleaned = "\n".join(lines)

        result_data = json.loads(cleaned)

        return GameScoreResult(
            detected=result_data.get("detected", False),
            game_type=result_data.get("game_type"),
            target_player=PlayerScore(**result_data["target_player"]) if result_data.get("target_player") else None,
            opponent=PlayerScore(**result_data["opponent"]) if result_data.get("opponent") else None,
            notes=result_data.get("notes", ""),
        )

    except json.JSONDecodeError as e:
        logger.error(f"Failed to parse LLM response as JSON: {e}")
        logger.error(f"Raw response was: {raw_response}")
        return GameScoreResult(detected=False, notes=f"LLM returned invalid JSON: {str(e)}")
    except Exception as e:
        logger.error(f"LLM vision API error: {e}")
        return GameScoreResult(detected=False, notes=f"LLM API error: {str(e)}")


# ─── API Endpoints ────────────────────────────────────────────────────

@app.get("/health")
def health_check():
    """Health check endpoint."""
    has_key = bool(
        os.getenv("GEMINI_API_KEY") or
        os.getenv("OPENAI_API_KEY") or
        os.getenv("GROQ_API_KEY")
    )
    return {
        "status": "healthy",
        "service": "betelite-ai-llm-vision",
        "llm_configured": has_key
    }


@app.post("/api/detect/frame")
async def detect_frame(
    game: str = Form(""),
    target_gamertag: str = Form(""),
    opponent_gamertag: str = Form(""),
    file: UploadFile = None,
    image_b64: str = Form("")
):
    """
    Endpoint to detect game scores from either a multipart file or base64 string.
    Compatible with the existing Go backend contract.
    """
    image_bytes = None

    try:
        if file and file.filename:
            image_bytes = await file.read()
        elif image_b64:
            # Strip base64 header if present
            if 'base64,' in image_b64:
                image_b64 = image_b64.split('base64,')[1]
            image_bytes = base64.b64decode(image_b64)
        else:
            raise HTTPException(status_code=400, detail="No image provided")

        result = extract_game_score(
            image_bytes,
            game_type=game,
            target_gamertag=target_gamertag,
            opponent_gamertag=opponent_gamertag
        )

        return result.model_dump()

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Detection error: {e}")
        raise HTTPException(status_code=500, detail="Internal server error during analysis")


if __name__ == "__main__":
    import uvicorn
    print("\nBETELITE AI Detection (LLM Vision) Starting...")
    print("API: http://localhost:5000\n")
    uvicorn.run(app, host="0.0.0.0", port=5000)
