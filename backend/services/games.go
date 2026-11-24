package services

// Game describes a title CrestArena supports for verified matches, plus the
// hints the OCR prompt needs to recognise a valid player-vs-player result.
type Game struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Short        string        `json:"short"`
	NameHint     string        `json:"nameHint"`           // shown in the profile form
	IDHint       string        `json:"idHint"`             // where to find the in-game ID number
	Football     bool          `json:"football"`           // draws in knockouts go to penalties
	ResultScreen string        `json:"resultScreen"`       // what to screenshot, in plain words
	ModeHint     string        `json:"modeHint"`           // which game mode counts
	Room         RoomSpec      `json:"room"`               // how the two players meet in the game
	Settings     *SettingsSpec `json:"settings,omitempty"` // room settings players pick (CODM)
	FFA          bool          `json:"ffa,omitempty"`      // can run Free-for-All cups (everyone in one room)
	ocrHints     string
}

// RoomSpec says how two players get into the same game room.
//
//   - Mode "host": one player creates a room in the game and posts its number
//     (and password or join link) on the match page; the other player joins.
//   - Mode "shared": CrestArena makes up the code and both players type it.
//
// Host "home" means the first-named player hosts and only they can post the
// room. Host "either" means whoever posts first becomes the host (Free Fire:
// only players with a Custom Room Card can create a room).
type RoomSpec struct {
	Mode      string   `json:"mode"`
	Host      string   `json:"host,omitempty"`
	CodeLabel string   `json:"codeLabel"`
	Password  bool     `json:"password,omitempty"`
	Link      bool     `json:"link,omitempty"` // the host may paste the game's join link
	HostSteps []string `json:"hostSteps"`      // for the host, or for both in "shared" mode
	JoinSteps []string `json:"joinSteps,omitempty"`
	Warning   string   `json:"warning,omitempty"`
}

var supportedGames = []Game{
	{
		ID:       "fc_mobile",
		Name:     "EA SPORTS FC Mobile",
		Short:    "FC Mobile",
		NameHint: "Your FC Mobile username, exactly as it appears on the Head-to-Head result screen.",
		IDHint:   "Optional. Lets your opponent find you in the game's Friends list.",
		Football: true, ResultScreen: "full-time result screen", ModeHint: "Quick Match, Head-to-Head or VS Attack vs a real player",
		Room: RoomSpec{
			Mode: "host", Host: "home", CodeLabel: "Room code", Link: true,
			HostSteps: []string{
				"Open FC Mobile and go to Play → Division Rivals → Quick Match.",
				"Create a lobby. Pick the mode your match uses (Head to Head unless the cup says VS Attack).",
				"Copy the room code (or the invite link) and paste it below.",
				"Wait in the lobby until your opponent joins, then start.",
			},
			JoinSteps: []string{
				"Wait for your opponent to post the room code here. You'll get a notification.",
				"Open FC Mobile and go to Play → Division Rivals → Quick Match.",
				"Join with the code (or tap the link below).",
			},
			Warning: "Both of you must use the same mode. Draws in a cup go to extra time and penalties, so turn those on.",
		},
		ocrHints: `EA SPORTS FC Mobile. Valid: Quick Match / Head-to-Head / VS Attack / friendly match against another human, on the FULL TIME result screen showing both usernames and the final score.
Reject: Scenario / Events / Campaign / "TARGET:" objectives, any match vs CPU/AI, the pause menu (RESUME / QUIT buttons, a running clock), replays, and screens that only show national/club team names without usernames.`,
	},
	{
		ID:       "efootball",
		Name:     "eFootball",
		Short:    "eFootball",
		NameHint: "Your eFootball user name, exactly as it appears on the match result screen.",
		IDHint:   "Optional. Your eFootball User ID, so your opponent can add you as a friend.",
		Football: true, ResultScreen: "full-time result screen", ModeHint: "Friend Match vs a real player",
		Room: RoomSpec{
			Mode: "host", Host: "home", CodeLabel: "Match Room No.",
			HostSteps: []string{
				"Open eFootball and go to Match → Friend Match.",
				"Tap Create 1 vs 1 Match Room.",
				"Type the Match Room No. it shows into the box below.",
				"Stay in the room until your opponent joins, then start.",
			},
			JoinSteps: []string{
				"Wait for your opponent to post the Match Room No. here. You'll get a notification.",
				"Open eFootball and go to Match → Friend Match.",
				"Tap Search Match Room, enter the number and join.",
			},
		},
		ocrHints: `eFootball (Konami). Valid: Friend Match / online match against another human, on the full-time result screen showing both user names and the final score (with penalty shoot-out score if any).
Reject: matches vs COM/AI, Tour/Campaign events vs AI, pause menus, highlights, and screens without user names.`,
	},
	{
		ID:       "dls",
		Name:     "Dream League Soccer",
		Short:    "DLS",
		NameHint: "Your DLS club name, exactly as it appears on the online match result screen.",
		IDHint:   "Optional. Your DLS friend code, if you have one.",
		Football: true, ResultScreen: "full-time result screen", ModeHint: "a Friend Match vs a real player",
		Room: RoomSpec{
			Mode: "shared", CodeLabel: "Friend Match code",
			HostSteps: []string{
				"Copy the code below. It's the same for both of you.",
				"Open DLS and go to Friend Match.",
				"Type the code and tap OK. When you both have, the game connects you.",
			},
			Warning: "Friend Match unlocks after 6 Career matches. Play those first if you haven't.",
		},
		ocrHints: `Dream League Soccer. Valid: Online / Friendly match against another human, on the full-time result screen showing both club names and the final score.
Reject: Career / Season / Cup matches vs AI, pause menus, replays, and screens without both club names.`,
	},
	{
		ID:           "codm",
		Name:         "Call of Duty: Mobile",
		Short:        "CODM",
		NameHint:     "Your Call of Duty: Mobile name, exactly as it shows on the match scoreboard.",
		IDHint:       "Optional. Your CODM UID, so your opponent can invite you.",
		ResultScreen: "final scoreboard (Victory / Defeat screen)",
		ModeHint:     "a 1v1 private room or Gunfight 1v1 vs a real player",
		Settings:     codmSettings,
		FFA:          true,
		Room: RoomSpec{
			Mode: "host", Host: "home", CodeLabel: "Room ID", Password: true,
			HostSteps: []string{
				"Open CODM and go to Multiplayer → the ≡ menu (top right) → Private.",
				"Set a password, and choose a 1v1 mode in the mode settings (bottom right).",
				"Type the Room ID and password below. Don't leave the room: the ID changes if you do.",
				"When your opponent joins, tap Start.",
			},
			JoinSteps: []string{
				"Wait for your opponent to post the Room ID here. You'll get a notification.",
				"Open CODM and go to Multiplayer → ≡ → Private → the home icon.",
				"Search the Room ID, enter the password and join.",
			},
		},
		ocrHints: `Call of Duty: Mobile (multiplayer). Valid: a finished 1v1 match against another human (private/custom room, or Gunfight 1v1), on the final MATCH RESULT / scoreboard screen (VICTORY or DEFEAT) showing both players' in-game names and the final team score.
Scoring: "score" is each side's final team score as shown at the top of the scoreboard (rounds won in Gunfight, points/kills in Team Deathmatch). There are no penalty shoot-outs: penalty_score must be null. List the side shown first (top or left) as LEFT.
Reject: Battle Royale results or placements, matches vs bots/AI or training, in-match HUD screens, kill-cams, lobby or loading screens, and scoreboards that don't show both players' names.`,
	},
	{
		ID:           "free_fire",
		Name:         "Free Fire",
		Short:        "Free Fire",
		NameHint:     "Your Free Fire nickname, exactly as it shows on the match result screen.",
		IDHint:       "Optional. Your Free Fire UID, so your opponent can add you.",
		ResultScreen: "final result screen (Booyah / Victory / Defeat)",
		ModeHint:     "a 1v1 custom room (Lone Wolf) vs a real player",
		Room: RoomSpec{
			Mode: "host", Host: "either", CodeLabel: "Room ID", Password: true,
			HostSteps: []string{
				"Creating a room needs a Custom Room Card. If you have one, you host.",
				"Open Free Fire and go to Custom → Create. Choose Lone Wolf (1v1) and set a password.",
				"Type the Room ID and password below. The first player to post becomes the host.",
				"When your opponent joins, start the match.",
			},
			JoinSteps: []string{
				"Your opponent is hosting. Open Free Fire and go to Custom → Join.",
				"Enter the Room ID and password shown below.",
			},
			Warning: "One of you needs a Custom Room Card (from events, or bought in the game). If neither of you has one, report the match before the time runs out.",
		},
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
