// task_managebuildings.go : M.A.X. AI building
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// aiBuildRateBase is the rate the AI plans its material NEEDS against (x1): the slowest, so
// an estimate of "can a bordering base cover this build" is never optimistic.
const aiBuildRateBase = 1

// aiBuildRate picks the speed a builder commits to a job - x1, x2 or x4 - from the material it
// can actually spend, using the same build.MaxBuildRate gate as the player's build menu. A faster rate spends more material each turn, so a poor
// team remains at x1 and the simulation reduces the rate if stock runs short.
func aiBuildRate(g *sim.Game, builder, product *game.UnitInfo) int {
	if builder == nil || product == nil || product.Values == nil {
		return aiBuildRateBase
	}

	turns := product.Values.Get(game.AttribTurns)
	if turns < 1 {
		turns = 1
	}

	rate := build.MaxBuildRate(builder.Type, turns, g.BuildMaterial(builder))
	if rate < aiBuildRateBase {
		return aiBuildRateBase
	}

	return rate
}

// taskManageBuildings is a team's economy brain: each turn it puts every idle
// constructor/engineer on the building the base most needs, by live DEMAND not a
// fixed list, in priority order (mining, power, storage, habitat, then the strategic
// backbone). The builder DRIVES to its site and only raises the structure on arrival,
// rather than teleporting into the build pose. TaskManager.BeginTurn requeues it each
// turn. C++: TaskManageBuildings (taskmanagebuildings.cpp).
// Reduced vs the original: no AreaMap/ConnectionMap planner (spiral findSite keeps the
// base compact), no bridge building, no per building subtasks; explicit connectors and
// defense and radar are handled by separate tasks.
type taskManageBuildings struct {
	TaskBase
	ctx      *aiContext
	strategy Strategy
	goal     winGoal                             // the team's match objective; a coloniser also builds Eco Spheres
	coastal  bool                                // base near water: include a shipyard in the roster, else drop it
	pending  map[*game.UnitInfo]pendingBuildSite // pending records, per builder
}

// pendingBuildSite is a build the manager has committed a builder to but not yet
// started: the builder is still driving to the site. C++: a TaskCreateBuilding in
// its TaskMove phase (taskcreatebuilding.cpp).
type pendingBuildSite struct {
	unitType assets.UnitType
	large    bool
	x, y     int
}

func newTaskManageBuildings(team int, strategy Strategy, goal winGoal, coastal bool, ctx *aiContext) *taskManageBuildings {
	return &taskManageBuildings{
		TaskBase: newTaskBase(team, nil, 1000), // high base priority (root economy task)
		ctx:      ctx,
		strategy: strategy,
		goal:     goal,
		coastal:  coastal,
		pending:  map[*game.UnitInfo]pendingBuildSite{},
	}
}

// BeginTurn snapshots the economy once, then assigns each idle builder the highest
// priority building the base needs. At most one order per builder per turn (issuing
// one poses it, so it is no longer idle - a natural rate limit). The task does not
// reschedule itself; TaskManager.BeginTurn requeues it each turn.
// C++: TaskManageBuildings::BeginTurn -> CheckNeeds.
func (t *taskManageBuildings) BeginTurn() {
	g := t.ctx.Game

	if g == nil {
		return
	}

	t.prunePending()
	econ := surveyTeamEconomy(g, t.team)
	t.traceEconomy(econ)
	t.rescueTrappedBuilders() // free (or replace) any builder its own base has boxed in

	for _, u := range g.Units.Units() {
		if u.Team != t.team || !u.Flags.Has(game.FlagConstructorUnit) || u.Building {
			continue
		}

		if _, headed := t.pending[u]; headed {
			t.advancePending(u) // keep driving to the chosen site, build on arrival
			continue
		}

		if t.isIdleBuilder(u) {
			t.tryBuild(econ, u)
		}
	}
}

// traceEconomy logs the demand snapshot the manager plans against once each turn:
// the net economy plus the demand flags that decide the next build, so a stalled
// AI (a tier that never clears - the "stuck on 2x2/1x1" hang) is visible in the
// log. Gated by dbg; off for normal players. C++ diag aid, no analogue in the original.
func (t *taskManageBuildings) traceEconomy(e teamEconomy) {
	dbg.Logf(dbg.AI, "team %d econ: mines=%d netRaw=%d netFuel=%d power=%d life=%d raw=%d/%d fuel=%d/%d storeCap=%d | needMining=%v needPower=%v needHabitat=%v", t.team, e.miningStations, e.netRaw, e.netFuel, e.power, e.life, e.rawStored, e.rawCap, e.fuelStored, e.fuelCap, aiStorageCap, needsMining(e), e.power > 0, e.life > 0)
}

// prunePending drops drive intents for builders that have gone away (destroyed)
// or have already started building, so the map never references a stale unit.
func (t *taskManageBuildings) prunePending() {
	if len(t.pending) == 0 {
		return
	}

	g := t.ctx.Game
	live := map[*game.UnitInfo]bool{}

	for _, u := range g.Units.Units() {
		if u.Team == t.team && u.Flags.Has(game.FlagConstructorUnit) {
			live[u] = true
		}
	}

	for u := range t.pending {
		if !live[u] || u.Building || g.BuildOrderFor(u) != nil {
			delete(t.pending, u)
		}
	}
}

// advancePending continues a builder's drive to its committed site: it builds on
// arrival, issues the move again while still en route, and abandons the site if it
// is no longer buildable or no longer reachable (so the manager plans again next
// turn). C++: TaskCreateBuilding advancing its TaskMove then raising the structure.
func (t *taskManageBuildings) advancePending(builder *game.UnitInfo) {
	ps := t.pending[builder]
	flags := buildFlagsFor(ps.unitType, ps.large)
	d := access.UnitDesc{Type: ps.unitType, Flags: flags, LandType: access.LandTypeForUnit(ps.unitType, flags)}

	if !t.siteFits(builder, d, ps.x, ps.y) {
		delete(t.pending, builder) // site taken/blocked while en route: plan again
		return
	}

	fp := footprintSize(ps.large)
	if builderOnFootprint(builder.GridX, builder.GridY, ps.x, ps.y, fp) {
		t.placeBuild(builder, ps.unitType, ps.large, ps.x, ps.y) // arrived: raise it
		return
	}

	if !t.driveToSite(builder, ps.x, ps.y, fp) {
		delete(t.pending, builder) // can no longer reach the site: give it up
	}
}

// tryBuild gives the builder its highest priority job by type: a Constructor raises
// the large (2x2) structures, an Engineer the small (1x1) ones - the same split the
// construction menu enforces, so a constructor never receives a 1x1 dump order.
// C++: Builder_GetBuildableUnits(CONSTRCT/ENGINEER) driving CheckNeeds.
func (t *taskManageBuildings) tryBuild(econ teamEconomy, builder *game.UnitInfo) bool {
	var committed bool

	switch builder.Type {
	case assets.Constrct:
		committed = t.tryBuildLarge(econ, builder)
	case assets.Engineer:
		committed = t.tryBuildSmall(econ, builder)
	default:
		return false
	}

	// An idle builder that takes NO job every turn is the stall signature: log it
	// so a demand that never clears (or no reachable site) is visible in the trace.
	dbg.Logf(dbg.AI, "team %d idle %s at (%d,%d) -> committed=%v", t.team, builder.Type.Tag(), builder.GridX, builder.GridY, committed)

	return committed
}

// tryBuildLarge assigns a CONSTRUCTOR (which raises 2x2 structures) the highest
// priority large building the base needs: mining, a power station, a habitat, then
// the strategic factory/research backbone. C++: the CONSTRCT roster +
// TaskManageBuildings::CheckNeeds / CheckPower / CheckWorkers.
func (t *taskManageBuildings) tryBuildLarge(econ teamEconomy, builder *game.UnitInfo) bool {
	// 1. Mining station - expand the resource base when production cannot sustain it.
	if needsMining(econ) && !t.handling(assets.MiningSt) {
		if t.startMiningStation(builder) {
			return true
		}
	}

	// 2. Power station - the constructor's 2x2 generator, when consumers outstrip
	//    generation.
	if econ.power > 0 && !t.powerInProgress() {
		if t.startBuilding(builder, buildSpec{t: assets.PowerStn, large: true}) {
			return true
		}
	}

	// 3. Habitat - house workers when life consumers outstrip habitat support.
	if econ.life > 0 && !t.handling(assets.Habitat) {
		if t.startBuilding(builder, buildSpec{t: assets.Habitat, large: true}) {
			return true
		}
	}

	// 4. Strategic backbone - factories / research / depot, plus a shipyard for a
	//    coastal team and Eco Spheres for a coloniser (all 2x2).
	if spec, ok := nextBuilding(t.ctx.Game, t.team, strategicBuildOrders(t.strategy, t.goal, t.coastal)); ok {
		return t.startBuilding(builder, spec)
	}

	return false
}

// tryBuildSmall assigns an ENGINEER (which raises 1x1 structures) the highest
// priority small building the base needs: a power generator, then storage dumps as
// the mines fill. Separate tasks handle connectors, radar, and defenses. C++: the
// ENGINEER roster + CheckPower / EndTurn.
func (t *taskManageBuildings) tryBuildSmall(econ teamEconomy, builder *game.UnitInfo) bool {
	// 1. Power generator - the engineer's 1x1 generator, when power is short.
	if econ.power > 0 && !t.powerInProgress() {
		if t.startBuilding(builder, buildSpec{t: assets.PowGen, large: false}) {
			return true
		}
	}

	// 2. Storage dumps - one per cargo type that the team mines and has filled past
	//    three quarters, but only up to a capacity cap so a mining surplus does not
	//    pile on dumps forever (the base mines faster than it spends at x1).
	mining := econ.miningStations > 0

	if needsStorage(econ.rawStored, econ.rawCap, mining) && !t.handling(assets.ADump) {
		if t.startBuilding(builder, buildSpec{t: assets.ADump, large: false}) {
			return true
		}
	}

	if needsStorage(econ.fuelStored, econ.fuelCap, mining) && !t.handling(assets.FDump) {
		if t.startBuilding(builder, buildSpec{t: assets.FDump, large: false}) {
			return true
		}
	}

	if needsStorage(econ.goldStored, econ.goldCap, mining) && !t.handling(assets.GoldSm) {
		if t.startBuilding(builder, buildSpec{t: assets.GoldSm, large: false}) {
			return true
		}
	}

	return false
}

// powerInProgress reports whether the team is already raising a power building of
// either kind (a constructor's power station or an engineer's generator), so the
// two builder types do not both pile generation on the same shortfall.
func (t *taskManageBuildings) powerInProgress() bool {
	return t.handling(assets.PowerStn) || t.handling(assets.PowGen)
}

// handling reports whether the team is already growing a building of this type:
// either a builder is en route to a site (pending) or an order is in progress
// (BuildOrders). It keeps a second idle builder from duplicating an effort
// another is already on. C++: the task list scan TaskManageBuildings does.
func (t *taskManageBuildings) handling(unitType assets.UnitType) bool {
	if pendingBuild(t.ctx.Game, t.team, unitType) {
		return true
	}

	for _, ps := range t.pending {
		if ps.unitType == unitType {
			return true
		}
	}

	return false
}

// isIdleBuilder reports whether u is one of this team's constructors that is
// free to take a new build order this turn (not already building or moving).
func (t *taskManageBuildings) isIdleBuilder(u *game.UnitInfo) bool {
	if u.Team != t.team || !u.Flags.Has(game.FlagConstructorUnit) {
		return false
	}

	if u.Building {
		return false // already raising a structure
	}

	g := t.ctx.Game

	return g.BuildOrderFor(u) == nil && !g.HasOrder(u)
}

// startBuilding finds a valid site near the builder for spec's footprint and
// either raises it (if the builder is already there) or drives the builder to it.
// Returns whether the builder was committed. C++: TaskCreateBuilding selecting a
// site + TaskMove to it + TaskManager.CreateBuilding.
func (t *taskManageBuildings) startBuilding(builder *game.UnitInfo, spec buildSpec) bool {
	flags := buildingFlags(spec.large)
	d := access.UnitDesc{Type: spec.t, Flags: flags, LandType: access.LandTypeForUnit(spec.t, flags)}

	x, y, ok := t.findSite(builder, d)
	if !ok {
		return false
	}

	return t.buildOrDrive(builder, spec.t, spec.large, x, y)
}

// buildOrDrive raises the structure now when the builder already stands INSIDE the
// footprint (a corner of it, as the player's build positions the constructor),
// otherwise drives it onto the footprint and records the intent to build on arrival.
// Building from inside keeps the build pose slide to a short nudge instead of a long
// unanimated glide in. C++: TaskCreateBuilding - build in place vs spawn a TaskMove.
func (t *taskManageBuildings) buildOrDrive(builder *game.UnitInfo, unitType assets.UnitType, large bool, x, y int) bool {
	fp := footprintSize(large)

	// A detached site cannot draw the base's pooled material, so the builder must
	// carry the whole job in its own cargo. Load it from the base first; if still
	// unsupplied, defer rather than start a build that would stall at empty forever.
	if !t.ensureSupplied(builder, unitType, large, x, y) {
		return false
	}

	if builderOnFootprint(builder.GridX, builder.GridY, x, y, fp) {
		return t.placeBuild(builder, unitType, large, x, y)
	}

	if t.driveToSite(builder, x, y, fp) {
		t.pending[builder] = pendingBuildSite{unitType: unitType, large: large, x: x, y: y}
		return true
	}

	return false // unreachable site: let the manager try a different demand
}

// ensureSupplied reports whether a build at (x, y) can be funded, provisioning the
// builder when it must supply itself. A site bordering the base joins the complex and
// draws its pool, so it needs nothing. A DETACHED site cannot reach the pool, so the
// builder carries the whole job: it is topped up from the base first, and if it still
// cannot cover the cost, the build waits rather than starting with no material.
// a 0 cargo constructor stalled a detached build at 0 forever while the base sat full).
// C++: the AI loads material (TaskGetMaterials) before a remote build.
func (t *taskManageBuildings) ensureSupplied(builder *game.UnitInfo, unitType assets.UnitType, large bool, x, y int) bool {
	if t.touchesStockedComplex(x, y, footprintSize(large), t.buildRawCostPerTurn()) {
		return true // bordered by a STOCKED complex: the build draws the base pool, no own cargo needed
	}

	cost := t.buildRawCost(unitType, large)
	if builder.Storage < cost {
		t.provisionBuilder(builder) // load up at the base before heading out
	}

	if builder.Storage >= cost {
		return true
	}

	dbg.Logf(dbg.AI, "team %d DEFER %s at (%d,%d): detached, cargo %d < cost %d (awaiting supply)", t.team, unitType.Tag(), x, y, builder.Storage, cost)

	return false
}

// provisionBuilder tops up an idle builder's raw cargo from a bordering base complex,
// up to capacity, the way a player loads a constructor before sending it out. A factory
// produced constructor comes out EMPTY, so without this it stalls the instant it builds
// where the base pool cannot reach.
func (t *taskManageBuildings) provisionBuilder(builder *game.UnitInfo) {
	if builder.Values == nil {
		return
	}

	capacity := builder.Values.Get(game.AttribStorage)
	if builder.Storage >= capacity {
		return
	}

	c := t.builderComplex(builder)
	if c == nil || c.Material <= 0 {
		return // not next to the base (needs a cargo truck) or the pool is empty
	}

	take := capacity - builder.Storage
	if take > c.Material {
		take = c.Material
	}

	c.Material -= take
	builder.Storage += take
	dbg.Logf(dbg.AI, "team %d loaded %s +%d raw (now %d/%d) from base pool", t.team, builder.Type.Tag(), take, builder.Storage, capacity)
}

// builderComplex returns the team complex bordering the builder's cell (so its
// pooled material can be loaded), or nil when the builder stands away from the base.
func (t *taskManageBuildings) builderComplex(builder *game.UnitInfo) *economy.Complex {
	g := t.ctx.Game
	n := g.Networks[t.team]

	if n == nil || g.Units == nil || g.Units.Cells == nil {
		return nil
	}

	bx, by := builder.GridX, builder.GridY

	for cy := by - 1; cy <= by+1; cy++ {
		for cx := bx - 1; cx <= bx+1; cx++ {
			for _, o := range g.Units.Cells.At(cx, cy) {
				if o.Team == t.team && o.Flags.Has(game.FlagStationary) && !o.Flags.Has(game.FlagGroundCover) {
					if c := n.ComplexOf(o); c != nil {
						return c
					}
				}
			}
		}
	}

	return nil
}

// buildRawCost is the total raw a structure of this type costs to finish: a
// stationary build draws the constructor's raw rate each turn for ATTRIB_TURNS
// turns. Used to decide whether a builder can supply a detached site itself.
func (t *taskManageBuildings) buildRawCost(unitType assets.UnitType, large bool) int {
	p := t.ctx.MakeUnit(t.team, unitType, buildFlagsFor(unitType, large))
	if p == nil || p.Values == nil {
		return 0
	}

	turns := p.Values.Get(game.AttribTurns)
	if turns < 1 {
		turns = 1
	}

	return economy.RawConsumptionRate(assets.Constrct, 1) * turns
}

// buildRawCostPerTurn is the raw a build draws each turn at the AI's build rate - the
// material a bordering complex must hold for the build to draw it, so
// touchesStockedComplex uses it to tell a stocked base from an empty detached island.
func (t *taskManageBuildings) buildRawCostPerTurn() int {
	return economy.RawConsumptionRate(assets.Constrct, aiBuildRateBase)
}

// driveToSite issues a move ONTO the footprint (its nearest cell) so the builder ends
// up inside the structure it will raise. The site was validated with the builder
// ignored, so its own presence on a footprint cell does not invalidate it. Falls back
// to the anchor cell when no footprint cell is reachable.
func (t *taskManageBuildings) driveToSite(builder *game.UnitInfo, x, y, fp int) bool {
	g := t.ctx.Game

	if fx, fy, ok := t.nearestFootprintCell(builder, x, y, fp); ok && g.OrderMove(builder, fx, fy) {
		return true
	}

	return g.OrderMove(builder, x, y)
}

// nearestFootprintCell returns the enterable cell OF the fp x fp footprint at
// (x, y) closest to the builder - the spot it should stand on to raise the
// structure from inside it.
func (t *taskManageBuildings) nearestFootprintCell(builder *game.UnitInfo, x, y, fp int) (int, int, bool) {
	g := t.ctx.Game
	bestX, bestY, bestD, found := 0, 0, 1<<30, false

	for cy := y; cy < y+fp; cy++ {
		for cx := x; cx < x+fp; cx++ {
			if cx < 0 || cy < 0 || cx >= g.W || cy >= g.H {
				continue
			}

			if !g.CanEnter(builder, cx, cy) {
				continue
			}

			if d := sim.SquaredDist(builder.GridX, builder.GridY, cx, cy); d < bestD {
				bestX, bestY, bestD, found = cx, cy, d, true
			}
		}
	}

	return bestX, bestY, found
}

// siteFits reports whether the structure fits at (x, y) with the builder IGNORED (as
// the player's build removes the constructor from the cell hash before the access
// test), so a builder inside its own footprint does not block its own site. C++ ported
// : gamescreen buildSiteValid (Hash_MapHash.Remove around Access_IsAccessible).
func (t *taskManageBuildings) siteFits(builder *game.UnitInfo, d access.UnitDesc, x, y int) bool {
	cells := t.ctx.Game.Units.Cells
	cells.Remove(builder)
	ok := build.SiteValid(t.ctx.Game.Grid, d, t.team, x, y)
	cells.Add(builder)

	return ok
}

// placeBuild mints the structure and starts the build order at (x, y), clearing
// any drive intent for the builder. C++: TaskManager.CreateBuilding / StartBuilding.
func (t *taskManageBuildings) placeBuild(builder *game.UnitInfo, unitType assets.UnitType, large bool, x, y int) bool {
	flags := buildFlagsFor(unitType, large)
	product := t.ctx.MakeUnit(t.team, unitType, flags)

	if product == nil {
		return false
	}

	t.ctx.Game.StartBuild(builder, product, x, y, aiBuildRate(t.ctx.Game, builder, product))
	delete(t.pending, builder)

	return true
}

// footprintSize is the side length of a building's footprint (2 for a 2x2, 1
// otherwise), matching buildingFlags / build.Footprint.
func footprintSize(large bool) int {
	if large {
		return 2
	}

	return 1
}

// builderOnFootprint reports whether a builder at (bx, by) stands inside the fp x fp
// footprint anchored at (x, y) - the build pose arrival test. The builder raises the
// structure from inside it (a corner of a 2x2, the cell of a 1x1).
func builderOnFootprint(bx, by, x, y, fp int) bool {
	return bx >= x && bx < x+fp && by >= y && by < y+fp
}

// minMineableDeposit is the deposit total a footprint must tap to be worth a
// mining station, the same cutoff the original uses to mark a site valuable. Below
// it a station would mine too little to be worth raising. C++:
// TaskManageBuildings_IsSiteValuable ((cargo_site & 0x1F) >= 8).
const minMineableDeposit = 8

// miningSearchRadius bounds how far the manager scans for a resource site, keeping the
// scan cheap and keeps new mines near the base, where they can join its complex.
const miningSearchRadius = 16

// startMiningStation targets the richest resource site in reach and raises a station
// or drives the builder out to it. A station mines whatever lies under its 2x2
// footprint, so the AI heads for the densest deposits. C++ ported :
// taskmanagebuildings.cpp - MarkMiningAreas + CreateBuilding(MININGST).
func (t *taskManageBuildings) startMiningStation(builder *game.UnitInfo) bool {
	g := t.ctx.Game
	if g.Resources == nil {
		return false // no resource map (headless harness): cannot site a mine
	}

	flags := buildingFlags(true) // a mining station has a 2x2 footprint
	d := access.UnitDesc{Type: assets.MiningSt, Flags: flags, LandType: access.LandTypeForUnit(assets.MiningSt, flags)}

	x, y, ok := t.findMiningSite(builder, d)
	if !ok {
		return false
	}

	return t.buildOrDrive(builder, assets.MiningSt, true, x, y)
}

// findMiningSite spirals out from the builder (bounded by miningSearchRadius) and
// returns the buildable site whose footprint taps the most resource, provided it
// clears minMineableDeposit. C++ ported : taskmanagebuildings.cpp - MarkMiningAreas
// (reduced: a bounded best site scan, no AreaMap reservation pass).
func (t *taskManageBuildings) findMiningSite(builder *game.UnitInfo, d access.UnitDesc) (int, int, bool) {
	g := t.ctx.Game
	cx, cy := builder.GridX, builder.GridY

	if cx < 0 || cy < 0 {
		return 0, 0, false
	}

	g.Units.Cells.Remove(builder) // ignore the builder so it may stand inside its own site
	defer g.Units.Cells.Add(builder)
	bestX, bestY, bestScore := 0, 0, 0

	for r := 0; r <= miningSearchRadius; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if r > 0 && dx > -r && dx < r && dy > -r && dy < r {
					continue // only the ring at radius r
				}

				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}

				if !build.SiteValid(g.Grid, d, t.team, x, y) {
					continue
				}

				if s := t.depositScore(x, y); s > bestScore {
					bestX, bestY, bestScore = x, y, s
				}
			}
		}
	}

	if bestScore >= minMineableDeposit {
		return bestX, bestY, true
	}

	return 0, 0, false
}

// depositScore is the total raw + fuel + gold a mining station's 2x2 footprint at
// (x, y) would tap, the value mining sites rank by. It counts ONLY deposits the team
// has SURVEYED (all=false), the same resource fog the player obeys, so the AI mines
// what it discovered, never the deposit map omnisciently. Until the surveyor reveals
// anything, findMiningSite finds no site and the manager falls through to other demands.
// C++ ported : survey.cpp - Survey_GetTotalResourcesInArea.
func (t *taskManageBuildings) depositScore(x, y int) int {
	raw, gold, fuel := t.ctx.Game.Resources.TotalResourcesInArea(x, y, 1, t.team, false)

	return raw + gold + fuel
}

// findSite spirals out for the nearest cell where the footprint fits, borders the team's
// stocked base (so the new building joins the complex and draws its pool instead of a
// detached, stalled island) AND leaves the builder a drivable way out, so a growing base
// never rings its own constructor in. The builder is ignored, so the first match can be
// its own cell (build in place, no drive). Once the team owns a building only connected
// sites count; with no base yet any valid site does; if no connected escape site exists it
// falls back to the nearest valid one rather than refuse. The escape guard is the spacing
// pressure that keeps drivable lanes through the base. C++: TaskCreateBuilding's site
// search + the adjacency join that keeps the base one complex.
func (t *taskManageBuildings) findSite(builder *game.UnitInfo, d access.UnitDesc) (int, int, bool) {
	g := t.ctx.Game
	cx, cy := builder.GridX, builder.GridY

	if cx < 0 || cy < 0 {
		return 0, 0, false
	}

	g.Units.Cells.Remove(builder) // ignore the builder so it may stand inside its own site
	defer g.Units.Cells.Add(builder)
	fp := build.Footprint(d.Flags)
	requireConnected := t.teamHasBuilding()
	costPerTurn := t.buildRawCostPerTurn()
	fallbackX, fallbackY, haveFallback := 0, 0, false
	maxR := g.W

	if g.H > maxR {
		maxR = g.H
	}

	for r := 0; r <= maxR; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if r > 0 && dx > -r && dx < r && dy > -r && dy < r {
					continue // only the ring at radius r
				}

				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}

				if !build.SiteValid(g.Grid, d, t.team, x, y) || !t.preservesEscape(builder, x, y, fp) {
					continue
				}

				if !requireConnected || t.touchesStockedComplex(x, y, fp, costPerTurn) {
					return x, y, true // nearest escape keeping site bordering the STOCKED base (or no base yet)
				}

				if !haveFallback {
					fallbackX, fallbackY, haveFallback = x, y, true // a detached fallback
				}
			}
		}
	}

	if haveFallback {
		return fallbackX, fallbackY, true // no connected site exists: build rather than stall
	}

	return 0, 0, false
}

// teamHasBuilding reports whether the team already owns a finished structure - so
// new buildings should connect to it rather than scatter as detached sites. The
// first building of a fresh base has nothing to connect to, so it may land anywhere.
func (t *taskManageBuildings) teamHasBuilding() bool {
	for _, u := range t.ctx.Game.Units.Units() {
		if u.Team == t.team && u.Flags.Has(game.FlagStationary) && !u.Flags.Has(game.FlagGroundCover) {
			return true
		}
	}

	return false
}

// touchesComplex reports whether the fp x fp footprint at (x, y) borders one of the
// team's stationary buildings, so a structure raised there joins the base complex
// instead of a useless detached island. C++: the AttachToPrimaryComplex adjacency join
// (units_manager.cpp); the AI's reduced stand in for the connection assistant.
func (t *taskManageBuildings) touchesComplex(x, y, fp int) bool {
	g := t.ctx.Game
	for cy := y - 1; cy <= y+fp; cy++ {
		for cx := x - 1; cx <= x+fp; cx++ {
			// inside the footprint, not a bordering cell
			if cx >= x && cx < x+fp && cy >= y && cy < y+fp {
				continue
			}

			if cx < 0 || cy < 0 || cx >= g.W || cy >= g.H {
				continue
			}

			for _, o := range g.Units.Cells.At(cx, cy) {
				if o.Team == t.team && o.Flags.Has(game.FlagStationary) && !o.Flags.Has(game.FlagGroundCover) {
					return true
				}
			}
		}
	}

	return false
}

// touchesStockedComplex reports whether the fp x fp footprint at (x, y) borders one of
// the team's STOCKED complexes (Material >= costPerTurn) - the honest "a build here
// draws the base pool" test. Bordering an EMPTY detached island (Material 0) does NOT
// count, so the AI never treats an unfundable island as connected (the false positive
// that stalled builds at 0 raw forever). C++: the AttachToPrimaryComplex join, gated on
// the complex carrying material.
func (t *taskManageBuildings) touchesStockedComplex(x, y, fp, costPerTurn int) bool {
	g := t.ctx.Game
	n := g.Networks[t.team]
	if n == nil {
		return false
	}

	for cy := y - 1; cy <= y+fp; cy++ {
		for cx := x - 1; cx <= x+fp; cx++ {
			// inside the footprint, not a bordering cell
			if cx >= x && cx < x+fp && cy >= y && cy < y+fp {
				continue
			}

			if cx < 0 || cy < 0 || cx >= g.W || cy >= g.H {
				continue
			}

			for _, o := range g.Units.Cells.At(cx, cy) {
				if o.Team == t.team && o.Flags.Has(game.FlagStationary) && !o.Flags.Has(game.FlagGroundCover) {
					if c := n.ComplexOf(o); c != nil && c.Material >= costPerTurn {
						return true
					}
				}
			}
		}
	}

	return false
}
