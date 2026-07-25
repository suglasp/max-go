// mine_stealth_test.go : tests mine stealth visibility rules on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestEnemyMineHiddenFromView checks the render/click stealth gate: an enemy mine
// the player has not uncovered is stealth hidden (skipped by drawUnits and not
// returned by topUnitAt), uncovered by a mine detector that drives up to it, and
// always shown when the fog overlay is off (a bench / the F2 reveal cheat).
func TestEnemyMineHiddenFromView(t *testing.T) {
	const w, h = 16, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	mine := game.NewUnitInfo(assets.LandMine, 1, 9001, 0) // a team-1 (enemy) laid mine
	mine.Flags = game.FlagStationary
	mine.Values = &game.UnitValues{}
	mine.Values.Set(game.AttribHits, 1)
	mine.Hits = 1
	mgr.Deploy(mine, 8, 4)
	g.RefreshVisibility()

	s := &Screen{game: g, fogShow: true, fogTeam: 0}

	if !s.stealthHidden(mine) {
		t.Fatal("an undetected enemy mine must be stealth-hidden from the player view")
	}
	if got := s.topUnitAt(8, 4); got != nil {
		t.Errorf("topUnitAt returned a hidden enemy mine (%v); want nil", got.Type)
	}

	// Fog overlay off (bench / F2 reveal): everything is shown.
	s.fogShow = false
	if s.stealthHidden(mine) {
		t.Error("with the fog overlay off the mine must be revealed")
	}
	if got := s.topUnitAt(8, 4); got != mine {
		t.Error("with fog off the mine should be selectable again")
	}
	s.fogShow = true

	// Infantry has the scan range but is not a mine detector: the mine stays hidden.
	infantry := game.NewUnitInfo(assets.Infantry, 0, 9002, 0)
	infantry.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	infantry.Values = &game.UnitValues{}
	infantry.Values.Set(game.AttribScan, 3)
	mgr.Deploy(infantry, 8, 4)
	g.RefreshVisibility()

	if !s.stealthHidden(mine) {
		t.Error("infantry standing on a mine must not uncover it; only a detector does")
	}

	mgr.Move(infantry, 1, 1) // off the mine's cell, so the pick below is the mine itself

	// A Surveyor next to the mine uncovers it, and it stays uncovered after the
	// surveyor drives away.
	surveyor := game.NewUnitInfo(assets.Surveyor, 0, 9003, 0)
	surveyor.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	surveyor.Values = &game.UnitValues{}
	surveyor.Values.Set(game.AttribScan, 3)
	mgr.Deploy(surveyor, 7, 4)
	g.RefreshVisibility()

	if s.stealthHidden(mine) {
		t.Error("a surveyor beside the mine must uncover it")
	}

	if got := s.topUnitAt(8, 4); got != mine {
		t.Error("an uncovered mine must be selectable")
	}

	mgr.Move(surveyor, 0, 0)
	g.RefreshVisibility()

	if s.stealthHidden(mine) {
		t.Error("an uncovered mine must stay uncovered after the detector leaves")
	}
}
