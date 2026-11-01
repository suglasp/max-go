// demomode_test.go : tests the attract demo playback state (turn cap, view follow, turn kick).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// demoTestScreen builds a headless attract demo board: two computer teams, no
// human, turn based, with a live map renderer (the demo camera jump needs it).
func demoTestScreen() (*Screen, *sim.Game) {
	m := &world.Map{Surface: make([]uint8, 16*16)}
	m.W, m.H = 16, 16
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamComputer
	p.Teams[1].Type = game.TeamComputer
	g := sim.New(p, m)

	r := drawmap.New(m)
	r.SetViewport(viewSize(MainMap))
	r.SetZoom(16)

	s := &Screen{game: g, wmap: m, r: r, turnFlow: turnFlowState{handover: -1, handBackTeam: -1}, debugMenu: debugMenuState{endless: true}}
	s.SetPlayMode(int(game.PlayTurnBased))
	s.SetDemoMode(true)

	return s, g
}

// TestDemoTurnCapExitsToMenu verifies the demo's self stop: once the turn counter
// has advanced demoTurnLimit turns past the demo's start, beginNextTeam exits
// to the menu instead of opening another team turn.
// C++: menu_check_end_game_conditions (menu.cpp, demo_turns capped at 5).
func TestDemoTurnCapExitsToMenu(t *testing.T) {
	s, g := demoTestScreen()

	// One turn before the cap: the demo keeps playing.
	g.State.TurnCounter = s.demoTurnStart + demoTurnLimit - 1
	s.beginNextTeam()
	if s.exitToMenu {
		t.Fatal("the demo must keep playing before the turn cap")
	}
	if !s.turnFlow.aiTurn {
		t.Fatal("beginNextTeam on a computer team should arm the animated AI turn")
	}

	// At the cap: back to the main menu, no new team turn opens.
	s.turnFlow.aiTurn = false
	g.State.TurnCounter = s.demoTurnStart + demoTurnLimit
	s.beginNextTeam()
	if !s.exitToMenu {
		t.Error("the demo should exit to the menu once the turn cap is reached")
	}
	if s.turnFlow.aiTurn {
		t.Error("no team turn should open past the demo turn cap")
	}
}

// TestDemoViewFollowsActiveTeam verifies the demo camera:
// each demo turn moves the view AND the fog to the team whose turn plays.
// C++: game_manager.cpp - GameManager_PlayerTeam = team in demo mode.
func TestDemoViewFollowsActiveTeam(t *testing.T) {
	s, g := demoTestScreen()

	// Give team 1 a building so the camera jump has an anchor.
	depot := game.NewUnitInfo(assets.Depot, 1, 1, 0)
	depot.Flags = game.FlagStationary | game.FlagSelectable
	depot.Values = &game.UnitValues{}
	g.Units.Deploy(depot, 12, 12)

	g.State.ActiveTeam = 1
	g.State.PlayerTeam = 0
	s.beginNextTeam()

	if g.State.PlayerTeam != 1 {
		t.Errorf("demo view team = %d, want the active team 1", g.State.PlayerTeam)
	}
}

// TestDemoTickIdleArmsComputerTurn verifies the demo's turn kick: no human ever
// opens the cycle, so the idle tick arms the active computer team's animation
// (MissionStart already ran its BeginTeamTurn), and leaves an armed turn alone.
func TestDemoTickIdleArmsComputerTurn(t *testing.T) {
	s, g := demoTestScreen()

	g.State.ActiveTeam = 0
	s.demoTickIdle()

	if !s.turnFlow.aiTurn || s.turnFlow.aiTurnTeam != 0 {
		t.Fatalf("demoTickIdle should arm team 0's AI turn (aiTurn=%v team=%d)", s.turnFlow.aiTurn, s.turnFlow.aiTurnTeam)
	}

	// Already armed: a second tick must not restart or retarget the turn.
	s.turnFlow.aiTurnTeam = 1
	s.demoTickIdle()
	if s.turnFlow.aiTurnTeam != 1 {
		t.Error("demoTickIdle must leave an in progress AI turn untouched")
	}
}

// TestDemoModeOffLeavesTurnsUncapped verifies the flag gating: a normal game
// (demo mode off) never trips the demo exit, whatever the turn counter reads.
func TestDemoModeOffLeavesTurnsUncapped(t *testing.T) {
	s, g := demoTestScreen()
	s.SetDemoMode(false)

	g.State.TurnCounter = demoTurnLimit * 10
	s.beginNextTeam()

	if s.exitToMenu {
		t.Error("a normal game must not exit on the demo turn cap")
	}
}
