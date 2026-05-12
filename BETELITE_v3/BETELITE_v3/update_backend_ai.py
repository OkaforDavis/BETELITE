import os
import re

file_path = 'detection_service/app.py'
with open(file_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Update analyze_image signature
content = content.replace(
    'def analyze_image(self, game_type: str, image_bytes: bytes):',
    'def analyze_image(self, game_type: str, image_bytes: bytes, target_gamertag: str = ""):'
)

# Update call to parse_fps_score
content = content.replace(
    'parsed = self.parse_fps_score(texts)',
    'parsed = self.parse_fps_score(texts, results, target_gamertag)'
)

# Update endpoint signature
content = content.replace(
    'async def detect_frame(game: str = Form(""), file: UploadFile = None, image_b64: str = Form("")):',
    'async def detect_frame(game: str = Form(""), target_gamertag: str = Form(""), file: UploadFile = None, image_b64: str = Form("")):'
)

# Update analyze_image call in endpoint
content = content.replace(
    'result = detector.analyze_image(game, image_bytes)',
    'result = detector.analyze_image(game, image_bytes, target_gamertag)'
)

# Replace parse_fps_score completely
new_fps_parser = '''
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
                    m = re.search(r'\\b(\\d+)\\b', str(texts[j]))
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
'''

# We need to replace the old parse_fps_score with the new one
old_parser_start = content.find('def parse_fps_score(self, texts):')
old_parser_end = content.find('def analyze_image(self, game_type: str', old_parser_start)

content = content[:old_parser_start] + new_fps_parser.strip() + '\\n\\n    ' + content[old_parser_end:]

with open(file_path, 'w', encoding='utf-8') as f:
    f.write(content)

print('Updated detection_service/app.py with advanced fuzzy tag matching')
