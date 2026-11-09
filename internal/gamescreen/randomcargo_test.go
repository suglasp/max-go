// randomcargo_test.go : tests random bench starting cargo allocation rules.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestSetStarterBuilderCargo checks the founder builders deploy at the partial
// default (first Constructor 40, first Engineer 20) while any extra builder of
// either kind starts empty.
func TestSetStarterBuilderCargo(t *testing.T) {
	constructorsSeen, engineersSeen := 0, 0

	c1 := &game.UnitInfo{Type: assets.Constrct, Storage: 999}
	e1 := &game.UnitInfo{Type: assets.Engineer, Storage: 999}
	c2 := &game.UnitInfo{Type: assets.Constrct, Storage: 999}
	e2 := &game.UnitInfo{Type: assets.Engineer, Storage: 999}

	// The first of each kind takes its default; the second is zeroed.
	setStarterBuilderCargo(c1, &constructorsSeen, &engineersSeen)
	setStarterBuilderCargo(e1, &constructorsSeen, &engineersSeen)
	setStarterBuilderCargo(c2, &constructorsSeen, &engineersSeen)
	setStarterBuilderCargo(e2, &constructorsSeen, &engineersSeen)

	if c1.Storage != startConstructorRaw {
		t.Errorf("first constructor cargo = %d, want %d", c1.Storage, startConstructorRaw)
	}
	if e1.Storage != startEngineerRaw {
		t.Errorf("first engineer cargo = %d, want %d", e1.Storage, startEngineerRaw)
	}
	if c2.Storage != 0 || e2.Storage != 0 {
		t.Errorf("extra builders cargo = %d / %d, want 0 / 0", c2.Storage, e2.Storage)
	}
}

// TestSetPurchasedCargo checks a unit landed from a purchased loadout carries the
// exact load the player set on the purchase screen, clamped to its own storage
// capacity, and that a negative or missing entry lands the unit empty.
func TestSetPurchasedCargo(t *testing.T) {
	constructor := &game.UnitInfo{Type: assets.Constrct, Storage: 999}
	constructor.Values = &game.UnitValues{}
	constructor.Values.Set(game.AttribStorage, 60)

	// The purchased load lands as set.
	setPurchasedCargo(constructor, startConstructorRaw)
	if constructor.Storage != startConstructorRaw {
		t.Errorf("purchased cargo = %d, want %d", constructor.Storage, startConstructorRaw)
	}

	// A load over the unit's capacity clamps to it; a negative one lands empty.
	setPurchasedCargo(constructor, 500)
	if constructor.Storage != 60 {
		t.Errorf("over capacity cargo = %d, want the 60 cap", constructor.Storage)
	}

	setPurchasedCargo(constructor, -5)
	if constructor.Storage != 0 {
		t.Errorf("negative cargo = %d, want 0", constructor.Storage)
	}

	// A roster longer than the cargo list lands its tail empty.
	if got := cargoAt([]int{40, 20}, 4); got != 0 {
		t.Errorf("cargoAt past the end = %d, want 0", got)
	}
	if got := cargoAt([]int{40, 20}, 1); got != 20 {
		t.Errorf("cargoAt(1) = %d, want 20", got)
	}
}

// TestStartSupplyRawFollowsRichness checks the landing raw buffer tracks the map's RAW
// richness as observed in vanilla: a poor map lands 10, an average one 13,
// and a rich one none at all. A game with no level set (both random benches) lands the
// average buffer.
func TestStartSupplyRawFollowsRichness(t *testing.T) {
	for level, want := range map[int]int{0: startSupplyRawPoor, 1: startSupplyRawAverage, 2: startSupplyRawRich} {
		if got := startSupplyRawFor(level); got != want {
			t.Errorf("raw level %d lands %d, want %d", level, got, want)
		}
	}

	if got := startSupplyRawFor(-1); got != startSupplyRawAverage {
		t.Errorf("an unset level lands %d, want the average %d", got, startSupplyRawAverage)
	}

	s := &Screen{}
	if got := s.rawResourceLevel(); got != 1 {
		t.Errorf("a screen with no resource levels reports raw level %d, want 1 (average)", got)
	}

	s.SetResourceLevels(0, 2, 1)
	if got := s.rawResourceLevel(); got != 0 {
		t.Errorf("after Poor raw, rawResourceLevel = %d, want 0", got)
	}
}

// TestSeedRawStoreDisplayMirrorsPool checks the turn 0 mine bar seed: a 13 raw
// pool shows 13/25 on the lone mining station, and an over cap pool clamps the
// store to its 25 capacity (the remainder stays in the pool).
func TestSeedRawStoreDisplayMirrorsPool(t *testing.T) {
	_, g := newTestScreen(t, 16)

	mine := mkBuilding(assets.MiningSt, 0, 1, 8, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 25)
	g.PlaceBuilding(mine, 8, 8)
	c := g.ComplexOf(mine)
	if c == nil {
		t.Fatal("mine has no complex")
	}

	// A medium richness start: 13 raw in the pool reads 13/25 on the mine.
	c.Material = startSupplyRaw
	seedRawStoreDisplay(c)
	if mine.Storage != startSupplyRaw {
		t.Errorf("mine turn-0 stock = %d, want %d (13/25)", mine.Storage, startSupplyRaw)
	}

	// An over cap pool clamps the store at its 25 capacity.
	c.Material = 30
	seedRawStoreDisplay(c)
	if mine.Storage != 25 {
		t.Errorf("mine stock with a 30 pool = %d, want 25 (capped)", mine.Storage)
	}
}
