// factorybuild_test.go : tests factory rosters and production rates.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
)

// TestFactoryRoster locks the production rosters for each factory: each factory type
// has a not empty list, and a type that is not a factory (a constructor / a tank) has none.
func TestFactoryRoster(t *testing.T) {
	for _, f := range []assets.UnitType{assets.LightPlt, assets.LandPlt, assets.AirPlt, assets.Shipyard, assets.TrainHal} {
		if len(factoryRoster(f)) == 0 {
			t.Errorf("factory %s has an empty roster", f.Tag())
		}
		/* // note: unused code in Go port
		if !IsFactoryType(f) {
			t.Errorf("IsFactoryType(%s) = false, want true", f.Tag())
		}
		*/
	}
	if factoryRoster(assets.Constrct) != nil {
		t.Error("a constructor is not a factory, want nil roster")
	}
	/* // note: unused code in Go port
	if IsFactoryType(assets.Tank) {
		t.Error("a tank is not a factory")
	}
	*/
}

// TestFactoryUnitName resolves a queued type's display name from the roster, and
// falls back to the type tag for a type the roster does not carry.
func TestFactoryUnitName(t *testing.T) {
	roster := factoryRoster(assets.LandPlt)
	if got := unitName(roster, assets.Tank); got != "Tank" {
		t.Errorf("unitName(LandPlt roster, Tank) = %q, want %q", got, "Tank")
	}
	if got := unitName(roster, assets.Scout); got != assets.Scout.Tag() {
		t.Errorf("unitName fallback = %q, want the type tag %q", got, assets.Scout.Tag())
	}
}

// TestFactoryRateMathReusesConstruct confirms the shared rate helpers work with a
// factory as the builder (the rate column reuses maxBuildRate / turnsAndCost).
func TestFactoryRateMathReusesConstruct(t *testing.T) {
	// A unit that takes 1 turn can only build at X1 (X2/X4 gated off), like CONBUILD.
	if got := build.MaxBuildRate(assets.LandPlt, 1, 100000); got != 1 {
		t.Errorf("maxBuildRate(LandPlt, 1 turn) = %d, want 1", got)
	}
	turns, cost := turnsAndCost(assets.LandPlt, assets.Tank, 1, 60, 5)
	if turns <= 0 || cost <= 0 {
		t.Errorf("turnsAndCost(LandPlt, Tank) = %d turns / %d cost, want positive", turns, cost)
	}
}

func TestFactoryQueueDeleteSelected(t *testing.T) {
	queue := []assets.UnitType{assets.Scout, assets.Tank, assets.Engineer}
	queue = appendFactoryQueue(queue, assets.Repair)
	if got, want := queue[3], assets.Repair; got != want {
		t.Fatalf("appendFactoryQueue tail = %s, want %s", got.Tag(), want.Tag())
	}

	var ok bool
	queue, ok = deleteFactoryQueueAt(queue, 1)
	if !ok {
		t.Fatal("deleteFactoryQueueAt selected row returned false")
	}
	want := []assets.UnitType{assets.Scout, assets.Engineer, assets.Repair}
	if len(queue) != len(want) {
		t.Fatalf("queue len = %d, want %d", len(queue), len(want))
	}
	for i := range want {
		if queue[i] != want[i] {
			t.Fatalf("queue[%d] = %s, want %s", i, queue[i].Tag(), want[i].Tag())
		}
	}

	queue, ok = deleteFactoryQueueAt(queue, 99)
	if ok {
		t.Fatal("out-of-range delete returned true")
	}
}
