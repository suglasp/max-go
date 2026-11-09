// inputseams_test.go : tests the click routing seams handleMouse and clickCell delegate to.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// seamScreen builds a headless screen on a small all land map with the turn live.
// handover stays at its zero value in a bare Screen, which reads as "team 0 is still
// handing over" and would gate every pick below, so the helper sets it to -1.
func seamScreen(t *testing.T) (*Screen, *units.Manager) {
	t.Helper()

	const w, h = 24, 24

	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer

	s := &Screen{game: sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)}
	s.turnFlow.handover = -1

	return s, mgr
}

// TestArmedPickRightClickCancels checks the contract every armed order pick shares:
// it ignores the click while nothing is armed, and once armed a right click cancels
// the order and reports the click consumed, so handleMouse stops there.
func TestArmedPickRightClickCancels(t *testing.T) {
	right := platform.MouseState{Buttons: platform.MouseReleaseRight}

	cases := []struct {
		name  string
		arm   func(*Screen)
		pick  func(*Screen, platform.MouseState) bool
		armed func(*Screen) bool
	}{
		{"xfer", func(s *Screen) { s.orderInput.xferArmed = true }, (*Screen).handleXferPick,
			func(s *Screen) bool { return s.orderInput.xferArmed }},
		{"attack", func(s *Screen) { s.orderInput.attackArmed = true }, (*Screen).handleAttackPick,
			func(s *Screen) bool { return s.orderInput.attackArmed }},
		{"infiltrate", func(s *Screen) { s.orderInput.infiltrateArmed = true }, (*Screen).handleInfiltratePick,
			func(s *Screen) bool { return s.orderInput.infiltrateArmed }},
		{"reload", func(s *Screen) { s.orderInput.reloadArmed = true }, (*Screen).handleReloadPick,
			func(s *Screen) bool { return s.orderInput.reloadArmed }},
		{"repair", func(s *Screen) { s.orderInput.repairArmed = true }, (*Screen).handleRepairPick,
			func(s *Screen) bool { return s.orderInput.repairArmed }},
		{"transport", func(s *Screen) { s.orderInput.loadArmed = true }, (*Screen).handleTransportPick,
			func(s *Screen) bool { return s.transportArmed() }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := seamScreen(t)

			if c.pick(s, right) {
				t.Fatal("consumed the click while nothing was armed")
			}

			c.arm(s)

			if !c.pick(s, right) {
				t.Fatal("armed pick did not consume the right click")
			}

			if c.armed(s) {
				t.Error("the right click did not disarm the pick")
			}
		})
	}
}

// TestClickCombatConsumesAnAttackClick: clickCombat fires on an attackable enemy in
// range and reports the click consumed, keeping the attacker selected; a click on
// empty ground is not its business and falls through to select or move.
func TestClickCombatConsumesAnAttackClick(t *testing.T) {
	s, mgr := seamScreen(t)
	tank := mkTank(mgr, 0, 1, 5, 5, clickTank)
	enemy := mkTank(mgr, 1, 2, 7, 5, clickTank) // two cells away, inside range 4
	s.selected = tank
	before := enemy.Hits

	if !s.clickCombat(7, 5) {
		t.Fatal("clickCombat did not consume the click on an in range enemy")
	}

	if enemy.Hits >= before {
		t.Errorf("enemy hits %d, want fewer than %d", enemy.Hits, before)
	}

	if s.selected != tank {
		t.Error("firing moved the selection off the attacker")
	}

	if s.clickCombat(5, 9) {
		t.Error("clickCombat consumed a click on empty ground")
	}
}

// TestResolveOccupantSelectPicksThenCycles: an empty cell is none of its business,
// an occupied one selects and reports the click resolved, and clicking the cell the
// selection already sits on advances to the next stacked occupant.
func TestResolveOccupantSelectPicksThenCycles(t *testing.T) {
	s, mgr := seamScreen(t)
	mkTank(mgr, 0, 1, 6, 6, clickTank)
	mkTank(mgr, 0, 2, 6, 6, clickTank) // stacked on the same cell

	if s.resolveOccupantSelect(9, 9) {
		t.Error("consumed a click on an empty cell")
	}

	if !s.resolveOccupantSelect(6, 6) {
		t.Fatal("did not resolve a click on an occupied cell")
	}

	first := s.selected
	if first == nil {
		t.Fatal("resolved the click without selecting anything")
	}

	if !s.resolveOccupantSelect(6, 6) {
		t.Fatal("did not resolve the repeat click")
	}

	if s.selected == first {
		t.Errorf("a repeat click stayed on unit %d instead of cycling to the other occupant", first.ID)
	}
}
