// buildorders.go : runs the in flight build orders: start, advance, finish and cancel.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - build order progress and BuildingReady
package sim

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// StartBuild begins constructing product (a pre built, unplaced unit) at (gx, gy)
// by builder at the given rate; the build loop advances the order each turn.
// C++: GameManager issuing ORDER_BUILD / a factory's StartBuilding (unitinfo.cpp).
func (g *Game) StartBuild(builder, product *game.UnitInfo, gx, gy, rate int) *build.BuildOrder {
	o := build.NewBuildOrder(builder, product, gx, gy, rate)
	g.builds = append(g.builds, o)
	// Committing to a build supersedes any queued move the builder still held, so it
	// cannot auto resume next turn and drive the parked build ready builder off its
	// site (skipping ExitBuildSite and stranding it under its finished structure).
	delete(g.pending, builder)
	// Only mobile constructors and engineers enter the build pose. Factories build internally and remain stationary even when they carry CONSTRUCTOR_UNIT.
	if builder != nil && builder.Flags.Has(game.FlagConstructorUnit) && !builder.Flags.Has(game.FlagStationary) {
		// Re bucket the builder across the whole build site: drop its single cell
		// entry, enter the build pose (which sets the footprint), then re add so the
		// cell hash reserves every site cell while the structure is raised.
		rebucket := g.Units != nil && g.Units.Cells != nil
		if rebucket {
			g.Units.Cells.Remove(builder)
		}
		builder.StartBuildPose(gx, gy, build.Footprint(product.Flags))
		if rebucket {
			g.Units.Cells.Add(builder)
		}
	}
	return o
}

// BuildOrders exposes the active build orders for read only inspection; the
// in game screen draws an "under construction" overlay on each one's site.
func (g *Game) BuildOrders() []*build.BuildOrder {
	return g.builds
}

// BuildOrderFor returns the active build order driven by builder, or nil.
func (g *Game) BuildOrderFor(builder *game.UnitInfo) *build.BuildOrder {
	for _, o := range g.builds {
		if o.Builder == builder {
			return o
		}
	}
	return nil
}

// CancelBuild drops any build order owned by builder (e.g. the site turned out
// to be unreachable, or the player cancels before work begins).
func (g *Game) CancelBuild(builder *game.UnitInfo) {
	kept := g.builds[:0]
	for _, o := range g.builds {
		if o.Builder != builder {
			kept = append(kept, o)
		}
	}
	g.builds = kept
	if builder != nil {
		// Re bucket from the reserved site footprint back to the builder's single
		// cell: remove (while the footprint still keys the hash), clear the pose,
		// re add. Skip the hash dance if it was never registered in the build pose.
		if builder.Building {
			fp := builder.BuildFootprint // read before BeginBuildLeave clears it
			g.Units.Cells.Remove(builder)
			// Slide the builder back out to its own cell instead of leaving it stranded
			// at the footprint centre off grid (the cancel time mirror of the bulldozer's
			// BeginClearLeave); the gamescreen slide loop animates it and frees the unit.
			builder.BeginBuildLeave(fp)
			g.Units.Cells.Add(builder)
		} else {
			builder.EndBuildPose()
		}
	}
}

// buildPool returns the material store a build order draws from. Mobile player builders spend their own cargo; computer builders refill from their
// connected complex; stationary factories use the shared complex pool. Every path stalls when its available material is insufficient.
// C++ ported : unitinfo.cpp - UnitInfo::UpdateProduction (MOBILE spends storage -=
// Cargo_GetRawConsumptionRate; only a STATIONARY factory draws complex material).
func (g *Game) buildPool(o *build.BuildOrder) *int {
	b := o.Builder
	if b == nil {
		return &o.Product.Storage // a factory's internal production (no separate builder)
	}
	// A player's mobile builder spends its own cargo only (no auto top up, no pool
	// fallback): it drains its load and stalls until the player refills it manually.
	if !b.Flags.Has(game.FlagStationary) && g.State.Teams[b.Team].Type != game.TeamComputer {
		return &b.Storage
	}
	cost := o.CostPerTurn()
	var supply *economy.Complex
	if g.buildPaths[b] == nil {
		supply = g.buildSupplyComplex(o)
		if supply != nil {
			topUpBuilderCargo(b, supply) // pool -> builder cargo, up to capacity (AI only)
		}
	}
	if b.Storage >= cost {
		return &b.Storage
	}
	// Own cargo still short (capacity too small to cover the turn, or the pool nearly
	// empty): draw directly from the connected pool if it can cover this turn.
	if supply != nil && supply.Material >= cost {
		return &supply.Material
	}
	return &b.Storage // nothing reachable covers this turn: stall on the builder's own cargo
}

// BuildMaterial reports the raw a builder can spend on a new job. Factories use their base pool; mobile builders use their own cargo, with computer
// builders topped up from the base each turn. The AI uses this value to choose a build rate.
//
// C++ ported : unitinfo.cpp - UpdateProduction, which clamps any builder's rate with
// BuildMenu_GetMaxPossibleBuildRate(storage).
func (g *Game) BuildMaterial(builder *game.UnitInfo) int {
	if builder == nil {
		return 0
	}

	if n := g.Networks[builder.Team]; n != nil && builder.Flags.Has(game.FlagStationary) {
		if c := n.ComplexOf(builder); c != nil {
			return c.Material
		}
	}

	return builder.Storage
}

// buildSupplyComplex returns the stocked complex a build order is connected to: the
// builder's own complex if it is a member, else a team complex bordering the build
// site (the connectors distribute resources to a builder working next to the base).
// Returns nil for a builder building away from any base.
func (g *Game) buildSupplyComplex(o *build.BuildOrder) *economy.Complex {
	b := o.Builder
	n := g.Networks[b.Team]
	if n == nil {
		return nil
	}
	if c := n.ComplexOf(b); c != nil {
		return c
	}
	return g.complexAdjacentTo(b.Team, o.GridX, o.GridY, build.Footprint(o.Product.Flags))
}

// topUpBuilderCargo moves raw from a connected complex pool into a mobile builder's
// cargo up to capacity, so the build then spends the builder's OWN storage and its
// bar shows supply draining + refilling (AI stats must look real when inspected).
// Conserved: material only moves pool -> builder. No op with no capacity or no pool.
func topUpBuilderCargo(b *game.UnitInfo, c *economy.Complex) {
	if b.Values == nil {
		return
	}
	capacity := b.Values.Get(game.AttribStorage)
	if capacity <= 0 || b.Storage >= capacity || c.Material <= 0 {
		return
	}
	take := capacity - b.Storage
	if take > c.Material {
		take = c.Material
	}
	c.Material -= take
	b.Storage += take
}

// complexAdjacentTo returns the RICHEST complex of team owning a stationary building
// in the ring just outside the fpxfp build site at (sx, sy), or nil. Lets a builder
// raising a structure next to the base draw on the base's pool. Richest matters when a
// site borders both the stocked base and an empty detached island (a bridging
// connector): it draws the base, not the 0-material island.
func (g *Game) complexAdjacentTo(team, sx, sy, fp int) *economy.Complex {
	n := g.Networks[team]
	if n == nil || g.Units == nil || g.Units.Cells == nil {
		return nil
	}
	var best *economy.Complex
	for y := sy - 1; y <= sy+fp; y++ {
		for x := sx - 1; x <= sx+fp; x++ {
			for _, o := range g.Units.Cells.At(x, y) {
				if o.Team == team && o.Flags.Has(game.FlagStationary) {
					if c := n.ComplexOf(o); c != nil && (best == nil || c.Material > best.Material) {
						best = c
					}
				}
			}
		}
	}
	return best
}

// AdvanceBuilds applies one turn of construction for a team: each build order draws
// material and progresses; a finished order deploys its product. Returns the builders
// that stalled this turn for lack of material, so the UI can prompt a refill.
// C++: the per turn ORDER_STATE_BUILD_IN_PROGRESS step (units_manager.cpp).
func (g *Game) AdvanceBuilds(team int) (stalled []*game.UnitInfo) {
	g.pathShort = nil // tickBuildPaths below records this pass's short path builders
	kept := g.builds[:0]
	for _, o := range g.builds {
		// Advance only orders owned by this team. ProductTeam persists ownership when an order has no builder.
		// C++: ORDER_STATE_BUILD steps only for the active team (GameManager_ProgressBuildState).
		if o.Product.Team != team {
			kept = append(kept, o)
			continue
		}
		// A constructor must reach the site before work begins: while it is
		// still walking over, the order holds without progressing. Once it
		// arrives the build is committed (Started) and runs to completion even
		// if the unit is later moved.
		if !o.Started {
			if !builderAtSite(o) {
				kept = append(kept, o)
				continue
			}
			o.Started = true
		}
		// A factory produces nothing this turn (order held, partial build KEPT) if idled
		// (OrderPowerOff), power STARVED (deficit cut it first), or STOPPED (BuildRate 0).
		// A power/pause hold auto resumes. A constructor's order is unaffected.
		if isFactoryOrder(o) && (!economy.IsActive(o.Builder) || o.Builder.PowerStarved || factoryPaused(o.Builder)) {
			kept = append(kept, o)
			continue
		}
		progressed := o.Advance(g.buildPool(o))
		if !o.Done() {
			if !progressed && o.Builder != nil {
				stalled = append(stalled, o.Builder) // out of material this turn
				// A running factory without raw material is ProdStalled until its order progresses again.
				if isFactoryOrder(o) {
					o.Builder.RawStarved = true
				}
			} else if o.Builder != nil {
				o.Builder.RawStarved = false
			}
			kept = append(kept, o)
			continue
		}
		if o.Builder != nil {
			o.Builder.RawStarved = false
		}
		// A FACTORY order (a stationary, non constructor builder turning out a mobile
		// unit): the finished unit parks ON the factory in a build ready state with
		// BLDMRK exit markers; the player (or AI) drives it out, and only THEN does the
		// next queued unit start (sweepFactoryReady). C++: ORDER_STATE_UNIT_READY output.
		if isFactoryOrder(o) {
			g.completeFactoryOutput(o, team)
			g.popFactoryQueue(o.Builder) // advance the queue; the next unit waits for the exit
			continue                     // order consumed
		}
		g.finishBuild(o, team)
	}
	g.builds = kept
	// A Path build whose cell ran out of material stops (the stalled message fires);
	// the rest of the runs step one cell - build the cell the engineer reached, or
	// drive it to the next.
	g.abandonStalledPaths(stalled)
	g.tickBuildPaths(team)
	return stalled
}

// builderAtSite reports whether o's builder has reached its site: its cell touches
// the product's footprint (Chebyshev <= 1). A builder still en route does not
// progress; a builder less order (a factory's own production) always counts ready.
// C++: ORDER_BUILD enters BUILD_IN_PROGRESS only once positioned (StartBuilding).
func builderAtSite(o *build.BuildOrder) bool {
	b := o.Builder
	// Only a mobile constructor travels to its site; a builder less or stationary
	// factory order is always ready. A mission factory carries CONSTRUCTOR_UNIT, so
	// gate on stationary too, not the flag alone.
	if b == nil || b.Flags.Has(game.FlagStationary) || !b.Flags.Has(game.FlagConstructorUnit) {
		return true
	}
	fp := build.Footprint(o.Product.Flags)
	return axisGap(b.GridX, o.GridX, o.GridX+fp-1) <= 1 &&
		axisGap(b.GridY, o.GridY, o.GridY+fp-1) <= 1
}

// axisGap returns how far p lies outside the inclusive range [lo, hi]
// (0 when inside).
func axisGap(p, lo, hi int) int {
	switch {
	case p < lo:
		return lo - p
	case p > hi:
		return p - hi
	default:
		return 0
	}
}

// finishBuild deploys a completed product and gives it full starting stats.
func (g *Game) finishBuild(o *build.BuildOrder, team int) {
	p := o.Product
	g.nextBuilt++
	p.ID = uint16(10000 + g.nextBuilt)
	p.Team = team
	p.InitState()
	// Free the builder's reserved construction footprint before placing the
	// product, so the site cells are clear and the builder never blocks itself.
	// Remove must precede EndBuildPose (which clears the footprint the cell hash
	// keys off).
	b := o.Builder
	// A real constructor/engineer is MOBILE. Require !Stationary so a mission
	// factory (which carries the CONSTRUCTOR_UNIT flag) is never parked as a
	// constructor here; isFactoryOrder already routes it to completeFactoryOutput.
	builderIsConstructor := b != nil && b.Flags.Has(game.FlagConstructorUnit) &&
		!b.Flags.Has(game.FlagStationary)
	// Flat terrain specials free the engineer immediately; only a building parks its builder until it drives off.
	flatSpecial := access.IsTerrainSpecial(p.Type) || p.Flags.Has(game.FlagGroundCover)
	needsExit := builderIsConstructor && !flatSpecial
	if needsExit {
		// Take the builder out of the cell hash while the structure is placed so the
		// site cells are clear, but KEEP its build pose: it stays parked CENTRED on
		// the footprint (the position the build slide glided it to) in the build
		// frame until the player drives it off. EndBuildPose is deferred to the exit
		// (clearBuildReady) so it does not snap back to the corner cell here.
		g.Units.Remove(b)
	}
	g.placeFinishedProduct(p, o)
	// A finished structure auto clears any debris pile under its footprint, the way
	// the original's build completion destroys the underlying rubble; a connector is the one
	// exception (it leaves the pile). No salvage - that is the bulldozer's job.
	g.clearFootprintRubble(p, o.GridX, o.GridY)
	// A mobile builder raised the structure from inside its footprint and must leave.
	// Park it on the site origin (shares those cells with the new building, a different
	// movement class, so not blocked) in the build ready state: a human clicks a BLDMRK
	// exit marker; an AI picks one next turn (buildExits/issueBuildExits). Either way it
	// DRIVES off, never teleports.
	// C++: on ORDER_STATE_UNIT_READY the builder waits parked; UnitsManager_BuildingReady
	// completes the structure only once the constructor leaves its bounds.
	if builderIsConstructor && flatSpecial {
		g.freeFlatSpecialBuilder(b)
	}
	if needsExit {
		g.parkBuildReadyBuilder(b, p, o, team)
	}
	// A Path (line) build just finished this cell: advance to the next one, which
	// tickBuildPaths starts once the engineer is free and standing on it.
	if b != nil {
		if bp := g.buildPaths[b]; bp != nil {
			bp.Idx++
		}
	}
	g.RefreshVisibility()
}

// placeFinishedProduct puts the completed product on the map: a structure goes down
// through PlaceBuilding on the default order for its type, anything else is deployed.
func (g *Game) placeFinishedProduct(p *game.UnitInfo, o *build.BuildOrder) {
	if p.Flags.Has(game.FlagStationary) {
		// A new building's default order by type, per UnitsManager_DeployUnit:
		// - Mine/HABITAT/Gold Refinery (CommTwr): PowerOn (passive providers; an off
		//   habitat houses no workers, blocking every worker consumer).
		// - Defense building (any stationary unit with an attack value): OrderSentry.
		// - All others (factories, Eco Sphere, research, power, depots): PowerOff.
		switch {
		case p.Type == assets.MiningSt || p.Type == assets.Habitat || p.Type == assets.CommTwr:
			p.Order = game.OrderPowerOn
		case p.Values != nil && p.Values.Get(game.AttribAttack) > 0:
			p.Order = game.OrderSentry
		default:
			p.Order = game.OrderPowerOff
		}
		p.State = game.OrderStateExecutingOrder
		g.PlaceBuilding(p, o.GridX, o.GridY)
	} else {
		g.Units.Deploy(p, o.GridX, o.GridY)
	}
}

// freeFlatSpecialBuilder releases a builder that just laid a flat terrain special.
//
// A flat special needs no drive off: drop the 1x1 build pose and free the builder where
// it stands (it laid the road/bridge/platform on its own cell and simply stays on it).
// C++: SetParent(nullptr) + ORDER_AWAIT immediately.
func (g *Game) freeFlatSpecialBuilder(b *game.UnitInfo) {
	if b.Building {
		g.Units.Cells.Remove(b)
		b.EndBuildPose()
		g.Units.Cells.Add(b)
	} else {
		b.EndBuildPose()
	}
}

// parkBuildReadyBuilder leaves the builder parked on the finished footprint in the build
// ready state, hides the structure behind it until it drives off, and queues an automatic
// exit for a computer team.
func (g *Game) parkBuildReadyBuilder(b, p *game.UnitInfo, o *build.BuildOrder, team int) {
	// Re index the parked builder WITHOUT SetPosition (Deploy would snap its pixel
	// centre to the corner cell): keep it drawn centred on the footprint. Building/
	// BuildSite stay set, so the cell hash re buckets it across the whole site.
	g.Units.Cells.Add(b)
	g.Units.ByID.Add(b)
	b.Order = game.OrderBuild
	b.State = game.OrderStateUnitReady
	// The structure stays HIDDEN behind the parked builder until the builder
	// clears the footprint; only then does it appear and unfold (Unfold 4 -> 0)
	// into place. C++: UnitsManager_BuildingReady map status adds the structure and
	// scales it up (ORDER_STATE_EXPAND) only once the constructor leaves its bounds.
	p.Unfold = 4
	p.HiddenUntilExpand = true
	if g.buildReady == nil {
		g.buildReady = make(map[*game.UnitInfo]*BuildReady)
	}
	g.buildReady[b] = &BuildReady{
		Product:   p.Type,
		Building:  p,
		SiteX:     o.GridX,
		SiteY:     o.GridY,
		Footprint: build.Footprint(p.Flags),
	}
	// A computer team has no player to click a marker, so queue an automatic
	// drive off to the nearest free cell for the start of its next turn.
	if g.State.Teams[team].Type != game.TeamPlayer {
		if bx, by, ok := g.findReachableSpot(b, o.GridX, o.GridY, build.Footprint(p.Flags)); ok {
			if g.buildExits == nil {
				g.buildExits = make(map[*game.UnitInfo][2]int)
			}
			g.buildExits[b] = [2]int{bx, by}
		}
	}
}

// issueBuildExits drives each of team's parked builders off its finished footprint
// to its recorded exit cell. Called from BeginTeamTurn after budgets refill so the
// one step drive can afford it. A builder with no drivable path is repositioned
// without animating; a destroyed one is dropped.
// C++: the builder leaving on ORDER_STATE_UNIT_READY (units_manager.cpp).
func (g *Game) issueBuildExits(team int) {
	// A computer team's parked builder with NO recorded exit re derives one here.
	// buildExits is not persisted, so a restored NPC builder would otherwise sit parked
	// forever (no player to click a marker). Human builders are left for the player.
	// C++: the AI re issues the ORDER_STATE_UNIT_READY exit each turn.
	if g.State.Teams[team].Type != game.TeamPlayer {
		for b, br := range g.buildReady {
			if b.Team != team || br.Factory {
				continue // factories deploy via their own path; only mobile builders drive off
			}
			if _, ok := g.buildExits[b]; ok {
				continue // already has a queued exit
			}
			if bx, by, ok := g.findReachableSpot(b, br.SiteX, br.SiteY, br.Footprint); ok {
				if g.buildExits == nil {
					g.buildExits = make(map[*game.UnitInfo][2]int)
				}
				g.buildExits[b] = [2]int{bx, by}
			}
		}
	}
	for b, dst := range g.buildExits {
		if b.Team != team {
			continue
		}
		// A FACTORY entry: the produced UNIT materialises on its deploy cell and grows in
		// (Unfold 4 -> 0), same as a player marker exit - it does NOT drive out. Leave the
		// build ready record alone; sweepFactoryReady clears it and starts the next queued
		// unit once the produced unit has vacated the footprint.
		if br := g.buildReady[b]; br != nil && br.Factory {
			delete(g.buildExits, b) // one shot: deploy it once
			p := br.Building
			if p == nil || p.Hits <= 0 {
				continue
			}
			g.Units.Move(p, dst[0], dst[1]) // materialise on the deploy cell
			p.Unfold = 4
			p.HiddenUntilExpand = false
			g.expandReleased = append(g.expandReleased, p) // queue the grow in animation
			continue
		}
		if b.Hits <= 0 {
			g.clearBuildReady(b) // destroyed before it could leave; also drops buildExits
			continue
		}
		br := g.buildReady[b]
		g.armExpand(b)       // the building unfolds once this builder clears it
		g.clearBuildReady(b) // leaving the site clears build ready (and buildExits)
		if br != nil {
			nx, ny := nearestFootprintCell(br, dst[0], dst[1])
			g.Units.Move(b, nx, ny) // re seat on the near side before the drive off
		}
		if !g.OrderMove(b, dst[0], dst[1]) {
			g.Units.Move(b, dst[0], dst[1]) // no drivable path out: reposition without animating
		}
	}
}
