// factory.go : runs a factory's production queue and places what it turns out.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - factory queue and unit deployment
package sim

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// SetFactoryQueue commits a player edited production queue to a factory: it
// replaces the factory's build list, sets the repeat flag + build rate, and
// starts building the head unit. An empty queue HALTS the factory (drops any
// active production). C++ ported : buildmenu.cpp - FactoryBuildMenu::Build.
func (g *Game) SetFactoryQueue(factory *game.UnitInfo, queue []assets.UnitType, rate int, repeat bool) {
	if factory == nil {
		return
	}
	wasProducing := g.FactoryProducing(factory) // capture before the queue is replaced
	if rate < 1 {
		rate = 1
	}
	g.CancelBuild(factory) // drop any active order (incl. an AwaitingDeploy hold)
	factory.RepeatBuild = repeat
	factory.BuildRate = rate
	if len(queue) == 0 {
		factory.BuildQueue = nil
		factory.BuildRate = 0
		return
	}
	factory.BuildQueue = append([]assets.UnitType(nil), queue...)
	// A new queue powers on the factory and announces production. The turn-start generator optimizer supplies the added demand or holds production
	// when capacity is short.
	if !wasProducing {
		// Production uses OrderPowerOn so the factory's active frame, select sound, and popup state match its running queue.
		factory.Order = game.OrderPowerOn
		factory.State = game.OrderStateExecutingOrder
		g.emitProd(ProdEvent{Kind: ProdStarted, Unit: factory, Team: factory.Team})
	}
	g.startFactoryHead(factory)
}

// FactoryQueue returns a factory's current production queue (head = the unit being
// built) and its repeat flag, for re opening the factory build menu.
func (g *Game) FactoryQueue(factory *game.UnitInfo) ([]assets.UnitType, bool) {
	if factory == nil {
		return nil, false
	}
	return append([]assets.UnitType(nil), factory.BuildQueue...), factory.RepeatBuild
}

// startFactoryHead mints the unit at the front of the factory's queue and starts
// building it (inside the factory) at the factory's rate. It does nothing while a
// finished unit is still parked awaiting its exit, or while a build is already in
// progress - the next unit only begins once the previous one has driven out.
func (g *Game) startFactoryHead(factory *game.UnitInfo) {
	if g.MakeUnit == nil || len(factory.BuildQueue) == 0 {
		return
	}
	if g.buildReady[factory] != nil || g.BuildOrderFor(factory) != nil {
		return // a unit is parked awaiting exit, or one is already building
	}
	head := factory.BuildQueue[0]
	product := g.MakeUnit(factory.Team, head, game.ProducedFlags(head))
	if product == nil {
		return
	}
	// The unit is built INSIDE the factory and parks on it when done; anchor the
	// order at the factory's own cell.
	g.StartBuild(factory, product, factory.GridX, factory.GridY, factory.BuildRate)
}

// popFactoryQueue drops the finished head of a factory's queue (re appending to the
// tail when RepeatBuild loops), returning the factory when a next unit remains. The
// caller starts it AFTER the AdvanceBuilds loop (startFactoryHead appends to g.builds,
// which the loop compacts in place). C++: the build list advance after a unit finishes.
func (g *Game) popFactoryQueue(factory *game.UnitInfo) *game.UnitInfo {
	if len(factory.BuildQueue) == 0 {
		return nil
	}
	head := factory.BuildQueue[0]
	if factory.RepeatBuild {
		factory.BuildQueue = append(factory.BuildQueue, head) // cycle the list
	}
	factory.BuildQueue = factory.BuildQueue[1:]
	if len(factory.BuildQueue) == 0 {
		factory.BuildRate = 0 // queue exhausted
		// Power the factory DOWN so it stops drawing its idle 1 power, and announce it.
		// C++: a factory whose build_list empties returns to ORDER_POWER_OFF.
		if economy.IsActive(factory) {
			factory.Order = game.OrderPowerOff
			factory.State = game.OrderStateExecutingOrder
			g.emitProd(ProdEvent{Kind: ProdStopped, Unit: factory, Team: factory.Team})
		}
		return nil
	}
	return factory
}

// isFactoryOrder reports whether a stationary builder is producing a mobile unit rather than a mobile constructor raising a structure.
func isFactoryOrder(o *build.BuildOrder) bool {
	b := o.Builder
	return b != nil &&
		b.Flags.Has(game.FlagStationary) &&
		!o.Product.Flags.Has(game.FlagStationary)
}

// completeFactoryOutput finishes a factory's produced unit: full stats, parked ON the
// factory (sharing cells) in a build ready state. BLDMRK exit markers ring the factory;
// the player (or AI) drives the unit out, and the next queued unit starts only once it
// has left (sweepFactoryReady). No unfold - a mobile unit does not balloon.
// C++: factory output waiting on ORDER_STATE_UNIT_READY (units_manager.cpp).
func (g *Game) completeFactoryOutput(o *build.BuildOrder, team int) {
	factory := o.Builder
	p := o.Product
	g.nextBuilt++
	p.ID = uint16(10000 + g.nextBuilt)
	p.Team = team
	p.InitState()
	g.recordUnitBuilt(team)                         // score screen tally (units built)
	g.Units.Deploy(p, factory.GridX, factory.GridY) // hold ON the factory until it exits
	p.HiddenUntilExpand = true                      // not drawn while inside: it materialises on the marker when it exits
	fp := build.Footprint(factory.Flags)
	if g.buildReady == nil {
		g.buildReady = make(map[*game.UnitInfo]*BuildReady)
	}
	g.buildReady[factory] = &BuildReady{
		Product: p.Type, Building: p,
		SiteX: factory.GridX, SiteY: factory.GridY, Footprint: fp,
		Factory: true,
	}
	// A computer team has no player to click a marker, so queue an automatic
	// drive off for the start of its next turn (issueBuildExits drives the unit).
	if g.State.Teams[team].Type != game.TeamPlayer {
		if bx, by, ok := g.FactoryDeployCell(factory, p.Flags); ok {
			if g.buildExits == nil {
				g.buildExits = make(map[*game.UnitInfo][2]int)
			}
			g.buildExits[factory] = [2]int{bx, by}
		}
	}
	g.RefreshVisibility()
}

// FactoryDeployCell spirals out from a factory for the nearest free, accessible
// cell a produced unit (with the given deploy flags) can stand on. Shared by the
// player factory queue and the AI's production. C++: the deploy site search around
// a factory (units_manager.cpp).
func (g *Game) FactoryDeployCell(factory *game.UnitInfo, flags game.Flags) (int, int, bool) {
	cx, cy := factory.GridX, factory.GridY
	if cx < 0 || cy < 0 {
		return 0, 0, false
	}
	team := factory.Team
	// The desc keys off the FACTORY's type but the PRODUCT's flags (matching the
	// original AI search); LandType is derived from the flags, which is what the
	// accessibility check uses for the movement class.
	d := access.UnitDesc{Type: factory.Type, Flags: flags, LandType: access.LandTypeForUnit(factory.Type, flags)}
	maxR := g.W
	if g.H > maxR {
		maxR = g.H
	}
	for r := 1; r <= maxR; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx > -r && dx < r && dy > -r && dy < r {
					continue // ring only
				}
				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}
				if len(g.Units.Cells.At(x, y)) != 0 {
					continue // occupied
				}
				if g.Grid.IsAccessible(d, team, x, y, access.SameClassBlocks).Passable() {
					return x, y, true
				}
			}
		}
	}
	return 0, 0, false
}
