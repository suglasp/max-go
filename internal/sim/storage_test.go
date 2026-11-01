// storage_test.go : tests resource storage clamping and complex pool redistribution.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// storageBuilding places a powered 2x2 storage building with the given bay capacity.
func storageBuilding(g *Game, t assets.UnitType, x, y, capacity int) *game.UnitInfo {
	b := stationaryBuilding(t, 0, x, y)
	b.Values.Set(game.AttribStorage, capacity)
	g.PlaceBuilding(b, x, y)
	return b
}

// adjStorable deploys a storable mobile unit at (x, y).
func adjStorable(g *Game, t assets.UnitType, flags game.Flags, x, y int) *game.UnitInfo {
	u := game.NewUnitInfo(t, 0, 0, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 20)
	u.Values.Set(game.AttribSpeed, 8)
	u.InitState()
	g.Units.Deploy(u, x, y)
	return u
}

// TestDepotStoresAndUnloadsLandUnit: a depot stores an adjacent land unit off the
// map, then the Activate flow places it back on a REACHABLE ADJACENT cell where it
// grows in - ProcessUnloading, not a drive out.
func TestDepotStoresAndUnloadsLandUnit(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	depot := storageBuilding(g, assets.Depot, 5, 5, 4) // 2x2 at (5,5)
	tank := adjStorable(g, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable, 7, 5)

	if !g.LoadUnit(depot, tank) {
		t.Fatal("depot did not store the adjacent tank")
	}
	if tank.Parent != depot || len(depot.Carried) != 1 {
		t.Fatalf("tank not stored: parent=%v carried=%d", tank.Parent, len(depot.Carried))
	}
	for _, u := range g.Units.Units() {
		if u == tank {
			t.Error("a stored tank should be OFF the unit registry")
		}
	}

	x, y, ok := g.UnloadSpot(depot, tank)
	if !ok {
		t.Fatal("UnloadSpot found no reachable cell beside the depot")
	}
	if !g.UnloadUnit(depot, tank, x, y) {
		t.Fatal("UnloadUnit refused a valid adjacent cell")
	}
	if len(depot.Carried) != 0 || tank.Parent != nil {
		t.Error("tank still in the bay after the unload")
	}
	if tank.GridX != x || tank.GridY != y {
		t.Errorf("unloaded unit at (%d,%d), want the picked cell (%d,%d)", tank.GridX, tank.GridY, x, y)
	}
	if tank.Unfold == 0 || tank.HiddenUntilExpand {
		t.Error("an unloaded unit grows in on its cell (Unfold>0, drawn)")
	}
	for _, mo := range g.MoveOrders() {
		if mo.Unit == tank {
			t.Error("an unloaded unit must NOT drive - it is placed")
		}
	}
	queued := false
	for _, u := range g.ReleasedExpansions() {
		if u == tank {
			queued = true
		}
	}
	if !queued {
		t.Error("unloaded unit not queued for the grow-in (expand) animation")
	}
	back := false
	for _, u := range g.Units.Units() {
		if u == tank {
			back = true
		}
	}
	if !back {
		t.Error("unloaded tank not back on the unit registry")
	}
}

// TestUnloadRefusesWhenBoxedIn: a carrier with no reachable adjacent cell cannot
// unload at all - the original refuses with _(a1f0) rather than stranding the cargo (the
// sea transport on open water case, here with a land carrier walled in by water).
func TestUnloadRefusesWhenBoxedIn(t *testing.T) {
	// Every cell but the carrier's own is water: a land passenger has nowhere to go.
	water := waterAt{}
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			if x != 5 || y != 5 {
				water[[2]int{x, y}] = true
			}
		}
	}
	g := newGameSurf(12, 12, water)
	g.Resources = economy.NewResourceMap(12, 12)

	boat := game.NewUnitInfo(assets.SeaTrans, 0, 1, 0)
	boat.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	boat.Values = &game.UnitValues{}
	boat.Values.Set(game.AttribStorage, 2)
	boat.InitState()
	g.Units.Deploy(boat, 5, 5)

	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribSpeed, 8)
	tank.InitState()
	tank.Parent = boat
	boat.Carried = append(boat.Carried, tank)

	if _, _, ok := g.UnloadSpot(boat, tank); ok {
		t.Error("UnloadSpot must find no cell for a land unit surrounded by water")
	}
	if g.UnloadUnit(boat, tank, 5, 6) {
		t.Error("UnloadUnit must refuse a water cell - the unit would be stranded")
	}
	if len(boat.Carried) != 1 {
		t.Error("the refused passenger stays aboard")
	}
}

// TestCanContainAndLoadCell: the building "Load" can pick a class matching unit
// regardless of distance (CanContain has no adjacency check, unlike CanLoad), still
// gates by class, and LoadCell finds a free cell beside the footprint to path to.
func TestCanContainAndLoadCell(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	depot := storageBuilding(g, assets.Depot, 5, 5, 4)
	far := adjStorable(g, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable, 12, 12) // not adjacent
	if !g.CanContain(depot, far) {
		t.Error("CanContain should accept a class-matching unit regardless of distance")
	}
	if g.CanLoad(depot, far) || g.AdjacentToContainer(depot, far) {
		t.Error("a far unit is not adjacent, so CanLoad/AdjacentToContainer must be false")
	}
	hangar := storageBuilding(g, assets.Hangar, 5, 10, 4)
	if g.CanContain(hangar, far) {
		t.Error("a hangar must reject a land unit by class")
	}
	x, y, ok := g.LoadCell(depot, far)
	if !ok {
		t.Fatal("LoadCell found no cell beside the footprint")
	}
	if !g.AdjacentToContainer(depot, &game.UnitInfo{GridX: x, GridY: y}) {
		t.Errorf("LoadCell (%d,%d) is not adjacent to the depot footprint", x, y)
	}
}

// TestStorageClassGating: a depot takes land but not air; a hangar takes air.
func TestStorageClassGating(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	depot := storageBuilding(g, assets.Depot, 5, 5, 4)
	hangar := storageBuilding(g, assets.Hangar, 5, 10, 4)
	airFlags := game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable

	if g.CanLoad(depot, adjStorable(g, assets.Bomber, airFlags, 7, 5)) {
		t.Error("a depot accepted an aircraft")
	}
	if !g.CanLoad(hangar, adjStorable(g, assets.Bomber, airFlags, 7, 10)) {
		t.Error("a hangar rejected an aircraft")
	}
}

// TestStorageCapacityFull: a depot with one slot rejects a second unit.
func TestStorageCapacityFull(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	depot := storageBuilding(g, assets.Depot, 5, 5, 1)
	land := game.FlagMobileLandUnit | game.FlagSelectable
	if !g.LoadUnit(depot, adjStorable(g, assets.Tank, land, 7, 5)) {
		t.Fatal("first load failed")
	}
	if g.LoadUnit(depot, adjStorable(g, assets.Tank, land, 7, 6)) {
		t.Error("depot over capacity accepted a second unit")
	}
}

// transportUnit places a 1x1 mobile land transport with the given bay capacity.
func transportUnit(g *Game, t assets.UnitType, x, y, capacity int) *game.UnitInfo {
	u := game.NewUnitInfo(t, 0, 0, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribStorage, capacity)
	u.Values.Set(game.AttribHits, 30)
	u.Values.Set(game.AttribSpeed, 8)
	u.InitState()
	g.Units.Deploy(u, x, y)
	return u
}

// TestCarrierAcceptanceTable: the receiver table (access.cpp:2097-2124) - the APC
// and the Barracks take INFANTRY/COMMANDO only, the Depot takes land VEHICLES only,
// the Dock takes sea units that are not amphibious, and the Sea Transport takes any
// land unit.
func TestCarrierAcceptanceTable(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	apc := transportUnit(g, assets.ClnTrans, 5, 5, 2)
	depot := storageBuilding(g, assets.Depot, 10, 5, 4)
	barracks := storageBuilding(g, assets.Barracks, 15, 5, 4)
	boat := transportUnit(g, assets.SeaTrans, 5, 15, 6)
	boat.Flags = game.FlagMobileSeaUnit | game.FlagSelectable

	land := game.FlagMobileLandUnit | game.FlagSelectable
	tank := adjStorable(g, assets.Tank, land, 6, 5)
	infantry := adjStorable(g, assets.Infantry, land, 4, 5)

	if g.CanContain(apc, tank) {
		t.Error("klei: the APC carries infantry/commando ONLY - a tank must be refused")
	}
	if !g.CanContain(apc, infantry) {
		t.Error("the APC must carry infantry")
	}
	if g.CanContain(barracks, tank) {
		t.Error("the Barracks garrisons infantry/commando ONLY")
	}
	if !g.CanContain(barracks, infantry) {
		t.Error("the Barracks must garrison infantry")
	}
	if !g.CanContain(depot, tank) {
		t.Error("the Depot must take a land vehicle")
	}
	if g.CanContain(depot, infantry) {
		t.Error("klei: the Depot takes vehicles, NOT infantry")
	}
	if !g.CanContain(boat, tank) || !g.CanContain(boat, infantry) {
		t.Error("the Sea Transport ferries any land unit")
	}

	// The Dock takes sea units, but not an amphibious (land AND sea) one.
	dock := storageBuilding(g, assets.Dock, 15, 15, 4)
	corvette := adjStorable(g, assets.Corvette, game.FlagMobileSeaUnit|game.FlagSelectable, 17, 15)
	amphib := adjStorable(g, assets.Surveyor, game.FlagMobileSeaUnit|game.FlagMobileLandUnit|game.FlagSelectable, 14, 15)
	if !g.CanContain(dock, corvette) {
		t.Error("the Dock must take a sea unit")
	}
	if g.CanContain(dock, amphib) {
		t.Error("klei: the Dock requires sea AND NOT land - an amphibious unit is refused")
	}
}

// TestTransportNestingKleiRule: the original ferries a loaded APC on a Sea Transport and
// stores it in a Depot (a CLNTRANS is an ordinary non infantry land unit to the
// receiver logic, access.cpp:2105-2109), but a sea/air transport is never cargo.
func TestTransportNestingKleiRule(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	depot := storageBuilding(g, assets.Depot, 10, 5, 4)
	boat := transportUnit(g, assets.SeaTrans, 5, 15, 6)
	boat.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	apc := transportUnit(g, assets.ClnTrans, 6, 15, 2)
	apc2 := transportUnit(g, assets.ClnTrans, 12, 5, 2)

	if !g.CanContain(boat, apc) {
		t.Error("klei: a Sea Transport ferries a (loaded) APC")
	}
	if !g.CanContain(depot, apc2) {
		t.Error("klei: a Depot stores an APC (a non infantry land unit)")
	}

	// A carried APC keeps its own cargo: the nesting recurses.
	infantry := adjStorable(g, assets.Infantry, game.FlagMobileLandUnit|game.FlagSelectable, 7, 15)
	if !g.LoadUnit(apc, infantry) {
		t.Fatal("the APC did not take its infantry")
	}
	if !g.LoadUnit(boat, apc) {
		t.Fatal("the Sea Transport did not take the loaded APC")
	}
	if len(boat.Carried) != 1 || len(apc.Carried) != 1 {
		t.Errorf("nesting lost cargo: boat=%d apc=%d", len(boat.Carried), len(apc.Carried))
	}

	// A sea transport is never cargo itself.
	if g.CanContain(depot, boat) {
		t.Error("a Sea Transport must never be stored in a Depot")
	}
}

// TestDisabledUnitCannotBoard: the original refuses to load a disabled unit (_(9897)), and a
// disabled passenger whose carrier dies must not leave a stale countdown entry behind.
func TestDisabledUnitCannotBoard(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	depot := storageBuilding(g, assets.Depot, 5, 5, 4)
	tank := adjStorable(g, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable, 7, 5)

	// Disable it (the commando's effect, applied directly here).
	disable := func(u *game.UnitInfo) {
		u.SetOrder(game.OrderDisable)
		u.SetOrderState(game.OrderStateExecutingOrder)
		if g.disabled == nil {
			g.disabled = make(map[*game.UnitInfo]int)
		}
		g.disabled[u] = 3
	}

	disable(tank)
	if g.CanContain(depot, tank) || g.LoadUnit(depot, tank) {
		t.Error("a DISABLED unit must not board (klei _(9897))")
	}

	// A disabled unit already aboard: the carrier's death purges its residue.
	delete(g.disabled, tank)
	tank.SetOrder(game.OrderAwait)
	if !g.LoadUnit(depot, tank) {
		t.Fatal("the healthy tank should board")
	}
	disable(tank)
	g.DestroyUnit(depot)
	if _, stale := g.disabled[tank]; stale {
		t.Error("a stale disabled countdown survived the carrier's destruction")
	}
}

// TestDropUnloadedUnitPlacesBeneathAndGrows: an air transport drops a carried unit onto
// its OWN cell (the ground beneath it) and starts the grow in animation - the
// air transport unload, distinct from the adjacent drop / marker exit.
func TestDropUnloadedUnitPlacesBeneathAndGrows(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)

	// An air transport hovering over an empty cell, carrying a tank (stored off the map).
	air := game.NewUnitInfo(assets.AirTrans, 0, 1, 0)
	air.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	air.Values = &game.UnitValues{}
	air.Values.Set(game.AttribStorage, 3)
	air.InitState()
	g.Units.Deploy(air, 8, 8)

	tank := adjStorable(g, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable, 2, 2)
	g.Units.Remove(tank) // stash it aboard (off the map)
	tank.Parent = air
	air.Carried = append(air.Carried, tank)

	if !g.DropUnloadedUnit(air, tank) {
		t.Fatal("DropUnloadedUnit refused a valid drop")
	}
	if len(air.Carried) != 0 {
		t.Error("tank still aboard after the drop")
	}
	if tank.Parent != nil {
		t.Error("dropped tank should have no parent")
	}
	if tank.GridX != 8 || tank.GridY != 8 {
		t.Errorf("tank dropped at (%d,%d), want the transport's own cell (8,8)", tank.GridX, tank.GridY)
	}
	if tank.Speed != 8 {
		t.Errorf("a just-unloaded unit keeps its remaining move, Speed = %d want 8 (klei: ProcessUnloading does not touch speed)", tank.Speed)
	}
	if tank.Unfold == 0 {
		t.Error("dropped tank should start collapsed (Unfold>0) and grow in")
	}
	// Queued for the grow in (expand) animation.
	queued := false
	for _, u := range g.ReleasedExpansions() {
		if u == tank {
			queued = true
		}
	}
	if !queued {
		t.Error("dropped tank not queued for the grow-in (expand) animation")
	}
	// Back on the unit registry.
	onMap := false
	for _, u := range g.Units.Units() {
		if u == tank {
			onMap = true
		}
	}
	if !onMap {
		t.Error("dropped tank not on the unit registry after the drop")
	}
}

// TestDropUnloadedUnitRefusesImpassableTerrain: an air transport hovering over
// water cannot drop a LAND unit onto it - the drop is refused and the unit stays
// aboard (ORDER_UNLOAD is gated on Access_IsAccessible, message _(eeb4)).
func TestDropUnloadedUnitRefusesImpassableTerrain(t *testing.T) {
	g := newGameSurf(16, 16, waterAt{{8, 8}: true})
	g.Resources = economy.NewResourceMap(16, 16)

	air := game.NewUnitInfo(assets.AirTrans, 0, 1, 0)
	air.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	air.Values = &game.UnitValues{}
	air.Values.Set(game.AttribStorage, 3)
	air.InitState()
	g.Units.Deploy(air, 8, 8)

	tank := adjStorable(g, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable, 2, 2)
	g.Units.Remove(tank) // stash it aboard (off the map)
	tank.Parent = air
	air.Carried = append(air.Carried, tank)

	if g.CanUnloadBeneath(air, tank) {
		t.Error("CanUnloadBeneath must refuse a land unit over open water")
	}
	if g.DropUnloadedUnit(air, tank) {
		t.Fatal("DropUnloadedUnit dropped a land unit onto water")
	}
	if len(air.Carried) != 1 || tank.Parent != air {
		t.Error("the refused unit must stay aboard")
	}
}

// TestCarriedUnitsRefreshAtTurnStart: a stored unit's move budget refills at
// BeginTeamTurn like any deployed unit's (the original keeps stored units in its main
// lists), so a later unload keeps the TURN's remaining speed, not a stale one
// from the turn it boarded.
func TestCarriedUnitsRefreshAtTurnStart(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)

	// Distinct IDs: the registry is keyed by unit ID, so the shared-ID test
	// helpers (both mint ID 0) would evict one another from it.
	depot := stationaryBuilding(assets.Depot, 0, 5, 5)
	depot.ID = 1
	depot.Values.Set(game.AttribStorage, 4)
	g.PlaceBuilding(depot, 5, 5)

	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 20)
	tank.Values.Set(game.AttribSpeed, 8)
	tank.InitState()
	g.Units.Deploy(tank, 7, 5)

	if !g.LoadUnit(depot, tank) {
		t.Fatal("depot did not store the adjacent tank")
	}
	tank.Speed = 0 // boarded with its move spent

	g.BeginTeamTurn(0)

	if tank.Speed != 8 {
		t.Errorf("carried tank Speed = %d after turn start, want the refreshed 8", tank.Speed)
	}
}
