"""
BETELITE AI Detection Service
Detects goals, scores, and gamer tags using Python + EasyOCR
Supported Games: FIFA, DLS, COD, eFootball, PUBG, Free Fire
"""

import cv2
import numpy as np
from flask import Flask, request, jsonify, Response
from threading import Thread
import requests
import json
import base64
import re
from datetime import datetime
from typing import Dict, List, Tuple
import logging
import easyocr

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Initialize EasyOCR Reader once
logger.info("Initializing EasyOCR Model...")
reader = easyocr.Reader(['en'], gpu=False) # set gpu=True if CUDA is available
logger.info("EasyOCR Initialized.")

class GameDetector:
    """Main detection class for game analysis using EasyOCR"""
    
    def __init__(self):
        self.active_matches: Dict = {}
        self.detection_history: Dict = {}
        self.confidence_threshold = 0.5
        
    def start_detection(self, match_data: Dict) -> Dict:
        """Start tracking a match session"""
        match_id = match_data.get('matchId')
        game = match_data.get('game')
        
        if match_id in self.active_matches:
            return {'error': 'Match already being detected'}
        
        self.active_matches[match_id] = {
            'game': game,
            'started_at': datetime.now(),
            'status': 'running'
        }
        self.detection_history[match_id] = []
        
        logger.info(f'Detection tracking started for match {match_id} ({game})')
        return {
            'status': 'started',
            'matchId': match_id,
        }

    def stop_detection(self, match_id: str) -> Dict:
        """End tracking for a match"""
        if match_id in self.active_matches:
            self.active_matches[match_id]['status'] = 'stopped'
            return {'status': 'stopped', 'matchId': match_id}
        return {'error': 'Match not found'}

    def parse_football_score(self, texts: List[str]) -> Dict:
        """Heuristics for FIFA, eFootball, DLS"""
        # Look for patterns like "2 - 1" or separate numbers near team names
        score_pattern = re.compile(r'(\d+)\s*[-:]\s*(\d+)')
        for text in texts:
            match = score_pattern.search(text)
            if match:
                return {
                    'scoreHome': int(match.group(1)),
                    'scoreAway': int(match.group(2)),
                    'scoreChanged': True,
                    'notes': f'Detected score {match.group(0)}'
                }
        
        # Fallback logic: find two numbers that might be scores
        numbers = [int(t) for t in texts if t.isdigit() and len(t) < 3]
        if len(numbers) >= 2:
             return {
                 'scoreHome': numbers[0],
                 'scoreAway': numbers[1],
                 'scoreChanged': True,
                 'notes': 'Detected loose numbers as scores'
             }
             
        return {'scoreChanged': False, 'notes': 'No score detected'}

    def parse_fps_score(self, texts: List[str]) -> Dict:
        """Heuristics for COD, PUBG, Free Fire"""
        # Scoreboards usually have lists of Tags and Kills
        # e.g., Player1 15, Player2 12
        # For this prototype, we'll return the raw extracted text so Node.js can map it to gamers
        
        tags_and_scores = []
        for i, text in enumerate(texts):
            # If text is a number and previous text is string, it might be Name + Score
            if text.isdigit() and i > 0 and not texts[i-1].isdigit():
                tags_and_scores.append({
                    'gamerTag': texts[i-1],
                    'score': int(text)
                })
        
        if tags_and_scores:
            return {
                'scoreChanged': True,
                'players': tags_and_scores,
                'notes': 'Detected FPS scoreboard'
            }
            
        return {'scoreChanged': False, 'notes': 'No scoreboard detected'}

    def analyze_frame(self, game_type: str, image_bytes: bytes) -> Dict:
        """Analyze a single frame with EasyOCR"""
        # Convert bytes to numpy array for OpenCV/EasyOCR
        nparr = np.frombuffer(image_bytes, np.uint8)
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        
        if img is None:
             return {'error': 'Invalid image'}
             
        # Run EasyOCR
        results = reader.readtext(img)
        
        # Extract text strings that meet confidence threshold
        texts = [res[1] for res in results if res[2] > self.confidence_threshold]
        logger.info(f"Extracted Texts: {texts}")
        
        game_type_lower = game_type.lower()
        if game_type_lower in ['fifa', 'efootball', 'dls', 'dream', 'dream league soccer']:
             parsed = self.parse_football_score(texts)
        elif game_type_lower in ['cod', 'pubg', 'free fire']:
             parsed = self.parse_fps_score(texts)
        else:
             parsed = {'error': f'Unsupported game type: {game_type}', 'raw_text': texts}
             
        parsed['raw_text'] = texts
        return parsed


detector = GameDetector()


# ============= API ENDPOINTS =============

@app.route('/health', methods=['GET'])
def health():
    return jsonify({
        'status': 'healthy',
        'service': 'betelite-detection-easyocr',
        'active_matches': len(detector.active_matches)
    })


@app.route('/api/detect/start', methods=['POST'])
def start_detection():
    data = request.get_json()
    if not data: return jsonify({'error': 'No data'}), 400
    return jsonify(detector.start_detection(data))


@app.route('/api/detect/stop', methods=['POST'])
def stop_detection():
    data = request.get_json()
    match_id = data.get('matchId') if data else None
    return jsonify(detector.stop_detection(match_id))


@app.route('/api/detect/analyze_frame', methods=['POST'])
def analyze_frame():
    """Receives a base64 image and game type, returns extracted scores"""
    data = request.get_json()
    if not data or 'image' not in data or 'game' not in data:
        return jsonify({'error': 'Missing image or game type'}), 400
        
    game_type = data['game']
    base64_img = data['image']
    
    # Strip base64 header if present
    if 'base64,' in base64_img:
        base64_img = base64_img.split('base64,')[1]
        
    try:
        image_bytes = base64.b64decode(base64_img)
    except Exception as e:
        return jsonify({'error': f'Invalid base64 encoding: {e}'}), 400
        
    try:
        result = detector.analyze_frame(game_type, image_bytes)
        return jsonify(result)
    except Exception as e:
        logger.error(f'Analysis error: {e}')
        return jsonify({'error': 'Analysis failed', 'details': str(e)}), 500


if __name__ == '__main__':
    print('\\n🔍 BETELITE Detection Service (EasyOCR) Starting...')
    print('📊 Detection API: http://localhost:5000')
    app.run(host='0.0.0.0', port=5000, debug=True, threaded=True)
