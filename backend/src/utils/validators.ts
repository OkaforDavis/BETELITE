import Joi from 'joi';

export const registerSchema = Joi.object({
  username: Joi.string().alphanum().min(3).max(50).required(),
  email: Joi.string().email().required(),
  password: Joi.string().min(8).required(),
  confirmPassword: Joi.string().valid(Joi.ref('password')).required(),
  dateOfBirth: Joi.date().max('now').optional(),
  countryCode: Joi.string().length(2).optional(),
});

export const loginSchema = Joi.object({
  email: Joi.string().email().required(),
  password: Joi.string().required(),
});

export const tournamentSchema = Joi.object({
  tournamentName: Joi.string().min(3).max(200).required(),
  gameId: Joi.number().integer().positive().required(),
  tournamentType: Joi.string().valid('solo', 'team').required(),
  tournamentFormat: Joi.string().valid('single_elimination', 'double_elimination', 'round_robin', 'swiss').required(),
  maxParticipants: Joi.number().integer().min(2).max(256).required(),
  entryFee: Joi.number().min(0).default(0),
  prizePool: Joi.number().min(0).default(0),
  registrationStart: Joi.date().required(),
  registrationEnd: Joi.date().min(Joi.ref('registrationStart')).required(),
  tournamentStart: Joi.date().min(Joi.ref('registrationEnd')).required(),
  rules: Joi.string().optional(),
  streamingEnabled: Joi.boolean().default(true),
  bettingEnabled: Joi.boolean().default(false),
});

export const betSchema = Joi.object({
  matchId: Joi.number().integer().positive().required(),
  betOnParticipantId: Joi.number().integer().positive().required(),
  betAmount: Joi.number().positive().min(1).required(),
  betType: Joi.string().valid('match_winner', 'score_prediction', 'handicap', 'over_under').default('match_winner'),
});
