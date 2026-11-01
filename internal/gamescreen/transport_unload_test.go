// transport_unload_test.go : tests transport unload targeting and disembark cell selection.
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
	"github.com/suglasp/max-go/internal/world"
)

// TestTransportRepairOpsActivateArmsPlacement: the Repair Shop screen's per unit
// Activate arms the PLACEMENT pick - the carried unit stays aboard until the
// player clicks an open adjacent square, which then places it there (per
// repairshopmenu.cpp Activate -> _(755b) -> the ORDER_ACTIVATE click), instead of
// the old drive-out that could strand cargo on water.
func TestTransportRepairOpsActivateArmsPlacement(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// A 1x1 personnel transport carrying an infantry squad (stored off the map).
	apc := game.NewUnitInfo(assets.ClnTrans, 0, 1, 0)
	apc.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	apc.Values = &game.UnitValues{}
	apc.Values.Set(game.AttribStorage, 2)
	apc.InitState()
	mgr.Deploy(apc, 5, 5)

	squad := game.NewUnitInfo(assets.Infantry, 0, 2, 0)
	squad.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	squad.Values = &game.UnitValues{}
	squad.Values.Set(game.AttribHits, 20)
	squad.Values.Set(game.AttribSpeed, 8)
	squad.InitState()
	squad.Parent = apc
	apc.Carried = append(apc.Carried, squad)

	s := &Screen{game: g}
	ops := &screenRepairOps{s: s, building: apc, team: 0}
	ops.units = apc.Carried // Activate reads the resolved unit list

	ops.Activate(0)
	// The placement pick is armed; the unit is still aboard until the click lands.
	if s.orderInput.unloadUnit != squad || s.orderInput.unloadFrom != apc {
		t.Fatal("Activate should arm the unload placement pick")
	}
	if len(apc.Carried) != 1 {
		t.Error("the unit stays aboard until the player picks a square")
	}

	// The click on an open adjacent square places it there and it grows in.
	if !s.tryTransportTarget(6, 5) {
		t.Fatal("the placement click should be consumed")
	}
	if len(apc.Carried) != 0 || squad.Parent != nil {
		t.Error("the picked square should have taken the unit out of the bay")
	}
	if squad.GridX != 6 || squad.GridY != 5 {
		t.Errorf("unit placed at (%d,%d), want the clicked (6,5)", squad.GridX, squad.GridY)
	}
	if squad.Unfold == 0 {
		t.Error("a placed unit grows in (Unfold>0)")
	}
	if s.transportArmed() {
		t.Error("the pick must disarm after the placement")
	}
}

// TestActivateRefusesBoxedInCarrier: a carrier with no reachable adjacent cell
// refuses the Activate outright (_(a1f0)) instead of stranding the cargo.
func TestActivateRefusesBoxedInCarrier(t *testing.T) {
	const w, h = 12, 12
	mgr := units.NewManager()
	// Everything but the boat's own cell is water: a land passenger has nowhere to go.
	water := waterSurf{}
	grid := &access.Grid{W: w, H: h, Surf: water, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	boat := game.NewUnitInfo(assets.SeaTrans, 0, 1, 0)
	boat.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	boat.Values = &game.UnitValues{}
	boat.Values.Set(game.AttribStorage, 2)
	boat.InitState()
	mgr.Deploy(boat, 5, 5)

	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribSpeed, 8)
	tank.InitState()
	tank.Parent = boat
	boat.Carried = append(boat.Carried, tank)

	s := &Screen{game: g}
	ops := &screenRepairOps{s: s, building: boat, team: 0}
	ops.units = boat.Carried

	ops.Activate(0)
	if s.orderInput.unloadUnit != nil {
		t.Error("a boxed-in carrier must NOT arm a placement pick")
	}
	if len(boat.Carried) != 1 {
		t.Error("the cargo stays aboard when there is nowhere to put it")
	}
}

// waterSurf is all water: a land unit can stand nowhere.
type waterSurf struct{}

func (waterSurf) SurfaceAt(x, y int) uint8 { return world.SurfaceWater }
