// explore_roles_test.go : testing unit for the explore roles in task_explore.go (recon units only, the caps, a scout under fire)
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// exploreFixture is a 24x24 land map with a player team 0 and a computer team 1, and an
// explore task for team 1 bound to a task manager.
type exploreFixture struct {
	g       *sim.Game
	units   *units.Manager
	tasks   *TaskManager
	explore *taskExplore
}

// newExploreFixture builds the fixture; the explore task is registered but has not run.
func newExploreFixture(t *testing.T) *exploreFixture {
	t.Helper()

	const w, h = 24, 24
	unitsMgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: unitsMgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), unitsMgr, grid, w, h)

	f := &exploreFixture{g: g, units: unitsMgr, tasks: NewTaskManager()}
	f.explore = newTaskExplore(1, &aiContext{Game: g, MakeUnit: testUnitFactory})

	return f
}

// run registers the explore task, which makes its first pass.
func (f *exploreFixture) run() {
	f.tasks.AppendTask(f.explore)
	f.tasks.ExecuteReminders()
}

// reconUnit deploys an armed unit of the type with scan range, ready to scout.
func reconUnit(f *exploreFixture, id int, ut assets.UnitType, flags game.Flags, gx, gy int) *game.UnitInfo {
	u := armedUnit(1, id, ut, flags, gx, gy, f.units)
	u.Values.Set(game.AttribScan, 3)

	return u
}

// TestExploreUsesOnlyReconUnits checks the roles: combat units with scan range never
// explore, the recon types do.
func TestExploreUsesOnlyReconUnits(t *testing.T) {
	f := newExploreFixture(t)
	cases := []struct {
		ut    assets.UnitType
		flags game.Flags
		want  bool
	}{
		{assets.Tank, flagLandUnit, false},
		{assets.Artillry, flagLandUnit, false},
		{assets.Corvette, flagSeaUnit, false},
		{assets.Bomber, flagAirUnit, false},
		{assets.Scout, flagLandUnit, true},
		{assets.FastBoat, flagSeaUnit, true},
		{assets.Awac, flagAirUnit, true},
		{assets.Scanner, flagLandUnit, true},
	}
	for i, c := range cases {
		u := reconUnit(f, 10+i, c.ut, c.flags, 2+2*i, 4)
		if got := f.explore.isReadyExplorer(u); got != c.want {
			t.Errorf("%s: isReadyExplorer = %v, want %v", c.ut.Tag(), got, c.want)
		}
	}
}

// TestExploreCapsReconUnitsPerType checks the caps: of four Scouts three explore and the
// fourth stays free; of two AWACs one explores.
func TestExploreCapsReconUnitsPerType(t *testing.T) {
	f := newExploreFixture(t)
	var scouts, awacs []*game.UnitInfo
	for i := 0; i < 4; i++ {
		scouts = append(scouts, reconUnit(f, 10+i, assets.Scout, flagLandUnit, 10+i, 12))
	}
	for i := 0; i < 2; i++ {
		awacs = append(awacs, reconUnit(f, 20+i, assets.Awac, flagAirUnit, 10+i, 14))
	}

	f.run()

	count := func(list []*game.UnitInfo) (driven, held int) {
		for _, u := range list {
			if f.g.HasOrder(u) {
				driven++
			}
			if f.tasks.owner[u] == Task(f.explore) {
				held++
			}
		}
		return driven, held
	}
	if driven, held := count(scouts); driven != 3 || held != 3 {
		t.Errorf("scouts driven %d held %d, want 3 and 3 (the cap)", driven, held)
	}
	if driven, held := count(awacs); driven != 1 || held != 1 {
		t.Errorf("AWACs driven %d held %d, want 1 and 1 (the cap)", driven, held)
	}
}

// underFire spots an enemy tank of team 0 at (x, y) for team 1, so the threat map covers the
// cells around it; reach sets its weapon range.
func underFire(f *exploreFixture, x, y, reach int) *game.UnitInfo {
	enemy := armedTank(0, 99, x, y, f.units)
	enemy.Values.Set(game.AttribRange, reach)
	f.g.AddSpotted(1, enemy, enemy.GridX, enemy.GridY, true)

	return enemy
}

// TestScoutUnderFireFightsBack checks the defence: a scout in enemy fire with the enemy in
// its own range fires instead of exploring, and the task holds it.
func TestScoutUnderFireFightsBack(t *testing.T) {
	f := newExploreFixture(t)
	stationary(1, 50, assets.MiningSt, 2, 2, f.units)
	scout := reconUnit(f, 10, assets.Scout, flagLandUnit, 12, 12)
	underFire(f, 13, 14, 4)
	startShots := scout.Shots

	f.run()

	if scout.Shots >= startShots {
		t.Errorf("scout shots = %d (was %d): it did not fight back", scout.Shots, startShots)
	}
	if f.tasks.owner[scout] != Task(f.explore) {
		t.Error("the explore task does not hold the scout that fought back")
	}
}

// TestScoutUnderFireFallsBackWhenItCannotFire checks the retreat: a scout in enemy fire
// that cannot reach the enemy does not explore on; it falls back to the base, and with no
// base it holds.
func TestScoutUnderFireFallsBackWhenItCannotFire(t *testing.T) {
	for _, withBase := range []bool{true, false} {
		f := newExploreFixture(t)
		if withBase {
			stationary(1, 50, assets.MiningSt, 2, 2, f.units)
		}
		scout := reconUnit(f, 10, assets.Scout, flagLandUnit, 12, 12)
		scout.Values.Set(game.AttribRange, 1)
		underFire(f, 16, 12, 8) // out of the scout's reach, the scout inside the enemy's
		startShots := scout.Shots

		f.run()

		if scout.Shots != startShots {
			t.Errorf("base=%v: the scout fired at an enemy out of its range", withBase)
		}
		if withBase != f.g.HasOrder(scout) {
			t.Errorf("base=%v: has order = %v; want a fall back with a base and a hold without", withBase, f.g.HasOrder(scout))
		}
	}
}

// TestScoutLowOnHealthFallsBack checks the health rule: a scout in enemy fire that could
// fire back but is low on health falls back instead.
func TestScoutLowOnHealthFallsBack(t *testing.T) {
	f := newExploreFixture(t)
	stationary(1, 50, assets.MiningSt, 2, 2, f.units)
	scout := reconUnit(f, 10, assets.Scout, flagLandUnit, 12, 12)
	scout.Hits = 2 // of 24: below the retreat threshold
	underFire(f, 13, 14, 4)
	startShots := scout.Shots

	f.run()

	if scout.Shots != startShots {
		t.Error("a scout low on health fought back instead of falling back")
	}
	if !f.g.HasOrder(scout) {
		t.Error("a scout low on health did not fall back")
	}
}

// TestLowHealthCoversUnarmedUnits checks lowHealth against unitDamaged: both read the retreat
// threshold, but only lowHealth also covers an unarmed unit.
func TestLowHealthCoversUnarmedUnits(t *testing.T) {
	f := newExploreFixture(t)
	awac := reconUnit(f, 10, assets.Awac, flagAirUnit, 5, 5)
	awac.Values.Set(game.AttribAttack, 0)
	awac.Hits = 11 // of 24

	if !lowHealth(awac) {
		t.Error("an unarmed unit below the threshold is not low on health")
	}
	if unitDamaged(awac) {
		t.Error("unitDamaged now counts an unarmed unit; the retreat task must not take it")
	}

	awac.Hits = 12 // exactly half: not below
	if lowHealth(awac) {
		t.Error("a unit at exactly the threshold counts as low")
	}
}

// moveGoal returns the last cell of u's queued move, if it has one.
func moveGoal(g *sim.Game, u *game.UnitInfo) (game.Point, bool) {
	for _, o := range g.MoveOrders() {
		if o.Unit == u && len(o.Steps) > 0 {
			return o.Steps[len(o.Steps)-1].To, true
		}
	}

	return game.Point{}, false
}

// exploreAllBut marks every cell explored for team 1 except the given ones.
func exploreAllBut(g *sim.Game, cells ...game.Point) {
	n := g.W * g.H
	bits := make([]byte, (n+7)/8)
	for i := range bits {
		bits[i] = 0xFF
	}
	open := map[int]bool{}
	for _, c := range cells {
		open[c.Y*g.W+c.X] = true
	}
	for i := 0; i < n; i++ {
		if open[i] {
			bits[i/8] &^= 1 << uint(i%8)
		}
	}
	teams := make([][]byte, game.TeamCount)
	teams[1] = bits
	g.Vis.ImportExplored(teams)
}

// TestExploreHeadsForTheNearestFog checks the ring search: with one near and one far fog
// cell, the scout drives to the near one, wherever it lies on the ring and at the map edges.
func TestExploreHeadsForTheNearestFog(t *testing.T) {
	cases := []struct {
		name      string
		scout     game.Point
		near, far game.Point
	}{
		{"left", game.Point{X: 12, Y: 12}, game.Point{X: 9, Y: 12}, game.Point{X: 22, Y: 22}},
		{"right", game.Point{X: 12, Y: 12}, game.Point{X: 15, Y: 12}, game.Point{X: 1, Y: 1}},
		{"above", game.Point{X: 12, Y: 12}, game.Point{X: 12, Y: 9}, game.Point{X: 22, Y: 22}},
		{"below", game.Point{X: 12, Y: 12}, game.Point{X: 12, Y: 15}, game.Point{X: 1, Y: 1}},
		{"a corner", game.Point{X: 12, Y: 12}, game.Point{X: 9, Y: 9}, game.Point{X: 22, Y: 22}},
		{"the top left edge", game.Point{X: 3, Y: 3}, game.Point{X: 0, Y: 0}, game.Point{X: 22, Y: 22}},
		{"the bottom right edge", game.Point{X: 20, Y: 20}, game.Point{X: 23, Y: 23}, game.Point{X: 1, Y: 1}},
	}
	for _, c := range cases {
		f := newExploreFixture(t)
		scout := reconUnit(f, 10, assets.Scout, flagLandUnit, c.scout.X, c.scout.Y)
		scout.Values.Set(game.AttribScan, 1)
		exploreAllBut(f.g, c.near, c.far)

		f.run()

		if goal, ok := moveGoal(f.g, scout); !ok || goal != c.near {
			t.Errorf("%s: goal %v (moving %v), want the near fog %v", c.name, goal, ok, c.near)
		}
	}
}

// TestExploreReachesFogOnATallMap checks the ring bound: on a map taller than it is wide the
// search still reaches a fog cell far down the long side.
func TestExploreReachesFogOnATallMap(t *testing.T) {
	const w, h = 4, 30
	unitsMgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: unitsMgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), unitsMgr, grid, w, h)
	f := &exploreFixture{g: g, units: unitsMgr, tasks: NewTaskManager()}
	f.explore = newTaskExplore(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	scout := reconUnit(f, 10, assets.Scout, flagLandUnit, 1, 1)
	scout.Values.Set(game.AttribScan, 1)
	far := game.Point{X: 1, Y: 27}
	exploreAllBut(g, far)

	f.run()

	if goal, ok := moveGoal(g, scout); !ok || goal != far {
		t.Errorf("goal %v (moving %v), want the fog far down the map %v", goal, ok, far)
	}
}

// TestExploreHoldsWithoutAFogView checks the guard: with no fog view the task does nothing.
func TestExploreHoldsWithoutAFogView(t *testing.T) {
	f := newExploreFixture(t)
	scout := reconUnit(f, 10, assets.Scout, flagLandUnit, 12, 12)
	f.g.Vis = nil

	f.run()

	if f.g.HasOrder(scout) {
		t.Error("the explore task moved a scout with no fog view")
	}
}

// TestExploreReadyRules checks the remaining gates: another team's scout, a scout with no
// scan and a scout with no movement left are not explorers.
func TestExploreReadyRules(t *testing.T) {
	f := newExploreFixture(t)
	theirs := reconUnit(f, 10, assets.Scout, flagLandUnit, 4, 4)
	theirs.Team = 2
	blind := reconUnit(f, 11, assets.Scout, flagLandUnit, 6, 4)
	blind.Values.Set(game.AttribScan, 0)
	spent := reconUnit(f, 12, assets.Scout, flagLandUnit, 8, 4)
	spent.Speed = 0
	ready := reconUnit(f, 13, assets.Scout, flagLandUnit, 10, 4)

	for name, u := range map[string]*game.UnitInfo{"another team's scout": theirs, "a scout with no scan": blind, "a scout with no movement left": spent} {
		if f.explore.isReadyExplorer(u) {
			t.Errorf("%s counts as an explorer", name)
		}
	}
	if !f.explore.isReadyExplorer(ready) {
		t.Error("a ready scout does not count as an explorer")
	}
}

// TestScoutFallsBackBesideTheBase checks the fall back target and hold: with a building on the
// base centre the scout drives to the first free cell beside it, and the task holds it.
func TestScoutFallsBackBesideTheBase(t *testing.T) {
	f := newExploreFixture(t)
	stationary(1, 50, assets.MiningSt, 2, 2, f.units)
	scout := reconUnit(f, 10, assets.Scout, flagLandUnit, 12, 12)
	scout.Values.Set(game.AttribRange, 1)
	underFire(f, 16, 12, 8)
	bx, by, _ := teamBaseCenter(f.g, 1)
	first := f.g.AdjacentApproachCells(scout, bx, by)[0]

	f.run()

	if goal, ok := moveGoal(f.g, scout); !ok || goal.X != first.X || goal.Y != first.Y {
		t.Errorf("fall back goal %v (moving %v), want the first free cell beside the base %v", goal, ok, first)
	}
	if f.tasks.owner[scout] != Task(f.explore) {
		t.Error("the explore task does not hold the scout that fell back")
	}
}
