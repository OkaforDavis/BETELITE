import re

with open('detection_service/app.py', 'r', encoding='utf-8') as f:
    content = f.read()

# 1. Replace parse_football_score completely
old_parse_football = re.search(r'    def parse_football_score\(self, texts, results=None, img_height=None\):.*?    def parse_fps_score\(self, texts, results, target_gamertag=""\):', content, re.DOTALL)

new_parse_football = '''    def parse_football_score(self, texts, results=None, img_height=None, img_width=None, target_gamertag="", opponent_gamertag=""):
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

    def parse_fps_score(self, texts, results, target_gamertag=""):'''

if old_parse_football:
    content = content.replace(old_parse_football.group(0), new_parse_football)

# 2. Update analyze_image
content = content.replace('def analyze_image(self, game_type: str, image_bytes: bytes, target_gamertag: str = ""):', 'def analyze_image(self, game_type: str, image_bytes: bytes, target_gamertag: str = "", opponent_gamertag: str = ""):')
content = content.replace('parsed = self.parse_football_score(texts, results, img_height)', 'parsed = self.parse_football_score(texts, results, img_height, img_width, target_gamertag, opponent_gamertag)')

# 3. Update detect_frame
content = content.replace('async def detect_frame(game: str = Form(""), target_gamertag: str = Form(""), file: UploadFile = None, image_b64: str = Form("")):', 'async def detect_frame(game: str = Form(""), target_gamertag: str = Form(""), opponent_gamertag: str = Form(""), file: UploadFile = None, image_b64: str = Form("")):')
content = content.replace('result = detector.analyze_image(game, image_bytes, target_gamertag)', 'result = detector.analyze_image(game, image_bytes, target_gamertag, opponent_gamertag)')

with open('detection_service/app.py', 'w', encoding='utf-8') as f:
    f.write(content)

print("Updated app.py successfully!")
