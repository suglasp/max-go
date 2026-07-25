// rescue.go : the M.A.X. AI recovery for a builder its own base has boxed in - free it
// by demolishing the least needed adjacent building, or build a replacement.
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
// sealed in (a builder that cannot roam to open ground and so takes no job, the "locked
// between buildings" stall) and recovers it. First it tries to FREE the builder by
// blowing up the least needed building beside it (the wreck leaves drivable rubble, so
// the builder drives out next turn). Then, for any builder type the team has none of in
// working order, it builds a REPLACEMENT at the right plant. Prevention (the lattice +
// escape guard in findSite, the exterior exit in the sim) makes this rare, but a loaded
// save or an irregular map can still strand one. C++ ported : the AI freeing / rebuilding
// a stuck builder (taskmanagebuildings.cpp - TaskManager unit recovery).
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
		if t.freeByDemolition(u) {
			dbg.Logf(dbg.AI, "team %d freed trapped %s at (%d,%d) by demolition", t.team, u.Type.Tag(), u.GridX, u.GridY)
		}
	}

	t.ensureBuilders() // replace a builder type the team has none of in working order
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

// ensureBuilders keeps the team fielding a working constructor and engineer: if every
// one of a type is trapped or gone (none can roam and none is mid build) and none is
// already in production, it builds a fresh one at the plant that makes it - a Constructor
// at the Heavy Vehicle Plant, an Engineer at the Light Vehicle Plant (menus/factorybuild
// rosters). This is why a base whose only constructor got boxed in no longer sits idle
// forever: the plant turns out a new one. C++ ported : aiplayer.cpp - TaskObtainUnits
// (CONSTRUCTOR / ENGINEER) obtaining a missing builder.
func (t *taskManageBuildings) ensureBuilders() {
	for _, d := range builderDemand {
		if t.usableBuilders(d.unit) > 0 || t.producing(d.unit) > 0 {
			continue // one is working or already on the way
		}
		if t.produceAtFactory(d.factory, d.unit) {
			dbg.Logf(dbg.AI, "team %d building a replacement %s at a %s", t.team, d.unit.Tag(), d.factory.Tag())
		}
	}
}

// builderNeed pairs a builder the team must keep with the plant producing it.
type builderNeed struct {
	unit    assets.UnitType
	factory assets.UnitType
}

// builderDemand maps each builder the team must keep to the plant that produces it.
var builderDemand = []builderNeed{
	{assets.Constrct, assets.LandPlt},  // 2x2 builder, Heavy Vehicle Plant
	{assets.Engineer, assets.LightPlt}, // 1x1 builder, Light Vehicle Plant
}

// usableBuilders counts the team's builders of a type that are still doing their job:
// mid build, or free and able to roam. A trapped one does not count, so a base with only
// a boxed in constructor reads as zero and triggers a replacement.
func (t *taskManageBuildings) usableBuilders(unitType assets.UnitType) int {
	g := t.ctx.Game
	n := 0

	for _, u := range g.Units.Units() {
		if u.Team != t.team || u.Type != unitType {
			continue
		}
		if u.Building || t.canRoam(u) {
			n++
		}
	}

	return n
}

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
