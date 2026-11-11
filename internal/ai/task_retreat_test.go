// task_retreat_test.go : testing unit for task_retreat.go
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

// newRetreatFixture builds the explore fixture's map with a retreat task for team 1.
func newRetreatFixture(t *testing.T) (*exploreFixture, *taskRetreat) {
	t.Helper()

	f := newExploreFixture(t)

	return f, newTaskRetreat(1, &aiContext{Game: f.g, MakeUnit: testUnitFactory})
}

// damagedTank deploys a team 1 tank below the retreat threshold.
func damagedTank(f *exploreFixture, id, gx, gy int) *game.UnitInfo {
	u := armedTank(1, id, gx, gy, f.units)
	u.Hits = 2

	return u
}

// flatThreat builds a threat map of the fixture's size with the given cell values.
func flatThreat(f *exploreFixture, cells map[game.Point]int) *threatMap {
	m := &threatMap{w: f.g.W, h: f.g.H, cell: make([][]int, f.g.W)}
	for x := range m.cell {
		m.cell[x] = make([]int, f.g.H)
	}
	for c, v := range cells {
		m.cell[c.X][c.Y] = v
	}

	return m
}

// TestRepairShopTypeByClass checks the shop each class goes to: infantry to the Barracks,
// land units to the Depot, air to the Hangar, sea to the Dock, and none for a building.
func TestRepairShopTypeByClass(t *testing.T) {
	cases := []struct {
		ut    assets.UnitType
		flags game.Flags
		shop  assets.UnitType
		ok    bool
	}{
		{assets.Infantry, flagLandUnit, assets.Barracks, true},
		{assets.Commando, flagLandUnit, assets.Barracks, true},
		{assets.Tank, flagLandUnit, assets.Depot, true},
		{assets.Fighter, flagAirUnit, assets.Hangar, true},
		{assets.Corvette, flagSeaUnit, assets.Dock, true},
		{assets.MiningSt, game.FlagStationary, 0, false},
	}
	for _, c := range cases {
		u := game.NewUnitInfo(c.ut, 1, 1, 0)
		u.Flags = c.flags
		shop, ok := repairShopType(u)
		if shop != c.shop || ok != c.ok {
			t.Errorf("%s: repairShopType = %s, %v; want %s, %v", c.ut.Tag(), shop.Tag(), ok, c.shop.Tag(), c.ok)
		}
	}
}

// TestNearestRepairShopPicksTheTeamsNearestOfTheClass checks the shop search: another
// team's shop and a shop of another class are passed over, and of two the nearer wins.
func TestNearestRepairShopPicksTheTeamsNearestOfTheClass(t *testing.T) {
	f, retreat := newRetreatFixture(t)
	tank := damagedTank(f, 1, 12, 12)
	theirs := stationary(2, 50, assets.Depot, 12, 13, f.units)
	hangar := stationary(1, 51, assets.Hangar, 13, 12, f.units)
	far := stationary(1, 52, assets.Depot, 2, 2, f.units)
	near := stationary(1, 53, assets.Depot, 18, 12, f.units)

	got := retreat.nearestRepairShop(tank)
	if got != near {
		t.Errorf("nearest shop = %v, want the team's nearer Depot (not %v, %v or %v)", got, theirs, hangar, far)
	}
}

// TestRetreatDrivesBesideTheShop checks the drive: a damaged tank heads for the first free
// cell beside its shop, one already beside the shop holds, and one with no movement left
// gets no order.
func TestRetreatDrivesBesideTheShop(t *testing.T) {
	f, retreat := newRetreatFixture(t)
	shop := stationary(1, 50, assets.Depot, 4, 4, f.units)
	tank := damagedTank(f, 1, 14, 14)
	first := f.g.AdjacentApproachCells(tank, shop.GridX, shop.GridY)[0]
	beside := damagedTank(f, 2, 5, 4)
	spent := damagedTank(f, 3, 18, 18)
	spent.Speed = 0

	retreat.BeginTurn()

	if goal, ok := moveGoal(f.g, tank); !ok || goal.X != first.X || goal.Y != first.Y {
		t.Errorf("retreat goal %v (moving %v), want the first free cell beside the shop %v", goal, ok, first)
	}
	if f.g.HasOrder(beside) {
		t.Error("a tank already beside the shop was ordered to move")
	}
	if f.g.HasOrder(spent) {
		t.Error("a tank with no movement left got an order")
	}
}

// TestFleeToSafetyPicksTheSafestCell checks the flee choice: the lowest threat cell in reach
// wins, a tie goes to the cell nearest home, a cell the unit cannot enter is passed over, and
// a unit already on the safest cell holds.
func TestFleeToSafetyPicksTheSafestCell(t *testing.T) {
	f, retreat := newRetreatFixture(t)
	stationary(1, 50, assets.MiningSt, 2, 12, f.units) // home lies west
	tank := damagedTank(f, 1, 12, 12)

	// Every cell in reach is under fire except two safe ones, one each side of the tank.
	threat := map[game.Point]int{}
	for x := 0; x < f.g.W; x++ {
		for y := 0; y < f.g.H; y++ {
			threat[game.Point{X: x, Y: y}] = 5
		}
	}
	west, east := game.Point{X: 10, Y: 12}, game.Point{X: 14, Y: 12}
	threat[west], threat[east] = 0, 0
	tm := flatThreat(f, threat)

	x, y, ok := retreat.fleeToSafety(tank, tm)
	if !ok || x != west.X || y != west.Y {
		t.Errorf("flee to (%d,%d) %v, want the safe cell nearer home %v", x, y, ok, west)
	}

	// The safe west cell is taken: the east one is the only safe cell left.
	armedTank(1, 2, west.X, west.Y, f.units)
	x, y, ok = retreat.fleeToSafety(tank, tm)
	if !ok || x != east.X || y != east.Y {
		t.Errorf("flee to (%d,%d) %v, want the free safe cell %v", x, y, ok, east)
	}

	// On the only safe cell already: hold.
	threat[game.Point{X: 12, Y: 12}] = 0
	threat[east] = 5
	tm = flatThreat(f, threat)
	if _, _, ok := retreat.fleeToSafety(tank, tm); ok {
		t.Error("a unit already on the safest cell was sent elsewhere")
	}
}

// TestFleeToSafetyBreaksTiesTowardHome checks the tie break from both sides, and a safe cell
// in the unit's own column: of two equally safe cells the one nearer home wins.
func TestFleeToSafetyBreaksTiesTowardHome(t *testing.T) {
	cases := []struct {
		name string
		home game.Point
		want game.Point
	}{
		{"home to the north", game.Point{X: 12, Y: 2}, game.Point{X: 12, Y: 10}},
		{"home to the south", game.Point{X: 12, Y: 22}, game.Point{X: 12, Y: 14}},
	}
	for _, c := range cases {
		f, retreat := newRetreatFixture(t)
		stationary(1, 50, assets.MiningSt, c.home.X, c.home.Y, f.units).Flags |= game.FlagBuilding
		tank := damagedTank(f, 1, 12, 12)

		threat := map[game.Point]int{}
		for x := 0; x < f.g.W; x++ {
			for y := 0; y < f.g.H; y++ {
				threat[game.Point{X: x, Y: y}] = 5
			}
		}
		threat[game.Point{X: 12, Y: 10}], threat[game.Point{X: 12, Y: 14}] = 0, 0

		x, y, ok := retreat.fleeToSafety(tank, flatThreat(f, threat))
		if !ok || x != c.want.X || y != c.want.Y {
			t.Errorf("%s: flee to (%d,%d) %v, want %v", c.name, x, y, ok, c.want)
		}
	}
}

// TestNearestFriendlyBuildingIgnoresUnitsAndOtherTeams checks the home search: another
// team's building and a mobile unit are passed over, and the nearer own building wins.
func TestNearestFriendlyBuildingIgnoresUnitsAndOtherTeams(t *testing.T) {
	f, retreat := newRetreatFixture(t)
	tank := damagedTank(f, 1, 12, 12)
	stationary(2, 50, assets.MiningSt, 12, 13, f.units).Flags |= game.FlagBuilding
	armedTank(1, 2, 13, 12, f.units)
	far := stationary(1, 51, assets.MiningSt, 1, 1, f.units)
	far.Flags |= game.FlagBuilding
	near := stationary(1, 52, assets.MiningSt, 18, 12, f.units)
	near.Flags |= game.FlagBuilding

	if got := retreat.nearestFriendlyBuilding(tank); got != near {
		t.Errorf("home = %v, want the team's nearer building", got)
	}
}

// TestUnitDamagedOnlyCountsMobileArmedUnits checks the retreat gate: a damaged armed turret
// (a building) is not sent to retreat.
func TestUnitDamagedOnlyCountsMobileArmedUnits(t *testing.T) {
	f, _ := newRetreatFixture(t)
	turret := armedUnit(1, 1, assets.GunTurrt, game.FlagStationary|game.FlagBuilding, 6, 6, f.units)
	turret.Hits = 2

	if unitDamaged(turret) {
		t.Error("a damaged turret counts as a unit to retreat")
	}
}
