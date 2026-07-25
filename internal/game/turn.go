// turn.go : manages team sequencing and round completion.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - turn sequencing and team-active logic
package game

import "github.com/suglasp/max-go/internal/assets"

// GamePhase is the high level game loop state (the in game subset of the original's state enum).
//
// C++ ported : enums.hpp - GAME_STATE_*
type GamePhase uint8

const (
	PhaseSiteSelect GamePhase = 7 // GAME_STATE_7_SITE_SELECT
	PhaseInGame     GamePhase = 8 // GAME_STATE_8_IN_GAME
	PhaseEndTurn    GamePhase = 9 // GAME_STATE_9_END_TURN
)

// PlayMode selects turn-based or simultaneous team actions.
//
// C++ ported : enums.hpp - PLAY_MODE_*
type PlayMode uint8

const (
	PlayTurnBased    PlayMode = iota // PLAY_MODE_TURN_BASED
	PlaySimultaneous                 // PLAY_MODE_SIMULTANEOUS_MOVES
)

// playModeFromOptions maps the play_mode value as stored in settings.ini (0 turn based,
// 1 simultaneous moves) onto the loop's PlayMode, defaulting anything else to
// turn based. C++: GameManager_PlayMode = settings->GetNumericValue("play_mode").
func playModeFromOptions(value int) PlayMode {
	if value == int(PlaySimultaneous) {
		return PlaySimultaneous
	}
	return PlayTurnBased
}

// TeamInfo is the per team turn bookkeeping: controller type, knocked out and finished turn flags,
// plus research progress and gold bought upgrades.
//
// C++ ported : units_manager.cpp - CTInfo (subset)
type TeamInfo struct {
	Type TeamType
	// Clan is the team's clan id (ClanRandom or 1..8), carried from StartParams
	// so the AI can pick a strategy suited to its clan. C++: TeamInfo::team_clan.
	Clan         int
	Eliminated   bool
	FinishedTurn bool
	// Research is the team's per topic research progress.
	Research [ResearchTopicCount]ResearchTopic
	/* // note: unused code in Go port - the live gold balance is sim.Game.Gold
	// Gold is the team's spendable gold reserve, drawn down by unit upgrades.
	Gold int
	*/
	// Upgrades is the per type, per attribute gold bought stat offset (nil = never upgraded).
	Upgrades map[assets.UnitType]*[AttribCount]int
}

// active reports whether the team takes turns (present and not knocked out). The fixed alien
// faction is never active: it has no turn, no AI and no economy.
func (t TeamInfo) active() bool {
	return t.Type != TeamNone && t.Type != TeamAlien && !t.Eliminated
}

// GameState holds shared turn-loop state: phase, play mode, teams, and player or active team. Units, map, and economy live in sim.
//
// C++ ported : game_manager.cpp - GameManager_* turn globals
type GameState struct {
	Phase       GamePhase
	PlayMode    PlayMode
	TurnCounter int
	ActiveTeam  int // whose turn it is (turn based)
	PlayerTeam  int // the local human team (rotates in hot seat)
	// Teams is sized to TeamSlots (playable teams + the fixed alien slot) so a unit owned by
	// the alien team has its [u.Team] index in bounds; only indices < TeamCount are ever active/iterated.
	Teams [TeamSlots]TeamInfo
	// Difficulty is the computer opponent tier (0..5 Clueless..God) for this game, or
	// DifficultyUnset when none was chosen. It lives here, on the persisted loop state,
	// so a save carries it and a loaded game restores the AI's strength. The AI reads it
	// on init and applies a real tier. DifficultyUnset keeps the controller's
	// launch defaults, while SetDifficulty records changes made through F2.
	Difficulty int
	// AIWinGoal is each computer team's committed match objective, stored as goal+1
	// (0 = unset, so the AI rolls one on its first turn and writes it back here;
	// internal/ai/wingoal.go winGoal: 1 = kill, 2 = colonise). It lives here, on the
	// persisted loop state, so a save carries the AI's chosen strategy and a reload
	// keeps it instead of re-rolling mid game. Only a computer team's slot is used.
	AIWinGoal [TeamCount]int
	// AIStrategy is each computer team's committed high level strategy, stored as
	// strategy+1 (0 = unset; internal/ai Strategy: 1 = Random, resolved to a concrete
	// one before it is stored). It lives here, on the persisted loop state, so a save
	// carries the AI's chosen posture and a reload keeps it instead of re-rolling from
	// the map. Only a computer team's slot is used. Mirrors AIWinGoal / Difficulty.
	AIStrategy [TeamCount]int
}

// DifficultyUnset marks a game with no explicit AI tier. Its negative value also
// lets saves without a stored tier load as unset rather than CLUELESS.
const DifficultyUnset = -1

// NewGameState seeds the loop state from StartParams: turn 1, in game phase, configured teams, and
// the active/player team set to the first that takes turns.
//
// C++ ported : game_manager.cpp - GameManager_GameSetup
func NewGameState(p *StartParams) *GameState {
	// Reseed the global RNG for each new game. Restoring a save replaces this state with the saved RNG state afterward.
	RandomizerInit()

	gs := &GameState{
		Phase:       PhaseInGame,
		PlayMode:    playModeFromOptions(p.Options.PlayMode),
		TurnCounter: 1,
		// Start without an explicit opponent tier. An F2 change or loaded save can
		// supply one later.
		Difficulty: DifficultyUnset,
	}
	for i := 0; i < TeamCount; i++ {
		gs.Teams[i].Type = p.Teams[i].Type
		gs.Teams[i].Clan = p.Teams[i].Clan
	}
	// The fixed alien slot is always present and never active (no turn, no AI).
	gs.Teams[AlienTeam].Type = TeamAlien
	gs.ActiveTeam = gs.FirstActiveTeam()
	gs.PlayerTeam = gs.ActiveTeam
	return gs
}

// IsActive reports whether team takes turns.
func (gs *GameState) IsActive(team int) bool {
	return team >= 0 && team < TeamCount && gs.Teams[team].active()
}

// FirstActiveTeam returns the lowest numbered active team, or -1 if none.
func (gs *GameState) FirstActiveTeam() int {
	return gs.NextActiveTeam(-1)
}

// NextActiveTeam returns the lowest active team index greater than after, or -1 if none
// (after=-1 yields the first active team).
//
// C++ ported : game_manager.cpp - GameManager_TeamTurnTurnBased (team loop)
func (gs *GameState) NextActiveTeam(after int) int {
	for team := after + 1; team < TeamCount; team++ {
		if gs.Teams[team].active() {
			return team
		}
	}
	return -1
}

// FinishTurn marks team done for this turn and, if it is the active turn based team, hands the
// active slot to the next.
//
// C++ ported : game_manager.cpp - GameManager_RefreshOrders
func (gs *GameState) FinishTurn(team int) {
	if team >= 0 && team < TeamCount {
		gs.Teams[team].FinishedTurn = true
	}
	if gs.PlayMode == PlayTurnBased && team == gs.ActiveTeam {
		if next := gs.NextActiveTeam(team); next != -1 {
			gs.ActiveTeam = next
		}
	}
}

// AllFinished reports whether every active team has finished this turn.
func (gs *GameState) AllFinished() bool {
	any := false
	for team := 0; team < TeamCount; team++ {
		if gs.Teams[team].active() {
			any = true
			if !gs.Teams[team].FinishedTurn {
				return false
			}
		}
	}
	return any
}

// AdvanceTurn rolls to the next turn: bumps the counter, clears finished flags, returns the active
// slot to the first team, and enters the in game phase again.
//
// C++ ported : game_manager.cpp - turn rollover after GameManager_AreTeamsFinishedTurn
func (gs *GameState) AdvanceTurn() {
	gs.TurnCounter++
	for i := range gs.Teams {
		gs.Teams[i].FinishedTurn = false
	}
	gs.ActiveTeam = gs.FirstActiveTeam()
	gs.Phase = PhaseInGame
}
