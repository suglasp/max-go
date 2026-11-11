// ledger_test.go : testing unit for the unit ledger in taskmanager.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// vetoTask is a probe task that never lets another task take its units.
type vetoTask struct {
	probeTask
}

// isUnitTransferable refuses every transfer.
func (v *vetoTask) isUnitTransferable(_ *game.UnitInfo) bool {
	return false
}

// newLedgerProbe registers a probe task of the given team and priority on m.
func newLedgerProbe(m *TaskManager, team int, priority uint16) *probeTask {
	task := &probeTask{TaskBase: newTaskBase(team, nil, priority)}
	m.AppendTask(task)

	return task
}

// TestLedgerClaimRules checks the claim rule: free units, a more urgent taker, a less
// urgent one, a parent veto, and the release on RemoveTask.
func TestLedgerClaimRules(t *testing.T) {
	m := NewTaskManager()
	unit := &game.UnitInfo{ID: 1, Team: 1}

	attack := newLedgerProbe(m, 1, taskPriorityAttackDefault)
	if !attack.claim(unit) {
		t.Fatal("a free unit was refused")
	}
	if !attack.claim(unit) {
		t.Error("the owner could not claim its own unit again")
	}

	survey := newLedgerProbe(m, 1, taskPriorityAutoSurvey)
	if survey.mayClaim(unit) || survey.claim(unit) {
		t.Error("a less urgent task took a unit from a more urgent one")
	}
	if m.owner[unit] != Task(attack) {
		t.Fatal("a refused claim changed the owner")
	}

	retreat := newLedgerProbe(m, 1, taskPriorityEmergency)
	if !retreat.claim(unit) {
		t.Fatal("a more urgent task could not take a transferable unit")
	}
	if m.owner[unit] != Task(retreat) {
		t.Error("the more urgent claim did not move the unit")
	}

	// A child whose parent vetoes keeps its unit even against the most urgent tier.
	parent := &vetoTask{probeTask{TaskBase: newTaskBase(1, nil, taskPriorityAutoSurvey)}}
	m.AppendTask(parent)
	child := &probeTask{TaskBase: newTaskBase(1, parent, taskPriorityAutoSurvey)}
	m.AppendTask(child)
	held := &game.UnitInfo{ID: 2, Team: 1}
	if !child.claim(held) {
		t.Fatal("the child could not claim a free unit")
	}
	if retreat.claim(held) {
		t.Error("the unit left a child whose parent vetoes the transfer")
	}

	// A removed task holds nothing.
	retreat.RemoveSelf()
	if m.owner[unit] != nil {
		t.Error("RemoveTask left the removed task's unit in the ledger")
	}
	if !survey.claim(unit) {
		t.Error("a released unit was refused")
	}
}

// claimProbe is a probe task whose pass claims the units in its list.
type claimProbe struct {
	TaskBase
	units []*game.UnitInfo
}

// BeginTurn claims every listed unit.
func (c *claimProbe) BeginTurn() {
	for _, u := range c.units {
		c.claim(u)
	}
}

// newClaimProbe registers a claim probe of team 1 with the given priority on m.
func newClaimProbe(m *TaskManager, priority uint16, units ...*game.UnitInfo) *claimProbe {
	task := &claimProbe{TaskBase: newTaskBase(1, nil, priority), units: units}
	m.AppendTask(task)

	return task
}

// runTurn runs one full turn of team 1 through the manager: every task makes one pass.
func runTurn(m *TaskManager) {
	m.BeginTurn(1)
	m.ExecuteReminders()
}

// TestLedgerKeepsAUnitAcrossTurns checks the reference keep: a task holds its unit from
// one turn to the next, so a less urgent task running before it in the next turn cannot
// take it, and a unit of another team is never touched.
func TestLedgerKeepsAUnitAcrossTurns(t *testing.T) {
	m := NewTaskManager()
	unit := &game.UnitInfo{ID: 1, Team: 1, Hits: 10}
	weak := newClaimProbe(m, taskPriorityAutoSurvey) // registered first: runs first every turn
	strong := newClaimProbe(m, taskPriorityEscort, unit)
	m.ExecuteReminders() // the first passes (Init): strong takes the unit

	if m.owner[unit] != Task(strong) {
		t.Fatal("the first pass did not give the unit to its task")
	}

	weak.units = []*game.UnitInfo{unit} // next turn the weak task wants it too, and runs first
	runTurn(m)

	if m.owner[unit] != Task(strong) {
		t.Errorf("owner after the second turn = %v, want the task that kept it", m.owner[unit])
	}
}

// TestLedgerLetsGoOfAUnitNotClaimedAgain checks letting go: a unit the task did not claim
// again in its pass (repaired, no longer wanted, dead) is free once the pass ends, so a
// later task of the same turn can take it.
func TestLedgerLetsGoOfAUnitNotClaimedAgain(t *testing.T) {
	m := NewTaskManager()
	unit := &game.UnitInfo{ID: 1, Team: 1, Hits: 10}
	strong := newClaimProbe(m, taskPriorityEscort, unit)
	weak := newClaimProbe(m, taskPriorityAutoSurvey) // registered after: runs after
	m.ExecuteReminders()

	strong.units = nil // the strong task stops using the unit
	weak.units = []*game.UnitInfo{unit}
	runTurn(m)

	if m.owner[unit] != Task(weak) {
		t.Errorf("owner = %v, want the later task (the unit was let go at the end of the first pass)", m.owner[unit])
	}

	// A task that keeps claiming keeps the unit: the weak one holds it next turn too.
	runTurn(m)
	if m.owner[unit] != Task(weak) {
		t.Error("a unit claimed again in every pass was let go")
	}

	// A task that drops the unit lets go of it at the end of that pass.
	weak.units = nil
	runTurn(m)
	if m.owner[unit] != nil {
		t.Error("a unit no task claims any more is still held")
	}
}

// TestLedgerOrderCheckBlocksABusyRecruit checks Task_IsRecruitable's order check: a more
// urgent task takes a held unit only when the unit is ready for orders.
func TestLedgerOrderCheckBlocksABusyRecruit(t *testing.T) {
	m := NewTaskManager()
	unit := &game.UnitInfo{ID: 1, Team: 1, Hits: 10}
	weak := newLedgerProbe(m, 1, taskPriorityAutoSurvey)
	urgent := newLedgerProbe(m, 1, taskPriorityEmergency)
	if !weak.claim(unit) {
		t.Fatal("fixture: the free unit was refused")
	}

	ready := false
	m.readyForOrders = func(*game.UnitInfo) bool { return ready }
	if urgent.mayClaim(unit) || urgent.claim(unit) {
		t.Error("a more urgent task took a unit that is busy")
	}

	ready = true
	if !urgent.claim(unit) {
		t.Error("a more urgent task could not take a ready unit")
	}

	// The owner's own claim and a free unit never need the check.
	ready = false
	free := &game.UnitInfo{ID: 2, Team: 1}
	if !urgent.claim(unit) || !weak.claim(free) {
		t.Error("the order check blocked a claim that needs no recruit")
	}
}

// TestUnitReadyForOrders checks the live check: alive, not building, not clearing rubble.
func TestUnitReadyForOrders(t *testing.T) {
	f := newObtainFixture(t)
	unit := liveSurveyor(1, 6, 6, f.units)
	if !unitReadyForOrders(f.g, unit) {
		t.Fatal("an idle living unit is not ready for orders")
	}

	unit.Building = true
	if unitReadyForOrders(f.g, unit) {
		t.Error("a building unit is ready for orders")
	}

	unit.Building = false
	unit.Hits = 0
	if unitReadyForOrders(f.g, unit) {
		t.Error("a dead unit is ready for orders")
	}
}

// TestLedgerKeepsTheSameUnitsApart is the same play seam: through the manager, in run
// order, retreat holds a damaged tank even on the tier that never retreats, so the attack
// and explore tasks leave it alone, while a healthy tank stays free for them.
func TestLedgerKeepsTheSameUnitsApart(t *testing.T) {
	prev := retreatEnabled
	retreatEnabled = false // the unit is held back but given no order, so only the ledger keeps it apart
	t.Cleanup(func() { retreatEnabled = prev })

	const w, h = 24, 24
	unitsMgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: unitsMgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), unitsMgr, grid, w, h)

	damaged := armedTank(1, 1, 4, 4, unitsMgr)
	damaged.Hits = 1
	healthy := armedTank(1, 2, 8, 8, unitsMgr)
	damaged.Values.Set(game.AttribScan, 3)
	healthy.Values.Set(game.AttribScan, 3)
	if !unitDamaged(damaged) || unitDamaged(healthy) {
		t.Fatal("fixture: want one damaged and one healthy tank")
	}

	ctx := &aiContext{Game: g, MakeUnit: testUnitFactory}
	m := NewTaskManager()
	retreat := newTaskRetreat(1, ctx)
	attack := newTaskAttackEnemies(1, ctx)
	explore := newTaskExplore(1, ctx)
	m.AppendTask(retreat)
	m.AppendTask(attack)
	m.AppendTask(explore)
	m.BeginTurn(1)
	m.ExecuteReminders()

	if m.owner[damaged] != Task(retreat) {
		t.Errorf("the damaged tank's owner = %v, want the retreat task", m.owner[damaged])
	}
	if g.HasOrder(damaged) {
		t.Error("the damaged tank got an order on the tier that never retreats")
	}
	if attack.isReadyAttacker(damaged) {
		t.Error("the attack task counts the damaged tank as a ready attacker")
	}
	if explore.isReadyExplorer(damaged) {
		t.Error("the explore task would send the damaged tank scouting")
	}
	// No enemy in sight: the attack leaves the healthy tank idle, so the explore task
	// sends it scouting and holds it.
	if m.owner[healthy] != Task(explore) || !g.HasOrder(healthy) {
		t.Errorf("the healthy tank: owner %v, has order %v; want the explore task driving it", m.owner[healthy], g.HasOrder(healthy))
	}
}
