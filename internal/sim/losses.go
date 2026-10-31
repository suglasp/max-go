// losses.go : destroys units and remembers where a team recently lost one.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - UnitInfo::Destroy
package sim

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// DestroyUnit removes a unit from the game. A destroyed building also has its
// foundation slab cleared, leaving only rubble on the bare ground.
// C++: the destroyed building leaves rubble in place of its slab (ORDER_EXPLODE).
// LossSite is a cell where a team lost a mobile unit, with the turn it happened. The
// AI reads these (RecentLosses) to avoid feeding more units into a fresh kill zone.
type LossSite struct {
	X, Y, Turn int // loss cell and turn number
}

// lossSiteRetention is how many turns sim keeps a loss site before pruning it. It is
// deliberately larger than the longest AI forget window, so the difficulty scaled
// window the AI applies on top of RecentLosses is never truncated by this prune.
const lossSiteRetention = 40

// recordLoss remembers the cell where a MOBILE unit just died, keyed by its team, so
// RecentLosses can report it to the AI. A downed building is skipped: a destroyed
// structure near a base must not mark that ground dangerous for the team's own movers.
// Stale sites are pruned as we record, keeping the per team slice bounded.
func (g *Game) recordLoss(u *game.UnitInfo) {
	if g.State == nil || u == nil || u.Team < 0 || u.Team >= game.TeamCount {
		return
	}

	mobile := u.Flags.Has(game.FlagMobileLandUnit) || u.Flags.Has(game.FlagMobileAirUnit) || u.Flags.Has(game.FlagMobileSeaUnit)
	if !mobile {
		return // only a lost mobile unit marks a spot; a downed building does not
	}

	if g.lossSites == nil {
		g.lossSites = make(map[int][]LossSite)
	}

	now := g.State.TurnCounter
	kept := g.lossSites[u.Team][:0]
	for _, s := range g.lossSites[u.Team] {
		if now-s.Turn < lossSiteRetention {
			kept = append(kept, s)
		}
	}

	g.lossSites[u.Team] = append(kept, LossSite{X: u.GridX, Y: u.GridY, Turn: now})
}

// RecentLosses returns the cells where team recently lost a mobile unit (within the
// sim retention window), each tagged with the turn it happened. The AI applies its own
// difficulty scaled forget window on top. Returns nil when the team has none.
// C++: n/a (a port aid; the AI shies from a fresh loss it cannot yet explain).
func (g *Game) RecentLosses(team int) []LossSite {
	if g.lossSites == nil {
		return nil
	}

	return g.lossSites[team]
}

// RestoreLossSites restores a team's recent loss memory so its AI can avoid ground where it recently lost a unit.
func (g *Game) RestoreLossSites(team int, sites []LossSite) {
	if team < 0 || team >= game.TeamCount || len(sites) == 0 {
		return
	}
	if g.lossSites == nil {
		g.lossSites = make(map[int][]LossSite)
	}

	g.lossSites[team] = append([]LossSite(nil), sites...)
}

// purgeCarriedResidue drops the sim side state keyed by units stored INSIDE a
// destroyed carrier (recursing into a carried transport): a disabled passenger
// otherwise leaves a stale countdown entry forever, since DestroyUnit purges
// only the carrier's own keys and the passengers never pass through it.
func purgeCarriedResidue(g *Game, u *game.UnitInfo) {
	for _, c := range u.Carried {
		delete(g.disabled, c)
		delete(g.pending, c)
		delete(g.held, c)
		purgeCarriedResidue(g, c)
	}
}

// DestroyUnit removes a unit from the game and clears every reference the rest of the
// sim still holds to it: pursuit targets, spotted memory, queued and held routes, build
// exits. Missing one of those leaves a dangling pointer to a dead unit.
// C++ ported : unitinfo.cpp - UnitInfo::Destroy
func (g *Game) DestroyUnit(u *game.UnitInfo) {
	g.recordLoss(u) // remember where a mobile unit died so the AI shies off the spot

	// A builder killed while parked on its finished building takes the never shown
	// building with it, slab and all: no casualty tally, no rubble.
	// C++ ported : units_manager.cpp - UnitsManager_BuildingReady, dead parent inside the bounds
	if br := g.buildReady[u]; br != nil && !br.Factory && br.Building != nil && br.Building.HiddenUntilExpand {
		g.purgeUnit(br.Building)
		g.Units.Remove(br.Building)
	}

	g.purgeUnit(u)
	g.recordCasualty(u) // score screen tally (buildings / units lost, incl. any carried)
	g.recordRubble(u)   // leave debris where it stood (a Bulldozer can later clear it)
	g.Units.Remove(u)
}

// purgeUnit clears every sim reference to a unit that is leaving the game: orders,
// pursuit, spotted memory, routes, build records, its slabs, build orders on its cells
// and its complex. The caller removes the unit itself and does any bookkeeping.
func (g *Game) purgeUnit(u *game.UnitInfo) {
	g.cancel(u)
	g.dropPursuitOf(u)         // any unit chasing this one drops its dangling pursuit pointer
	g.forgetSpotted(u)         // and every team forgets it (no chasing a ghost cell)
	delete(g.pending, u)       // drop any queued route so we keep no stale pointer
	delete(g.held, u)          // and any route it was holding after an enemy halt
	delete(g.buildExits, u)    // drop any pending build exit drive likewise
	delete(g.buildReady, u)    // drop any build ready record likewise
	delete(g.pendingExpand, u) // drop any pending unfold keyed by this building
	delete(g.disabled, u)      // drop any commando disable countdown likewise
	purgeCarriedResidue(g, u)  // the cargo dies with its carrier: purge each passenger's residue too
	g.abortClear(u)            // drop any in progress rubble clear + its cones/tape
	un := 1
	if u.Flags.Has(game.FlagBuilding) {
		un = 2
	}
	if u.Flags.Has(game.FlagStationary) {
		g.invalidateReach() // a destroyed building/bridge changes terrain connectivity
		slabs := map[*game.UnitInfo]bool{}
		for dy := 0; dy < un; dy++ {
			for dx := 0; dx < un; dx++ {
				for _, e := range g.Units.Cells.At(u.GridX+dx, u.GridY+dy) {
					if e != u && e.Flags.Has(game.FlagGroundCover) {
						slabs[e] = true
					}
				}
			}
		}
		for sl := range slabs {
			g.Units.Remove(sl)
		}
	}

	// Drop any construction order tied to this unit; it was the builder, or it
	// is the building/product standing on that site; so the build site overlay
	// (slab/tape/cones drawn from build orders) stops rendering after the blast.
	kept := g.builds[:0]
	for _, o := range g.builds {
		on := 1
		if o.Product != nil && o.Product.Flags.Has(game.FlagBuilding) {
			on = 2
		}
		overlaps := u.GridX < o.GridX+on && o.GridX < u.GridX+un &&
			u.GridY < o.GridY+on && o.GridY < u.GridY+un
		if o.Builder == u || overlaps {
			continue // drop this order
		}
		kept = append(kept, o)
	}
	g.builds = kept

	// A destroyed building/connector must also LEAVE its complex: else it lingers in
	// Complex.Members and the economy keeps crediting it (a blown up Habitat still
	// "housed" its workers, so the worker shed never fired). ComplexOf returns nil for
	// a mobile unit or the alien faction, so this only touches a real network member.
	if c := g.ComplexOf(u); c != nil {
		housedBefore, drawnBefore, _ := c.LifeReport()
		g.network(u.Team).Remove(u)
		g.syncComplexes()
		dbg.Printf("destroy: building type=%d team=%d left complex %d (complex workers before: housed=%d drawn=%d); economy reconciles next turn", u.Type, u.Team, c.ID, housedBefore, drawnBefore)
	}
}
