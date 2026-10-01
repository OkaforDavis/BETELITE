package services

// Game describes a title CrestArena supports for verified matches, plus the
// hints the OCR prompt needs to recognise a valid player-vs-player result.
type Game struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Short    string `json:"short"`
	NameHint string `json:"nameHint"` // shown in the profile form
	ocrHints string
}

var supportedGames = []Game{
	{
		ID:       "fc_mobile",
		Name:     "EA SPORTS FC Mobile",
		Short:    "FC Mobile",
		NameHint: "Your FC Mobile username, exactly as it appears on the Head-to-Head result screen.",
		ocrHints: `EA SPORTS FC Mobile. Valid: Head-to-Head / VS Attack / friendly match against another human, on the FULL TIME result screen showing both usernames and the final score.
Reject: Scenario / Events / Campaign / "TARGET:" objectives, any match vs CPU/AI, the pause menu (RESUME / QUIT buttons, a running clock), replays, and screens that only show national/club team names without usernames.`,
	},
	{
		ID:       "efootball",
		Name:     "eFootball",
		Short:    "eFootball",
		NameHint: "Your eFootball user name, exactly as it appears on the match result screen.",
		ocrHints: `eFootball (Konami). Valid: Friend Match / online match against another human, on the full-time result screen showing both user names and the final score (with penalty shoot-out score if any).
Reject: matches vs COM/AI, Tour/Campaign events vs AI, pause menus, highlights, and screens without user names.`,
	},
	{
		ID:       "dls",
		Name:     "Dream League Soccer",
		Short:    "DLS",
		NameHint: "Your DLS club name, exactly as it appears on the online match result screen.",
		ocrHints: `Dream League Soccer. Valid: Online / Friendly match against another human, on the full-time result screen showing both club names and the final score.
Reject: Career / Season / Cup matches vs AI, pause menus, replays, and screens without both club names.`,
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
