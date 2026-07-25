// planningbulb_test.go : tests planningbulb.go.
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

// TestPlanningBulbArmsForSelectedUnitOnly: armPlanningBulb arms only for the player's
// currently selected unit; a different unit never arms it.
func TestPlanningBulbArmsForSelectedUnitOnly(t *testing.T) {
	sel := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	other := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	s := &Screen{}
	s.selected = sel

	s.armPlanningBulb(sel, 5, 5)
	if s.effects.bulbUnit != sel {
		t.Fatalf("armPlanningBulb should arm the bulb for the selected unit, got %v", s.effects.bulbUnit)
	}

	// A unit that is not the selection never arms the bulb.
	s.effects.bulbUnit = nil
	s.armPlanningBulb(other, 5, 5)
	if s.effects.bulbUnit != nil {
		t.Error("armPlanningBulb must only arm for the player's selected unit")
	}
}

// TestPlanningBulbDefersMoveUntilDwellElapses: ordering the selected unit to move shows
// the bulb and HOLDS the unit still; the move starts only after the dwell (vanilla:
// bulb appears, unit thinks, bulb gone, then it moves).
func TestPlanningBulbDefersMoveUntilDwellElapses(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := clickTestUnit(0, 1, 5, 5, mgr)
	s := &Screen{game: g}
	s.selected = tank

	s.orderMoveTo(tank, 9, 5)
	// The bulb is armed and the move is deferred: the unit has NOT started moving yet.
	if s.effects.bulbUnit != tank {
		t.Fatal("ordering the selected unit to move should arm the planning bulb")
	}
	if g.HasOrder(tank) {
		t.Error("the move must be deferred behind the bulb, not started immediately")
	}

	// The dwell elapses: tickPlanningBulb clears the bulb and starts the move.
	s.effects.bulbDue = 0
	s.tickPlanningBulb()
	if s.effects.bulbUnit != nil {
		t.Error("tickPlanningBulb should clear the bulb once the dwell elapses")
	}
	if !g.HasOrder(tank) {
		t.Error("the deferred move should start once the bulb clears")
	}
}

// TestPlanningBulbDeadUnitDropsMove: a unit that dies during the dwell drops its
// deferred move (you cannot move a dead unit).
func TestPlanningBulbDeadUnitDropsMove(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := clickTestUnit(0, 1, 5, 5, mgr)
	s := &Screen{game: g}
	s.selected = tank
	s.orderMoveTo(tank, 9, 5)

	mgr.ByID.Remove(tank) // destroyed/removed during the dwell
	s.effects.bulbDue = 0
	s.tickPlanningBulb()
	if s.effects.bulbUnit != nil {
		t.Error("tickPlanningBulb should clear the bulb for a removed unit")
	}
	if g.HasOrder(tank) {
		t.Error("a unit removed during the dwell must not start its deferred move")
	}
}

// TestPlanningBulbCancelledByTurnCommit: a move still deferred behind the bulb when
// the turn commits belongs to the OLD turn - the commit drops it, so the stale order
// cannot execute in the next turn and silently spend the fresh budget.
func TestPlanningBulbCancelledByTurnCommit(t *testing.T) {
	s, g := simTestScreen()

	tank := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribSpeed, 8)
	tank.InitState()
	g.Units.Deploy(tank, 5, 5)

	s.beginSimRound()
	s.selected = tank
	s.orderMoveTo(tank, 9, 5)
	if s.effects.bulbUnit != tank {
		t.Fatal("test setup: the move should be deferred behind the bulb")
	}

	s.commitTurn() // End Turn inside the dwell window

	if s.effects.bulbUnit != nil {
		t.Error("commitTurn must cancel the pending bulb move")
	}
	s.effects.bulbDue = 0
	s.tickPlanningBulb()
	if g.HasOrder(tank) {
		t.Error("the stale deferred move must not execute in the new turn")
	}
}

// TestPlanningBulbCancelDropsPendingMove: cancelPlanningBulb drops the pending move so a
// later tick does not fire it (used when the player issues a different order mid dwell).
func TestPlanningBulbCancelDropsPendingMove(t *testing.T) {
	sel := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	s := &Screen{}
	s.selected = sel
	s.armPlanningBulb(sel, 5, 5)
	s.cancelPlanningBulb()
	if s.effects.bulbUnit != nil {
		t.Error("cancelPlanningBulb should drop the pending bulb/move")
	}
}
