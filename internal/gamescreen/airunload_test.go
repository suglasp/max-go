// airunload_test.go : tests airunload.go.
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

// TestAirTransportUnloadDescendsDropsAscends: Activating a carried unit on an AIR
// transport runs the descend -> drop beneath -> ascend animation (not the build ready
// marker exit): FlagHovering clears (descending), then at AirAlt 0 the unit drops on
// the transport's own cell and FlagHovering sets again (rising), then the job clears once
// the transport is back at flying altitude.
func TestAirTransportUnloadDescendsDropsAscends(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	air := game.NewUnitInfo(assets.AirTrans, 0, 1, 0)
	air.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	air.Values = &game.UnitValues{}
	air.Values.Set(game.AttribStorage, 3)
	air.InitState() // AirAlt starts at AirAltMax (flying)
	mgr.Deploy(air, 8, 8)

	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 20)
	tank.Values.Set(game.AttribSpeed, 8)
	tank.InitState()
	tank.Parent = air
	air.Carried = append(air.Carried, tank)

	s := &Screen{game: g}

	// Activate on the air transport host arms the descend, NOT a build ready eject.
	ops := &screenRepairOps{s: s, building: air, team: 0}
	ops.units = air.Carried
	ops.Activate(0)
	if s.effects.airUnload == nil {
		t.Fatal("Activate on an air transport should arm the unload animation")
	}
	if air.Flags.Has(game.FlagHovering) {
		t.Error("the air transport should be descending (FlagHovering cleared)")
	}
	if g.BuildReadyInfo(air) != nil {
		t.Error("an air transport must NOT eject build-ready (it drops beneath itself)")
	}
	if len(air.Carried) != 1 {
		t.Error("the unit stays aboard until the transport touches down")
	}

	// Still descending (AirAlt > 0): tickAirUnload waits, does not drop yet.
	air.AirAlt = game.AirAltMax
	s.tickAirUnload()
	if s.effects.airUnload == nil || s.effects.airUnload.dropped {
		t.Fatal("tickAirUnload should wait out the descent, not drop yet")
	}

	// Touched down (AirAlt 0): the unit drops beneath and the transport starts rising.
	air.AirAlt = 0
	s.tickAirUnload()
	if s.effects.airUnload == nil || !s.effects.airUnload.dropped {
		t.Fatal("at AirAlt 0 the unit should drop and the job mark dropped")
	}
	if len(air.Carried) != 0 {
		t.Error("the unit should have left the bay on drop")
	}
	if tank.GridX != 8 || tank.GridY != 8 {
		t.Errorf("the unit should be dropped beneath the transport (8,8), got (%d,%d)", tank.GridX, tank.GridY)
	}
	if !air.Flags.Has(game.FlagHovering) {
		t.Error("the transport should be rising again (FlagHovering re-set)")
	}

	// Risen back to flying altitude: the job clears.
	air.AirAlt = game.AirAltMax
	s.tickAirUnload()
	if s.effects.airUnload != nil {
		t.Error("the unload job should clear once the transport is flying again")
	}
}
