package services

// Game describes a title CrestArena supports for verified matches, plus the
// hints the OCR prompt needs to recognise a valid player-vs-player result.
type Game struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Short        string `json:"short"`
	NameHint     string `json:"nameHint"`     // shown in the profile form
	Football     bool   `json:"football"`     // draws in knockouts go to penalties
	ResultScreen string `json:"resultScreen"` // what to screenshot, in plain words
	ModeHint     string `json:"modeHint"`     // which game mode counts
	ocrHints     string
}

var supportedGames = []Game{
	{
		ID:       "fc_mobile",
		Name:     "EA SPORTS FC Mobile",
		Short:    "FC Mobile",
		NameHint: "Your FC Mobile username, exactly as it appears on the Head-to-Head result screen.",
		Football: true, ResultScreen: "full-time result screen", ModeHint: "Head-to-Head, VS Attack or a friendly vs a real player",
		ocrHints: `EA SPORTS FC Mobile. Valid: Head-to-Head / VS Attack / friendly match against another human, on the FULL TIME result screen showing both usernames and the final score.
Reject: Scenario / Events / Campaign / "TARGET:" objectives, any match vs CPU/AI, the pause menu (RESUME / QUIT buttons, a running clock), replays, and screens that only show national/club team names without usernames.`,
	},
	{
		ID:       "efootball",
		Name:     "eFootball",
		Short:    "eFootball",
		NameHint: "Your eFootball user name, exactly as it appears on the match result screen.",
		Football: true, ResultScreen: "full-time result screen", ModeHint: "Friend Match or an online match vs a real player",
		ocrHints: `eFootball (Konami). Valid: Friend Match / online match against another human, on the full-time result screen showing both user names and the final score (with penalty shoot-out score if any).
Reject: matches vs COM/AI, Tour/Campaign events vs AI, pause menus, highlights, and screens without user names.`,
	},
	{
		ID:       "dls",
		Name:     "Dream League Soccer",
		Short:    "DLS",
		NameHint: "Your DLS club name, exactly as it appears on the online match result screen.",
		Football: true, ResultScreen: "full-time result screen", ModeHint: "an Online or Friendly match vs a real player",
		ocrHints: `Dream League Soccer. Valid: Online / Friendly match against another human, on the full-time result screen showing both club names and the final score.
Reject: Career / Season / Cup matches vs AI, pause menus, replays, and screens without both club names.`,
	},
	{
		ID:           "codm",
		Name:         "Call of Duty: Mobile",
		Short:        "CODM",
		NameHint:     "Your Call of Duty: Mobile name, exactly as it shows on the match scoreboard.",
		ResultScreen: "final scoreboard (Victory / Defeat screen)",
		ModeHint:     "a 1v1 private room or Gunfight 1v1 vs a real player",
		ocrHints: `Call of Duty: Mobile (multiplayer). Valid: a finished 1v1 match against another human (private/custom room, or Gunfight 1v1), on the final MATCH RESULT / scoreboard screen (VICTORY or DEFEAT) showing both players' in-game names and the final team score.
Scoring: "score" is each side's final team score as shown at the top of the scoreboard (rounds won in Gunfight, points/kills in Team Deathmatch). There are no penalty shoot-outs: penalty_score must be null. List the side shown first (top or left) as LEFT.
Reject: Battle Royale results or placements, matches vs bots/AI or training, in-match HUD screens, kill-cams, lobby or loading screens, and scoreboards that don't show both players' names.`,
	},
	{
		ID:           "free_fire",
		Name:         "Free Fire",
		Short:        "Free Fire",
		NameHint:     "Your Free Fire nickname, exactly as it shows on the match result screen.",
		ResultScreen: "final result screen (Booyah / Victory / Defeat)",
		ModeHint:     "Lone Wolf 1v1 or a 1v1 custom room vs a real player",
		ocrHints: `Garena Free Fire / Free Fire MAX. Valid: a finished 1v1 match against another human, in Lone Wolf mode or a 1v1 custom room (Clash Squad rules), on the final result screen (BOOYAH! / VICTORY or DEFEAT) showing both players' nicknames and the rounds each player won.
Scoring: "score" is the number of rounds each player won. There are no penalty shoot-outs: penalty_score must be null. List the side shown first (top or left) as LEFT.
Reject: Battle Royale placements (e.g. a "#1 BOOYAH" ranking of squads), training ground or bot matches, screens during a round, lobby screens, and screens without both nicknames.`,
	},
}

// Games returns the supported games in display order.
func Games() []Game { return supportedGames }

// GameByID returns the game with the given id, or nil if unsupported.
func GameByID(id string) *Game {
	for i := range supportedGames {
		if supportedGames[i].ID == id {
			return &supportedGames[i]
		}
	}
	return nil
}
