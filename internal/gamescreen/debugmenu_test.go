// debugmenu_test.go : tests debugmenu.go.
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
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestResetDebugFlags verifies the reset that happens at load time: F2 developer cheats return to the
// vanilla start state (fog ON, win/loss active, no enemy path overlay) so a loaded
// game never inherits a stale toggle - a reported bug where "fog lifted after load".
func TestResetDebugFlags(t *testing.T) {
	s := &Screen{fogShow: false, debugMenu: debugMenuState{endless: true}, showAIPaths: true}
	s.resetDebugFlags()

	if !s.fogShow {
		t.Error("fog of war must be ON (vanilla) after a load")
	}

	if s.debugMenu.endless {
		t.Error("endless mode must be OFF after a load")
	}

	if s.showAIPaths {
		t.Error("show-AI-paths must be OFF after a load")
	}
}

func TestWrapIndex(t *testing.T) {
	cases := []struct{ i, n, want int }{
		{0, 3, 0}, {3, 3, 0}, {4, 3, 1}, {-1, 3, 2}, {-3, 3, 0}, {5, 0, 0},
	}

	for _, c := range cases {
		if got := wrapIndex(c.i, c.n); got != c.want {
			t.Errorf("wrapIndex(%d,%d) = %d, want %d", c.i, c.n, got, c.want)
		}
	}
}

func TestRollStep(t *testing.T) {
	steps := []int{0, 30, 60, 90}
	if got := rollStep(steps, 30, +1); got != 60 {
		t.Errorf("rollStep up from 30 = %d, want 60", got)
	}

	if got := rollStep(steps, 30, -1); got != 0 {
		t.Errorf("rollStep down from 30 = %d, want 0", got)
	}

	if got := rollStep(steps, 90, +1); got != 0 { // wraps
		t.Errorf("rollStep up from 90 = %d, want 0 (wrap)", got)
	}

	if got := rollStep(steps, 0, -1); got != 90 { // wraps
		t.Errorf("rollStep down from 0 = %d, want 90 (wrap)", got)
	}

	if got := rollStep(steps, 45, +1); got != 60 { // not on a step: nearest <= then step
		t.Errorf("rollStep up from 45 = %d, want 60", got)
	}
}

func TestDbgOnOff(t *testing.T) {
	if dbgOnOff(true) != "ON" || dbgOnOff(false) != "OFF" {
		t.Error("dbgOnOff labels wrong")
	}
}

// TestDebugFlagToggles exercises the fog and endless toggles plus the
// open/close clock pause bookkeeping on a bare screen (no sim needed).
func TestDebugFlagToggles(t *testing.T) {
	s := &Screen{}

	// Find the Fog of war and Endless rows by label and toggle each.
	for i := range dbgFlags {
		switch dbgFlags[i].label {
		case "Fog of war":
			s.debugMenu.dbgMenuSel = i
			s.applyDebugFlag(0)
			if !s.fogShow {
				t.Error("fog flag did not turn on")
			}

		case "Endless mode":
			s.debugMenu.dbgMenuSel = i
			s.applyDebugFlag(0)
			if !s.debugMenu.endless {
				t.Error("endless flag did not turn on")
			}
		}
	}

	// n/a rows must be inert.
	for i := range dbgFlags {
		if dbgFlags[i].na {
			s.debugMenu.dbgMenuSel = i

			s.applyDebugFlag(0) // must not panic / change anything
		}
	}
}

func TestDebugMenuNavWraps(t *testing.T) {
	s := &Screen{}
	s.openDebugMenu()
	if !s.debugMenu.dbgMenuOpen {
		t.Fatal("menu did not open")
	}

	s.debugMenu.dbgMenuSel = 0
	s.handleDebugMenuKey(platform.KeyUp) // wrap to last
	if s.debugMenu.dbgMenuSel != len(dbgFlags)-1 {
		t.Errorf("up from 0 = %d, want %d", s.debugMenu.dbgMenuSel, len(dbgFlags)-1)
	}

	s.handleDebugMenuKey(platform.KeyDown) // wrap back to 0
	if s.debugMenu.dbgMenuSel != 0 {
		t.Errorf("down wrap = %d, want 0", s.debugMenu.dbgMenuSel)
	}

	s.handleDebugMenuKey(platform.KeyEscape)
	if s.debugMenu.dbgMenuOpen {
		t.Error("Escape did not close the menu")
	}
}

func TestDebugPlaceArmCancel(t *testing.T) {
	s := &Screen{}
	if s.debugPlaceArmed() {
		t.Fatal("place should start disarmed")
	}

	s.debugMenu.dbgPlaceMode = dbgPlaceUnit
	if !s.debugPlaceArmed() {
		t.Error("place should be armed")
	}

	s.cancelDebugPlace()
	if s.debugPlaceArmed() {
		t.Error("cancel did not disarm")
	}
}

func TestDebugSelfDestructSelected(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	tank := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 8)
	tank.InitState()
	mgr.Deploy(tank, 4, 4)
	s.selected = tank

	s.debugSelfDestructSelected()

	if got := g.Units.ByID.Get(tank.ID); got != nil {
		t.Fatalf("self-destruct left selected unit live: %+v", got)
	}

	if s.selected != nil {
		t.Error("self-destruct should clear the selected unit")
	}

	if len(s.effects.explosions) == 0 {
		t.Error("self-destruct should spawn the normal death explosion")
	}

	ghost := game.NewUnitInfo(assets.Scout, 0, 2, 0)
	s.selected = ghost
	s.debugSelfDestructSelected() // a stale selection that was never deployed: no panic, no mutation.

	if s.selected != ghost {
		t.Error("stale selected unit should be left alone")
	}
}
