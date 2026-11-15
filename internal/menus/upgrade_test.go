// upgrade_test.go : tests the Upgrades menu category plates and their filter state.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

// TestFilterUnitsDefaults checks the default filter lists ground, air, sea and building units, and that the
// combat plate keeps only the units with an attack.
func TestFilterUnitsDefaults(t *testing.T) {
	units := []*game.UpgradeUnit{
		{Ground: true, Combat: true},
		{Air: true},
		{Sea: true},
		{Building: true, Combat: true},
	}
	def := upgradeFilter{ground: true, air: true, sea: true, building: true}
	if got := len(filterUnits(units, def)); got != 4 {
		t.Errorf("default filter lists %d units, want 4", got)
	}
	def.combat = true
	if got := len(filterUnits(units, def)); got != 2 {
		t.Errorf("combat filter lists %d units, want 2", got)
	}
	def.combat, def.air = false, false
	if got := len(filterUnits(units, def)); got != 3 {
		t.Errorf("air off lists %d units, want 3", got)
	}
}

// TestUpgradeFilterStateDefault checks the package level filter state starts on the reference defaults.
func TestUpgradeFilterStateDefault(t *testing.T) {
	want := upgradeFilter{ground: true, air: true, sea: true, building: true, combat: false}
	if upgradeFilterState != want {
		t.Errorf("upgradeFilterState = %+v, want %+v", upgradeFilterState, want)
	}
}

// newTestUpgradeMenu builds an upgradeMenu on a headless gnw System, with just the filter plates created.
func newTestUpgradeMenu(t *testing.T) (*upgradeMenu, gnw.WinID) {
	t.Helper()
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	a, err := assets.Load(lib, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	fs, err := fonts.LoadSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	sys := gnw.New(gfx.New(platform.ScreenWidth, platform.ScreenHeight), fs)
	ctx := &Context{Sys: sys, Lib: lib, Assets: a, DataDir: dir}
	win := sys.AddWindow(0, 0, platform.ScreenWidth, platform.ScreenHeight, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	m := &upgradeMenu{ctx: ctx, win: win, filter: upgradeFilterState}
	return m, win
}

// clickFilterPlate presses and releases the left mouse button over the plate at column x of the filter strip.
func clickFilterPlate(m *upgradeMenu, x int) {
	px, py := x+4, 411+4
	m.ctx.Sys.CheckButtons(platform.MouseState{X: px, Y: py, Buttons: platform.MousePressLeft})
	m.ctx.Sys.CheckButtons(platform.MouseState{X: px, Y: py, Buttons: platform.MouseReleaseLeft})
}

// TestUpgradeFilterPlatesSticky checks the plates are sticky toggles that rest in the filter state, that a
// click flips the filter, and that the state survives reopening the menu.
func TestUpgradeFilterPlatesSticky(t *testing.T) {
	saved := upgradeFilterState
	t.Cleanup(func() { upgradeFilterState = saved })
	upgradeFilterState = upgradeFilter{ground: true, air: true, sea: true, building: true}

	m, win := newTestUpgradeMenu(t)
	m.buildFilterPlates(win)

	air := m.filterBtns[1005].bid
	cbt := m.filterBtns[1011].bid
	if !m.ctx.Sys.ButtonChecked(air) || m.ctx.Sys.ButtonChecked(cbt) {
		t.Fatal("plates do not rest in the default filter state")
	}

	clickFilterPlate(m, 500) // Air off
	if !m.applyFilterToggle(1005) {
		t.Fatal("1005 is not a plate event")
	}
	if m.ctx.Sys.ButtonChecked(air) || m.filter.air || upgradeFilterState.air {
		t.Errorf("Air plate still on after a click: checked=%v filter=%v state=%v",
			m.ctx.Sys.ButtonChecked(air), m.filter.air, upgradeFilterState.air)
	}
	clickFilterPlate(m, 598) // Combat on
	m.applyFilterToggle(1011)
	if !m.filter.combat || !upgradeFilterState.combat {
		t.Error("Combat plate click did not turn the filter on")
	}
	if m.applyFilterToggle(1300) {
		t.Error("a non plate event was taken as a plate")
	}

	// Reopen: a fresh menu starts from the package state, and its plates show it.
	m2, win2 := newTestUpgradeMenu(t)
	m2.buildFilterPlates(win2)
	if m2.filter.air || !m2.filter.combat {
		t.Errorf("reopened filter = %+v, want air off and combat on", m2.filter)
	}
	if m2.ctx.Sys.ButtonChecked(m2.filterBtns[1005].bid) || !m2.ctx.Sys.ButtonChecked(m2.filterBtns[1011].bid) {
		t.Error("reopened plates do not show the saved state")
	}
	clickFilterPlate(m2, 500) // Air back on: a second click releases the toggle
	m2.applyFilterToggle(1005)
	if !m2.filter.air {
		t.Error("second click did not turn Air back on")
	}
}
