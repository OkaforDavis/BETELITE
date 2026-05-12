"""
BETELITE AI Detection Service - FastAPI + EasyOCR
Detects game scores from uploaded screenshots
"""
import os
os.environ['PYTHONIOENCODING'] = 'utf-8'
os.environ['PYTHONUTF8'] = '1'


import cv2
import numpy as np
import base64
import re
import io
import logging
from fastapi import FastAPI, UploadFile, Form, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
import easyocr

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

logger.info("Loading EasyOCR Model (this takes a moment)...")
# cpu mode for Intel Iris (auto-fallback if no CUDA)
reader = easyocr.Reader(['en'], gpu=False) 
logger.info("EasyOCR Initialized.")

class GameDetector:
    def __init__(self):
        self.confidence_threshold = 0.4
        
    def parse_football_score(self, texts, results=None, img_height=None):
        """
        Smart football score parser (FIFA, eFootball, DLS).
        Uses bounding box positions to find the scoreline near the TOP of the image,
        and filters out stat numbers (e.g. percentages, large numbers).
        """
        # First try direct pattern "4 - 0" or "4:0"
        score_pattern = re.compile(r'(\d{1,2})\s*[-:]\s*(\d{1,2})')
        for text in texts:
            m = score_pattern.search(text)
            if m:
                return {
                    'scoreHome': int(m.group(1)),
                    'scoreAway': int(m.group(2)),
                    'detected': True,
                    'notes': f'Direct score pattern: {m.group(0)}'
                }

        # Use positional data: look for small numbers (0-20) in the TOP 30% of the image
        if results and img_height:
            top_threshold = img_height * 0.30
            top_numbers = []
            for (bbox, text, conf) in results:
                if conf < self.confidence_threshold:
                    continue
                # bbox is [[x1,y1],[x2,y2],[x3,y3],[x4,y4]]
                avg_y = sum(pt[1] for pt in bbox) / 4
                if avg_y < top_threshold and text.isdigit():
                    val = int(text)
                    if 0 <= val <= 20:  # Realistic football scores
                        top_numbers.append(val)

            if len(top_numbers) >= 2:
                return {
                    'scoreHome': top_numbers[0],
                    'scoreAway': top_numbers[1],
                    'detected': True,
                    'notes': f'Positional score detection (top {int(top_threshold)}px)'
                }

        # Final fallback: first two numbers <= 20 in text order
        score_numbers = [int(t) for t in texts if t.isdigit() and 0 <= int(t) <= 20]
        if len(score_numbers) >= 2:
            return {
                'scoreHome': score_numbers[0],
                'scoreAway': score_numbers[1],
                'detected': True,
                'notes': 'First two plausible score numbers found'
            }

        return {'detected': False, 'notes': 'No football score detected'}

    def parse_fps_score(self, texts, results, target_gamertag=""):
        """Advanced heuristics for FPS scoreboards (COD Mobile, etc)"""
        import difflib
        
        # If target_gamertag is provided, do a fuzzy search to find it in the texts
        if target_gamertag:
            target_gamertag = target_gamertag.upper()
            best_match = None
            best_ratio = 0
            best_idx = -1
            
            for i, text in enumerate(texts):
                ratio = difflib.SequenceMatcher(None, target_gamertag, str(text).upper()).ratio()
                if ratio > 0.75 and ratio > best_ratio:
                    best_ratio = ratio
                    best_match = text
                    best_idx = i
                    
            if best_match:
                # Look for numbers nearby in the text stream
                # Often it is "Gamertag", "KILLS", "19"
                found_score = None
                
                # Check up to 5 elements ahead for a number
                for j in range(best_idx + 1, min(best_idx + 6, len(texts))):
                    # Sometimes "KILLS 19" is parsed as one string, or "19" is isolated
                    m = re.search(r'\b(\d+)\b', str(texts[j]))
                    if m:
                        found_score = int(m.group(1))
                        break
                        
                if found_score is not None:
                    return {
                        'detected': True,
                        'target_player': { 'gamerTag': best_match, 'score': found_score },
                        'notes': f'Found target gamertag {best_match} with score {found_score}'
                    }
                else:
                    return {
                        'detected': False,
                        'notes': f'Found gamertag {best_match} but could not extract nearby score'
                    }
            else:
                return {
                    'detected': False,
                    'notes': f'Target gamertag {target_gamertag} not found in screenshot'
                }

        # Fallback to general player extraction
        tags_and_scores = []
        for i, text in enumerate(texts):
            text_str = str(text)
            if text_str.isdigit() and i > 0 and not str(texts[i-1]).isdigit():
                tags_and_scores.append({
                    'gamerTag': texts[i-1],
                    'score': int(text_str)
                })

        if tags_and_scores:
            return {
                'detected': True,
                'players': tags_and_scores,
                'notes': 'Detected FPS players and scores'
            }

        return {'detected': False, 'notes': 'No FPS scoreboard detected'}

    def analyze_image(self, game_type: str, image_bytes: bytes, target_gamertag: str = ""):
        nparr = np.frombuffer(image_bytes, np.uint8)
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)

        if img is None:
            raise ValueError("Invalid image file")

        img_height, img_width = img.shape[:2]

        # Run EasyOCR with full result including bounding boxes
        results = reader.readtext(img)
        texts = [res[1] for res in results if res[2] > self.confidence_threshold]
        logger.info(f"Detected texts: {texts}")

        game_type_lower = game_type.lower() if game_type else ""
        if game_type_lower in ['fifa', 'efootball', 'dls', 'dream', 'football']:
            parsed = self.parse_football_score(texts, results, img_height)
        elif game_type_lower in ['cod', 'pubg', 'free fire', 'fps']:
            parsed = self.parse_fps_score(texts, results, target_gamertag)
        else:
            parsed = {'detected': False, 'notes': f'Unsupported game: {game_type}'}

        parsed['raw_text'] = texts
        return parsed


detector = GameDetector()

@app.get("/health")
def health_check():
    return {"status": "healthy", "service": "betelite-ai-fastapi"}

@app.post("/api/detect/frame")
async def detect_frame(game: str = Form(""), target_gamertag: str = Form(""), file: UploadFile = None, image_b64: str = Form("")):
    """Endpoint to detect scores from either a multipart file or base64 string"""
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
            
        result = detector.analyze_image(game, image_bytes, target_gamertag)
        return result
    except ValueError as ve:
        raise HTTPException(status_code=400, detail=str(ve))
    except Exception as e:
        logger.error(f"Analysis error: {e}")
        raise HTTPException(status_code=500, detail="Internal server error during analysis")

if __name__ == "__main__":
    import uvicorn
    print("\\nBETELITE AI Detection (FastAPI + EasyOCR) Starting...")
    print("API: http://localhost:5000\\n")
    uvicorn.run(app, host="0.0.0.0", port=5000)
