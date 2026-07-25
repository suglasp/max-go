// power.go : wires team power, complex membership and the per turn economy pass.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : production_manager.cpp - power and resource reconciliation
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// network returns (lazily creating) the team's building network. Complex ids
// are namespaced per team so they never collide.
func (g *Game) network(team int) *build.Network {
	if g.Networks[team] == nil {
		g.Networks[team] = build.NewNetwork(team, team*1000+1)
	}
	return g.Networks[team]
}

// ComplexOf returns the complex a placed building belongs to, or nil.
//
// C++: UnitInfo::GetComplex (units_manager.cpp).
func (g *Game) ComplexOf(u *game.UnitInfo) *economy.Complex {
	if u == nil || u.Team < 0 || u.Team >= game.TeamCount || g.Networks[u.Team] == nil {
		return nil
	}
	return g.Networks[u.Team].ComplexOf(u)
}

// PowerShort reports whether any of team's complexes is in a power DEFICIT this turn -
// a powered consumer (factories are cut first) was starved by the balance. The in game
// screen flashes the red "not enough power" alert when true. C++: the power deficit warning.
func (g *Game) PowerShort(team int) bool {
	for _, c := range g.Complexes {
		if c != nil && c.Team == team && c.PowerShort() {
			return true
		}
	}
	return false
}

// PowerOnConsumer tries to power building u on. Returns false (caller BOUNCEs it,
// "not enough power") when even all generators cannot cover u's draw on top of the
// existing demand; else powers u on and re optimizes the generators. An idle factory
// draws nothing, so it always powers on.
// C++ ported : production_manager.cpp - CheckPowerNeed gating ProcessOrderPowerOn.
func (g *Game) PowerOnConsumer(u *game.UnitInfo) bool {
	if u == nil {
		return false
	}
	c := g.ComplexOf(u)
	if c == nil {
		return false // isolated: no connected generator to power it
	}
	if !c.CanAbsorbPower(economy.PowerOnDraw(u)) {
		return false // not enough generator capacity even with every generator on
	}
	u.Order = game.OrderPowerOn
	u.State = game.OrderStateExecutingOrder
	c.OptimizeProduction()
	return true
}

// SetConsumerPowerOff powers building u off and re optimizes its complex, powering
// DOWN any generator u's demand had kept on (the conserve path). Returns any buildings
// the rebalance shut (normally none: turning a consumer off only frees power). For a
// GENERATOR the optimizer may power it straight back on if its output is still needed
// (accepted near inert toggle).
// C++: ProcessOrderPowerOff + ProductionManager_OptimizeProduction.
func (g *Game) SetConsumerPowerOff(u *game.UnitInfo) []*game.UnitInfo {
	if u == nil {
		return nil
	}
	u.Order = game.OrderPowerOff
	u.State = game.OrderStateExecutingOrder
	if c := g.ComplexOf(u); c != nil {
		return c.OptimizeProduction()
	}
	return nil
}

// syncComplexes rebuilds the economy complex list from every team's network,
// so RunEconomy balances exactly the live building networks.
func (g *Game) syncComplexes() {
	var all []*economy.Complex
	for _, n := range g.Networks {
		if n != nil {
			all = append(all, n.Complexes...)
		}
	}
	g.Complexes = all
}

// ReconcilePower idles any of a team's buildings whose complex has no power source,
// without running a full economy turn. Scenario/bench setup calls it after bases are
// laid so a detached, unpowered building shows idle from the first frame (BalanceTurn
// does the same each turn, but the bench drops in before the first one).
func (g *Game) ReconcilePower(team int) {
	for _, c := range g.Complexes {
		if c.Team == team {
			c.IdleUnpowered()
		}
	}
}

// RunEconomy applies one turn of the team's economy: each complex balances its
// members' net production into the shared stores.
// C++: the per turn production cycle over a team's complexes (production_manager).
func (g *Game) RunEconomy(team int) {
	diff := g.MiningDifficulty
	if diff <= 0 {
		diff = 4
	}
	g.turnPowerLost = nil    // recomputed from each complex's shed below
	g.turnLifeLost = nil     // recomputed from each complex's worker shed below
	g.turnGeneratorsOn = nil // recomputed from each complex's turn start generator edges below
	for _, c := range g.Complexes {
		if c.Team == team {
			// Snapshot which generators are already running, so after the balance we can
			// name the ones the optimizer brought online on its own because power demand
			// rose this turn (the auto management is otherwise silent - see announceTurnStart).
			genWasOn := generatorPowerState(c)

			c.AutoBalanceFuel(g.Resources) // shift mining toward fuel so the base covers its generators
			// A Gold Refinery (COMMTWR) converts raw gold into spendable upgrade credits.
			// BalanceTurn applies the gold draw; RefineGold returns only the credit, capped
			// by the reserve present BEFORE the draw, landing in g.Gold[team] (the balance the
			// Upgrade screen spends). C++ ported : unitinfo.cpp - UpdateProduction (gold->credit).
			goldBefore := c.Gold
			housed, drawn, surplus := c.LifeReport() // pre shed worker balance (for tracing the worker shed)
			c.BalanceTurn(diff)
			g.turnPowerLost = append(g.turnPowerLost, c.LastShed()...)                           // buildings shut for lack of power
			g.turnLifeLost = append(g.turnLifeLost, c.LastLifeShed()...)                         // buildings shut for lack of workers
			g.turnGeneratorsOn = append(g.turnGeneratorsOn, generatorsPoweredOn(c, genWasOn)...) // generators the optimizer brought online
			if surplus < 0 || len(c.LastLifeShed()) > 0 {
				dbg.Printf("economy team %d complex %d: workers housed=%d drawn=%d surplus=%d -> life-shed %d building(s)", team, c.ID, housed, drawn, surplus, len(c.LastLifeShed()))
			}
			if team >= 0 && team < game.TeamCount {
				g.Gold[team] += c.RefineGold(goldBefore)
			}
		}
	}
}

// LastPowerLost returns the consumers shed or held for lack of power during the most
// recent RunEconomy, so the in game screen can name them in a "not enough power"
// banner for the team whose turn just started (mirrors LastStalledBuilders).
func (g *Game) LastPowerLost() []*game.UnitInfo {
	return g.turnPowerLost
}

// LastLifeLost returns the personnel buildings powered off for lack of workers during
// the most recent RunEconomy, so the screen can name them in a "not enough workers"
// banner for the team whose turn just started (mirrors LastPowerLost).
func (g *Game) LastLifeLost() []*game.UnitInfo {
	return g.turnLifeLost
}

// LastGeneratorsPoweredOn returns the generators the optimizer brought online on its own
// during the most recent RunEconomy - power demand rose between turns (a queued factory
// began drawing, or the player powered a consumer on), so an idle generator was needed.
// The screen surfaces it at turn start; the direct on/off toggle already reports itself.
func (g *Game) LastGeneratorsPoweredOn() []*game.UnitInfo {
	return g.turnGeneratorsOn
}

// isGeneratorType reports whether t is a power generator or station (mirrors the
// unexported economy.isGenerator, used here to spot generator members of a complex).
func isGeneratorType(t assets.UnitType) bool {
	return t == assets.PowGen || t == assets.PowerStn
}

// generatorPowerState snapshots which of a complex's generators are powered on right now,
// so a later comparison can spot the ones the optimizer turns on by itself.
func generatorPowerState(c *economy.Complex) map[*game.UnitInfo]bool {
	state := make(map[*game.UnitInfo]bool)
	for _, u := range c.Members {
		if isGeneratorType(u.Type) {
			state[u] = u.Order == game.OrderPowerOn
		}
	}
	return state
}

// generatorsPoweredOn returns the complex's generators that were off in wasOn but are now
// powered on - the ones the optimizer brought online this balance. Net turn over turn
// edge: any off then on thrash within a single reconcile is invisible.
func generatorsPoweredOn(c *economy.Complex, wasOn map[*game.UnitInfo]bool) []*game.UnitInfo {
	var on []*game.UnitInfo
	for _, u := range c.Members {
		if isGeneratorType(u.Type) && u.Order == game.OrderPowerOn && !wasOn[u] {
			on = append(on, u)
		}
	}
	return on
}

// accrueScore adds the team's turn score: each operating Eco Sphere (a powered
// Greenhouse) earns one point (feeds the score victory + win/loss tiebreaker). Called
// from EndTeamTurn once per team turn (RunEconomy runs twice per round, so accruing
// there would double count). C++ ported : unitinfo.cpp - GREENHSE && ORDER_POWER_ON.
func (g *Game) accrueScore(team int) {
	if team < 0 || team >= game.TeamCount {
		return
	}
	// Only OPERATING Eco Spheres score: a powered off OR power STARVED Greenhouse
	// adds nothing, so the colonization score PLATEAUS while a team's colony is
	// stopped and resumes when power returns (SnapshotScoreHistory then graphs the
	// flat run). operatingEcoSphereUnits applies both gates.
	spheres := g.operatingEcoSphereUnits(team)
	for _, u := range spheres {
		u.Storage++ // the sphere banks its point as cargo too, unclamped (klei ++storage)
	}
	operating := len(spheres)
	g.TeamPoints[team] += operating
	// Detect colony start and stop transitions from the same count.
	g.detectColonyEdges(team, operating)
}
