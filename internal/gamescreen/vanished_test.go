// vanished_test.go : tests forgetVanished (units removed without a death event).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// TestStrandedSelectedTankIsForgotten: a selected, camera locked tank on a bridge that the sim
// destroys vanishes without an event; the screen must drop it from selection and lock.
func TestStrandedSelectedTankIsForgotten(t *testing.T) {
	s, g := newTestScreen(t, 12)
	s.wmap.Surface[5*s.wmap.W+5] = world.SurfaceWater
	bridge := mkUnit(g, assets.Bridge, 0, 2, 5, 5, game.FlagStationary)
	bridge.Hits = 1
	tank := mkTank(g.Units, 0, 1, 5, 5, tankStats{hits: 30})
	other := mkTank(g.Units, 0, 3, 8, 8, tankStats{hits: 30})
	s.selected = tank
	s.selection = []*game.UnitInfo{tank, other}
	s.camera.lockUnit = tank

	g.DestroyUnit(bridge)
	if g.Units.ByID.Get(tank.ID) != nil {
		t.Fatal("fixture: the tank must be stranded by the bridge loss")
	}

	s.forgetVanished()

	if s.selected == tank || s.selected != other {
		t.Errorf("selected = %v, want the surviving group member", s.selected)
	}
	for _, u := range s.selection {
		if u == tank {
			t.Error("the stranded tank must leave the selection")
		}
	}
	if s.camera.lockUnit != nil {
		t.Error("the camera lock must clear")
	}
}

// TestUnaffectedSelectionStays: a live selected unit is left alone.
func TestUnaffectedSelectionStays(t *testing.T) {
	s, g := newTestScreen(t, 12)
	tank := mkTank(g.Units, 0, 1, 5, 5, tankStats{hits: 30})
	s.selected = tank
	s.camera.lockUnit = tank

	s.forgetVanished()

	if s.selected != tank || s.camera.lockUnit != tank {
		t.Error("a live selected unit must stay selected and locked")
	}
}

// TestVanishedLockAndMemberForgotten: a camera lock on an unselected vanished unit, and a
// vanished non leader group member, are dropped on their own.
func TestVanishedLockAndMemberForgotten(t *testing.T) {
	s, g := newTestScreen(t, 12)
	leader := mkTank(g.Units, 0, 1, 2, 2, tankStats{hits: 30})
	member := mkTank(g.Units, 0, 2, 3, 3, tankStats{hits: 30})
	third := mkTank(g.Units, 0, 3, 4, 4, tankStats{hits: 30})
	locked := mkTank(g.Units, 0, 4, 6, 6, tankStats{hits: 30})
	s.selected = leader
	s.selection = []*game.UnitInfo{leader, member, third}
	s.camera.lockUnit = locked
	g.Units.Remove(member)
	g.Units.Remove(locked)

	s.forgetVanished()

	if s.camera.lockUnit != nil {
		t.Error("the lock on a vanished unit must clear")
	}
	for _, u := range s.selection {
		if u == member {
			t.Error("the vanished member must leave the selection")
		}
	}
	if s.selected != leader {
		t.Error("the live leader must stay selected")
	}
}
