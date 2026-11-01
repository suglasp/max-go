// alloc_test.go : tests mining allocation and station capacity.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package economy

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestAllocAddReduce checks AddMining/ReduceMining respect station capacity and
// deposits and move extraction between resources.
func TestAllocAddReduce(t *testing.T) {
	// One mining station with deposits in distinct cells of its area (a cell
	// holds a single deposit type): raw=10, fuel=5, gold=3.
	rmap := NewResourceMap(8, 8)
	// The mining area is gx..gx+radius / gy..gy+radius (down right footprint).
	rmap.SetDeposit(4, 4, CargoRaw, 10)
	rmap.SetDeposit(5, 4, CargoFuel, 5)
	rmap.SetDeposit(4, 5, CargoGold, 3)
	u := game.NewUnitInfo(assets.MiningSt, 0, 1, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary
	u.GridX, u.GridY = 4, 4
	c := &Complex{Members: []*game.UnitInfo{u}}

	// Add 6 raw: a station holds 16 total, deposit 10 -> all 6 fit.
	if got := c.AddMining(rmap, CargoRaw, 6); got != 6 || u.RawMining != 6 {
		t.Fatalf("AddMining raw 6 = %d, raw=%d", got, u.RawMining)
	}
	// Add 5 fuel: deposit 5, capacity left 16-6=10 -> all 5 fit.
	if got := c.AddMining(rmap, CargoFuel, 5); got != 5 || u.FuelMining != 5 {
		t.Fatalf("AddMining fuel 5 = %d, fuel=%d", got, u.FuelMining)
	}
	// Add 9 gold: deposit only 3, so only 3 fit (and 16-11=5 capacity left).
	if got := c.AddMining(rmap, CargoGold, 9); got != 3 || u.GoldMining != 3 {
		t.Fatalf("AddMining gold capped by deposit = %d, gold=%d", got, u.GoldMining)
	}
	if u.TotalMining != 14 {
		t.Errorf("total mining = %d, want 14", u.TotalMining)
	}

	// Reduce 4 raw.
	if got := c.ReduceMining(CargoRaw, 4); got != 4 || u.RawMining != 2 {
		t.Errorf("ReduceMining raw 4 = %d, raw=%d", got, u.RawMining)
	}

	// MiningTotals reflects the state + free capacity (16 - 2 - 5 - 3 = 6).
	raw, fuel, gold, free := c.MiningTotals()
	if raw != 2 || fuel != 5 || gold != 3 || free != 6 {
		t.Errorf("MiningTotals = %d/%d/%d free %d, want 2/5/3 free 6", raw, fuel, gold, free)
	}
}

// TestMiningHeadroomRespectsPerStationLimits reproduces the Allocate screen
// hazard tape bug: with more than one mining station, a
// pooled free-slot total overstates a resource's real headroom once the
// station that HOLDS the extra deposit is itself full, even though a
// DIFFERENT station has plenty of spare slots it simply cannot use for that
// resource (its own area has none of it).
func TestMiningHeadroomRespectsPerStationLimits(t *testing.T) {
	rmap := NewResourceMap(16, 16)

	// Station A: a 20 unit fuel deposit in its area (two cells, since a single
	// cell reads capped at 16), but its 16 slots are already FULL (fuel=6,
	// raw=10): zero spare slots, even though 14 more fuel exists in the ground.
	rmap.SetDeposit(4, 4, CargoFuel, 16)
	rmap.SetDeposit(5, 4, CargoFuel, 4)
	a := game.NewUnitInfo(assets.MiningSt, 0, 1, 0)
	a.Flags = game.FlagBuilding | game.FlagStationary
	a.GridX, a.GridY = 4, 4
	a.FuelMining = 6
	a.RawMining = 10

	// Station B: no fuel deposit at all nearby, mining only 2 of its 16 slots
	// -> 14 spare slots that a naive pooled total would credit to fuel, but
	// none of them can ever mine fuel that is not there.
	rmap.SetDeposit(10, 10, CargoRaw, 16)
	b := game.NewUnitInfo(assets.MiningSt, 0, 2, 0)
	b.Flags = game.FlagBuilding | game.FlagStationary
	b.GridX, b.GridY = 10, 10
	b.RawMining = 2

	c := &Complex{Members: []*game.UnitInfo{a, b}}

	if got := c.MiningHeadroom(rmap, CargoFuel); got != 0 {
		t.Errorf("MiningHeadroom(fuel) = %d, want 0 (A has no spare slots, B has no fuel deposit)", got)
	}

	// The naive pooled MiningTotals free count (0 + 14 = 14) is NOT the right
	// answer here - recorded so a future change does not "fix" this test by
	// reintroducing the pooled formula.
	if _, _, _, free := c.MiningTotals(); free != 14 {
		t.Fatalf("test setup drifted: pooled free = %d, want 14 (the overstated total this fix avoids)", free)
	}
}
