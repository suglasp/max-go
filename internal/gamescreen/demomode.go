// demomode.go : implements the attract demo playback state.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - GameManager_DemoMode, menu.cpp - menu_check_end_game_conditions
package gamescreen

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// demoTurnLimit caps an attract demo at this many full turns before it returns
// to the main menu. The original reads the ini demo_turns value capped at 5;
// the port has no such setting, so the cap itself is the value.
//
// C++: menu_check_end_game_conditions (menu.cpp)
const demoTurnLimit = 5

// SetDemoMode puts the screen in attract demo playback: every input exits to
// the main menu, the view follows each team's turn, and the demo ends by
// itself after demoTurnLimit turns. Call it after the demo's MissionStart so
// the turn cap measures from the loaded turn counter.
//
// C++: game_manager.cpp - GameManager_DemoMode / turn_counter_session_start
func (s *Screen) SetDemoMode(on bool) {
	s.demoMode = on

	if on && s.game != nil {
		s.demoTurnStart = s.game.State.TurnCounter
	}
}

// demoTickIdle opens the demo's turn cycle: a demo has no human player to act,
// so the moment the board is live the active (computer) team's turn starts
// animating. MissionStart already ran that team's BeginTeamTurn (its AI issued
// orders), so only the animation state is armed; tickComputerTurn then rolls
// team after team through beginNextTeam for the rest of the demo.
func (s *Screen) demoTickIdle() {
	if s.turnFlow.aiTurn || s.turnFlow.endTurnSettling {
		return
	}

	s.turnFlow.aiTurn = true
	s.turnFlow.aiTurnTeam = s.game.State.ActiveTeam
	s.turnFlow.aiTurnStartMS = platform.Ticks()
	s.centerViewOnTeam(s.turnFlow.aiTurnTeam)
}

// demoMouseExit leaves the demo on a FRESH mouse click; merely moving the
// mouse keeps it playing. Any key does the same via the Run loop's key gate.
// Deviation: klei only exits on ESC/SPACE (game_manager.cpp:5349/5377); the
// port exits on any input, the classic attract mode contract (user choice).
func (s *Screen) demoMouseExit() {
	m := s.plat.Mouse()

	if m.Buttons != 0 && s.demoLastButtons == 0 {
		s.exitToMenu = true
	}

	s.demoLastButtons = m.Buttons
}

// centerViewOnTeam scrolls the map to team's base: its first real building
// (lowest ID, ground cover like roads excluded), else its first unit. The
// demo's per turn camera jump; the original restores each team's saved
// camera_position, which the port does not track per team.
//
// C++ ported : game_manager.cpp - GameManager_TeamTurnTurnBased (demo branch)
func (s *Screen) centerViewOnTeam(team int) {
	// Units() returns random map order; pick by flags then lowest ID so the
	// demo camera lands on the same spot every turn.
	var best *game.UnitInfo

	isBuilding := func(u *game.UnitInfo) bool {
		return u.Flags.Has(game.FlagStationary) && !u.Flags.Has(game.FlagGroundCover)
	}

	for _, u := range s.game.Units.Units() {
		if u.Team != team {
			continue
		}

		switch {
		case best == nil:
			best = u

		case isBuilding(u) && !isBuilding(best):
			best = u

		case isBuilding(u) == isBuilding(best) && u.ID < best.ID:
			best = u
		}
	}

	if best == nil {
		return
	}

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(best.GridX, best.GridY, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}
