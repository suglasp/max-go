// factorybuild_test.go : tests factory queue build and repeat build logic.
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

// factoryUnitMaker returns a Game.MakeUnit closure that mints a fully valued
// mobile unit taking `turns` build turns and able to drive (Speed) so it can roll
// out of the factory. IDs are 0 (completeFactoryOutput assigns the real one).
func factoryUnitMaker(turns int) UnitFactory {
	return func(team int, t assets.UnitType, flags game.Flags) *game.UnitInfo {
		u := game.NewUnitInfo(t, team, 0, 0)
		u.Flags = flags
		u.Values = &game.UnitValues{}
		u.Values.Set(game.AttribTurns, turns)
		u.Values.Set(game.AttribHits, 10)
		u.Values.Set(game.AttribSpeed, 8) // enough budget to drive out of the plant
		return u
	}
}

// placeFactory puts a powered Heavy Vehicle Plant on the grid holding plenty of
// its own material so its production never stalls for funding.
func placeFactory(g *Game, x, y int) *game.UnitInfo {
	f := stationaryBuilding(assets.LandPlt, 0, x, y)
	g.PlaceBuilding(f, x, y)
	f.Storage = 1000 // own cargo funds every build turn (buildPool prefers it)
	return f
}

// countType returns how many mapped units of type t the game holds.
func countType(g *Game, t assets.UnitType) int {
	n := 0
	for _, u := range g.Units.Units() {
		if u.Type == t {
			n++
		}
	}
	return n
}

// buildUntilReady advances construction until the factory's produced unit parks
// (build ready), bounded against a runaway.
func buildUntilReady(t *testing.T, g *Game, f *game.UnitInfo) {
	t.Helper()
	for turn := 0; turn < 12 && g.BuildReadyInfo(f) == nil; turn++ {
		g.AdvanceBuilds(0)
	}
	if g.BuildReadyInfo(f) == nil {
		t.Fatal("factory never produced a build-ready unit")
	}
}

// exitFactoryReady drives the factory's parked output unit off the plant via its
// first exit marker and resolves the move, freeing the factory to build the next
// queued unit (Tick.sweepFactoryReady).
func exitFactoryReady(t *testing.T, g *Game, f *game.UnitInfo) {
	t.Helper()
	markers := g.ExitMarkers(f)
	if len(markers) == 0 {
		t.Fatal("no exit markers - the produced unit is boxed in")
	}
	if !g.ExitBuildSite(f, markers[0][0], markers[0][1]) {
		t.Fatal("ExitBuildSite refused a valid marker")
	}
	g.ResolveMovement() // drive the unit out; the sweep then frees the plant
}

// TestFactoryQueueAdvances: the head unit parks build ready ON the factory, the
// queue advances, and the next unit only starts once the parked unit drives out.
func TestFactoryQueueAdvances(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(2)
	f := placeFactory(g, 4, 2)

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank, assets.Scout}, 1, false)

	buildUntilReady(t, g, f)
	if br := g.BuildReadyInfo(f); br == nil || br.Product != assets.Tank || !br.Factory {
		t.Fatalf("head not parked as a factory build-ready Tank: %+v", br)
	}
	if q, _ := g.FactoryQueue(f); len(q) != 1 || q[0] != assets.Scout {
		t.Errorf("queue after the head finished = %v, want [Scout]", q)
	}
	if g.BuildOrderFor(f) != nil {
		t.Error("the next unit started before the Tank exited")
	}

	exitFactoryReady(t, g, f) // drive the Tank out -> the factory starts the Scout
	if g.BuildReadyInfo(f) != nil {
		t.Error("factory still build-ready after the Tank drove out")
	}
	if g.BuildOrderFor(f) == nil {
		t.Error("the Scout build did not start after the Tank left")
	}

	buildUntilReady(t, g, f) // the Scout finishes + parks
	exitFactoryReady(t, g, f)

	if got := countType(g, assets.Tank); got != 1 {
		t.Errorf("tanks produced = %d, want 1", got)
	}
	if got := countType(g, assets.Scout); got != 1 {
		t.Errorf("scouts produced = %d, want 1", got)
	}
	if q, _ := g.FactoryQueue(f); len(q) != 0 {
		t.Errorf("queue not drained: %v", q)
	}
}

// TestFactoryWithConstructorFlagIsNotPosed verifies that StartBuild does not give a stationary factory the mobile constructor build pose.
// Retail factories may carry CONSTRUCTOR_UNIT because they produce units.
func TestFactoryWithConstructorFlagIsNotPosed(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(2)
	f := placeFactory(g, 4, 2)
	f.Flags |= game.FlagConstructorUnit // a mission factory carries the C++ CONSTRUCTOR_UNIT flag
	wantX, wantY := f.X, f.Y

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false) // starts the head build

	if f.Building {
		t.Error("a stationary factory must not enter the build pose (Building set)")
	}
	if f.X != wantX || f.Y != wantY {
		t.Errorf("factory pixel pos moved to (%d,%d); want its slab (%d,%d)", f.X, f.Y, wantX, wantY)
	}
	if g.BuildOrderFor(f) == nil {
		t.Error("the factory should still have started its build order")
	}
}

// TestFactoryWithConstructorFlagProducesAndExits verifies that a stationary factory carrying CONSTRUCTOR_UNIT still parks and releases its output as a factory.
func TestFactoryWithConstructorFlagProducesAndExits(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(2)
	f := placeFactory(g, 4, 2) // a 2x2 Light/Heavy Vehicle Plant
	f.Flags |= game.FlagConstructorUnit

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)
	buildUntilReady(t, g, f)

	br := g.BuildReadyInfo(f)
	if br == nil || !br.Factory || br.Product != assets.Tank {
		t.Fatalf("flagged factory did not park a factory-ready Tank: %+v", br)
	}
	if br.Footprint != 2 {
		t.Errorf("build-ready footprint = %d, want 2 (the plant), not the product footprint", br.Footprint)
	}
	product := br.Building
	if product == nil || !product.HiddenUntilExpand {
		t.Fatalf("produced unit should be parked hidden on the plant: %+v", product)
	}

	exitFactoryReady(t, g, f) // click a marker: the unit materialises and drives out

	if g.BuildReadyInfo(f) != nil {
		t.Error("factory still build-ready after the Tank drove out (plant not freed)")
	}
	if got := countType(g, assets.Tank); got != 1 {
		t.Errorf("tanks on the map = %d, want 1 (the unit was lost)", got)
	}
	if product.HiddenUntilExpand {
		t.Error("produced unit is still hidden after the exit (never materialised)")
	}
	// It drove out of the 2x2 plant footprint at (f.GridX, f.GridY).
	onPlant := product.GridX >= f.GridX && product.GridX < f.GridX+2 &&
		product.GridY >= f.GridY && product.GridY < f.GridY+2
	if onPlant {
		t.Errorf("produced unit still sits on the plant footprint at (%d,%d)", product.GridX, product.GridY)
	}
}

// TestSetFactoryQueuePowersOnFromRetailOrder verifies that queuing production changes retail OrderBuild to OrderPowerOn.
// Economy treats OrderBuild as active, but factory animation, audio, and the popup power toggle require OrderPowerOn.
func TestSetFactoryQueuePowersOnFromRetailOrder(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(2)
	f := placeFactory(g, 4, 2)
	f.Order = game.OrderBuild           // a mission factory carries the retail producing order
	f.Flags |= game.FlagConstructorUnit // ... and the retail CONSTRUCTOR_UNIT flag

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)

	if f.Order != game.OrderPowerOn {
		t.Errorf("queued factory Order = %#x, want OrderPowerOn (%#x) so the running frame/hum/popup read it as on", f.Order, game.OrderPowerOn)
	}
	if !g.FactoryProducing(f) {
		t.Error("factory should read as producing after the queue commit")
	}
}

// TestFactoryRepeatCycles: with Repeat on, each finished unit is appended again to
// the tail, so producing+exiting keeps turning out units and the queue never
// drains.
func TestFactoryRepeatCycles(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(1)
	f := placeFactory(g, 4, 2)

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank, assets.Scout}, 1, true)
	for cycle := 0; cycle < 3; cycle++ {
		buildUntilReady(t, g, f)
		exitFactoryReady(t, g, f)
	}

	// Cycle order Tank, Scout, Tank -> 2 tanks, 1 scout; the queue still cycles.
	if got := countType(g, assets.Tank); got < 2 {
		t.Errorf("tanks produced = %d, want >= 2 (repeat keeps building)", got)
	}
	if got := countType(g, assets.Scout); got < 1 {
		t.Errorf("scouts produced = %d, want >= 1", got)
	}
	if q, rep := g.FactoryQueue(f); len(q) != 2 || !rep {
		t.Errorf("repeat queue = %d items repeat=%v, want 2 items repeat=true", len(q), rep)
	}
}

// TestFactoryEmptyCommitHalts: an empty Done drops the active build order and
// clears the queue.
func TestFactoryEmptyCommitHalts(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(3)
	f := placeFactory(g, 4, 2)

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)
	if len(g.builds) != 1 {
		t.Fatalf("active orders after commit = %d, want 1", len(g.builds))
	}
	g.SetFactoryQueue(f, nil, 1, false) // empty Done -> halt

	if len(g.builds) != 0 {
		t.Errorf("active orders after empty commit = %d, want 0 (halted)", len(g.builds))
	}
	if q, _ := g.FactoryQueue(f); len(q) != 0 {
		t.Errorf("queue after halt = %d, want 0", len(q))
	}
}

// TestFactoryOutputBoxedInParks: a 2x2 plant filling a 2x2 grid still parks its
// finished unit build ready, but with NO exit markers (nowhere to drive out) -
// the unit waits until the player clears space, instead of being lost.
func TestFactoryOutputBoxedInParks(t *testing.T) {
	g := newGame(t, 2, 2)
	g.Resources = economy.NewResourceMap(2, 2)
	g.MakeUnit = factoryUnitMaker(1)
	f := placeFactory(g, 0, 0) // 2x2 footprint fills the whole grid

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)
	buildUntilReady(t, g, f)

	if br := g.BuildReadyInfo(f); br == nil || br.Product != assets.Tank {
		t.Fatalf("boxed-in factory did not park its output: %+v", br)
	}
	if m := g.ExitMarkers(f); len(m) != 0 {
		t.Errorf("boxed-in factory offered %d exit markers, want 0", len(m))
	}
}

// TestFactoryDeployCell checks the deploy cell search the AI exit uses.
func TestFactoryDeployCell(t *testing.T) {
	g := newGame(t, 8, 8)
	f := placeFactory(g, 3, 3)
	x, y, ok := g.FactoryDeployCell(f, game.FlagMobileLandUnit|game.FlagSelectable)
	if !ok {
		t.Fatal("no deploy cell found on an open map")
	}
	if len(g.Units.Cells.At(x, y)) != 0 {
		t.Errorf("deploy cell (%d,%d) is occupied", x, y)
	}
}

// TestFactoryQueueProducesFromComplex verifies that a powered mining and factory complex funds queued production from pooled material and deploys both units in order.
func TestFactoryQueueProducesFromComplex(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.Resources.SetDeposit(2, 2, economy.CargoRaw, 16)
	g.Resources.SetDeposit(3, 2, economy.CargoRaw, 16)
	g.MakeUnit = factoryUnitMaker(2)

	mine := stationaryBuilding(assets.MiningSt, 0, 2, 2)
	g.PlaceBuilding(mine, 2, 2)
	factory := stationaryBuilding(assets.LandPlt, 0, 4, 2)
	g.PlaceBuilding(factory, 4, 2)
	gen := stationaryBuilding(assets.PowGen, 0, 2, 4)
	gen.Flags = game.FlagStationary // 1x1
	g.PlaceBuilding(gen, 2, 4)
	// A second generator: one PowGen powers only one consumer and mining is served
	// first, so a single generator would starve the factory (the power deficit rule).
	gen2 := stationaryBuilding(assets.PowGen, 0, 3, 4)
	gen2.Flags = game.FlagStationary // 1x1
	g.PlaceBuilding(gen2, 3, 4)
	for _, cx := range g.Complexes {
		if cx.Team == 0 {
			cx.Fuel = 50
			cx.Material = 100
		}
	}

	g.SetFactoryQueue(factory, []assets.UnitType{assets.Tank, assets.Scout}, 1, false)
	for produced := 0; produced < 2; produced++ {
		for turn := 0; turn < 16 && g.BuildReadyInfo(factory) == nil; turn++ {
			g.BeginTeamTurn(0)
			g.EndTeamTurn(0)
			g.State.AdvanceTurn()
		}
		if g.BuildReadyInfo(factory) == nil {
			t.Fatalf("unit %d never finished", produced+1)
		}
		exitFactoryReady(t, g, factory)
	}

	if got := countType(g, assets.Tank); got != 1 {
		t.Errorf("tanks produced from the complex = %d, want 1", got)
	}
	if got := countType(g, assets.Scout); got != 1 {
		t.Errorf("scouts produced from the complex = %d, want 1", got)
	}
}
