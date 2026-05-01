import Tesseract from 'tesseract.js';
import axios from 'axios';
import * as fs from 'fs';
import * as path from 'path';

/**
 * Score detection service using Tesseract OCR
 * Detects game scores, player names, and match types from game screenshots
 */

export interface ScoreDetectionResult {
  gameType: 'call_of_duty' | 'fortnite' | 'valorant' | 'csgo' | 'league' | 'dota2' | 'unknown';
  matchType: '1v1' | '15_player' | 'tournament' | 'unknown';
  playerName: string;
  opponentNames: string[];
  scores: {
    player: number;
    opponent: number;
  };
  kills?: number;
  deaths?: number;
  assists?: number;
  timestamp: Date;
  confidence: number; // 0-100
  rawText: string;
}

interface GamePattern {
  gameCode: string;
  gameName: string;
  patterns: RegExp[];
}

// Game detection patterns
const GAME_PATTERNS: GamePattern[] = [
  {
    gameCode: 'cod',
    gameName: 'Call of Duty',
    patterns: [
      /Call of Duty/i,
      /COD/,
      /kills?:\s*(\d+)/i,
      /deaths?:\s*(\d+)/i,
    ],
  },
  {
    gameCode: 'fortnite',
    gameName: 'Fortnite',
    patterns: [/fortnite/i, /eliminations?:\s*(\d+)/i, /placed:\s*(1st|2nd|3rd)/i],
  },
  {
    gameCode: 'valorant',
    gameName: 'Valorant',
    patterns: [/valorant/i, /agents?:/i, /economy:/i],
  },
  {
    gameCode: 'csgo',
    gameName: 'CS:GO',
    patterns: [/csgo|counter\s*strike/i, /eco:\s*\$(\d+)/i],
  },
  {
    gameCode: 'lol',
    gameName: 'League of Legends',
    patterns: [/league\s*of\s*legends|lol/i, /gold:\s*(\d+)/i, /turrets?:\s*(\d+)/i],
  },
  {
    gameCode: 'dota2',
    gameName: 'Dota 2',
    patterns: [/dota\s*2|dota2/i, /gold:\s*(\d+)/i, /items?:/i],
  },
];

// Score patterns for different games
const SCORE_PATTERNS = {
  scores: [
    /score[:\s]+(\d+)[:\s]*(?:vs|-)?\s*(\d+)/i,
    /(\d+)\s*[-:]\s*(\d+)/,
    /final[:\s]+(\d+)[:\s]*-\s*(\d+)/i,
  ],
  playerName: [
    /player[:\s]+([A-Za-z0-9_#]+)/i,
    /username[:\s]+([A-Za-z0-9_#]+)/i,
    /gamertag[:\s]+([A-Za-z0-9_#]+)/i,
    /^([A-Za-z0-9_#]+)[\s:]+(kills?|score|points?)/i,
  ],
  kills: [/kills?[:\s]+(\d+)/i, /k[:\s]+(\d+)/i],
  deaths: [/deaths?[:\s]+(\d+)/i, /d[:\s]+(\d+)/i],
  assists: [/assists?[:\s]+(\d+)/i, /a[:\s]+(\d+)/i],
};

/**
 * Extract text from image using Tesseract OCR
 */
export const extractTextFromImage = async (imagePath: string): Promise<string> => {
  try {
    const { data } = await Tesseract.recognize(imagePath, 'eng', {
      logger: (m) => console.log('OCR Progress:', m),
    });

    return data.text;
  } catch (error) {
    console.error('OCR Error:', error);
    throw new Error(`Failed to extract text from image: ${error}`);
  }
};

/**
 * Detect game type from extracted text
 */
export const detectGameType = (text: string): string => {
  for (const game of GAME_PATTERNS) {
    for (const pattern of game.patterns) {
      if (pattern.test(text)) {
        return game.gameCode;
      }
    }
  }
  return 'unknown';
};

/**
 * Extract player name from text
 */
export const extractPlayerName = (text: string): string => {
  for (const pattern of SCORE_PATTERNS.playerName) {
    const match = text.match(pattern);
    if (match && match[1]) {
      return match[1].trim();
    }
  }
  return 'Unknown';
};

/**
 * Extract scores from text
 */
export const extractScores = (text: string): { player: number; opponent: number } => {
  for (const pattern of SCORE_PATTERNS.scores) {
    const match = text.match(pattern);
    if (match && match[1] && match[2]) {
      return {
        player: parseInt(match[1], 10),
        opponent: parseInt(match[2], 10),
      };
    }
  }
  return { player: 0, opponent: 0 };
};

/**
 * Extract K/D/A stats from text
 */
export const extractStats = (text: string): { kills?: number; deaths?: number; assists?: number } => {
  const stats: any = {};

  const killMatch = text.match(SCORE_PATTERNS.kills[0]) || text.match(SCORE_PATTERNS.kills[1]);
  if (killMatch) stats.kills = parseInt(killMatch[1], 10);

  const deathMatch = text.match(SCORE_PATTERNS.deaths[0]);
  if (deathMatch) stats.deaths = parseInt(deathMatch[1], 10);

  const assistMatch = text.match(SCORE_PATTERNS.assists[0]);
  if (assistMatch) stats.assists = parseInt(assistMatch[1], 10);

  return stats;
};

/**
 * Determine match type based on player count and game type
 */
export const determineMatchType = (gameType: string, text: string): '1v1' | '15_player' | 'tournament' | 'unknown' => {
  const playerCountPatterns = [
    /tournament|tournament\s*mode/i,
    /15\s*player|15player|squad\s*15/i,
    /1v1|1\s*vs\s*1|duel|head\s*to\s*head/i,
  ];

  if (playerCountPatterns[0].test(text)) return 'tournament';
  if (playerCountPatterns[1].test(text)) return '15_player';
  if (playerCountPatterns[2].test(text)) return '1v1';

  return 'unknown';
};

/**
 * Main score detection function
 */
export const detectScores = async (imagePath: string): Promise<ScoreDetectionResult> => {
  try {
    // Extract text using OCR
    const rawText = await extractTextFromImage(imagePath);
    console.log('Extracted Text:', rawText);

    // Detect game type
    const gameType = detectGameType(rawText) as any;

    // Extract player data
    const playerName = extractPlayerName(rawText);
    const scores = extractScores(rawText);
    const stats = extractStats(rawText);

    // Determine match type
    const matchType = determineMatchType(gameType, rawText);

    // Calculate confidence based on detection accuracy
    let confidence = 0;
    if (playerName !== 'Unknown') confidence += 25;
    if (scores.player > 0 || scores.opponent > 0) confidence += 25;
    if (stats.kills !== undefined || stats.deaths !== undefined) confidence += 25;
    if (gameType !== 'unknown') confidence += 25;

    return {
      gameType,
      matchType,
      playerName,
      opponentNames: [], // Would be populated from additional detection
      scores,
      ...stats,
      timestamp: new Date(),
      confidence,
      rawText,
    };
  } catch (error) {
    console.error('Score Detection Error:', error);
    throw error;
  }
};

/**
 * Process multiple screenshots for better accuracy
 */
export const detectScoresFromMultiple = async (imagePaths: string[]): Promise<ScoreDetectionResult[]> => {
  const results = await Promise.all(imagePaths.map((path) => detectScores(path)));
  return results;
};

/**
 * Download and process image from URL
 */
export const detectScoresFromURL = async (imageURL: string): Promise<ScoreDetectionResult> => {
  try {
    const response = await axios.get(imageURL, { responseType: 'arraybuffer' });
    const tempPath = path.join('/tmp', `score-${Date.now()}.png`);

    // Save image temporarily
    fs.writeFileSync(tempPath, response.data);

    // Process image
    const result = await detectScores(tempPath);

    // Cleanup
    fs.unlinkSync(tempPath);

    return result;
  } catch (error) {
    console.error('URL Processing Error:', error);
    throw error;
  }
};
