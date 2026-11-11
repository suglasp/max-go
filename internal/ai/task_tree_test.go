// task_tree_test.go : testing unit for task.go and taskmanager.go (parent and child tasks)
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

// newChildProbe builds a probe task that hangs under parent.
func newChildProbe(parent Task) *probeTask {
	return &probeTask{TaskBase: newTaskBase(parent.Team(), parent, 100)}
}

// TestRootTaskIsNeededUntilRemoved checks a root task: needed while it lives, never after RemoveSelf.
func TestRootTaskIsNeededUntilRemoved(t *testing.T) {
	m := NewTaskManager()
	root := newProbeTask(0, 0)
	m.AppendTask(root)

	if !root.IsNeeded() {
		t.Fatal("a live root task is not needed")
	}
	if root.parent != nil {
		t.Error("a root task has a parent")
	}

	root.RemoveSelf()
	if root.IsNeeded() {
		t.Error("a removed root task is still needed")
	}
}

// TestChildIsNotNeededOnceItsParentIsRemoved checks the parent chain: a child follows its parent.
func TestChildIsNotNeededOnceItsParentIsRemoved(t *testing.T) {
	m := NewTaskManager()
	parent := newProbeTask(0, 0)
	m.AppendTask(parent)
	child := newChildProbe(parent)
	m.AppendTask(child)
	grandchild := newChildProbe(child)
	m.AppendTask(grandchild)

	if child.parent != Task(parent) {
		t.Fatal("the child does not report its parent")
	}
	if !child.IsNeeded() || !grandchild.IsNeeded() {
		t.Fatal("children of a live parent are not needed")
	}

	parent.RemoveSelf()
	if child.IsNeeded() {
		t.Error("the child is still needed after its parent was removed")
	}
	if grandchild.IsNeeded() {
		t.Error("the grandchild is still needed after the root was removed")
	}
}

// TestManagerPrunesTheChildrenOfARemovedTask checks the turn start prune: the children of a
// removed task leave the list and never plan again; other teams are left alone.
func TestManagerPrunesTheChildrenOfARemovedTask(t *testing.T) {
	m := NewTaskManager()
	parent := newProbeTask(0, 0)
	m.AppendTask(parent)
	child := newChildProbe(parent)
	m.AppendTask(child)
	other := newProbeTask(1, 0)
	m.AppendTask(other)
	m.ExecuteReminders() // drain the Init reminders

	parent.RemoveSelf()
	if m.TaskCount() != 2 {
		t.Fatalf("TaskCount after the parent's RemoveSelf = %d, want 2 (child + other team)", m.TaskCount())
	}

	before := child.beginCount
	m.BeginTurn(0)
	m.ExecuteReminders()

	if m.TaskCount() != 1 {
		t.Errorf("TaskCount after the prune = %d, want 1 (only the other team's task)", m.TaskCount())
	}
	if child.beginCount != before {
		t.Errorf("the pruned child planned again: beginCount %d -> %d", before, child.beginCount)
	}
}

// TestRemovedTaskReminderDoesNothing checks that a reminder still queued for a removed task
// runs neither BeginTurn nor EndTurn.
func TestRemovedTaskReminderDoesNothing(t *testing.T) {
	m := NewTaskManager()
	task := newProbeTask(0, 0)
	m.AppendTask(task) // queues the Init turn start reminder
	task.RemindTurnEnd(false)

	task.RemoveSelf()
	m.ExecuteReminders()

	if task.beginCount != 0 || task.endCount != 0 {
		t.Errorf("a removed task ran: beginCount = %d, endCount = %d, want 0 and 0", task.beginCount, task.endCount)
	}
}

// TestConnectionAssistantIsAChildOfTheBuildingManager checks the live seam: the team's first
// turn registers the connection assistant under the building manager, and removing the
// manager takes the assistant with it at the next turn start.
func TestConnectionAssistantIsAChildOfTheBuildingManager(t *testing.T) {
	const w, h = 8, 8
	unitsMgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: unitsMgr.Cells}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), unitsMgr, grid, w, h)

	a := New(testUnitFactory)
	g.AI = a
	g.BeginTeamTurn(1)

	var assistant *taskConnectionAssistant
	for _, task := range a.mgr.tasks {
		if ca, ok := task.(*taskConnectionAssistant); ok && ca.Team() == 1 {
			assistant = ca
		}
	}
	if assistant == nil {
		t.Fatal("no connection assistant registered for team 1")
	}
	if assistant.parent != Task(assistant.manager) {
		t.Fatal("the connection assistant's parent is not its building manager")
	}
	if assistant.basePriority != assistant.manager.basePriority {
		t.Errorf("assistant priority = %#x, want the manager's %#x", assistant.basePriority, assistant.manager.basePriority)
	}

	count := a.mgr.TaskCount()
	assistant.manager.RemoveSelf()
	a.mgr.BeginTurn(1)

	if a.mgr.TaskCount() != count-2 {
		t.Errorf("TaskCount = %d, want %d (manager and assistant gone)", a.mgr.TaskCount(), count-2)
	}
	for _, task := range a.mgr.tasks {
		if task == Task(assistant) {
			t.Error("the assistant outlived its building manager")
		}
	}
}
