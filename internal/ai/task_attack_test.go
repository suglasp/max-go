// task_attack_test.go : testing unit for task_attack.go, task_attack_search.go and task_attack_approach.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/vis"
)

// withFog turns the fixture's fog on (a fresh view) and refreshes it from the units.
func withFog(f *attackFixture) {
	f.g.Vis = vis.NewView(24, 24)
	f.g.RefreshVisibility()
}

// durableEnemy deploys a team 0 tank that survives a shot and stays put.
func durableEnemy(f *attackFixture, id, x, y int) *game.UnitInfo {
	e := armedTank(0, id, x, y, f.units)
	e.Values.Set(game.AttribHits, 400)
	e.Hits = 400

	return e
}

// lostContact deploys a team 0 tank the hunter sees at (x, y), then moves it far south into
// fog, so the team keeps a last seen cell at (x, y).
func lostContact(f *attackFixture, id, x, y int) *game.UnitInfo {
	e := durableEnemy(f, id, x, y)
	f.g.RefreshVisibility()
	f.units.Move(e, x, 23)
	f.g.RefreshVisibility()

	return e
}

// TestCommittedAttackerFiresOnArrival checks the visible target path: a committed attacker out
// of range advances and its shot goes off when it parks in range.
func TestCommittedAttackerFiresOnArrival(t *testing.T) {
	setMinStrikeForce(t, 1)
	f := newAttackFixture(t)
	tank := armedTank(1, 1, 6, 8, f.units)
	durableEnemy(f, 90, 12, 8) // 6 cells: out of range 4
	start := tank.Shots

	f.attack.BeginTurn()
	f.g.ResolveMovement()

	if tank.GridX == 6 {
		t.Fatal("the attacker did not advance")
	}
	if tank.Shots >= start {
		t.Errorf("shots %d (was %d): the attacker did not fire when it parked in range", tank.Shots, start)
	}
}

// TestLostContactSearchHasOneSearcherPerCell checks the hunt for a lost target: units that can
// get no closer to the last seen cell begin a search, but only one of them per stale cell.
func TestLostContactSearchHasOneSearcherPerCell(t *testing.T) {
	setMinStrikeForce(t, 1)
	prev := pickZoneSearch
	pickZoneSearch = func() bool { return false } // the explore mode: the searcher holds
	t.Cleanup(func() { pickZoneSearch = prev })

	f := newAttackFixture(t)
	withFog(f)
	first := armedTank(1, 1, 11, 10, f.units)
	second := armedTank(1, 2, 13, 10, f.units)
	first.Values.Set(game.AttribScan, 7)
	lostContact(f, 90, 12, 9) // last seen beside both: neither can get closer

	f.attack.BeginTurn()

	searching := 0
	for _, u := range []*game.UnitInfo{first, second} {
		if _, ok := f.attack.searchMode[u]; ok {
			searching++
		}
	}
	if searching != 1 {
		t.Errorf("searchers = %d, want exactly 1 for the one stale cell", searching)
	}
	if _, ok := f.attack.searchMode[first]; !ok {
		t.Error("the first unit in ID order did not take the stale cell")
	}
}

// TestSearchGivesUpAfterItsPatience checks the search limit: a searcher within its patience
// keeps hunting, one past it shelves the target and ends its search.
func TestSearchGivesUpAfterItsPatience(t *testing.T) {
	for _, over := range []bool{false, true} {
		setMinStrikeForce(t, 1)
		f := newAttackFixture(t)
		withFog(f)
		hunter := armedTank(1, 1, 12, 12, f.units)
		hunter.Values.Set(game.AttribScan, 7)
		enemy := lostContact(f, 90, 12, 9)

		f.g.State.TurnCounter = 50
		f.attack.searchMode = map[*game.UnitInfo]int{hunter: searchExplore}
		since := 50 - searchPatience
		if over {
			since--
		}
		f.attack.searchSince = map[*game.UnitInfo]int{hunter: since}

		f.attack.BeginTurn()

		_, shelved := f.attack.abandoned[enemy]
		_, searching := f.attack.searchMode[hunter]
		if shelved != over || searching == over {
			t.Errorf("past patience %v: shelved %v, still searching %v", over, shelved, searching)
		}
	}
}

// TestCommitBuildsTheFocusAndLaunchersLeadOnlyBefore checks the two sides of the commit: a
// committed team picks a shared objective and keeps the ID order; before commit there is no
// objective and a launcher leads the probe.
func TestCommitBuildsTheFocusAndLaunchersLeadOnlyBefore(t *testing.T) {
	for _, committed := range []bool{false, true} {
		setMinStrikeForce(t, 1)
		if !committed {
			setMinStrikeForce(t, 10)
		}
		f := newAttackFixture(t)
		stationary(1, 60, assets.MiningSt, 2, 2, f.units) // a base to mass out from
		armedTank(1, 1, 6, 6, f.units)
		launcher := armedUnit(1, 9, assets.MisslLch, flagLandUnit, 7, 6, f.units)
		durableEnemy(f, 90, 20, 20)

		ready, _ := f.attack.planAssault(f.g)

		if (f.attack.focus != nil) != committed {
			t.Errorf("committed %v: focus %v", committed, f.attack.focus)
		}
		if len(ready) != 2 || (ready[0] == launcher) == committed {
			t.Errorf("committed %v: the launcher leads = %v, want %v", committed, len(ready) == 2 && ready[0] == launcher, !committed)
		}
	}
}

// TestCommitBoundaries checks the commit gates: the grace floor turn itself allows the commit,
// and an enemy spotted on the map edge counts.
func TestCommitBoundaries(t *testing.T) {
	setMinStrikeForce(t, 1)
	prev := aiGraceTurns
	t.Cleanup(func() { aiGraceTurns = prev })
	aiGraceTurns = 10

	for _, c := range []struct {
		turn int
		edge game.Point
		want bool
	}{
		{9, game.Point{X: 20, Y: 20}, false},
		{10, game.Point{X: 20, Y: 20}, true},
		{10, game.Point{X: 0, Y: 20}, true},
		{10, game.Point{X: 20, Y: 0}, true},
	} {
		f := newAttackFixture(t)
		armedTank(1, 1, 6, 6, f.units)
		durableEnemy(f, 90, c.edge.X, c.edge.Y)
		f.g.State.TurnCounter = c.turn

		if got := f.attack.isCommitted(); got != c.want {
			t.Errorf("turn %d, enemy at %v: committed %v, want %v", c.turn, c.edge, got, c.want)
		}
	}
}

// TestSortieBudgetTimeline checks the probe timeline before commit: nothing before the
// harassment start, one unit from that turn, two from the buildup midpoint.
func TestSortieBudgetTimeline(t *testing.T) {
	ph, pg := aiHarassStart, aiGraceTurns
	t.Cleanup(func() { aiHarassStart, aiGraceTurns = ph, pg })
	aiHarassStart, aiGraceTurns = 20, 40 // midpoint 30

	f := newAttackFixture(t)
	for turn, want := range map[int]int{19: 0, 20: 1, 29: 1, 30: 2} {
		if got := f.attack.sortieBudget(turn, false); got != want {
			t.Errorf("turn %d: sortie %d, want %d", turn, got, want)
		}
	}
}

// TestAdvanceTowardHoldsAtRange checks the stand off: a unit exactly at its weapon range holds.
func TestAdvanceTowardHoldsAtRange(t *testing.T) {
	f := newAttackFixture(t)
	tank := armedTank(1, 1, 6, 8, f.units) // range 4

	if f.attack.advanceToward(tank, 10, 8) || f.g.HasOrder(tank) {
		t.Error("a unit exactly at its range was moved")
	}
}

// TestAdvanceTowardPrefersTheSafestFiringCell checks the threat aware stand off: with a threat
// map the unit drives to a firing cell nobody covers, not the nearest one under fire.
func TestAdvanceTowardPrefersTheSafestFiringCell(t *testing.T) {
	f := newAttackFixture(t)
	tank := armedTank(1, 1, 2, 8, f.units)
	cells := f.g.FiringApproachCells(tank, 14, 8, 4)
	if len(cells) < 2 {
		t.Fatal("fixture: too few firing cells")
	}

	tm := &threatMap{w: 24, h: 24, cell: make([][]int, 24)}
	for x := range tm.cell {
		tm.cell[x] = make([]int, 24)
	}
	for _, c := range cells[:len(cells)-1] {
		tm.cell[c.X][c.Y] = 9 // every firing cell but the last is covered
	}
	f.attack.tm = tm
	safe := cells[len(cells)-1]

	f.attack.advanceToward(tank, 14, 8)

	if goal, ok := moveGoal(f.g, tank); !ok || goal.X != safe.X || goal.Y != safe.Y {
		t.Errorf("goal %v (moving %v), want the one firing cell no enemy covers %v", goal, ok, safe)
	}
}

// TestZoneSearchScansTheWholeDisc checks the zone sweep: it finds the one unexplored cell of
// the disc around the last seen cell; the explore mode leaves the unit idle and stays live.
func TestZoneSearchScansTheWholeDisc(t *testing.T) {
	f := newAttackFixture(t)
	withFog(f)
	hunter := armedTank(1, 1, 2, 2, f.units)
	open := game.Point{X: 15, Y: 13}
	exploreAllBut(f.g, open)

	if !f.attack.runSearch(hunter, 12, 12, searchExplore) || f.g.HasOrder(hunter) {
		t.Error("the explore mode moved the unit or ended the search")
	}

	if !f.attack.runSearch(hunter, 12, 12, searchZone) {
		t.Fatal("the zone search found nothing in the disc")
	}
	if goal, ok := moveGoal(f.g, hunter); !ok || goal != open {
		t.Errorf("zone search goal %v (moving %v), want the one unexplored cell %v", goal, ok, open)
	}
}
