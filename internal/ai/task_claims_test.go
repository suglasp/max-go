// task_claims_test.go : testing unit for the units each task holds in the unit ledger
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// TestEachTaskHoldsTheUnitsItUses checks that after one pass through a manager each task
// holds the units it works with, so a working unit never counts as idle for the
// production check, and leaves the ones it cannot use free.
func TestEachTaskHoldsTheUnitsItUses(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *obtainFixture) (Task, *game.UnitInfo, bool) // the task, a unit, whether it must hold it
	}{
		{"attack holds a ready tank", func(f *obtainFixture) (Task, *game.UnitInfo, bool) {
			return newTaskAttackEnemies(1, f.ctx), armedTank(1, 50, 8, 8, f.units), true
		}},
		{"commando holds a ready commando", func(f *obtainFixture) (Task, *game.UnitInfo, bool) {
			u := commandoUnit(1, 50, 8, 8, f.units)
			u.Shots = 1
			u.Speed = 8
			return newTaskCommando(1, f.ctx), u, true
		}},
		{"mines holds a layer with mines aboard", func(f *obtainFixture) (Task, *game.UnitInfo, bool) {
			u := mineLayerUnit(1, 50, 8, 8, f.units)
			u.Storage = 5
			u.Speed = 8
			return newTaskMines(1, f.ctx), u, true
		}},
		{"mines leaves an empty layer free", func(f *obtainFixture) (Task, *game.UnitInfo, bool) {
			u := mineLayerUnit(1, 50, 8, 8, f.units)
			u.Storage = 0
			u.Speed = 8
			return newTaskMines(1, f.ctx), u, false
		}},
		{"survey holds a ready surveyor", func(f *obtainFixture) (Task, *game.UnitInfo, bool) {
			f.g.Resources = economy.NewResourceMap(16, 16)
			u := liveSurveyor(1, 8, 8, f.units)
			u.Speed = 12
			return newTaskSurvey(1, f.ctx), u, true
		}},
		{"bulldozer holds its dozer with no rubble about", func(f *obtainFixture) (Task, *game.UnitInfo, bool) {
			return newTaskBulldozer(1, f.ctx), bulldozerUnit(1, 50, 8, 8, f.units), true
		}},
		{"the building manager holds a builder", func(f *obtainFixture) (Task, *game.UnitInfo, bool) {
			return newTaskManageBuildings(1, StrategyCombinedArms, 0, false, f.ctx), constructorUnit(1, 50, 8, 8, f.units), true
		}},
		{"the building manager holds a builder at work", func(f *obtainFixture) (Task, *game.UnitInfo, bool) {
			u := constructorUnit(1, 50, 8, 8, f.units)
			u.Building = true
			return newTaskManageBuildings(1, StrategyCombinedArms, 0, false, f.ctx), u, true
		}},
	}
	for _, c := range cases {
		f := newObtainFixture(t)
		task, unit, hold := c.setup(f)
		f.tasks.AppendTask(task)
		f.tasks.ExecuteReminders() // the first pass

		held := f.tasks.owner[unit] == task
		if held != hold {
			t.Errorf("%s: held = %v, want %v (owner %v)", c.name, held, hold, f.tasks.owner[unit])
		}
	}
}

// TestExploreLeavesCombatUnitsToTheAttack is the keep seam with the explore roles: an idle
// tank with scan stays in the attack's force over two turns, and explore never drives it
// (only recon units explore).
func TestExploreLeavesCombatUnitsToTheAttack(t *testing.T) {
	f := newObtainFixture(t)
	tank := armedTank(1, 50, 8, 8, f.units)
	tank.Hits = 10
	tank.Values.Set(game.AttribHits, 10)
	tank.Values.Set(game.AttribScan, 3)
	attack := newTaskAttackEnemies(1, f.ctx)
	explore := newTaskExplore(1, f.ctx)
	f.tasks.AppendTask(attack)
	f.tasks.AppendTask(explore)
	f.tasks.ExecuteReminders()

	for turn := 1; turn <= 2; turn++ {
		if f.tasks.owner[tank] != Task(attack) || f.g.HasOrder(tank) {
			t.Errorf("turn %d: owner %v, order %v; want the attack holding the idle tank", turn, f.tasks.owner[tank], f.g.HasOrder(tank))
		}
		if explore.isReadyExplorer(tank) {
			t.Errorf("turn %d: explore counts a tank as an explorer", turn)
		}
		f.tasks.BeginTurn(1)
		f.tasks.ExecuteReminders()
	}
}
