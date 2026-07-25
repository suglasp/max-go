// build_test.go : tests construction orders, placement, and complex networks.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package build

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// TestBuildOrderTurnsLeft checks the "N turns left" readout: it rounds up by the
// build rate and bottoms out at 0 once the work is done.
func TestBuildOrderTurnsLeft(t *testing.T) {
	cases := []struct {
		progress, total, rate, want int
	}{
		{0, 12, 1, 12}, // fresh, normal rate
		{4, 12, 1, 8},  // a third done
		{0, 12, 2, 6},  // double rate halves the turns
		{0, 9, 2, 5},   // 9 turns at rate 2 rounds up to 5
		{0, 12, 4, 3},  // quad rate
		{12, 12, 1, 0}, // finished
		{15, 12, 1, 0}, // finished past total clamps to 0
	}
	for _, c := range cases {
		o := &BuildOrder{Progress: c.progress, TotalTurns: c.total, BuildRate: c.rate}
		if got := o.TurnsLeft(); got != c.want {
			t.Errorf("TurnsLeft(progress=%d total=%d rate=%d) = %d, want %d", c.progress, c.total, c.rate, got, c.want)
		}
	}
}

// allLand is a synthetic land surface for site validation tests.
type allLand struct{}

func (allLand) SurfaceAt(x, y int) uint8 {
	return world.SurfaceLand
}

// noUnits is an empty unit source (no occupancy blocking).
type noUnits struct{}

func (noUnits) At(x, y int) []*game.UnitInfo {
	return nil
}

func landGrid(w, h int) *access.Grid {
	return &access.Grid{W: w, H: h, Surf: allLand{}, Units: noUnits{}}
}

// product makes an unplaced unit of a type with the given flags and build time.
func product(t assets.UnitType, flags game.Flags, turns int) *game.UnitInfo {
	u := game.NewUnitInfo(t, 0, 0, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribTurns, turns)

	return u
}

// placed makes a placed building of a type at a cell for network tests.
func placed(t assets.UnitType, team, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, 0, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary
	u.GridX, u.GridY = gx, gy

	return u
}

func TestFootprint(t *testing.T) {
	if Footprint(game.FlagBuilding) != 2 {
		t.Error("a building should occupy 2x2")
	}

	if Footprint(game.FlagMobileLandUnit) != 1 {
		t.Error("a mobile unit should occupy 1x1")
	}
}

func TestSiteValid(t *testing.T) {
	g := landGrid(8, 8)
	d := access.UnitDesc{Type: assets.LandPlt, Flags: game.FlagBuilding, LandType: world.SurfaceLand}

	if !SiteValid(g, d, 0, 2, 2) {
		t.Error("a 2x2 building should fit on open land")
	}

	// Against the right edge the 2x2 footprint runs off the map.
	if SiteValid(g, d, 0, 7, 2) {
		t.Error("a 2x2 building must not fit with its footprint off the map edge")
	}
}

func TestBuildCost(t *testing.T) {
	// A building costs the constructor base rate (2) each turn to build.
	bld := product(assets.Radar, game.FlagBuilding|game.FlagStationary, 6)
	if got := Cost(bld); got != 12 { // 2 * 6
		t.Errorf("building cost = %d, want 12", got)
	}

	// A vehicle costs the factory base rate (3) per turn.
	tank := product(assets.Tank, game.FlagMobileLandUnit, 5)
	if got := Cost(tank); got != 15 { // 3 * 5
		t.Errorf("vehicle cost = %d, want 15", got)
	}

	// Infantry costs the training hall rate (1).
	inf := product(assets.Infantry, game.FlagMobileLandUnit, 4)
	if got := Cost(inf); got != 4 { // 1 * 4
		t.Errorf("infantry cost = %d, want 4", got)
	}
}

func TestBuildOrderProducesUnit(t *testing.T) {
	tank := product(assets.Tank, game.FlagMobileLandUnit, 5) // cost 15, 5 turns @ rate 1
	o := NewBuildOrder(nil, tank, 3, 3, 1)
	if o.CostPerTurn() != 3 {
		t.Fatalf("cost/turn = %d, want 3", o.CostPerTurn())
	}

	pool := 100
	turns := 0
	for !o.Done() {
		if !o.Advance(&pool) {
			t.Fatal("advance stalled with material available")
		}
		turns++
	}

	if turns != 5 {
		t.Errorf("took %d turns, want 5", turns)
	}

	if spent := 100 - pool; spent != 15 {
		t.Errorf("spent %d material, want 15 (total cost)", spent)
	}

	if o.Product != tank {
		t.Error("finished order should yield the product unit")
	}
}

func TestBuildOrderStallsWithoutMaterial(t *testing.T) {
	tank := product(assets.Tank, game.FlagMobileLandUnit, 5)
	o := NewBuildOrder(nil, tank, 0, 0, 1)
	pool := 5 // enough for 1 turn (3), then 2 left < 3

	if !o.Advance(&pool) || pool != 2 {
		t.Fatalf("first turn should consume 3, pool=%d", pool)
	}

	if o.Advance(&pool) {
		t.Error("second turn should stall (2 < 3 cost)")
	}

	if o.Progress != 1 {
		t.Errorf("progress = %d, want 1 (stalled)", o.Progress)
	}
}

func TestBuildRateScalesCostAndSpeed(t *testing.T) {
	// At rate 2 a vehicle builds twice as fast but the raw draw each turn grows
	// faster than linear: RawConsumptionRate(LandPlt, 2) = 12 (not 3*2=6), so a
	// rushed build costs more total material (Cargo_GetRawConsumptionRate scaling).
	tank := product(assets.Tank, game.FlagMobileLandUnit, 4) // 4 turns, X1 cost 12
	o := NewBuildOrder(nil, tank, 0, 0, 2)
	if o.CostPerTurn() != 12 {
		t.Fatalf("cost/turn @rate2 = %d, want 12", o.CostPerTurn())
	}

	pool := 100 // ample, so the rate is never throttled down
	turns := 0
	for !o.Done() {
		o.Advance(&pool)
		turns++
	}

	if turns != 2 {
		t.Errorf("rate 2 build took %d turns, want 2", turns)
	}

	if spent := 100 - pool; spent != 24 {
		t.Errorf("spent %d, want 24 (X2 costs double the X1 total)", spent)
	}
}

// TestBuildRateThrottlesDownOnLowStorage checks that an X2 build drops to X1
// when its material pool cannot sustain the faster rate. It must finish without
// overspending or stalling with excess material.
func TestBuildRateThrottlesDownOnLowStorage(t *testing.T) {
	// Heavy factory analogue: 20 turn building, constructor rate (2 per X1 turn).
	bld := product(assets.Radar, game.FlagBuilding|game.FlagStationary, 20)
	o := NewBuildOrder(nil, bld, 0, 0, 2) // request X2
	pool := 60                            // like a full 60-cargo constructor
	for !o.Done() {
		if !o.Advance(&pool) {
			t.Fatalf("stalled with pool=%d, progress=%d", pool, o.Progress)
		}
	}
	// The build finishes after dropping from X2 to X1 and leaves little material.
	if pool > 6 {
		t.Errorf("finished with pool=%d, want it drained near empty (<=6)", pool)
	}
}

func TestNetworkIsolatedBuildingNewComplex(t *testing.T) {
	n := NewNetwork(0, 1)
	a := placed(assets.MiningSt, 0, 2, 2)
	c := n.Add(a)

	if c.ID != 1 || c.Buildings != 1 {
		t.Errorf("isolated building: complex id %d buildings %d, want 1/1", c.ID, c.Buildings)
	}

	if len(n.Complexes) != 1 {
		t.Errorf("network has %d complexes, want 1", len(n.Complexes))
	}
}

func TestNetworkAdjacentBuildingsJoin(t *testing.T) {
	n := NewNetwork(0, 1)
	a := placed(assets.MiningSt, 0, 2, 2) // occupies (2,2)..(3,3)
	b := placed(assets.PowGen, 0, 4, 2)   // occupies (4,2)..(5,3): touches a on the east edge
	ca := n.Add(a)
	cb := n.Add(b)

	if ca != cb {
		t.Error("adjacent buildings should share one complex")
	}

	if cb.Buildings != 2 || len(n.Complexes) != 1 {
		t.Errorf("joined complex buildings %d, network complexes %d, want 2/1", cb.Buildings, len(n.Complexes))
	}
}

func TestNetworkBridgeMergesToLowestID(t *testing.T) {
	n := NewNetwork(0, 1)
	// Two separate complexes, ids 1 and 2.
	a := placed(assets.MiningSt, 0, 0, 0) // (0,0)..(1,1)  complex 1
	b := placed(assets.PowGen, 0, 6, 0)   // (6,0)..(7,1)  complex 2
	n.Add(a)
	c2 := n.Add(b)

	if c2.ID != 2 {
		t.Fatalf("second isolated complex id = %d, want 2", c2.ID)
	}

	if len(n.Complexes) != 2 {
		t.Fatalf("expected 2 separate complexes, got %d", len(n.Complexes))
	}

	// A bridge touching both merges them into the lowest id (1).
	bridge1 := placed(assets.Cnct4W, 0, 2, 0) // touches a's east edge
	bridge2 := placed(assets.Cnct4W, 0, 4, 0) // between, touches bridge1 and b? lay a chain
	n.Add(bridge1)
	n.Add(bridge2)
	merged := n.Add(placed(assets.Cnct4W, 0, 5, 0)) // now touches bridge2 and b
	_ = merged

	// After the chain connects, every building shares complex id 1.
	if n.ComplexOf(a).ID != 1 || n.ComplexOf(b).ID != 1 {
		t.Errorf("after bridging, ids = %d/%d, want both 1", n.ComplexOf(a).ID, n.ComplexOf(b).ID)
	}

	if len(n.Complexes) != 1 {
		t.Errorf("after merge network has %d complexes, want 1", len(n.Complexes))
	}
}

func TestNetworkRemoveEmptiesComplex(t *testing.T) {
	n := NewNetwork(0, 1)
	a := placed(assets.MiningSt, 0, 2, 2)
	n.Add(a)
	n.Remove(a)

	if len(n.Complexes) != 0 {
		t.Errorf("removing the only building should drop the complex, have %d", len(n.Complexes))
	}

	if n.ComplexOf(a) != nil {
		t.Error("removed building should have no complex")
	}
}
