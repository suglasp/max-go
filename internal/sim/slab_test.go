// slab_test.go : tests the unit slab allocation pool in sim.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/game"
)

// TestBuildRoadIsGroundCover drives the terrain special build flow for a Road: an
// engineer raises it as drivable GROUND COVER - it lays NO foundation slab and joins
// NO complex (unlike a stationary building, which goes through PlaceBuilding). This is
// the downstream that makes the buildDesc(Road)=FlagGroundCover fix faithful.
func TestBuildRoadIsGroundCover(t *testing.T) {
	g := newGame(t, 24, 24)

	// A power station forms a complex with pooled material for the build to draw.
	st := game.NewUnitInfo(assets.PowerStn, 0, 1, 0)
	st.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	st.Values = &game.UnitValues{}
	st.Values.Set(game.AttribHits, 50)
	c := g.PlaceBuilding(st, 5, 5)
	if c == nil {
		t.Fatal("no complex formed")
	}
	c.Material = 50

	// An engineer adjacent to the station raises a 1x1 road on its own cell. It funds
	// the build from its OWN cargo (a player builder spends its load, not the base pool).
	eng := game.NewUnitInfo(assets.Engineer, 0, 2, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.InitState()
	eng.Storage = 50
	g.Units.Deploy(eng, 7, 5)

	road := game.NewUnitInfo(assets.Road, 0, 3, 0)
	road.Flags = game.FlagGroundCover // what buildDesc(Road) now produces
	road.Values = &game.UnitValues{}
	road.Values.Set(game.AttribTurns, 1)
	road.Values.Set(game.AttribHits, 2)
	g.StartBuild(eng, road, 7, 5, 1)

	g.AdvanceBuilds(0) // one turn: the 1-turn road finishes and deploys

	// The road is on the map as ground cover, with no slab and no complex.
	var placed *game.UnitInfo
	for _, u := range g.Units.Units() {
		switch u.Type {
		case assets.Road:
			placed = u
		case assets.SmlSlab, assets.LrgSlab:
			t.Errorf("road build laid a %v slab; a road lays none", u.Type)
		}
	}
	if placed == nil {
		t.Fatal("road was not deployed after the build finished")
	}
	if !placed.Flags.Has(game.FlagGroundCover) {
		t.Errorf("built road flags = %#x; want FlagGroundCover", placed.Flags)
	}
	for _, n := range g.Networks {
		if n != nil && n.ComplexOf(placed) != nil {
			t.Error("a road joined a complex; ground cover never does")
		}
	}
}

// TestFlatSpecialBuildNoExit verifies that a flat terrain special such as Road frees its engineer immediately, while a building parks its builder for exit.
// C++ ported:
// unitinfo.cpp - the ROAD/BRIDGE/WTRPLTFM/CNCT_4W branch frees the builder at once
// vs the BUILDING branch's ORDER_AWAIT_TAPE_POSITIONING exit.
func TestFlatSpecialBuildNoExit(t *testing.T) {
	g := newGame(t, 24, 24)

	// A power station with pooled material both builds draw from.
	st := game.NewUnitInfo(assets.PowerStn, 0, 1, 0)
	st.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	st.Values = &game.UnitValues{}
	st.Values.Set(game.AttribHits, 50)
	c := g.PlaceBuilding(st, 5, 5)
	if c == nil {
		t.Fatal("no complex formed")
	}
	c.Material = 200

	// An engineer lays a 1-turn road on its own cell: it must be free, not parked.
	eng := game.NewUnitInfo(assets.Engineer, 0, 2, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.InitState()
	eng.Storage = 200 // build from its own cargo, away from the base
	g.Units.Deploy(eng, 8, 5)

	road := game.NewUnitInfo(assets.Road, 0, 3, 0)
	road.Flags = game.FlagGroundCover
	road.Values = &game.UnitValues{}
	road.Values.Set(game.AttribTurns, 1)
	road.Values.Set(game.AttribHits, 2)
	g.StartBuild(eng, road, 8, 5, 1)
	g.AdvanceBuilds(0)

	if g.IsBuildReady(eng) {
		t.Error("engineer is parked in build-ready after a road; a flat special needs no exit")
	}
	if eng.Building {
		t.Error("engineer kept its build pose after a road; it must be freed in place")
	}
	if eng.Order != game.OrderAwait {
		t.Errorf("engineer order = %v after a road; want OrderAwait (free)", eng.Order)
	}

	// A constructor raising a real building IS parked awaiting its drive off exit.
	con := game.NewUnitInfo(assets.Constrct, 0, 4, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.InitState()
	g.Units.Deploy(con, 12, 12)

	depot := game.NewUnitInfo(assets.Depot, 0, 5, 0)
	depot.Flags = game.FlagBuilding | game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable
	depot.Values = &game.UnitValues{}
	depot.Values.Set(game.AttribTurns, 1)
	depot.Values.Set(game.AttribHits, 10)
	con.Storage = 200 // build away from the base, from the constructor's own cargo
	g.StartBuild(con, depot, 12, 12, 1)
	g.AdvanceBuilds(0)

	if !g.IsBuildReady(con) {
		t.Error("constructor is not parked after a building; a real building requires the exit")
	}
}

// TestDestroyBuildingClearsSlab confirms that destroying a building (DestroyUnit)
// removes both the building and its foundation slab, leaving only the bare
// ground: the rubble (a gamescreen effect) is what remains, not the slab.
func TestDestroyBuildingClearsSlab(t *testing.T) {
	g := newGame(t, 8, 8)
	b := game.NewUnitInfo(assets.MiningSt, 0, 1, 0)
	b.Flags = game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable | game.FlagBuilding
	g.PlaceBuilding(b, 2, 2)

	slabBefore := 0
	for _, u := range g.Units.Units() {
		if u.Type == assets.LrgSlab && u.Flags.Has(game.FlagGroundCover) {
			slabBefore++
		}
	}
	if slabBefore == 0 {
		t.Fatal("no large slab laid under the building")
	}

	g.DestroyUnit(b)

	slabAfter, buildingAfter := 0, 0
	for _, u := range g.Units.Units() {
		switch u.Type {
		case assets.LrgSlab:
			slabAfter++
		case assets.MiningSt:
			buildingAfter++
		}
	}
	if buildingAfter != 0 {
		t.Errorf("building still present after destruction: %d", buildingAfter)
	}
	if slabAfter != 0 {
		t.Errorf("slab still present after destruction: %d (a destroyed building must leave no slab)", slabAfter)
	}
}

// TestWaterPlatformFoundation verifies that a Water Platform acts as the foundation without a land slab and makes its water cell valid for a land building.
func TestWaterPlatformFoundation(t *testing.T) {
	const waterX = 4
	g := newMixedGame(t, 8, 8, waterX)

	// A Water Platform is site valid on the open water column, not on land.
	platformFlags := game.FlagStationary | game.FlagSelectable // no FlagRequiresSlab: it is the foundation
	platDesc := access.UnitDesc{
		Type:     assets.WtrPltfm,
		Flags:    platformFlags,
		LandType: access.LandTypeForUnit(assets.WtrPltfm, platformFlags),
	}
	if !build.SiteValid(g.Grid, platDesc, 0, waterX, 2) {
		t.Fatal("Water Platform should be site-valid on open water")
	}
	if build.SiteValid(g.Grid, platDesc, 0, waterX-1, 2) {
		t.Error("Water Platform should NOT be site-valid on land")
	}

	// Lay the platform; it must leave no slab (it is itself the foundation).
	platform := game.NewUnitInfo(assets.WtrPltfm, 0, 1, 0)
	platform.Flags = platformFlags
	g.PlaceBuilding(platform, waterX, 2)
	for _, u := range g.Units.Units() {
		if u.Type == assets.LrgSlab || u.Type == assets.SmlSlab {
			t.Errorf("a Water Platform on water laid a %v slab; it must lay none", u.Type)
		}
	}

	// A land building (Power Generator) is now site valid on the platform cell.
	landFlags := game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable
	powerGen := access.UnitDesc{
		Type:     assets.PowGen,
		Flags:    landFlags,
		LandType: access.LandTypeForUnit(assets.PowGen, landFlags),
	}
	if !build.SiteValid(g.Grid, powerGen, 0, waterX, 2) {
		t.Error("a land building should be site-valid on a Water Platform over water")
	}
	if build.SiteValid(g.Grid, powerGen, 0, waterX, 3) {
		t.Error("a land building should NOT be site-valid on bare water (no platform)")
	}
}
