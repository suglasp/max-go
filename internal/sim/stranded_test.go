// stranded_test.go : tests stranded.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// addCover deploys a stationary terrain cover (road, bridge, platform) at a cell.
func addCover(g *Game, id int, typ assets.UnitType, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(typ, 0, id, 0)
	u.Flags = game.FlagStationary
	u.Hits = 1
	g.Units.Deploy(u, gx, gy)

	return u
}

// assertSilent fails when a destroy queued any blast, explosion or kill event.
func assertSilent(t *testing.T, g *Game) {
	t.Helper()

	if n := len(g.DrainMineBlasts()); n != 0 {
		t.Errorf("mine blasts queued = %d, want 0", n)
	}
	if n := len(g.DrainCasualties()); n != 0 {
		t.Errorf("casualty events queued = %d, want 0", n)
	}
	if n := len(g.DrainProjectileImpacts()); n != 0 {
		t.Errorf("projectile impacts queued = %d, want 0", n)
	}
}

// TestTankOnDestroyedBridgeIsStranded: a land unit on a bridge over water goes down with it.
func TestTankOnDestroyedBridgeIsStranded(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{{5, 5}: true})
	bridge := addCover(g, 2, assets.Bridge, 5, 5)
	tank := addTankTarget(g, 1, 5, 5, 0)

	g.DestroyUnit(bridge)

	if g.Units.ByID.Get(tank.ID) != nil {
		t.Error("the tank on the destroyed bridge must be gone")
	}
	if got := g.TeamStats[0].UnitsLost; got != 1 {
		t.Errorf("UnitsLost = %d, want 1", got)
	}
	if got := g.TeamStats[0].Casualties[assets.Tank]; got != 1 {
		t.Errorf("tank Casualties = %d, want 1", got)
	}
	assertSilent(t, g)
}

// TestMineOnDestroyedPlatformSinksSilently: a land mine on a platform sinks, counted once, no blast.
func TestMineOnDestroyedPlatformSinksSilently(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{{5, 5}: true})
	platform := addCover(g, 2, assets.WtrPltfm, 5, 5)
	mine := game.NewUnitInfo(assets.LandMine, 0, 3, 0)
	mine.Flags = game.FlagStationary
	mine.Hits = 2
	g.Units.Deploy(mine, 5, 5)

	g.DestroyUnit(platform)

	if g.Units.ByID.Get(mine.ID) != nil || len(g.Units.Cells.At(5, 5)) != 0 {
		t.Error("the land mine on the destroyed platform must be gone")
	}
	if got := g.TeamStats[0].Casualties[assets.LandMine]; got != 1 {
		t.Errorf("mine Casualties = %d, want 1", got)
	}
	assertSilent(t, g)
}

// TestSeaMineUnderBridgeStays: a sea mine rests on water, so a dead bridge leaves it alone.
func TestSeaMineUnderBridgeStays(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{{5, 5}: true})
	bridge := addCover(g, 2, assets.Bridge, 5, 5)
	mine := game.NewUnitInfo(assets.SeaMine, 0, 3, 0)
	mine.Flags = game.FlagStationary
	mine.Hits = 2
	g.Units.Deploy(mine, 5, 5)

	g.DestroyUnit(bridge)

	if g.Units.ByID.Get(mine.ID) != mine {
		t.Error("a sea mine must stay on water when the bridge above it dies")
	}
}

// TestShipUnderDestroyedBridgeStays: a sea unit under a bridge is on water either way.
func TestShipUnderDestroyedBridgeStays(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{{5, 5}: true})
	bridge := addCover(g, 2, assets.Bridge, 5, 5)
	ship := game.NewUnitInfo(assets.Corvette, 0, 1, 0)
	ship.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	ship.Values = &game.UnitValues{}
	ship.Hits = 10
	g.Units.Deploy(ship, 5, 5)

	g.DestroyUnit(bridge)

	if g.Units.ByID.Get(ship.ID) != ship {
		t.Error("the ship under the bridge must stay")
	}
	if got := g.TeamStats[0].UnitsLost; got != 0 {
		t.Errorf("UnitsLost = %d, want 0", got)
	}
}

// TestLandedPlaneLosesPad: a landed aircraft with its pad gone is stranded, even on land.
func TestLandedPlaneLosesPad(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{})
	pad := addLandPad(g, 9, 5, 5)
	plane := addAirUnit(g, 1, 5, 5, false)
	plane.Hits = 20

	g.DestroyUnit(pad)

	if g.Units.ByID.Get(plane.ID) != nil {
		t.Error("the landed plane must be gone once its pad is destroyed")
	}
	if got := g.TeamStats[0].Casualties[assets.Bomber]; got != 1 {
		t.Errorf("plane Casualties = %d, want 1", got)
	}
	assertSilent(t, g)
}

// TestFlyingPlaneOverDestroyedPadStays: a hovering aircraft is never stranded.
func TestFlyingPlaneOverDestroyedPadStays(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{})
	pad := addLandPad(g, 9, 5, 5)
	plane := addAirUnit(g, 1, 5, 5, true)
	plane.Hits = 20

	g.DestroyUnit(pad)

	if g.Units.ByID.Get(plane.ID) != plane {
		t.Error("a flying plane must stay")
	}
}

// TestTankDeathRemovesRoadUnderIt: the road under a dead tank goes, the next cell is untouched.
func TestTankDeathRemovesRoadUnderIt(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{})
	road := addCover(g, 10, assets.Road, 5, 5)
	roadNext := addCover(g, 11, assets.Road, 6, 5)
	tank := addTankTarget(g, 1, 5, 5, 0)
	other := addTankTarget(g, 2, 6, 5, 0)

	g.DestroyUnit(tank)

	if g.Units.ByID.Get(road.ID) != nil || len(g.Units.Cells.At(5, 5)) != 0 {
		t.Error("the road under the dead tank must be gone")
	}
	if g.Units.ByID.Get(roadNext.ID) != roadNext || g.Units.ByID.Get(other.ID) != other {
		t.Error("the next cell must be untouched")
	}
	if got := g.TeamStats[0].Casualties[assets.Road]; got != 0 {
		t.Errorf("road Casualties = %d, want 0", got)
	}
	if got := g.TeamStats[0].UnitsLost; got != 1 {
		t.Errorf("UnitsLost = %d, want 1 (the tank only)", got)
	}
}

// TestBuildingDeathRemovesFourPlatforms: a 2x2 building on four platforms takes them all, uncounted.
func TestBuildingDeathRemovesFourPlatforms(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{{5, 5}: true, {6, 5}: true, {5, 6}: true, {6, 6}: true})
	id := 20
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			addCover(g, id, assets.WtrPltfm, 5+dx, 5+dy)
			id++
		}
	}
	bld := addBuilding(g, 1, 5, 5)

	g.DestroyUnit(bld)

	for _, u := range g.Units.Units() {
		if u.Type == assets.WtrPltfm {
			t.Errorf("platform id %d at (%d,%d) survived", u.ID, u.GridX, u.GridY)
		}
	}
	if got := g.TeamStats[0].Casualties[assets.WtrPltfm]; got != 0 {
		t.Errorf("platform Casualties = %d, want 0", got)
	}
	if got := g.TeamStats[0].BuildingsLost; got != 1 {
		t.Errorf("BuildingsLost = %d, want 1", got)
	}
}

// TestStrandedTransportTakesCargo: a stranded transport is destroyed and its cargo counted too.
func TestStrandedTransportTakesCargo(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{{5, 5}: true})
	bridge := addCover(g, 2, assets.Bridge, 5, 5)
	transport := addTankTarget(g, 1, 5, 5, 0)
	cargo := addTankTarget(g, 3, 8, 8, 0)
	g.Units.Remove(cargo) // stored inside, off the map
	transport.Carried = []*game.UnitInfo{cargo}

	g.DestroyUnit(bridge)

	if g.Units.ByID.Get(transport.ID) != nil {
		t.Error("the stranded transport must be gone")
	}
	if got := g.TeamStats[0].UnitsLost; got != 2 {
		t.Errorf("UnitsLost = %d, want 2 (transport + cargo)", got)
	}
}
