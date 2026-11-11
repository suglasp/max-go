// task_attack_target_test.go : testing unit for task_attack_target.go
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
	"github.com/suglasp/max-go/internal/vis"
)

// attackFixture is a 24x24 land map: team 1 computer (the attacker), team 0 player, team 2
// computer, team 3 player; an attack task for team 1. Vis is nil (the bare harness: every
// enemy is spotted and visible) unless a test turns fog on.
type attackFixture struct {
	g      *sim.Game
	units  *units.Manager
	attack *taskAttackEnemies
}

// newAttackFixture builds the fixture.
func newAttackFixture(t *testing.T) *attackFixture {
	t.Helper()

	const w, h = 24, 24
	unitsMgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: unitsMgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	p.Teams[2].Type = game.TeamComputer
	p.Teams[3].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), unitsMgr, grid, w, h)
	g.Vis = nil

	return &attackFixture{g: g, units: unitsMgr, attack: newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})}
}

// unarmedAt deploys an unarmed unit of the team at (x, y): a target that never threatens.
func unarmedAt(f *attackFixture, team, id, x, y int) *game.UnitInfo {
	u := armedUnit(team, id, assets.SplyTrck, flagLandUnit, x, y, f.units)
	u.Values.Set(game.AttribAttack, 0)

	return u
}

// TestNearestEnemySeesTheMapEdge checks the off map test: an enemy in column 0 or row 0 is a
// target.
func TestNearestEnemySeesTheMapEdge(t *testing.T) {
	for _, edge := range []game.Point{{X: 0, Y: 3}, {X: 3, Y: 0}} {
		f := newAttackFixture(t)
		hunter := armedTank(1, 1, 3, 3, f.units)
		enemy := unarmedAt(f, 0, 90, edge.X, edge.Y)

		if got := f.attack.nearestEnemy(hunter); got != enemy {
			t.Errorf("edge %v: target %v, want the enemy on the map edge", edge, got)
		}
	}
}

// TestNearestEnemyPrefersTheCloserOfAClass checks the cost order inside one priority class:
// of two plain targets the closer wins, whatever the unit list order.
func TestNearestEnemyPrefersTheCloserOfAClass(t *testing.T) {
	f := newAttackFixture(t)
	hunter := armedTank(1, 1, 3, 3, f.units)
	near := unarmedAt(f, 0, 90, 10, 3)
	unarmedAt(f, 0, 91, 14, 3) // listed after the near one

	if got := f.attack.nearestEnemy(hunter); got != near {
		t.Errorf("target %v, want the closer plain target", got)
	}
}

// TestImmediateThreatRules checks the threat test: an armed enemy whose range just covers the
// unit is a threat; one a cell further, an unarmed one, and one hidden in fog are not.
func TestImmediateThreatRules(t *testing.T) {
	f := newAttackFixture(t)
	hunter := armedTank(1, 1, 3, 3, f.units)
	atRange := armedTank(0, 90, 7, 3, f.units) // range 4, 4 cells away
	beyond := armedTank(0, 91, 3, 8, f.units)  // 5 cells away
	unarmed := unarmedAt(f, 0, 92, 4, 4)

	if !f.attack.immediateThreat(hunter, atRange) {
		t.Error("an armed enemy exactly at its range is not a threat")
	}
	if f.attack.immediateThreat(hunter, beyond) {
		t.Error("an enemy a cell past its range is a threat")
	}
	if f.attack.immediateThreat(hunter, unarmed) {
		t.Error("an unarmed enemy is a threat")
	}

	// Fog on: the hunter's short scan does not reach a gun that can reach it.
	g := newAttackFixture(t)
	g.g.Vis = vis.NewView(24, 24)
	hunter2 := armedTank(1, 1, 3, 3, g.units)
	hunter2.Values.Set(game.AttribScan, 1)
	hidden := armedTank(0, 90, 6, 3, g.units)
	g.g.RefreshVisibility()
	if g.attack.immediateThreat(hunter2, hidden) {
		t.Error("an enemy hidden in fog is a threat")
	}
}

// TestImmediateThreatForPicksTheNearestGun checks the self defence pick: of two guns trained
// on the unit the nearer wins, and a gun on the map edge counts.
func TestImmediateThreatForPicksTheNearestGun(t *testing.T) {
	f := newAttackFixture(t)
	hunter := armedTank(1, 1, 2, 3, f.units)
	armedTank(0, 90, 2, 6, f.units)
	near := armedTank(0, 91, 0, 3, f.units) // the map edge, 2 cells away

	if got := f.attack.immediateThreatFor(hunter); got != near {
		t.Errorf("threat %v, want the nearer gun on the map edge", got)
	}
}

// targetTeamCase sets up one pickTargetTeam situation.
type targetTeamCase struct {
	name  string
	setup func(f *attackFixture)
	want  int
}

// TestPickTargetTeamRules checks the target team: the highest score, then the fewest
// buildings, then the lowest index; an eliminated team, an empty slot, a team with no units
// and the own team are never chosen.
func TestPickTargetTeamRules(t *testing.T) {
	cases := []targetTeamCase{
		{"the higher score wins, team 0 first", func(f *attackFixture) {
			unarmedAt(f, 0, 90, 5, 5)
			unarmedAt(f, 2, 91, 6, 6)
			f.g.TeamPoints[0], f.g.TeamPoints[2] = 5, 3
		}, 0},
		{"the higher score wins, team 2 later", func(f *attackFixture) {
			unarmedAt(f, 0, 90, 5, 5)
			unarmedAt(f, 2, 91, 6, 6)
			f.g.TeamPoints[0], f.g.TeamPoints[2] = 3, 5
		}, 2},
		{"a tie goes to the fewest buildings", func(f *attackFixture) {
			stationary(0, 90, assets.MiningSt, 5, 5, f.units)
			stationary(0, 91, assets.MiningSt, 7, 5, f.units)
			stationary(2, 92, assets.MiningSt, 9, 5, f.units)
			for i := 0; i < 3; i++ {
				unarmedAt(f, 2, 93+i, 11+i, 5)
			}
		}, 2},
		{"a full tie goes to the lowest index", func(f *attackFixture) {
			stationary(0, 90, assets.MiningSt, 5, 5, f.units)
			stationary(2, 91, assets.MiningSt, 9, 5, f.units)
		}, 0},
		{"an eliminated team is skipped", func(f *attackFixture) {
			unarmedAt(f, 0, 90, 5, 5)
			unarmedAt(f, 2, 91, 6, 6)
			f.g.TeamPoints[0] = 9
			f.g.State.Teams[0].Eliminated = true
		}, 2},
		{"an empty slot is skipped", func(f *attackFixture) {
			unarmedAt(f, 0, 90, 5, 5)
			unarmedAt(f, 3, 91, 6, 6)
			f.g.TeamPoints[3] = 9
			f.g.State.Teams[3].Type = game.TeamNone
		}, 0},
		{"a team with no units is skipped", func(f *attackFixture) {
			unarmedAt(f, 2, 90, 5, 5)
			f.g.TeamPoints[0] = 9
		}, 2},
		{"the own team is never the target", func(f *attackFixture) {
			unarmedAt(f, 1, 90, 5, 5)
			unarmedAt(f, 2, 91, 6, 6)
			f.g.TeamPoints[1] = 9
		}, 2},
	}
	for _, c := range cases {
		f := newAttackFixture(t)
		c.setup(f)
		if got := f.attack.pickTargetTeam(f.g); got != c.want {
			t.Errorf("%s: target team %d, want %d", c.name, got, c.want)
		}
	}
}

// TestBestSpottedTargetRules checks the force objective: the search keeps to the target team
// (team 0 included), a key facility outranks a closer plain unit, and the closer of a class
// wins.
func TestBestSpottedTargetRules(t *testing.T) {
	f := newAttackFixture(t)
	mine := unarmedAt(f, 0, 90, 20, 20)
	unarmedAt(f, 2, 91, 3, 3) // nearer, but another team
	if got := f.attack.bestSpottedTarget(0, 2, 2); got != mine {
		t.Errorf("objective %v, want team 0's unit (the search keeps to team 0)", got)
	}

	g := newAttackFixture(t)
	near := unarmedAt(g, 0, 90, 5, 2)
	unarmedAt(g, 0, 91, 9, 2) // listed after the near one
	if got := g.attack.bestSpottedTarget(-1, 2, 2); got != near {
		t.Errorf("objective %v, want the closer plain unit", got)
	}

	h := newAttackFixture(t)
	unarmedAt(h, 0, 90, 3, 2)
	facility := stationary(0, 91, assets.MiningSt, 15, 2, h.units)
	if got := h.attack.bestSpottedTarget(-1, 2, 2); got != facility {
		t.Errorf("objective %v, want the key facility over a closer plain unit", got)
	}
}

// TestBestSpottedTargetSkipsAShelvedFoggedTarget checks the shelve rule of the objective: a
// target shelved while fogged is skipped during its cooldown and retried after it, and a
// shelved target in plain sight is not skipped.
func TestBestSpottedTargetSkipsAShelvedFoggedTarget(t *testing.T) {
	f := newAttackFixture(t)
	f.g.Vis = vis.NewView(24, 24)
	hunter := armedTank(1, 1, 12, 12, f.units)
	hunter.Values.Set(game.AttribScan, 7)
	facility := stationary(0, 90, assets.MiningSt, 12, 7, f.units) // spotted, then fogged
	f.g.RefreshVisibility()
	f.units.Move(facility, 12, 22)
	f.g.RefreshVisibility()
	other := unarmedAt(f, 0, 91, 12, 14) // in sight
	f.g.RefreshVisibility()
	if f.g.Vis.VisibleToTeam(facility, 1) {
		t.Fatal("setup: the facility should be fogged")
	}

	f.g.State.TurnCounter = 100
	f.attack.abandon(facility)
	if got := f.attack.bestSpottedTarget(-1, 12, 12); got != other {
		t.Errorf("objective %v during the cooldown, want the other target", got)
	}

	f.g.State.TurnCounter = 100 + abandonCooldown
	if got := f.attack.bestSpottedTarget(-1, 12, 12); got != facility {
		t.Errorf("objective %v after the cooldown, want the facility again", got)
	}

	// Back in sight: a shelved target is pursued at once.
	f.g.State.TurnCounter = 105
	f.attack.abandon(facility)
	f.units.Move(facility, 12, 9)
	f.g.RefreshVisibility()
	if got := f.attack.bestSpottedTarget(-1, 12, 12); got != facility {
		t.Errorf("objective %v with the facility in sight, want the facility", got)
	}
}

// TestTargetSearchesSeeBothMapEdges checks the remaining edges: a gun in row 0 for the self
// defence pick, and objectives in column 0 and row 0 for the force objective.
func TestTargetSearchesSeeBothMapEdges(t *testing.T) {
	f := newAttackFixture(t)
	hunter := armedTank(1, 1, 3, 2, f.units)
	gun := armedTank(0, 90, 3, 0, f.units)
	if got := f.attack.immediateThreatFor(hunter); got != gun {
		t.Errorf("threat %v, want the gun in row 0", got)
	}

	for _, edge := range []game.Point{{X: 0, Y: 5}, {X: 5, Y: 0}} {
		g := newAttackFixture(t)
		target := unarmedAt(g, 0, 90, edge.X, edge.Y)
		if got := g.attack.bestSpottedTarget(-1, 2, 2); got != target {
			t.Errorf("edge %v: objective %v, want the target on the map edge", edge, got)
		}
	}
}
