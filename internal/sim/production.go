// production.go : advances factory queues, consumes materials, and delivers completed units.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : production_manager.cpp - GameManager_ManageProduction
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// ProductionState classifies a factory or Eco Sphere from its work, requested state, and available power; factories also require raw material.
//
// C++: the running / powered off / held states of a producing structure
// (unitinfo.cpp order + the power deficit / materials shortage holds).
type ProductionState int

const (
	ProdIdle    ProductionState = iota // no work queued (an empty factory)
	ProdRunning                        // working, intent on, resources available
	ProdPaused                         // working but the player STOPPED it (intent off)
	ProdStalled                        // working, intent on, but out of power/raw
)

// ProdEventKind tags a powered production transition for the in game screen.
type ProdEventKind int

const (
	ProdStarted   ProdEventKind = iota // a factory resumed (player START)
	ProdStopped                        // a factory paused (player STOP)
	ColonyStarted                      // a team's FIRST Eco Sphere began operating
	ColonyStopped                      // a team's LAST operating Eco Sphere stopped (GLOBAL announce)
)

// ProdEvent is one edge triggered production transition. Unit is the producer for a
// factory start/stop (nil for the colony events); Team is the owning team.
type ProdEvent struct {
	Kind ProdEventKind
	Unit *game.UnitInfo
	Team int
}

// DrainProductionEvents returns the production transitions since the last call and
// clears the queue. The in game screen drains it each frame to voice + message +
// banner the changes.
func (g *Game) DrainProductionEvents() []ProdEvent {
	out := g.prodEvents
	g.prodEvents = nil
	return out
}

func (g *Game) emitProd(e ProdEvent) {
	g.prodEvents = append(g.prodEvents, e)
}

// factoryPaused reports whether a factory has been STOPPED by the player: it still
// holds a queue (work to do) but its rate is 0. A directly started order with no
// queue (the lower level StartBuild path) is not "paused" - it just has no queue.
func factoryPaused(f *game.UnitInfo) bool {
	return f != nil && len(f.BuildQueue) > 0 && f.BuildRate == 0
}

// factoryHasResources reports whether a factory can produce this turn: it must be
// powered (active, not power starved) AND its complex must hold raw material. This
// is the resource half of ProductionState. C++: the power + materials gates on a
// factory's build step.
func (g *Game) factoryHasResources(f *game.UnitInfo) bool {
	if !economy.IsActive(f) || f.PowerStarved {
		return false
	}
	// A factory funds a build from its OWN cargo first, the complex pool second
	// (buildPool's preference order). Either source covers it.
	if f.Storage > 0 {
		return true
	}
	if c := g.ComplexOf(f); c != nil {
		return c.Material > 0
	}
	return true // a lone factory with no complex pool falls back to its own draw
}

// ProductionState reports the running/paused/stalled/idle state of a powered
// producer. A factory keys off its queue (work), its BuildRate (intent), and its
// resources; an Eco Sphere keys off its power state (work is always present).
func (g *Game) ProductionState(b *game.UnitInfo) ProductionState {
	if b == nil {
		return ProdIdle
	}
	if b.Type == assets.Greenhse {
		// An Eco Sphere always has work (it just needs power). Powered off / idled by
		// the player = paused; in a power deficit = stalled; otherwise running.
		switch {
		case !economy.IsActive(b):
			return ProdPaused
		case b.PowerStarved:
			return ProdStalled
		default:
			return ProdRunning
		}
	}
	// Factory family: no queue = idle; rate 0 = the player paused it; rate>0 but no
	// power/raw = stalled; otherwise running.
	switch {
	case len(b.BuildQueue) == 0:
		return ProdIdle
	case b.BuildRate == 0:
		return ProdPaused
	case !g.factoryHasResources(b):
		return ProdStalled
	default:
		return ProdRunning
	}
}

// FactoryProducing reports whether b is a factory actively turning out units: it has
// a queue that is not empty and is not player paused (running or resource stalled both count
// as "building"). Used for the "Building!" voice on selecting a producing factory.
func (g *Game) FactoryProducing(b *game.UnitInfo) bool {
	if b == nil {
		return false
	}
	q, _ := g.FactoryQueue(b)
	return len(q) > 0 && g.ProductionState(b) != ProdPaused
}

// PauseFactory STOPS a factory: it remembers the current rate and sets BuildRate to
// 0, which holds the in progress order (its partial build is KEPT) and the whole
// queue. It does NOT clear the queue. START resumes from here. C++: pausing a
// factory keeps its build_list and rate.
func (g *Game) PauseFactory(factory *game.UnitInfo) {
	if factory == nil || factory.BuildRate == 0 {
		return
	}
	factory.LastBuildRate = factory.BuildRate
	factory.BuildRate = 0
	g.emitProd(ProdEvent{Kind: ProdStopped, Unit: factory, Team: factory.Team})
}

// StartFactory resumes a paused factory at its remembered rate (X1 if it never ran)
// and starts the head unit if none is building or parked awaiting its exit. A
// factory with an empty queue stays idle. C++: resuming production from the build
// menu / the popup START.
func (g *Game) StartFactory(factory *game.UnitInfo) {
	if factory == nil || len(factory.BuildQueue) == 0 || factory.BuildRate > 0 {
		return
	}
	rate := factory.LastBuildRate
	if rate < 1 {
		rate = 1
	}
	factory.BuildRate = rate
	g.startFactoryHead(factory) // start the head unless one is already building/parked
	g.emitProd(ProdEvent{Kind: ProdStarted, Unit: factory, Team: factory.Team})
}

// RestoreFactoryProduction restores a factory's saved queue, rate, and repeat flag. It powers the factory on and starts the head unit without a
// production-start announcement.
func (g *Game) RestoreFactoryProduction(factory *game.UnitInfo, queue []assets.UnitType, rate int, repeat bool) {
	if factory == nil || len(queue) == 0 {
		return
	}
	if rate < 1 {
		rate = 1
	}
	factory.BuildQueue = append([]assets.UnitType(nil), queue...)
	factory.BuildRate = rate
	factory.RepeatBuild = repeat
	// Restored production uses OrderPowerOn so the factory's active frame, select sound, and popup state match its running queue.
	factory.Order = game.OrderPowerOn
	factory.State = game.OrderStateExecutingOrder
	g.startFactoryHead(factory)
}

// OperatingEcoSpheres counts a team's powered Eco Spheres that are not starved (the ones
// currently earning colonization score). Drives accrueScore and the colony edges.
func (g *Game) OperatingEcoSpheres(team int) int {
	return len(g.operatingEcoSphereUnits(team))
}

// operatingEcoSphereUnits lists the team's operating Eco Spheres: powered on and
// not power starved. accrueScore walks the units themselves because each sphere
// also accumulates its points as cargo.
func (g *Game) operatingEcoSphereUnits(team int) []*game.UnitInfo {
	var spheres []*game.UnitInfo
	for _, u := range g.Units.Units() {
		if u.Team == team && u.Type == assets.Greenhse && economy.IsActive(u) && !u.PowerStarved {
			spheres = append(spheres, u)
		}
	}
	return spheres
}

// RestoreColonyState derives the colony announcement latch from the restored board after power is reconciled. This prevents a running colony from
// appearing to start again on the first score pass after loading.
func (g *Game) RestoreColonyState() {
	for team := 0; team < game.TeamCount; team++ {
		g.colonyActive[team] = g.OperatingEcoSpheres(team) > 0
	}
}

// detectColonyEdges flips colonyActive[team] and emits an edge triggered
// ColonyStarted (the team's first Eco Sphere began operating) or ColonyStopped (the
// last one stopped). Called once per team turn from accrueScore. ColonyStopped is a
// global event announced to all players; ColonyStarted belongs to the affected team.
func (g *Game) detectColonyEdges(team int, operating int) {
	if team < 0 || team >= game.TeamCount {
		return
	}
	active := operating > 0
	if active == g.colonyActive[team] {
		return // no transition this turn
	}
	g.colonyActive[team] = active
	if active {
		g.emitProd(ProdEvent{Kind: ColonyStarted, Team: team})
	} else {
		g.emitProd(ProdEvent{Kind: ColonyStopped, Team: team})
	}
}
