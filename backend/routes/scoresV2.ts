import express, { Request, Response, Router } from 'express';
import multer from 'multer';
import path from 'path';
import {
  detectScores,
  detectScoresFromMultiple,
  detectScoresFromURL,
  ScoreDetectionResult,
} from '../services/scoreDetection';
import { verifyToken } from '../middleware/googleOAuth';
import { db } from '../services/firebase';

const router: Router = express.Router();

// Configure multer for file uploads
const storage = multer.diskStorage({
  destination: (req, file, cb) => {
    cb(null, path.join(__dirname, '../../uploads/scores'));
  },
  filename: (req, file, cb) => {
    cb(null, `${Date.now()}-${Math.random().toString(36).substr(2, 9)}${path.extname(file.originalname)}`);
  },
});

const upload = multer({
  storage,
  fileFilter: (req, file, cb) => {
    const allowedMimes = ['image/png', 'image/jpeg', 'image/jpg', 'image/webp'];
    if (allowedMimes.includes(file.mimetype)) {
      cb(null, true);
    } else {
      cb(new Error('Only image files are allowed'));
    }
  },
  limits: {
    fileSize: 10 * 1024 * 1024, // 10MB
  },
});

/**
 * ═══════════════════════════════════════════════════════════
 * SCORE DETECTION ROUTES
 * ═══════════════════════════════════════════════════════════
 */

/**
 * POST /api/scores/detect
 * Upload screenshot and detect score
 */
router.post('/detect', verifyToken, upload.single('screenshot'), async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    if (!req.file) {
      return res.status(400).json({ error: 'No file uploaded' });
    }

    // Process image
    const detectionResult = await detectScores(req.file.path);

    // Save detection to database
    const savedDetection = await saveScoreDetection(req.user.userId, detectionResult, req.file.path);

    res.json({
      success: true,
      detection: savedDetection,
      confidence: detectionResult.confidence,
    });
  } catch (error) {
    console.error('Score Detection Error:', error);
    res.status(500).json({ error: `Score detection failed: ${error}` });
  }
});

/**
 * POST /api/scores/detect-url
 * Detect score from image URL
 */
router.post('/detect-url', verifyToken, async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    const { imageURL } = req.body;

    if (!imageURL) {
      return res.status(400).json({ error: 'Image URL required' });
    }

    // Process image from URL
    const detectionResult = await detectScoresFromURL(imageURL);

    // Save detection to database
    const savedDetection = await saveScoreDetection(req.user.userId, detectionResult, imageURL);

    res.json({
      success: true,
      detection: savedDetection,
      confidence: detectionResult.confidence,
    });
  } catch (error) {
    console.error('URL Score Detection Error:', error);
    res.status(500).json({ error: `Score detection from URL failed: ${error}` });
  }
});

/**
 * POST /api/scores/detect-batch
 * Detect scores from multiple screenshots
 */
router.post('/detect-batch', verifyToken, upload.array('screenshots', 10), async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    if (!req.files || req.files.length === 0) {
      return res.status(400).json({ error: 'No files uploaded' });
    }

    const files = req.files as Express.Multer.File[];
    const filePaths = files.map((f) => f.path);

    // Process multiple images
    const detectionResults = await detectScoresFromMultiple(filePaths);

    // Save all detections
    const savedDetections = await Promise.all(
      detectionResults.map((result, index) => saveScoreDetection(req.user!.userId, result, filePaths[index]))
    );

    res.json({
      success: true,
      detections: savedDetections,
      count: savedDetections.length,
    });
  } catch (error) {
    console.error('Batch Score Detection Error:', error);
    res.status(500).json({ error: `Batch score detection failed: ${error}` });
  }
});

/**
 * POST /api/scores/{detectionId}/confirm
 * Confirm detected score and update match/tournament
 */
router.post('/:detectionId/confirm', verifyToken, async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    const { detectionId } = req.params;
    const { matchId, tournamentId, updateBoard = true } = req.body;

    // Get detection
    const detection = await getScoreDetectionById(detectionId);
    if (!detection) {
      return res.status(404).json({ error: 'Detection not found' });
    }

    // Verify ownership
    if (detection.user_id !== req.user.userId) {
      return res.status(403).json({ error: 'Not authorized' });
    }

    // Update match score in database
    if (matchId) {
      await updateMatchScore(matchId, detection.scores);
    }

    // Update tournament standings if applicable
    if (tournamentId) {
      await updateTournamentStandings(tournamentId, detection);
    }

    // Mark detection as confirmed
    await confirmScoreDetection(detectionId);

    res.json({
      success: true,
      message: 'Score confirmed and board updated',
      detection,
    });
  } catch (error) {
    console.error('Confirm Score Error:', error);
    res.status(500).json({ error: `Failed to confirm score: ${error}` });
  }
});

/**
 * GET /api/scores/detections
 * Get user's score detections
 */
router.get('/detections', verifyToken, async (req: Request, res: Response) => {
  try {
    if (!req.user) {
      return res.status(401).json({ error: 'Not authenticated' });
    }

    const { limit = 20, offset = 0 } = req.query;

    const detections = await getUserScoreDetections(req.user.userId, parseInt(limit as string), parseInt(offset as string));

    res.json({
      success: true,
      count: detections.length,
      detections,
    });
  } catch (error) {
    console.error('Get Detections Error:', error);
    res.status(500).json({ error: 'Failed to get detections' });
  }
});

/**
 * ═══════════════════════════════════════════════════════════
 * DATABASE HELPER FUNCTIONS
 * ═══════════════════════════════════════════════════════════
 */

interface ScoreDetectionRecord {
  detection_id: string;
  user_id: string;
  game_type: string;
  match_type: string;
  player_name: string;
  scores: { player: number; opponent: number };
  kills?: number;
  deaths?: number;
  assists?: number;
  confidence: number;
  is_confirmed: boolean;
  image_url?: string;
  created_at: Date;
  raw_text: string;
}

async function saveScoreDetection(
  userId: string,
  detection: ScoreDetectionResult,
  imagePath: string
): Promise<ScoreDetectionRecord> {
  const newDetectionRef = db.ref('score_detections').push();
  const detectionId = newDetectionRef.key as string;

  const record: ScoreDetectionRecord = {
    detection_id: detectionId,
    user_id: userId,
    game_type: detection.gameType,
    match_type: detection.matchType,
    player_name: detection.playerName,
    scores: detection.scores,
    kills: detection.kills,
    deaths: detection.deaths,
    assists: detection.assists,
    confidence: detection.confidence,
    is_confirmed: false,
    image_url: imagePath,
    created_at: new Date(),
    raw_text: detection.rawText,
  };

  await newDetectionRef.set(record);
  return record;
}

async function getScoreDetectionById(detectionId: string): Promise<ScoreDetectionRecord | null> {
  try {
    const snapshot = await db.ref(`score_detections/${detectionId}`).once('value');
    if (snapshot.exists()) {
      return { detection_id: detectionId, ...snapshot.val() };
    }
    return null;
  } catch (error) {
    console.error('Get Detection Error:', error);
    return null;
  }
}

async function getUserScoreDetections(userId: string, limit: number, offset: number): Promise<ScoreDetectionRecord[]> {
  try {
    const snapshot = await db.ref('score_detections').orderByChild('user_id').equalTo(userId).once('value');
    if (snapshot.exists()) {
      const detections = snapshot.val();
      return Object.entries(detections)
        .map(([id, data]: any) => ({
          detection_id: id,
          ...data,
        }))
        .slice(offset, offset + limit);
    }
    return [];
  } catch (error) {
    console.error('Get User Detections Error:', error);
    return [];
  }
}

async function confirmScoreDetection(detectionId: string): Promise<void> {
  await db.ref(`score_detections/${detectionId}`).update({
    is_confirmed: true,
    confirmed_at: new Date(),
  });
}

async function updateMatchScore(matchId: string, scores: { player: number; opponent: number }): Promise<void> {
  await db.ref(`matches/${matchId}`).update({
    score_player: scores.player,
    score_opponent: scores.opponent,
    updated_at: new Date(),
  });
}

async function updateTournamentStandings(tournamentId: string, detection: ScoreDetectionResult): Promise<void> {
  // Update tournament participant standings
  const participantsRef = db.ref(`tournament_participants`).orderByChild('tournament_id').equalTo(tournamentId);

  const snapshot = await participantsRef.once('value');
  if (snapshot.exists()) {
    const participants = snapshot.val();
    for (const [id, participant]: any of Object.entries(participants)) {
      if (participant.player_name === detection.playerName) {
        await db.ref(`tournament_participants/${id}`).update({
          score: detection.scores.player,
          kills: detection.kills || 0,
          deaths: detection.deaths || 0,
          assists: detection.assists || 0,
          updated_at: new Date(),
        });
      }
    }
  }
}

export default router;
