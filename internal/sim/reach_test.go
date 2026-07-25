// reach_test.go : tests reach.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/units"
)

// TestReachabilityRejectsAcrossWater verifies the O(1) component precheck for land units separated by open water.
// Same-landmass and amphibious routes remain valid, and a water platform invalidates the cache and reconnects the landmasses.
func TestReachabilityRejectsAcrossWater(t *testing.T) {
	const waterX = 3
	g := newMixedGame(t, 7, 5, waterX) // cols 0-2 land, col 3 water, cols 4-6 land

	tank := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribSpeed, 12)
	g.Units.Deploy(tank, 1, 2) // left landmass

	// The two cells are in different terrain components for a land unit.
	if d := g.unitDesc(tank); g.reachable(d, 0, 1, 2, 5, 2) {
		t.Fatal("reachable() reported (1,2)->(5,2) connected for a land unit across water")
	}
	// So an order across the water is refused (no flood, no route).
	if g.OrderMove(tank, 5, 2) {
		t.Error("Tank routed across open water; the reachability precheck should reject it")
	}
	// A target on the same landmass still routes normally.
	if !g.OrderMove(tank, 0, 1) {
		t.Error("Tank should reach a cell on its own landmass")
	}

	// An amphibious Surveyor (land_type includes water) treats the whole map as one
	// component, so the precheck must NOT reject a cross water order.
	surv := game.NewUnitInfo(assets.Surveyor, 0, 2, 0)
	surv.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	surv.Values = &game.UnitValues{}
	surv.Values.Set(game.AttribSpeed, 30)
	g.Units.Deploy(surv, 1, 3)
	if !g.OrderMove(surv, 5, 3) {
		t.Error("amphibious Surveyor should route across the water column")
	}

	// Bridge the water column with a platform: PlaceBuilding invalidates the cached
	// components, so the tank can now cross to the right landmass.
	plat := game.NewUnitInfo(assets.WtrPltfm, 0, 3, 0)
	plat.Flags = game.FlagStationary | game.FlagSelectable
	g.PlaceBuilding(plat, waterX, 2) // (3,2) becomes land passable
	if d := g.unitDesc(tank); !g.reachable(d, 0, 1, 2, 5, 2) {
		t.Fatal("a water platform should reconnect the two landmasses for a land unit")
	}
	if !g.OrderMove(tank, 5, 2) {
		t.Error("Tank should reach the right landmass once a platform bridges the water")
	}
}

// BenchmarkBuildReachLabels measures the one new cost S2 adds: building the
// connected component labels for the whole 112x112 grid once per invalidation
// (then every move order's reachability check is an O(1) lookup). It must be
// comfortably cheaper than the flood it replaces (a single unreachable A* was
// ~0.59 ms after S1), since it is reused across all of a turn's move orders.
func BenchmarkBuildReachLabels(b *testing.B) {
	const w, h, waterX = 112, 112, 56
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: mixedSurf{waterX: waterX}, Units: mgr.Cells}
	d := access.UnitDesc{
		Type:     assets.Tank,
		Flags:    game.FlagMobileLandUnit,
		LandType: access.LandTypeForUnit(assets.Tank, game.FlagMobileLandUnit),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildReachLabels(grid, d, 0, w, h)
	}
}
