// rescue.go recovers builders trapped by their own base.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// rescueTrappedBuilders finds any of the team's builders that its own structures have
// sealed in and unable to reach open ground. If parked friendly units block it, the task
// opens a clear zone so they can move aside. Otherwise it demolishes the least needed
// adjacent storage building, whose rubble lets the builder leave. If no working builder
// of a type remains, it orders a replacement at the appropriate plant. Site placement and
// map exits reduce these cases, but saved games or irregular maps can still trap a builder.
// C++ reference: taskmanagebuildings.cpp, TaskManager unit recovery.
func (t *taskManageBuildings) rescueTrappedBuilders() {
	g := t.ctx.Game
	if g == nil {
		return
	}

	var trapped []*game.UnitInfo
	for _, u := range g.Units.Units() {
		if u.Team != t.team || !u.Flags.Has(game.FlagConstructorUnit) || u.Building {
			continue // a builder mid build is not free to move anyway
		}
		if _, headed := t.pending[u]; headed {
			continue // already committed to a drive this turn
		}
		if !t.isIdleBuilder(u) || t.canRoam(u) {
			continue // busy or not boxed in
		}
		trapped = append(trapped, u)
	}

	sort.Slice(trapped, func(i, j int) bool { return trapped[i].ID < trapped[j].ID })
	for _, u := range trapped {
		// Its own parked units walled it in: have them moved aside, keep the buildings.
		if t.clearWayOut(u) {
			continue
		}

		// Sealed by buildings: demolition stays the last resort.
		if t.freeByDemolition(u) {
			dbg.Logf(dbg.AI, "team %d freed trapped %s at (%d,%d) by demolition", t.team, u.Type.Tag(), u.GridX, u.GridY)
		}
	}

	t.ensureBuilders() // replace a builder type the team has none of in working order
}

// clearWayOut asks the zone clearing task to move friendly parked units off the cells
// blocking the builder. It returns false when buildings block the route or no clearing
// task is available, allowing the caller to try demolition. C++ reference:
// taskcreatebuilding.cpp, ClearZone.
func (t *taskManageBuildings) clearWayOut(builder *game.UnitInfo) bool {
	// A manager without a clearing task cannot move units aside.
	if t.clearer == nil {
		return false
	}

	cells, ok := t.ownUnitsInTheWay(builder)

	// Not boxed in by own units alone, or nothing to move: leave it to demolition.
	if !ok || len(cells) == 0 {
		return false
	}

	t.clearer.addZone(builder, cells)
	dbg.Logf(dbg.AI, "team %d clears a way out for %s at (%d,%d): %d own units in the way", t.team, builder.Type.Tag(), builder.GridX, builder.GridY, len(cells))

	return true
}

// boxedByOwnUnits reports whether the zone clearing task will free the builder soon: it
// has a clearing task and only the team's own parked units box it in.
func (t *taskManageBuildings) boxedByOwnUnits(builder *game.UnitInfo) bool {
	if t.clearer == nil {
		return false
	}

	_, ok := t.ownUnitsInTheWay(builder)

	return ok
}

// freeByDemolition blows up the least needed building next to a trapped builder so its
// cell (rubble, which is drivable) opens an exit. It never touches a building the base
// depends on - only a surplus storage dump - so it frees the builder without crippling
// the economy. Returns whether a building was demolished. C++: the AI reclaiming ground
// around a stuck builder (units_manager.cpp - ORDER_CLEAR after a self destruct).
func (t *taskManageBuildings) freeByDemolition(builder *game.UnitInfo) bool {
	g := t.ctx.Game
	seen := map[*game.UnitInfo]bool{}
	var candidates []*game.UnitInfo

	for cy := builder.GridY - 1; cy <= builder.GridY+1; cy++ {
		for cx := builder.GridX - 1; cx <= builder.GridX+1; cx++ {
			if cx == builder.GridX && cy == builder.GridY {
				continue
			}
			for _, o := range g.Units.Cells.At(cx, cy) {
				if o.Team != t.team || seen[o] || demolitionRank(o.Type) < 0 {
					continue
				}
				seen[o] = true
				candidates = append(candidates, o)
			}
		}
	}

	if len(candidates) == 0 {
		return false // nothing redundant borders it: leave the recovery to a replacement
	}

	// Least needed first (gold store before fuel before raw), then lowest id for a stable pick.
	sort.Slice(candidates, func(i, j int) bool {
		ri, rj := demolitionRank(candidates[i].Type), demolitionRank(candidates[j].Type)
		if ri != rj {
			return ri < rj
		}
		return candidates[i].ID < candidates[j].ID
	})

	g.DestroyUnit(candidates[0]) // leaves drivable rubble on the cell; the bulldozer clears it later

	return true
}

// demolitionRank orders the buildings safe to blow up to free a trapped builder, least
// needed first (0 = the first to go). A negative rank means the building is load bearing
// (mining, power, a factory, a habitat, the last of anything) and must never be
// demolished for this. Only surplus storage dumps qualify: losing one trims capacity,
// which redistribute handles, and never stops production.
func demolitionRank(t assets.UnitType) int {
	switch t {
	case assets.GoldSm:
		return 0
	case assets.FDump:
		return 1
	case assets.ADump:
		return 2
	default:
		return -1
	}
}

// ensureBuilders keeps a working Constructor and Engineer available. When no builder of
// a type is working or in production, it requests one from the plant that builds it. The
// child obtain request counts a builder only when it can roam. C++ reference: aiplayer.cpp,
// TaskObtainUnits(CONSTRUCTOR / ENGINEER).
func (t *taskManageBuildings) ensureBuilders() {
	for _, d := range builderDemand {
		if t.usableBuilders(d.unit) > 0 || unitsInProduction(t.ctx.Game, t.team, d.unit) > 0 {
			continue // one is working or already on the way
		}

		t.requestUnit(t.ctx, unitNeed{unit: d.unit, factory: d.factory, flags: d.flags, accept: t.canRoam})
	}
}

// builderNeed pairs a builder the team must keep with the plant producing it and its
// deploy flags.
type builderNeed struct {
	unit    assets.UnitType
	factory assets.UnitType
	flags   game.Flags
}

// builderDemand maps each builder the team must keep to the plant that produces it.
var builderDemand = []builderNeed{
	{assets.Constrct, assets.LandPlt, flagLandUnit},  // 2x2 builder, Heavy Vehicle Plant
	{assets.Engineer, assets.LightPlt, flagLandUnit}, // 1x1 builder, Light Vehicle Plant
}

// usableBuilders counts builders that are working, can roam, or will be freed by the zone
// clearing task. A builder sealed by buildings does not count and can trigger a replacement.
func (t *taskManageBuildings) usableBuilders(unitType assets.UnitType) int {
	g := t.ctx.Game
	n := 0

	for _, u := range g.Units.Units() {
		if u.Team != t.team || u.Type != unitType {
			continue
		}
		// Working, free to roam, or about to be freed by the zone clearing task.
		if u.Building || t.canRoam(u) || t.boxedByOwnUnits(u) {
			n++
		}
	}

	return n
}

/* // note: unused code in Go port - replaced by unitsInProduction (task_obtainunits.go, step 6004)
// producing counts how many of a type the team is already turning out at its factories,
// so ensureBuilders never queues a second replacement while one is under construction.
func (t *taskManageBuildings) producing(unitType assets.UnitType) int {
	n := 0

	for _, o := range t.ctx.Game.BuildOrders() {
		if o.Builder != nil && o.Builder.Team == t.team && o.Product != nil && o.Product.Type == unitType {
			n++
		}
	}

	return n
}
*/

/* // note: unused code in Go port - replaced by the obtain request (requestUnit + produceUnit, task_obtainunits.go, step 6004)
// produceAtFactory starts one unit of unitType at an idle plant of factoryType, on a
// free deploy cell. Returns whether an order was issued. C++: TaskManager.CreateUnit /
// StartBuilding on the plant.
func (t *taskManageBuildings) produceAtFactory(factoryType, unitType assets.UnitType) bool {
	g := t.ctx.Game
	flags := flagLandUnit // both builders are land units

	for _, f := range g.Units.Units() {
		if f.Team != t.team || f.Type != factoryType || f.Building || g.BuildOrderFor(f) != nil {
			continue
		}

		x, y, ok := g.FactoryDeployCell(f, flags)
		if !ok {
			continue
		}

		unit := t.ctx.MakeUnit(t.team, unitType, flags)
		if unit == nil {
			return false
		}

		g.StartBuild(f, unit, x, y, aiBuildRate(g, f, unit))

		return true
	}

	return false
}
*/
