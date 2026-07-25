// airload_test.go : tests airload.go.
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

// TestAirTransportLoadDescendsPicksUpAscends: loading a unit INTO an air transport
// runs the fly over -> descend -> pickup -> ascend animation (not the land/sea
// suck up): FlagHovering clears (descending), then at AirAlt 0 the unit is taken
// aboard and FlagHovering resets (rising), then the job clears once the transport
// is back at flying altitude. The transport and unit start on the same cell so the
// flight phase is a move that does nothing, isolating the descend/pickup/ascend phases.
func TestAirTransportLoadDescendsPicksUpAscends(t *testing.T) {
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
	mgr.Deploy(tank, 8, 8) // already beneath the transport: no fly over move needed

	s := &Screen{game: g}

	// Begin the air load: the transport is already over the unit, so it arms directly.
	s.beginAirLoad(air, tank)
	if s.effects.airLoad == nil {
		t.Fatal("beginAirLoad should arm the air-load animation")
	}

	// First tick: arrived over the unit -> start descending (FlagHovering cleared).
	s.tickAirLoad()
	if s.effects.airLoad == nil || !s.effects.airLoad.descending {
		t.Fatal("tickAirLoad should start the descent once over the unit")
	}
	if air.Flags.Has(game.FlagHovering) {
		t.Error("the air transport should be descending (FlagHovering cleared)")
	}
	if len(air.Carried) != 0 {
		t.Error("the unit must stay on the map until the transport touches down")
	}

	// Still descending (AirAlt > 0): tickAirLoad waits, does not pick up yet.
	air.AirAlt = game.AirAltMax
	s.tickAirLoad()
	if s.effects.airLoad == nil || s.effects.airLoad.loaded {
		t.Fatal("tickAirLoad should wait out the descent, not pick up yet")
	}

	// Touched down (AirAlt 0): the unit is taken aboard and the transport rises.
	air.AirAlt = 0
	s.tickAirLoad()
	if s.effects.airLoad == nil || !s.effects.airLoad.loaded {
		t.Fatal("at AirAlt 0 the unit should load and the job mark loaded")
	}
	if len(air.Carried) != 1 {
		t.Errorf("the unit should be aboard after pickup, got %d carried", len(air.Carried))
	}
	if !air.Flags.Has(game.FlagHovering) {
		t.Error("the transport should be rising again (FlagHovering re-set)")
	}

	// Risen back to flying altitude: the job clears.
	air.AirAlt = game.AirAltMax
	s.tickAirLoad()
	if s.effects.airLoad != nil {
		t.Error("the load job should clear once the transport is flying again")
	}
}

// TestAirTransportLoadSurvivesPausedRoute verifies that
// an air transport that runs out of movement points before reaching the unit must
// KEEP its load job and resume next turn, not forget what it was going to pick up.
// A transport stalled mid flight has a PENDING (over budget) route - not moving this
// frame, but queued to drive on - and tickAirLoad must wait it out, not abort.
func TestAirTransportLoadSurvivesPausedRoute(t *testing.T) {
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
	air.InitState()
	mgr.Deploy(air, 2, 2)

	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 20)
	tank.InitState()
	mgr.Deploy(tank, 12, 12) // far away: the transport cannot reach it in one turn

	s := &Screen{game: g}
	s.beginAirLoad(air, tank)
	if s.effects.airLoad == nil {
		t.Fatal("beginAirLoad should arm the air-load animation")
	}

	// Simulate "ran out of moves mid flight": no active move order remains, but a
	// pending route is queued to resume next turn.
	g.CancelOrder(air)
	g.SetPendingDestination(air, tank.GridX, tank.GridY)
	if s.isMoving(air) {
		t.Fatal("test setup: the transport should have no ACTIVE move now")
	}

	// tickAirLoad must NOT abort: the pending route means it is still on its way.
	s.tickAirLoad()
	if s.effects.airLoad == nil {
		t.Fatal("a paused (pending) route must keep the air-load job alive, not abort it")
	}
	if s.effects.airLoad.descending {
		t.Error("the transport has not arrived yet; it must not start descending")
	}
}

// TestAirTransportLoadCompletesAcrossTurns drives the FULL turn cycle: a slow air
// transport sent to load a far unit runs out of moves each turn, and must finish the
// trip + pick the unit up over several turns (resumePending reissues the flight at
// each turn start), so the load job continues until the carrier reaches its target.
func TestAirTransportLoadCompletesAcrossTurns(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	air := game.NewUnitInfo(assets.AirTrans, 0, 1, 0)
	air.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	air.Values = &game.UnitValues{}
	air.Values.Set(game.AttribStorage, 3)
	air.Values.Set(game.AttribSpeed, 4) // only 4 cells/turn -> the far unit takes several turns
	air.InitState()
	air.Speed = 4
	mgr.Deploy(air, 2, 2)

	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 20)
	tank.InitState()
	mgr.Deploy(tank, 14, 2) // 12 cells east: needs ~3 turns at speed 4

	s := &Screen{game: g}
	s.beginAirLoad(air, tank)

	for turn := 0; turn < 8 && len(air.Carried) == 0; turn++ {
		g.ResolveMovement() // fly this turn's budget; the remainder shelves as pending
		s.tickAirLoad()     // keep the job alive (pending) or, on arrival, start descending
		// Simulate the descend tween: once descending, drop to the ground so pickup fires.
		if s.effects.airLoad != nil && s.effects.airLoad.descending && air.AirAlt > 0 {
			air.AirAlt = 0
			s.tickAirLoad()
		}
		if len(air.Carried) == 0 {
			if s.effects.airLoad == nil {
				t.Fatalf("turn %d: the air-load job was dropped before pickup (transport at %d,%d, pending=%v)",
					turn, air.GridX, air.GridY, g.HasPendingRoute(air))
			}
			g.BeginTeamTurn(0) // refill movement + resumePending reissues the flight
		}
	}

	if len(air.Carried) != 1 {
		t.Fatalf("the air transport never picked up the unit across turns; carried=%d, transport at (%d,%d)",
			len(air.Carried), air.GridX, air.GridY)
	}
}

// TestAirTransportLoadReissuesWhenRouteDropped covers the robustness path: even when
// the transport stalls mid flight with NO shelved route (the over budget shelve only
// fires on an exact Speed==0 stop, so a stall with leftover budget drops the route),
// tickAirLoad must REISSUE the flight next turn instead of forgetting the pickup.
func TestAirTransportLoadReissuesWhenRouteDropped(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	air := game.NewUnitInfo(assets.AirTrans, 0, 1, 0)
	air.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	air.Values = &game.UnitValues{}
	air.Values.Set(game.AttribStorage, 3)
	air.Values.Set(game.AttribSpeed, 6)
	air.InitState()
	mgr.Deploy(air, 2, 2)

	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 20)
	tank.InitState()
	mgr.Deploy(tank, 14, 2)

	s := &Screen{game: g}
	s.beginAirLoad(air, tank)

	// Simulate a stall that DROPPED the route (no active order, no pending route) with
	// the transport still short of the unit and out of moves this turn.
	g.CancelOrder(air)
	air.Speed = 0
	s.tickAirLoad()
	if s.effects.airLoad == nil {
		t.Fatal("with no budget the job must wait, not abort")
	}
	if s.isMoving(air) {
		t.Fatal("no move should be issued while out of budget")
	}

	// Next turn refills the budget: tickAirLoad must reissue the flight toward the unit.
	air.Speed = air.Values.Get(game.AttribSpeed)
	s.tickAirLoad()
	if !s.isMoving(air) {
		t.Error("with budget refilled, tickAirLoad should RE-ISSUE the flight toward the unit")
	}
}
