// production_test.go : tests production.go.
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

// TestFactoryPauseKeepsQueue: STOP (PauseFactory) sets the rate to 0 and HOLDS the
// build (no progress, queue KEPT); START resumes and the unit completes.
func TestFactoryPauseKeepsQueue(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(3)
	f := placeFactory(g, 4, 2)

	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)
	if got := g.ProductionState(f); got != ProdRunning {
		t.Fatalf("fresh queued factory state = %v, want ProdRunning", got)
	}
	g.DrainProductionEvents() // clear the auto power on ProdStarted from queuing

	// Pause: rate 0, queue kept, last rate remembered, a Stopped event emitted.
	g.PauseFactory(f)
	if f.BuildRate != 0 || f.LastBuildRate != 1 {
		t.Fatalf("after pause: BuildRate=%d LastBuildRate=%d, want 0/1", f.BuildRate, f.LastBuildRate)
	}
	if got := g.ProductionState(f); got != ProdPaused {
		t.Errorf("paused factory state = %v, want ProdPaused", got)
	}
	if evs := g.DrainProductionEvents(); len(evs) != 1 || evs[0].Kind != ProdStopped {
		t.Fatalf("pause events = %+v, want one ProdStopped", evs)
	}

	// A paused factory makes no progress and never finishes.
	for turn := 0; turn < 8; turn++ {
		g.AdvanceBuilds(0)
	}
	if g.BuildReadyInfo(f) != nil {
		t.Error("a paused factory must not finish its build")
	}
	if q, _ := g.FactoryQueue(f); len(q) != 1 || q[0] != assets.Tank {
		t.Errorf("queue after pause = %v, want [Tank] (kept)", q)
	}

	// Start: resumes at the remembered rate and completes the tank.
	g.StartFactory(f)
	if f.BuildRate != 1 {
		t.Fatalf("after start: BuildRate=%d, want 1 (remembered)", f.BuildRate)
	}
	if evs := g.DrainProductionEvents(); len(evs) != 1 || evs[0].Kind != ProdStarted {
		t.Fatalf("start events = %+v, want one ProdStarted", evs)
	}
	buildUntilReady(t, g, f)
}

// TestFactoryStateIdleAndStalled: an empty factory is Idle; a queued factory whose
// complex pool runs dry is Stalled (raw starved).
func TestFactoryStateIdleAndStalled(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(2)
	f := stationaryBuilding(assets.LandPlt, 0, 4, 2)
	g.PlaceBuilding(f, 4, 2)

	if got := g.ProductionState(f); got != ProdIdle {
		t.Errorf("empty factory state = %v, want ProdIdle", got)
	}

	// Queue with no funding (no Storage, no pool material) -> it cannot draw raw, so
	// after a build pass it is RawStarved and reads as Stalled.
	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)
	if c := g.ComplexOf(f); c != nil {
		c.Material = 0
	}
	g.AdvanceBuilds(0)
	if !f.RawStarved {
		t.Error("a factory that could not draw raw should be RawStarved")
	}
	if got := g.ProductionState(f); got != ProdStalled {
		t.Errorf("raw-short factory state = %v, want ProdStalled", got)
	}
}

// TestColonyScorePlateauAndEdges: an operating Eco Sphere scores and fires
// ColonyStarted; powering it off plateaus the score and fires ColonyStopped.
func TestColonyScorePlateauAndEdges(t *testing.T) {
	g := newGame(t, 16, 16)
	eco := stationaryBuilding(assets.Greenhse, 0, 5, 5)
	eco.Flags = game.FlagBuilding | game.FlagStationary
	g.Units.Deploy(eco, 5, 5)

	// Operating: scores a point per turn and announces ColonyStarted once.
	g.accrueScore(0)
	if g.TeamPoints[0] != 1 {
		t.Fatalf("operating eco-sphere score = %d, want 1", g.TeamPoints[0])
	}
	evs := g.DrainProductionEvents()
	if len(evs) != 1 || evs[0].Kind != ColonyStarted || evs[0].Team != 0 {
		t.Fatalf("first-turn events = %+v, want one ColonyStarted for team 0", evs)
	}

	// A second operating turn: still scores, NO new edge event.
	g.accrueScore(0)
	if g.TeamPoints[0] != 2 {
		t.Fatalf("score after two turns = %d, want 2", g.TeamPoints[0])
	}
	if evs := g.DrainProductionEvents(); len(evs) != 0 {
		t.Fatalf("steady-state events = %+v, want none (edge already fired)", evs)
	}

	// The sphere banks each scored point as cargo too (klei ++storage beside
	// ++team_points), so its gauge shows the accumulated colony points.
	if eco.Storage != 2 {
		t.Errorf("eco-sphere Storage after two scoring turns = %d, want 2", eco.Storage)
	}

	// Power it off: score PLATEAUS and ColonyStopped fires once (global).
	eco.Order = game.OrderPowerOff
	g.accrueScore(0)
	if g.TeamPoints[0] != 2 {
		t.Errorf("score after the colony stopped = %d, want 2 (plateau)", g.TeamPoints[0])
	}
	if eco.Storage != 2 {
		t.Errorf("a stopped sphere's Storage = %d, want the plateaued 2", eco.Storage)
	}
	if evs := g.DrainProductionEvents(); len(evs) != 1 || evs[0].Kind != ColonyStopped {
		t.Fatalf("stop events = %+v, want one ColonyStopped", evs)
	}

	// Restart: scores again and announces ColonyStarted again.
	eco.Order = game.OrderPowerOn
	g.accrueScore(0)
	if g.TeamPoints[0] != 3 {
		t.Errorf("score after restart = %d, want 3", g.TeamPoints[0])
	}
	if eco.Storage != 3 {
		t.Errorf("eco-sphere Storage after restart = %d, want 3", eco.Storage)
	}
	if evs := g.DrainProductionEvents(); len(evs) != 1 || evs[0].Kind != ColonyStarted {
		t.Fatalf("restart events = %+v, want one ColonyStarted", evs)
	}
}
