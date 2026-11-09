// mine_kill_select_test.go : tests selection and camera lock cleanup after a mine kill.
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

// TestMineKillClearsSelection verifies that a selected
// unit that drove onto a hidden enemy mine and was destroyed kept showing in the
// panel (0 hits + a phantom move bulb, still "clickable"). The death event drain
// destroys through the sim's DestroyUnit, so the screen must drop its dangling
// selection / camera lock / portrait pointer when it processes the mine blast.
func TestMineKillClearsSelection(t *testing.T) {
	const w, h = 16, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.State.PlayerTeam = 0

	// A team-1 (enemy) mine sitting on (8,4).
	mine := game.NewUnitInfo(assets.LandMine, 1, 9001, 0)
	mine.Flags = game.FlagStationary
	mine.Values = &game.UnitValues{}
	mine.Values.Set(game.AttribAttack, 24)
	mine.Values.Set(game.AttribHits, 1)
	mine.Hits = 1
	mgr.Deploy(mine, 8, 4)

	// The player's own weak unit on the mine cell, currently selected + locked.
	victim := game.NewUnitInfo(assets.Scout, 0, 9002, 0)
	victim.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	victim.Values = &game.UnitValues{}
	victim.Values.Set(game.AttribHits, 8) // < mineAttackDamage(24) - armor: a one hit kill
	victim.Hits = 8
	mgr.Deploy(victim, 8, 4)

	s := &Screen{game: g, fogShow: true, fogTeam: 0}
	s.selected = victim
	s.camera.lockUnit = victim
	s.portraitUI.portraitUnit = victim

	g.Tick() // runs detonateMines: the mine kills the victim and queues a destroyed MineBlast
	if g.Units.ByID.Get(victim.ID) != nil {
		t.Fatal("victim should have been destroyed by the mine")
	}

	s.tickMineBlasts() // drains the blast; must drop the dangling selection
	if s.selected != nil {
		t.Error("selection still points at the mine-killed unit (panel ghost)")
	}
	if s.camera.lockUnit != nil {
		t.Error("camera lock still points at the destroyed unit")
	}

	// The portrait FLC is torn down on the next updatePortrait: selection is now nil
	// and no longer matches the stale portraitUnit, so the player clears (the FLI must
	// not keep playing the dead unit's portrait). forgetDestroyed deliberately leaves
	// portraitUnit alone so this reconcile actually fires (see its comment).
	s.updatePortrait()
	if s.portraitUI.portraitUnit != nil {
		t.Error("portrait still points at the destroyed unit after reconcile")
	}
	if s.portraitUI.portrait != nil {
		t.Error("portrait FLC player still running for the destroyed unit")
	}
}
