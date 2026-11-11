// task_escort_test.go : testing unit for task_escort.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// newEscortFixture builds the explore fixture's map on an escort tier with an escort task
// for team 1.
func newEscortFixture(t *testing.T) (*exploreFixture, *taskEscort) {
	t.Helper()

	setDifficulty(t, OpponentExpert) // escort on, muster 3
	f := newExploreFixture(t)

	return f, newTaskEscort(1, &aiContext{Game: f.g, MakeUnit: testUnitFactory})
}

// TestEngageableEnemyFilters checks the shared fire search: an own unit, an unspotted enemy
// and one off the map are passed over, and of two enemies in range the nearer wins.
func TestEngageableEnemyFilters(t *testing.T) {
	f, _ := newEscortFixture(t)
	guard := armedTank(1, 1, 12, 12, f.units)
	armedTank(1, 2, 12, 13, f.units)  // own
	armedTank(0, 90, 13, 12, f.units) // not spotted
	offMap := spotted(f, 91, 11, 12)
	offMap.GridX, offMap.GridY = -1, -1
	spotted(f, 92, 12, 15)
	near := spotted(f, 93, 12, 14)

	if got := engageableEnemy(f.g, 1, guard); got != near {
		t.Errorf("engageable = %v, want the nearer spotted enemy in range", got)
	}
}

// TestEscortFollowHoldsOnStation checks the follow: a guard within the hold radius holds, one
// further off drives to the first free cell beside its charge.
func TestEscortFollowHoldsOnStation(t *testing.T) {
	f, escort := newEscortFixture(t)
	charge := scannerUnit(1, 1, 12, 12, f.units)
	onStation := armedTank(1, 2, 14, 12, f.units) // 2 cells: the radius
	far := armedTank(1, 3, 20, 20, f.units)
	first := f.g.AdjacentApproachCells(far, 12, 12)[0]

	escort.follow(onStation, charge)
	escort.follow(far, charge)

	if f.g.HasOrder(onStation) {
		t.Error("a guard on the hold radius was moved")
	}
	if goal, ok := moveGoal(f.g, far); !ok || goal.X != first.X || goal.Y != first.Y {
		t.Errorf("follow goal %v (moving %v), want the first free cell beside the charge %v", goal, ok, first)
	}
}

// TestEscortGuardWithNoMovementLeftWaits checks the pass: a guard with no movement left gets no
// order even when it is far from its charge.
func TestEscortGuardWithNoMovementLeftWaits(t *testing.T) {
	f, escort := newEscortFixture(t)
	scannerUnit(1, 1, 12, 12, f.units)
	guard := armedTank(1, 2, 20, 20, f.units)
	for i := 0; i < 3; i++ {
		armedTank(1, 10+i, 2, 2+2*i, f.units)
	}
	if a := escortAssignments(f.g, 1); a[guard] == nil {
		t.Fatal("fixture: the far tank is not the escort")
	}
	guard.Speed = 0

	escort.BeginTurn()

	if f.g.HasOrder(guard) {
		t.Error("an escort with no movement left got an order")
	}
}

// TestEscortAssignmentsCapAndDistinctGuards checks the pairing: the spare force caps the
// escorts, and two charges never share one guard.
func TestEscortAssignmentsCapAndDistinctGuards(t *testing.T) {
	f, _ := newEscortFixture(t)
	scannerUnit(1, 1, 12, 12, f.units)
	scannerUnit(1, 2, 12, 14, f.units)
	armedTank(1, 3, 12, 13, f.units) // nearest to both
	for i := 0; i < 3; i++ {
		armedTank(1, 10+i, 2, 2+2*i, f.units)
	}

	if a := escortAssignments(f.g, 1); len(a) != 1 {
		t.Errorf("escorts with one spare guard = %d, want 1", len(a))
	}

	armedTank(1, 4, 20, 13, f.units) // a second spare
	a := escortAssignments(f.g, 1)
	if len(a) != 2 {
		t.Fatalf("escorts with two spare guards = %d, want 2", len(a))
	}
	charges := map[*game.UnitInfo]bool{}
	for _, charge := range a {
		charges[charge] = true
	}
	if len(charges) != 2 {
		t.Error("two guards escort the same charge; each charge wants its own")
	}
}

// TestEscortPrefersTheNearestGuardOfTheBestRank checks the guard choice: a direct fire guard
// beats a nearer artillery unit, and of two direct fire guards the nearer wins.
func TestEscortPrefersTheNearestGuardOfTheBestRank(t *testing.T) {
	f, _ := newEscortFixture(t)
	scannerUnit(1, 1, 12, 12, f.units)
	armedUnit(1, 2, assets.Artillry, flagLandUnit, 12, 13, f.units) // nearest, but artillery
	farTank := armedTank(1, 3, 12, 19, f.units)
	nearTank := armedTank(1, 4, 12, 16, f.units)
	for i := 0; i < 3; i++ {
		armedTank(1, 10+i, 2, 2+2*i, f.units)
	}

	a := escortAssignments(f.g, 1)
	if a[nearTank] == nil || a[farTank] != nil {
		t.Errorf("escort pick: near tank %v, far tank %v; want the nearer direct fire tank", a[nearTank] != nil, a[farTank] != nil)
	}
}

// TestEscortableTargetSkipsOtherTeams checks the charge filter: another team's Scanner gets
// no escort from this team.
func TestEscortableTargetSkipsOtherTeams(t *testing.T) {
	f, _ := newEscortFixture(t)
	theirs := scannerUnit(2, 1, 12, 12, f.units)

	if escortableTarget(theirs, 1) {
		t.Error("another team's Scanner is escortable")
	}
}

// TestEnemySearchesSeeTheMapEdge checks the off map test: an enemy in column 0 or row 0 is on
// the map, both for the escort's fire search and the reserve's threat ring.
func TestEnemySearchesSeeTheMapEdge(t *testing.T) {
	for _, edge := range []game.Point{{X: 0, Y: 3}, {X: 3, Y: 0}} {
		f, _ := newEscortFixture(t)
		guard := armedTank(1, 1, 2, 2, f.units)
		enemy := spotted(f, 90, edge.X, edge.Y)
		if got := engageableEnemy(f.g, 1, guard); got != enemy {
			t.Errorf("edge %v: engageable = %v, want the enemy on the map edge", edge, got)
		}

		g, reserve := newReserveFixture(t)
		onEdge := spotted(g, 90, edge.X, edge.Y)
		if got := reserve.threatNear(3, 3); got != onEdge {
			t.Errorf("edge %v: threat = %v, want the enemy on the map edge", edge, got)
		}
	}
}
