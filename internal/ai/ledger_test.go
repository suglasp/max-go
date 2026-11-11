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

// TestLedgerTurnStartClearsOnlyThatTeam checks that a team's turn start empties only its
// own ledger entries.
func TestLedgerTurnStartClearsOnlyThatTeam(t *testing.T) {
	m := NewTaskManager()
	first := newLedgerProbe(m, 1, taskPriorityAttackDefault)
	second := newLedgerProbe(m, 2, taskPriorityAttackDefault)
	mine := &game.UnitInfo{ID: 1, Team: 1}
	theirs := &game.UnitInfo{ID: 2, Team: 2}
	first.claim(mine)
	second.claim(theirs)

	m.BeginTurn(1)
	if m.owner[mine] != nil {
		t.Error("team 1's entry survived its turn start")
	}
	if m.owner[theirs] != Task(second) {
		t.Error("team 1's turn start cleared team 2's entry")
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
