// task_attack_search.go : hunts a target the team has lost sight of: the zone sweep,
// the patience timer, and shelving a contact it cannot reacquire.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : zonewalker.cpp - ZoneWalker, plus AiPlayer aging out a stale SpottedUnit
package ai

import (
	"math/rand"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

const (
	// zoneSearchRadius is the radius of the disc a unit sweeps around a lost contact's
	// last seen cell in the ZoneWalker search: big enough to have unscanned ground to
	// enter, small enough to stay a LOCAL reacquire sweep. Interpreted value.
	zoneSearchRadius = 6

	searchZone    = 1 // a focused ZoneWalker sweep around the last sighting (the original)
	searchExplore = 2 // hand the idle unit to the generic taskExplore wander (ours)

	// searchPatience is how many turns one unit may search a single lost target before
	// giving up on it (its last seen cell is then shelved). Bounds a fruitless hunt.
	searchPatience = 8
	// abandonCooldown is how many turns a shelved (searched out or timed out) fogged
	// contact is skipped before the team retries it; seeing it again clears it sooner.
	abandonCooldown = 15
)

// zoneSearchChance is the probability a unit that just lost sight of its target runs
// the focused zone sweep (searchZone) rather than the generic explore wander
// (searchExplore). Equal by default so the AI varies how it searches. pickZoneSearch
// does the roll and is a package var so tests can force a branch.
var zoneSearchChance = 0.5

var pickZoneSearch = func() bool { return rand.Float64() < zoneSearchChance }

// beginSearch runs when a unit reaches a lost target's last seen cell. It selects and latches either a focused ZoneWalker sweep or a generic
// explore search, then takes the first step.
func (t *taskAttackEnemies) beginSearch(u, target *game.UnitInfo, cx, cy int) {
	if t.searchMode == nil {
		t.searchMode = make(map[*game.UnitInfo]int)
	}

	if t.searchSince == nil {
		t.searchSince = make(map[*game.UnitInfo]int)
	}

	mode := searchExplore
	if pickZoneSearch() {
		mode = searchZone
	}

	t.searchMode[u] = mode
	t.searchSince[u] = t.ctx.Game.State.TurnCounter
	dbg.Printf("ai: team %d %s lost sight of %s, searching (%s) around (%d,%d)", t.team, u.Type.Tag(), target.Type.Tag(), searchModeName(mode), cx, cy)

	// A zone sweep that is empty on its very first step (nothing reachable left to
	// reveal around the sighting) means the search is futile: shelve the target now.
	if !t.runSearch(u, cx, cy, mode) {
		t.abandon(target)
		t.endSearch(u)
	}
}

// abandon shelves enemy e: nearestEnemy skips it while it stays fogged for the next
// abandonCooldown turns (seeing it again clears it sooner), so the team stops
// pursuing a contact it cannot reach or reacquire, again and again. C++: AiPlayer
// dropping a stale SpottedUnit.
func (t *taskAttackEnemies) abandon(e *game.UnitInfo) {
	if t.abandoned == nil {
		t.abandoned = make(map[*game.UnitInfo]int)
	}

	if _, done := t.abandoned[e]; !done {
		dbg.Printf("ai: team %d gives up on lost %s (shelving %d turns)",
			t.team, e.Type.Tag(), abandonCooldown)
	}

	t.abandoned[e] = t.ctx.Game.State.TurnCounter
}

// endSearch clears a unit's latched search state (mode + start turn).
func (t *taskAttackEnemies) endSearch(u *game.UnitInfo) {
	delete(t.searchMode, u)
	delete(t.searchSince, u)
}

// claimSearchCell reserves stale cell (x,y) for ONE searcher this turn, returning false
// if another unit already claimed it - so a stack of units does not all begin (and log)
// a search for the same cell. Reset each turn in BeginTurn.
func (t *taskAttackEnemies) claimSearchCell(x, y int) bool {
	if t.searchClaims == nil {
		t.searchClaims = make(map[int]bool)
	}

	key := y*t.ctx.Game.W + x
	if t.searchClaims[key] {
		return false
	}

	t.searchClaims[key] = true
	return true
}

// runSearch advances the unit's chosen search around the last seen cell (cx, cy) and
// reports whether it is still live. searchZone drives to the nearest unexplored reachable
// cell of the disc and returns false when it holds nothing new (a failed search, caller
// shelves the target). searchExplore leaves the unit idle for taskExplore and stays live.
func (t *taskAttackEnemies) runSearch(u *game.UnitInfo, cx, cy, mode int) bool {
	if mode == searchZone {
		return t.zoneSearch(u, cx, cy)
	}

	return true
}

// zoneSearch drives u to the nearest reachable unexplored cell within zoneSearchRadius of the enemy's last known position. It returns false once
// the disc is fully explored. ZoneWalker supplies the disc; this search selects the nearest cell without threat weighting.
// C++ ported : zonewalker.cpp - ZoneWalker (the disc enumeration).
func (t *taskAttackEnemies) zoneSearch(u *game.UnitInfo, cx, cy int) bool {
	g := t.ctx.Game
	if g.Vis == nil {
		return false
	}

	fog := g.Vis.Team(t.team)
	zw := newZoneWalker(game.Point{X: cx, Y: cy}, zoneSearchRadius, g.W, g.H)
	bestX, bestY, bestD, found := 0, 0, 1<<30, false

	for {
		x, y := zw.x, zw.y

		if (fog == nil || !fog.Explored(x, y)) && g.CanEnter(u, x, y) {
			if d := sim.SquaredDist(u.GridX, u.GridY, x, y); d < bestD {
				bestX, bestY, bestD, found = x, y, d, true
			}
		}

		if !zw.findNext() {
			break
		}
	}

	if found {
		return g.OrderMove(u, bestX, bestY)
	}

	return false
}

// searchModeName labels a search mode for the debug trace.
func searchModeName(mode int) string {
	if mode == searchZone {
		return "zone"
	}

	return "explore"
}
