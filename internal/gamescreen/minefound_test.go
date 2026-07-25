// minefound_test.go : pins what the player is told about mines: found ones are announced, and a
// move onto one they already know about is flagged before they drive into it.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// mineAt lays an enemy mine, the way the sim does (the retail LANDMINE row).
func mineAt(g *sim.Game, team, id, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.LandMine, team, id, 0)
	u.Flags = game.FlagStationary
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, 28)
	u.Values.Set(game.AttribHits, 2)
	u.InitState()
	g.Units.Deploy(u, gx, gy)

	return u
}

// lastLogged is the newest line in the screen's message log.
func lastLogged(s *Screen) string {
	if len(s.banner.msgLog) == 0 {
		return ""
	}

	return s.banner.msgLog[len(s.banner.msgLog)-1].text
}

// TestSurveyorFindingAMineAnnouncesIt: a mine is a small dark sprite that appears at the edge
// of a driving surveyor. The player is told, so it is not missed and then driven into.
func TestSurveyorFindingAMineAnnouncesIt(t *testing.T) {
	s, g := newHaltScreen(t)
	g.State.ActiveTeam = 0

	surveyor := game.NewUnitInfo(assets.Surveyor, 0, 1, 2)
	surveyor.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	surveyor.Values = &game.UnitValues{}
	surveyor.Values.Set(game.AttribScan, 4)
	surveyor.Values.Set(game.AttribHits, 20)
	surveyor.InitState()
	g.Units.Deploy(surveyor, 10, 10)

	mine := mineAt(g, 1, 2, 11, 10) // the computer's mine, right beside it

	g.RefreshVisibility() // the detector sweep runs from here
	if !mine.SpottedByTeam(0) {
		t.Fatal("setup: the surveyor should have uncovered the mine")
	}

	s.tickMinesFound()

	if got := lastLogged(s); !strings.Contains(got, "mine") {
		t.Errorf("the player was told nothing about the mine their surveyor found (log: %q)", got)
	}
}

// TestMoveOntoAKnownMineWarns: ordering a unit onto a mine the player has already found says
// so. It does NOT block the order - spending a cheap unit to clear a mine is a real tactic.
func TestMoveOntoAKnownMineWarns(t *testing.T) {
	s, g := newHaltScreen(t)
	g.State.ActiveTeam = 0

	tank := haltMover(g, 0, 1, 5, 5, 10, 3)
	mine := mineAt(g, 1, 2, 9, 5)
	mine.SpotBy(0) // the player has found this one

	g.BeginTeamTurn(0)
	s.executeMove(tank, mine.GridX, mine.GridY)

	if got := lastLogged(s); !strings.Contains(strings.ToLower(got), "mine") {
		t.Errorf("no warning for a move onto a known mine (log: %q)", got)
	}
	if g.ActiveOrders() != 1 {
		t.Error("the warning must not block the order: the player is allowed to spend the unit")
	}
}

// TestMoveOntoAnUNKNOWNMineIsSilent: the warning is not omniscient. A mine the player has
// never found must not be given away by a message.
func TestMoveOntoAnUNKNOWNMineIsSilent(t *testing.T) {
	s, g := newHaltScreen(t)
	g.State.ActiveTeam = 0

	tank := haltMover(g, 0, 1, 5, 5, 10, 3)
	mine := mineAt(g, 1, 2, 9, 5) // never detected

	g.BeginTeamTurn(0)
	s.executeMove(tank, mine.GridX, mine.GridY)

	if got := lastLogged(s); strings.Contains(strings.ToLower(got), "mine") {
		t.Errorf("a mine the player never found was announced (log: %q): that is free intelligence", got)
	}
}
