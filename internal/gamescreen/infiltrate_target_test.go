// infiltrate_target_test.go : tests the player side commando pick rules (visible target, picked cell next to the commando).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
)

// infiltrateScreen builds a screen with a selected player commando at (1, 1) and the steal pick armed.
func infiltrateScreen(t *testing.T, scan int) (*Screen, *sim.Game, *game.UnitInfo) {
	t.Helper()

	s, g := newTestScreen(t, 16, 1)
	s.turnFlow = turnFlowState{handover: -1}
	s.fogTeam = 0

	c := raidCommando(g, 0, 1, 1)
	c.Values.Set(game.AttribScan, scan)
	s.selected = c
	s.orderInput.infiltrateArmed = true
	s.orderInput.infiltrateSteal = true

	return s, g, c
}

// TestInfiltrateCursorNeedsVisibleTarget: an adjacent enemy tank in the fog gives no steal cursor and no click.
func TestInfiltrateCursorNeedsVisibleTarget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		scan    int
		fogShow bool
		want    bool
	}{
		{"seen", 3, true, true},
		{"fogged", 0, true, false},
	} {
		s, g, c := infiltrateScreen(t, tc.scan)
		mkTank(g.Units, 1, 7, 2, 1, raidTank)
		g.RefreshVisibility()
		s.fogShow = tc.fogShow

		id, armed := s.cursorForArmedOrder(2, 1)
		if !armed || (id == cursors.Steal) != tc.want {
			t.Errorf("%s: cursor = %v armed=%v, want steal=%v", tc.name, id, armed, tc.want)
		}

		px, py := cellPixel(s, 2, 1)
		s.handleInfiltratePick(platform.MouseState{X: px, Y: py, Buttons: platform.MouseReleaseLeft})

		if spent := c.Shots == 0; spent != tc.want {
			t.Errorf("%s: click spent the shot = %v, want %v", tc.name, spent, tc.want)
		}
	}
}

// TestInfiltrateCursorNeedsAdjacentCell: only a cell of a 2x2 enemy building next to the commando gives the disable cursor.
func TestInfiltrateCursorNeedsAdjacentCell(t *testing.T) {
	s, g, c := infiltrateScreen(t, 3)
	s.orderInput.infiltrateSteal = false
	s.fogShow = false

	b := game.NewUnitInfo(assets.Radar, 1, 8, 0)
	b.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable | game.FlagElectronicUnit
	b.Values = &game.UnitValues{}
	b.Values.Set(game.AttribHits, 30)
	b.InitState()
	g.Units.Deploy(b, 2, 1) // covers (2..3, 1..2)

	if !g.CanDisableAt(c, 3, 2) {
		t.Fatal("the sim must accept the far cell (the building is adjacent), or this test proves nothing")
	}

	if id, _ := s.cursorForArmedOrder(3, 2); id == cursors.Disable {
		t.Error("a far cell of the building (not next to the commando) gave the disable cursor")
	}

	px, py := cellPixel(s, 3, 2)
	s.handleInfiltratePick(platform.MouseState{X: px, Y: py, Buttons: platform.MouseReleaseLeft})

	if c.Shots == 0 {
		t.Error("a click on the far cell infiltrated")
	}

	s.orderInput.infiltrateArmed = true // the far click above cancelled the pick

	if id, _ := s.cursorForArmedOrder(2, 1); id != cursors.Disable {
		t.Errorf("an adjacent cell of the building gave cursor %v, want disable", id)
	}
}

// cellPixel returns a screen pixel inside map cell (cellX, cellY) with the camera at the origin.
func cellPixel(s *Screen, cellX, cellY int) (int, int) {
	z := s.r.Zoom()

	return MainMap.ULX + cellX*z + 1, MainMap.ULY + cellY*z + 1
}
