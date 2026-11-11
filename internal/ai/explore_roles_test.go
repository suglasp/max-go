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
