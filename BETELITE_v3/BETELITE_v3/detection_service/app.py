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
        
    def parse_football_score(self, texts, results=None, img_height=None, img_width=None, target_gamertag="", opponent_gamertag=""):
        """
        Spatial football score parser (FIFA, eFootball, DLS).
        Uses bounding box positions to map Left/Right scores to Left/Right players.
        """
        import difflib
        
        if not (results and img_height and img_width and target_gamertag):
            return {'detected': False, 'notes': 'Missing required parameters for spatial OCR'}
            
        target_gamertag = target_gamertag.upper()
        opponent_gamertag = opponent_gamertag.upper() if opponent_gamertag else ""
        
        # 1. Find all digits <= 20 in the top 30% of the image
        top_threshold = img_height * 0.35
        top_scores = []
        for (bbox, text, conf) in results:
            if conf < self.confidence_threshold: continue
            
            clean_text = "".join(filter(str.isdigit, str(text)))
            if not clean_text: continue
            
            avg_y = sum(pt[1] for pt in bbox) / 4
            avg_x = sum(pt[0] for pt in bbox) / 4
            
            if avg_y < top_threshold:
                val = int(clean_text)
                if 0 <= val <= 20:
                    top_scores.append({'val': val, 'x': avg_x})
                    
        # Sort scores from left to right based on X coordinate
        top_scores.sort(key=lambda s: s['x'])
        
        if len(top_scores) < 2:
            return {'detected': False, 'notes': f'Found {len(top_scores)} top scores, need 2'}
            
        left_score = top_scores[0]['val']
        right_score = top_scores[1]['val']
        
        # 2. Find target gamertag side
        target_x = None
        for (bbox, text, conf) in results:
            text_upper = str(text).upper()
            ratio = difflib.SequenceMatcher(None, target_gamertag, text_upper).ratio()
            if ratio > 0.75 or (target_gamertag in text_upper and len(target_gamertag) > 3):
                target_x = sum(pt[0] for pt in bbox) / 4
                break
                
        if target_x is None:
            return {'detected': False, 'notes': f'Target {target_gamertag} not found'}
            
        # 3. Determine if target is Left or Right
        is_target_left = target_x < (img_width / 2)
        
        target_final_score = left_score if is_target_left else right_score
        opponent_final_score = right_score if is_target_left else left_score
        
        return {
            'detected': True,
            'target_player': {
                'gamerTag': target_gamertag,
                'score': target_final_score,
                'side': 'LEFT' if is_target_left else 'RIGHT'
            },
            'opponent': {
                'gamerTag': opponent_gamertag or "Opponent",
                'score': opponent_final_score,
                'side': 'RIGHT' if is_target_left else 'LEFT'
            },
            'notes': f'Spatial OCR success: {target_gamertag} ({target_final_score}) vs Opponent ({opponent_final_score})'
        }

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
                text_upper = str(text).upper()
                ratio = difflib.SequenceMatcher(None, target_gamertag, text_upper).ratio()
                
                # Check if it's a very close match OR if the gamertag is a direct substring of a longer phrase (like "SQUANTAKAY KILLS 19")
                if ratio > 0.75 or (target_gamertag in text_upper and len(target_gamertag) > 3):
                    best_ratio = max(ratio, 1.0 if target_gamertag in text_upper else ratio)
                    best_match = text
                    best_idx = i
                    
            if best_match:
                # Look for numbers nearby in the text stream OR inside the same text block
                found_score = None
                
                # First check if the score is in the exact same text block (e.g., "SQUANTAKAY KILLS 19")
                m_inline = re.search(r'\b(\d+)\b', str(best_match))
                if m_inline:
                    found_score = int(m_inline.group(1))
                else:
                    # Check up to 5 elements ahead for a number
                    for j in range(best_idx + 1, min(best_idx + 6, len(texts))):
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

    def analyze_image(self, game_type: str, image_bytes: bytes, target_gamertag: str = "", opponent_gamertag: str = ""):
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
        if any(g in game_type_lower for g in ['fifa', 'efootball', 'dls', 'dream', 'football']):
            parsed = self.parse_football_score(texts, results, img_height, img_width, target_gamertag, opponent_gamertag)
        elif any(g in game_type_lower for g in ['cod', 'pubg', 'free fire', 'fps', 'call of duty']):
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
async def detect_frame(game: str = Form(""), target_gamertag: str = Form(""), opponent_gamertag: str = Form(""), file: UploadFile = None, image_b64: str = Form("")):
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
            
        result = detector.analyze_image(game, image_bytes, target_gamertag, opponent_gamertag)
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
