// task_defensereserve_test.go : testing unit for task_defensereserve.go
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

// newReserveFixture builds the explore fixture's map on a reserve keeping tier with a defence
// reserve task for team 1.
func newReserveFixture(t *testing.T) (*exploreFixture, *taskDefenseReserve) {
	t.Helper()

	setDifficulty(t, OpponentMaster) // keepReserve = true
	f := newExploreFixture(t)

	return f, newTaskDefenseReserve(1, &aiContext{Game: f.g, MakeUnit: testUnitFactory})
}

// spotted deploys a spotted enemy tank of team 0 at (x, y).
func spotted(f *exploreFixture, id, x, y int) *game.UnitInfo {
	e := armedTank(0, id, x, y, f.units)
	f.g.AddSpotted(1, e, e.GridX, e.GridY, true)

	return e
}

// TestGuardSitePicksTheNearestKeyFacility checks the post: the team's nearest key facility,
// past another team's facility and a building that is not key; the base centre without one.
func TestGuardSitePicksTheNearestKeyFacility(t *testing.T) {
	f, reserve := newReserveFixture(t)
	guard := armedTank(1, 1, 12, 12, f.units)
	stationary(2, 50, assets.MiningSt, 12, 13, f.units) // another team's, nearest
	stationary(1, 51, assets.Habitat, 13, 12, f.units)  // own, but not a key facility
	stationary(1, 52, assets.MiningSt, 2, 2, f.units)   // own, far
	stationary(1, 53, assets.LightPlt, 18, 12, f.units) // own, near

	if x, y := reserve.guardSite(guard); x != 18 || y != 12 {
		t.Errorf("guard site (%d,%d), want the near Light Vehicle Plant (18,12)", x, y)
	}

	g, plain := newReserveFixture(t)
	lone := armedTank(1, 1, 12, 12, g.units)
	stationary(1, 50, assets.Habitat, 4, 6, g.units)
	bx, by, _ := teamBaseCenter(g.g, 1)
	if x, y := plain.guardSite(lone); x != bx || y != by {
		t.Errorf("guard site (%d,%d) with no key facility, want the base centre (%d,%d)", x, y, bx, by)
	}
}

// TestThreatNearHonoursTheRadius checks the engage ring: an enemy exactly at the radius
// counts, one past it does not, the nearer of two wins, and an own or unspotted unit or one
// off the map never counts.
func TestThreatNearHonoursTheRadius(t *testing.T) {
	f, reserve := newReserveFixture(t)
	edge := spotted(f, 90, 12, 2) // 10 cells: on the ring
	if got := reserve.threatNear(12, 12); got != edge {
		t.Fatalf("threat = %v, want the enemy on the ring", got)
	}

	g, far := newReserveFixture(t)
	spotted(g, 90, 12, 1) // 11 cells: past the ring
	if got := far.threatNear(12, 12); got != nil {
		t.Errorf("threat = %v, want none past the ring", got)
	}

	h, pick := newReserveFixture(t)
	armedTank(1, 1, 12, 13, h.units)  // own unit, nearest
	armedTank(0, 91, 13, 12, h.units) // not spotted
	offMap := spotted(h, 92, 12, 11)  // loaded: off the map
	offMap.GridX, offMap.GridY = -1, -1
	spotted(h, 93, 12, 6)
	near := spotted(h, 94, 12, 9)
	if got := pick.threatNear(12, 12); got != near {
		t.Errorf("threat = %v, want the nearest spotted enemy", got)
	}
}

// TestReserveGuardTakesItsPost checks the moves: a guard far from its post drives to the first
// free cell beside it, one within the post radius holds, and one with no movement left gets
// no order.
func TestReserveGuardTakesItsPost(t *testing.T) {
	f, reserve := newReserveFixture(t)
	stationary(1, 50, assets.MiningSt, 4, 4, f.units)
	far := armedTank(1, 1, 16, 16, f.units)
	first := f.g.AdjacentApproachCells(far, 4, 4)[0]
	onPost := armedTank(1, 2, 7, 4, f.units) // 3 cells: on the radius

	reserve.holdPost(far, 4, 4)
	reserve.holdPost(onPost, 4, 4)

	if goal, ok := moveGoal(f.g, far); !ok || goal.X != first.X || goal.Y != first.Y {
		t.Errorf("post goal %v (moving %v), want the first free cell beside the post %v", goal, ok, first)
	}
	if f.g.HasOrder(onPost) {
		t.Error("a guard on the post radius was moved")
	}

	g, reserve2 := newReserveFixture(t)
	stationary(1, 50, assets.MiningSt, 4, 4, g.units)
	for i := 0; i < 7; i++ {
		armedTank(1, 10+i, 20, 2+2*i, g.units)
	}
	spent := armedTank(1, 1, 8, 8, g.units) // the closest: a guard
	spent.Speed = 0
	reserve2.BeginTurn()
	if g.g.HasOrder(spent) {
		t.Error("a guard with no movement left got an order")
	}
}

// TestReserveInterceptsBesideTheEnemy checks the intercept move: an enemy pressing the post but
// out of the guard's range is met at the first free cell beside it.
func TestReserveInterceptsBesideTheEnemy(t *testing.T) {
	f, reserve := newReserveFixture(t)
	guard := armedTank(1, 1, 4, 4, f.units)
	enemy := spotted(f, 90, 12, 4)
	first := f.g.AdjacentApproachCells(guard, 12, 4)[0]

	reserve.intercept(guard, enemy)

	if goal, ok := moveGoal(f.g, guard); !ok || goal.X != first.X || goal.Y != first.Y {
		t.Errorf("intercept goal %v (moving %v), want the first free cell beside the enemy %v", goal, ok, first)
	}
}

// TestReservedGuardsRankByDistanceThenID checks the reserve pick: the units closest to the
// base, whatever their ID, and on a tie the lower ID.
func TestReservedGuardsRankByDistanceThenID(t *testing.T) {
	f, _ := newReserveFixture(t)
	stationary(1, 50, assets.MiningSt, 4, 4, f.units)
	for i := 0; i < 6; i++ {
		armedTank(1, 1+i, 20, 2+3*i, f.units) // far, low IDs
	}
	closest := armedTank(1, 30, 6, 4, f.units) // highest ID, closest
	tieLow := armedTank(1, 20, 4, 9, f.units)  // 5 cells, lower ID
	tieHigh := armedTank(1, 25, 9, 4, f.units) // 5 cells, higher ID

	got := reservedGuards(f.g, 1) // 9 armed: a quarter is 2
	if len(got) != 2 || !got[closest] || !got[tieLow] || got[tieHigh] {
		t.Errorf("reserve = %d units (closest %v, tie low %v, tie high %v), want the closest and the lower ID of the tie", len(got), got[closest], got[tieLow], got[tieHigh])
	}
}

// TestArmedMobileUnitSkipsBuildings checks the military count: an armed turret is not part
// of the mobile force.
func TestArmedMobileUnitSkipsBuildings(t *testing.T) {
	f, _ := newReserveFixture(t)
	turret := armedUnit(1, 1, assets.GunTurrt, game.FlagStationary|game.FlagBuilding, 6, 6, f.units)

	if armedMobileUnit(turret, 1) {
		t.Error("an armed turret counts as an armed mobile unit")
	}
}
