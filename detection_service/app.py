"""
BETELITE AI Detection Service
Detects goals, offsides, and other game events using Python + OpenCV + ML
"""

import cv2
import numpy as np
from flask import Flask, request, jsonify, Response
from threading import Thread
import requests
import json
from datetime import datetime
from typing import Dict, List, Tuple
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

class GameDetector:
    """Main detection class for game analysis"""
    
    def __init__(self):
        self.active_matches: Dict = {}
        self.detection_history: Dict = {}
        self.confidence_threshold = 0.75
        
    def start_detection(self, match_data: Dict) -> Dict:
        """Start detection stream for a match"""
        match_id = match_data.get('matchId')
        game = match_data.get('game')
        
        if match_id in self.active_matches:
            return {'error': 'Match already being detected'}
        
        self.active_matches[match_id] = {
            'game': game,
            'started_at': datetime.now(),
            'goals_detected': 0,
            'offsides_detected': 0,
            'status': 'running'
        }
        
        self.detection_history[match_id] = []
        
        # Start detection in background thread
        thread = Thread(target=self._run_detection, args=(match_id, game))
        thread.daemon = True
        thread.start()
        
        logger.info(f'Detection started for match {match_id} ({game})')
        return {
            'status': 'started',
            'matchId': match_id,
            'detectionId': f'{match_id}_{datetime.now().timestamp()}'
        }
    
    def _run_detection(self, match_id: str, game: str):
        """Run actual detection loop (placeholder)"""
        # This would connect to screen capture stream
        # For now, we simulate detection events
        
        import time
        import random
        
        while self.active_matches.get(match_id, {}).get('status') == 'running':
            time.sleep(5)
            
            # Simulate random goal detection for demo
            if random.random() < 0.1:  # 10% chance per 5 seconds
                self._emit_goal_detected(match_id, game)
            
            # Simulate random offside detection
            if random.random() < 0.05:  # 5% chance per 5 seconds
                self._emit_offside_detected(match_id, game)
    
    def _emit_goal_detected(self, match_id: str, game: str):
        """Send goal detection event to backend"""
        event = {
            'type': 'goal_detected',
            'game': game,
            'matchId': match_id,
            'team': 'Team A' if np.random.rand() > 0.5 else 'Team B',
            'confidence': round(np.random.uniform(0.8, 0.99), 2),
            'timestamp': datetime.now().isoformat()
        }
        
        self.detection_history[match_id].append(event)
        self.active_matches[match_id]['goals_detected'] += 1
        
        # Send to backend
        try:
            requests.post(
                'http://localhost:3000/api/detect/event',
                json=event,
                timeout=5
            )
            logger.info(f'Goal detected in match {match_id}: {event}')
        except Exception as e:
            logger.error(f'Failed to send goal event: {e}')
    
    def _emit_offside_detected(self, match_id: str, game: str):
        """Send offside detection event to backend"""
        event = {
            'type': 'offside_detected',
            'game': game,
            'matchId': match_id,
            'team': 'Team A' if np.random.rand() > 0.5 else 'Team B',
            'confidence': round(np.random.uniform(0.7, 0.95), 2),
            'timestamp': datetime.now().isoformat()
        }
        
        self.detection_history[match_id].append(event)
        self.active_matches[match_id]['offsides_detected'] += 1
        
        try:
            requests.post(
                'http://localhost:3000/api/detect/event',
                json=event,
                timeout=5
            )
            logger.info(f'Offside detected in match {match_id}: {event}')
        except Exception as e:
            logger.error(f'Failed to send offside event: {e}')
    
    def end_detection(self, match_id: str) -> Dict:
        """End detection for a match"""
        if match_id not in self.active_matches:
            return {'error': 'Match not found'}
        
        self.active_matches[match_id]['status'] = 'stopped'
        
        return {
            'status': 'stopped',
            'matchId': match_id,
            'summary': self.active_matches[match_id],
            'history_count': len(self.detection_history.get(match_id, []))
        }
    
    def get_match_events(self, match_id: str) -> List[Dict]:
        """Get all detected events for a match"""
        return self.detection_history.get(match_id, [])


detector = GameDetector()


# ============= API ENDPOINTS =============

@app.route('/health', methods=['GET'])
def health():
    """Health check"""
    return jsonify({
        'status': 'healthy',
        'service': 'betelite-detection',
        'active_matches': len(detector.active_matches)
    })


@app.route('/api/detect', methods=['POST'])
def start_detection():
    """Start detection for a match"""
    data = request.get_json()
    
    if not data:
        return jsonify({'error': 'No data provided'}), 400
    
    result = detector.start_detection(data)
    
    if 'error' in result:
        return jsonify(result), 400
    
    return jsonify(result), 201


@app.route('/api/detect/<match_id>', methods=['GET'])
def get_match_status(match_id: str):
    """Get detection status for a match"""
    match_data = detector.active_matches.get(match_id)
    
    if not match_data:
        return jsonify({'error': 'Match not found'}), 404
    
    return jsonify({
        'matchId': match_id,
        'status': match_data.get('status'),
        'goalsDetected': match_data.get('goals_detected'),
        'offsidesDetected': match_data.get('offsides_detected'),
        'game': match_data.get('game')
    })


@app.route('/api/detect/<match_id>/stop', methods=['POST'])
def stop_detection(match_id: str):
    """Stop detection for a match"""
    result = detector.end_detection(match_id)
    
    if 'error' in result:
        return jsonify(result), 400
    
    return jsonify(result)


@app.route('/api/detect/<match_id>/events', methods=['GET'])
def get_match_events(match_id: str):
    """Get all detected events for a match"""
    events = detector.get_match_events(match_id)
    
    return jsonify({
        'matchId': match_id,
        'events': events,
        'totalEvents': len(events)
    })


@app.route('/api/detect/stream/<match_id>', methods=['GET'])
def stream_events(match_id: str):
    """Stream detection events (Server-Sent Events)"""
    def event_generator():
        while match_id in detector.active_matches:
            try:
                events = detector.get_match_events(match_id)
                yield f'data: {json.dumps({"events": events})}\n\n'
                import time
                time.sleep(2)
            except Exception as e:
                logger.error(f'Stream error: {e}')
                break
    
    return Response(event_generator(), mimetype='text/event-stream')


# ============= ERROR HANDLERS =============

@app.errorhandler(404)
def not_found(error):
    return jsonify({'error': 'Not found'}), 404


@app.errorhandler(500)
def server_error(error):
    logger.error(f'Server error: {error}')
    return jsonify({'error': 'Internal server error'}), 500


if __name__ == '__main__':
    print('\n🔍 BETELITE Detection Service Starting...')
    print('📊 Detection API: http://localhost:5000')
    print('🎥 Stream Endpoint: http://localhost:5000/api/detect/stream/<match_id>\n')
    
    # Run Flask app
    app.run(
        host='0.0.0.0',
        port=5000,
        debug=True,
        threaded=True
    )
